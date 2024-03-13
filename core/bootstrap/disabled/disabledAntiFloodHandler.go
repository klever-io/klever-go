package disabled

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/network/p2p"
)

var _ retriever.P2PAntifloodHandler = (*antiFloodHandler)(nil)

type antiFloodHandler struct {
}

// NewAntiFloodHandler returns a new instance of antiFloodHandler
func NewAntiFloodHandler() *antiFloodHandler {
	return &antiFloodHandler{}
}

// CanProcessMessage returns nil regardless of the input
func (a *antiFloodHandler) CanProcessMessage(_ p2p.MessageP2P, _ core.PeerID) error {
	return nil
}

func (a *antiFloodHandler) ResetForTopic(_ string) {
}

// CanProcessMessagesOnTopic returns nil regardless of the input
func (a *antiFloodHandler) CanProcessMessagesOnTopic(_ core.PeerID, _ string, _ uint32, _ uint64, _ []byte) error {
	return nil
}

func (a *antiFloodHandler) SetMaxMessagesForTopic(_ string, _ uint32) {

}

// ApplyConsensusSize does nothing
func (a *antiFloodHandler) ApplyConsensusSize(_ int) {
}

// SetDebugger returns nil
func (a *antiFloodHandler) SetDebugger(_ process.AntifloodDebugger) error {
	return nil
}

// BlacklistPeer does nothing
func (a *antiFloodHandler) BlacklistPeer(_ core.PeerID, _ string, _ time.Duration) {
}

// IsOriginatorElectedForTopic returns nil
func (a *antiFloodHandler) IsOriginatorElectedForTopic(_ core.PeerID, _ string) error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (a *antiFloodHandler) IsInterfaceNil() bool {
	return a == nil
}
