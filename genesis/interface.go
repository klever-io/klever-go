package genesis

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
)

// AccountsParser contains the parsed genesis json file and has some functionality regarding processed data
type AccountsParser interface {
	InitialAccounts() []InitialAccountHandler
	GetTotalStakedForDelegationAddress(delegationAddress string) int64
	GetInitialAccountsForDelegated(addressBytes []byte) []InitialAccountHandler
	IsInterfaceNil() bool
}

// InitialNodesHandler contains the initial nodes setup
type InitialNodesHandler interface {
	InitialNodesInfo() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error)
	MinNumberOfNodes() uint32
	IsInterfaceNil() bool
}

// InitialAccountHandler represents the interface that describes the data held by an initial account
type InitialAccountHandler interface {
	Clone() InitialAccountHandler
	GetAddress() string
	AddressBytes() []byte
	GetBalanceValue() int64
	SetBalanceValue(value int64)
	GetKFIBalanceValue() int64
	SetKFIBalanceValue(value int64)
	GetDelegationHandler() DelegationDataHandler
	GetPermissionsHandler() PermissionsDataHandler
	IsInterfaceNil() bool
}

// DelegationDataHandler represents the interface that describes the data held by a delegation address
type DelegationDataHandler interface {
	GetAddress() string
	AddressBytes() []byte
	GetValue() int64
	SetValue(value int64)
	IsInterfaceNil() bool
}

// PermissionHandler -
type PermissionHandler interface {
	GetID() int32
	GetType() int
	GetPermissionName() string
	GetThreshold() int64
	GetOperations() []byte
	GetSigners() map[string]int64
}

// PermissionsDataHandler represents the interface that describes the data held by address permissions
type PermissionsDataHandler interface {
	IsInterfaceNil() bool
	Len() int
	Get(int) PermissionHandler
}

// TxExecutionProcessor represents a transaction builder and executor containing also related helper functions
type TxExecutionProcessor interface {
	ExecuteTransaction() error
	GetAccount(address []byte) (state.UserAccountHandler, bool)
	AddBalance(senderBytes []byte, value int64) error
	IsInterfaceNil() bool
}

// NodesListSplitter is able to split de initial nodes based on some criteria
type NodesListSplitter interface {
	GetAllNodes() []sharding.GenesisNodeInfoHandler
	GetDelegatedNodes(delegationScAddress []byte) []sharding.GenesisNodeInfoHandler
	IsInterfaceNil() bool
}
