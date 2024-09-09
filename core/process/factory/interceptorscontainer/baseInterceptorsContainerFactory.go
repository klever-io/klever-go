package interceptorscontainer

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/core/process/interceptors"
	interceptorFactory "github.com/klever-io/klever-go/core/process/interceptors/factory"
	"github.com/klever-io/klever-go/core/process/interceptors/processor"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

const numGoRoutines = 100

type baseInterceptorsContainerFactory struct {
	container              process.InterceptorsContainer
	accounts               state.AccountsAdapter
	marshalizer            marshal.Marshalizer
	hasher                 hashing.Hasher
	store                  retriever.StorageService
	dataPool               retriever.PoolsHolder
	messenger              process.TopicHandler
	multiSigner            crypto.MultiSigner
	nodesCoordinator       sharding.NodesCoordinator
	blockBlackList         process.TimeCacher
	argInterceptorFactory  *interceptorFactory.ArgInterceptedDataFactory
	globalThrottler        process.InterceptorThrottler
	maxTxNonceDeltaAllowed int
	antifloodHandler       process.P2PAntifloodHandler
	whiteListHandler       process.WhiteListHandler
	whiteListerVerifiedTxs process.WhiteListHandler
	addressPubkeyConverter core.PubkeyConverter
	kAppController         kapp.KAppController
}

