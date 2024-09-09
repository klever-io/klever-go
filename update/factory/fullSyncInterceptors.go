package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/core/process/interceptors"
	interceptorFactory "github.com/klever-io/klever-go/core/process/interceptors/factory"
	"github.com/klever-io/klever-go/core/process/interceptors/processor"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/update"
)

var _ process.InterceptorsContainerFactory = (*fullSyncInterceptorsContainerFactory)(nil)

const numGoRoutines = 2000

// fullSyncInterceptorsContainerFactory will handle the creation the interceptors container for shards
type fullSyncInterceptorsContainerFactory struct {
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
	keyGen                 crypto.KeyGenerator
	singleSigner           crypto.SingleSigner
	addressPubkeyConv      core.PubkeyConverter
	whiteListHandler       update.WhiteListHandler
	whiteListerVerifiedTxs update.WhiteListHandler
	antifloodHandler       process.P2PAntifloodHandler
	kAppController         kapp.KAppController
}

// ArgsNewFullSyncInterceptorsContainerFactory holds the arguments needed for fullSyncInterceptorsContainerFactory
type ArgsNewFullSyncInterceptorsContainerFactory struct {
	Accounts                state.AccountsAdapter
	NodesCoordinator        sharding.NodesCoordinator
	Messenger               process.TopicHandler
	Store                   retriever.StorageService
	Marshalizer             marshal.Marshalizer
	TxSignMarshalizer       marshal.Marshalizer
	Hasher                  hashing.Hasher
	KeyGen                  crypto.KeyGenerator
	BlockSignKeyGen         crypto.KeyGenerator
	SingleSigner            crypto.SingleSigner
	BlockSingleSigner       crypto.SingleSigner
	MultiSigner             crypto.MultiSigner
	DataPool                retriever.PoolsHolder
	AddressPubkeyConverter  core.PubkeyConverter
	MaxTxNonceDeltaAllowed  int
	TxFeeHandler            process.EconomicsDataHandler
	BlockBlackList          process.TimeCacher
	HeaderSigVerifier       process.InterceptedHeaderSigVerifier
	HeaderIntegrityVerifier process.HeaderIntegrityVerifier
	//ValidityAttester          process.ValidityAttester
	EpochStartTrigger         process.EpochStartTriggerHandler
	WhiteListHandler          update.WhiteListHandler
	WhiteListerVerifiedTxs    update.WhiteListHandler
	InterceptorsContainer     process.InterceptorsContainer
	AntifloodHandler          process.P2PAntifloodHandler
	NonceConverter            typeConverters.Uint64ByteSliceConverter
	ChainID                   []byte
	MinTxVersion              uint32
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	KAppController            kapp.KAppController
}

