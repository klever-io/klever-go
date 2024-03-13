//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. validatorData.proto

package validators

import (
	"github.com/klever-io/klever-go/data/state"
)

type ValidatorAccountInfo struct {
	*ValidatorData
	Stats state.PeerAccountHandler
}

func (vai *ValidatorAccountInfo) GetStats() state.PeerAccountHandler {
	return vai.Stats
}

// SetCanDelegate -
func (x *ValidatorData) SetCanDelegate(canDelegate bool) {
	x.CanDelegate = canDelegate
}

// SetMaxDelegation -
func (x *ValidatorData) SetMaxDelegation(maxDelegationAmount int64) {
	x.MaxDelegation = maxDelegationAmount
}

// SetCommission -
func (x *ValidatorData) SetCommission(commission uint32) {
	x.Commission = commission
}
