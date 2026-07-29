package block_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	blproc "github.com/klever-io/klever-go/core/process/block"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/eventNotifier/epochStart"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var marshalizer = marshal.NewProtoMarshalizer()

func createMockMetaArguments() blproc.ArgMetaProcessor {
	mdp := initDataPool([]byte("tx_hash"))

	hasher := &sha256.Sha256{}
	trieFactoryManager, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())

	kappAccountsDB := createKappsAccountsDB(hasher, marshalizer, factory.NewKAppAccountCreator(), trieFactoryManager)

	accountsDb := make(map[state.AccountsDbIdentifier]state.AccountsAdapter)
	accountsDb[state.UserAccountsState] = &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return nil, nil
		},
	}
	accountsDb[state.PeerAccountsState] = &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return nil, nil
		},
	}
	accountsDb[state.KAppAccountsState] = kappAccountsDB

	marshalizerMock := &mock.ProtoMarshalizerMock{}

	accCacher, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: accountsDb[state.UserAccountsState],
			Kapps:    accountsDb[state.KAppAccountsState],
			Peers:    accountsDb[state.PeerAccountsState],
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
		Hasher:         hasher,
		Marshalizer:    marshalizerMock,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    &mock.RatingsInfoMock{},
	}

	kAppController, _ := kappcontroller.NewKappController(argsKapp)
	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)

	arguments := blproc.ArgMetaProcessor{
		ArgBaseProcessor: blproc.ArgBaseProcessor{
			AccountsDB:        accountsDb,
			ForkDetector:      &mock.ForkDetectorMock{},
			Hasher:            &mock.HasherStub{},
			Marshalizer:       marshalizerMock,
			Store:             &mock.ChainStorerMock{},
			NodesCoordinator:  mock.NewNodesCoordinatorMock(),
			Uint64Converter:   &mock.Uint64ByteSliceConverterMock{},
			RequestHandler:    &mock.RequestHandlerStub{},
			TxCoordinator:     &mock.TransactionCoordinatorMock{},
			FeeHandler:        &mock.FeeAccumulatorStub{},
			EpochStartTrigger: &mock.EpochStartTriggerStub{},
			SlotManager:       &consensusMock.SlotManagerMock{},
			BootStorer: &mock.BoostrapStorerMock{
				PutCalled: func(slot int64, bootData *bootstrapStorage.BootstrapData) error {
					return nil
				},
			},
			DataPool:                mdp,
			BlockChain:              createTestBlockchain(),
			BlockChainHook:          &contextmock.BlockchainHookStub{},
			BlockSizeThrottler:      &mock.BlockSizeThrottlerStub{},
			TpsBenchmark:            &mock.TpsBenchmarkMock{},
			EpochNotifier:           epochNotifier,
			HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
			KAppController:          kAppController,
		},
		EconomicsData:                &mock.EconomicsHandlerStub{},
		ValidatorStatisticsProcessor: &mock.ValidatorStatisticsProcessorStub{},
		ForkController:               forkController,
	}

	return arguments
}

func createMetaBlockHeader() *block.Block {
	hdr := block.Block{
		Header: &block.BlockHeader{
			Nonce:        1,
			Slot:         1,
			ParentHash:   []byte(""),
			TxRootHash:   []byte("txRootHash"),
			TrieRoot:     []byte("rootHash"),
			TxCount:      1,
			PrevRandSeed: make([]byte, 0),
			RandSeed:     make([]byte, 0),
		},
		ProducerSignature: []byte("signature"),
		PubKeysBitmap:     make([]byte, 21),
	}

	return &hdr
}

func createTestBlockchain() *mock.BlockChainMock {
	return &mock.BlockChainMock{
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &mock.HeaderHandlerStub{
				GetNonceCalled: func() uint64 {
					return 0
				},
				GetSlotCalled: func() uint64 {
					return 0
				},
				GetProducerPublicKeyCalled: func() []byte {
					return []byte("signature")
				},
			}
		}}
}

func createMemUnit() storage.Storer {
	capacity := uint32(10)
	shards := uint32(1)
	sizeInBytes := uint64(0)
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: capacity, Shards: shards, SizeInBytes: sizeInBytes})
	persist, _ := memorydb.NewlruDB(100000)
	unit, _ := storageUnit.NewStorageUnit(cache, persist)

	return unit
}

func createKappsAccountsDB(
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	accountFactory state.AccountFactory,
	trieStorageManager data.StorageManager,
) *state.AccountsDB {
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)

	kdaKapp := loadKAppAccount(adb, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(adb, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(adb, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = adb.SaveAccounts(kdaKapp, stakingKapp)

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	_, err := proposalKapp.StartProposalsKApp(forkController)
	if err != nil {
		return nil
	}

	_ = adb.SaveAccount(proposalKapp)

	return adb
}

func loadKAppAccount(kappsDB state.AccountsAdapter, address []byte) state.KAppAccountHandler {
	acc, _ := kappsDB.LoadAccount(address)
	kappAcc := acc.(state.KAppAccountHandler)

	return kappAcc
}

func initKLVAndKFIintoKapps(kdaKapp, stakingKapp state.KAppAccountHandler) {
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)
	aprKey := kdautils.ToKDAKey([]byte("APR"), nil)

	staking := kapps.StakingData{
		InterestType: kapps.StakingData_FPRI,
		TotalStaked:  0,
		FPR: []*kapps.FPRData{
			{
				TotalAmount: 1000,
				TotalStaked: 50000,
				Epoch:       1,
			},
			{
				TotalAmount: 500,
				TotalStaked: 52000,
				Epoch:       2,
			},
			{
				TotalAmount: 1300,
				TotalStaked: 43000,
				Epoch:       3,
			},
			{
				TotalAmount: 1000,
				TotalStaked: 30000,
				Epoch:       4,
			},
		},
		MinEpochsToUnstake: 0,
		MinEpochsToClaim:   0,
	}

	aprStaking := kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		TotalStaked:  0,
		APR: []*kapps.APRData{
			{
				Timestamp: time.Now().AddDate(0, 0, -10).Unix(),
				Epoch:     0,
				Value:     1000,
			},
		},
		MinEpochsToUnstake: 0,
		MinEpochsToClaim:   0,
	}

	stakingData, _ := marshalizer.Marshal(&staking)
	aprStakingData, _ := marshalizer.Marshal(&aprStaking)

	_ = stakingKapp.DataTrieTracker().SaveKeyValue(klvKey, stakingData)
	_ = stakingKapp.DataTrieTracker().SaveKeyValue(kfiKey, stakingData)
	_ = stakingKapp.DataTrieTracker().SaveKeyValue(aprKey, aprStakingData)

	klv := kapps.KDAData{
		ID:                kdautils.KLVIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER"),
		Ticker:            kdautils.KLVIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	kfi := kapps.KDAData{
		ID:                kdautils.KFIIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("KLEVER FINANCE"),
		Ticker:            kdautils.KFIIdentifier,
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Royalties:         &kapps.RoyaltiesData{},
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	apr := kapps.KDAData{
		ID:                []byte("APR"),
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("APR TEST"),
		Ticker:            []byte("APR"),
		OwnerAddress:      nil,
		Precision:         6,
		InitialSupply:     10000000000000000,
		CirculatingSupply: 100000000000000,
		MaxSupply:         90000000000000000,
		IssueDate:         time.Now().Unix(),
		Properties: &kapps.PropertiesData{
			CanFreeze: true,
			CanMint:   true,
			CanBurn:   true,
		},
		Attributes: &kapps.AttributesData{
			IsPaused:         false,
			IsNFTMintStopped: true,
		},
	}

	klvData, _ := marshalizer.Marshal(&klv)
	kfiData, _ := marshalizer.Marshal(&kfi)
	aprData, _ := marshalizer.Marshal(&apr)

	_ = kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(kfiKey, kfiData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(aprKey, aprData)
}

//------- NewMetaProcessor

func TestNewMetaProcessor_NilAccountsAdapterShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.AccountsDB[state.UserAccountsState] = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilDataPoolShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.DataPool = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilDataPoolHolder, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilForkDetectorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.ForkDetector = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilForkDetector, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.Hasher = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilHasher, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.Marshalizer = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilMarshalizer, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilChainStorerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.Store = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilStorage, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilRequestHeaderHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.RequestHandler = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilRequestHandler, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilEpochStartShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.EpochStartTrigger = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilEpochStartTrigger, err)
	assert.Nil(t, be)
}
func TestNewMetaProcessor_NilSlotManagerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.SlotManager = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilSlotManager, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilBootStorerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.BootStorer = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilStorage, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilTxCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.TxCoordinator = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilTransactionCoordinator, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilEconomicsFeeHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.FeeHandler = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilEconomicsFeeHandler, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilBlockChainHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.BlockChain = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilBlockChain, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilHeaderIntegrityVerifierShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.HeaderIntegrityVerifier = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilHeaderIntegrityVerifier, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilTpsBenchmarkShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.TpsBenchmark = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilTpsBenchmark, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.EpochNotifier = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilEpochNotifier, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilErrNilBlockChainHookShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.BlockChainHook = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilBlockchainHooks, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilBlockSizeThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.BlockSizeThrottler = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilBlockSizeThrottler, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilDataPoolHeaderShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.DataPool.(*mock.PoolsHolderStub).HeadersCalled = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilHeadersDataPool, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilEconomicsDataShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.EconomicsData = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilEconomicsData, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilValidatorStatisticsProcessorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.ValidatorStatisticsProcessor = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrNilValidatorStatistics, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_NilKAppControllerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.KAppController = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, common.ErrKAppController, err)
	assert.Nil(t, be)
}

