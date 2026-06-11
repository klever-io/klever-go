package kapps

import (
	"github.com/klever-io/klever-go/core"
)

// Governance scripts are hardcoded, named actions that an approved proposal can
// trigger by setting the ExecuteScript parameter to the script's Name. Some
// scripts may run any number of times over the chain's lifetime (OneTime=false),
// while others must run exactly once (OneTime=true). The one-time guarantee is
// derived from the persisted proposal history rather than a dedicated index: a
// one-time script counts as executed when some earlier approved proposal carried
// the same ExecuteScript trigger (see ScriptExecutedInHistory).

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

// ScriptExecutedInHistory reports whether a script has already been executed by an
// approved proposal somewhere in the chain's history. It scans the persisted proposals
// (IDs 1..proposalCount) and returns true as soon as it finds an approved proposal whose
// parameters carry the same ExecuteScript trigger.
//
// loadProposal returns the stored ProposalData for a given ID. A nil proposal is skipped,
// so callers may return nil for IDs that are absent instead of erroring.
func ScriptExecutedInHistory(name string, proposalCount uint64, loadProposal func(id uint64) (*ProposalData, error)) (bool, error) {
	scriptKey := int32(EnumParameter_ExecuteScript)
	for id := uint64(1); id <= proposalCount; id++ {
		proposal, err := loadProposal(id)
		if err != nil {
			return false, err
		}
		if proposal == nil || proposal.ProposalStatus != ProposalData_ApprovedProposal {
			continue
		}
		if val, ok := proposal.Parameters[scriptKey]; ok && string(val) == name {
			return true, nil
		}
	}
	return false, nil
}
