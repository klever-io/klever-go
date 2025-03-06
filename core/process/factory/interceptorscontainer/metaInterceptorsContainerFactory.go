package interceptorscontainer

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/factory/containers"
	interceptorFactory "github.com/klever-io/klever-go/core/process/interceptors/factory"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.InterceptorsContainerFactory = (*metaInterceptorsContainerFactory)(nil)

// metaInterceptorsContainerFactory will handle the creation the interceptors container for metachain
type metaInterceptorsContainerFactory struct {
	*baseInterceptorsContainerFactory
}

// NewMetaInterceptorsContainerFactory is responsible for creating a new interceptors factory object
func NewMetaInterceptorsContainerFactory(
	args MetaInterceptorsContainerFactoryArgs,
) (*metaInterceptorsContainerFactory, error) {

	err := checkBaseParams(
		&MetaInterceptorsContainerFactoryArgs{
			Accounts:               args.Accounts,
			ProtoMarshalizer:       args.ProtoMarshalizer,
			TxSignMarshalizer:      args.TxSignMarshalizer,
			Hasher:                 args.Hasher,
			Store:                  args.Store,
			DataPool:               args.DataPool,
			Messenger:              args.Messenger,
			MultiSigner:            args.MultiSigner,
			NodesCoordinator:       args.NodesCoordinator,
			BlackList:              args.BlackList,
			AntifloodHandler:       args.AntifloodHandler,
			WhiteListHandler:       args.WhiteListHandler,
			WhiteListerVerifiedTxs: args.WhiteListerVerifiedTxs,
			AddressPubkeyConverter: args.AddressPubkeyConverter,
			KAppController:         args.KAppController,
			MaxTxNonceDeltaAllowed: args.MaxTxNonceDeltaAllowed,
			ForkController:         args.ForkController,
		},
	)
	if err != nil {
		return nil, err
	}
	if check.IfNil(args.SingleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(args.KeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(args.BlockKeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(args.BlockSingleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(args.HeaderSigVerifier) {
		return nil, common.ErrNilHeaderSigVerifier
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return nil, common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(args.EpochStartTrigger) {
		return nil, common.ErrNilEpochStartTrigger
	}
	if len(args.ChainID) == 0 {
		return nil, process.ErrInvalidChainID
	}
	if args.MinTransactionVersion == 0 {
		return nil, process.ErrInvalidTransactionVersion
	}
	if check.IfNil(args.TxSignHasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}

	argInterceptorFactory := &interceptorFactory.ArgInterceptedDataFactory{
		ProtoMarshalizer:          args.ProtoMarshalizer,
		TxSignMarshalizer:         args.TxSignMarshalizer,
		Hasher:                    args.Hasher,
		NodesCoordinator:          args.NodesCoordinator,
		MultiSigVerifier:          args.MultiSigner,
		AccountKeyGen:             args.KeyGen,
		BlockKeyGen:               args.BlockKeyGen,
		Signer:                    args.SingleSigner,
		BlockSigner:               args.BlockSingleSigner,
		HeaderSigVerifier:         args.HeaderSigVerifier,
		HeaderIntegrityVerifier:   args.HeaderIntegrityVerifier,
		EpochStartTrigger:         args.EpochStartTrigger,
		AddressPubkeyConv:         args.AddressPubkeyConverter,
		WhiteListerVerifiedTxs:    args.WhiteListerVerifiedTxs,
		ChainID:                   args.ChainID,
		MinTransactionVersion:     args.MinTransactionVersion,
		EnableSignTxWithHashEpoch: args.EnableSignTxWithHashEpoch,
		TxSignHasher:              args.TxSignHasher,
		EpochNotifier:             args.EpochNotifier,
		FeeHandler:                args.TxFeeHandler,
		ForkController:            args.ForkController,
	}

	container := containers.NewInterceptorsContainer()
	base := &baseInterceptorsContainerFactory{
		container:              container,
		messenger:              args.Messenger,
		store:                  args.Store,
		marshalizer:            args.ProtoMarshalizer,
		hasher:                 args.Hasher,
		multiSigner:            args.MultiSigner,
		dataPool:               args.DataPool,
		nodesCoordinator:       args.NodesCoordinator,
		blockBlackList:         args.BlackList,
		argInterceptorFactory:  argInterceptorFactory,
		accounts:               args.Accounts,
		maxTxNonceDeltaAllowed: args.MaxTxNonceDeltaAllowed,
		antifloodHandler:       args.AntifloodHandler,
		whiteListHandler:       args.WhiteListHandler,
		whiteListerVerifiedTxs: args.WhiteListerVerifiedTxs,
		addressPubkeyConverter: args.AddressPubkeyConverter,
		kAppController:         args.KAppController,
		forkController:         args.ForkController,
		requestedItemsHandler:  args.RequestedItemsHandler,
	}

	icf := &metaInterceptorsContainerFactory{
		baseInterceptorsContainerFactory: base,
	}

	icf.globalThrottler, err = throttler.NewNumGoRoutinesThrottler(numGoRoutines)
	if err != nil {
		return nil, err
	}

	return icf, nil
}

// Create returns an interceptor container that will hold all interceptors in the system
func (micf *metaInterceptorsContainerFactory) Create() (process.InterceptorsContainer, error) {
	err := micf.generateMetachainHeaderInterceptors()
	if err != nil {
		return nil, err
	}

	err = micf.generateTxInterceptors()
	if err != nil {
		return nil, err
	}

	err = micf.generateTrieNodesInterceptors()
	if err != nil {
		return nil, err
	}

	return micf.container, nil
}

func (micf *metaInterceptorsContainerFactory) generateTrieNodesInterceptors() error {
	keys := make([]string, 0)
	trieInterceptors := make([]process.Interceptor, 0)

	identifierTrieNodes := common.ValidatorTrieNodesTopic
	interceptor, err := micf.createOneTrieNodesInterceptor(identifierTrieNodes)
	if err != nil {
		return err
	}

	keys = append(keys, identifierTrieNodes)
	trieInterceptors = append(trieInterceptors, interceptor)

	identifierTrieNodes = common.AccountTrieNodesTopic
	interceptor, err = micf.createOneTrieNodesInterceptor(identifierTrieNodes)
	if err != nil {
		return err
	}

	keys = append(keys, identifierTrieNodes)
	trieInterceptors = append(trieInterceptors, interceptor)

	identifierTrieNodes = common.KappTrieNodesTopic
	interceptor, err = micf.createOneTrieNodesInterceptor(identifierTrieNodes)
	if err != nil {
		return err
	}

	keys = append(keys, identifierTrieNodes)
	trieInterceptors = append(trieInterceptors, interceptor)

	return micf.container.AddMultiple(keys, trieInterceptors)
}

// IsInterfaceNil returns true if there is no value under the interface
func (micf *metaInterceptorsContainerFactory) IsInterfaceNil() bool {
	return micf == nil
}
