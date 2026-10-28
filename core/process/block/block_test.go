package block_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	"google.golang.org/protobuf/encoding/protowire"
)

var marshalizer = marshal.NewProtoMarshalizer()

// testChronologySlotIndex is the slot the mocked chronology reports. It is well
// above every slot used by the fixtures so that the future-slot guard (KLR-39)
// only fires in the tests that target it explicitly.
const testChronologySlotIndex = int64(1000)

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
			// the chronology is far enough ahead that the future-slot guard
			// (KLR-39) never fires on the small slot numbers used by fixtures
			SlotManager: &consensusMock.SlotManagerMock{
				IndexCalled: func() int64 { return testChronologySlotIndex },
			},
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

// createEpochStartHeaderForProcess builds the minimal epoch start header accepted
// by checkBlockValidity, so ProcessBlock reaches the epoch start branch.
func createEpochStartHeaderForProcess() *block.Block {
	return &block.Block{
		Header: &block.BlockHeader{
			Nonce:        1,
			ParentHash:   []byte(""),
			TrieRoot:     []byte("roothash"),
			IsEpochStart: true,
		},
		ProducerSignature: []byte("signature"),
	}
}

// Pins that every failure mode of the epoch start branch reaches the rollback
// defer of ProcessBlock, so a rejected epoch start block leaves no accounts
// snapshot behind.
func TestMetaProcessor_ProcessBlockEpochStartErrorShouldRevertState(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("epoch start failure")

	tests := []struct {
		name        string
		expectedErr error
		prepare     func(hdr *block.Block, arguments *blproc.ArgMetaProcessor)
	}{
		{
			name:        "invalid tx count",
			expectedErr: process.ErrInvalidTXCount,
			prepare: func(hdr *block.Block, _ *blproc.ArgMetaProcessor) {
				hdr.Header.TxCount = 1
			},
		},
		{
			name:        "invalid tx fees",
			expectedErr: process.ErrInvalidTXFees,
			prepare: func(hdr *block.Block, _ *blproc.ArgMetaProcessor) {
				hdr.Header.TxFees = 1
			},
		},
		{
			name:        "validator statistics root hash fails",
			expectedErr: expectedErr,
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					RootHashCalled: func() ([]byte, error) {
						return nil, expectedErr
					},
				}
			},
		},
		{
			name:        "process ratings end of epoch fails",
			expectedErr: expectedErr,
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					ProcessRatingsEndOfEpochCalled: func(_ []*state.ValidatorInfo, _ uint32) error {
						return expectedErr
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			revertCalls := 0
			hdr := createEpochStartHeaderForProcess()

			arguments := createMockMetaArguments()
			arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
				JournalLenCalled: func() int { return 0 },
				RevertToSnapshotCalled: func(_ int) error {
					revertCalls++
					return nil
				},
			}
			tt.prepare(hdr, &arguments)

			mp, err := blproc.NewMetaProcessor(arguments)
			require.NoError(t, err)

			controller, _ := kapps.NewProposalController(arguments.ForkController)
			_ = mp.SetProposalController(controller)

			err = mp.ProcessBlock(hdr, haveTime)
			require.Equal(t, tt.expectedErr, err)
			assert.Positive(t, revertCalls, "epoch start error must trigger RevertStateToSnapshot")
		})
	}
}

// Without the rollback the accounts journal stays dirty after a failed epoch
// start block, and every subsequent block is rejected with ErrAccountStateDirty
// - a liveness DoS on the node.
func TestMetaProcessor_ProcessBlockEpochStartErrorShouldNotLeaveAccountsDirty(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("epoch start failure")
	journalLen := 0

	arguments := createMockMetaArguments()
	arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
		JournalLenCalled: func() int { return journalLen },
		RevertToSnapshotCalled: func(_ int) error {
			journalLen = 0
			return nil
		},
	}
	arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
		ProcessRatingsEndOfEpochCalled: func(_ []*state.ValidatorInfo, _ uint32) error {
			// stand in for the account mutations performed by the epoch start
			// chain (ratings, proposals, economics) before the failure
			journalLen = 5
			return expectedErr
		},
	}

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	err = mp.ProcessBlock(createEpochStartHeaderForProcess(), haveTime)
	require.Equal(t, expectedErr, err)
	require.Zero(t, journalLen, "failed epoch start block must leave a clean accounts journal")

	// the next block must not be rejected because of leftover dirty state
	nextHdr := createEpochStartHeaderForProcess()
	nextHdr.Header.IsEpochStart = false

	err = mp.ProcessBlock(nextHdr, haveTime)
	assert.NotEqual(t, process.ErrAccountStateDirty, err)
}

// SetEpochValidatorsInfo writes an in-memory map on the nodes coordinator and
// UpdateProposalsAndParameters notifies the indexer. Neither is covered by
// RevertStateToSnapshot, so a rejected epoch start block must not reach them.
func TestMetaProcessor_ProcessBlockEpochStartErrorShouldNotApplySideEffects(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("epoch start failure")

	// every failure point of the epoch start chain, from the earliest step of
	// baseProcessEpochStartBlock down to the last trie root verification: none of
	// them may reach the nodes coordinator or the indexer
	tests := []struct {
		name        string
		errContains string
		errIs       []error
		errIsNot    []error
		prepare     func(hdr *block.Block, arguments *blproc.ArgMetaProcessor)
	}{
		{
			name:        "get validator info for root hash fails",
			errContains: expectedErr.Error(),
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					GetValidatorInfoForRootHashCalled: func(_ []byte) ([]*state.ValidatorInfo, error) {
						return nil, expectedErr
					},
				}
			},
		},
		{
			name: "process proposals end of epoch fails",
			// the proposal kapp account is never seeded, so getProposalController
			// fails while unmarshalling an empty controller payload
			errContains: "cannot parse invalid wire-format data",
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.AccountsDB[state.KAppAccountsState] = &mock.AccountsStub{
					LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
						return nil, errors.New("cannot parse invalid wire-format data")
					},
				}
			},
		},
		{
			name:        "process economics end of epoch fails",
			errContains: common.ErrEmptyString.Error(),
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				// wipe the staked KLV entry, so rolling the FPR fails
				adapter := arguments.AccountsDB[state.KAppAccountsState]
				acnt, err := adapter.LoadAccount(kapps.StakingKAppAddress)
				require.NoError(t, err)

				stakingKApp, ok := acnt.(state.KAppAccountHandler)
				require.True(t, ok)
				require.NoError(t, stakingKApp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, nil))
				require.NoError(t, adapter.SaveAccount(stakingKApp))
			},
		},
		{
			name:        "reset validator statistics at new epoch fails",
			errContains: expectedErr.Error(),
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					ResetValidatorStatisticsAtNewEpochCalled: func(_ []*state.ValidatorInfo) ([]*state.ValidatorInfo, error) {
						return nil, expectedErr
					},
				}
			},
		},
		{
			name:        "account trie root does not match",
			errContains: process.ErrRootStateDoesNotMatch.Error(),
			errIs:       []error{process.ErrRootStateDoesNotMatch, process.ErrBlockExecutionNotMatchingHeader},
			prepare: func(hdr *block.Block, _ *blproc.ArgMetaProcessor) {
				hdr.Header.TrieRoot = []byte("another root hash")
			},
		},
		{
			name:        "validator statistics trie root does not match",
			errContains: expectedErr.Error(),
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					UpdatePeerStateCalled: func(_ data.HeaderHandler, _ data.HeaderHandler) ([]byte, error) {
						return nil, expectedErr
					},
				}
			},
		},
		{
			name:        "validator statistics root hash does not match the header",
			errContains: process.ErrValidatorStatsRootHashDoesNotMatch.Error(),
			errIs:       []error{process.ErrValidatorStatsRootHashDoesNotMatch},
			errIsNot:    []error{process.ErrBlockExecutionNotMatchingHeader},
			prepare: func(_ *block.Block, arguments *blproc.ArgMetaProcessor) {
				arguments.ValidatorStatisticsProcessor = &mock.ValidatorStatisticsProcessorStub{
					UpdatePeerStateCalled: func(_ data.HeaderHandler, _ data.HeaderHandler) ([]byte, error) {
						return []byte("another validators root hash"), nil
					},
				}
			},
		},
		{
			name:        "kapp trie root does not match",
			errContains: process.ErrRootStateDoesNotMatch.Error(),
			errIs:       []error{process.ErrRootStateDoesNotMatch, process.ErrBlockExecutionNotMatchingHeader},
			prepare: func(hdr *block.Block, _ *blproc.ArgMetaProcessor) {
				// verifyKAppRootHash is the last check before the side effects
				hdr.Header.KAppsTrieRoot = []byte("another kapp root hash")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			setEpochValidatorsCalls := 0
			indexerCalls := 0
			hdr := createEpochStartHeaderForProcess()

			arguments := createMockMetaArguments()
			arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
				JournalLenCalled: func() int { return 0 },
				RootHashCalled: func() ([]byte, error) {
					// matches the header trie root, so verifyStateRootAccount
					// passes and the failure lands after the old
					// SetEpochValidatorsInfo call site
					return []byte("roothash"), nil
				},
				RevertToSnapshotCalled: func(_ int) error { return nil },
			}
			arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
				SetEpochValidatorsInfoCalled: func(_ uint32, _ []*state.ValidatorInfo) error {
					setEpochValidatorsCalls++
					return nil
				},
			}
			arguments.EventsProcessor = &mock.EventsProcessorStub{
				UpdateProposalsAndParametersCalled: func(_ []string) {
					indexerCalls++
				},
			}
			tt.prepare(hdr, &arguments)

			mp, err := blproc.NewMetaProcessor(arguments)
			require.NoError(t, err)

			controller, _ := kapps.NewProposalController(arguments.ForkController)
			_ = mp.SetProposalController(controller)
			_ = arguments.KAppController.SetProposalController(controller)
			_ = arguments.KAppController.GetValidatorsKApp().SetKAppController(arguments.KAppController)

			err = mp.ProcessBlock(hdr, haveTime)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errContains)
			for _, expected := range tt.errIs {
				require.ErrorIs(t, err, expected)
			}
			for _, unexpected := range tt.errIsNot {
				assert.False(t, errors.Is(err, unexpected))
			}

			assert.Zero(t, setEpochValidatorsCalls, "rejected epoch start block must not update the nodes coordinator")
			assert.Zero(t, indexerCalls, "rejected epoch start block must not notify the indexer")
		})
	}
}

