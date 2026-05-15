package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
)

// RedundancyHandlerStub -
type RedundancyHandlerStub struct {
	IsRedundancyNodeCalled     func() bool
	IsMainMachineActiveCalled  func() bool
	ObserverPrivateKeyCalled   func() crypto.PrivateKey
	GetSlotsOfInactivityCalled func() uint64
}

func (rhs *RedundancyHandlerStub) AdjustInactivityIfNeeded(selfPubKey string, consensusPubKeys []string, slotIndex int64) {
}

func (rhs *RedundancyHandlerStub) ResetInactivityIfNeeded(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID) {
}

func (rhs *RedundancyHandlerStub) SetInternalRedundancyLevel(level int64) error {
	return nil
}

func (rhs *RedundancyHandlerStub) GetInternalRedundancyLevel() int64 {
	return 0
}

// IsRedundancyNode -
func (rhs *RedundancyHandlerStub) IsRedundancyNode() bool {
	if rhs.IsRedundancyNodeCalled != nil {
		return rhs.IsRedundancyNodeCalled()
	}

	return false
}

// IsMainMachineActive -
func (rhs *RedundancyHandlerStub) IsMainMachineActive() bool {
	if rhs.IsMainMachineActiveCalled != nil {
		return rhs.IsMainMachineActiveCalled()
	}

	return true
}

// ObserverPrivateKey -
func (rhs *RedundancyHandlerStub) ObserverPrivateKey() crypto.PrivateKey {
	if rhs.ObserverPrivateKeyCalled != nil {
		return rhs.ObserverPrivateKeyCalled()
	}

	return &PrivateKeyStub{}
}

// GetSlotsOfInactivity -
func (rhs *RedundancyHandlerStub) GetSlotsOfInactivity() uint64 {
	if rhs.GetSlotsOfInactivityCalled != nil {
		return rhs.GetSlotsOfInactivityCalled()
	}
	return 0
}

// Snapshot -
func (rhs *RedundancyHandlerStub) Snapshot() (int64, uint64, bool) {
	return rhs.GetInternalRedundancyLevel(), rhs.GetSlotsOfInactivity(), rhs.IsMainMachineActive()
}

// IsInterfaceNil -
func (rhs *RedundancyHandlerStub) IsInterfaceNil() bool {
	return rhs == nil
}
