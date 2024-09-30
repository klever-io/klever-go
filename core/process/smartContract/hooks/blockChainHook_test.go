package hooks

import (
	"math/big"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
)

var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)
var testOwnerAddress, _ = addressConverter.Decode("klv10gq6xsegedacd084vmpr2xus950j3d6lhqjfe8ue2xkmfwtkzavqnqhz99")
var testToAddress, _ = addressConverter.Decode("klv1mt8yw657z6nk9002pccmwql8w90k0ac6340cjqkvm9e7lu0z2wjqudt69s")

var marshalizer = marshal.NewProtoMarshalizer()

func newBlockChainHookImpl() *BlockChainHookImpl {
	return &BlockChainHookImpl{
		kappController: newKAppController(),
	}
}

func createAccountsDB(
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	accountFactory state.AccountFactory,
	trieStorageManager data.StorageManager,
) *state.AccountsDB {
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	return adb
}

func createFullArgumentsForKAppsProcessing(trieStorer storage.Storer) (state.AccountsAdapter, state.AccountsAdapter, state.AccountsAdapter, state.AccountsCacher) {
	hasher := &sha256.Sha256{}
	trieFactoryManager, _ := trie.NewTrieStorageManagerWithoutPruning(trieStorer)
	userAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewAccountCreator(), trieFactoryManager)
	kappAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewKAppAccountCreator(), trieFactoryManager)
	peerAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewPeerAccountCreator(), trieFactoryManager)

	accCacher, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: userAccountsDB,
			Kapps:    kappAccountsDB,
			Peers:    peerAccountsDB,
		},
	)
	accCacher.ResetAll(true)

	if err != nil {
		panic(err)
	}

	return userAccountsDB, kappAccountsDB, peerAccountsDB, accCacher
}

func createMockPubkeyConverter() *cryptoMock.PubkeyConverterMock {
	return cryptoMock.NewPubkeyConverterMock(32)
}

func freeFeeHandlerMock() *commonMock.FeeHandlerStub {
	return &commonMock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return &transaction.CostResponse{}, nil
		},
	}
}

func createArgsForTxProcessorWithAccounts(accsMock, peersMock, kappsMock state.AccountsAdapter, accCacher state.AccountsCacher) pTX.ArgsNewTxProcessor {

	marshalizerMock := &commonMock.ProtoMarshalizerMock{}
	pubkeyConvMock := createMockPubkeyConverter()
	ratingsDataMock := &commonMock.RatingsInfoMock{}

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
		SmartContracts:        0,
	}, epochNotifier)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         &commonMock.HasherMock{},
		Marshalizer:    marshalizerMock,
		PubkeyConv:     pubkeyConvMock,
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    ratingsDataMock,
	}

	kAppController, _ := kappcontroller.NewKappController(argsKapp)

	args := pTX.ArgsNewTxProcessor{
		Hasher:         &commonMock.HasherMock{},
		PubkeyConv:     pubkeyConvMock,
		Marshalizer:    marshalizerMock,
		EconomicsFee:   freeFeeHandlerMock(),
		TxFeeHandler:   &commonMock.FeeAccumulatorStub{},
		EpochNotifier:  epochNotifier,
		RatingsData:    ratingsDataMock,
		KAppController: kAppController,
		SingleSigner:   &singlesig.DisabledSingleSig{},
		KeyGen:         &commonMock.KeyGenMock{},
		AccountsCacher: accCacher,
		ScProcessor:    &commonMock.SCProcessorMock{},
		ForkController: forkController,
	}
	return args
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

func createArgsForTxProcessor() pTX.ArgsNewTxProcessor {
	accsMock, peersMock, kappsMock, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	return createArgsForTxProcessorWithAccounts(accsMock, peersMock, kappsMock, accCacher)
}

func loadUserAccount(accountsDB state.AccountsCacher, address []byte) state.UserAccountHandler {
	userAcc, _ := accountsDB.LoadUser(address)
	return userAcc
}

func loadKAppAccount(kappsDB state.AccountsCacher, address []byte) state.KAppAccountHandler {
	kappAcc, _ := kappsDB.LoadKApp(address)
	return kappAcc
}

