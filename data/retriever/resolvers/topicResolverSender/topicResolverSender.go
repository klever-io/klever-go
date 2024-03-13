package topicResolverSender

import (
	"bytes"
	"fmt"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/message"
	"github.com/klever-io/klever-go/tools/check"
	resolverDebug "github.com/klever-io/klever-go/tools/debug/resolver"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/random"
)

// topicRequestSuffix represents the topic name suffix
const topicRequestSuffix = "_REQUEST"

const minPeersToQuery = 2

var _ retriever.TopicResolverSender = (*topicResolverSender)(nil)
var log = logger.GetOrCreate("data/retriever/resolverstopicresolversender")

// ArgTopicResolverSender is the argument structure used to create new TopicResolverSender instance
type ArgTopicResolverSender struct {
	Messenger          retriever.MessageHandler
	TopicName          string
	PeerListCreator    retriever.PeerListCreator
	Marshalizer        marshal.Marshalizer
	Randomizer         retriever.IntRandomizer
	OutputAntiflooder  retriever.P2PAntifloodHandler
	NumIntraShardPeers int
	NumCrossShardPeers int
}

type topicResolverSender struct {
	messenger               retriever.MessageHandler
	marshalizer             marshal.Marshalizer
	topicName               string
	peerListCreator         retriever.PeerListCreator
	randomizer              retriever.IntRandomizer
	outputAntiflooder       retriever.P2PAntifloodHandler
	mutNumPeersToQuery      sync.RWMutex
	numIntraShardPeers      int
	numCrossShardPeers      int
	mutResolverDebugHandler sync.RWMutex
	resolverDebugHandler    retriever.ResolverDebugHandler
}

