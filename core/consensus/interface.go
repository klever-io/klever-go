package consensus

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/network/p2p"
)

// SlotManager defines the actions which should be handled by a slot manager implementation
type SlotManager interface {
	Index() int64
	GenesisTimestamp() time.Time
	BeforeGenesis() bool
	// UpdateSlot updates the index and the time stamp of the slot depending of the genesis time and the current time given
	UpdateSlot(time.Time)
	Timestamp() time.Time
	TimeDuration() time.Duration
	RemainingTime(startTime time.Time, maxTime time.Duration) time.Duration
	ValidateSlotTimestamp(slot int64, timestamp int64) bool
	IsInterfaceNil() bool
}

// ChronologyHandler defines the actions which should be handled by a chronology implementation
type ChronologyHandler interface {
	Close() error
	AddSubslot(SubslotHandler)
	RemoveAllSubslots()
	// StartSlots starts slots in a sequential manner, one after the other
	StartSlots()
	IsInterfaceNil() bool
}

// SlotHandler defines the actions which should be handled in a slot
type SlotHandler interface {
	// IsMiner returns true if node can mine the block
	IsMiner(blsPubKey []byte) bool
	// ProducerPubKey returns handler public key
	ProducerPubKey() ([]byte, error)
	// ProduceBlock implements of the subslot's job
	ProduceBlock(blkc data.ChainHandler, sm SlotManager) (data.HeaderHandler, []byte, error)
	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// BroadcastMessenger defines the behaviour of the broadcast messages by the consensus group
type BroadcastMessenger interface {
	BroadcastBlock(data.HeaderHandler) error
	BroadcastBlockAndTransactions([]byte, [][]byte) error
	BroadcastTransactions([][]byte) error
	BroadcastConsensusMessage(*Message) error
	BroadcastBlockDataLeader(header data.HeaderHandler, blockBuff []byte, transactions [][]byte) error
	PrepareBroadcastHeaderValidator(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte)
	PrepareBroadcastBlockDataValidator(header data.HeaderHandler, transactions [][]byte, idx int, pkBytes []byte)
	IsInterfaceNil() bool
}

// P2PMessenger defines a subset of the p2p.Messenger interface
type P2PMessenger interface {
	Broadcast(topic string, buff []byte)
	IsInterfaceNil() bool
}

// HeadersPoolSubscriber can subscribe for notifications when a new block header is added to the headers pool
type HeadersPoolSubscriber interface {
	RegisterHandler(handler func(headerHandler data.HeaderHandler, headerHash []byte))
	IsInterfaceNil() bool
}

// PeerHonestyHandler defines the behaivour of a component able to handle/monitor the peer honesty of nodes which are
// participating in consensus
type PeerHonestyHandler interface {
	ChangeScore(pk string, topic string, units int)
	IsInterfaceNil() bool
}

// SubslotHandler defines the actions which should be handled by a subslot implementation
type SubslotHandler interface {
	// DoWork implements of the subslot's job
	DoWork(slotManager SlotManager) bool
	// Previous returns the ID of the previous subslot
	Previous() int
	// Next returns the ID of the next subslot
	Next() int
	// Current returns the ID of the current subslot
	Current() int
	// StartTime returns the start time, in the slotHandler time, of the current subslot
	StartTime() int64
	// EndTime returns the top limit time, in the slotHandler time, of the current subslot
	EndTime() int64
	// Name returns the name of the current slotHandler
	Name() string
	// ConsensusChannel returns the consensus channel
	ConsensusChannel() chan bool
	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool
}

// NetworkShardingCollector defines the updating methods used by the network sharding component
// The interface assures that the collected data will be used by the p2p network sharding components
type NetworkShardingCollector interface {
	UpdatePeerIDPublicKey(pid core.PeerID, pk []byte)
	GetPeerInfo(pid core.PeerID) core.P2PPeerInfo
	IsInterfaceNil() bool
}

// P2PAntifloodHandler defines the behavior of a component able to signal that the system is too busy (or flooded) processing
// p2p messages
type P2PAntifloodHandler interface {
	CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopic(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error
	ResetForTopic(topic string)
	SetMaxMessagesForTopic(topic string, maxNum uint32)
	BlacklistPeer(peer core.PeerID, reason string, duration time.Duration)
	IsInterfaceNil() bool
}

// HeaderSigVerifier encapsulates methods that are check if header rand seed, leader signature and aggregate signature are correct
type HeaderSigVerifier interface {
	VerifyRandSeed(header data.HeaderHandler) error
	VerifyLeaderSignature(header data.HeaderHandler) error
	VerifySignature(header data.HeaderHandler) error
	VerifyRandSeedAndLeaderSignature(header data.HeaderHandler) error
	IsInterfaceNil() bool
}

// FallbackHeaderValidator defines the behaviour of a component able to signal when a fallback header validation could be applied
type FallbackHeaderValidator interface {
	ShouldApplyFallbackValidation(headerHandler data.HeaderHandler) bool
	IsInterfaceNil() bool
}

// NodeRedundancyHandler provides functionality to handle the redundancy mechanism for a node
type NodeRedundancyHandler interface {
	IsRedundancyNode() bool
	IsMainMachineActive() bool
	AdjustInactivityIfNeeded(selfPubKey string, consensusPubKeys []string, slotIndex int64)
	ResetInactivityIfNeeded(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID)
	ObserverPrivateKey() crypto.PrivateKey
	SetInternalRedundancyLevel(level int64) error
	GetInternalRedundancyLevel() int64
	IsInterfaceNil() bool
}