func initKLVintoKapps(kdaKapp state.KAppAccountHandler) {
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)

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

	klvData, _ := marshalizer.Marshal(&klv)

	_ = kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
}

func createBlockHeader() *block.Block {
	hdr := block.Block{
		Header: &block.BlockHeader{
			Timestamp:    time.Now().Add(time.Hour).Unix(),
			Nonce:        1,
			Epoch:        0,
			Slot:         1,
			ParentHash:   []byte(""),
			TxRootHash:   []byte("txRootHash"),
			TrieRoot:     []byte("rootHash"),
			TxCount:      1,
			PrevRandSeed: make([]byte, 0),
			RandSeed:     make([]byte, 0),
		},
		ProducerSignature: []byte("signature"),
	}

	return &hdr
}

func newKAppController() kapp.KAppController {
	userDB, _, _, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	initKLVintoKapps(kdaKapp)
	_ = ownerAcc.AddToBalance(10_000_000_000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)

	args := createArgsForTxProcessor()
	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         args.Hasher,
		Marshalizer:    args.Marshalizer,
		PubkeyConv:     args.PubkeyConv,
		AccountsCacher: accCacher,
		ForkController: args.ForkController,
		RatingsData:    args.RatingsData,
	}

	kAppController, err := kappcontroller.NewKappController(argsKapp)
	if err != nil {
		panic(err)
	}
	_ = kAppController.InitKApps(accCacher)

	hdr := createBlockHeader()

	kappContext := kapp.NewKappContext(
		kapp.ArgsNewKAppContext{
			ContractID:   0,
			ContractType: transaction.TXContract_SmartContractType,
			Block:        hdr,
		})

	kAppController.SetCurrentKAppContext(kappContext)
	return kAppController
}

func TestBlockChainHookImpl_KDATransfer(t *testing.T) {
	t.Parallel()

	t.Run("should transfer KDA", func(t *testing.T) {
		hookImpl := newBlockChainHookImpl()

		tc := &transaction.TransferContract{
			ToAddress: testToAddress,
			AssetID:   kdautils.KLVIdentifier,
			Amount:    1,
		}

		err := hookImpl.KDATransfer(testOwnerAddress, tc)
		assert.Nil(t, err)
	})

	t.Run("should return error if any", func(t *testing.T) {
		hookImpl := newBlockChainHookImpl()

		tc := &transaction.TransferContract{
			ToAddress: testToAddress,
			AssetID:   kdautils.KLVIdentifier,
			Amount:    -1,
		}

		err := hookImpl.KDATransfer(testOwnerAddress, tc)
		assert.ErrorContains(t, err, "result code: 4, invalid value provided")
	})
}

func TestBlockChainHookImpl_TransferValueOnly(t *testing.T) {
	t.Parallel()

	t.Run("should transfer KDA", func(t *testing.T) {
		hookImpl := newBlockChainHookImpl()
		err := hookImpl.TransferValueOnly(testToAddress, testOwnerAddress, big.NewInt(1))
		assert.Nil(t, err)
	})

	t.Run("should return error if any", func(t *testing.T) {
		hookImpl := newBlockChainHookImpl()
		err := hookImpl.TransferValueOnly(testToAddress, testOwnerAddress, big.NewInt(-1))
		assert.ErrorContains(t, err, "result code: 4, invalid value provided")
	})

	t.Run("should return error in same format as KDATransfer", func(t *testing.T) {
		hookImpl := newBlockChainHookImpl()

		tc := &transaction.TransferContract{
			ToAddress: testToAddress,
			AssetID:   kdautils.KLVIdentifier,
			Amount:    -1,
		}

		errKda := hookImpl.KDATransfer(testOwnerAddress, tc)
		errTransfer := hookImpl.TransferValueOnly(testToAddress, testOwnerAddress, big.NewInt(-1))
		assert.ErrorContains(t, errKda, "result code: 4, invalid value provided")
		assert.Equal(t, errKda.Error(), errTransfer.Error())
	})
}