// Counterpart of the test above: once the block is verified the side effects
// must still be applied, otherwise the epoch start would be a no-op.
func TestMetaProcessor_ApplyEpochStartSideEffects(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("nodes coordinator failure")

	tests := []struct {
		name             string
		proposalsToIndex []string
		nilEventsProc    bool
		coordinatorErr   error
		expectedErr      error
		expectIndexed    bool
	}{
		{
			name:             "nodes coordinator fails, indexer is not notified",
			proposalsToIndex: []string{"1"},
			coordinatorErr:   expectedErr,
			expectedErr:      expectedErr,
			expectIndexed:    false,
		},
		{
			name:             "no proposals settled, indexer is not notified",
			proposalsToIndex: nil,
			expectIndexed:    false,
		},
		{
			name:             "nil events processor is skipped",
			proposalsToIndex: []string{"1"},
			nilEventsProc:    true,
			expectIndexed:    false,
		},
		{
			name:             "proposals settled, indexer is notified",
			proposalsToIndex: []string{"1", "2"},
			expectIndexed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			indexerCalls := 0

			var gotEpoch uint32
			var gotValidators []*state.ValidatorInfo
			var gotProposals []string

			arguments := createMockMetaArguments()
			arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
				SetEpochValidatorsInfoCalled: func(epoch uint32, validatorsInfo []*state.ValidatorInfo) error {
					gotEpoch = epoch
					gotValidators = validatorsInfo
					return tt.coordinatorErr
				},
			}
			if !tt.nilEventsProc {
				arguments.EventsProcessor = &mock.EventsProcessorStub{
					UpdateProposalsAndParametersCalled: func(proposalIDs []string) {
						indexerCalls++
						gotProposals = proposalIDs
					},
				}
			}

			mpp, err := blproc.NewMetaProcessor(arguments)
			require.NoError(t, err)
			mp := blproc.NewMetaProcessorForTests(mpp)

			hdr := createEpochStartHeaderForProcess()
			hdr.Header.Epoch = 7
			validators := []*state.ValidatorInfo{{PublicKey: []byte("pubkey")}}

			err = mp.ApplyEpochStartSideEffects(hdr, validators, tt.proposalsToIndex)
			require.Equal(t, tt.expectedErr, err)

			// the epoch and the validators info always reach the nodes coordinator
			assert.Equal(t, uint32(7), gotEpoch)
			assert.Equal(t, validators, gotValidators)

			if tt.expectIndexed {
				assert.Equal(t, 1, indexerCalls)
				assert.Equal(t, tt.proposalsToIndex, gotProposals)
			} else {
				assert.Zero(t, indexerCalls)
			}
		})
	}
}

// UpdateProposalsAndParameters used to fire from inside processProposalsEndOfEpoch,
// so a proposal settled by an epoch start block that was later rejected was still
// pushed to the indexer - an external effect no state rollback can undo. The
// settlement must still produce the ids, but only the caller may publish them.
func TestMetaProcessor_RejectedEpochStartWithSettledProposalsShouldNotNotifyIndexer(t *testing.T) {
	t.Parallel()

	// an epoch start header whose kapp trie root cannot match, so the block is
	// rejected by the very last verification of processEpochStartBlock - after
	// the proposals have been settled
	newHeader := func() *block.Block {
		hdr := createEpochStartHeaderForProcess()
		hdr.Header.Epoch = 1
		hdr.Header.KAppsTrieRoot = []byte("mismatching kapp root hash")

		return hdr
	}

	// one active proposal for epoch 1, so the settlement yields a non-empty id list
	seedActiveProposal := func(_ *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
		err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{Parameters: map[int32][]byte{1: []byte("1234")}})
		if err != nil {
			return nil, err
		}

		return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
			ActiveProposals: map[uint32]*kapps.ActiveProposals{
				1: {ProposalIDs: []uint64{1}},
			},
		})
	}

	indexerCalls := 0
	newProcessor := func(t *testing.T) *blproc.MetaProcessorForTests {
		arguments := createMockMetaArguments()
		arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
			JournalLenCalled:       func() int { return 0 },
			RootHashCalled:         func() ([]byte, error) { return []byte("roothash"), nil },
			RevertToSnapshotCalled: func(_ int) error { return nil },
		}
		arguments.EventsProcessor = &mock.EventsProcessorStub{
			UpdateProposalsAndParametersCalled: func(_ []string) { indexerCalls++ },
		}

		mpp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)

		mp := blproc.NewMetaProcessorForTests(mpp)

		controller, _ := kapps.NewProposalController(arguments.ForkController)
		_ = mp.SetProposalController(controller)
		_ = arguments.KAppController.SetProposalController(controller)
		_ = arguments.KAppController.GetValidatorsKApp().SetKAppController(arguments.KAppController)

		SetProposal(t, mp, seedActiveProposal)

		return mp
	}

	// the settlement itself still happens and reports the id to its caller
	proposalsToIndex, err := newProcessor(t).ProcessProposalsEndOfEpoch(newHeader())
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, proposalsToIndex)
	require.Zero(t, indexerCalls, "settling proposals must not notify the indexer by itself")

	// but a block rejected after the settlement never publishes them
	err = newProcessor(t).ProcessBlock(newHeader(), haveTime)
	require.Error(t, err)
	require.Contains(t, err.Error(), process.ErrRootStateDoesNotMatch.Error())
	assert.Zero(t, indexerCalls, "a rejected epoch start block must not notify the indexer")
}

