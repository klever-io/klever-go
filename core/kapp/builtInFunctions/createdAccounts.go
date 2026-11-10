package builtInFunctions

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/vmcommon"
)

// countCreatedAccounts starts counting the user accounts the accounts cacher creates (KLC-2938) and
// returns the charge for them, which takes from vmOutput a contract transfer (the VM's TransferValue
// and the Transfer builtin) for each account created since.
// A kapp's payout loads its receiver with LoadUserAccount, which creates the account when it is
// missing, and a builtin's flat cost does not price that. The kapp has already run when the charge
// is taken, so a call short of gas fails with ErrNotEnoughGas and the transaction discards what the
// kapp wrote, as it does for every failed builtin. Before FixAuditChangesV7 the charge takes nothing.
func countCreatedAccounts(
	accountsCacher state.AccountsCacher,
	transferCost uint64,
	forkController core.ForkController,
) func(vmOutput *vmcommon.VMOutput) (*vmcommon.VMOutput, error) {
	if !forkController.FixAuditChangesV7() {
		return func(vmOutput *vmcommon.VMOutput) (*vmcommon.VMOutput, error) { return vmOutput, nil }
	}

	createdBefore := accountsCacher.CreatedUsers()
	return func(vmOutput *vmcommon.VMOutput) (*vmcommon.VMOutput, error) {
		createdAfter := accountsCacher.CreatedUsers()
		created := createdAfter - createdBefore
		if createdAfter < createdBefore {
			// a ResetAll or SaveAll during the call restarted the count
			created = createdAfter
		}
		gasRemaining, err := computeBuiltinGasRemaining(
			vmOutput.GasRemaining, math.MulUint64(transferCost, created), forkController)
		if err != nil {
			return nil, err
		}

		vmOutput.GasRemaining = gasRemaining
		return vmOutput, nil
	}
}
