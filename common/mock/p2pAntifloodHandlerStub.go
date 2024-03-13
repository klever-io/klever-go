package mock

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
)

// P2PAntifloodHandlerStub -
type P2PAntifloodHandlerStub struct {
	CanProcessMessageCalled            func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopicCalled    func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error
	ApplyConsensusSizeCalled           func(size int)
	SetDebuggerCalled                  func(debugger process.AntifloodDebugger) error
	BlacklistPeerCalled                func(peer core.PeerID, reason string, duration time.Duration)
	IsOriginatorEligibleForTopicCalled func(pid core.PeerID, topic string) error
	IsOriginatorElectedForTopicCalled  func(pid core.PeerID, topic string) error
	ResetForTopicCalled                func(topic string)
	SetMaxMessagesForTopicCalled       func(topic string, num uint32)
}

// CanProcessMessage -
func (p2pahs *P2PAntifloodHandlerStub) CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if p2pahs.CanProcessMessageCalled == nil {
		return nil
	}

	return p2pahs.CanProcessMessageCalled(message, fromConnectedPeer)
}

// CanProcessMessagesOnTopic -
func (p2pahs *P2PAntifloodHandlerStub) CanProcessMessagesOnTopic(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
	if p2pahs.CanProcessMessagesOnTopicCalled == nil {
		return nil
	}

	return p2pahs.CanProcessMessagesOnTopicCalled(peer, topic, numMessages, totalSize, sequence)
}

// ApplyConsensusSize -
func (p2pahs *P2PAntifloodHandlerStub) ApplyConsensusSize(size int) {
	if p2pahs.ApplyConsensusSizeCalled != nil {
		p2pahs.ApplyConsensusSizeCalled(size)
	}
}

// SetDebugger -
func (p2pahs *P2PAntifloodHandlerStub) SetDebugger(debugger process.AntifloodDebugger) error {
	if p2pahs.SetDebuggerCalled != nil {
		return p2pahs.SetDebuggerCalled(debugger)
	}

	return nil
}

// IsOriginatorEligibleForTopic -
func (p2pahs *P2PAntifloodHandlerStub) IsOriginatorEligibleForTopic(pid core.PeerID, topic string) error {
	if p2pahs.IsOriginatorEligibleForTopicCalled != nil {
		return p2pahs.IsOriginatorEligibleForTopicCalled(pid, topic)
	}
	return nil
}

// IsOriginatorElectedForTopic -
func (p2pahs *P2PAntifloodHandlerStub) IsOriginatorElectedForTopic(pid core.PeerID, topic string) error {
	if p2pahs.IsOriginatorElectedForTopicCalled != nil {
		return p2pahs.IsOriginatorElectedForTopicCalled(pid, topic)
	}
	return nil
}

// BlacklistPeer -
func (p2pahs *P2PAntifloodHandlerStub) BlacklistPeer(peer core.PeerID, reason string, duration time.Duration) {
	if p2pahs.BlacklistPeerCalled != nil {
		p2pahs.BlacklistPeerCalled(peer, reason, duration)
	}
}

// IsInterfaceNil -
func (p2pahs *P2PAntifloodHandlerStub) IsInterfaceNil() bool {
	return p2pahs == nil
}

// ResetForTopic -
func (p2pahs *P2PAntifloodHandlerStub) ResetForTopic(topic string) {
	if p2pahs.ResetForTopicCalled != nil {
		p2pahs.ResetForTopicCalled(topic)
	}
}

// SetMaxMessagesForTopic -
func (p2pahs *P2PAntifloodHandlerStub) SetMaxMessagesForTopic(topic string, num uint32) {
	if p2pahs.SetMaxMessagesForTopicCalled != nil {
		p2pahs.SetMaxMessagesForTopicCalled(topic, num)
	}
}