// finalizeProposalUpdates writes the settled parameters straight into the
// in-memory proposalController, which RevertStateToSnapshot does not cover: it
// restores every accounts DB, including the KApp trie copy of the controller,
// but not that map. A rejected epoch start block therefore left the node
// computing rewards and fees from parameters of a block that never committed.
//
// The write stays mid-processing (epochStartNativeStakingKapps reads
// MaxEpochsUnclaimed back out of it in the same run), so ProcessBlock snapshots
// the set up front and restores it on failure.
func TestMetaProcessor_RejectedEpochStartMustRollBackProposalParameters(t *testing.T) {
	t.Parallel()

	const paramKey = int32(1)
	settledValue := []byte("1234")

	// an epoch start header whose kapp trie root cannot match, so the block is
	// rejected after the proposals have already been settled
	newHeader := func() *block.Block {
		hdr := createEpochStartHeaderForProcess()
		hdr.Header.Epoch = 1
		hdr.Header.KAppsTrieRoot = []byte("mismatching kapp root hash")

		return hdr
	}

	seedActiveProposal := func(_ *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
		err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{
			Parameters: map[int32][]byte{paramKey: settledValue},
			Votes:      map[int32]int64{int32(kapps.ProposalData_VoteDetail_Yes): 10},
		})
		if err != nil {
			return nil, err
		}

		return mp.GetMarshalizer().Marshal(&kapps.ProposalController{
			ActiveProposals: map[uint32]*kapps.ActiveProposals{
				1: {ProposalIDs: []uint64{1}},
			},
		})
	}

	// onRevert, when set, runs inside the accounts revert, which the rejection
	// path calls right before it restores the proposal parameters
	newProcessor := func(t *testing.T, onRevert func()) (*blproc.MetaProcessorForTests, kapps.ActiveProposalController) {
		arguments := createMockMetaArguments()
		arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
			JournalLenCalled: func() int { return 0 },
			RootHashCalled:   func() ([]byte, error) { return []byte("roothash"), nil },
			RevertToSnapshotCalled: func(_ int) error {
				if onRevert != nil {
					onRevert()
				}
				return nil
			},
		}

		mpp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)

		mp := blproc.NewMetaProcessorForTests(mpp)

		controller, _ := kapps.NewProposalController(arguments.ForkController)
		_ = mp.SetProposalController(controller)
		_ = arguments.KAppController.SetProposalController(controller)
		_ = arguments.KAppController.GetValidatorsKApp().SetKAppController(arguments.KAppController)

		SetProposal(t, mp, seedActiveProposal)

		return mp, controller
	}

	// settling applies the parameter immediately: readers later in the same
	// epoch start run depend on seeing it
	mp, _ := newProcessor(t, nil)
	initialValue := mp.GetActiveParameters()[paramKey].Value
	require.NotEqual(t, settledValue, initialValue, "the test parameter must actually change")

	_, err := mp.ProcessProposalsEndOfEpoch(newHeader())
	require.NoError(t, err)
	require.Equal(t, settledValue, mp.GetActiveParameters()[paramKey].Value,
		"settling must apply the parameter in place for the rest of the run")

	// but a block rejected afterwards must leave nothing behind
	mp, _ = newProcessor(t, nil)
	err = mp.ProcessBlock(newHeader(), haveTime)
	require.Error(t, err)
	require.Contains(t, err.Error(), process.ErrRootStateDoesNotMatch.Error())
	assert.Equal(t, initialValue, mp.GetActiveParameters()[paramKey].Value,
		"a rejected epoch start block must not leave settled parameters behind")

	// KLR-18 residual: the rollback is a write to the same set the API and the
	// fee path read from other goroutines. Readers must never see a partially
	// restored set, nor a value that was never live. Meaningful under -race.
	t.Run("concurrent API and fee reads during the rollback", func(t *testing.T) {
		// The accounts revert runs right before the restore, with the settled
		// set still live. Holding it until the readers are looping guarantees
		// they overlap the restore instead of relying on the scheduler.
		const minReadsBeforeRestore = 100
		var reads, readsAtRevert atomic.Int64
		waitForReaders := func() {
			deadline := time.Now().Add(5 * time.Second)
			for reads.Load() < minReadsBeforeRestore && time.Now().Before(deadline) {
				runtime.Gosched()
			}
			readsAtRevert.CompareAndSwap(0, reads.Load())
		}

		mp, controller := newProcessor(t, waitForReaders)
		fullLen := len(mp.GetActiveParameters())
		initialInt := controller.GetParameterInt(kapps.EnumParameter(paramKey))
		settledInt := int64(1234)

		// keep only the first failure: the readers loop until ProcessBlock
		// returns, so reporting each one would repeat it thousands of times
		var firstFailure atomic.Pointer[string]
		fail := func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			firstFailure.CompareAndSwap(nil, &msg)
		}

		done := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
					}

					// API path (Node.GetProposalParameters)
					params := mp.GetActiveParameters()
					if len(params) != fullLen {
						fail("API read returned %d parameters, want %d", len(params), fullLen)
						return
					}
					param := params[paramKey]
					if param == nil || (!bytes.Equal(param.Value, initialValue) && !bytes.Equal(param.Value, settledValue)) {
						fail("API read returned parameter %d = %v, want %q or %q", paramKey, param, initialValue, settledValue)
						return
					}

					// fee path (EconomicsData.GetParameterInt)
					value := controller.GetParameterInt(kapps.EnumParameter(paramKey))
					if value != initialInt && value != settledInt {
						fail("fee read returned %d, want %d or %d", value, initialInt, settledInt)
						return
					}

					reads.Add(1)
				}
			}()
		}

		err := mp.ProcessBlock(newHeader(), haveTime)
		close(done)
		wg.Wait()

		if msg := firstFailure.Load(); msg != nil {
			t.Fatal(*msg)
		}
		require.GreaterOrEqual(t, readsAtRevert.Load(), int64(minReadsBeforeRestore),
			"the readers must already be running when the rollback restores the parameters")
		require.Error(t, err)
		require.Contains(t, err.Error(), process.ErrRootStateDoesNotMatch.Error())
		assert.Equal(t, initialValue, mp.GetActiveParameters()[paramKey].Value)
	})
}

// Round trip over an epoch start block: the proposer applies the side effects
// straight away (no trie roots to verify, they are computed from this very
// execution), the validator applies them only after every root has been
// verified. Both must end up with the nodes coordinator updated exactly once.
func TestMetaProcessor_CreateAndProcessEpochStartBlockAppliesSideEffects(t *testing.T) {
	t.Parallel()

	hash := []byte("hash1")
	hasher := &mock.HasherStub{}
	hasher.ComputeCalled = func(_ string) []byte { return hash }

	blkc := &mock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 0}}
		},
		GetCurrentBlockHeaderHashCalled: func() []byte { return hash },
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 0}}
		},
	}

	// two independent nodes, each with its own freshly seeded kapp trie, so the
	// validator replays the epoch start from the same starting state as the
	// proposer instead of on top of the proposer's mutations
	newNode := func(epochs *[]uint32) blproc.ArgMetaProcessor {
		arguments := createMockMetaArguments()
		arguments.Hasher = hasher
		arguments.BlockChain = blkc
		arguments.EpochStartTrigger = &mock.EpochStartTriggerStub{
			IsEpochStartCalled: func() bool { return true },
			EpochCalled:        func() uint32 { return 1 },
		}
		arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
			SetEpochValidatorsInfoCalled: func(epoch uint32, _ []*state.ValidatorInfo) error {
				*epochs = append(*epochs, epoch)
				return nil
			},
		}

		return arguments
	}

	newProcessor := func(t *testing.T, arguments blproc.ArgMetaProcessor) process.BlockProcessor {
		mp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)

		pc, _ := kapps.NewProposalController(arguments.ForkController)
		_ = mp.SetProposalController(pc)
		_ = arguments.KAppController.SetProposalController(pc)
		_ = arguments.KAppController.GetValidatorsKApp().SetKAppController(arguments.KAppController)

		return mp
	}

	proposerEpochs := make([]uint32, 0)
	proposer := newProcessor(t, newNode(&proposerEpochs))

	createdHdr, err := proposer.CreateBlock(&block.Block{Header: &block.BlockHeader{}}, func() bool { return true })
	require.NoError(t, err)
	require.Equal(t, []uint32{1}, proposerEpochs, "the proposer applies the side effects while creating the block")

	hdr, ok := createdHdr.(*block.Block)
	require.True(t, ok)
	require.True(t, hdr.GetIsEpochStart())

	hdr.Header.Nonce = 1
	hdr.Header.Slot = 1
	hdr.Header.ParentHash = hash

	validatorEpochs := make([]uint32, 0)
	validator := newProcessor(t, newNode(&validatorEpochs))

	err = validator.ProcessBlock(hdr, haveTime)
	require.NoError(t, err)
	assert.Equal(t, []uint32{1}, validatorEpochs, "a verified epoch start block must update the nodes coordinator once")
}

// verifyFees is the last check of the epoch start path, but it used to run only
// after processEpochStartBlock had already applied the side effects. A block
// rejected for mismatching rewards had therefore already updated the nodes
// coordinator and notified the indexer - the very bypass this fix closes for the
// three trie roots, left open for the fee check. It is reachable on any current
// epoch start block, since FixStakingBuckets is already active.
func TestMetaProcessor_EpochStartWithMismatchingFeesMustNotApplySideEffects(t *testing.T) {
	t.Parallel()

	hash := []byte("hash1")
	hasher := &mock.HasherStub{}
	hasher.ComputeCalled = func(_ string) []byte { return hash }

	blkc := &mock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 0}}
		},
		GetCurrentBlockHeaderHashCalled: func() []byte { return hash },
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 0}}
		},
	}

	newNode := func(epochs *[]uint32) blproc.ArgMetaProcessor {
		arguments := createMockMetaArguments()
		arguments.Hasher = hasher
		arguments.BlockChain = blkc
		arguments.EpochStartTrigger = &mock.EpochStartTriggerStub{
			IsEpochStartCalled: func() bool { return true },
			EpochCalled:        func() uint32 { return 1 },
		}
		arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
			SetEpochValidatorsInfoCalled: func(epoch uint32, _ []*state.ValidatorInfo) error {
				*epochs = append(*epochs, epoch)
				return nil
			},
		}

		return arguments
	}

	newProcessor := func(t *testing.T, arguments blproc.ArgMetaProcessor) (process.BlockProcessor, kapps.ActiveProposalController) {
		mp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)

		pc, _ := kapps.NewProposalController(arguments.ForkController)
		_ = mp.SetProposalController(pc)
		_ = arguments.KAppController.SetProposalController(pc)
		_ = arguments.KAppController.GetValidatorsKApp().SetKAppController(arguments.KAppController)

		return mp, pc
	}

	proposerEpochs := make([]uint32, 0)
	proposer, _ := newProcessor(t, newNode(&proposerEpochs))

	createdHdr, err := proposer.CreateBlock(&block.Block{Header: &block.BlockHeader{}}, func() bool { return true })
	require.NoError(t, err)

	hdr, ok := createdHdr.(*block.Block)
	require.True(t, ok)
	require.True(t, hdr.GetIsEpochStart())

	hdr.Header.Nonce = 1
	hdr.Header.Slot = 1
	hdr.Header.ParentHash = hash

	validatorEpochs := make([]uint32, 0)
	validator, validatorParams := newProcessor(t, newNode(&validatorEpochs))

	// Disagree with the proposer on BlockRewards only. Tampering with the header
	// instead would move the kapp trie root as well (verifyKAppRootHash feeds the
	// header rewards into UpdateKLVCirculationSupply) and the block would be
	// rejected one check earlier, never reaching verifyFees.
	validatorParams.UpdateParameters(map[int32]*kapps.Parameter{
		int32(kapps.EnumParameter_BlockRewards): {Type: kapps.EnumType_Int64, Value: []byte("123456")},
	})
	require.NotEqual(t, hdr.GetBlockRewards(),
		validatorParams.GetParameterInt(kapps.EnumParameter_BlockRewards),
		"the validator must actually disagree on the rewards")

	err = validator.ProcessBlock(hdr, haveTime)
	require.ErrorIs(t, err, process.ErrInvalidMiningRewards)
	assert.Empty(t, validatorEpochs, "a block rejected by verifyFees must not update the nodes coordinator")
}

