package process

import (
	"fmt"
	"time"
)

// BlockHeaderState specifies which is the state of the block header received
type BlockHeaderState int

const (
	// BHReceived defines ID of a received block header
	BHReceived BlockHeaderState = iota
	// BHReceivedTooLate defines ID of a late received block header
	BHReceivedTooLate
	// BHProcessed defines ID of a processed block header
	BHProcessed
	// BHProposed defines ID of a proposed block header
	BHProposed
	// BHNotarized defines ID of a notarized block header
	BHNotarized
)

// MaxHeadersToRequestInAdvance defines the maximum number of headers which will be requested in advance,
// if they are missing
const MaxHeadersToRequestInAdvance = 20

// MaxSlotsWithoutNewBlockReceived defines the maximum number of slots to wait for a new block to be received,
// before a special action to be applied
const MaxSlotsWithoutNewBlockReceived = 10

// MaxSyncWithErrorsAllowed defines the maximum allowed number of sync with errors,
// before a special action to be applied
const MaxSyncWithErrorsAllowed = 10

// SlotModulusTrigger defines a slot modulus on which a trigger for an action will be released
const SlotModulusTrigger = 5

// SlotModulusTriggerWhenSyncIsStuck defines a slot modulus on which a trigger for an action when sync is stuck will be released
const SlotModulusTriggerWhenSyncIsStuck = 20

// MinForkSlot represents the minimum fork slot set by a notarized header received
const MinForkSlot = uint64(0)

// MaxSlotsWithoutCommittedBlock defines the maximum slots to wait for a new block to be committed,
// before a special action to be applied
const MaxSlotsWithoutCommittedBlock = 10

// NonceDifferenceWhenSynced defines the difference between probable highest nonce seen from network and node's last
// committed block nonce, after which, node is considered himself not synced
const NonceDifferenceWhenSynced = 0

// BlockFinality defines the block finality which is used in meta-chain/shards (the real finality in shards is given
// by meta-chain)
const BlockFinality = 1

// MaxNumOfTxsToSelect defines the maximum number of transactions that should be selected from the cache
const MaxNumOfTxsToSelect = 12000

// NumTxPerBatchForFillingBlock defines the number of transactions to be drawn
// from the transactions pool, for a specific sender, in a single pass.
// Drawing transactions for a block happens in multiple passes, until "MaxItemsInBlock" are drawn.
const NumTxPerBatchForFillingBlock = 10

// EpochChangeGracePeriod defines the allowed slot numbers till the node has to change the epoch
const EpochChangeGracePeriod = 1

// TimeDurationMultiplierForProcessBlockWhenSync represents the constant that will be multiplied with the slot duration
// when considering the maximum available time window for block processing when syncing blocks
// timeout now will be slotManager.TimeDuration() * TimeDurationMultiplierForProcessBlockWhenSync
const TimeDurationMultiplierForProcessBlockWhenSync = time.Duration(4)

// DefaultMaxComputableSlots defines the default maximum number of computable slots
const DefaultMaxComputableSlots = uint64(100)

// TransactionType specifies the type of the transaction
type TransactionType int

const (
	// MoveBalance defines ID of a payment transaction - moving balances
	MoveBalance TransactionType = iota
	// SCDeployment defines ID of a transaction to store a smart contract
	SCDeployment
	// SCInvoking defines ID of a transaction of type smart contract call
	SCInvoking
	// BuiltInFunctionCall defines ID of a builtin function call
	BuiltInFunctionCall
	// RelayedTx defines ID of a transaction of type relayed
	RelayedTx
	// RelayedTxV2 defines the ID of a slim relayed transaction version
	RelayedTxV2
	// RewardTx defines ID of a reward transaction
	RewardTx
	// InvalidTransaction defines unknown transaction type
	InvalidTransaction
)

func (transactionType TransactionType) String() string {
	switch transactionType {
	case MoveBalance:
		return "MoveBalance"
	case SCDeployment:
		return "SCDeployment"
	case SCInvoking:
		return "SCInvoking"
	case BuiltInFunctionCall:
		return "BuiltInFunctionCall"
	case RelayedTx:
		return "RelayedTx"
	case RelayedTxV2:
		return "RelayedTxV2"
	case RewardTx:
		return "RewardTx"
	case InvalidTransaction:
		return "InvalidTransaction"
	default:
		return fmt.Sprintf("type %d", transactionType)
	}
}

// MaxGasFeeHigherFactorAccepted defines the maximum higher factor of gas fee put inside a transaction compared with
// the real gas used, after which the transaction will be considered an attack and all the gas will be consumed and
// nothing will be refunded to the sender
const MaxGasFeeHigherFactorAccepted = 10
