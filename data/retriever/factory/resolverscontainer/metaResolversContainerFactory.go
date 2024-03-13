package resolverscontainer

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/resolvers"
	triesFactory "github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/tools/random"
)

var _ retriever.ResolversContainerFactory = (*metaResolversContainerFactory)(nil)

type metaResolversContainerFactory struct {
	*baseResolversContainerFactory
}

// NewMetaResolversContainerFactory creates a new container filled with topic resolvers for metachain
func NewMetaResolversContainerFactory(
	args FactoryArgs,
) (*metaResolversContainerFactory, error) {
	thr, err := throttler.NewNumGoRoutinesThrottler(args.NumConcurrentResolvingJobs)
	if err != nil {
		return nil, err
	}

	container := containers.NewResolversContainer()
	base := &baseResolversContainerFactory{
		container:                container,
		messenger:                args.Messenger,
		store:                    args.Store,
		marshalizer:              args.Marshalizer,
		dataPools:                args.DataPools,
		uint64ByteSliceConverter: args.Uint64ByteSliceConverter,
		intRandomizer:            &random.ConcurrentSafeIntRandomizer{},
		dataPacker:               args.DataPacker,
		triesContainer:           args.TriesContainer,
		inputAntifloodHandler:    args.InputAntifloodHandler,
		outputAntifloodHandler:   args.OutputAntifloodHandler,
		throttler:                thr,
	}

	err = base.checkParams()
	if err != nil {
		return nil, err
	}

	base.intraShardTopic = common.ConsensusTopic

	return &metaResolversContainerFactory{
		baseResolversContainerFactory: base,
	}, nil
}

// Create returns an interceptor container that will hold all interceptors in the system
func (mrcf *metaResolversContainerFactory) Create() (retriever.ResolversContainer, error) {
	err := mrcf.generateChainHeaderResolvers()
	if err != nil {
		return nil, err
	}

	err = mrcf.generateTxResolvers(
		common.TransactionTopic,
		retriever.TransactionUnit,
		mrcf.dataPools.Transactions(),
	)
	if err != nil {
		return nil, err
	}

	err = mrcf.generateTrieNodesResolvers()
	if err != nil {
		return nil, err
	}

	return mrcf.container, nil
}

//------- Meta header resolvers

func (mrcf *metaResolversContainerFactory) generateChainHeaderResolvers() error {
	identifierHeader := common.BlocksTopic
	resolver, err := mrcf.createChainHeaderResolver(identifierHeader)
	if err != nil {
		return err
	}

	return mrcf.container.Add(identifierHeader, resolver)
}

func (mrcf *metaResolversContainerFactory) createChainHeaderResolver(
	identifier string,
) (retriever.Resolver, error) {
	hdrStorer := mrcf.store.GetStorer(retriever.BlockUnit)

	resolverSender, err := mrcf.createOneResolverSender(identifier, EmptyExcludePeersOnTopic)
	if err != nil {
		return nil, err
	}

	hdrNonceStore := mrcf.store.GetStorer(retriever.HdrNonceHashDataUnit)
	arg := resolvers.ArgHeaderResolver{
		SenderResolver:       resolverSender,
		Headers:              mrcf.dataPools.Headers(),
		HdrStorage:           hdrStorer,
		HeadersNoncesStorage: hdrNonceStore,
		Marshalizer:          mrcf.marshalizer,
		NonceConverter:       mrcf.uint64ByteSliceConverter,
		AntifloodHandler:     mrcf.inputAntifloodHandler,
		Throttler:            mrcf.throttler,
		DataPacker:           mrcf.dataPacker,
	}
	resolver, err := resolvers.NewHeaderResolver(arg)
	if err != nil {
		return nil, err
	}

	err = mrcf.messenger.RegisterMessageProcessor(resolver.RequestTopic(), resolver)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}

func (mrcf *metaResolversContainerFactory) generateTrieNodesResolvers() error {
	keys := make([]string, 0)
	resolversSlice := make([]retriever.Resolver, 0)

	identifierTrieNodes := common.AccountTrieNodesTopic
	resolver, err := mrcf.createTrieNodesResolver(identifierTrieNodes, triesFactory.UserAccountTrie, 0, numIntraShardPeers+numCrossShardPeers)
	if err != nil {
		return err
	}

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	identifierTrieNodes = common.ValidatorTrieNodesTopic
	resolver, err = mrcf.createTrieNodesResolver(identifierTrieNodes, triesFactory.PeerAccountTrie, 0, numIntraShardPeers+numCrossShardPeers)
	if err != nil {
		return err
	}

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	identifierTrieNodes = common.KappTrieNodesTopic
	resolver, err = mrcf.createTrieNodesResolver(identifierTrieNodes, triesFactory.KAppAccountTrie, 0, numIntraShardPeers+numCrossShardPeers)
	if err != nil {
		return err
	}

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	return mrcf.container.AddMultiple(keys, resolversSlice)
}

// IsInterfaceNil returns true if there is no value under the interface
func (mrcf *metaResolversContainerFactory) IsInterfaceNil() bool {
	return mrcf == nil
}
