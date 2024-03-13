package epochStart

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// TriggerRegistry holds the data required to correctly initialize the trigger when booting from saved state
type TriggerRegistry struct {
	Epoch                      uint32
	CurrentSlot                uint64
	EpochFinalityAttestingSlot uint64
	CurrEpochStartSlot         uint64
	PrevEpochStartSlot         uint64
	EpochStartHash             []byte
	EpochStartBlock            *block.Block
}

// AccountsDBSyncer defines the methods for the accounts db syncer
type AccountsDBSyncer interface {
	GetSyncedTries() map[string]data.Trie
	SyncAccounts(rootHash []byte) error
	IsInterfaceNil() bool
}