func TestNewMetaProcessor_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()

	mp, err := blproc.NewMetaProcessor(arguments)
	assert.Nil(t, err)
	assert.NotNil(t, mp)
}

//------- ProcessBlock

func TestMetaProcessor_ProcessBlockWithNilHeaderShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	err = mp.ProcessBlock(nil, haveTime)
	assert.Equal(t, process.ErrNilBlockHeader, err)
}

func TestMetaProcessor_ProcessBlockWithNilHaveTimeFuncShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	err = mp.ProcessBlock(&block.Block{}, nil)
	assert.Equal(t, process.ErrNilHaveTimeHandler, err)
}

func TestMetaProcessor_ProcessWithDirtyAccountShouldErr(t *testing.T) {
	t.Parallel()

	// set accounts dirty
	journalLen := func() int { return 3 }
	revToSnapshot := func(snapshot int) error { return nil }
	hdr := block.Block{
		Header: &block.BlockHeader{
			Nonce:      1,
			ParentHash: []byte(""),
			TrieRoot:   []byte("roothash"),
		},
		ProducerSignature: []byte("signature"),
	}
	arguments := createMockMetaArguments()
	arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
		JournalLenCalled:       journalLen,
		RevertToSnapshotCalled: revToSnapshot,
	}
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	// should return err
	err = mp.ProcessBlock(&hdr, haveTime)
	assert.NotNil(t, err)
	assert.Equal(t, err, process.ErrAccountStateDirty)
}

func TestMetaProcessor_ProcessWithHeaderNotFirstShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 2,
		},
	}
	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrWrongNonceInBlock, err)
}

func TestMetaProcessor_ProcessWithHeaderNotCorrectNonceShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	blkc := blockchain.NewBlockChain()
	_ = blkc.SetCurrentBlockHeader(
		&block.Block{
			Header: &block.BlockHeader{
				Slot:  1,
				Nonce: 1,
			},
		},
	)
	_ = blkc.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{Nonce: 0},
	})

	arguments.BlockChain = blkc
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	hdr := &block.Block{
		Header: &block.BlockHeader{
			Slot:  3,
			Nonce: 3,
		},
	}

	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrWrongNonceInBlock, err)
}

//------- CommitBlock

func TestMetaProcessor_CommitBlockMarshalizerFailForHeaderShouldErr(t *testing.T) {
	t.Parallel()

	accounts := &mock.AccountsStub{
		RevertToSnapshotCalled: func(snapshot int) error {
			return nil
		},
	}
	errMarshalizer := errors.New("failure")
	hdr := createMetaBlockHeader()
	marshalizer := &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) (i []byte, e error) {
			if reflect.DeepEqual(obj, hdr) {
				return nil, errMarshalizer
			}

			return []byte("obj"), nil
		},
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return nil
		},
	}
	arguments := createMockMetaArguments()
	arguments.AccountsDB[state.UserAccountsState] = accounts
	arguments.Marshalizer = marshalizer
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	err = mp.CommitBlock(hdr)
	assert.Equal(t, errMarshalizer, err)
}

func TestMetaProcessor_CommitBlockWithNilHeaderShouldErr(t *testing.T) {
	t.Parallel()

	accounts := &mock.AccountsStub{
		RevertToSnapshotCalled: func(snapshot int) error {
			return nil
		},
	}
	errMarshalizer := errors.New("failure")
	hdr := createMetaBlockHeader()
	marshalizer := &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) (i []byte, e error) {
			if reflect.DeepEqual(obj, hdr) {
				return nil, errMarshalizer
			}

			return []byte("obj"), nil
		},
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return nil
		},
	}
	arguments := createMockMetaArguments()
	arguments.AccountsDB[state.UserAccountsState] = accounts
	arguments.Marshalizer = marshalizer
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	hdr = nil
	err = mp.CommitBlock(hdr)
	require.EqualError(t, err, process.ErrNilBlockHeader.Error())
}

