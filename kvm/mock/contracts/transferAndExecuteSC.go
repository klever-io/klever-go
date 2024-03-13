package contracts

import (
	"fmt"

	"github.com/klever-io/klever-go/kvm/testcommon"
)

// TransferAndExecuteFuncName -
var TransferAndExecuteFuncName = "transferAndExecute"

// TransferAndExecuteReturnData -
var TransferAndExecuteReturnData = []byte{1, 2, 3}

// GetChildAddressForTransfer -
func GetChildAddressForTransfer(transfer int) []byte {
	return testcommon.MakeTestSCAddress(fmt.Sprintf("childSC-%d", transfer))
}
