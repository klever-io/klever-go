package topicResolverSender

import (
	"bytes"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
)

var _ retriever.PeerListCreator = (*DiffPeerListCreator)(nil)

// DiffPeerListCreator can create a peer list by making the set difference between peers on
// main topic and the exclusion topic. If the resulting list is empty, will return the peers on the main topic.
type DiffPeerListCreator struct {
	messenger             retriever.MessageHandler
	mainTopic             string
	intraShardTopic       string
	excludePeersFromTopic string
}

// NewDiffPeerListCreator is the constructor for DiffPeerListCreator
func NewDiffPeerListCreator(
	messenger retriever.MessageHandler,
	mainTopic string,
	intraShardTopic string,
	excludePeersFromTopic string,
) (*DiffPeerListCreator, error) {
	if check.IfNil(messenger) {
		return nil, common.ErrNilMessenger
	}
	if len(mainTopic) == 0 {
		return nil, fmt.Errorf("%w for mainTopic", common.ErrEmptyString)
	}
	if len(intraShardTopic) == 0 {
		return nil, fmt.Errorf("%w for intraShardTopic", common.ErrEmptyString)
	}

	return &DiffPeerListCreator{
		messenger:             messenger,
		mainTopic:             mainTopic,
		intraShardTopic:       intraShardTopic,
		excludePeersFromTopic: excludePeersFromTopic,
	}, nil
}

// PeerList will return the generated list of peers
func (dplc *DiffPeerListCreator) PeerList() []core.PeerID {
	allConnectedPeers := dplc.messenger.ConnectedPeersOnTopic(dplc.mainTopic)
	mainTopicHasPeers := len(allConnectedPeers) != 0
	if !mainTopicHasPeers {
		return allConnectedPeers
	}

	excludedConnectedPeers := make([]core.PeerID, 0)
	isExcludedTopicSet := len(dplc.excludePeersFromTopic) > 0
	if isExcludedTopicSet {
		excludedConnectedPeers = dplc.messenger.ConnectedPeersOnTopic(dplc.excludePeersFromTopic)
	}

	diffList := makeDiffList(allConnectedPeers, excludedConnectedPeers)
	if len(diffList) == 0 {
		//no differences: fallback to all connected peers
		diffList = allConnectedPeers
	}

	return diffList
}

// IntraShardPeerList returns the intra shard peer list
func (dplc *DiffPeerListCreator) IntraShardPeerList() []core.PeerID {
	return dplc.messenger.ConnectedPeersOnTopic(dplc.intraShardTopic)
}

// IsInterfaceNil returns true if there is no value under the interface
func (dplc *DiffPeerListCreator) IsInterfaceNil() bool {
	return dplc == nil
}

func makeDiffList(
	allConnectedPeers []core.PeerID,
	excludedConnectedPeers []core.PeerID,
) []core.PeerID {

	if len(excludedConnectedPeers) == 0 {
		return allConnectedPeers
	}

	diff := make([]core.PeerID, 0)
	for _, pid := range allConnectedPeers {
		isPeerExcluded := false

		for _, excluded := range excludedConnectedPeers {
			if bytes.Equal(pid.Bytes(), excluded.Bytes()) {
				isPeerExcluded = true
				break
			}
		}

		if !isPeerExcluded {
			diff = append(diff, pid)
		}
	}

	return diff
}