// KLR-39: every slot check on the block path was a lower bound, so a header
// dated ahead of the local chronology was processed and committed. Once
// committed it stalls the chain, because validateBlockAgainstCurrentHeader then
// rejects every honest block until the wall clock catches up.
func TestMetaProcessor_ProcessBlockWithFutureSlotShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 1,
			Slot:  uint64(testChronologySlotIndex) + 2,
		},
	}

	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrSlotAheadOfChronology, err)
}

// The next slot is accepted: a header for slot index+1 may legitimately arrive
// before the local chronology has ticked over. This matches the tolerance the
// fork detector already applies in checkBlockBasicValidity. The nonce is
// deliberately wrong so the header is rejected by the following check, which
// proves the slot guard let it through.
func TestMetaProcessor_ProcessBlockWithNextSlotShouldNotBeRejectedAsFuture(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 5,
			Slot:  uint64(testChronologySlotIndex) + 1,
		},
	}

	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrWrongNonceInBlock, err)
}

// Behind the fork flag the guard must be inert, so nodes that have not yet
// activated FixAuditChangesV4 keep the previous behaviour.
func TestMetaProcessor_ProcessBlockWithFutureSlotIsInertBeforeTheFork(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.ForkController = mock.NewForkControllerStub().
		SetFork("FixAuditChangesV4", false)

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	// same future-dated header as the test above, but the nonce is wrong too:
	// with the guard active the slot is reported first, with it inert the header
	// falls through to the pre-existing nonce check
	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 5,
			Slot:  uint64(testChronologySlotIndex) + 2,
		},
	}

	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrWrongNonceInBlock, err)
}

// Past slots stay rejected by the pre-existing lower bound; locked in so the
// new upper bound does not displace it.
func TestMetaProcessor_ProcessBlockWithPastSlotShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	blkc := blockchain.NewBlockChain()
	_ = blkc.SetCurrentBlockHeader(
		&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 10}},
	)
	_ = blkc.SetGenesisHeader(&block.Block{Header: &block.BlockHeader{Nonce: 0}})
	arguments.BlockChain = blkc

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 2,
			Slot:  9,
		},
	}

	err = mp.ProcessBlock(hdr, haveTime)
	assert.Equal(t, process.ErrLowerSlotInBlock, err)
}

// TxResults carries the consensus result code the validator compares its own execution against
// (KLR-63), and a proposer fills it in lockstep with TxHashes. A list of any other length is
// malformed, and an omitted one would skip the comparison entirely, so the header is rejected
// before the transactions are processed - and stays accepted behind the fork flag.
func TestMetaProcessor_ProcessBlockWithMismatchedTxResultsCount(t *testing.T) {
	t.Parallel()

	createHeader := func(txResults []uint32) *block.Block {
		return &block.Block{
			Header: &block.BlockHeader{
				Nonce:   2,
				Slot:    11,
				TxCount: 1,
			},
			TxHashes:  [][]byte{(&sha256.Sha256{}).Compute("txHash")},
			TxResults: txResults,
		}
	}

	processBlock := func(t *testing.T, txResults []uint32, prepare func(arguments *blproc.ArgMetaProcessor)) error {
		t.Helper()

		arguments := createMockMetaArguments()
		blkc := blockchain.NewBlockChain()
		_ = blkc.SetCurrentBlockHeader(
			&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 10}},
		)
		_ = blkc.SetGenesisHeader(&block.Block{Header: &block.BlockHeader{Nonce: 0}})
		arguments.BlockChain = blkc
		if prepare != nil {
			prepare(&arguments)
		}

		mp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)

		controller, _ := kapps.NewProposalController(arguments.ForkController)
		_ = mp.SetProposalController(controller)

		return mp.ProcessBlock(createHeader(txResults), haveTime)
	}

	t.Run("missing tx results", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, processBlock(t, nil, nil), process.ErrInvalidTXResultsCount)
	})

	t.Run("more tx results than tx hashes", func(t *testing.T) {
		t.Parallel()

		assert.ErrorIs(t, processBlock(t, []uint32{0, 0}, nil), process.ErrInvalidTXResultsCount)
	})

	t.Run("fork disabled: mismatch is not rejected here", func(t *testing.T) {
		t.Parallel()

		err := processBlock(t, nil, func(arguments *blproc.ArgMetaProcessor) {
			arguments.ForkController = mock.NewForkControllerStub().
				SetFork("FixAuditChangesV5", false)
		})

		// The gate being inert must be observable, so pin the check the header
		// falls through to instead of merely asserting the gate's error is absent.
		assert.ErrorIs(t, err, process.ErrTxRootHashDoesNotMatch)
	})
}

// TestMetaProcessor_ProcessBlockRejectsMismatchedTxResultsOnFirstBlockOfActivationEpoch pins
// the boundary the gate is read at. ProcessBlock validates (block.go:127) before it advances
// the epoch notifier (block.go:167), so a gate read from the notifier's current epoch still
// holds the previous epoch's value while the first block of the activation epoch is being
// processed - and that block would be fully executed, with the per-tx result comparison
// bypassed by the empty TxResults, before commitBlock's second checkBlockValidity rejected it.
//
// This uses a real forkController rather than the stub so the flag genuinely lags: registration
// confirms epoch 0, and nothing advances it to epoch 1 before validation runs.
func TestMetaProcessor_ProcessBlockRejectsMismatchedTxResultsOnFirstBlockOfActivationEpoch(t *testing.T) {
	t.Parallel()

	const activationEpoch = uint32(1)

	arguments := createMockMetaArguments()

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{FixAuditChangesV5: activationEpoch},
		epochNotifier,
	)
	require.NoError(t, err)

	// Wire the notifier to the controller the way the node does, and record whether the
	// epoch had already been advanced by the time the header was rejected.
	epochAdvanced := false
	epochNotifier.CheckEpochCalled = func(epoch uint32) {
		epochAdvanced = true
		forkController.EpochConfirmed(epoch)
	}

	// The flag is off for the epoch the node is currently on: a gate read from it skips.
	require.False(t, forkController.FixAuditChangesV5())
	require.True(t, forkController.FixAuditChangesV5InEpoch(activationEpoch))

	blkc := blockchain.NewBlockChain()
	_ = blkc.SetCurrentBlockHeader(
		&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 10, Epoch: activationEpoch - 1}},
	)
	_ = blkc.SetGenesisHeader(&block.Block{Header: &block.BlockHeader{Nonce: 0}})
	arguments.BlockChain = blkc
	arguments.EpochNotifier = epochNotifier
	arguments.ForkController = forkController

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	controller, _ := kapps.NewProposalController(arguments.ForkController)
	_ = mp.SetProposalController(controller)

	// First block of the activation epoch, with TxResults omitted.
	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce:   2,
			Slot:    11,
			Epoch:   activationEpoch,
			TxCount: 1,
		},
		TxHashes:  [][]byte{(&sha256.Sha256{}).Compute("txHash")},
		TxResults: nil,
	}

	err = mp.ProcessBlock(header, haveTime)

	assert.ErrorIs(t, err, process.ErrInvalidTXResultsCount)
	assert.False(t, epochAdvanced,
		"the header must be rejected before the epoch notifier is advanced, otherwise the "+
			"gate is being read from the notifier instead of from the header's own epoch")
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

	commitBlockOkVals(t, &mock.TransactionCoordinatorMock{})
}

// SC logs reach storage only through CommitBlock, and a flush failure must not fail it.
func TestMetaProcessor_CommitBlockFlushesLogsOnce(t *testing.T) {
	t.Parallel()

	for _, flushErr := range []error{nil, errors.New("log flush failure")} {
		flushCalls := 0
		commitBlockOkVals(t, &mock.TransactionCoordinatorMock{
			SaveLogsToStorageCalled: func() error {
				flushCalls++
				return flushErr
			},
		})
		assert.Equal(t, 1, flushCalls)
	}
}

