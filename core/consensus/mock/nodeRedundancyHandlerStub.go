package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
)

// NodeRedundancyHandlerStub -
type NodeRedundancyHandlerStub struct {
	IsRedundancyNodeCalled         func() bool
	IsMainMachineActiveCalled      func() bool
	AdjustInactivityIfNeededCalled func(selfPubKey string, consensusPubKeys []string, roundIndex int64)
	ResetInactivityIfNeededCalled  func(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID)
	ObserverPrivateKeyCalled       func() crypto.PrivateKey
	GetSlotsOfInactivityCalled     func() uint64
}

// IsRedundancyNode -
func (nrhs *NodeRedundancyHandlerStub) IsRedundancyNode() bool {
	if nrhs.IsRedundancyNodeCalled != nil {
		return nrhs.IsRedundancyNodeCalled()
	}
	return false
}

// IsMainMachineActive -
func (nrhs *NodeRedundancyHandlerStub) IsMainMachineActive() bool {
	if nrhs.IsMainMachineActiveCalled != nil {
		return nrhs.IsMainMachineActiveCalled()
	}
	return true
}

// AdjustInactivityIfNeeded -
func (nrhs *NodeRedundancyHandlerStub) AdjustInactivityIfNeeded(selfPubKey string, consensusPubKeys []string, roundIndex int64) {
	if nrhs.AdjustInactivityIfNeededCalled != nil {
		nrhs.AdjustInactivityIfNeededCalled(selfPubKey, consensusPubKeys, roundIndex)
	}
}

// ResetInactivityIfNeeded -
func (nrhs *NodeRedundancyHandlerStub) ResetInactivityIfNeeded(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID) {
	if nrhs.ResetInactivityIfNeededCalled != nil {
		nrhs.ResetInactivityIfNeededCalled(selfPubKey, consensusMsgPubKey, consensusMsgPeerID)
	}
}

// SetInternalRedundancyLevel -
func (nrhs *NodeRedundancyHandlerStub) SetInternalRedundancyLevel(level int64) error {
	return nil
}

func (nrhs *NodeRedundancyHandlerStub) GetInternalRedundancyLevel() int64 {
	return 0
}

// ObserverPrivateKey -
func (nrhs *NodeRedundancyHandlerStub) ObserverPrivateKey() crypto.PrivateKey {
	if nrhs.ObserverPrivateKeyCalled != nil {
		return nrhs.ObserverPrivateKeyCalled()
	}

	return &cryptoMock.PrivateKeyMock{}
}

// GetSlotsOfInactivity -
func (nrhs *NodeRedundancyHandlerStub) GetSlotsOfInactivity() uint64 {
	if nrhs.GetSlotsOfInactivityCalled != nil {
		return nrhs.GetSlotsOfInactivityCalled()
	}
	return 0
}

// IsInterfaceNil -
func (nrhs *NodeRedundancyHandlerStub) IsInterfaceNil() bool {
	return nrhs == nil
}
