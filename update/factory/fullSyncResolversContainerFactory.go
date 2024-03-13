package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/data/retriever"
	factoryDataRetriever "github.com/klever-io/klever-go/data/retriever/factory/resolverscontainer"
	"github.com/klever-io/klever-go/data/retriever/resolvers"
	"github.com/klever-io/klever-go/data/retriever/resolvers/topicResolverSender"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/random"
	"github.com/klever-io/klever-go/update/genesis"
)

const numCrossShardPeers = 1
const numIntraShardPeers = 2

type resolversContainerFactory struct {
	messenger              retriever.TopicMessageHandler
	marshalizer            marshal.Marshalizer
	intRandomizer          retriever.IntRandomizer
	dataTrieContainer      state.TriesHolder
	container              retriever.ResolversContainer
	inputAntifloodHandler  retriever.P2PAntifloodHandler
	outputAntifloodHandler retriever.P2PAntifloodHandler
	throttler              retriever.ResolverThrottler
}

// ArgsNewResolversContainerFactory defines the arguments for the resolversContainerFactory constructor
type ArgsNewResolversContainerFactory struct {
	Messenger                  retriever.TopicMessageHandler
	Marshalizer                marshal.Marshalizer
	DataTrieContainer          state.TriesHolder
	ExistingResolvers          retriever.ResolversContainer
	InputAntifloodHandler      retriever.P2PAntifloodHandler
	OutputAntifloodHandler     retriever.P2PAntifloodHandler
	NumConcurrentResolvingJobs int32
}

// NewResolversContainerFactory creates a new container filled with topic resolvers
func NewResolversContainerFactory(args ArgsNewResolversContainerFactory) (*resolversContainerFactory, error) {
	if check.IfNil(args.Messenger) {
		return nil, common.ErrNilMessenger
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.DataTrieContainer) {
		return nil, common.ErrNilTrieDataGetter
	}
	if check.IfNil(args.ExistingResolvers) {
		return nil, common.ErrNilResolverContainer
	}

	thr, err := throttler.NewNumGoRoutinesThrottler(args.NumConcurrentResolvingJobs)
	if err != nil {
		return nil, err
	}
	return &resolversContainerFactory{
		messenger:              args.Messenger,
		marshalizer:            args.Marshalizer,
		intRandomizer:          &random.ConcurrentSafeIntRandomizer{},
		dataTrieContainer:      args.DataTrieContainer,
		container:              args.ExistingResolvers,
		inputAntifloodHandler:  args.InputAntifloodHandler,
		outputAntifloodHandler: args.OutputAntifloodHandler,
		throttler:              thr,
	}, nil
}

// Create returns a resolver container that will hold all resolvers in the system
func (rcf *resolversContainerFactory) Create() (retriever.ResolversContainer, error) {
	err := rcf.generateTrieNodesResolvers()
	if err != nil {
		return nil, err
	}

	return rcf.container, nil
}

func (rcf *resolversContainerFactory) generateTrieNodesResolvers() error {

	keys := make([]string, 0)
	resolversSlice := make([]retriever.Resolver, 0)

	identifierTrieNodes := common.AccountTrieNodesTopic
	if rcf.checkIfResolverExists(identifierTrieNodes) {
		return nil
	}

	if !rcf.checkIfResolverExists(identifierTrieNodes) {
		trieId := genesis.CreateTrieIdentifier(genesis.UserAccount)
		resolver, err := rcf.createTrieNodesResolver(identifierTrieNodes, trieId)
		if err != nil {
			return err
		}

		resolversSlice = append(resolversSlice, resolver)
		keys = append(keys, identifierTrieNodes)
	}

	identifierTrieNodes = common.ValidatorTrieNodesTopic
	if !rcf.checkIfResolverExists(identifierTrieNodes) {
		trieID := genesis.CreateTrieIdentifier(genesis.ValidatorAccount)
		resolver, err := rcf.createTrieNodesResolver(identifierTrieNodes, trieID)
		if err != nil {
			return err
		}

		resolversSlice = append(resolversSlice, resolver)
		keys = append(keys, identifierTrieNodes)
	}

	return rcf.container.AddMultiple(keys, resolversSlice)
}

func (rcf *resolversContainerFactory) checkIfResolverExists(topic string) bool {
	_, err := rcf.container.Get(topic)
	return err == nil
}

func (rcf *resolversContainerFactory) createTrieNodesResolver(baseTopic string, trieId string) (retriever.Resolver, error) {
	//for each resolver we create a pseudo-intra shard topic as to make at least of half of the requests target the proper peers
	//this pseudo-intra shard topic is the consensus_targetShardID

	targetConsensusStopic := common.ConsensusTopic
	peerListCreator, err := topicResolverSender.NewDiffPeerListCreator(
		rcf.messenger,
		baseTopic,
		targetConsensusStopic,
		factoryDataRetriever.EmptyExcludePeersOnTopic,
	)
	if err != nil {
		return nil, err
	}

	arg := topicResolverSender.ArgTopicResolverSender{
		Messenger:          rcf.messenger,
		TopicName:          baseTopic,
		PeerListCreator:    peerListCreator,
		Marshalizer:        rcf.marshalizer,
		Randomizer:         rcf.intRandomizer,
		OutputAntiflooder:  rcf.outputAntifloodHandler,
		NumCrossShardPeers: numCrossShardPeers,
		NumIntraShardPeers: numIntraShardPeers,
	}
	resolverSender, err := topicResolverSender.NewTopicResolverSender(arg)
	if err != nil {
		return nil, err
	}

	trie := rcf.dataTrieContainer.Get([]byte(trieId))
	argTrieResolver := resolvers.ArgTrieNodeResolver{
		SenderResolver:   resolverSender,
		TrieDataGetter:   trie,
		Marshalizer:      rcf.marshalizer,
		AntifloodHandler: rcf.inputAntifloodHandler,
		Throttler:        rcf.throttler,
	}
	resolver, err := resolvers.NewTrieNodeResolver(argTrieResolver)
	if err != nil {
		return nil, err
	}

	err = rcf.messenger.RegisterMessageProcessor(resolver.RequestTopic(), resolver)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (rcf *resolversContainerFactory) IsInterfaceNil() bool {
	return rcf == nil
}