func commitBlockOkVals(t *testing.T, txCoordinator process.TransactionCoordinator) {
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
	arguments.TxCoordinator = txCoordinator
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
		// ids handed back to the caller for the indexer notification, which is
		// only fired once the block is verified
		proposalsToIndex []string
		err              interface{}
		forks            core.ForkController
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
			name: "Error Missing Native Staking Data",
			blk:  &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: 1}},
			perset: func(_ *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
				err := mp.SetProposalKApp(kApp, 1, &kapps.ProposalData{Parameters: map[int32][]byte{1: []byte("1234")}})
				if err != nil {
					return nil, err
				}

				// wipe the staked KLV entry, so retrieving the staking data fails
				// after the active proposals have been found
				adapter := mp.GetKAppAdapter()
				acnt, err := adapter.LoadAccount(kapps.StakingKAppAddress)
				if err != nil {
					return nil, err
				}

				stakingKApp, ok := acnt.(state.KAppAccountHandler)
				if !ok {
					return nil, common.ErrWrongTypeAssertion
				}

				err = stakingKApp.DataTrieTracker().SaveKeyValue(kdautils.KLVKey, nil)
				if err != nil {
					return nil, err
				}

				err = adapter.SaveAccount(stakingKApp)
				if err != nil {
					return nil, err
				}

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
			check:            map[int32][]byte{1: []byte("50000000000")},
			proposalsToIndex: []string{"1"},
			err:              nil,
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
			check:            map[int32][]byte{1: []byte("1234")},
			proposalsToIndex: []string{"1"},
			err:              nil,
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
			// before FixAuditChangesV6 an invalid approved value fails the block
			forks: mock.NewForkControllerStub().SetFork("FixAuditChangesV6", false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arguments := createMockMetaArguments()
			if tt.forks != nil {
				arguments.ForkController = tt.forks
			}
			mpp, err := blproc.NewMetaProcessor(arguments)
			require.NoError(t, err)
			mp := blproc.NewMetaProcessorForTests(mpp)

			SetProposal(t, mp, tt.perset)

			proposalsToIndex, err := mp.ProcessProposalsEndOfEpoch(tt.blk)
			if tt.err != nil {
				if str, ok := tt.err.(string); ok {
					assert.True(t, strings.Contains(err.Error(), str), fmt.Sprintf("expected %s to contain %s", err.Error(), str))
				} else {
					assert.Equal(t, tt.err, err, "expected error equal")
				}
				assert.Nil(t, proposalsToIndex, "a failed settlement must not hand any id to the indexer")
			} else {
				assert.Nil(t, err, "expected no error")
			}

			assert.Equal(t, tt.proposalsToIndex, proposalsToIndex)

			for k, v := range tt.check {
				assert.Equal(t, v, mp.GetActiveParameters()[k].Value)
			}
		})
	}
}

// settleApprovedProposal stores proposal 1 as approved with params and settles it at the
// start of epoch 1.
func settleApprovedProposal(t *testing.T, mp *blproc.MetaProcessorForTests, params map[int32][]byte) {
	settleApprovedProposals(t, mp, 1, 1, params)
}

// settleApprovedProposals stores each params set as an approved proposal, numbered from
// firstID, and settles them in that order at the start of epoch. The stored parameter set
// is kept, so a later settlement builds on an earlier one.
func settleApprovedProposals(t *testing.T, mp *blproc.MetaProcessorForTests, epoch uint32, firstID uint64, params ...map[int32][]byte) {
	ids := make([]uint64, 0, len(params))
	SetProposal(t, mp, func(pc *kapps.ProposalController, kApp state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
		for i, p := range params {
			id := firstID + uint64(i)
			err := mp.SetProposalKApp(kApp, id, &kapps.ProposalData{
				Parameters: p,
				Votes:      map[int32]int64{int32(kapps.ProposalData_VoteDetail_Yes): 10},
			})
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}

		pc.ActiveProposals = map[uint32]*kapps.ActiveProposals{epoch: {ProposalIDs: ids}}
		return mp.GetMarshalizer().Marshal(pc)
	})

	proposalsToIndex, err := mp.ProcessProposalsEndOfEpoch(&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1, Epoch: epoch}})
	require.NoError(t, err)
	require.Len(t, proposalsToIndex, len(params))
}

func storedProposal(t *testing.T, mp *blproc.MetaProcessorForTests, id uint64) *kapps.ProposalData {
	acnt, err := mp.GetKAppAdapter().LoadAccount(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	kApp, ok := acnt.(state.KAppAccountHandler)
	require.True(t, ok)

	proposal, err := mp.GetProposalKApp(kApp, id)
	require.NoError(t, err)

	return proposal
}

// From FixAuditChangesV6 an approved proposal holding an invalid parameter is denied whole
// rather than applied in part, and the epoch start block still settles: failing it would
// fail identically on every node and every retry. Proposals settle one by one, so another
// proposal in the same epoch still lands.
func TestMetaProcessor_ProcessProposalEndOfEpochDeniesAProposalWithAnInvalidParameter(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.ForkController = mock.NewForkControllerStub()
	mpp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	mp := blproc.NewMetaProcessorForTests(mpp)

	before := mp.GetActiveParameters()
	createValidator := int32(kapps.EnumParameter_KAppFeeCreateValidator)
	transfer := int32(kapps.EnumParameter_KAppFeeTransfer)
	threshold := int32(kapps.EnumParameter_MinKFIStakedToEnableProposals)

	settleApprovedProposals(t, mp, 1, 1,
		// a valid fee next to a negative one: neither lands
		map[int32][]byte{createValidator: []byte("1234"), transfer: []byte("-1")},
		// above the KFI staked, which no check compares against: an in-range threshold settles
		map[int32][]byte{threshold: []byte("2000000000000")},
	)

	assert.Equal(t, kapps.ProposalData_DeniedProposal, storedProposal(t, mp, 1).ProposalStatus)
	assert.Equal(t, kapps.ProposalData_ApprovedProposal, storedProposal(t, mp, 2).ProposalStatus)

	params := mp.GetActiveParameters()
	assert.Equal(t, before[createValidator].Value, params[createValidator].Value)
	assert.Equal(t, before[transfer].Value, params[transfer].Value)
	assert.Equal(t, []byte("2000000000000"), params[threshold].Value)
}

// The live parameter set sits outside the accounts snapshot, so both revert paths reload
// it from the KApp trie: otherwise a slot timeout, a failed commit or a sync rollback past
// an epoch start leaves the parameters it settled in effect.
func TestMetaProcessor_RevertReloadsProposalParameters(t *testing.T) {
	t.Parallel()

	key := int32(kapps.EnumParameter_KAppFeeCreateValidator)

	t.Run("RevertStateToSnapshot", func(t *testing.T) {
		t.Parallel()

		arguments := createMockMetaArguments()
		mpp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)
		mp := blproc.NewMetaProcessorForTests(mpp)

		// a committed value other than the default, so only a reload from the trie restores it
		settleApprovedProposal(t, mp, map[int32][]byte{key: []byte("1234")})
		_, err = arguments.AccountsDB[state.KAppAccountsState].Commit()
		require.NoError(t, err)

		settleApprovedProposals(t, mp, 2, 2, map[int32][]byte{key: []byte("5678")})
		require.Equal(t, []byte("5678"), mp.GetActiveParameters()[key].Value)

		// through the interface Worker.Extend calls it on
		var blockProcessor process.BlockProcessor = mpp
		blockProcessor.RevertStateToSnapshot(nil)

		assert.Equal(t, []byte("1234"), mp.GetActiveParameters()[key].Value)
	})

	t.Run("RevertStateToBlock", func(t *testing.T) {
		t.Parallel()

		arguments := createMockMetaArguments()
		arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
			RecreateTrieCalled: func([]byte) error { return nil },
		}
		mpp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)
		mp := blproc.NewMetaProcessorForTests(mpp)

		// a committed value other than the default, so only a reload from the trie restores it
		settleApprovedProposal(t, mp, map[int32][]byte{key: []byte("1234")})
		kappsRoot, err := arguments.AccountsDB[state.KAppAccountsState].Commit()
		require.NoError(t, err)

		settleApprovedProposals(t, mp, 2, 2, map[int32][]byte{key: []byte("5678")})
		_, err = arguments.AccountsDB[state.KAppAccountsState].Commit()
		require.NoError(t, err)
		require.Equal(t, []byte("5678"), mp.GetActiveParameters()[key].Value)

		err = mpp.RevertStateToBlock(&block.Block{Header: &block.BlockHeader{KAppsTrieRoot: kappsRoot}})
		require.NoError(t, err)

		assert.Equal(t, []byte("1234"), mp.GetActiveParameters()[key].Value)
	})

	// Genesis holds no proposal KApp (startup writes it, block 1 commits it), so rolling
	// block 1 back must fall back to the defaults instead of failing the rollback.
	t.Run("RevertStateToBlock to genesis", func(t *testing.T) {
		t.Parallel()

		arguments := createMockMetaArguments()
		arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
			RecreateTrieCalled: func([]byte) error { return nil },
		}
		mpp, err := blproc.NewMetaProcessor(arguments)
		require.NoError(t, err)
		mp := blproc.NewMetaProcessorForTests(mpp)

		settleApprovedProposal(t, mp, map[int32][]byte{key: []byte("1234")})
		_, err = arguments.AccountsDB[state.KAppAccountsState].Commit()
		require.NoError(t, err)

		err = mpp.RevertStateToBlock(&block.Block{Header: &block.BlockHeader{KAppsTrieRoot: trie.EmptyTrieHash}})
		require.NoError(t, err)

		defaults := kapps.InitialProposalParameters(arguments.ForkController)
		assert.Equal(t, defaults[key].Value, mp.GetActiveParameters()[key].Value)
	})
}