// NewFullSyncInterceptorsContainerFactory is responsible for creating a new interceptors factory object
func NewFullSyncInterceptorsContainerFactory(
	args ArgsNewFullSyncInterceptorsContainerFactory,
) (*fullSyncInterceptorsContainerFactory, error) {
	err := checkBaseParams(
		args.Accounts,
		args.Marshalizer,
		args.Hasher,
		args.Store,
		args.DataPool,
		args.Messenger,
		args.MultiSigner,
		args.NodesCoordinator,
		args.BlockBlackList,
		args.NonceConverter,
		args.WhiteListerVerifiedTxs,
	)
	if err != nil {
		return nil, err
	}

	if check.IfNil(args.KeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(args.SingleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(args.AddressPubkeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.TxFeeHandler) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.BlockSignKeyGen) {
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
	if check.IfNil(args.InterceptorsContainer) {
		return nil, common.ErrNilInterceptorsContainer
	}
	if check.IfNil(args.WhiteListHandler) {
		return nil, common.ErrNilWhiteListHandler
	}
	if check.IfNil(args.AntifloodHandler) {
		return nil, process.ErrNilAntifloodHandler
	}
	if check.IfNil(args.TxSignHasher) {
		return nil, process.ErrNilHasher
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}
	if check.IfNil(args.KAppController) {
		return nil, common.ErrNilKAppController
	}

	argInterceptorFactory := &interceptorFactory.ArgInterceptedDataFactory{
		Hasher:                  args.Hasher,
		ProtoMarshalizer:        args.Marshalizer,
		TxSignMarshalizer:       args.TxSignMarshalizer,
		MultiSigVerifier:        args.MultiSigner,
		NodesCoordinator:        args.NodesCoordinator,
		AccountKeyGen:           args.KeyGen,
		BlockKeyGen:             args.BlockSignKeyGen,
		Signer:                  args.SingleSigner,
		BlockSigner:             args.BlockSingleSigner,
		AddressPubkeyConv:       args.AddressPubkeyConverter,
		FeeHandler:              args.TxFeeHandler,
		HeaderSigVerifier:       args.HeaderSigVerifier,
		HeaderIntegrityVerifier: args.HeaderIntegrityVerifier,
		//ValidityAttester:          args.ValidityAttester,
		EpochStartTrigger:         args.EpochStartTrigger,
		WhiteListerVerifiedTxs:    args.WhiteListerVerifiedTxs,
		ChainID:                   args.ChainID,
		MinTransactionVersion:     args.MinTxVersion,
		EnableSignTxWithHashEpoch: args.EnableSignTxWithHashEpoch,
		TxSignHasher:              args.TxSignHasher,
		EpochNotifier:             args.EpochNotifier,
	}

	icf := &fullSyncInterceptorsContainerFactory{
		container:              args.InterceptorsContainer,
		accounts:               args.Accounts,
		messenger:              args.Messenger,
		store:                  args.Store,
		marshalizer:            args.Marshalizer,
		hasher:                 args.Hasher,
		multiSigner:            args.MultiSigner,
		dataPool:               args.DataPool,
		nodesCoordinator:       args.NodesCoordinator,
		argInterceptorFactory:  argInterceptorFactory,
		blockBlackList:         args.BlockBlackList,
		maxTxNonceDeltaAllowed: args.MaxTxNonceDeltaAllowed,
		keyGen:                 args.KeyGen,
		singleSigner:           args.SingleSigner,
		addressPubkeyConv:      args.AddressPubkeyConverter,
		whiteListHandler:       args.WhiteListHandler,
		whiteListerVerifiedTxs: args.WhiteListerVerifiedTxs,
		antifloodHandler:       args.AntifloodHandler,
		kAppController:         args.KAppController,
	}

	icf.globalThrottler, err = throttler.NewNumGoRoutinesThrottler(numGoRoutines)
	if err != nil {
		return nil, err
	}

	return icf, nil
}

// Create returns an interceptor container that will hold all interceptors in the system
func (ficf *fullSyncInterceptorsContainerFactory) Create() (process.InterceptorsContainer, error) {
	err := ficf.generateTxInterceptors()
	if err != nil {
		return nil, err
	}

	err = ficf.generateMetachainHeaderInterceptors()
	if err != nil {
		return nil, err
	}

	err = ficf.generateTrieNodesInterceptors()
	if err != nil {
		return nil, err
	}

	return ficf.container, nil
}

func checkBaseParams(
	accounts state.AccountsAdapter,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	store retriever.StorageService,
	dataPool retriever.PoolsHolder,
	messenger process.TopicHandler,
	multiSigner crypto.MultiSigner,
	nodesCoordinator sharding.NodesCoordinator,
	blockBlackList process.TimeCacher,
	nonceConverter typeConverters.Uint64ByteSliceConverter,
	whiteListerVerifiedTxs update.WhiteListHandler,
) error {
	if check.IfNil(messenger) {
		return common.ErrNilMessenger
	}
	if check.IfNil(store) {
		return common.ErrNilStore
	}
	if check.IfNil(marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(hasher) {
		return process.ErrNilHasher
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
	if check.IfNil(blockBlackList) {
		return storage.ErrNilTimeCache
	}
	if check.IfNil(nonceConverter) {
		return process.ErrNilUint64Converter
	}
	if check.IfNil(whiteListerVerifiedTxs) {
		return process.ErrNilWhiteListHandler
	}

	return nil
}

func (ficf *fullSyncInterceptorsContainerFactory) checkIfInterceptorExists(identifier string) bool {
	_, err := ficf.container.Get(identifier)
	return err == nil
}

func (ficf *fullSyncInterceptorsContainerFactory) generateTrieNodesInterceptors() error {
	keys := make([]string, 0)
	trieInterceptors := make([]process.Interceptor, 0)

	identifierTrieNodes := common.ValidatorTrieNodesTopic
	if !ficf.checkIfInterceptorExists(identifierTrieNodes) {
		interceptor, err := ficf.createOneTrieNodesInterceptor(identifierTrieNodes)
		if err != nil {
			return err
		}

		keys = append(keys, identifierTrieNodes)
		trieInterceptors = append(trieInterceptors, interceptor)
	}

	identifierTrieNodes = common.AccountTrieNodesTopic
	if !ficf.checkIfInterceptorExists(identifierTrieNodes) {
		interceptor, err := ficf.createOneTrieNodesInterceptor(identifierTrieNodes)
		if err != nil {
			return err
		}

		keys = append(keys, identifierTrieNodes)
		trieInterceptors = append(trieInterceptors, interceptor)
	}

	return ficf.container.AddMultiple(keys, trieInterceptors)
}

func (ficf *fullSyncInterceptorsContainerFactory) createTopicAndAssignHandler(
	topic string,
	interceptor process.Interceptor,
	createChannel bool,
) (process.Interceptor, error) {

	err := ficf.messenger.CreateTopic(topic, createChannel)
	if err != nil {
		return nil, err
	}

	return interceptor, ficf.messenger.RegisterMessageProcessor(topic, interceptor)
}

func (ficf *fullSyncInterceptorsContainerFactory) generateTxInterceptors() error {
	identifierTx := common.TransactionTopic

	if ficf.checkIfInterceptorExists(identifierTx) {
		return nil
	}

	interceptor, err := ficf.createOneTxInterceptor(identifierTx)
	if err != nil {
		return err
	}

	return ficf.container.Add(identifierTx, interceptor)
}

func (ficf *fullSyncInterceptorsContainerFactory) createOneTxInterceptor(topic string) (process.Interceptor, error) {
	txStorer := ficf.store.GetStorer(retriever.TransactionUnit)

	txValidator, err := dataValidators.NewTxValidator(
		ficf.accounts,
		txStorer,
		ficf.dataPool,
		ficf.whiteListHandler,
		ficf.addressPubkeyConv,
		ficf.singleSigner,
		ficf.keyGen,
		ficf.kAppController,
		core.MaxTxNonceDeltaAllowed,
	)
	if err != nil {
		return nil, err
	}

	argProcessor := &processor.ArgTxInterceptorProcessor{
		TxDataCache: ficf.dataPool.Transactions(),
		TxValidator: txValidator,
	}
	txProcessor, err := processor.NewTxInterceptorProcessor(argProcessor)
	if err != nil {
		return nil, err
	}

	txFactory, err := interceptorFactory.NewInterceptedTxDataFactory(ficf.argInterceptorFactory)
	if err != nil {
		return nil, err
	}

	interceptor, err := interceptors.NewMultiDataInterceptor(
		interceptors.ArgMultiDataInterceptor{
			Topic:            topic,
			Marshalizer:      ficf.marshalizer,
			DataFactory:      txFactory,
			Processor:        txProcessor,
			Throttler:        ficf.globalThrottler,
			AntifloodHandler: ficf.antifloodHandler,
			WhiteListRequest: ficf.whiteListHandler,
			CurrentPeerID:    ficf.messenger.ID(),
		},
	)
	if err != nil {
		return nil, err
	}

	return ficf.createTopicAndAssignHandler(topic, interceptor, true)
}

func (ficf *fullSyncInterceptorsContainerFactory) generateMetachainHeaderInterceptors() error {
	identifierHdr := common.BlocksTopic
	if ficf.checkIfInterceptorExists(identifierHdr) {
		return nil
	}

	hdrFactory, err := interceptorFactory.NewInterceptedBlockDataFactory(ficf.argInterceptorFactory)
	if err != nil {
		return err
	}

	argProcessor := &processor.ArgHdrInterceptorProcessor{
		Headers:        ficf.dataPool.Headers(),
		BlockBlackList: ficf.blockBlackList,
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
			Throttler:        ficf.globalThrottler,
			AntifloodHandler: ficf.antifloodHandler,
			WhiteListRequest: ficf.whiteListHandler,
			CurrentPeerID:    ficf.messenger.ID(),
		},
	)
	if err != nil {
		return err
	}

	_, err = ficf.createTopicAndAssignHandler(identifierHdr, interceptor, true)
	if err != nil {
		return err
	}

	return ficf.container.Add(identifierHdr, interceptor)
}

func (ficf *fullSyncInterceptorsContainerFactory) createOneTrieNodesInterceptor(topic string) (process.Interceptor, error) {
	trieNodesProcessor, err := processor.NewTrieNodesInterceptorProcessor(ficf.dataPool.TrieNodes())
	if err != nil {
		return nil, err
	}

	trieNodesFactory, err := interceptorFactory.NewInterceptedTrieNodeDataFactory(ficf.argInterceptorFactory)
	if err != nil {
		return nil, err
	}

	interceptor, err := interceptors.NewMultiDataInterceptor(
		interceptors.ArgMultiDataInterceptor{
			Topic:            topic,
			Marshalizer:      ficf.marshalizer,
			DataFactory:      trieNodesFactory,
			Processor:        trieNodesProcessor,
			Throttler:        ficf.globalThrottler,
			AntifloodHandler: ficf.antifloodHandler,
			WhiteListRequest: ficf.whiteListHandler,
			CurrentPeerID:    ficf.messenger.ID(),
		},
	)
	if err != nil {
		return nil, err
	}

	return ficf.createTopicAndAssignHandler(topic, interceptor, true)
}

// IsInterfaceNil returns true if there is no value under the interface
func (ficf *fullSyncInterceptorsContainerFactory) IsInterfaceNil() bool {
	return ficf == nil
}
