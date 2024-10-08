package resolverscontainer

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/resolvers"
	"github.com/klever-io/klever-go/data/retriever/resolvers/topicResolverSender"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

const numCommonPeers = 2
const numConsensusPeers = 1

type baseResolversContainerFactory struct {
	container                retriever.ResolversContainer
	messenger                retriever.TopicMessageHandler
	store                    retriever.StorageService
	marshalizer              marshal.Marshalizer
	dataPools                retriever.PoolsHolder
	uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	intRandomizer            retriever.IntRandomizer
	dataPacker               retriever.DataPacker
	triesContainer           state.TriesHolder
	inputAntifloodHandler    process.P2PAntifloodHandler
	outputAntifloodHandler   process.P2PAntifloodHandler
	throttler                retriever.ResolverThrottler
	consensusTopic           string
}

func (brcf *baseResolversContainerFactory) checkParams() error {
	if check.IfNil(brcf.messenger) {
		return common.ErrNilMessenger
	}
	if check.IfNil(brcf.store) {
		return common.ErrNilStore
	}
	if check.IfNil(brcf.marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(brcf.dataPools) {
		return common.ErrNilDataPoolHolder
	}
	if check.IfNil(brcf.uint64ByteSliceConverter) {
		return common.ErrNilUint64ByteSliceConverter
	}
	if check.IfNil(brcf.dataPacker) {
		return common.ErrNilDataPacker
	}
	if check.IfNil(brcf.triesContainer) {
		return common.ErrNilTrieDataGetter
	}
	if check.IfNil(brcf.inputAntifloodHandler) {
		return fmt.Errorf("%w for input", common.ErrNilAntifloodHandler)
	}
	if check.IfNil(brcf.outputAntifloodHandler) {
		return fmt.Errorf("%w for output", common.ErrNilAntifloodHandler)
	}
	if check.IfNil(brcf.throttler) {
		return common.ErrNilThrottler
	}

	return nil
}

func (brcf *baseResolversContainerFactory) generateTxResolvers(
	topic string,
	unit retriever.UnitType,
	dataPool retriever.ShardedDataCacherNotifier,
) error {

	identifierTx := topic

	resolver, err := brcf.createTxResolver(identifierTx, unit, dataPool)
	if err != nil {
		return err
	}

	return brcf.container.Add(identifierTx, resolver)
}

func (brcf *baseResolversContainerFactory) createTxResolver(
	topic string,
	unit retriever.UnitType,
	dataPool retriever.ShardedDataCacherNotifier,
) (retriever.Resolver, error) {

	txStorer := brcf.store.GetStorer(unit)

	resolverSender, err := brcf.createOneResolverSender(topic)
	if err != nil {
		return nil, err
	}

	arg := resolvers.ArgTxResolver{
		SenderResolver:   resolverSender,
		TxPool:           dataPool,
		TxStorage:        txStorer,
		Marshalizer:      brcf.marshalizer,
		DataPacker:       brcf.dataPacker,
		AntifloodHandler: brcf.inputAntifloodHandler,
		Throttler:        brcf.throttler,
	}
	resolver, err := resolvers.NewTxResolver(arg)
	if err != nil {
		return nil, err
	}

	err = brcf.messenger.RegisterMessageProcessor(resolver.RequestTopic(), resolver)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}

func (brcf *baseResolversContainerFactory) createOneResolverSender(
	topic string,
) (retriever.TopicResolverSender, error) {
	return brcf.createOneResolverSenderWithSpecifiedNumRequests(topic, numCommonPeers, numConsensusPeers)
}

func (brcf *baseResolversContainerFactory) createOneResolverSenderWithSpecifiedNumRequests(
	topic string,
	numCommon int,
	numConsensus int,
) (retriever.TopicResolverSender, error) {

	peerListCreator, err := topicResolverSender.NewPeerListCreator(brcf.messenger, topic, brcf.consensusTopic)
	if err != nil {
		return nil, err
	}

	arg := topicResolverSender.ArgTopicResolverSender{
		Messenger:         brcf.messenger,
		TopicName:         topic,
		PeerListCreator:   peerListCreator,
		Marshalizer:       brcf.marshalizer,
		Randomizer:        brcf.intRandomizer,
		OutputAntiflooder: brcf.outputAntifloodHandler,
		NumCommonPeers:    numCommon,
		NumConsensusPeers: numConsensus,
	}

	resolverSender, err := topicResolverSender.NewTopicResolverSender(arg)
	if err != nil {
		return nil, err
	}

	return resolverSender, nil
}

func (brcf *baseResolversContainerFactory) createTrieNodesResolver(
	topic string,
	trieID string,
	numCommon int,
	numConsensus int,
) (retriever.Resolver, error) {
	resolverSender, err := brcf.createOneResolverSenderWithSpecifiedNumRequests(
		topic,
		numCommon,
		numConsensus,
	)
	if err != nil {
		return nil, err
	}

	trie := brcf.triesContainer.Get([]byte(trieID))
	argTrie := resolvers.ArgTrieNodeResolver{
		SenderResolver:   resolverSender,
		TrieDataGetter:   trie,
		Marshalizer:      brcf.marshalizer,
		AntifloodHandler: brcf.inputAntifloodHandler,
		Throttler:        brcf.throttler,
	}
	resolver, err := resolvers.NewTrieNodeResolver(argTrie)
	if err != nil {
		return nil, err
	}

	err = brcf.messenger.RegisterMessageProcessor(resolver.RequestTopic(), resolver)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}