// A node restarted on a committed chain has its epoch notifier at 0 until the first block
// advances it. If that first block is rejected, the revert must restore the fork flags of the
// committed head, not of epoch 0: the reload backfills the defaults of the forks active at the
// restored epoch, and the smart contract ones (GasMultiplier among them) are not in the trie. At
// epoch 0 they were dropped, every contract call then got 0 gas, and the block failed on its
// fees on every retry.
func TestMetaProcessor_RevertOfTheFirstBlockAfterRestartKeepsTheCommittedEpochDefaults(t *testing.T) {
	t.Parallel()

	const committedEpoch = uint32(5)
	gasMultiplier := int32(kapps.EnumParameter_GasMultiplier)
	maxGasPerTX := int32(kapps.EnumParameter_MaxGasPerTX)

	epochNotifier := notifier.NewGenericEpochNotifier()
	forkController, err := fork.NewForkController(config.EnableEpochs{SmartContracts: 1, FixAuditChanges: 1}, epochNotifier)
	require.NoError(t, err)

	arguments := createMockMetaArguments()
	arguments.EpochNotifier = epochNotifier
	arguments.ForkController = forkController
	arguments.BlockChain = &mock.BlockChainMock{
		GetGenesisHeaderCalled: createTestBlockchain().GetGenesisHeaderCalled,
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{Nonce: 0, Slot: 1, Epoch: committedEpoch}}
		},
	}
	// fails once the epoch has been advanced, so it reaches the rollback of ProcessBlock
	expectedErr := errors.New("block transactions failure")
	arguments.TxCoordinator = &mock.TransactionCoordinatorMock{
		ProcessBlockTransactionsCalled: func(*block.Block, func() time.Duration, bool) (data.ProcessResults, error) {
			return nil, expectedErr
		},
	}
	arguments.AccountsDB[state.UserAccountsState] = &mock.AccountsStub{
		JournalLenCalled:       func() int { return 0 },
		RevertToSnapshotCalled: func(_ int) error { return nil },
	}

	mpp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)
	mp := blproc.NewMetaProcessorForTests(mpp)

	// the committed trie holds the controller as it was stored before the smart contract fork
	// defaults were ever merged into it
	SetProposal(t, mp, func(pc *kapps.ProposalController, _ state.KAppAccountHandler, mp *blproc.MetaProcessorForTests) ([]byte, error) {
		pc.ActiveParameters = kapps.InitialProposalParameters(forkController)
		return mp.GetMarshalizer().Marshal(pc)
	})
	_, err = arguments.AccountsDB[state.KAppAccountsState].Commit()
	require.NoError(t, err)
	require.NotContains(t, kapps.InitialProposalParameters(forkController), gasMultiplier,
		"precondition: at epoch 0 the smart contract defaults are not part of the initial set")

	// the live set a node at the committed epoch holds, then the state right after a restart
	epochNotifier.CheckEpoch(committedEpoch)
	controller, err := kapps.NewProposalController(forkController)
	require.NoError(t, err)
	require.NoError(t, mpp.SetProposalController(controller))
	require.Contains(t, mp.GetActiveParameters(), gasMultiplier)
	epochNotifier.CheckEpoch(0)

	// an ordinary block: only an epoch start block snapshots the parameters and puts them back
	// after the revert, which would hide the loss
	hdr := createEpochStartHeaderForProcess()
	hdr.Header.IsEpochStart = false
	hdr.Header.Epoch = committedEpoch
	hdr.Header.Slot = 2

	err = mpp.ProcessBlock(hdr, haveTime)
	require.ErrorIs(t, err, expectedErr)

	params := mp.GetActiveParameters()
	assert.Contains(t, params, gasMultiplier, "a contract call would be given 0 gas")
	assert.Contains(t, params, maxGasPerTX)
	assert.True(t, forkController.EnableSmartContracts(), "fork flags must be those of the committed epoch")
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
	committed := false
	accounts := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			committed = true
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

	// the indexer resolves account documents from live state inside SaveBlock, so it
	// must run after commitAll and before CommitBlock returns (KLR-68)
	var saveBlockCalled, committedAtSave bool
	eventsProc := &mock.EventsProcessorStub{
		EnabledCalled: func() bool { return true },
		SaveBlockCalled: func(args *indexer.ArgsSaveBlockData) {
			saveBlockCalled = true
			committedAtSave = committed
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
	assert.True(t, committedAtSave, "SaveBlock must see the committed state")
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

const (
	nativeFPRInitialKLVSupply = int64(100_000_000_000_000)
	// rewards accumulated in the epoch that the first roll closes, so the entry it
	// appends is told apart from the empty ones the later rolls append
	nativeFPRPendingKLV = int64(77)
	nativeFPRPendingKFI = int64(33)
)

// seededKLVFPR and seededKFIFPR are four epochs of native rewards, each partly claimed by a
// different amount so that every trimmed entry leaves its own unclaimed remainder:
// KLV 900, 450, 1000, 750 and KFI 630, 500, 1099, 400.
func seededKLVFPR() []*kapps.FPRData {
	return []*kapps.FPRData{
		{TotalAmount: 1000, TotalStaked: 50000, Epoch: 1, TotalClaimed: 100},
		{TotalAmount: 500, TotalStaked: 52000, Epoch: 2, TotalClaimed: 50},
		{TotalAmount: 1300, TotalStaked: 43000, Epoch: 3, TotalClaimed: 300},
		{TotalAmount: 1000, TotalStaked: 30000, Epoch: 4, TotalClaimed: 250},
	}
}

func seededKFIFPR() []*kapps.FPRData {
	return []*kapps.FPRData{
		{TotalAmount: 700, TotalStaked: 9000, Epoch: 1, TotalClaimed: 70},
		{TotalAmount: 900, TotalStaked: 9100, Epoch: 2, TotalClaimed: 400},
		{TotalAmount: 1100, TotalStaked: 9200, Epoch: 3, TotalClaimed: 1},
		{TotalAmount: 600, TotalStaked: 9300, Epoch: 4, TotalClaimed: 200},
	}
}

// nativeFPRHarness drives the production epochStartNativeStakingKapps over a trie backed KApp
// accounts DB and the real proposal controller.
type nativeFPRHarness struct {
	t              *testing.T
	mp             *blproc.MetaProcessorForTests
	controller     kapps.ActiveProposalController
	forkController core.ForkController
	epochNotifier  process.EpochNotifier
}

func newNativeFPRHarness(
	t *testing.T,
	fixAuditChangesV6 bool,
	maxEpochsUnclaimed int64,
	klvFPR, kfiFPR []*kapps.FPRData,
) *nativeFPRHarness {
	t.Helper()

	forkController := mock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixAuditChangesV6)

	return newNativeFPRHarnessWithFork(t, forkController, nil, maxEpochsUnclaimed, klvFPR, kfiFPR)
}

// newNativeFPRHarnessActivatingAt answers the gate from the production fork controller, fed by
// the epoch notifier the way ProcessBlock and CreateBlock feed it, with FixAuditChangesV6
// enabled at the given epoch.
func newNativeFPRHarnessActivatingAt(
	t *testing.T,
	fixAuditChangesV6Epoch uint32,
	maxEpochsUnclaimed int64,
	klvFPR, kfiFPR []*kapps.FPRData,
) *nativeFPRHarness {
	t.Helper()

	epochNotifier := notifier.NewGenericEpochNotifier()
	forkController, err := fork.NewForkController(
		config.EnableEpochs{FixAuditChangesV6: fixAuditChangesV6Epoch},
		epochNotifier,
	)
	require.NoError(t, err)

	return newNativeFPRHarnessWithFork(t, forkController, epochNotifier, maxEpochsUnclaimed, klvFPR, kfiFPR)
}

func newNativeFPRHarnessWithFork(
	t *testing.T,
	forkController core.ForkController,
	epochNotifier process.EpochNotifier,
	maxEpochsUnclaimed int64,
	klvFPR, kfiFPR []*kapps.FPRData,
) *nativeFPRHarness {
	t.Helper()

	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()

	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10, Shards: 1})
	require.NoError(t, err)
	persister, err := memorydb.NewlruDB(100000)
	require.NoError(t, err)
	unit, err := storageUnit.NewStorageUnit(cache, persister)
	require.NoError(t, err)

	trieStorage, err := trie.NewTrieStorageManagerWithoutPruning(unit)
	require.NoError(t, err)
	tr, err := trie.NewTrie(trieStorage, marshalizer, hasher, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hasher, marshalizer, factory.NewKAppAccountCreator(), core.Normal)
	require.NoError(t, err)

	kdaAcc, err := adb.LoadAccount(kapps.KDAKAppAddress)
	require.NoError(t, err)
	kdaKapp := kdaAcc.(state.KAppAccountHandler)

	stakingAcc, err := adb.LoadAccount(kapps.StakingKAppAddress)
	require.NoError(t, err)
	stakingKapp := stakingAcc.(state.KAppAccountHandler)

	save := func(kappAcc state.KAppAccountHandler, key []byte, value interface{}) {
		raw, errMarshal := marshalizer.Marshal(value)
		require.NoError(t, errMarshal)
		require.NoError(t, kappAcc.DataTrieTracker().SaveKeyValue(key, raw))
	}

	save(stakingKapp, kdautils.KLVKey, &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		TotalStaked:      61000,
		CurrentFPRAmount: nativeFPRPendingKLV,
		FPR:              klvFPR,
	})
	save(stakingKapp, kdautils.KFIKey, &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		TotalStaked:      9400,
		CurrentFPRAmount: nativeFPRPendingKFI,
		FPR:              kfiFPR,
	})
	save(kdaKapp, kdautils.KLVKey, &kapps.KDAData{
		ID:                kdautils.KLVIdentifier,
		AssetType:         kapps.KDAData_Fungible,
		Name:              kdautils.KLVIdentifier,
		Ticker:            kdautils.KLVIdentifier,
		Precision:         6,
		CirculatingSupply: nativeFPRInitialKLVSupply,
		MaxSupply:         90_000_000_000_000_000,
	})
	require.NoError(t, adb.SaveAccounts(kdaKapp, stakingKapp))

	mp, controller, err := blproc.NewMetaProcessorForEpochStartTests(adb, hasher, marshalizer, forkController, epochNotifier)
	require.NoError(t, err)

	h := &nativeFPRHarness{
		t:              t,
		mp:             mp,
		controller:     controller,
		forkController: forkController,
		epochNotifier:  epochNotifier,
	}
	h.setMaxEpochsUnclaimed(maxEpochsUnclaimed)

	return h
}