func TestMetaProcessor_CommitBlockStorageFailsForHeaderShouldErr(t *testing.T) {
	t.Parallel()

	wasCalled := false
	errPersister := errors.New("failure")
	marshalizer := &mock.MarshalizerMock{}
	accounts := &mock.AccountsStub{
		CommitCalled: func() (i []byte, e error) {
			return nil, nil
		},
	}
	hdr := createMetaBlockHeader()
	wg := sync.WaitGroup{}
	wg.Add(1)
	hdrUnit := &mock.StorerStub{
		PutCalled: func(key, data []byte) error {
			wasCalled = true
			wg.Done()
			return errPersister
		},
		GetCalled: func(key []byte) (i []byte, e error) {
			hdrBuff, _ := marshalizer.Marshal(&block.Block{})
			return hdrBuff, nil
		},
	}

	store := initStore()
	store.AddStorer(retriever.BlockUnit, hdrUnit)
	arguments := createMockMetaArguments()
	arguments.Marshalizer = &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) (i []byte, e error) {
			hdrBuff, _ := marshalizer.Marshal(hdr)
			return hdrBuff, nil
		},
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return nil
		},
	}
	arguments.Hasher = &mock.HasherStub{
		ComputeCalled: func(s string) []byte {
			return nil
		},
	}
	arguments.AccountsDB[state.UserAccountsState] = accounts
	arguments.AccountsDB[state.PeerAccountsState] = accounts
	arguments.Store = store
	arguments.ForkDetector = &mock.ForkDetectorMock{
		AddHeaderCalled: func(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
			return nil
		},
		GetHighestFinalBlockNonceCalled: func() uint64 {
			return 0
		},
	}

	blkc := blockchain.NewBlockChain()
	_ = blkc.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{Nonce: 0},
	})
	arguments.BlockChain = blkc
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	err = mp.CommitBlock(hdr)
	wg.Wait()
	assert.True(t, wasCalled)
	assert.Nil(t, err)
}

func TestMetaProcessor_CommitBlockOkValsShouldWork(t *testing.T) {
	t.Parallel()

	mdp := initDataPool([]byte("tx_hash"))
	rootHash := []byte("rootHash")
	hdr := createMetaBlockHeader()
	marshalizer := &mock.MarshalizerMock{}
	accounts := &mock.AccountsStub{
		CommitCalled: func() (i []byte, e error) {
			return rootHash, nil
		},
		RootHashCalled: func() ([]byte, error) {
			return rootHash, nil
		},
	}
	forkDetectorAddCalled := false
	fd := &mock.ForkDetectorMock{
		AddHeaderCalled: func(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
			if header == hdr {
				forkDetectorAddCalled = true
				return nil
			}

			return errors.New("should have not got here")
		},
		GetHighestFinalBlockNonceCalled: func() uint64 {
			return 0
		},
	}
	hdrUnit := &mock.StorerStub{
		PutCalled: func(key, data []byte) error {
			return nil
		},
		GetCalled: func(key []byte) (i []byte, e error) {
			hdrBuff, _ := marshalizer.Marshal(&block.Block{})
			return hdrBuff, nil
		},
	}
	store := initStore()
	store.AddStorer(retriever.BlockUnit, hdrUnit)

	arguments := createMockMetaArguments()
	arguments.DataPool = mdp
	arguments.AccountsDB[state.UserAccountsState] = accounts
	arguments.AccountsDB[state.PeerAccountsState] = accounts
	arguments.ForkDetector = fd
	arguments.Store = store
	arguments.Hasher = &mock.HasherStub{
		ComputeCalled: func(s string) []byte {
			return nil
		},
	}
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	mdp.HeadersCalled = func() retriever.HeadersPool {
		cs := &mock.HeadersCacherStub{}
		cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {
		}
		cs.GetHeaderByHashCalled = func(hash []byte) (handler data.HeaderHandler, e error) {
			return &block.Block{}, nil
		}
		cs.LenCalled = func() int {
			return 0
		}
		cs.MaxSizeCalled = func() int {
			return 1000
		}
		cs.NoncesCalled = func() []uint64 {
			return nil
		}
		return cs
	}

	err = mp.CommitBlock(hdr)
	assert.Nil(t, err)
	assert.True(t, forkDetectorAddCalled)
}

func TestMetaProcessor_RevertStateShouldWork(t *testing.T) {
	recreateAccountsTrieWasCalled := false
	recreateKAppsTrieWasCalled := false
	revertPeerStateWasCalled := false

	arguments := createMockMetaArguments()
	arguments.DataPool = initDataPool([]byte("tx_hash"))
	arguments.Store = initStore()

	arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
		RecreateTrieCalled: func(rootHash []byte) error {
			recreateAccountsTrieWasCalled = true
			return nil
		},
	}

	arguments.AccountsDB[state.KAppAccountsState] = &mock.AccountsStub{
		RecreateTrieCalled: func(rootHash []byte) error {
			recreateKAppsTrieWasCalled = true
			return nil
		},
		LoadAccountCalled: func(container []byte) (state.AccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) {
							return []byte{}, nil
						},
					}
				},
			}, nil
		},
	}

	arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
		RevertPeerStateCalled: func(header data.HeaderHandler) error {
			revertPeerStateWasCalled = true
			return nil
		},
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	hdr := block.Block{Header: &block.BlockHeader{Nonce: 37}}
	err = mp.RevertStateToBlock(&hdr)
	assert.Nil(t, err)
	assert.True(t, revertPeerStateWasCalled)
	assert.True(t, recreateAccountsTrieWasCalled)
	assert.True(t, recreateKAppsTrieWasCalled)
}

func TestMetaProcessor_MarshalizedDataToBroadcastShouldWork(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.Store = initStore()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	msh, mstx, err := mp.MarshalizedDataToBroadcast(&block.Block{})

	assert.Nil(t, err)
	assert.NotNil(t, msh)
	assert.Nil(t, mstx)
}

func TestMetaProcessor_RestoreBlockIntoPoolsShouldErrNilBlockHeader(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.Store = initStore()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	err = mp.RestoreBlockIntoPools(nil)
	assert.NotNil(t, err)
	assert.Equal(t, err, process.ErrNilBlockHeader)
}

