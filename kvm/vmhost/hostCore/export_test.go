package hostCore

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

// PayCreateTransfersForTest exposes payCreateTransfers, which pays a deploy's KDA transfers.
func PayCreateTransfersForTest(
	forks core.ForkController,
	accKapp func() kapp.AccountsKapp,
	caller, contract []byte,
	transfers []*vmcommon.KDATransfer,
) error {
	return payCreateTransfers(forks, accKapp, caller, contract, transfers)
}

// GetEffectiveTimeoutForTest exposes the private getEffectiveTimeout so external tests can assert
// the duration selected for each execution mode.
func GetEffectiveTimeoutForTest(host vmhost.VMHost) time.Duration {
	return host.(*vmHost).getEffectiveTimeout()
}

func CallBuiltinFunctionForTest(host vmhost.VMHost, input *vmcommon.ContractCallInput) error {
	_, _, err := host.(*vmHost).callBuiltinFunction(input)
	return err
}