// NewTopicResolverSender returns a new topic resolver instance
func NewTopicResolverSender(arg ArgTopicResolverSender) (*topicResolverSender, error) {
	if check.IfNil(arg.Messenger) {
		return nil, common.ErrNilMessenger
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(arg.Randomizer) {
		return nil, common.ErrNilRandomizer
	}
	if check.IfNil(arg.PeerListCreator) {
		return nil, common.ErrNilPeerListCreator
	}
	if check.IfNil(arg.OutputAntiflooder) {
		return nil, common.ErrNilAntifloodHandler
	}
	if arg.NumIntraShardPeers < 0 {
		return nil, fmt.Errorf("%w for NumIntraShardPeers as the value should be greater or equal than 0",
			common.ErrInvalidValue)
	}
	if arg.NumCrossShardPeers < 0 {
		return nil, fmt.Errorf("%w for NumCrossShardPeers as the value should be greater or equal than 0",
			common.ErrInvalidValue)
	}
	if arg.NumCrossShardPeers+arg.NumIntraShardPeers < minPeersToQuery {
		return nil, fmt.Errorf("%w for NumCrossShardPeers, NumIntraShardPeers as their sum should be greater or equal than %d",
			common.ErrInvalidValue, minPeersToQuery)
	}

	resolver := &topicResolverSender{
		messenger:          arg.Messenger,
		topicName:          arg.TopicName,
		peerListCreator:    arg.PeerListCreator,
		marshalizer:        arg.Marshalizer,
		randomizer:         arg.Randomizer,
		outputAntiflooder:  arg.OutputAntiflooder,
		numIntraShardPeers: arg.NumIntraShardPeers,
		numCrossShardPeers: arg.NumCrossShardPeers,
	}
	resolver.resolverDebugHandler = resolverDebug.NewDisabledInterceptorResolver()

	return resolver, nil
}

// SendOnRequestTopic is used to send request data over channels (topics) to other peers
// This method only sends the request, the received data should be handled by interceptors
func (trs *topicResolverSender) SendOnRequestTopic(rd *retriever.RequestData, originalHashes [][]byte) error {
	buff, err := trs.marshalizer.Marshal(rd)
	if err != nil {
		return err
	}

	topicToSendRequest := trs.topicName + topicRequestSuffix

	// TODO: FIXME: remove cross peer?
	crossPeers := trs.peerListCreator.PeerList()
	numSentCross := trs.sendOnTopic(crossPeers, topicToSendRequest, buff, trs.numCrossShardPeers, "cross peer")

	intraPeers := trs.peerListCreator.IntraShardPeerList()
	numSentIntra := trs.sendOnTopic(intraPeers, topicToSendRequest, buff, trs.numIntraShardPeers, "intra peer")

	trs.callDebugHandler(originalHashes, numSentIntra, numSentCross)

	if numSentCross+numSentIntra == 0 {
		return fmt.Errorf("%w, topic: %s, crossPeers: %d, intraPeers: %d",
			common.ErrSendRequest,
			trs.topicName,
			len(crossPeers),
			len(intraPeers))
	}

	return nil
}

// SendOnRequestTopic is used to send request data over channels (topics) to other peers
// This method only sends the request, the received data should be handled by interceptors
func (trs *topicResolverSender) SendOnRequestTopicTo(rd *retriever.RequestData, originalHashes [][]byte, peer core.PeerID) error {
	buff, err := trs.marshalizer.Marshal(rd)
	if err != nil {
		return err
	}

	topicToSendRequest := trs.topicName + topicRequestSuffix

	numSentDirect := trs.sendOnTopic([]core.PeerID{peer}, topicToSendRequest, buff, 1, "direct peer")

	intraPeers := trs.peerListCreator.IntraShardPeerList()
	// remove origin peer
	for index := range intraPeers {
		if bytes.Equal(intraPeers[index].Bytes(), peer.Bytes()) {
			intraPeers = append(intraPeers[:index], intraPeers[index+1:]...)
			break
		}
	}

	numIntraShardPeers := trs.numIntraShardPeers
	if numSentDirect > 0 {
		// if sent to origin, remove one from max to send
		numIntraShardPeers = numIntraShardPeers - 1
	}
	numSentIntra := trs.sendOnTopic(intraPeers, topicToSendRequest, buff, numIntraShardPeers, "intra peer")

	trs.callDebugHandler(originalHashes, numSentDirect+numSentIntra, 0)

	if numSentDirect == 0 {
		return fmt.Errorf("%w, topic: %s, directPeer: %s",
			common.ErrSendRequest,
			trs.topicName,
			peer.Pretty())
	}

	return nil
}

func (trs *topicResolverSender) callDebugHandler(originalHashes [][]byte, numSentIntra int, numSentCross int) {
	trs.mutResolverDebugHandler.RLock()
	defer trs.mutResolverDebugHandler.RUnlock()

	trs.resolverDebugHandler.LogRequestedData(trs.topicName, originalHashes, numSentIntra, numSentCross)
}

func createIndexList(listLength int) []int {
	indexes := make([]int, listLength)
	for i := 0; i < listLength; i++ {
		indexes[i] = i
	}

	return indexes
}

func (trs *topicResolverSender) sendOnTopic(peerList []core.PeerID, topicToSendRequest string, buff []byte, maxToSend int, peerType string) int {
	if len(peerList) == 0 {
		return 0
	}
	if maxToSend == 0 {
		maxToSend = 1
	}

	indexes := createIndexList(len(peerList))
	shuffledIndexes := random.FisherYatesShuffle(indexes, trs.randomizer)

	logData := make([]interface{}, 0)
	msgSentCounter := 0
	for _, shuffledIndex := range shuffledIndexes {
		peer := peerList[shuffledIndex]

		err := trs.sendToConnectedPeer(topicToSendRequest, buff, peer)
		if err != nil {
			continue
		}

		logData = append(logData, peerType)
		logData = append(logData, peer.Pretty())
		msgSentCounter++
		if msgSentCounter == maxToSend {
			break
		}
	}
	log.Trace("requests are sent to", logData...)

	return msgSentCounter
}

// Send is used to send an array buffer to a connected peer
// It is used when replying to a request
func (trs *topicResolverSender) Send(buff []byte, peer core.PeerID) error {
	return trs.sendToConnectedPeer(trs.topicName, buff, peer)
}

func (trs *topicResolverSender) sendToConnectedPeer(topic string, buff []byte, peer core.PeerID) error {
	msg := &message.Message{
		DataField:  buff,
		PeerField:  peer,
		TopicField: topic,
	}

	err := trs.outputAntiflooder.CanProcessMessage(msg, peer)
	if err != nil {
		return fmt.Errorf("%w while sending %d bytes to peer %s",
			err,
			len(buff),
			p2p.PeerIDToShortString(peer),
		)
	}

	return trs.messenger.SendToConnectedPeer(topic, buff, peer)
}

// ResolverDebugHandler returns the debug handler used in resolvers
func (trs *topicResolverSender) ResolverDebugHandler() retriever.ResolverDebugHandler {
	trs.mutResolverDebugHandler.RLock()
	defer trs.mutResolverDebugHandler.RUnlock()

	return trs.resolverDebugHandler
}

// SetResolverDebugHandler sets the debug handler used in resolvers
func (trs *topicResolverSender) SetResolverDebugHandler(handler retriever.ResolverDebugHandler) error {
	if check.IfNil(handler) {
		return common.ErrNilResolverDebugHandler
	}

	trs.mutResolverDebugHandler.Lock()
	trs.resolverDebugHandler = handler
	trs.mutResolverDebugHandler.Unlock()

	return nil
}

// RequestTopic returns the topic with the request suffix used for sending requests
func (trs *topicResolverSender) RequestTopic() string {
	return trs.topicName + topicRequestSuffix
}

// SetNumPeersToQuery will set the number of intra shard and cross shard number of peers to query
func (trs *topicResolverSender) SetNumPeersToQuery(intra int, cross int) {
	trs.mutNumPeersToQuery.Lock()
	trs.numIntraShardPeers = intra
	trs.numCrossShardPeers = cross
	trs.mutNumPeersToQuery.Unlock()
}

// NumPeersToQuery will return the number of intra shard and cross shard number of peer to query
func (trs *topicResolverSender) NumPeersToQuery() (int, int) {
	trs.mutNumPeersToQuery.RLock()
	defer trs.mutNumPeersToQuery.RUnlock()

	return trs.numIntraShardPeers, trs.numCrossShardPeers
}

// IsInterfaceNil returns true if there is no value under the interface
func (trs *topicResolverSender) IsInterfaceNil() bool {
	return trs == nil
}
