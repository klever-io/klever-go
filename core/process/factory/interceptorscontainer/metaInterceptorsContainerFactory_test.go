package interceptorscontainer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	cMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/factory/interceptorscontainer"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
)

var errExpected = errors.New("expected error")

func createMetaStubTopicHandler(matchStrToErrOnCreate string, matchStrToErrOnRegister string) process.TopicHandler {
	return &mock.TopicHandlerStub{
		CreateTopicCalled: func(name string, createChannelForTopic bool) error {
			if matchStrToErrOnCreate == "" {
				return nil
			}

			if strings.Contains(name, matchStrToErrOnCreate) {
				return errExpected
			}

			return nil
		},
		RegisterMessageProcessorCalled: func(topic string, handler p2p.MessageProcessor) error {
			if matchStrToErrOnRegister == "" {
				return nil
			}

			if strings.Contains(topic, matchStrToErrOnRegister) {
				return errExpected
			}

			return nil
		},
	}
}

func createMetaDataPools() retriever.PoolsHolder {
	pools := &mock.PoolsHolderStub{
		HeadersCalled: func() retriever.HeadersPool {
			return &mock.HeadersCacherStub{}
		},
		BlocksCalled: func() storage.Cacher {
			return mock.NewCacherStub()
		},
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
		UnsignedTransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
		TrieNodesCalled: func() storage.Cacher {
			return mock.NewCacherStub()
		},
		RewardTransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
	}

	return pools
}

func createMetaStore() *mock.ChainStorerMock {
	return &mock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{}
		},
	}
}

//------- NewInterceptorsContainerFactory

func TestNewMetaInterceptorsContainerFactory_InvalidChainIDShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.ChainID = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, process.ErrInvalidChainID, err)
}

func TestNewMetaInterceptorsContainerFactory_InvalidMinTransactionVersionShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.MinTransactionVersion = 0
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, process.ErrInvalidTransactionVersion, err)
}

func TestNewMetaInterceptorsContainerFactory_NilNodesCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.NodesCoordinator = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewMetaInterceptorsContainerFactory_NilTopicHandlerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilMessenger, err)
}

func TestNewMetaInterceptorsContainerFactory_NilStoreShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Store = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilStore, err)
}

func TestNewMetaInterceptorsContainerFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.ProtoMarshalizer = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewMetaInterceptorsContainerFactory_NilMarshalizerAndSizeCheckShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.ProtoMarshalizer = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewMetaInterceptorsContainerFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Hasher = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewMetaInterceptorsContainerFactory_NilMultiSignerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.MultiSigner = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilMultiSigVerifier, err)
}

func TestNewMetaInterceptorsContainerFactory_NilDataPoolShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.DataPool = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilDataPoolHolder, err)
}

func TestNewMetaInterceptorsContainerFactory_NilAccountsShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Accounts = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestNewMetaInterceptorsContainerFactory_NilAddrConvShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.AddressPubkeyConverter = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewMetaInterceptorsContainerFactory_NilSingleSignerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.SingleSigner = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilSingleSigner, err)
}

func TestNewMetaInterceptorsContainerFactory_NilKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.KeyGen = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilKeyGen, err)
}

func TestNewMetaInterceptorsContainerFactory_NilTxSignHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.TxSignHasher = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewMetaInterceptorsContainerFactory_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.EpochNotifier = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, common.ErrNilEpochNotifier, err)
}

func TestNewMetaInterceptorsContainerFactory_NilBlackListHandlerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.BlackList = nil
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.Nil(t, icf)
	assert.Equal(t, process.ErrNilBlackListCacher, err)
}

func TestNewMetaInterceptorsContainerFactory_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.NotNil(t, icf)
	assert.Nil(t, err)
}

func TestNewMetaInterceptorsContainerFactory_ShouldWorkWithSizeCheck(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	icf, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	assert.NotNil(t, icf)
	assert.Nil(t, err)
	assert.False(t, icf.IsInterfaceNil())
}

//------- Create