// setMaxEpochsUnclaimed writes the parameter the way a settled proposal does. It goes around
// ParseParamAndValidate on purpose: what the trim does with a value is under test here, not
// which values governance may still vote in.
func (h *nativeFPRHarness) setMaxEpochsUnclaimed(value int64) {
	h.t.Helper()

	h.controller.UpdateParameters(map[int32]*kapps.Parameter{
		int32(kapps.EnumParameter_MaxEpochsUnclaimed): {
			Type:  kapps.EnumType_Int64,
			Value: []byte(strconv.FormatInt(value, 10)),
		},
	})
	require.Equal(h.t, value, h.controller.GetParameterInt(kapps.EnumParameter_MaxEpochsUnclaimed))
}

// roll runs one epoch start and returns what the header reports as burned. The epoch is
// confirmed first, as block processing does before it reaches the epoch start.
func (h *nativeFPRHarness) roll(epoch uint32) int64 {
	h.t.Helper()

	if h.epochNotifier != nil {
		h.epochNotifier.CheckEpoch(epoch)
	}

	header := &block.Block{Header: &block.BlockHeader{Nonce: uint64(epoch), Epoch: epoch, IsEpochStart: true}}
	require.NoError(h.t, h.mp.EpochStartNativeStakingKapps(header))

	return header.GetBurnedUnclaimed()
}

func (h *nativeFPRHarness) staking() (*kapps.StakingData, *kapps.StakingData) {
	h.t.Helper()

	klv, kfi, err := h.mp.GetNativeStakingKApps()
	require.NoError(h.t, err)

	return klv, kfi
}

// epochs returns the epochs the KLV and the KFI lists currently hold, oldest first.
func (h *nativeFPRHarness) epochs() ([]uint32, []uint32) {
	h.t.Helper()

	klv, kfi := h.staking()

	return fprEpochs(klv.FPR), fprEpochs(kfi.FPR)
}

func fprEpochs(fprs []*kapps.FPRData) []uint32 {
	epochs := make([]uint32, 0, len(fprs))
	for _, fpr := range fprs {
		epochs = append(epochs, fpr.GetEpoch())
	}

	return epochs
}

// klvBurned returns what left the KLV circulating supply since the harness was built.
func (h *nativeFPRHarness) klvBurned() int64 {
	h.t.Helper()

	acnt, err := h.mp.GetKAppAdapter().LoadAccount(kapps.KDAKAppAddress)
	require.NoError(h.t, err)

	raw, err := acnt.(state.KAppAccountHandler).DataTrieTracker().RetrieveValue(kdautils.KLVKey)
	require.NoError(h.t, err)

	klv := &kapps.KDAData{}
	require.NoError(h.t, h.mp.GetMarshalizer().Unmarshal(klv, raw))
	require.Equal(h.t, nativeFPRInitialKLVSupply-klv.CirculatingSupply, klv.BurnedValue,
		"the supply and the burned counter must move together")

	return klv.BurnedValue
}

// A proposal that lowers MaxEpochsUnclaimed below the current length used to leave the equality
// trim unreachable: the lists then grew by one entry per epoch and nothing was burned again.
func TestMetaProcessor_NativeFPRTrim_LoweredParameterTrimsTheExcess(t *testing.T) {
	t.Parallel()

	h := newNativeFPRHarness(t, true, 4, seededKLVFPR(), seededKFIFPR())
	h.setMaxEpochsUnclaimed(2)

	// epochs 1, 2 and 3 no longer fit: KLV 900+450+1000 and KFI 630+500+1099
	burned := h.roll(5)
	assert.Equal(t, int64(2350+2229), burned)
	assert.Equal(t, int64(2350+2229), h.klvBurned())

	klvEpochs, kfiEpochs := h.epochs()
	assert.Equal(t, []uint32{4, 5}, klvEpochs)
	assert.Equal(t, []uint32{4, 5}, kfiEpochs)

	// the entry appended for the closing epoch carries what that epoch accumulated
	klv, kfi := h.staking()
	require.Len(t, klv.FPR, 2)
	require.Len(t, kfi.FPR, 2)
	assert.Equal(t, nativeFPRPendingKLV, klv.FPR[1].GetTotalAmount())
	assert.Equal(t, int64(61000), klv.FPR[1].GetTotalStaked())
	assert.Zero(t, klv.FPR[1].GetTotalClaimed())
	assert.Equal(t, nativeFPRPendingKFI, kfi.FPR[1].GetTotalAmount())
	assert.Equal(t, int64(9400), kfi.FPR[1].GetTotalStaked())
	assert.Zero(t, kfi.FPR[1].GetTotalClaimed())
	// the survivor of the trim is untouched
	assert.Equal(t, int64(1000), klv.FPR[0].GetTotalAmount())
	assert.Equal(t, int64(250), klv.FPR[0].GetTotalClaimed())
	assert.Equal(t, int64(600), kfi.FPR[0].GetTotalAmount())
	assert.Equal(t, int64(200), kfi.FPR[0].GetTotalClaimed())
	assert.Zero(t, klv.CurrentFPRAmount)
	assert.Zero(t, kfi.CurrentFPRAmount)

	// from here on it is one entry out, one entry in: epoch 4 (750+400), then epoch 5 (77+33),
	// then the empty epoch 6
	assert.Equal(t, int64(750+400), h.roll(6))
	assert.Equal(t, nativeFPRPendingKLV+nativeFPRPendingKFI, h.roll(7))
	assert.Zero(t, h.roll(8))

	klvEpochs, kfiEpochs = h.epochs()
	assert.Equal(t, []uint32{7, 8}, klvEpochs)
	assert.Equal(t, []uint32{7, 8}, kfiEpochs)
	assert.Equal(t, int64(2350+2229+750+400+77+33), h.klvBurned())
}

// The lists are trimmed independently, so one that is already within the new limit keeps its
// entries while the other sheds its excess.
func TestMetaProcessor_NativeFPRTrim_KLVAndKFIAreTrimmedIndependently(t *testing.T) {
	t.Parallel()

	h := newNativeFPRHarness(t, true, 2, seededKLVFPR(), seededKFIFPR()[3:])

	// KLV sheds epochs 1, 2 and 3; KFI only holds epoch 4 and keeps it
	assert.Equal(t, int64(2350), h.roll(5))

	klvEpochs, kfiEpochs := h.epochs()
	assert.Equal(t, []uint32{4, 5}, klvEpochs)
	assert.Equal(t, []uint32{4, 5}, kfiEpochs)
}

// Where the parameter was never lowered the loop runs at most once per epoch, so the fork
// changes neither the lists nor the burn.
func TestMetaProcessor_NativeFPRTrim_UnchangedOrRaisedParameterMatchesBothSidesOfTheFork(t *testing.T) {
	t.Parallel()

	type roll struct {
		burned int64
		epochs []uint32
	}

	tests := []struct {
		name               string
		maxEpochsUnclaimed int64
		rolls              []roll
	}{
		{
			name:               "unchanged parameter trims one entry per epoch",
			maxEpochsUnclaimed: 4,
			rolls: []roll{
				{burned: 900 + 630, epochs: []uint32{2, 3, 4, 5}},
				{burned: 450 + 500, epochs: []uint32{3, 4, 5, 6}},
				{burned: 1000 + 1099, epochs: []uint32{4, 5, 6, 7}},
				{burned: 750 + 400, epochs: []uint32{5, 6, 7, 8}},
				{burned: 77 + 33, epochs: []uint32{6, 7, 8, 9}},
			},
		},
		{
			name:               "raised parameter grows to the new limit and holds there",
			maxEpochsUnclaimed: 6,
			rolls: []roll{
				{burned: 0, epochs: []uint32{1, 2, 3, 4, 5}},
				{burned: 0, epochs: []uint32{1, 2, 3, 4, 5, 6}},
				{burned: 900 + 630, epochs: []uint32{2, 3, 4, 5, 6, 7}},
				{burned: 450 + 500, epochs: []uint32{3, 4, 5, 6, 7, 8}},
			},
		},
		{
			name:               "a parameter beyond any list length never trims",
			maxEpochsUnclaimed: math.MaxInt64,
			rolls: []roll{
				{burned: 0, epochs: []uint32{1, 2, 3, 4, 5}},
				{burned: 0, epochs: []uint32{1, 2, 3, 4, 5, 6}},
			},
		},
	}

	for _, tt := range tests {
		for _, fixAuditChangesV6 := range []bool{false, true} {
			t.Run(tt.name+"/FixAuditChangesV6="+strconv.FormatBool(fixAuditChangesV6), func(t *testing.T) {
				t.Parallel()

				h := newNativeFPRHarness(t, fixAuditChangesV6, tt.maxEpochsUnclaimed, seededKLVFPR(), seededKFIFPR())

				total := int64(0)
				for i, want := range tt.rolls {
					burned := h.roll(uint32(5 + i))
					total += want.burned

					klvEpochs, kfiEpochs := h.epochs()
					assert.Equal(t, want.burned, burned, "roll %d", i)
					assert.Equal(t, want.epochs, klvEpochs, "roll %d", i)
					assert.Equal(t, want.epochs, kfiEpochs, "roll %d", i)
				}
				assert.Equal(t, total, h.klvBurned())
			})
		}
	}
}

