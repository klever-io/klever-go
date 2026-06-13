package kapps

import (
	"slices"

	"github.com/klever-io/klever-go/core"
)

// Governance scripts are hardcoded, named actions that an approved proposal can
// trigger by setting the ExecuteScript parameter to the script's Name. Some
// scripts may run any number of times over the chain's lifetime (OneTime=false),
// while others must run exactly once (OneTime=true). The one-time guarantee is
// derived from the persisted proposal history rather than a dedicated index, via
// two complementary checks:
//
//   - creation: ScriptPendingOrExecuted rejects a one-time proposal when another
//     non-denied proposal (still active, or already approved) already carries the
//     same trigger — so a one-time script has at most one live proposal at a time;
//   - execution: ScriptExecutedInHistory skips the script at end-of-epoch if an
//     earlier approved proposal already ran it.

// Script name constants. Add a new constant here and register it in scriptRegistry
// to expose a new governance-triggered action.
const (
	// ScriptBurnKLV wipes the native KLV balance of the configured target
	// addresses. It is a one-time action over the whole chain history.
	ScriptBurnKLV = "BurnKLV"
)

// ScriptDefinition describes the metadata of a governance script. The actual
// execution lives in the block processor, which dispatches on Name.
type ScriptDefinition struct {
	// Name is the identifier set as the ExecuteScript parameter value.
	Name string
	// OneTime, when true, restricts the script to a single execution over the
	// chain's lifetime. Repeatable scripts leave it false.
	OneTime bool
	// Enabled optionally gates the script behind a fork flag. A nil func means
	// the script is enabled whenever the ExecuteScript parameter itself is active.
	Enabled func(core.ForkController) bool
}

// IsEnabled reports whether the script may be proposed under the current forks.
func (d ScriptDefinition) IsEnabled(fc core.ForkController) bool {
	if d.Enabled == nil {
		return true
	}
	return d.Enabled(fc)
}

// scriptRegistry is the single source of truth for the available governance scripts.
var scriptRegistry = map[string]ScriptDefinition{
	ScriptBurnKLV: {
		Name:    ScriptBurnKLV,
		OneTime: true,
		Enabled: func(fc core.ForkController) bool { return fc.ProposalScriptExecution() },
	},
}

// LookupScript returns the definition for the given script name, if registered.
func LookupScript(name string) (ScriptDefinition, bool) {
	def, ok := scriptRegistry[name]
	return def, ok
}

// RegisteredScriptNames returns the names of every registered governance script.
func RegisteredScriptNames() []string {
	names := make([]string, 0, len(scriptRegistry))
	for name := range scriptRegistry {
		names = append(names, name)
	}
	return names
}

// scriptInHistory scans the persisted proposals (IDs 1..proposalCount) and returns true as soon
// as it finds a proposal whose status is one of `statuses` and whose parameters carry the given
// ExecuteScript trigger. loadProposal returns the stored ProposalData for an ID; a nil proposal
// (absent ID) is skipped, so callers may return nil instead of erroring.
func scriptInHistory(
	name string,
	proposalCount uint64,
	loadProposal func(id uint64) (*ProposalData, error),
	statuses ...ProposalData_EnumProposalStatus,
) (bool, error) {
	scriptKey := int32(EnumParameter_ExecuteScript)
	for id := uint64(1); id <= proposalCount; id++ {
		proposal, err := loadProposal(id)
		if err != nil {
			return false, err
		}
		if proposal == nil || !slices.Contains(statuses, proposal.ProposalStatus) {
			continue
		}
		if val, ok := proposal.Parameters[scriptKey]; ok && string(val) == name {
			return true, nil
		}
	}
	return false, nil
}

// ScriptExecutedInHistory reports whether a script has already been EXECUTED by an approved
// proposal somewhere in chain history. It backs the end-of-epoch execution guard, so a one-time
// script runs at most once even if more than one approved proposal carries it.
func ScriptExecutedInHistory(name string, proposalCount uint64, loadProposal func(id uint64) (*ProposalData, error)) (bool, error) {
	return scriptInHistory(name, proposalCount, loadProposal, ProposalData_ApprovedProposal)
}

// ScriptPendingOrExecuted reports whether a one-time script already has a non-denied proposal —
// either still ACTIVE (awaiting its vote) or already APPROVED (executed). It backs the creation
// guard so a one-time script has at most one live proposal at a time: a second proposal naming it
// is rejected at submission rather than silently deduplicated at end-of-epoch. A DENIED prior
// proposal does not block — the script may be proposed again.
func ScriptPendingOrExecuted(name string, proposalCount uint64, loadProposal func(id uint64) (*ProposalData, error)) (bool, error) {
	return scriptInHistory(name, proposalCount, loadProposal, ProposalData_ActiveProposal, ProposalData_ApprovedProposal)
}