func TestMetaProcessor_CreateBlockCreateHeaderProcessBlock(t *testing.T) {
	t.Parallel()

	hash := []byte("hash1")
	hdrHash1Bytes := []byte("hdr_hash1")
	hrdHash2Bytes := []byte("hdr_hash2")
	hasher := &mock.HasherStub{}
	hasher.ComputeCalled = func(s string) []byte {
		return hash
	}
	arguments := createMockMetaArguments()
	kappsRootHash, _ := arguments.AccountsDB[state.KAppAccountsState].RootHash()

	dPool := initDataPool([]byte("tx_hash"))
	dPool.TransactionsCalled = func() retriever.ShardedDataCacherNotifier {
		return mock.NewShardedDataStub()
	}
	dPool.HeadersCalled = func() retriever.HeadersPool {
		cs := &mock.HeadersCacherStub{}
		cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {
		}
		cs.GetHeaderByHashCalled = func(key []byte) (handler data.HeaderHandler, e error) {
			if bytes.Equal(hdrHash1Bytes, key) {
				return &block.Block{
					Header: &block.BlockHeader{
						ParentHash:    []byte("hash1"),
						Nonce:         1,
						Slot:          1,
						PrevRandSeed:  []byte("roothash"),
						KAppsTrieRoot: kappsRootHash,
					},
				}, nil
			}
			if bytes.Equal(hrdHash2Bytes, key) {
				return &block.Block{
					Header: &block.BlockHeader{Nonce: 2, Slot: 2},
				}, nil
			}
			return nil, errors.New("err")
		}
		cs.LenCalled = func() int {
			return 0
		}
		cs.NoncesCalled = func() []uint64 {
			return []uint64{1, 2}
		}
		cs.MaxSizeCalled = func() int {
			return 1000
		}
		return cs
	}

	arguments.DataPool = dPool
	arguments.Hasher = hasher
	blkc := &mock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &block.Block{
				Header: &block.BlockHeader{Nonce: 0},
			}
		},
		GetCurrentBlockHeaderHashCalled: func() []byte {
			return hash
		},
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{
				Header: &block.BlockHeader{Nonce: 0},
			}
		},
	}
	arguments.BlockChain = blkc

	mp, err := blproc.NewMetaProcessor(arguments)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	require.NoError(t, err)
	slot := uint64(1)
	nonce := uint64(1)

	headerHandler := mp.CreateNewHeader(slot, nonce)
	headerHandler.SetParentHash(hash)
	headerHandler.SetKAppsTrieRoot(kappsRootHash)

	err = mp.ProcessBlock(headerHandler, func() time.Duration { return time.Second })
	assert.Nil(t, err)
}

func TestMetaProcessor_CreateAndProcessBlockCallsProcessAfterFirstEpoch(t *testing.T) {
	t.Parallel()

	hash := []byte("hash1")
	hdrHash1Bytes := []byte("hdr_hash1")
	hrdHash2Bytes := []byte("hdr_hash2")
	hasher := &mock.HasherStub{}
	hasher.ComputeCalled = func(s string) []byte {
		return hash
	}

	dPool := initDataPool([]byte("tx_hash"))
	dPool.TransactionsCalled = func() retriever.ShardedDataCacherNotifier {
		return mock.NewShardedDataStub()
	}
	dPool.HeadersCalled = func() retriever.HeadersPool {
		cs := &mock.HeadersCacherStub{}
		cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {
		}
		cs.GetHeaderByHashCalled = func(key []byte) (handler data.HeaderHandler, e error) {
			if bytes.Equal(hdrHash1Bytes, key) {
				return &block.Block{
					Header: &block.BlockHeader{
						ParentHash:   []byte("hash1"),
						Nonce:        1,
						Slot:         1,
						PrevRandSeed: []byte("roothash"),
					},
				}, nil
			}
			if bytes.Equal(hrdHash2Bytes, key) {
				return &block.Block{
					Header: &block.BlockHeader{
						Nonce: 2,
						Slot:  2,
					},
				}, nil
			}
			return nil, errors.New("err")
		}
		cs.LenCalled = func() int {
			return 0
		}
		cs.NoncesCalled = func() []uint64 {
			return []uint64{1, 2}
		}
		cs.MaxSizeCalled = func() int {
			return 1000
		}
		return cs
	}

	arguments := createMockMetaArguments()
	arguments.DataPool = dPool
	arguments.Hasher = hasher

	calledUpdateValidatorsStatus := false
	arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
		SaveNodesCoordinatorUpdatesCalled: func(epoch uint32) (bool, error) {
			calledUpdateValidatorsStatus = true
			return false, nil
		},
	}

	toggleCalled := true

	blkc := &mock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Slot: 0, Nonce: 0, IsEpochStart: true}}
		},
		GetCurrentBlockHeaderHashCalled: func() []byte {
			return hash
		},
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Slot: 0, Nonce: 0, IsEpochStart: true}}
		},
	}
	arguments.BlockChain = blkc

	mp, err := blproc.NewMetaProcessor(arguments)
	require.Nil(t, err)

	pc, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(pc)

	metaHdr := &block.Block{Header: &block.BlockHeader{}}
	headerHandler, err := mp.CreateBlock(metaHdr, func() bool { return true })
	require.Nil(t, err)
	assert.True(t, toggleCalled, calledUpdateValidatorsStatus)

	b := headerHandler.(*block.Block)
	b.Header.Nonce = 1
	b.Header.Slot = 1
	b.Header.ParentHash = hash

	// Recreate Kapps Trie in new processor
	hasherTrie := &sha256.Sha256{}
	trieFactoryManager, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	arguments.AccountsDB[state.KAppAccountsState] = createKappsAccountsDB(hasherTrie, marshalizer, factory.NewKAppAccountCreator(), trieFactoryManager)
	mp, err = blproc.NewMetaProcessor(arguments)
	require.Nil(t, err)

	_ = mp.SetProposalController(pc)

	calledUpdateValidatorsStatus = false
	err = mp.ProcessBlock(headerHandler, func() time.Duration { return time.Second })
	assert.Nil(t, err)
	assert.True(t, toggleCalled, calledUpdateValidatorsStatus)
}

func TestMetaProcessor_ProcessProposalEndOfEpoch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		blk    *block.Block
		perset func(*kapps.ProposalController, state.KAppAccountHandler, *blproc.MetaProcessorForTests) ([]byte, error)
		check  map[int32][]byte
		err    interface{}
	}{
		{
			name: "Error ProposalController",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1}},
			perset: func(pc *kapps.ProposalController, _ state.KAppAccountHandler, _ *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal controller with invalid data
				return []byte("invalid"), nil
			},
			err: "cannot parse invalid wire-format data",
		},
		{
			name: "No Active Proposals",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1}},
			perset: func(pc *kapps.ProposalController, _ state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal controller with valid data no active proposals/params
				return mp.GetMarshalizer().Marshal(&kapps.ProposalController{})
			},
			err: nil,
		},
		{
			name: "Error Invalida Proposal Data",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: 1}},
			perset: func(pc *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal 1
				err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{})
				if err != nil {
					return nil, err
				}

				// set proposal controller with valid data active proposals/params
				return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
					ActiveProposals: map[uint32]*kapps.ActiveProposals{
						1: {ProposalIDs: []uint64{1}},
					},
				})
			},
			err: common.ErrEmptyString,
		},
		{
			name: "Proposal Denied",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: 1}},
			perset: func(pc *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal 1
				err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{Parameters: map[int32][]byte{1: []byte("1234")}})
				if err != nil {
					return nil, err
				}

				// set proposal controller with valid data active proposals/params
				return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
					ActiveProposals: map[uint32]*kapps.ActiveProposals{
						1: {ProposalIDs: []uint64{1}},
					},
				})
			},
			check: map[int32][]byte{1: []byte("50000000000")},
			err:   nil,
		},
		{
			name: "Proposal Approved",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: 1}},
			perset: func(pc *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal 1
				err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{Parameters: map[int32][]byte{1: []byte("1234")}, Votes: map[int32]int64{int32(kapps.ProposalData_VoteDetail_Yes): 10}})
				if err != nil {
					return nil, err
				}

				// set proposal controller with valid data active proposals/params
				return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
					ActiveProposals: map[uint32]*kapps.ActiveProposals{
						1: {ProposalIDs: []uint64{1}},
					},
				})
			},
			check: map[int32][]byte{1: []byte("1234")},
			err:   nil,
		},
		{
			name: "Proposal Approved InvalidValue",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: 1}},
			perset: func(pc *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				// set proposal 1
				err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{Parameters: map[int32][]byte{1: []byte("invalid")}, Votes: map[int32]int64{int32(kapps.ProposalData_VoteDetail_Yes): 10}})
				if err != nil {
					return nil, err
				}

				// set proposal controller with valid data active proposals/params
				return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
					ActiveProposals: map[uint32]*kapps.ActiveProposals{
						1: {ProposalIDs: []uint64{1}},
					},
				})
			},
			check: map[int32][]byte{1: []byte("50000000000")},
			err:   "strconv.ParseInt: parsing \"invalid\": invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := createMockMetaArguments()
			mpp, err := blproc.NewMetaProcessor(arguments)
			require.NoError(t, err)
			mp := blproc.NewMetaProcessorForTests(mpp)

			SetProposal(t, mp, tt.perset)

			err = mp.ProcessProposalsEndOfEpoch(tt.blk)
			if tt.err != nil {
				if str, ok := tt.err.(string); ok {
					assert.True(t, strings.Contains(err.Error(), str), fmt.Sprintf("expected %s to contain %s", err.Error(), str))
				} else {
					assert.Equal(t, tt.err, err, "expected error equal")
				}
			} else {
				assert.Nil(t, err, "expected no error")
			}

			for k, v := range tt.check {
				assert.Equal(t, v, mp.GetActiveParameters()[k].Value)
			}
		})
	}
}

