package storageResolversContainers

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/factory/containers"
	"github.com/klever-io/klever-go/data/retriever/storageResolvers"
)

var _ retriever.ResolversContainerFactory = (*metaResolversContainerFactory)(nil)

type metaResolversContainerFactory struct {
	*baseResolversContainerFactory
}

// NewMetaResolversContainerFactory creates a new container filled with topic resolvers for metachain
func NewMetaResolversContainerFactory(
	args FactoryArgs,
) (*metaResolversContainerFactory, error) {
	container := containers.NewResolversContainer()
	base := &baseResolversContainerFactory{
		container:                container,
		messenger:                args.Messenger,
		store:                    args.Store,
		marshalizer:              args.Marshalizer,
		uint64ByteSliceConverter: args.Uint64ByteSliceConverter,
		dataPacker:               args.DataPacker,
		manualEpochStartNotifier: args.ManualEpochStartNotifier,
		chanGracefullyClose:      args.ChanGracefullyClose,
	}

	err := base.checkParams()
	if err != nil {
		return nil, err
	}

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
	resolver, err := mrcf.createChainHeaderResolver()
	if err != nil {
		return err
	}

	return mrcf.container.Add(identifierHeader, resolver)
}

func (mrcf *metaResolversContainerFactory) createChainHeaderResolver() (retriever.Resolver, error) {
	hdrStorer := mrcf.store.GetStorer(retriever.BlockUnit)

	hdrNonceStore := mrcf.store.GetStorer(retriever.HdrNonceHashDataUnit)
	arg := storageResolvers.ArgHeaderResolver{
		Messenger:                mrcf.messenger,
		ResponseTopicName:        common.BlocksTopic,
		NonceConverter:           mrcf.uint64ByteSliceConverter,
		HdrStorage:               hdrStorer,
		DataPacker:               mrcf.dataPacker,
		HeadersNoncesStorage:     hdrNonceStore,
		ManualEpochStartNotifier: mrcf.manualEpochStartNotifier,
		ChanGracefullyClose:      mrcf.chanGracefullyClose,
		DelayBeforeGracefulClose: defaultBeforeGracefulClose,
	}
	resolver, err := storageResolvers.NewHeaderResolver(arg)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}

func (mrcf *metaResolversContainerFactory) generateTrieNodesResolvers() error {
	keys := make([]string, 0)
	resolversSlice := make([]retriever.Resolver, 0)

	identifierTrieNodes := common.AccountTrieNodesTopic
	resolver := storageResolvers.NewTrieNodeResolver()

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	identifierTrieNodes = common.ValidatorTrieNodesTopic
	resolver = storageResolvers.NewTrieNodeResolver()

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	identifierTrieNodes = common.KappTrieNodesTopic
	resolver = storageResolvers.NewTrieNodeResolver()

	resolversSlice = append(resolversSlice, resolver)
	keys = append(keys, identifierTrieNodes)

	return mrcf.container.AddMultiple(keys, resolversSlice)
}

// IsInterfaceNil returns true if there is no value under the interface
func (mrcf *metaResolversContainerFactory) IsInterfaceNil() bool {
	return mrcf == nil
}
