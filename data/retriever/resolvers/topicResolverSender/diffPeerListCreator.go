package topicResolverSender

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
)

var _ retriever.PeerListCreator = (*PeerListCreator)(nil)

// PeerListCreator can create a peer list by making the set difference between peers on
// main topic and the exclusion topic. If the resulting list is empty, will return the peers on the main topic.
type PeerListCreator struct {
	messenger      retriever.MessageHandler
	mainTopic      string
	consensusTopic string
}

// NewPeerListCreator is the constructor for PeerListCreator
func NewPeerListCreator(
	messenger retriever.MessageHandler,
	mainTopic string,
	consensusTopic string,
) (*PeerListCreator, error) {
	if check.IfNil(messenger) {
		return nil, common.ErrNilMessenger
	}
	if len(mainTopic) == 0 {
		return nil, fmt.Errorf("%w for mainTopic", common.ErrEmptyString)
	}
	if len(consensusTopic) == 0 {
		return nil, fmt.Errorf("%w for consensusTopic", common.ErrEmptyString)
	}

	return &PeerListCreator{
		messenger:      messenger,
		mainTopic:      mainTopic,
		consensusTopic: consensusTopic,
	}, nil
}

// PeerList will return the common list of peers
func (dplc *PeerListCreator) PeerList() []core.PeerID {
	return dplc.messenger.ConnectedPeersOnTopic(dplc.mainTopic)
}

// ConsensusPeerList returns the consensus peer list
func (dplc *PeerListCreator) ConsensusPeerList() []core.PeerID {
	return dplc.messenger.ConnectedPeersOnTopic(dplc.consensusTopic)
}

// IsInterfaceNil returns true if there is no value under the interface
func (dplc *PeerListCreator) IsInterfaceNil() bool {
	return dplc == nil
}