func TestMetaInterceptorsContainerFactory_CreateTopicMetablocksFailsShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = createMetaStubTopicHandler(common.BlocksTopic, "")
	icf, _ := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	container, err := icf.Create()

	assert.Nil(t, container)
	assert.Equal(t, errExpected, err)
}

func TestMetaInterceptorsContainerFactory_CreateRegisterForMetablocksFailsShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = createMetaStubTopicHandler("", common.BlocksTopic)
	icf, _ := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	container, err := icf.Create()

	assert.Nil(t, container)
	assert.Equal(t, errExpected, err)
}

func TestMetaInterceptorsContainerFactory_CreateRegisterShardHeadersForMetachainFailsShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = createMetaStubTopicHandler("", common.BlocksTopic)
	icf, _ := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	container, err := icf.Create()

	assert.Nil(t, container)
	assert.Equal(t, errExpected, err)
}

func TestMetaInterceptorsContainerFactory_CreateRegisterTrieNodesFailsShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = createMetaStubTopicHandler("", common.AccountTrieNodesTopic)
	icf, _ := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	container, err := icf.Create()

	assert.Nil(t, container)
	assert.Equal(t, errExpected, err)
}

func TestMetaInterceptorsContainerFactory_CreateShouldWork(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = &mock.TopicHandlerStub{
		CreateTopicCalled: func(name string, createChannelForTopic bool) error {
			return nil
		},
		RegisterMessageProcessorCalled: func(topic string, handler p2p.MessageProcessor) error {
			return nil
		},
	}
	icf, _ := interceptorscontainer.NewMetaInterceptorsContainerFactory(args)

	container, err := icf.Create()

	assert.NotNil(t, container)
	assert.Nil(t, err)
}

func getArgumentsMeta() interceptorscontainer.MetaInterceptorsContainerFactoryArgs {
	marshalizerMock := &mock.ProtoMarshalizerMock{}

	accCacher, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{},
			Kapps:    &mock.AccountsStub{},
			Peers:    &mock.AccountsStub{},
		},
	)
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         &mock.HasherMock{},
		Marshalizer:    marshalizerMock,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    &mock.RatingsInfoMock{},
	}

	kAppController, _ := kappcontroller.NewKappController(argsKapp)

	return interceptorscontainer.MetaInterceptorsContainerFactoryArgs{
		NodesCoordinator:        mock.NewNodesCoordinatorMock(),
		Messenger:               &mock.TopicHandlerStub{},
		Store:                   createMetaStore(),
		ProtoMarshalizer:        &mock.MarshalizerMock{},
		TxSignMarshalizer:       &mock.MarshalizerMock{},
		Hasher:                  &mock.HasherMock{},
		MultiSigner:             mock.NewMultiSigner(),
		DataPool:                createMetaDataPools(),
		Accounts:                &mock.AccountsStub{},
		AddressPubkeyConverter:  cryptoMock.NewPubkeyConverterMock(32),
		SingleSigner:            &cryptoMock.SignerMock{},
		BlockSingleSigner:       &cryptoMock.SignerMock{},
		KeyGen:                  &cryptoMock.SingleSignKeyGenMock{},
		BlockKeyGen:             &cryptoMock.SingleSignKeyGenMock{},
		TxFeeHandler:            &mock.FeeHandlerStub{},
		BlackList:               &mock.BlackListHandlerStub{},
		HeaderSigVerifier:       &cMock.HeaderSigVerifierStub{},
		HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
		//ValidityAttester:        &mock.ValidityAttesterStub{},
		EpochStartTrigger:      &mock.EpochStartTriggerStub{},
		AntifloodHandler:       &mock.P2PAntifloodHandlerStub{},
		WhiteListHandler:       &mock.WhiteListHandlerStub{},
		WhiteListerVerifiedTxs: &mock.WhiteListHandlerStub{},
		ChainID:                []byte("chainID"),
		MinTransactionVersion:  1,
		TxSignHasher:           mock.HasherMock{},
		EpochNotifier:          &mock.EpochNotifierStub{},
		KAppController:         kAppController,
		ForkController:         forkController,
	}
}