func SetProposal(
	t *testing.T,
	mp *blproc.MetaProcessorForTests,
	exec func(*kapps.ProposalController, state.KAppAccountHandler, *blproc.MetaProcessorForTests) ([]byte, error),
) {
	adapter := mp.GetKAppAdapter()

	acnt, err := adapter.LoadAccount(kapps.ProposalKAppAddress)
	require.NoError(t, err)

	proposalKApp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		t.Error("could not load proposal kapp")
	}

	controllerBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(kdautils.ProposalControllerKey)
	require.NoError(t, err)

	currentProposal := &kapps.ProposalController{}
	err = mp.GetMarshalizer().Unmarshal(currentProposal, controllerBytes)
	require.NoError(t, err)

	result, err := exec(currentProposal, proposalKApp, mp)
	require.NoError(t, err)

	err = proposalKApp.DataTrieTracker().SaveKeyValue(kdautils.ProposalControllerKey, result)
	require.NoError(t, err)

	// set proposal controller
	require.NoError(t, adapter.SaveAccount(proposalKApp))
}

func createShardedDataChacherNotifier(
	handler data.TransactionHandler,
	testHash []byte,
) func() retriever.ShardedDataCacherNotifier {
	return func() retriever.ShardedDataCacherNotifier {
		return &mock.ShardedDataStub{
			ShardDataStoreCalled: func(id string) (c storage.Cacher) {
				return &mock.CacherStub{
					PeekCalled: func(key []byte) (value interface{}, ok bool) {
						if reflect.DeepEqual(key, testHash) {
							return handler, true
						}
						return nil, false
					},
					KeysCalled: func() [][]byte {
						return [][]byte{[]byte("key1"), []byte("key2")}
					},
					LenCalled: func() int {
						return 0
					},
					MaxSizeCalled: func() int {
						return 1000
					},
				}
			},
			RemoveSetOfDataFromPoolCalled: func(keys [][]byte, id string) {},
			SearchFirstDataCalled: func(key []byte) (value interface{}, ok bool) {
				if reflect.DeepEqual(key, []byte("tx1_hash")) {
					return handler, true
				}
				return nil, false
			},
			AddDataCalled: func(key []byte, data interface{}, sizeInBytes int, cacheId string) {
			},
		}
	}
}

func initDataPool(testHash []byte) *mock.PoolsHolderStub {
	txCalled := createShardedDataChacherNotifier(&transaction.Transaction{}, testHash)

	sdp := &mock.PoolsHolderStub{
		TransactionsCalled: txCalled,
		BlocksCalled: func() storage.Cacher {
			return &mock.CacherStub{
				GetCalled: func(key []byte) (value interface{}, ok bool) {
					if reflect.DeepEqual(key, []byte("tx1_hash")) {
						return &transaction.Transaction{}, true
					}
					return nil, false
				},
				KeysCalled: func() [][]byte {
					return nil
				},
				LenCalled: func() int {
					return 0
				},
				MaxSizeCalled: func() int {
					return 1000
				},
				PeekCalled: func(key []byte) (value interface{}, ok bool) {
					if reflect.DeepEqual(key, []byte("tx1_hash")) {
						return &transaction.Transaction{}, true
					}
					return nil, false
				},
				RegisterHandlerCalled: func(i func(key []byte, value interface{})) {},
				RemoveCalled:          func(key []byte) {},
			}
		},
		HeadersCalled: func() retriever.HeadersPool {
			cs := &mock.HeadersCacherStub{}
			cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {
			}
			cs.GetHeaderByHashCalled = func(hash []byte) (data.HeaderHandler, error) {
				return nil, process.ErrMissingHeader
			}
			cs.RemoveHeaderByHashCalled = func(key []byte) {
			}
			cs.LenCalled = func() int {
				return 0
			}
			cs.MaxSizeCalled = func() int {
				return 1000
			}
			cs.NoncesCalled = func() []uint64 {
				return nil
			}
			return cs
		},
	}

	return sdp
}

func initStore() *retriever.ChainStorer {
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.TransactionUnit, generateTestUnit())
	store.AddStorer(retriever.BlockUnit, generateTestUnit())
	store.AddStorer(retriever.HdrNonceHashDataUnit, generateTestUnit())
	return store
}
func generateTestCache() storage.Cacher {
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000, Shards: 1, SizeInBytes: 0})
	return cache
}

func generateTestUnit() storage.Storer {
	storer, _ := storageUnit.NewStorageUnit(
		generateTestCache(),
		memorydb.New(),
	)

	return storer
}

func haveTime() time.Duration {
	return 2000 * time.Millisecond
}

func TestMetaProcessor_ProcessWithHeaderNotCorrectPrevHashShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	blkc := blockchain.NewBlockChain()
	_ = blkc.SetCurrentBlockHeader(
		&block.Block{
			Header: &block.BlockHeader{
				Slot:  1,
				Nonce: 1,
			},
		},
	)
	_ = blkc.SetGenesisHeader(&block.Block{
		Header: &block.BlockHeader{Nonce: 0},
	})
	arguments.BlockChain = blkc
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	hdr := &block.Block{
		Header: &block.BlockHeader{
			Slot:       2,
			Nonce:      2,
			ParentHash: []byte("X"),
		},
	}
	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrBlockHashDoesNotMatch, err)
}