// A window of no epochs at all cannot hold any closed entry: every one of them is burned and each
// list is left with just the entry of the epoch being closed, the same as a window of one.
func TestMetaProcessor_NativeFPRTrim_ParameterOfOneOrLessKeepsOnlyTheNewEntry(t *testing.T) {
	t.Parallel()

	for _, maxEpochsUnclaimed := range []int64{1, 0, -1, math.MinInt64} {
		t.Run(strconv.FormatInt(maxEpochsUnclaimed, 10), func(t *testing.T) {
			t.Parallel()

			h := newNativeFPRHarness(t, true, maxEpochsUnclaimed, seededKLVFPR(), seededKFIFPR())

			// all four seeded epochs: KLV 900+450+1000+750 and KFI 630+500+1099+400
			require.NotPanics(t, func() {
				assert.Equal(t, int64(3100+2629), h.roll(5))
			})

			klvEpochs, kfiEpochs := h.epochs()
			assert.Equal(t, []uint32{5}, klvEpochs)
			assert.Equal(t, []uint32{5}, kfiEpochs)

			require.NotPanics(t, func() {
				assert.Equal(t, nativeFPRPendingKLV+nativeFPRPendingKFI, h.roll(6))
			})

			klvEpochs, kfiEpochs = h.epochs()
			assert.Equal(t, []uint32{6}, klvEpochs)
			assert.Equal(t, []uint32{6}, kfiEpochs)
			assert.Equal(t, int64(3100+2629+77+33), h.klvBurned())
		})
	}
}

// With nothing to trim the loop must not reach for the first entry: the equality check did so
// when both the list length and the parameter were zero.
func TestMetaProcessor_NativeFPRTrim_ZeroParameterOnEmptyListsDoesNotPanic(t *testing.T) {
	t.Parallel()

	h := newNativeFPRHarness(t, true, 0, nil, nil)

	require.NotPanics(t, func() {
		assert.Zero(t, h.roll(5))
	})

	klvEpochs, kfiEpochs := h.epochs()
	assert.Equal(t, []uint32{5}, klvEpochs)
	assert.Equal(t, []uint32{5}, kfiEpochs)
	assert.Zero(t, h.klvBurned())
}

// Before FixAuditChangesV6 the equality trim is kept as it was, so a lowered parameter still
// leaves the lists growing with nothing burned.
func TestMetaProcessor_NativeFPRTrim_LoweredParameterBeforeTheForkKeepsGrowing(t *testing.T) {
	t.Parallel()

	h := newNativeFPRHarness(t, false, 4, seededKLVFPR(), seededKFIFPR())
	h.setMaxEpochsUnclaimed(2)

	for epoch := uint32(5); epoch < 10; epoch++ {
		assert.Zero(t, h.roll(epoch), "epoch %d", epoch)
	}

	klvEpochs, kfiEpochs := h.epochs()
	assert.Equal(t, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9}, klvEpochs)
	assert.Equal(t, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9}, kfiEpochs)
	assert.Zero(t, h.klvBurned())
}

// One chain across the activation epoch, with the gate answered by the production fork controller.
// The parameter is lowered while the fork is still ahead: up to the last epoch before it the lists
// keep growing with nothing burned, and the epoch start of the activation epoch itself trims all
// that piled up and burns it.
func TestMetaProcessor_NativeFPRTrim_AcrossTheForkActivationEpoch(t *testing.T) {
	t.Parallel()

	const activationEpoch = uint32(7)

	h := newNativeFPRHarnessActivatingAt(t, activationEpoch, 4, seededKLVFPR(), seededKFIFPR())
	h.setMaxEpochsUnclaimed(2)

	// pre fork: the equality never matches again
	for epoch := uint32(5); epoch < activationEpoch; epoch++ {
		assert.Zero(t, h.roll(epoch), "epoch %d", epoch)
		require.False(t, h.forkController.FixAuditChangesV6(), "epoch %d", epoch)
	}

	klvEpochs, kfiEpochs := h.epochs()
	assert.Equal(t, []uint32{1, 2, 3, 4, 5, 6}, klvEpochs)
	assert.Equal(t, []uint32{1, 2, 3, 4, 5, 6}, kfiEpochs)
	assert.Zero(t, h.klvBurned())

	// activation: epochs 1 to 5 no longer fit, KLV 900+450+1000+750+77 and KFI 630+500+1099+400+33
	burned := h.roll(activationEpoch)
	require.True(t, h.forkController.FixAuditChangesV6())
	assert.Equal(t, int64(3177+2662), burned)
	assert.Equal(t, int64(3177+2662), h.klvBurned())

	klvEpochs, kfiEpochs = h.epochs()
	assert.Equal(t, []uint32{6, 7}, klvEpochs)
	assert.Equal(t, []uint32{6, 7}, kfiEpochs)

	// post fork: one entry out, one entry in; epochs 6 and 7 accumulated nothing
	assert.Zero(t, h.roll(activationEpoch+1))
	assert.Zero(t, h.roll(activationEpoch+2))

	klvEpochs, kfiEpochs = h.epochs()
	assert.Equal(t, []uint32{8, 9}, klvEpochs)
	assert.Equal(t, []uint32{8, 9}, kfiEpochs)
	assert.Equal(t, int64(3177+2662), h.klvBurned())
}

func appendUnknownField(buff []byte, fieldNumber protowire.Number, size int) []byte {
	buff = protowire.AppendTag(buff, fieldNumber, protowire.BytesType)
	return protowire.AppendBytes(buff, bytes.Repeat([]byte{0xab}, size))
}

func createMetaProcessorDecodingBlocks(t *testing.T) process.BlockProcessor {
	arguments := createMockMetaArguments()
	blockChain := createTestBlockchain()
	blockChain.CreateNewHeaderCalled = func() data.HeaderHandler {
		return &block.Block{}
	}
	arguments.BlockChain = blockChain

	mp, err := blproc.NewMetaProcessor(arguments)
	require.NoError(t, err)

	return mp
}

func TestMetaProcessor_DecodeBlockHeaderDropsUnknownFieldsOutsideTheHeader(t *testing.T) {
	t.Parallel()

	mp := createMetaProcessorDecodingBlocks(t)
	original := &block.Block{
		Header:        &block.BlockHeader{Nonce: 3, Slot: 4, ParentHash: []byte("parent")},
		TxHashes:      [][]byte{[]byte("tx hash")},
		PubKeysBitmap: []byte{1},
		Signature:     []byte("signature"),
	}
	originalBuff, err := marshalizer.Marshal(original)
	require.NoError(t, err)

	decoded := mp.DecodeBlockHeader(appendUnknownField(originalBuff, 99, 4096))
	require.NotNil(t, decoded)

	reencoded, err := marshalizer.Marshal(decoded)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(originalBuff, reencoded), "re-encoded block must match the original encoding")
}

func TestMetaProcessor_DecodeBlockHeaderLeavesTheHeaderBytesUnchanged(t *testing.T) {
	t.Parallel()

	mp := createMetaProcessorDecodingBlocks(t)
	headerBuff, err := marshalizer.Marshal(&block.BlockHeader{Nonce: 3, Slot: 4, ParentHash: []byte("parent")})
	require.NoError(t, err)
	headerWithUnknown := appendUnknownField(headerBuff, 99, 4096)

	header := &block.BlockHeader{}
	require.NoError(t, marshalizer.Unmarshal(header, headerWithUnknown))
	blockBuff, err := marshalizer.Marshal(&block.Block{Header: header, Signature: []byte("signature")})
	require.NoError(t, err)

	decoded := mp.DecodeBlockHeader(appendUnknownField(blockBuff, 98, 64))
	require.NotNil(t, decoded)
	decodedBlock, ok := decoded.(*block.Block)
	require.True(t, ok)

	reencodedHeader, err := marshalizer.Marshal(decodedBlock.Header)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(headerWithUnknown, reencodedHeader), "re-encoded header must match the received header bytes")
	reencodedBlock, err := marshalizer.Marshal(decodedBlock)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(blockBuff, reencodedBlock), "re-encoded block must drop only the unknown fields outside the header")
}
