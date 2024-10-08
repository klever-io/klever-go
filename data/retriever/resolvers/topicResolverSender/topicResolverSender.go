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
	Messenger         retriever.MessageHandler
	TopicName         string
	PeerListCreator   retriever.PeerListCreator
	Marshalizer       marshal.Marshalizer
	Randomizer        retriever.IntRandomizer
	OutputAntiflooder retriever.P2PAntifloodHandler
	NumConsensusPeers int
	NumCommonPeers    int
}

type topicResolverSender struct {
	messenger               retriever.MessageHandler
	marshalizer             marshal.Marshalizer
	topicName               string
	peerListCreator         retriever.PeerListCreator
	randomizer              retriever.IntRandomizer
	outputAntiflooder       retriever.P2PAntifloodHandler
	mutNumPeersToQuery      sync.RWMutex
	numConsensusPeers       int
	numCommonPeers          int
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
	if arg.NumConsensusPeers < 0 {
		return nil, fmt.Errorf("%w for NumConsensusPeers as the value should be greater or equal than 0",
			common.ErrInvalidValue)
	}
	if arg.NumCommonPeers < 0 {
		return nil, fmt.Errorf("%w for NumCommonPeers as the value should be greater or equal than 0",
			common.ErrInvalidValue)
	}
	if arg.NumCommonPeers+arg.NumConsensusPeers < minPeersToQuery {
		return nil, fmt.Errorf("%w for NumCommonPeers, NumConsensusPeers as their sum should be greater or equal than %d",
			common.ErrInvalidValue, minPeersToQuery)
	}

	resolver := &topicResolverSender{
		messenger:         arg.Messenger,
		topicName:         arg.TopicName,
		peerListCreator:   arg.PeerListCreator,
		marshalizer:       arg.Marshalizer,
		randomizer:        arg.Randomizer,
		outputAntiflooder: arg.OutputAntiflooder,
		numConsensusPeers: arg.NumConsensusPeers,
		numCommonPeers:    arg.NumCommonPeers,
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

	commonPeers := trs.peerListCreator.PeerList()
	numSentCommon := trs.sendOnTopic(commonPeers, topicToSendRequest, buff, trs.numCommonPeers, "common peer")

	consensusPeers := trs.peerListCreator.ConsensusPeerList()
	numSentConsensus := trs.sendOnTopic(consensusPeers, topicToSendRequest, buff, trs.numConsensusPeers, "consensus peer")

	trs.callDebugHandler(originalHashes, numSentConsensus, numSentCommon)

	if numSentCommon+numSentConsensus == 0 {
		return fmt.Errorf("%w, topic: %s, commonPeers: %d, consensusPeers: %d",
			common.ErrSendRequest,
			trs.topicName,
			len(commonPeers),
			len(consensusPeers))
	}

	return nil
}

// SendOnRequestTopicTo is used to send request data over channels (topics) to other peers
// This method only sends the request, the received data should be handled by interceptors
func (trs *topicResolverSender) SendOnRequestTopicTo(rd *retriever.RequestData, originalHashes [][]byte, peer core.PeerID) error {
	buff, err := trs.marshalizer.Marshal(rd)
	if err != nil {
		return err
	}

	topicToSendRequest := trs.topicName + topicRequestSuffix

	numSentDirect := trs.sendOnTopic([]core.PeerID{peer}, topicToSendRequest, buff, 1, "direct peer")

	consensusPeers := trs.peerListCreator.ConsensusPeerList()
	// remove origin peer
	for index := range consensusPeers {
		if bytes.Equal(consensusPeers[index].Bytes(), peer.Bytes()) {
			consensusPeers = append(consensusPeers[:index], consensusPeers[index+1:]...)
			break
		}
	}

	numConsensusPeers := trs.numConsensusPeers
	if numSentDirect > 0 {
		// if sent to origin, remove one from max to send
		numConsensusPeers = numConsensusPeers - 1
	}
	numSentConsensus := trs.sendOnTopic(consensusPeers, topicToSendRequest, buff, numConsensusPeers, "consensus peer")

	trs.callDebugHandler(originalHashes, numSentDirect+numSentConsensus, 0)

	if numSentDirect == 0 {
		return fmt.Errorf("%w, topic: %s, directPeer: %s",
			common.ErrSendRequest,
			trs.topicName,
			peer.Pretty())
	}

	return nil
}

func (trs *topicResolverSender) callDebugHandler(originalHashes [][]byte, numSentConsensus int, numSentCommon int) {
	trs.mutResolverDebugHandler.RLock()
	defer trs.mutResolverDebugHandler.RUnlock()

	trs.resolverDebugHandler.LogRequestedData(trs.topicName, originalHashes, numSentConsensus, numSentCommon)
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

// SetNumPeersToQuery will set the number of consensus and common number of peers to query
func (trs *topicResolverSender) SetNumPeersToQuery(consensus int, common int) {
	trs.mutNumPeersToQuery.Lock()
	trs.numConsensusPeers = consensus
	trs.numCommonPeers = common
	trs.mutNumPeersToQuery.Unlock()
}

// NumPeersToQuery will return the number of consensus and common number of peer to query
func (trs *topicResolverSender) NumPeersToQuery() (int, int) {
	trs.mutNumPeersToQuery.RLock()
	defer trs.mutNumPeersToQuery.RUnlock()

	return trs.numConsensusPeers, trs.numCommonPeers
}

// IsInterfaceNil returns true if there is no value under the interface
func (trs *topicResolverSender) IsInterfaceNil() bool {
	return trs == nil
}