func TestMetaProcessor_CreateEpochStartBodyWithInvalidTxCountShouldErr(t *testing.T) {
	t.Parallel()

	hdr := createMetaBlockHeader()

	arguments := createMockMetaArguments()

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	err = mp.CreateEpochStartHeader(hdr)
	require.Error(t, err, "invalid block tx count")
}

func TestMetaProcessor_CreateAndProcessWithInvalidProcess(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()

	arguments.ArgBaseProcessor.TxCoordinator = &mock.TransactionCoordinatorMock{
		ProcessBlockTransactionsCalled: func(blk *block.Block, timeRemaining func() time.Duration, isImportDB bool) (data.ProcessResults, error) {
			// Invalid ProcessResult
			return nil, nil
		},
		CreateAndProcessBlockTransactionsCalled: func(blk *block.Block, haveTime func() bool) (data.ProcessResults, error) {
			return nil, assert.AnError
		},
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.Nil(t, err)

	pc, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(pc)
	metaHdr := &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1}}

	_, err = mp.CreateBlock(metaHdr, func() bool { return true })
	assert.Nil(t, err)
	err = mp.ProcessBlock(metaHdr, haveTime)
	assert.Nil(t, err)
}

func TestMetaProcessor_UpdateState_ImportDbMode_SkipsEpochSnapshot(t *testing.T) {
	t.Parallel()

	snapshotCallCount := 0

	// Create mock accounts that track SnapshotState calls
	userAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("rootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}
	peerAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("validatorsRootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}
	kappAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("kappsRootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}

	arguments := createMockMetaArguments()
	arguments.ProcessingMode = core.ImportDb
	arguments.AccountsDB[state.UserAccountsState] = userAccountsStub
	arguments.AccountsDB[state.PeerAccountsState] = peerAccountsStub
	arguments.AccountsDB[state.KAppAccountsState] = kappAccountsStub

	// Set up the data pool to return the previous header
	prevHeader := &block.Block{
		Header: &block.BlockHeader{
			Nonce:    0,
			Slot:     0,
			TrieRoot: []byte("prevRootHash"),
		},
	}
	prevHeaderHash := []byte("prevHeaderHash")

	arguments.DataPool.(*mock.PoolsHolderStub).HeadersCalled = func() retriever.HeadersPool {
		return &mock.HeadersCacherStub{
			GetHeaderByHashCalled: func(hash []byte) (data.HeaderHandler, error) {
				if bytes.Equal(hash, prevHeaderHash) {
					return prevHeader, nil
				}
				return nil, errors.New("not found")
			},
		}
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	mpTest := blproc.NewMetaProcessorForTests(mp)

	// Create an epoch start header
	epochStartHeader := &block.Block{
		Header: &block.BlockHeader{
			Nonce:              1,
			Slot:               1,
			ParentHash:         prevHeaderHash,
			TrieRoot:           []byte("newRootHash"),
			ValidatorsTrieRoot: []byte("validatorsRoot"),
			KAppsTrieRoot:      []byte("kappsRoot"),
			IsEpochStart:       true,
			Epoch:              1,
		},
	}

	// Call updateState - in ImportDb mode, snapshots should be skipped
	mpTest.UpdateState(epochStartHeader)

	// Verify SnapshotState was NOT called
	assert.Equal(t, 0, snapshotCallCount, "SnapshotState should NOT be called in import-db mode during epoch start")
}

func TestMetaProcessor_UpdateState_NormalMode_CallsEpochSnapshot(t *testing.T) {
	t.Parallel()

	snapshotCallCount := 0

	// Create mock accounts that track SnapshotState calls
	userAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("rootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}
	peerAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("validatorsRootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}
	kappAccountsStub := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return []byte("kappsRootHash"), nil
		},
		SnapshotStateCalled: func(rootHash []byte) {
			snapshotCallCount++
		},
		IsPruningEnabledCalled: func() bool {
			return true
		},
	}

	arguments := createMockMetaArguments()
	arguments.ProcessingMode = core.Normal // Normal mode
	arguments.AccountsDB[state.UserAccountsState] = userAccountsStub
	arguments.AccountsDB[state.PeerAccountsState] = peerAccountsStub
	arguments.AccountsDB[state.KAppAccountsState] = kappAccountsStub

	// Set up the data pool to return the previous header
	prevHeader := &block.Block{
		Header: &block.BlockHeader{
			Nonce:    0,
			Slot:     0,
			TrieRoot: []byte("prevRootHash"),
		},
	}
	prevHeaderHash := []byte("prevHeaderHash")

	arguments.DataPool.(*mock.PoolsHolderStub).HeadersCalled = func() retriever.HeadersPool {
		return &mock.HeadersCacherStub{
			GetHeaderByHashCalled: func(hash []byte) (data.HeaderHandler, error) {
				if bytes.Equal(hash, prevHeaderHash) {
					return prevHeader, nil
				}
				return nil, errors.New("not found")
			},
		}
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	mpTest := blproc.NewMetaProcessorForTests(mp)

	// Create an epoch start header
	epochStartHeader := &block.Block{
		Header: &block.BlockHeader{
			Nonce:              1,
			Slot:               1,
			ParentHash:         prevHeaderHash,
			TrieRoot:           []byte("newRootHash"),
			ValidatorsTrieRoot: []byte("validatorsRoot"),
			KAppsTrieRoot:      []byte("kappsRoot"),
			IsEpochStart:       true,
			Epoch:              1,
		},
	}

	// Call updateState - in Normal mode, snapshots should be called
	mpTest.UpdateState(epochStartHeader)

	// Verify SnapshotState WAS called for all 3 account types (User, Peer, KApp)
	assert.Equal(t, 3, snapshotCallCount, "SnapshotState should be called 3 times in normal mode during epoch start")
}

func TestMetaProcessor_CommitBlockCallsEventsProcessor(t *testing.T) {
	t.Parallel()

	mdp := initDataPool([]byte("tx_hash"))
	rootHash := []byte("rootHash")
	hdr := createMetaBlockHeader()
	hdr.Header.PrevRandSeed = []byte("prevRandSeed")
	hdr.PubKeysBitmap = []byte{0x01}

	marshalerMock := &mock.MarshalizerMock{}
	accounts := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return rootHash, nil
		},
		RootHashCalled: func() ([]byte, error) {
			return rootHash, nil
		},
	}
	fd := &mock.ForkDetectorMock{
		AddHeaderCalled: func(header data.HeaderHandler, hash []byte, st process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
			return nil
		},
		GetHighestFinalBlockNonceCalled: func() uint64 {
			return 0
		},
	}
	hdrUnit := &mock.StorerStub{
		PutCalled: func(key, d []byte) error {
			return nil
		},
		GetCalled: func(key []byte) ([]byte, error) {
			hdrBuff, _ := marshalerMock.Marshal(&block.Block{})
			return hdrBuff, nil
		},
	}
	store := initStore()
	store.AddStorer(retriever.BlockUnit, hdrUnit)

	var saveBlockCalled bool
	eventsProc := &mock.EventsProcessorStub{
		EnabledCalled: func() bool { return true },
		SaveBlockCalled: func(args *indexer.ArgsSaveBlockData) {
			saveBlockCalled = true
		},
	}

	arguments := createMockMetaArguments()
	arguments.EventsProcessor = eventsProc
	arguments.DataPool = mdp
	arguments.AccountsDB[state.UserAccountsState] = accounts
	arguments.AccountsDB[state.PeerAccountsState] = accounts
	arguments.ForkDetector = fd
	arguments.Store = store
	arguments.Hasher = &mock.HasherStub{
		ComputeCalled: func(s string) []byte {
			return nil
		},
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	mdp.HeadersCalled = func() retriever.HeadersPool {
		cs := &mock.HeadersCacherStub{}
		cs.RegisterHandlerCalled = func(i func(header data.HeaderHandler, key []byte)) {}
		cs.GetHeaderByHashCalled = func(hash []byte) (data.HeaderHandler, error) {
			return &block.Block{}, nil
		}
		cs.LenCalled = func() int { return 0 }
		cs.MaxSizeCalled = func() int { return 1000 }
		cs.NoncesCalled = func() []uint64 { return nil }
		return cs
	}

	err = mp.CommitBlock(hdr)
	assert.Nil(t, err)
	assert.True(t, saveBlockCalled)
}

type epochAwareRequestHandler struct {
	mock.RequestHandlerStub
	epoch uint32
}

func (rh *epochAwareRequestHandler) SetEpoch(epoch uint32) {
	rh.epoch = epoch
}

// processBlockTestDeps groups the collaborators wired into the processor under test.
type processBlockTestDeps struct {
	currentHeader  data.HeaderHandler
	currentHash    []byte
	trigger        process.EpochStartTriggerHandler
	requestHandler *epochAwareRequestHandler
	hook           *contextmock.BlockchainHookStub
	epochNotifier  process.EpochNotifier
	txCoordinator  process.TransactionCoordinator
	reverts        *int
}

func newEpochStartTriggerForProcessBlock(
	t *testing.T,
	forkController core.ForkController,
	epoch uint32,
	epochStartSlot uint64,
	slotsPerEpoch uint64,
) process.EpochStartTriggerHandler {
	t.Helper()
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.BootstrapUnit, createMemUnit())
	store.AddStorer(retriever.BlockUnit, createMemUnit())
	trigger, err :=
		epochStart.NewEpochStartTrigger(&epochStart.ArgsNewEpochStartTrigger{
			GenesisTime:        time.Unix(0, 0),
			Epoch:              epoch,
			EpochStartSlot:     epochStartSlot,
			SlotsPerEpoch:      slotsPerEpoch,
			EpochStartNotifier: notifier.NewEpochStartSubscriptionHandler(),
			Marshalizer:        marshalizer,
			Hasher:             &sha256.Sha256{},
			Storage:            store,
			ForkController:     forkController,
		})
	require.NoError(t, err)
	return trigger
}

func newMetaProcessorForProcessBlock(t *testing.T, deps processBlockTestDeps) process.BlockProcessor {
	t.Helper()
	args := createMockMetaArguments()
	args.BlockChain = &mock.BlockChainMock{
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0}}
		},
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return deps.currentHeader
		},
		GetCurrentBlockHeaderHashCalled: func() []byte {
			return deps.currentHash
		},
	}
	args.EpochStartTrigger = deps.trigger
	args.RequestHandler = deps.requestHandler
	args.BlockChainHook = deps.hook
	args.EpochNotifier = deps.epochNotifier
	if deps.txCoordinator != nil {
		args.TxCoordinator = deps.txCoordinator
	}
	for key := range args.AccountsDB {
		args.AccountsDB[key] = &mock.AccountsStub{
			JournalLenCalled: func() int {
				return 0
			},
			RevertToSnapshotCalled: func(snapshot int) error {
				(*deps.reverts)++
				return nil
			},
			RootHashCalled: func() ([]byte, error) {
				return nil, nil
			},
		}
	}
	mp, err := blproc.NewMetaProcessor(args)
	require.NoError(t, err)
	return mp
}

