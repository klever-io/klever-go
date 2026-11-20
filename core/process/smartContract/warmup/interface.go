package warmup

import (
	"github.com/klever-io/klever-go/storage"
)

// TxPool is the part of the transactions pool the warmer needs: notification of newly
// added transactions and read access to the pool contents for a rescan
type TxPool interface {
	RegisterOnAdded(handler func(key []byte, value interface{}))
	ShardDataStore(cacheID string) storage.Cacher
	IsInterfaceNil() bool
}

// AccountsReader reads contract code from the accounts adapter. GetAccountCode must read the
// account and its code under one hold of the adapter lock and must not load the account's data
// trie (state.AccountsDB.GetAccountCode does both), so the warmer's reads neither race a trie
// swap nor change what the consensus adapter tracks for the next Commit.
type AccountsReader interface {
	GetAccountCode(address []byte) ([]byte, error)
	IsInterfaceNil() bool
}

// CompiledCodeCache is the compiled-code cache of the blockchain hook, with the
// generation-guarded save the warmer must use
type CompiledCodeCache interface {
	GetCompiledCode(codeHash []byte) (bool, []byte)
	CompiledCodeGeneration() uint64
	SaveCompiledCodeIfCurrent(generation uint64, isCurrent func() bool, codeHash []byte, code []byte) bool
	RegisterCompiledCodesClearedHandler(handler func())
	IsInterfaceNil() bool
}

// GasScheduleProvider returns the gas schedule currently in force
type GasScheduleProvider interface {
	LatestGasSchedule() map[string]map[string]uint64
	IsInterfaceNil() bool
}

// ForkFlags are the fork gates that change how a contract is compiled
type ForkFlags interface {
	FixAuditChangesV4() bool
	FixAuditChangesV5() bool
	IsInterfaceNil() bool
}

// CompiledCodeWarmer precompiles the modules of contracts called by pending transactions
type CompiledCodeWarmer interface {
	TriggerRescan()
	Counters() Counters
	Close() error
	IsInterfaceNil() bool
}
