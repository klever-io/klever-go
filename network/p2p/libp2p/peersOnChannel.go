package libp2p

import (
	"slices"
	"sync"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

// peersOnChannel manages peers on topics
// it buffers the data and refresh the peers list continuously (in refreshInterval intervals)
type peersOnChannel struct {
	mutPeers    sync.RWMutex
	peers       map[string][]core.PeerID
	lastUpdated map[string]time.Time

	refreshInterval   time.Duration
	ttlInterval       time.Duration
	fetchPeersHandler func(topic string) []peer.ID
	getTimeHandler    func() time.Time
}

// newPeersOnChannel returns a new peersOnChannel object
func newPeersOnChannel(
	fetchPeersHandler func(topic string) []peer.ID,
	refreshInterval time.Duration,
	ttlInterval time.Duration,
) (*peersOnChannel, error) {

	if fetchPeersHandler == nil {
		return nil, p2p.ErrNilFetchPeersOnTopicHandler
	}
	if refreshInterval == 0 {
		return nil, p2p.ErrInvalidDurationProvided
	}
	if ttlInterval == 0 {
		return nil, p2p.ErrInvalidDurationProvided
	}

	poc := &peersOnChannel{
		peers:             make(map[string][]core.PeerID),
		lastUpdated:       make(map[string]time.Time),
		refreshInterval:   refreshInterval,
		ttlInterval:       ttlInterval,
		fetchPeersHandler: fetchPeersHandler,
	}
	poc.getTimeHandler = poc.clockTime

	go poc.refreshPeersOnAllKnownTopics()

	return poc, nil
}

func (poc *peersOnChannel) clockTime() time.Time {
	return time.Now()
}

// ConnectedPeersOnChannel returns a copy of the known peers on a topic
// if the list was not initialized, it will trigger a manual fetch
// the copy keeps callers from editing the cached slice in place
func (poc *peersOnChannel) ConnectedPeersOnChannel(topic string) []core.PeerID {
	poc.mutPeers.RLock()
	peers := poc.peers[topic]
	poc.mutPeers.RUnlock()

	if peers == nil {
		peers = poc.refreshPeersOnTopic(topic)
	}

	// a refresh always stores a new slice and never writes into the old one,
	// so copying after the read lock is released is safe
	return slices.Clone(peers)
}

// updateConnectedPeersOnTopic updates the connected peers on a topic and the last update timestamp
func (poc *peersOnChannel) updateConnectedPeersOnTopic(topic string, connectedPeers []core.PeerID) {
	poc.mutPeers.Lock()
	poc.peers[topic] = connectedPeers
	poc.lastUpdated[topic] = poc.getTimeHandler()
	poc.mutPeers.Unlock()
}

// refreshPeersOnAllKnownTopics iterates each topic, fetching its last timestamp
// it the timestamp + ttlInterval < time.Now, will trigger a fetch of connected peers on topic
func (poc *peersOnChannel) refreshPeersOnAllKnownTopics() {
	for {
		listTopicsToBeRefreshed := make([]string, 0)

		//build required topic list
		poc.mutPeers.RLock()
		for topic, lastRefreshed := range poc.lastUpdated {
			needsToBeRefreshed := poc.getTimeHandler().Sub(lastRefreshed) > poc.ttlInterval
			if needsToBeRefreshed {
				listTopicsToBeRefreshed = append(listTopicsToBeRefreshed, topic)
			}
		}
		poc.mutPeers.RUnlock()

		for _, topic := range listTopicsToBeRefreshed {
			_ = poc.refreshPeersOnTopic(topic)
		}

		time.Sleep(poc.refreshInterval)
	}
}

// refreshPeersOnTopic
func (poc *peersOnChannel) refreshPeersOnTopic(topic string) []core.PeerID {
	list := poc.fetchPeersHandler(topic)
	connectedPeers := make([]core.PeerID, len(list))
	for i, pid := range list {
		connectedPeers[i] = core.PeerID(pid)
	}

	poc.updateConnectedPeersOnTopic(topic, connectedPeers)
	return connectedPeers
}