// A block whose epoch is two ahead of the committed header must be rejected
// before ProcessBlock touches any epoch-sensitive state, so no rollback of those
// components is even needed.
func TestMetaProcessor_ProcessBlockRejectsEpochTwoAheadWithoutMutatingState(t *testing.T) {
	const (
		currentEpoch   = uint32(7)
		epochStartSlot = uint64(100)
		currentSlot    = uint64(110)
		futureSlot     = uint64(250)
		currentNonce   = uint64(11)
		slotsPerEpoch  = uint64(100)
	)
	currentHash := []byte("current-hash")
	currentRandSeed := []byte("current-rand-seed")
	currentHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:    currentEpoch,
		Slot:     currentSlot,
		Nonce:    currentNonce,
		RandSeed: currentRandSeed,
	}}
	epochNotifier := notifier.NewGenericEpochNotifier()
	epochNotifier.CheckEpoch(currentEpoch)
	requestHandler := &epochAwareRequestHandler{}
	var hookHeader data.HeaderHandler
	hook := &contextmock.BlockchainHookStub{
		SetCurrentHeaderCalled: func(hdr data.HeaderHandler) {
			hookHeader = hdr
		},
	}
	args := createMockMetaArguments()
	trigger := newEpochStartTriggerForProcessBlock(t, args.ForkController, currentEpoch,
		epochStartSlot, slotsPerEpoch)
	reverts := 0
	mp := newMetaProcessorForProcessBlock(t, processBlockTestDeps{
		currentHeader:  currentHeader,
		currentHash:    currentHash,
		trigger:        trigger,
		requestHandler: requestHandler,
		hook:           hook,
		epochNotifier:  epochNotifier,
		reverts:        &reverts,
	})
	futureEpochHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:              currentEpoch + 2,
		Slot:               futureSlot,
		Nonce:              currentNonce + 1,
		ParentHash:         currentHash,
		PrevRandSeed:       currentRandSeed,
		IsEpochStart:       true,
		PrevEpochStartSlot: epochStartSlot,
	}}
	err := mp.ProcessBlock(futureEpochHeader, haveTime)
	require.ErrorIs(t, err, process.ErrEpochDoesNotMatch)

	assert.Equal(t, currentEpoch, epochNotifier.CurrentEpoch(), "epoch notifier must not be advanced by a rejected future-epoch block")
	assert.NotEqual(t, currentEpoch+2, requestHandler.epoch, "request handler epoch must not be advanced by a rejected future-epoch block")
	assert.Nil(t, hookHeader, "blockchain hook must not keep the rejected future header")
	assert.Equal(t, currentEpoch, trigger.Epoch(), "epoch start trigger must not be advanced by the rejected future slot")
	assert.False(t, trigger.IsEpochStart(), "epoch start trigger must not enter epoch start state for a rejected block")
	assert.Equal(t, 0, reverts, "block is rejected before any state mutation, so no account revert is needed")
}

