package block_test

import (
	"bytes"
	"errors"
	"reflect"
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
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
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
			Indexer:                 &consensusMock.IndexerMock{},
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
	_ = adb.SaveAccount(kdaKapp)
	_ = adb.SaveAccount(stakingKapp)

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

func TestNewMetaProcessor_NilBlockSizeThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockMetaArguments()
	arguments.BlockSizeThrottler = nil

	be, err := blproc.NewMetaProcessor(arguments)
	assert.Equal(t, process.ErrNilBlockSizeThrottler, err)
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