func checkBaseParams(
	accounts state.AccountsAdapter,
	marshalizer marshal.Marshalizer,
	signMarshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	store retriever.StorageService,
	dataPool retriever.PoolsHolder,
	messenger process.TopicHandler,
	multiSigner crypto.MultiSigner,
	nodesCoordinator sharding.NodesCoordinator,
	blackList process.TimeCacher,
	antifloodHandler process.P2PAntifloodHandler,
	whiteListHandler process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	addressPubkeyConverter core.PubkeyConverter,
	kAppController kapp.KAppController,
	maxTxNonceDeltaAllowed int,
) error {
	if check.IfNil(messenger) {
		return common.ErrNilMessenger
	}
	if check.IfNil(store) {
		return common.ErrNilStore
	}
	if check.IfNil(marshalizer) || check.IfNil(signMarshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(multiSigner) {
		return common.ErrNilMultiSigVerifier
	}
	if check.IfNil(dataPool) {
		return common.ErrNilDataPoolHolder
	}
	if check.IfNil(nodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(accounts) {
		return common.ErrNilAccountsAdapter
	}
	if check.IfNil(blackList) {
		return process.ErrNilBlackListCacher
	}
	if check.IfNil(antifloodHandler) {
		return common.ErrNilAntifloodHandler
	}
	if check.IfNil(whiteListHandler) {
		return common.ErrNilWhiteListHandler
	}
	if check.IfNil(whiteListerVerifiedTxs) {
		return process.ErrNilWhiteListHandler
	}
	if check.IfNil(addressPubkeyConverter) {
		return common.ErrNilPubkeyConverter
	}
	if check.IfNil(kAppController) {
		return common.ErrNilKAppController
	}

	return nil
}

func (bicf *baseInterceptorsContainerFactory) createTopicAndAssignHandler(
	topic string,
	interceptor process.Interceptor,
	createChannel bool,
) (process.Interceptor, error) {

	err := bicf.messenger.CreateTopic(topic, createChannel)
	if err != nil {
		return nil, err
	}

	return interceptor, bicf.messenger.RegisterMessageProcessor(topic, interceptor)
}

//------- Tx interceptors

func (bicf *baseInterceptorsContainerFactory) generateTxInterceptors() error {

	identifierTx := common.TransactionTopic

	interceptor, err := bicf.createOneTxInterceptor(identifierTx)
	if err != nil {
		return err
	}

	return bicf.container.Add(identifierTx, interceptor)
}

func (bicf *baseInterceptorsContainerFactory) createOneTxInterceptor(topic string) (process.Interceptor, error) {

	txStorer := bicf.store.GetStorer(retriever.TransactionUnit)

	txValidator, err := dataValidators.NewTxValidator(
		bicf.accounts,
		txStorer,
		bicf.dataPool,
		bicf.whiteListHandler,
		bicf.addressPubkeyConverter,
		bicf.argInterceptorFactory.Signer,
		bicf.argInterceptorFactory.AccountKeyGen,
		bicf.kAppController,
		core.MaxTxNonceDeltaAllowed,
	)
	if err != nil {
		return nil, err
	}

	argProcessor := &processor.ArgTxInterceptorProcessor{
		TxDataCache: bicf.dataPool.Transactions(),
		TxValidator: txValidator,
	}
	txProcessor, err := processor.NewTxInterceptorProcessor(argProcessor)
	if err != nil {
		return nil, err
	}

	txFactory, err := interceptorFactory.NewInterceptedTxDataFactory(bicf.argInterceptorFactory)
	if err != nil {
		return nil, err
	}

	interceptor, err := interceptors.NewMultiDataInterceptor(
		interceptors.ArgMultiDataInterceptor{
			Topic:            topic,
			Marshalizer:      bicf.marshalizer,
			DataFactory:      txFactory,
			Processor:        txProcessor,
			Throttler:        bicf.globalThrottler,
			AntifloodHandler: bicf.antifloodHandler,
			WhiteListRequest: bicf.whiteListHandler,
			CurrentPeerID:    bicf.messenger.ID(),
		},
	)
	if err != nil {
		return nil, err
	}

	return bicf.createTopicAndAssignHandler(topic, interceptor, true)
}

func (bicf *baseInterceptorsContainerFactory) createOneTrieNodesInterceptor(topic string) (process.Interceptor, error) {
	trieNodesProcessor, err := processor.NewTrieNodesInterceptorProcessor(bicf.dataPool.TrieNodes())
	if err != nil {
		return nil, err
	}

	trieNodesFactory, err := interceptorFactory.NewInterceptedTrieNodeDataFactory(bicf.argInterceptorFactory)
	if err != nil {
		return nil, err
	}

	interceptor, err := interceptors.NewMultiDataInterceptor(
		interceptors.ArgMultiDataInterceptor{
			Topic:            topic,
			Marshalizer:      bicf.marshalizer,
			DataFactory:      trieNodesFactory,
			Processor:        trieNodesProcessor,
			Throttler:        bicf.globalThrottler,
			AntifloodHandler: bicf.antifloodHandler,
			WhiteListRequest: bicf.whiteListHandler,
			CurrentPeerID:    bicf.messenger.ID(),
		},
	)
	if err != nil {
		return nil, err
	}

	return bicf.createTopicAndAssignHandler(topic, interceptor, true)
}

//------- MetachainHeader interceptors

func (bicf *baseInterceptorsContainerFactory) generateMetachainHeaderInterceptors() error {
	identifierHdr := common.BlocksTopic

	hdrFactory, err := interceptorFactory.NewInterceptedBlockDataFactory(bicf.argInterceptorFactory)
	if err != nil {
		return err
	}

	argProcessor := &processor.ArgHdrInterceptorProcessor{
		Headers:          bicf.dataPool.Headers(),
		BlockBlackList:   bicf.blockBlackList,
		Marshalizer:      bicf.marshalizer,
		Hasher:           bicf.hasher,
		WhiteListHandler: bicf.whiteListHandler,
	}
	hdrProcessor, err := processor.NewHdrInterceptorProcessor(argProcessor)
	if err != nil {
		return err
	}

	//only one metachain header topic
	interceptor, err := interceptors.NewSingleDataInterceptor(
		interceptors.ArgSingleDataInterceptor{
			Topic:            identifierHdr,
			DataFactory:      hdrFactory,
			Processor:        hdrProcessor,
			Throttler:        bicf.globalThrottler,
			AntifloodHandler: bicf.antifloodHandler,
			WhiteListRequest: bicf.whiteListHandler,
			CurrentPeerID:    bicf.messenger.ID(),
		},
	)
	if err != nil {
		return err
	}

	_, err = bicf.createTopicAndAssignHandler(identifierHdr, interceptor, true)
	if err != nil {
		return err
	}

	return bicf.container.Add(identifierHdr, interceptor)
}