// A currentEpoch+1 epoch-start block advances the trigger and reaches processing
// after checkEpochCorrectness. If a later processing step fails, the epoch-side
// state must be restored to the committed epoch instead of staying advanced.
func TestMetaProcessor_ProcessBlockRestoresEpochStateOnLateFailure(t *testing.T) {
	const (
		currentEpoch   = uint32(7)
		epochStartSlot = uint64(100)
		currentSlot    = uint64(110)
		futureSlot     = uint64(250)
		currentNonce   = uint64(11)
		slotsPerEpoch  = uint64(100)
	)
	currentHash := []byte("current-hash")
	currentRandSeed := []byte("current-rand-seed")
	currentHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:    currentEpoch,
		Slot:     currentSlot,
		Nonce:    currentNonce,
		RandSeed: currentRandSeed,
	}}
	epochNotifier := notifier.NewGenericEpochNotifier()
	epochNotifier.CheckEpoch(currentEpoch)
	// Register a fork-flag-like subscriber whose config differs between currentEpoch
	// and currentEpoch+1. The whole point of the rollback is that a rejected
	// future-epoch block does not leave such flags stuck in the future config, so we
	// drive a flag through the notifier and assert it toggles back on the failure path
	// (CurrentEpoch() resetting alone would not prove the derived flags reverted).
	futureFlagEnabled := false
	sawFutureConfig := false
	epochNotifier.RegisterNotifyHandler(&mock.EpochSubscriberHandlerStub{
		EpochConfirmedCalled: func(epoch uint32) {
			futureFlagEnabled = epoch >= currentEpoch+1
			if futureFlagEnabled {
				sawFutureConfig = true
			}
		},
	})
	require.False(t, futureFlagEnabled, "fork flag must start in the committed-epoch config")
	requestHandler := &epochAwareRequestHandler{}
	requestHandler.SetEpoch(currentEpoch)
	hook := &contextmock.BlockchainHookStub{}
	args := createMockMetaArguments()
	trigger := newEpochStartTriggerForProcessBlock(t, args.ForkController, currentEpoch,
		epochStartSlot, slotsPerEpoch)
	reverts := 0
	reachedPostEpochCheck := false
	txCoordinator := &mock.TransactionCoordinatorMock{
		RequestBlockTransactionsCalled: func(blk *block.Block) {
			reachedPostEpochCheck = true
		},
		IsDataPreparedForProcessingCalled: func(haveTime func() time.Duration) error {
			return assert.AnError
		},
	}
	mp := newMetaProcessorForProcessBlock(t, processBlockTestDeps{
		currentHeader:  currentHeader,
		currentHash:    currentHash,
		trigger:        trigger,
		requestHandler: requestHandler,
		hook:           hook,
		epochNotifier:  epochNotifier,
		txCoordinator:  txCoordinator,
		reverts:        &reverts,
	})
	futureEpochHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:              currentEpoch + 1,
		Slot:               futureSlot,
		Nonce:              currentNonce + 1,
		ParentHash:         currentHash,
		PrevRandSeed:       currentRandSeed,
		IsEpochStart:       true,
		PrevEpochStartSlot: epochStartSlot,
	}}
	err := mp.ProcessBlock(futureEpochHeader, haveTime)
	require.ErrorIs(t, err, assert.AnError)
	require.True(t, reachedPostEpochCheck, "future epoch-start header should reach processing after checkEpochCorrectness")

	assert.Equal(t, currentEpoch, trigger.Epoch(), "epoch start trigger must be restored to the committed epoch after a late failure")
	assert.False(t, trigger.IsEpochStart(), "epoch start trigger must be restored out of epoch start state after a late failure")
	assert.Equal(t, currentEpoch, epochNotifier.CurrentEpoch(), "epoch notifier must be restored to the committed epoch after a late failure")
	assert.Equal(t, currentEpoch, requestHandler.epoch, "request handler epoch must be restored to the committed epoch after a late failure")
	assert.True(t, sawFutureConfig, "the future-epoch block must have advanced the fork flag before failing, otherwise the revert assertion is vacuous")
	assert.False(t, futureFlagEnabled, "fork flags derived from the epoch notifier must revert to the committed-epoch config after a late failure")
}

// The time-out check returns its error directly (not through the err variable),
// so the deferred cleanup must still restore the epoch state advanced earlier in
// ProcessBlock. This guards the named-return wiring that funnels every exit
// through the rollback handler.
func TestMetaProcessor_ProcessBlockRestoresEpochStateWhenTimeIsOut(t *testing.T) {
	const (
		currentEpoch   = uint32(7)
		epochStartSlot = uint64(100)
		currentSlot    = uint64(110)
		futureSlot     = uint64(250)
		currentNonce   = uint64(11)
		slotsPerEpoch  = uint64(100)
	)
	currentHash := []byte("current-hash")
	currentRandSeed := []byte("current-rand-seed")
	currentHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:    currentEpoch,
		Slot:     currentSlot,
		Nonce:    currentNonce,
		RandSeed: currentRandSeed,
	}}
	epochNotifier := notifier.NewGenericEpochNotifier()
	epochNotifier.CheckEpoch(currentEpoch)
	requestHandler := &epochAwareRequestHandler{}
	requestHandler.SetEpoch(currentEpoch)
	hook := &contextmock.BlockchainHookStub{}
	args := createMockMetaArguments()
	trigger := newEpochStartTriggerForProcessBlock(t, args.ForkController, currentEpoch,
		epochStartSlot, slotsPerEpoch)
	reverts := 0
	reachedPostEpochCheck := false
	txCoordinator := &mock.TransactionCoordinatorMock{
		RequestBlockTransactionsCalled: func(blk *block.Block) {
			reachedPostEpochCheck = true
		},
	}
	mp := newMetaProcessorForProcessBlock(t, processBlockTestDeps{
		currentHeader:  currentHeader,
		currentHash:    currentHash,
		trigger:        trigger,
		requestHandler: requestHandler,
		hook:           hook,
		epochNotifier:  epochNotifier,
		txCoordinator:  txCoordinator,
		reverts:        &reverts,
	})
	futureEpochHeader := &block.Block{Header: &block.BlockHeader{
		Epoch:              currentEpoch + 1,
		Slot:               futureSlot,
		Nonce:              currentNonce + 1,
		ParentHash:         currentHash,
		PrevRandSeed:       currentRandSeed,
		IsEpochStart:       true,
		PrevEpochStartSlot: epochStartSlot,
	}}
	timeIsOut := func() time.Duration { return -1 }

	err := mp.ProcessBlock(futureEpochHeader, timeIsOut)
	require.ErrorIs(t, err, process.ErrTimeIsOut)
	require.True(t, reachedPostEpochCheck, "future epoch-start header should advance the trigger before the time-out check")

	assert.Equal(t, currentEpoch, trigger.Epoch(), "epoch start trigger must be restored when processing times out")
	assert.False(t, trigger.IsEpochStart(), "epoch start trigger must leave epoch start state when processing times out")
	assert.Equal(t, currentEpoch, epochNotifier.CurrentEpoch(), "epoch notifier must be restored when processing times out")
	assert.Equal(t, currentEpoch, requestHandler.epoch, "request handler epoch must be restored when processing times out")
}
