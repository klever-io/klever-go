package redundancy

import (
	"math"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("redundancy")

// maxSlotsOfInactivityAccepted defines the maximum slots of inactivity accepted, after which the main or lower
// level redundancy machines will be considered inactive
const maxSlotsOfInactivityAccepted = 5

type nodeRedundancy struct {
	redundancyLevel    int64
	lastSlotIndexCheck int64
	slotsOfInactivity  uint64
	mutNodeRedundancy  sync.RWMutex
	messenger          P2PMessenger
	observerPrivateKey crypto.PrivateKey
}

// ArgNodeRedundancy represents the DTO structure used by the nodeRedundancy's constructor
type ArgNodeRedundancy struct {
	RedundancyLevel    int64
	Messenger          P2PMessenger
	ObserverPrivateKey crypto.PrivateKey
}

// NewNodeRedundancy creates a node redundancy object which implements NodeRedundancyHandler interface
func NewNodeRedundancy(arg ArgNodeRedundancy) (*nodeRedundancy, error) {
	if check.IfNil(arg.Messenger) {
		return nil, ErrNilMessenger
	}
	if check.IfNil(arg.ObserverPrivateKey) {
		return nil, ErrNilObserverPrivateKey
	}

	nr := &nodeRedundancy{
		redundancyLevel:    arg.RedundancyLevel,
		messenger:          arg.Messenger,
		observerPrivateKey: arg.ObserverPrivateKey,
	}

	return nr, nil
}

// IsRedundancyNode returns true if the current instance is used as a redundancy node
func (nr *nodeRedundancy) IsRedundancyNode() bool {
	return nr.redundancyLevel != 0
}

// IsMainMachineActive returns true if the main or lower level redundancy machines are active
func (nr *nodeRedundancy) IsMainMachineActive() bool {
	nr.mutNodeRedundancy.RLock()
	defer nr.mutNodeRedundancy.RUnlock()

	return nr.isMainMachineActive()
}

// AdjustInactivityIfNeeded increments slots of inactivity for main or lower level redundancy machines if needed
func (nr *nodeRedundancy) AdjustInactivityIfNeeded(selfPubKey string, consensusPubKeys []string, slotIndex int64) {
	nr.mutNodeRedundancy.Lock()
	defer nr.mutNodeRedundancy.Unlock()

	if slotIndex <= nr.lastSlotIndexCheck {
		return
	}

	if nr.isMainMachineActive() {
		log.Debug("main or lower level redundancy machines are active", "node redundancy level", nr.redundancyLevel)
	} else {
		log.Warn("main or lower level redundancy machines are inactive", "node redundancy level", nr.redundancyLevel)
	}

	log.Debug("slots of inactivity for main or lower level redundancy machines",
		"num", nr.slotsOfInactivity)

	for _, pubKey := range consensusPubKeys {
		if pubKey == selfPubKey {
			nr.slotsOfInactivity++
			break
		}
	}

	nr.lastSlotIndexCheck = slotIndex
}

// ResetInactivityIfNeeded resets slots of inactivity for main or lower level redundancy machines if needed
func (nr *nodeRedundancy) ResetInactivityIfNeeded(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID) {
	if selfPubKey != consensusMsgPubKey {
		return
	}
	if consensusMsgPeerID == nr.messenger.ID() {
		return
	}

	nr.mutNodeRedundancy.Lock()
	nr.slotsOfInactivity = 0
	nr.mutNodeRedundancy.Unlock()
}

// SetInternalRedundancyLevel set forced internal redundancy level
func (nr *nodeRedundancy) SetInternalRedundancyLevel(level int64) error {
	nr.mutNodeRedundancy.Lock()
	nr.slotsOfInactivity = uint64(level * maxSlotsOfInactivityAccepted) // #nosec G115
	nr.mutNodeRedundancy.Unlock()

	log.Info("node redundancy forced", "level", level, "IsMainMachineActive", nr.IsMainMachineActive())

	return nil
}

func (nr *nodeRedundancy) GetInternalRedundancyLevel() int64 {
	return nr.redundancyLevel
}

// GetSlotsOfInactivity returns the current slots of inactivity counter
func (nr *nodeRedundancy) GetSlotsOfInactivity() uint64 {
	nr.mutNodeRedundancy.RLock()
	defer nr.mutNodeRedundancy.RUnlock()

	return nr.slotsOfInactivity
}

func (nr *nodeRedundancy) isMainMachineActive() bool {
	if nr.redundancyLevel < 0 {
		return true
	}

	if nr.slotsOfInactivity > math.MaxInt64 {
		return false
	}

	// #nosec G115
	return int64(nr.slotsOfInactivity) < maxSlotsOfInactivityAccepted*nr.redundancyLevel
}

// ObserverPrivateKey returns the stored private key by this instance. This key will be used whenever a new key,
// different from the main key is required. Example: sending anonymous heartbeat messages while the node is in backup mode.
func (nr *nodeRedundancy) ObserverPrivateKey() crypto.PrivateKey {
	return nr.observerPrivateKey
}

// IsInterfaceNil returns true if there is no value under the interface
func (nr *nodeRedundancy) IsInterfaceNil() bool {
	return nr == nil
}
