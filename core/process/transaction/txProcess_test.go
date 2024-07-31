package transaction_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/kapp/validators"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
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
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const MaxGasLimitPerBlock = uint64(100000)
const MinFreezeAmount = int64(1_000)
const initialStaking = int64(3_000_000_000_000)
const MarketIDLength = 6

var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)
var testOwnerAddress, _ = addressConverter.Decode("klv10gq6xsegedacd084vmpr2xus950j3d6lhqjfe8ue2xkmfwtkzavqnqhz99")
var testAdminAddress, _ = addressConverter.Decode("klv1mt8yw657z6nk9002pccmwql8w90k0ac6340cjqkvm9e7lu0z2wjqudt69s")
var testToAddress, _ = addressConverter.Decode("klv15zssmvht00ugvge5le9n885kahc5ykxzvmxx6xwz5ya2an562yyssfa0c5")
var testReferralAddress, _ = addressConverter.Decode("klv1kxevjek45u94k9kpsm3en5amw7cgxrpjjyryvpwmavhl9w4da65s9ul66d")
var testWhitelistAddress, _ = addressConverter.Decode("klv1vrcecp3f6d8r6gk3p5r8m3lu7ndrzsfce2dhr5werntr43lvjcpsq97c7y")

var marshalizer = marshal.NewProtoMarshalizer()

func freeFeeHandlerMock() *commonMock.FeeHandlerStub {
	return &commonMock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return &transaction.CostResponse{}, nil
		},
	}
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

func createAccountStub(sndAddr, rcvAddr []byte,
	acntSrc, acntDst state.UserAccountHandler,
) *commonMock.AccountsStub {
	adb := commonMock.AccountsStub{}

	adb.LoadAccountCalled = func(address []byte) (state.AccountHandler, error) {
		if bytes.Equal(address, sndAddr) {
			return acntSrc, nil
		}

		if bytes.Equal(address, rcvAddr) {
			return acntDst, nil
		}

		return nil, errors.New("failure")
	}

	adb.GetExistingAccountCalled = func(address []byte) (state.AccountHandler, error) {
		if bytes.Equal(address, sndAddr) {
			return acntSrc, nil
		}

		if bytes.Equal(address, rcvAddr) {
			return acntDst, nil
		}

		return nil, errors.New("failure")
	}

	return &adb
}

func createProposalController() kapps.ActiveProposalController {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	pc, _ := kapps.NewProposalController(forkController)

	pc.ActiveParameters[int32(kapps.EnumParameter_MinKLVBucketAmount)] = &kapps.Parameter{
		Type:  kapps.EnumType_Int64,
		Value: []byte(fmt.Sprintf("%d", MinFreezeAmount)),
	}

	return pc
}

func createArgsForTxProcessor() pTX.ArgsNewTxProcessor {
	accsMock, peersMock, kappsMock, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	return createArgsForTxProcessorWithAccounts(accsMock, peersMock, kappsMock, accCacher)
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

func createTransactionMock(contract proto.Message, txType transaction.TXContract_ContractType, sender []byte, nonce uint64) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{Signature: make([][]byte, 1)}

	serialized, err := marshalizer.Marshal(contract)
	if err != nil {
		return nil, errors.New("could not serialize contract")
	}

	tx.RawData = &transaction.Transaction_Raw{
		Sender: sender,
		Nonce:  nonce,
		Contract: []*transaction.TXContract{
			{
				Type: txType,
				Parameter: &anypb.Any{
					TypeUrl: "github.com/klever-io/klever-go/" + string(proto.MessageName(contract)),
					Value:   serialized,
				},
			},
		},
	}

	return tx, nil
}

func preProcessTransactionMock(tx *transaction.Transaction) ([]byte, error) {
	computedHash, err := tools.CalculateHash(&commonMock.MarshalizerMock{}, &commonMock.HasherMock{}, tx.RawData)

	return computedHash, err
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

func createFullArgumentsForKAppsProcessingMemory() (state.AccountsAdapter, state.AccountsAdapter, state.AccountsAdapter, state.AccountsCacher) {
	hasher := &sha256.Sha256{}
	trieFactoryManagerAcc, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	trieFactoryManagerKApp, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	trieFactoryManagerPeer, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())

	userAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewAccountCreator(), trieFactoryManagerAcc)
	kappAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewKAppAccountCreator(), trieFactoryManagerKApp)
	peerAccountsDB := createAccountsDB(hasher, marshalizer, factory.NewPeerAccountCreator(), trieFactoryManagerPeer)

	accCacher, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: userAccountsDB,
			Kapps:    kappAccountsDB,
			Peers:    peerAccountsDB,
		},
	)
	accCacher.ResetAll(true)

	return userAccountsDB, kappAccountsDB, peerAccountsDB, accCacher
}

func loadUserAccount(accountsDB state.AccountsCacher, address []byte) state.UserAccountHandler {
	userAcc, _ := accountsDB.LoadUser(address)
	return userAcc
}

func loadKAppAccount(kappsDB state.AccountsCacher, address []byte) state.KAppAccountHandler {
	kappAcc, _ := kappsDB.LoadKApp(address)
	return kappAcc
}

func loadPeerAccount(peersDB state.AccountsCacher, address []byte) state.PeerAccountHandler {
	peerAcc, _ := peersDB.LoadPeer(address)
	return peerAcc
}

func initKLVAndKFIintoKapps(kdaKapp, stakingKapp state.KAppAccountHandler) {
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)
	aprKey := kdautils.ToKDAKey([]byte("APR"), nil)

	staking := kapps.StakingData{
		InterestType: kapps.StakingData_FPRI,
		TotalStaked:  3_000_000_000_000,
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

func initProposalKapp(proposalKapp state.KAppAccountHandler) kapps.ActiveProposalController {
	proposalKey := kdautils.ToProposalKey(0)

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	pc, _ := kapps.NewProposalController(forkController)
	minAmount := fmt.Sprintf("%d", MinFreezeAmount)
	pc.UpdateParameters(
		map[int32]*kapps.Parameter{
			int32(kapps.EnumParameter_MinKLVBucketAmount):      {Type: kapps.EnumType_Int64, Value: []byte(minAmount)},
			int32(kapps.EnumParameter_MinSelfDelegatedAmount):  {Type: kapps.EnumType_Int64, Value: []byte("0")},
			int32(kapps.EnumParameter_MinTotalDelegatedAmount): {Type: kapps.EnumType_Int64, Value: []byte("0")},
		})
	controllerData, _ := marshalizer.Marshal(pc.ProposalController)

	_ = proposalKapp.DataTrieTracker().SaveKeyValue(proposalKey, controllerData)

	return pc
}

func initMarketplaceKapp(marketplaceKapp state.KAppAccountHandler) {
	marketplaceKey := kdautils.ToMarketplaceKey(kdautils.KLVIdentifier)

	marketplace := kapps.Marketplace{
		ID:                 kdautils.KLVIdentifier,
		Name:               []byte("KLEVER"),
		OwnerAddress:       testOwnerAddress,
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 10,
	}

	marketplaceData, _ := marshalizer.Marshal(&marketplace)

	_ = marketplaceKapp.DataTrieTracker().SaveKeyValue(marketplaceKey, marketplaceData)
}

func createBaseKAppsProcessingArgs() pTX.ArgsNewTxProcessor {
	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())
	return createBaseKAppsProcessingArgsCommon(userDB, kappDB, peerDB, accCacher)
}

func createBaseKAppsProcessingArgs2() pTX.ArgsNewTxProcessor {
	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessingMemory()
	return createBaseKAppsProcessingArgsCommon(userDB, kappDB, peerDB, accCacher)
}

func createBaseKAppsProcessingArgsCommon(userDB, kappDB, peerDB state.AccountsAdapter, accCacher state.AccountsCacher) pTX.ArgsNewTxProcessor {

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	adminAcc := loadUserAccount(accCacher, testAdminAddress)
	_ = adminAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	kdaFeesPoolKapp := loadKAppAccount(accCacher, kapps.KDAFeesPoolKAppAddress)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(adminAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(kdaFeesPoolKapp)

	args := createArgsForTxProcessorWithAccounts(userDB, kappDB, peerDB, accCacher)
	args.SingleSigner = &singlesig.DisabledSingleSig{}
	args.KeyGen = &commonMock.KeyGenMock{}

	_ = args.KAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)
	_ = args.KAppController.GetKDAFeesPoolKApp().SetAccountsCacher(accCacher)

	return args
}

//------- pTX.NewTxProcessor

func TestNewTxProcessor_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	args.Hasher = nil
	txProc, err := pTX.NewTxProcessor(args)

	assert.Equal(t, common.ErrNilHasher, err)
	assert.Nil(t, txProc)
}

func TestNewTxProcessor_NilPubkeyConverterMockShouldErr(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	args.PubkeyConv = nil
	txProc, err := pTX.NewTxProcessor(args)

	assert.Equal(t, common.ErrNilPubkeyConverter, err)
	assert.Nil(t, txProc)
}

func TestNewTxProcessor_NilMarshalizerMockShouldErr(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	args.Marshalizer = nil
	txProc, err := pTX.NewTxProcessor(args)

	assert.Equal(t, common.ErrNilMarshalizer, err)
	assert.Nil(t, txProc)
}

func TestNewTxProcessor_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	args.EpochNotifier = nil
	txProc, err := pTX.NewTxProcessor(args)

	assert.Equal(t, common.ErrNilEpochNotifier, err)
	assert.Nil(t, txProc)
}

func TestNewTxProcessor_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	txProc, err := pTX.NewTxProcessor(args)

	assert.Nil(t, err)
	assert.NotNil(t, txProc)
}

//------- GetAccounts

func TestTxProcessor_GetAccountsShouldErrNilAddressContainer(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	execTx := NewTXProcessor(t, args)

	adr1 := []byte{65}
	adr2 := []byte{67}

	_, _, err := execTx.GetAccounts(nil, adr2)
	assert.Equal(t, process.ErrNilAddressContainer, err)

	_, _, err = execTx.GetAccounts(adr1, nil)
	assert.Equal(t, process.ErrNilAddressContainer, err)
}

func TestTxProcessor_GetAccountsOkValsShouldWork(t *testing.T) {
	t.Parallel()

	adr1 := []byte{65}
	adr2 := []byte{67}

	args := createArgsForTxProcessor()
	acnt1, _ := args.AccountsCacher.LoadUser(adr1)
	acnt1.AddToBalance(100_000_000, nil, true)
	acnt2, _ := args.AccountsCacher.LoadUser(adr2)
	acnt2.AddToBalance(200_000_000, nil, true)
	args.AccountsCacher.SaveAll()

	execTx := NewTXProcessor(t, args)

	a1, a2, err := execTx.GetAccounts(adr1, adr2)
	assert.Nil(t, err)
	assert.True(t, a1.Equal(acnt1))
	assert.True(t, a2.Equal(acnt2))
}

func TestTxProcessor_GetSameAccountShouldWork(t *testing.T) {
	t.Parallel()

	adr1 := []byte{65}
	adr2 := []byte{65}

	args := createArgsForTxProcessor()
	_, _ = args.AccountsCacher.LoadUser(adr1)
	_, _ = args.AccountsCacher.LoadUser(adr2)
	args.AccountsCacher.SaveAll()
	execTx := NewTXProcessor(t, args)

	a1, a2, err := execTx.GetAccounts(adr1, adr1)
	assert.Nil(t, err)
	assert.True(t, a1 == a2)
}

//------- ProcessTransaction

func TestTxProcessor_ProcessTransactionNilTxShouldErr(t *testing.T) {
	t.Parallel()

	execTx, _ := NexTXProcessorV2(t)

	err := execTx.ProcessTransaction(createBlockHeader(), nil, nil)
	assert.Equal(t, process.ErrNilTransaction, err)
}

func TestTxProcessor_ProcessTransactionMalfunctionAccountsShouldErr(t *testing.T) {
	t.Parallel()

	args := createArgsForTxProcessor()
	execTx := NewTXProcessor(t, args)

	contract := transaction.TransferContract{
		ToAddress: []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
		Amount:    45,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"), 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.NotNil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.NotNil(t, err)
}

func InitTestAccounts(db state.AccountsCacher) {
	ownerAcc := loadUserAccount(db, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	adminAcc := loadUserAccount(db, testAdminAddress)
	_ = adminAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(db, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	_ = db.SaveAll()
}

func TestTxProcessor_ProcessWithInsufficientFundsShouldCreateReceiptErr(t *testing.T) {
	t.Parallel()

	c := NewController(t)
	execTx := c.execTx

	initKLVAndKFIintoKapps(c.kdaKapp, c.stakingKapp)
	InitTestAccounts(c.accCacher)

	contract := transaction.TransferContract{
		ToAddress: testToAddress,
		Amount:    999_999_999_999,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInsufficientFunds, err)
}

func TestTxProcessor_ProcessTransferWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, kdautils.KLVIdentifier, testOwnerAddress)

	//Send to the same address
	contract := transaction.TransferContract{
		ToAddress: testOwnerAddress,
		Amount:    1,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSameSenderAndReceiverAddress, err)
}

func TestTxProcessor_InvalidAssetShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, kdautils.KLVIdentifier, testOwnerAddress)

	//Send asset that dont exist
	contract := transaction.TransferContract{
		ToAddress: testToAddress,
		AssetID:   []byte("INVALID_KDA"),
		Amount:    1,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)
}

func TestTxProcessor_InvalidBalanceShouldErr(t *testing.T) {
	t.Parallel()
	args := createBaseKAppsProcessingArgs()

	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	//Send amount greater than balance
	contract := transaction.TransferContract{
		ToAddress: testToAddress,
		Amount:    200_000_000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInsufficientFunds, err)
}
func TestTxProcessor_NegativeAmountShouldErr(t *testing.T) {
	t.Parallel()
	args := createBaseKAppsProcessingArgs()

	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	//Send negative amount
	contract := transaction.TransferContract{
		ToAddress: testToAddress,
		Amount:    -10_000_000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestTxProcessor_InvalidAddressShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, kdautils.KLVIdentifier, testOwnerAddress)

	//Send to invalid address
	contract := transaction.TransferContract{
		ToAddress: []byte("INVALID_ADDR"),
		Amount:    10_000_000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)
}

func TestTxProcessor_ProcessTransferOkValsShouldWork2(t *testing.T) {
	t.Parallel()

	args := createBaseKAppsProcessingArgs()

	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	contract := transaction.TransferContract{
		ToAddress: testToAddress,
		Amount:    61,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Nil(t, err)

	ownerAcc := loadUserAccount(args.AccountsCacher, testOwnerAddress)
	toAcc := loadUserAccount(args.AccountsCacher, testToAddress)

	assert.Equal(t, int64(99999939), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(100000061), toAcc.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTxProcessor_ProcessCreateAssetInvalidTypeShouldErr(t *testing.T) {
	t.Parallel()

	args := createBaseKAppsProcessingArgs()
	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	//Create asset with invalid type
	contract := transaction.CreateAssetContract{
		Type:          3,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetTypeInvalid, err)

	//Create asset with invalid name
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA$"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrTokenNameNotHumanReadable, err)

}

func TestTxProcessor_ProcessCreateAssetInvalidTickerShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 1, kdautils.KLVIdentifier, testOwnerAddress)

	//Create asset with invalid ticker
	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST$"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrTickerNameNotValid, err)

}
func TestTxProcessor_ProcessCreateAssetInvalidOwnerShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 1, kdautils.KLVIdentifier, testOwnerAddress)

	//Create asset with invalid owner address
	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  []byte("INVALID"),
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidOwnerAddr, err)
}

func TestTxProcessor_ProcessCreateAssetInvalidPRecisionShouldErr(t *testing.T) {
	t.Parallel()

	args := createBaseKAppsProcessingArgs()
	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	//Create asset with invalid precision
	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     9, // > 8
		InitialSupply: 10000,
		MaxSupply:     10000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetPrecision, err)

	//Create asset with invalid max supply
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 0,
		MaxSupply:     -1,
		Properties: &transaction.PropertiesInfo{
			CanMint: false,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSupplyNotValid, err)

}

func TestTxProcessor_ProcessCreateAssetInvalidCirculationShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, kdautils.KLVIdentifier, testOwnerAddress)

	//Create asset that cant mint with init != circ/max supplies
	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 0,
		MaxSupply:     10000,
		Properties: &transaction.PropertiesInfo{
			CanMint: false,
		},
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSupplyNotValid, err)

	//Create asset that cant mint with infinite max supply but not mintable
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanMint: false,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSupplyNotValid, err)

	//Create asset that can mint with init/circ > max supply
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10001,
		MaxSupply:     10000,
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSupplyNotValid, err)

	//Create asset that can init with negative supply
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: -10000,
		MaxSupply:     -10000,
		Properties: &transaction.PropertiesInfo{
			CanMint: false,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrSupplyNotValid, err)

	//Create Fungible with negative royalties transfer fixed
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
		Royalties: &transaction.RoyaltiesInfo{
			TransferFixed: -1000,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create Fungible with negative royalties transfer percentage
	contract = transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
		Royalties: &transaction.RoyaltiesInfo{
			TransferPercentage: []*transaction.RoyaltyInfo{{Amount: -100, Percentage: 10}},
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create NFT asset with negative royalties fixed
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketFixed: -1000,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create NFT asset with negative royalties transfer
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			TransferFixed: -1000,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create NFT asset with invalid royalties percentage
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketPercentage: 1000000,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create NFT asset that cant mint
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Properties: &transaction.PropertiesInfo{
			CanMint: false,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Create NFT asset with mint already stopped
	contract = transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("KDA"),
		Ticker:       []byte("TEST"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Attributes: &transaction.AttributesInfo{
			IsNFTMintStopped: true,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

}

func TestTxProcessor_ProcessCreateAssetInvalidRoleAddressShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, kdautils.KLVIdentifier, testOwnerAddress)

	//Create asset with invalid role address
	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
		Roles: []*transaction.RolesInfo{
			{
				Address:     []byte("INVALID"),
				HasRoleMint: true,
			},
		},
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRoleAddr, err)
}

func TestTxProcessor_ProcessCreateAssetOkValsShouldWork(t *testing.T) {
	t.Parallel()

	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"),
		AdminAddress:  testAdminAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, contract.OwnerAddress, 0)

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, contract.GetOwnerAddress())

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Nil(t, err)

	ownerAcc := loadUserAccount(args.AccountsCacher, contract.GetOwnerAddress())
	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)

	assetIdentifier := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), contract.GetOwnerAddress(), ownerAcc.GetNonce(), contract.GetTicker())
	kdaKey := kdautils.ToKDAKey(assetIdentifier, nil)

	userKDABytes, err := ownerAcc.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	userKDA := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKDA, userKDABytes)
	assert.Nil(t, err)

	kdaDataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	kdaData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(kdaData, kdaDataBytes)
	assert.Nil(t, err)

	assert.Equal(t, contract.InitialSupply, userKDA.Balance)
	assert.Equal(t, int64(0), userKDA.FrozenBalance)
	assert.Equal(t, map[string]*kapps.UserBucket(nil), userKDA.Buckets)

	assert.Equal(t, kapps.KDAData_EnumAssetType(contract.Type), kdaData.AssetType)
	assert.Equal(t, contract.Name, kdaData.Name)
	assert.Equal(t, contract.Ticker, kdaData.Ticker)
	assert.Equal(t, contract.OwnerAddress, kdaData.OwnerAddress)
	assert.Equal(t, contract.AdminAddress, kdaData.AdminAddress)
	assert.Equal(t, contract.Precision, kdaData.Precision)
	assert.Equal(t, contract.InitialSupply, kdaData.InitialSupply)
	assert.Equal(t, contract.InitialSupply, kdaData.CirculatingSupply)
	assert.Equal(t, contract.MaxSupply, kdaData.MaxSupply)
}

func TestTxProcessor_ProcessTransferAssetOkValsShouldWork(t *testing.T) {
	t.Parallel()

	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 10000,
		MaxSupply:     10000,
		Staking:       &transaction.StakingInfo{MinEpochsToUnstake: 1, MinEpochsToClaim: 1},
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, contract.OwnerAddress, 0)

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, contract.GetOwnerAddress())

	block := createBlockHeader()

	ownAcc, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	assetIdentifier := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), contract.GetOwnerAddress(), ownAcc.GetNonce(), contract.GetTicker())

	transferContract := transaction.TransferContract{
		ToAddress: testToAddress,
		AssetID:   assetIdentifier,
		Amount:    1234,
	}

	tx, _ = createTransactionMock(&transferContract, transaction.TXContract_TransferContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Nil(t, err)

	err = args.AccountsCacher.SaveAll()
	assert.Nil(t, err)

	ownerAcc := loadUserAccount(args.AccountsCacher, contract.GetOwnerAddress())
	toAcc := loadUserAccount(args.AccountsCacher, transferContract.GetToAddress())

	kdaKey := kdautils.ToKDAKey(assetIdentifier, nil)

	ownerKDABytes, err := ownerAcc.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	ownerKDA := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(ownerKDA, ownerKDABytes)
	assert.Nil(t, err)

	toKDABytes, err := toAcc.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	toKDA := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(toKDA, toKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, contract.InitialSupply-transferContract.Amount, ownerKDA.Balance)
	assert.Equal(t, int64(0), ownerKDA.FrozenBalance)
	assert.Equal(t, map[string]*kapps.UserBucket(nil), ownerKDA.Buckets)

	assert.Equal(t, transferContract.Amount, toKDA.Balance)
	assert.Equal(t, int64(0), toKDA.FrozenBalance)
	assert.Equal(t, map[string]*kapps.UserBucket(nil), toKDA.Buckets)
}

func NewTXProcessor(t *testing.T, argsp ...pTX.ArgsNewTxProcessor) process.TransactionProcessor {
	args := createBaseKAppsProcessingArgs()
	if len(argsp) > 0 {
		args = argsp[0]
	}

	proposalKapp := loadKAppAccount(args.AccountsCacher, kapps.ProposalKAppAddress)
	pc := initProposalKapp(proposalKapp)
	err := args.AccountsCacher.SaveAll()
	assert.Nil(t, err)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)
	_ = execTx.SetProposalController(pc)

	InitKapps(args.KAppController, args.AccountsCacher, pc)

	return execTx
}

func NexTXProcessorV2(t *testing.T) (process.TransactionProcessor, pTX.ArgsNewTxProcessor) {
	_, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)

	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = peerDB.SaveAccount(peerAcc)

	argsp := createArgsForTxProcessor()
	argsp.AccountsCacher = accCacher

	proposalKapp := loadKAppAccount(argsp.AccountsCacher, kapps.ProposalKAppAddress)
	pc := initProposalKapp(proposalKapp)
	_ = argsp.AccountsCacher.SaveAll()

	execTx, err := pTX.NewTxProcessor(argsp)
	require.Nil(t, err)
	_ = execTx.SetProposalController(pc)

	InitKapps(argsp.KAppController, argsp.AccountsCacher, pc)

	return execTx, argsp
}

func AddBalanceAccount(db state.AccountsCacher, amount int64, asset []byte, address []byte) {
	acc, _ := db.LoadUser(address)
	_ = acc.AddToBalance(amount, asset, true)
	_ = db.SaveUser(acc)
}

func TestTxProcessor_ProcessFreezeInvalidAssetShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 1, kdautils.KLVIdentifier, testOwnerAddress)

	//Freeze invalid asset
	contract := transaction.FreezeContract{
		AssetID: []byte("INVALID"),
		Amount:  100_000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

}

func TestTxProcessor_ProcessFreezeSmallAmountShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, testOwnerAddress)

	//Freeze KLV with amount lower than min
	contract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount - 1,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

}

func TestTxProcessor_ProcessFreezeInvalidAmountShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, testOwnerAddress)

	//Freeze asset with negative amount
	contract := transaction.FreezeContract{
		AssetID: []byte("KDA"),
		Amount:  -1_000,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

}

func TestTxProcessor_ProcessUnfreezeInvalidAssetShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, testOwnerAddress)

	//Unfreeze with invalid asset
	ufzContract := transaction.UnfreezeContract{
		AssetID:  []byte("INVALID"),
		BucketID: []byte("BucketID"),
	}

	tx, _ := createTransactionMock(&ufzContract, transaction.TXContract_UnfreezeContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

}

func TestTxProcessor_ProcessUnfreezeInvalidBucketIDShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 90, nil, testOwnerAddress)

	//Unfreeze with invalid bucketID
	ufzContract := transaction.UnfreezeContract{
		AssetID:  kdautils.KLVIdentifier,
		BucketID: []byte("INVALID"),
	}

	tx, _ := createTransactionMock(&ufzContract, transaction.TXContract_UnfreezeContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, state.ErrNotStaked, err)
}

func TestTxProcessor_ProcessFreezeAndUnfreezeOkValsShouldWork(t *testing.T) {
	t.Parallel()

	frozenTxEpoch := uint32(1)
	fronzeTxTimestamp := time.Now().Add(time.Hour).Unix()
	timeBetweenTx := time.Hour
	unfrozenTxEpoch := frozenTxEpoch + 2
	unfronzeTxTimestamp := time.Unix(fronzeTxTimestamp, 0).Add(timeBetweenTx).Unix()
	initialKDAAmount := int64(10_000_000_000)
	frozenAmount := int64(1_234_000_000)
	kdaAPR := uint32(1600)
	expectedRewards := int64(float64(timeBetweenTx.Seconds()) * float64(frozenAmount) * float64(kdaAPR) / float64(core.OneYearTimestamp) / float64(core.HundredPercent))

	contract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf"),
		Precision:     6,
		InitialSupply: initialKDAAmount,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanFreeze: true,
			CanMint:   true,
		},
		Staking: &transaction.StakingInfo{MinEpochsToUnstake: 0, MinEpochsToClaim: 0, APR: kdaAPR},
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateAssetContractType, contract.GetOwnerAddress(), 0)

	execTx, args := NexTXProcessorV2(t)
	AddBalanceAccount(args.AccountsCacher, 1_000_000_000, nil, contract.GetOwnerAddress())

	ownerAcc := loadUserAccount(args.AccountsCacher, contract.GetOwnerAddress())

	block := createBlockHeader()
	block.Header.Epoch = frozenTxEpoch
	block.Header.Timestamp = fronzeTxTimestamp

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	assetIdentifier := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), contract.GetOwnerAddress(), ownerAcc.GetNonce(), contract.GetTicker())

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeAssetContract := transaction.FreezeContract{
		AssetID: assetIdentifier,
		Amount:  frozenAmount,
	}

	freezeTX1, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, contract.OwnerAddress, 0)
	freezeTX2, _ := createTransactionMock(&freezeAssetContract, transaction.TXContract_FreezeContractType, contract.OwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(freezeTX1)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, freezeTX1)
	assert.Nil(t, err)

	_, hash, err = execTx.PreProcessTransaction(freezeTX2)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, freezeTX2)
	assert.Nil(t, err)

	bucketID := kdautils.ToBucketID(args.Hasher, block.GetRandSeed(), contract.OwnerAddress, freezeContract.AssetID, ownerAcc.GetNonce(), 0, freezeContract.Amount)

	ownerAcc = loadUserAccount(args.AccountsCacher, contract.GetOwnerAddress())
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)

	kdaKey := kdautils.ToKDAKey(freezeAssetContract.AssetID, nil)
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)

	userKDABytes, err := ownerAcc.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	userKDAFreeze := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKDAFreeze, userKDABytes)
	assert.Nil(t, err)

	userKLVBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKLVFreeze := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKLVFreeze, userKLVBytes)
	require.Nil(t, err)

	stakingKDABytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	stakingKDAFreeze := &kapps.StakingData{}
	err = marshalizer.Unmarshal(stakingKDAFreeze, stakingKDABytes)
	assert.Nil(t, err)

	stakingKLVBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	stakingKLVFreeze := &kapps.StakingData{}
	err = marshalizer.Unmarshal(stakingKLVFreeze, stakingKLVBytes)
	assert.Nil(t, err)

	assert.Equal(t, initialKDAAmount-frozenAmount, userKDAFreeze.Balance)
	assert.Equal(t, frozenAmount, userKDAFreeze.FrozenBalance)
	assert.Equal(t, frozenTxEpoch, userKDAFreeze.LastClaim.Epoch)
	assert.Equal(t, fronzeTxTimestamp, userKDAFreeze.LastClaim.Timestamp)
	assert.Equal(t, frozenTxEpoch, userKDAFreeze.Buckets[string(freezeAssetContract.AssetID)].StakedEpoch)
	assert.Equal(t, uint32(4294967295), userKDAFreeze.Buckets[string(freezeAssetContract.AssetID)].UnstakedEpoch)
	assert.Equal(t, frozenAmount, userKDAFreeze.Buckets[string(freezeAssetContract.AssetID)].Value)
	assert.Equal(t, 0, len(userKDAFreeze.Buckets[string(freezeAssetContract.AssetID)].Delegation))

	assert.Equal(t, int64(0), userKLVFreeze.Balance)
	assert.Equal(t, MinFreezeAmount, userKLVFreeze.FrozenBalance)
	assert.Equal(t, frozenTxEpoch, userKLVFreeze.LastClaim.Epoch)
	assert.Equal(t, frozenTxEpoch, userKLVFreeze.Buckets[hex.EncodeToString(bucketID)].StakedEpoch)
	assert.Equal(t, uint32(4294967295), userKLVFreeze.Buckets[hex.EncodeToString(bucketID)].UnstakedEpoch)
	assert.Equal(t, MinFreezeAmount, userKLVFreeze.Buckets[hex.EncodeToString(bucketID)].Value)
	assert.Equal(t, 0, len(userKLVFreeze.Buckets[hex.EncodeToString(bucketID)].Delegation))

	assert.Equal(t, kapps.StakingData_APRI, stakingKDAFreeze.InterestType)
	assert.Equal(t, frozenAmount, stakingKDAFreeze.TotalStaked)
	assert.Equal(t, uint32(0), stakingKDAFreeze.MinEpochsToUnstake)
	assert.Equal(t, uint32(0), stakingKDAFreeze.MinEpochsToClaim)
	assert.Equal(t, uint32(1), stakingKDAFreeze.APR[0].Epoch)
	assert.Equal(t, kdaAPR, stakingKDAFreeze.APR[0].Value)
	assert.Equal(t, 0, len(stakingKDAFreeze.FPR))

	assert.Equal(t, kapps.StakingData_FPRI, stakingKLVFreeze.InterestType)
	assert.Equal(t, MinFreezeAmount+initialStaking, stakingKLVFreeze.TotalStaked)
	assert.Equal(t, uint32(0), stakingKLVFreeze.MinEpochsToUnstake)
	assert.Equal(t, uint32(0), stakingKLVFreeze.MinEpochsToClaim)
	assert.Equal(t, uint32(1), stakingKLVFreeze.FPR[0].Epoch)
	assert.Equal(t, int64(1000), stakingKLVFreeze.FPR[0].TotalAmount)
	assert.Equal(t, int64(50000), stakingKLVFreeze.FPR[0].TotalStaked)
	assert.Equal(t, 0, len(stakingKLVFreeze.APR))

	unfreezeContract := transaction.UnfreezeContract{
		AssetID:  nil,
		BucketID: bucketID,
	}

	unfreezeAssetContract := transaction.UnfreezeContract{
		AssetID:  assetIdentifier,
		BucketID: nil,
	}

	unfreezeTX1, _ := createTransactionMock(&unfreezeContract, transaction.TXContract_UnfreezeContractType, contract.OwnerAddress, 0)
	unfreezeTX2, _ := createTransactionMock(&unfreezeAssetContract, transaction.TXContract_UnfreezeContractType, contract.OwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(unfreezeTX1)
	assert.Nil(t, err)

	block.Header.Epoch = unfrozenTxEpoch
	block.Header.Timestamp = unfronzeTxTimestamp

	err = execTx.ProcessTransaction(block, hash, unfreezeTX1)
	assert.Nil(t, err)

	_, hash, err = execTx.PreProcessTransaction(unfreezeTX2)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, unfreezeTX2)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(args.AccountsCacher, contract.GetOwnerAddress())
	stakingKapp = loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)

	userKDABytes, err = ownerAcc.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	userKDAUnfreeze := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKDAUnfreeze, userKDABytes)
	assert.Nil(t, err)

	userKLVBytes, err = ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKLVUnfreeze := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKLVUnfreeze, userKLVBytes)
	assert.Nil(t, err)

	stakingKDABytes, err = stakingKapp.DataTrieTracker().RetrieveValue(kdaKey)
	assert.Nil(t, err)

	stakingKDAUnfreeze := &kapps.StakingData{}
	err = marshalizer.Unmarshal(stakingKDAUnfreeze, stakingKDABytes)
	assert.Nil(t, err)

	stakingKLVBytes, err = stakingKapp.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	stakingKLVUnfreeze := &kapps.StakingData{}
	err = marshalizer.Unmarshal(stakingKLVUnfreeze, stakingKLVBytes)
	assert.Nil(t, err)

	// balance should update after unfreeze
	// 1234000 * 16% * block diff time
	assert.Equal(t, initialKDAAmount-frozenAmount+expectedRewards, userKDAUnfreeze.Balance)
	assert.Equal(t, int64(0), userKDAUnfreeze.FrozenBalance)
	assert.Equal(t, unfrozenTxEpoch, userKDAUnfreeze.LastClaim.Epoch)
	assert.Equal(t, unfronzeTxTimestamp, userKDAUnfreeze.LastClaim.Timestamp)
	assert.Equal(t, frozenTxEpoch, userKDAUnfreeze.Buckets[string(freezeAssetContract.AssetID)].StakedEpoch)
	assert.Equal(t, unfrozenTxEpoch, userKDAUnfreeze.Buckets[string(freezeAssetContract.AssetID)].UnstakedEpoch)
	assert.Equal(t, frozenAmount, userKDAUnfreeze.Buckets[string(freezeAssetContract.AssetID)].Value)
	assert.Equal(t, 0, len(userKDAUnfreeze.Buckets[string(freezeAssetContract.AssetID)].Delegation))

	assert.Equal(t, int64(0), userKLVUnfreeze.Balance)
	assert.Equal(t, int64(0), userKLVUnfreeze.FrozenBalance)
	assert.Equal(t, unfrozenTxEpoch, userKLVUnfreeze.LastClaim.Epoch)
	assert.Equal(t, frozenTxEpoch, userKLVUnfreeze.Buckets[hex.EncodeToString(bucketID)].StakedEpoch)
	assert.Equal(t, unfrozenTxEpoch, userKLVUnfreeze.Buckets[hex.EncodeToString(bucketID)].UnstakedEpoch)
	assert.Equal(t, MinFreezeAmount, userKLVUnfreeze.Buckets[hex.EncodeToString(bucketID)].Value)
	assert.Equal(t, 0, len(userKLVUnfreeze.Buckets[hex.EncodeToString(bucketID)].Delegation))

	assert.Equal(t, kapps.StakingData_APRI, stakingKDAUnfreeze.InterestType)
	assert.Equal(t, int64(0), stakingKDAUnfreeze.TotalStaked)
	assert.Equal(t, uint32(0), stakingKDAUnfreeze.MinEpochsToUnstake)
	assert.Equal(t, uint32(0), stakingKDAUnfreeze.MinEpochsToClaim)
	assert.Equal(t, uint32(1), stakingKDAUnfreeze.APR[0].Epoch)
	assert.Equal(t, kdaAPR, stakingKDAUnfreeze.APR[0].Value) //
	assert.Equal(t, 0, len(stakingKDAUnfreeze.FPR))

	assert.Equal(t, kapps.StakingData_FPRI, stakingKLVUnfreeze.InterestType)
	assert.Equal(t, initialStaking, stakingKLVUnfreeze.TotalStaked)
	assert.Equal(t, uint32(0), stakingKLVUnfreeze.MinEpochsToUnstake)
	assert.Equal(t, uint32(0), stakingKLVUnfreeze.MinEpochsToClaim)
	assert.Equal(t, uint32(1), stakingKLVUnfreeze.FPR[0].Epoch)
	assert.Equal(t, int64(1000), stakingKLVUnfreeze.FPR[0].TotalAmount)
	assert.Equal(t, int64(50000), stakingKLVUnfreeze.FPR[0].TotalStaked)
	assert.Equal(t, 0, len(stakingKLVUnfreeze.APR))
}

func TestTxProcessor_ProcessDelegateAndUndelegateWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Delegate with invalid to address
	contract := transaction.DelegateContract{
		ToAddress: []byte("INVALID"),
		BucketID:  []byte("BucketID"),
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_DelegateContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//Delegate with invalid bucketID
	contract = transaction.DelegateContract{
		ToAddress: testToAddress,
		BucketID:  []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_DelegateContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrNilTrie, err)

	//Undelegate with invalid bucketID
	udgContract := transaction.UndelegateContract{
		BucketID: []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&udgContract, transaction.TXContract_UndelegateContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrNilTrie, err)
}

func TestTxProcessor_ProcessDelegateAndUndelegateOkValsShouldWork(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")
	peerAddress2 := []byte("52f3e4d40ec83d109c3d346b5adfb87bbaee1b3369166d0e3bca472b0f38caab0327a01eca784c474a5e2126aec2e604a3082320301afda05765b4f7eb9f69cd67c94d2d4acc713f814611f15b91888ffda86d135eaaf18f1efac5bbeb1dd08f")

	contract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	toAcc := loadUserAccount(accCacher, testToAddress)
	peerAcc := loadPeerAccount(accCacher, peerAddress)
	peerAcc2 := loadPeerAccount(accCacher, peerAddress2)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	validatorsKapp := loadKAppAccount(accCacher, kapps.ValidatorsKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = ownerAcc.AddToBalance(10_000_000_000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(validatorsKapp)
	_ = peerDB.SaveAccount(peerAcc)
	_ = peerDB.SaveAccount(peerAcc2)

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
	assert.Nil(t, err)

	controller := createProposalController()
	_ = kAppController.SetProposalController(controller)

	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)

	args.AccountsCacher = accCacher
	args.KAppController = kAppController

	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	bucketID := kdautils.ToBucketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, contract.AssetID, ownerAcc.GetNonce(), 0, contract.Amount)

	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)

	kappContext := kapp.NewKappContext(
		kapp.ArgsNewKAppContext{
			ContractID:   0,
			ContractType: transaction.TXContract_CreateValidatorContractType,
			Block:        block,
			TxNonce:      0,
		})
	args.KAppController.SetCurrentKAppContext(kappContext)

	//Creating Validators
	_, err = args.KAppController.GetValidatorsKApp().Register(
		&transaction.CreateValidatorContract{
			OwnerAddress: testToAddress,
			Config: &transaction.ValidatorConfig{
				BLSPublicKey:        peerAddress,
				RewardAddress:       testReferralAddress,
				CanDelegate:         true,
				MaxDelegationAmount: 100_000_000_000,
			},
		})
	assert.Nil(t, err)

	kappContext = kapp.NewKappContext(
		kapp.ArgsNewKAppContext{
			ContractID:   0,
			ContractType: transaction.TXContract_CreateValidatorContractType,
			Block:        block,
			TxNonce:      0,
		})
	args.KAppController.SetCurrentKAppContext(kappContext)

	_, err = args.KAppController.GetValidatorsKApp().Register(

		&transaction.CreateValidatorContract{
			OwnerAddress: testOwnerAddress,
			Config: &transaction.ValidatorConfig{
				BLSPublicKey:        peerAddress2,
				RewardAddress:       testReferralAddress,
				CanDelegate:         true,
				MaxDelegationAmount: 100_000_000_000,
			},
		})
	assert.Nil(t, err)

	err = accCacher.SaveAll()
	require.Nil(t, err)

	//Delegations

	delegateContract := transaction.DelegateContract{
		ToAddress: testOwnerAddress,
		BucketID:  bucketID,
	}

	dgTX, _ := createTransactionMock(&delegateContract, transaction.TXContract_DelegateContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(dgTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, dgTX)
	assert.Nil(t, err)

	delegateContract = transaction.DelegateContract{
		ToAddress: testToAddress,
		BucketID:  bucketID,
	}

	dgTX, _ = createTransactionMock(&delegateContract, transaction.TXContract_DelegateContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(dgTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, dgTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	validatorsKapp = loadKAppAccount(accCacher, kapps.ValidatorsKAppAddress)

	userKDABytes, err := ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKDADelegate := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKDADelegate, userKDABytes)
	assert.Nil(t, err)

	peerKDABytes, err := validatorsKapp.DataTrieTracker().RetrieveValue(append([]byte(validators.VALIDATOR_BUCKETS+kapps.Sp), testToAddress...))
	assert.Nil(t, err)

	peerKDADelegate := &validators.PeerData{}
	err = marshalizer.Unmarshal(peerKDADelegate, peerKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(0), userKDADelegate.Balance)
	assert.Equal(t, int64(MinFreezeAmount), userKDADelegate.FrozenBalance)
	assert.Equal(t, uint32(0), userKDADelegate.LastClaim.Epoch)
	assert.Equal(t, uint32(0), userKDADelegate.Buckets[hex.EncodeToString(bucketID)].StakedEpoch)
	assert.Equal(t, uint32(4294967295), userKDADelegate.Buckets[hex.EncodeToString(bucketID)].UnstakedEpoch)
	assert.Equal(t, int64(MinFreezeAmount), userKDADelegate.Buckets[hex.EncodeToString(bucketID)].Value)
	assert.Equal(t, delegateContract.ToAddress, userKDADelegate.Buckets[hex.EncodeToString(bucketID)].Delegation)

	assert.Equal(t, uint32(0), peerKDADelegate.Buckets[hex.EncodeToString(bucketID)].DelegatedEpoch)
	assert.Equal(t, uint32(4294967295), peerKDADelegate.Buckets[hex.EncodeToString(bucketID)].UndelegatedEpoch)
	assert.Equal(t, int64(MinFreezeAmount), peerKDADelegate.Buckets[hex.EncodeToString(bucketID)].Value)
	assert.Equal(t, testOwnerAddress, peerKDADelegate.Buckets[hex.EncodeToString(bucketID)].Address)

	undelegateContract := transaction.UndelegateContract{
		BucketID: bucketID,
	}

	udgTX, _ := createTransactionMock(&undelegateContract, transaction.TXContract_UndelegateContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(udgTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, udgTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userKDABytes, err = ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKDAUnDelegate := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKDAUnDelegate, userKDABytes)
	assert.Nil(t, err)

	peerKDABytesUndelegate, err := validatorsKapp.DataTrieTracker().RetrieveValue(append([]byte(validators.VALIDATOR_BUCKETS+kapps.Sp), testOwnerAddress...))
	assert.Nil(t, err)

	peerKDAUnDelegate := &validators.PeerData{}
	err = marshalizer.Unmarshal(peerKDAUnDelegate, peerKDABytesUndelegate)
	assert.Nil(t, err)

	assert.Equal(t, int64(0), userKDAUnDelegate.Balance)
	assert.Equal(t, int64(MinFreezeAmount), userKDAUnDelegate.FrozenBalance)
	assert.Equal(t, uint32(0), userKDAUnDelegate.LastClaim.Epoch)
	assert.Equal(t, uint32(0), userKDAUnDelegate.Buckets[hex.EncodeToString(bucketID)].StakedEpoch)
	assert.Equal(t, uint32(4294967295), userKDAUnDelegate.Buckets[hex.EncodeToString(bucketID)].UnstakedEpoch)
	assert.Equal(t, int64(MinFreezeAmount), userKDAUnDelegate.Buckets[hex.EncodeToString(bucketID)].Value)
	assert.Equal(t, []byte(nil), userKDAUnDelegate.Buckets[hex.EncodeToString(bucketID)].Delegation)

	assert.Nil(t, peerKDAUnDelegate.Buckets[hex.EncodeToString(bucketID)])
}

func TestTxProcessor_ProcessWithdrawWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Withdraw with invalid asset
	contract := transaction.WithdrawContract{
		AssetID: []byte("INVALID"),
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_WithdrawContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrNilTrie, err)
}

func TestTxProcessor_ProcessWithdrawOkValsShouldWork(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)

	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)

	userKDA := kapps.UserKDA{
		Balance:       0,
		LastClaim:     &kapps.LastClaim{},
		FrozenBalance: 0,
		Buckets:       make(map[string]*kapps.UserBucket),
	}

	userKDA.Buckets["MY_BUCKET"] = &kapps.UserBucket{
		StakedAt:      time.Now().AddDate(0, 0, -15).Unix(),
		StakedEpoch:   0,
		UnstakedEpoch: 5,
		Value:         12345,
		Delegation:    nil,
	}

	klvBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(klvKey, klvBucket)
	assert.Nil(t, err)

	userKDA.Buckets["MY_BUCKET"].Value = 6789

	kfiBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(kfiKey, kfiBucket)
	assert.Nil(t, err)

	_ = ownerAcc.AddToBalance(1000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	block.Header.Epoch = 16

	withdrawContract := transaction.WithdrawContract{
		AssetID: kdautils.KLVIdentifier,
	}

	wtdTX, _ := createTransactionMock(&withdrawContract, transaction.TXContract_WithdrawContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(wtdTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, wtdTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userKLVBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKLVWithdraw := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKLVWithdraw, userKLVBytes)
	assert.Nil(t, err)

	withdrawKFIContract := transaction.WithdrawContract{
		AssetID: kdautils.KFIIdentifier,
	}

	kfiTX, _ := createTransactionMock(&withdrawKFIContract, transaction.TXContract_WithdrawContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(kfiTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, kfiTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userKFIBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(kfiKey)
	assert.Nil(t, err)

	userKFIWithdraw := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKFIWithdraw, userKFIBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(13345), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(6789), ownerAcc.GetBalance(kdautils.KFIIdentifier, true))
	assert.Equal(t, 0, len(userKLVWithdraw.Buckets))
	assert.Equal(t, 0, len(userKFIWithdraw.Buckets))
}

func TestTxProcessor_ProcessClaimWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	execTx, args := NexTXProcessorV2(t)

	_ = loadKAppAccount(args.AccountsCacher, kapps.MarketKAppAddress)
	_ = args.AccountsCacher.SaveAll()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	AddBalanceAccount(args.AccountsCacher, 100_000_000, nil, testOwnerAddress)
	AddBalanceAccount(args.AccountsCacher, 100_000_000, nil, testToAddress)

	peerAcc := loadPeerAccount(args.AccountsCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	_ = loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	_ = loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)

	args.AccountsCacher.SaveAll()
	//Claim with invalid type
	contract := transaction.ClaimContract{
		ClaimType: 9,
		ID:        kdautils.KLVIdentifier,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrClaimTypeInvalid, err)

	//Claim staking with invalid asset id
	contract = transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrStakingNotFound, err)

	//Claim market with invalid market id
	contract = transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_MarketClaim,
		ID:        []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrNotFoundInKApp, err)
}

func TestTxProcessor_ProcessClaimStakingShouldWork(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)

	epoch0Time := time.Now().AddDate(0, 0, -10).Unix()

	_ = ownerAcc.AddToBalance(1000000, kdautils.KLVIdentifier, true)

	userKDA := kapps.UserKDA{
		Balance:       1000000,
		LastClaim:     &kapps.LastClaim{Epoch: 0, Timestamp: epoch0Time},
		FrozenBalance: 10000,
		Buckets:       make(map[string]*kapps.UserBucket),
	}

	userKDA.Buckets["MY_BUCKET"] = &kapps.UserBucket{
		StakedAt:      epoch0Time,
		StakedEpoch:   0,
		UnstakedEpoch: core.DefaultUnstakedEpoch,
		Value:         10000,
		Delegation:    nil,
	}

	APR_BALANCE := int64(1000000)
	APR_FROZEN := int64(20_000_000_000_000_000)
	APR := float64(1000) / float64(core.HundredPercent)
	v, err := time.ParseDuration("241h")
	require.Nil(t, err)
	COMPUTE_TIME := float64(v.Seconds())
	APR_REWARDS := int64(COMPUTE_TIME * float64(APR_FROZEN) * APR / float64(core.OneYearTimestamp))
	fmt.Println("reqrads", APR_REWARDS)
	userKDA_APR := kapps.UserKDA{
		Balance:       APR_BALANCE,
		LastClaim:     &kapps.LastClaim{Epoch: 0, Timestamp: epoch0Time},
		FrozenBalance: APR_FROZEN,
		Buckets: map[string]*kapps.UserBucket{
			"APR": {
				StakedAt:      epoch0Time,
				StakedEpoch:   0,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
				Value:         APR_FROZEN,
				Delegation:    nil,
			},
		},
	}

	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)
	aprKey := kdautils.ToKDAKey([]byte("APR"), nil)

	klvBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(klvKey, klvBucket)
	assert.Nil(t, err)

	kfiBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(kfiKey, kfiBucket)
	assert.Nil(t, err)

	aprBucket, err := marshalizer.Marshal(&userKDA_APR)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(aprKey, aprBucket)
	assert.Nil(t, err)

	_ = userDB.SaveAccount(ownerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	block.Header.Epoch = 8

	klvClaim := transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        kdautils.KLVIdentifier,
	}

	klvTX, _ := createTransactionMock(&klvClaim, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(klvTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, klvTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userKLVBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(klvKey)
	assert.Nil(t, err)

	userKLVClaim := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKLVClaim, userKLVBytes)
	assert.Nil(t, err)

	kfiClaim := transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        kdautils.KFIIdentifier,
	}

	kfiTX, _ := createTransactionMock(&kfiClaim, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(kfiTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, kfiTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userKFIBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(kfiKey)
	assert.Nil(t, err)

	userKFIClaim := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userKFIClaim, userKFIBytes)
	assert.Nil(t, err)

	aprClaim := transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        []byte("APR"),
	}

	aprTX, _ := createTransactionMock(&aprClaim, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(aprTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, aprTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userAPRBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(aprKey)
	assert.Nil(t, err)

	userAPRClaim := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userAPRClaim, userAPRBytes)
	assert.Nil(t, err)

	klvStakingBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kfiKey)
	assert.Nil(t, err)

	klvStaking := &kapps.StakingData{}
	err = marshalizer.Unmarshal(klvStaking, klvStakingBytes)
	assert.Nil(t, err)

	kfiStakingBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(kfiKey)
	assert.Nil(t, err)

	kfiStaking := &kapps.StakingData{}
	err = marshalizer.Unmarshal(kfiStaking, kfiStakingBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(1001862), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(1000000), ownerAcc.GetBalance(kdautils.KFIIdentifier, true))
	assert.Equal(t, int64(APR_BALANCE+APR_REWARDS), ownerAcc.GetBalance([]byte("APR"), true))
	assert.Equal(t, int64(block.Header.Timestamp), userKLVClaim.LastClaim.Timestamp)
	assert.Equal(t, uint32(block.Header.Epoch), userKLVClaim.LastClaim.Epoch)
	assert.Equal(t, block.Header.Timestamp, userKFIClaim.LastClaim.Timestamp)
	assert.Equal(t, uint32(block.Header.Epoch), userKFIClaim.LastClaim.Epoch)
	assert.Equal(t, block.Header.Timestamp, userAPRClaim.LastClaim.Timestamp)
	assert.Equal(t, uint32(block.Header.Epoch), userAPRClaim.LastClaim.Epoch)

	assert.Equal(t, int64(200), klvStaking.FPR[0].TotalClaimed) // 10k/50k * 1k = 200
	assert.Equal(t, int64(96), klvStaking.FPR[1].TotalClaimed)  // 10k/52k * 500 = 96
	assert.Equal(t, int64(302), klvStaking.FPR[2].TotalClaimed) // 10k/43k * 1k3 = 302
	assert.Equal(t, int64(333), klvStaking.FPR[3].TotalClaimed) // 10k/30k * 1k = 333

	assert.Equal(t, int64(200), kfiStaking.FPR[0].TotalClaimed) // 10k/50k * 1k = 200
	assert.Equal(t, int64(96), kfiStaking.FPR[1].TotalClaimed)  // 10k/52k * 500 = 96
	assert.Equal(t, int64(302), kfiStaking.FPR[2].TotalClaimed) // 10k/43k * 1k3 = 302
	assert.Equal(t, int64(333), kfiStaking.FPR[3].TotalClaimed) // 10k/30k * 1k = 333

}

func TestTxProcessor_ProcessClaimAllowanceOkValsShouldWork(t *testing.T) {
	t.Parallel()

	contract := transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim}
	err := contract.Validate()
	assert.Nil(t, err)

	tx, _ := createTransactionMock(&contract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	args := createArgsForTxProcessor()
	acntSrc := loadUserAccount(args.AccountsCacher, testOwnerAddress)
	acntSrc.AddToBalance(90, nil, true)
	acntSrc.AddToAllowance(123)
	args.AccountsCacher.SaveAll()
	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	assert.Nil(t, err)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)
	acntSrc = loadUserAccount(args.AccountsCacher, testOwnerAddress)
	assert.Equal(t, int64(213), acntSrc.GetBalance(nil, true))
	assert.Equal(t, int64(0), acntSrc.GetAllowance())
}

func TestTxProcessor_ProcessClaimAllowanceKFIShouldFail(t *testing.T) {
	t.Parallel()

	contract := transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim, ID: kdautils.KFIIdentifier}
	err := contract.Validate()
	assert.Equal(t, common.ErrAssetIDInvalid, err)

	tx, _ := createTransactionMock(&contract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	args := createArgsForTxProcessor()
	acntSrc := loadUserAccount(args.AccountsCacher, testOwnerAddress)
	acntSrc.AddToBalance(90, nil, true)
	acntSrc.AddToAllowance(123)
	err = args.AccountsCacher.SaveAll()
	assert.Nil(t, err)

	SetupKappController(t, &args)
	execTx, err := pTX.NewTxProcessor(args)
	assert.Nil(t, err)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	acntSrc = loadUserAccount(args.AccountsCacher, testOwnerAddress)

	assert.Equal(t, common.ErrAssetIDInvalid, err)
	assert.Equal(t, int64(90), acntSrc.GetBalance(nil, true))
	assert.Equal(t, int64(123), acntSrc.GetAllowance())
}

func TestTxProcessor_ProcessAssetTriggerWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	args := createBaseKAppsProcessingArgs()
	kdaKapp := loadKAppAccount(args.AccountsCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(args.AccountsCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(args.AccountsCacher, kapps.ProposalKAppAddress)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	err := args.AccountsCacher.SaveAll()
	require.Nil(t, err)

	kappArgs := kappcontroller.ArgsNewKApp{
		Hasher:         args.Hasher,
		Marshalizer:    args.Marshalizer,
		PubkeyConv:     args.PubkeyConv,
		ForkController: args.ForkController,
		AccountsCacher: args.AccountsCacher,
		RatingsData:    args.RatingsData,
	}

	pc := initProposalKapp(proposalKapp)

	args.KAppController, err = kappcontroller.NewKappController(kappArgs)
	require.NoError(t, err)

	InitKapps(args.KAppController, args.AccountsCacher, pc)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	_ = execTx.SetProposalController(pc)

	//Asset Creation to trigger
	kdaContract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("NFT"),
		Ticker:       []byte("NFT"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10_000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
		Staking: &transaction.StakingInfo{MinEpochsToUnstake: 0, MinEpochsToClaim: 0, APR: 16},
	}

	tx, _ := createTransactionMock(&kdaContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	block := createBlockHeader()

	ownerAcc, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	assetID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), kdaContract.GetOwnerAddress(), ownerAcc.GetNonce(), kdaContract.GetTicker())

	//Asset Trigger with wrong type
	contract := transaction.AssetTriggerContract{
		TriggerType: 999,
		AssetID:     assetID,
		ToAddress:   testOwnerAddress,
		Amount:      9999,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetTriggerInvalid, err)

	//Asset Trigger with invalid assetID
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     []byte("INVALID"),
		ToAddress:   testOwnerAddress,
		Amount:      9999,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//Mint with invalid ToAddress
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetID,
		ToAddress:   []byte("INVALID"),
		Amount:      10,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//Mint with invalid amount
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetID,
		ToAddress:   testOwnerAddress,
		Amount:      9999999,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidArgument, err)

	//Mint without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetID,
		ToAddress:   testOwnerAddress,
		Amount:      1,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrRoleNotFound, err)

	//Burn with invalid assetID
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Burn,
		AssetID:     []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//Wipe with invalid ToAddress
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Wipe,
		AssetID:     assetID,
		ToAddress:   []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//Wipe without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Wipe,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Pause without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Pause,
		AssetID:     assetID,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Resume without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Resume,
		AssetID:     assetID,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//ChangeOwner without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeOwner,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//ChangeOwner with invalid ToAddress
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeOwner,
		AssetID:     assetID,
		ToAddress:   []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//AddRole without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_AddRole,
		AssetID:     assetID,
		Role: &transaction.RolesInfo{
			Address:     testToAddress,
			HasRoleMint: true,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//AddRole with invalid address
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_AddRole,
		AssetID:     assetID,
		Role: &transaction.RolesInfo{
			Address:     []byte("INVALID"),
			HasRoleMint: true,
		},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//RemoveRole without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_RemoveRole,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateMetadata without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateMetadata data size
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetIDInvalid, err)

	//UpdateMetadata data size
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     []byte(fmt.Sprintf("%s%s%d", assetID, kapps.Sp, 1)),
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)
	tx.RawData.Data = [][]byte{[]byte("data")}

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateMetadata with invalid address
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     assetID,
		ToAddress:   []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//StopNFTMint without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMint,
		AssetID:     assetID,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//StopNFTMetadataChange without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMetadataChange,
		AssetID:     assetID,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateLogo without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateLogo,
		AssetID:     assetID,
		Logo:        "",
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateLogo with invalid UTF8 logo
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateLogo,
		AssetID:     assetID,
		Logo:        "\xf0\x28\x8c\x28",
	}

	tx, err = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)
	assert.Equal(t, "could not serialize contract", err.Error())
	assert.Nil(t, tx)

	//UpdateURIs without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateURIs,
		AssetID:     assetID,
		URIs:        make(map[string]string),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//UpdateURIs with invalid UTF8 logo
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateURIs,
		AssetID:     assetID,
		URIs:        map[string]string{"test": "\xf0\x28\x8c\x28"},
	}

	tx, err = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)
	assert.Equal(t, "could not serialize contract", err.Error())
	assert.Nil(t, tx)

	//Change royalties receiver without beign asset owner
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeRoyaltiesReceiver,
		AssetID:     assetID,
		ToAddress:   testToAddress,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAccNotOwner, err)

	//Change royalties receiver with invalid address
	contract = transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeRoyaltiesReceiver,
		AssetID:     assetID,
		ToAddress:   []byte("INVALID"),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)
}

func TestTxProcessor_ProcessAssetTriggerOkValsShouldWork(t *testing.T) {
	t.Parallel()

	fungibleContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("KDA"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 1000000,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	nftContract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("NFT"),
		Ticker:       []byte("NFT"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	fungibleTX, _ := createTransactionMock(&fungibleContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)
	nftTX, _ := createTransactionMock(&nftContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	execTx, args := NexTXProcessorV2(t)
	accCacher := args.AccountsCacher
	AddBalanceAccount(accCacher, 1000, kdautils.KLVIdentifier, testOwnerAddress)

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	block := createBlockHeader()

	//KDA creation ###################################################

	_, hash, err := execTx.PreProcessTransaction(fungibleTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTX)
	assert.Nil(t, err)

	fungibleID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), fungibleContract.GetTicker())

	_, hash, err = execTx.PreProcessTransaction(nftTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTX)
	assert.Nil(t, err)

	nftID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), nftContract.GetTicker())

	//MINT FUNGIBLE ######################################################

	fungibleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     fungibleID,
		ToAddress:   testOwnerAddress,
		Amount:      9999,
	}

	fungibleTriggerTX, _ := createTransactionMock(&fungibleTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTriggerTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	fungibleKey := kdautils.ToKDAKey(fungibleID, nil)

	userFungibleBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	userFungible := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userFungible, userFungibleBytes)
	assert.Nil(t, err)

	fungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleKDAData, fungibleKDADataBytes)
	assert.Nil(t, err)

	//MINT NFT ######################################################

	nftTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     nftID,
		ToAddress:   testOwnerAddress,
		Amount:      2,
	}

	nftTriggerTX, _ := createTransactionMock(&nftTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	nftKey := kdautils.ToKDAKey(nftID, nil)
	userNFTKey := kdautils.ToKDAKey(nftID, []byte("1"))

	userNFTKey2 := kdautils.ToKDAKey(nftID, []byte("2"))

	userNFTBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFT, userNFTBytes)
	assert.Nil(t, err)

	nftKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftKDAData, nftKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(1009999), ownerAcc.GetBalance(fungibleID, true))
	assert.Equal(t, int64(1009999), fungibleKDAData.MintedValue)
	assert.Equal(t, int64(1009999), fungibleKDAData.CirculatingSupply)
	assert.Greater(t, len(userNFTBytes), 0)
	assert.Equal(t, []uint8([]byte(nil)), userNFT.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFT.Metadata)
	assert.Equal(t, int64(2), nftKDAData.MintedValue)
	assert.Equal(t, int64(2), nftKDAData.CirculatingSupply)

	//PAUSE ######################################################

	pauseTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Pause,
		AssetID:     fungibleID,
	}

	pauseTriggerTX, _ := createTransactionMock(&pauseTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(pauseTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, pauseTriggerTX)
	assert.Nil(t, err)

	pausedKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	pausedKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(pausedKDAData, pausedKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, pausedKDAData.Attributes.IsPaused)

	//RESUME ######################################################

	resumeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Resume,
		AssetID:     fungibleID,
	}

	resumeTriggerTX, _ := createTransactionMock(&resumeTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(resumeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, resumeTriggerTX)
	assert.Nil(t, err)

	resumedKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	resumedKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(resumedKDAData, resumedKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, false, resumedKDAData.Attributes.IsPaused)

	//ADD ROLE ######################################################

	addRoleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_AddRole,
		AssetID:     fungibleID,
		Role: &transaction.RolesInfo{
			Address:     []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
			HasRoleMint: true,
		},
	}

	addRoleTriggerTX, _ := createTransactionMock(&addRoleTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(addRoleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, addRoleTriggerTX)
	assert.Nil(t, err)

	addRoleKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	addRoleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(addRoleKDAData, addRoleKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, addRoleTrigger.Role.Address, addRoleKDAData.Roles[0].Address)
	assert.Equal(t, true, addRoleKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, false, addRoleKDAData.Roles[0].HasRoleSetITOPrices)

	//REMOVE ROLE ######################################################

	removeRoleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_RemoveRole,
		AssetID:     fungibleID,
		ToAddress:   []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
	}

	removeRoleTriggerTX, _ := createTransactionMock(&removeRoleTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(removeRoleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, removeRoleTriggerTX)
	assert.Nil(t, err)

	removeRoleKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	removeRoleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(removeRoleKDAData, removeRoleKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(removeRoleKDAData.Roles))

	//UPDATE Logo ######################################################

	updateLogoTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateLogo,
		AssetID:     fungibleID,
		Logo:        "https://github.com/klever-io/klever-go",
	}

	updateLogoTriggerTX, _ := createTransactionMock(&updateLogoTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateLogoTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, updateLogoTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	updateLogoKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	updateLogoKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(updateLogoKDAData, updateLogoKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, "https://github.com/klever-io/klever-go", updateLogoKDAData.Logo)

	//UPDATE URIS ######################################################

	updateURIsTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateURIs,
		AssetID:     fungibleID,
		URIs:        map[string]string{"Github": "https://github.com/klever-io/klever-go"},
	}

	updateURIsTriggerTX, _ := createTransactionMock(&updateURIsTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateURIsTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, updateURIsTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	updateURIsKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	updateURIsKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(updateURIsKDAData, updateURIsKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, "https://github.com/klever-io/klever-go", updateURIsKDAData.URIs["Github"])

	// UPDATE MULTIPLE METADATA ############################################

	updateMultipleMetadataTrigger := []transaction.AssetTriggerContract{
		{
			TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
			AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
			ToAddress:   testOwnerAddress,
			MIME:        []byte("application/octet-stream"),
		},
		{
			TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
			AssetID:     []byte(string(nftID) + kapps.Sp + "2"),
			ToAddress:   testOwnerAddress,
			MIME:        []byte("application/octet-stream"),
		},
	}

	serialized, serializeErr := marshalizer.Marshal(&updateMultipleMetadataTrigger[1])
	assert.NoError(t, serializeErr)

	updateMultipleMetadataTriggerTX, _ := createTransactionMock(&updateMultipleMetadataTrigger[0], transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)
	updateMultipleMetadataTriggerTX.RawData.Contract = append(updateMultipleMetadataTriggerTX.RawData.Contract, &transaction.TXContract{
		Type: transaction.TXContract_AssetTriggerContractType,
		Parameter: &anypb.Any{
			TypeUrl: "github.com/klever-io/klever-go/" + string(proto.MessageName(&updateMultipleMetadataTrigger[1])),
			Value:   serialized,
		},
	})

	_, hash, err = execTx.PreProcessTransaction(updateMultipleMetadataTriggerTX)
	assert.Nil(t, err)

	metadataFirst := []byte("first metadata value")
	metadataSecond := []byte("second metadata value")

	updateMultipleMetadataTriggerTX.RawData.Data = [][]byte{metadataFirst, metadataSecond}

	err = execTx.ProcessTransaction(block, hash, updateMultipleMetadataTriggerTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userFirstMetadataNFTBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userFirstMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userFirstMetadaNFT, userFirstMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMultipleMetadataTrigger[0].MIME, userFirstMetadaNFT.MIME)
	assert.Equal(t, metadataFirst, userFirstMetadaNFT.Metadata)

	userSecondMetadataNFTBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	userSecondMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userSecondMetadaNFT, userSecondMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMultipleMetadataTrigger[1].MIME, userSecondMetadaNFT.MIME)
	assert.Equal(t, metadataSecond, userSecondMetadaNFT.Metadata)

	//UPDATE METADATA ######################################################

	updateMetadataTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
		ToAddress:   testOwnerAddress,
		MIME:        []byte("application/octet-stream"),
	}

	updateMetadataTriggerTX, _ := createTransactionMock(&updateMetadataTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateMetadataTriggerTX)
	assert.Nil(t, err)

	metadataValue := []byte("data")

	updateMetadataTriggerTX.RawData.Data = [][]byte{metadataValue}

	err = execTx.ProcessTransaction(block, hash, updateMetadataTriggerTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userMetadataNFTBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userMetadaNFT, userMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMetadataTrigger.MIME, userMetadaNFT.MIME)
	assert.Equal(t, metadataValue, userMetadaNFT.Metadata)

	//BURN NFT ######################################################

	nftTrigger2 := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     nftID,
		ToAddress:   testOwnerAddress,
		Amount:      1,
	}

	nftTriggerTX2, _ := createTransactionMock(&nftTrigger2, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX2)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX2)
	assert.Nil(t, err)

	nftBurnTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Burn,
		AssetID:     []byte(string(nftID) + kapps.Sp + "2"),
		Amount:      1,
	}

	nftBurnTX, _ := createTransactionMock(&nftBurnTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftBurnTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftBurnTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	userNFT13Bytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	nftBurnKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftBurnKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftBurnKDAData, nftBurnKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(userNFT13Bytes))
	assert.Equal(t, int64(3), nftBurnKDAData.MintedValue)
	assert.Equal(t, int64(1), nftBurnKDAData.BurnedValue)
	assert.Equal(t, int64(2), nftBurnKDAData.CirculatingSupply)

	//BURN FUNGIBLE ######################################################

	fungibleBurnTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Burn,
		AssetID:     fungibleID,
		Amount:      1234,
	}

	fungibleBurnTX, _ := createTransactionMock(&fungibleBurnTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleBurnTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleBurnTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	userBurnFungibleBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	userBurnFungible := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userBurnFungible, userBurnFungibleBytes)
	assert.Nil(t, err)

	burnFungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	burnFungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(burnFungibleKDAData, burnFungibleKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(1008765), userBurnFungible.Balance)
	assert.Equal(t, int64(1009999), burnFungibleKDAData.MintedValue)
	assert.Equal(t, int64(1234), burnFungibleKDAData.BurnedValue)
	assert.Equal(t, int64(1008765), burnFungibleKDAData.CirculatingSupply)

	//CHANGE OWNER ######################################################

	changeOwnerTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeOwner,
		AssetID:     nftID,
		ToAddress:   []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
	}

	changeOwnerTriggerTX, _ := createTransactionMock(&changeOwnerTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(changeOwnerTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, changeOwnerTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	changeOwnerKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	changeOwnerKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(changeOwnerKDAData, changeOwnerKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, changeOwnerTrigger.ToAddress, changeOwnerKDAData.OwnerAddress)

	//WIPE ######################################################

	newOwner := changeOwnerTrigger.ToAddress

	_ = loadUserAccount(accCacher, newOwner)
	_ = accCacher.SaveAll()

	wipeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Wipe,
		AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
		ToAddress:   testOwnerAddress,
		MIME:        []byte("application/octet-stream"),
		Amount:      1,
	}

	wipeTriggerTX, _ := createTransactionMock(&wipeTrigger, transaction.TXContract_AssetTriggerContractType, newOwner, 0)

	_, hash, err = execTx.PreProcessTransaction(wipeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, wipeTriggerTX)
	assert.Nil(t, err)

	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)

	userWipeNFTBytes, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(userWipeNFTBytes))

	//STOP NFT MINT ######################################################
	StopNFTMintTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMint,
		AssetID:     nftID,
	}

	StopNFTMintTriggerTX, _ := createTransactionMock(&StopNFTMintTrigger, transaction.TXContract_AssetTriggerContractType, newOwner, 0)

	_, hash, err = execTx.PreProcessTransaction(StopNFTMintTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, StopNFTMintTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	stopNFTMintKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	stopNFTMintKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(stopNFTMintKDAData, stopNFTMintKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, stopNFTMintKDAData.Attributes.IsNFTMintStopped)

	//STOP NFT METADATA CHANGE ######################################################
	StopNFTMetadataChangeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMetadataChange,
		AssetID:     nftID,
	}

	stopNFTMetadataChangeTriggerTX, _ := createTransactionMock(&StopNFTMetadataChangeTrigger, transaction.TXContract_AssetTriggerContractType, newOwner, 0)

	_, hash, err = execTx.PreProcessTransaction(stopNFTMetadataChangeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, stopNFTMetadataChangeTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	stopNFTMetadataChangeTriggerTXKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	stopNFTMetadataChangeKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(stopNFTMetadataChangeKDAData, stopNFTMetadataChangeTriggerTXKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, stopNFTMetadataChangeKDAData.Attributes.IsNFTMetadataChangeStopped)
}

func TestTxProcessor_ProcessAssetTriggerOkValsWithAdminShouldWork(t *testing.T) {
	t.Parallel()

	fungibleContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("KDA"),
		OwnerAddress:  testOwnerAddress,
		AdminAddress:  testAdminAddress,
		Precision:     6,
		InitialSupply: 1000000,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	nftContract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("NFT"),
		Ticker:       []byte("NFT"),
		OwnerAddress: testOwnerAddress,
		AdminAddress: testAdminAddress,
		MaxSupply:    10000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	fungibleTX, _ := createTransactionMock(&fungibleContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)
	nftTX, _ := createTransactionMock(&nftContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	execTx, args := NexTXProcessorV2(t)
	accCacher := args.AccountsCacher
	AddBalanceAccount(accCacher, 1000, kdautils.KLVIdentifier, testOwnerAddress)
	AddBalanceAccount(accCacher, 1000, kdautils.KLVIdentifier, testAdminAddress)

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	loadUserAccount(accCacher, testAdminAddress)
	block := createBlockHeader()

	//KDA creation ###################################################

	_, hash, err := execTx.PreProcessTransaction(fungibleTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTX)
	assert.Nil(t, err)

	fungibleID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), fungibleContract.GetTicker())

	_, hash, err = execTx.PreProcessTransaction(nftTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTX)
	assert.Nil(t, err)

	nftID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), nftContract.GetTicker())

	//MINT FUNGIBLE ######################################################

	fungibleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     fungibleID,
		ToAddress:   testAdminAddress,
		Amount:      9999,
	}

	fungibleTriggerTX, _ := createTransactionMock(&fungibleTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTriggerTX)
	assert.Nil(t, err)

	adminAcc := loadUserAccount(accCacher, testAdminAddress)
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	fungibleKey := kdautils.ToKDAKey(fungibleID, nil)

	userFungibleBytes, err := adminAcc.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	userFungible := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userFungible, userFungibleBytes)
	assert.Nil(t, err)

	fungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleKDAData, fungibleKDADataBytes)
	assert.Nil(t, err)

	//MINT NFT ######################################################

	nftTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     nftID,
		ToAddress:   testAdminAddress,
		Amount:      2,
	}

	nftTriggerTX, _ := createTransactionMock(&nftTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)

	nftKey := kdautils.ToKDAKey(nftID, nil)
	userNFTKey := kdautils.ToKDAKey(nftID, []byte("1"))

	userNFTKey2 := kdautils.ToKDAKey(nftID, []byte("2"))

	userNFTBytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFT, userNFTBytes)
	assert.Nil(t, err)

	nftKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftKDAData, nftKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(9999), adminAcc.GetBalance(fungibleID, true))
	assert.Equal(t, int64(1009999), fungibleKDAData.MintedValue)
	assert.Equal(t, int64(1009999), fungibleKDAData.CirculatingSupply)
	assert.Greater(t, len(userNFTBytes), 0)
	assert.Equal(t, []uint8([]byte(nil)), userNFT.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFT.Metadata)
	assert.Equal(t, int64(2), nftKDAData.MintedValue)
	assert.Equal(t, int64(2), nftKDAData.CirculatingSupply)

	//PAUSE ######################################################

	pauseTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Pause,
		AssetID:     fungibleID,
	}

	pauseTriggerTX, _ := createTransactionMock(&pauseTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(pauseTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, pauseTriggerTX)
	assert.Nil(t, err)

	pausedKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	pausedKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(pausedKDAData, pausedKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, pausedKDAData.Attributes.IsPaused)

	//RESUME ######################################################

	resumeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Resume,
		AssetID:     fungibleID,
	}

	resumeTriggerTX, _ := createTransactionMock(&resumeTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(resumeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, resumeTriggerTX)
	assert.Nil(t, err)

	resumedKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	resumedKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(resumedKDAData, resumedKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, false, resumedKDAData.Attributes.IsPaused)

	//ADD ROLE ######################################################

	addRoleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_AddRole,
		AssetID:     fungibleID,
		Role: &transaction.RolesInfo{
			Address:     []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
			HasRoleMint: true,
		},
	}

	addRoleTriggerTX, _ := createTransactionMock(&addRoleTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(addRoleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, addRoleTriggerTX)
	assert.Nil(t, err)

	addRoleKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	addRoleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(addRoleKDAData, addRoleKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, addRoleTrigger.Role.Address, addRoleKDAData.Roles[0].Address)
	assert.Equal(t, true, addRoleKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, false, addRoleKDAData.Roles[0].HasRoleSetITOPrices)

	//REMOVE ROLE ######################################################

	removeRoleTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_RemoveRole,
		AssetID:     fungibleID,
		ToAddress:   []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
	}

	removeRoleTriggerTX, _ := createTransactionMock(&removeRoleTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(removeRoleTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, removeRoleTriggerTX)
	assert.Nil(t, err)

	removeRoleKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	removeRoleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(removeRoleKDAData, removeRoleKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(removeRoleKDAData.Roles))

	//UPDATE Logo ######################################################

	updateLogoTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateLogo,
		AssetID:     fungibleID,
		Logo:        "https://github.com/klever-io/klever-go",
	}

	updateLogoTriggerTX, _ := createTransactionMock(&updateLogoTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateLogoTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, updateLogoTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	updateLogoKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	updateLogoKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(updateLogoKDAData, updateLogoKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, "https://github.com/klever-io/klever-go", updateLogoKDAData.Logo)

	//UPDATE URIS ######################################################

	updateURIsTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateURIs,
		AssetID:     fungibleID,
		URIs:        map[string]string{"Github": "https://github.com/klever-io/klever-go"},
	}

	updateURIsTriggerTX, _ := createTransactionMock(&updateURIsTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateURIsTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, updateURIsTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	updateURIsKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	updateURIsKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(updateURIsKDAData, updateURIsKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, "https://github.com/klever-io/klever-go", updateURIsKDAData.URIs["Github"])

	// UPDATE MULTIPLE METADATA ############################################

	updateMultipleMetadataTrigger := []transaction.AssetTriggerContract{
		{
			TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
			AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
			ToAddress:   testAdminAddress,
			MIME:        []byte("application/octet-stream"),
		},
		{
			TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
			AssetID:     []byte(string(nftID) + kapps.Sp + "2"),
			ToAddress:   testAdminAddress,
			MIME:        []byte("application/octet-stream"),
		},
	}

	serialized, serializeErr := marshalizer.Marshal(&updateMultipleMetadataTrigger[1])
	assert.NoError(t, serializeErr)

	updateMultipleMetadataTriggerTX, _ := createTransactionMock(&updateMultipleMetadataTrigger[0], transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)
	updateMultipleMetadataTriggerTX.RawData.Contract = append(updateMultipleMetadataTriggerTX.RawData.Contract, &transaction.TXContract{
		Type: transaction.TXContract_AssetTriggerContractType,
		Parameter: &anypb.Any{
			TypeUrl: "github.com/klever-io/klever-go/" + string(proto.MessageName(&updateMultipleMetadataTrigger[1])),
			Value:   serialized,
		},
	})

	_, hash, err = execTx.PreProcessTransaction(updateMultipleMetadataTriggerTX)
	assert.Nil(t, err)

	metadataFirst := []byte("first metadata value")
	metadataSecond := []byte("second metadata value")

	updateMultipleMetadataTriggerTX.RawData.Data = [][]byte{metadataFirst, metadataSecond}

	err = execTx.ProcessTransaction(block, hash, updateMultipleMetadataTriggerTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)

	userFirstMetadataNFTBytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userFirstMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userFirstMetadaNFT, userFirstMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMultipleMetadataTrigger[0].MIME, userFirstMetadaNFT.MIME)
	assert.Equal(t, metadataFirst, userFirstMetadaNFT.Metadata)

	userSecondMetadataNFTBytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	userSecondMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userSecondMetadaNFT, userSecondMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMultipleMetadataTrigger[1].MIME, userSecondMetadaNFT.MIME)
	assert.Equal(t, metadataSecond, userSecondMetadaNFT.Metadata)

	//UPDATE METADATA ######################################################

	updateMetadataTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateMetadata,
		AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
		ToAddress:   testAdminAddress,
		MIME:        []byte("application/octet-stream"),
	}

	updateMetadataTriggerTX, _ := createTransactionMock(&updateMetadataTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(updateMetadataTriggerTX)
	assert.Nil(t, err)

	metadataValue := []byte("data")

	updateMetadataTriggerTX.RawData.Data = [][]byte{metadataValue}

	err = execTx.ProcessTransaction(block, hash, updateMetadataTriggerTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)

	userMetadataNFTBytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	userMetadaNFT := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userMetadaNFT, userMetadataNFTBytes)
	assert.Nil(t, err)

	assert.Equal(t, updateMetadataTrigger.MIME, userMetadaNFT.MIME)
	assert.Equal(t, metadataValue, userMetadaNFT.Metadata)

	//BURN NFT ######################################################

	nftTrigger2 := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     nftID,
		ToAddress:   testAdminAddress,
		Amount:      1,
	}

	nftTriggerTX2, _ := createTransactionMock(&nftTrigger2, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX2)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX2)
	assert.Nil(t, err)

	nftBurnTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Burn,
		AssetID:     []byte(string(nftID) + kapps.Sp + "2"),
		Amount:      1,
	}

	nftBurnTX, _ := createTransactionMock(&nftBurnTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftBurnTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftBurnTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	userNFT13Bytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	nftBurnKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftBurnKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftBurnKDAData, nftBurnKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(userNFT13Bytes))
	assert.Equal(t, int64(3), nftBurnKDAData.MintedValue)
	assert.Equal(t, int64(1), nftBurnKDAData.BurnedValue)
	assert.Equal(t, int64(2), nftBurnKDAData.CirculatingSupply)

	//BURN FUNGIBLE ######################################################

	fungibleBurnTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Burn,
		AssetID:     fungibleID,
		Amount:      1234,
	}

	fungibleBurnTX, _ := createTransactionMock(&fungibleBurnTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleBurnTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleBurnTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	userBurnFungibleBytes, err := adminAcc.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	userBurnFungible := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userBurnFungible, userBurnFungibleBytes)
	assert.Nil(t, err)

	burnFungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	burnFungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(burnFungibleKDAData, burnFungibleKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(8765), userBurnFungible.Balance)
	assert.Equal(t, int64(1009999), burnFungibleKDAData.MintedValue)
	assert.Equal(t, int64(1234), burnFungibleKDAData.BurnedValue)
	assert.Equal(t, int64(1008765), burnFungibleKDAData.CirculatingSupply)

	//CHANGE OWNER ######################################################
	//Should err, only owner can change owner
	changeOwnerTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeOwner,
		AssetID:     nftID,
		ToAddress:   []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
	}

	changeOwnerTriggerTX, _ := createTransactionMock(&changeOwnerTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(changeOwnerTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, changeOwnerTriggerTX)
	assert.Equal(t, "account is not the owner", err.Error())

	//WIPE ######################################################

	_ = loadUserAccount(accCacher, testAdminAddress)
	_ = accCacher.SaveAll()

	wipeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Wipe,
		AssetID:     []byte(string(nftID) + kapps.Sp + "1"),
		ToAddress:   testAdminAddress,
		MIME:        []byte("application/octet-stream"),
		Amount:      1,
	}

	wipeTriggerTX, _ := createTransactionMock(&wipeTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(wipeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, wipeTriggerTX)
	assert.Nil(t, err)

	adminAcc = loadUserAccount(accCacher, testAdminAddress)

	userWipeNFTBytes, err := adminAcc.DataTrieTracker().RetrieveValue(userNFTKey)
	assert.Nil(t, err)

	assert.Equal(t, 0, len(userWipeNFTBytes))

	//STOP NFT MINT ######################################################
	StopNFTMintTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMint,
		AssetID:     nftID,
	}

	StopNFTMintTriggerTX, _ := createTransactionMock(&StopNFTMintTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(StopNFTMintTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, StopNFTMintTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	stopNFTMintKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	stopNFTMintKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(stopNFTMintKDAData, stopNFTMintKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, stopNFTMintKDAData.Attributes.IsNFTMintStopped)

	//STOP NFT METADATA CHANGE ######################################################
	StopNFTMetadataChangeTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_StopNFTMetadataChange,
		AssetID:     nftID,
	}

	stopNFTMetadataChangeTriggerTX, _ := createTransactionMock(&StopNFTMetadataChangeTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(stopNFTMetadataChangeTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, stopNFTMetadataChangeTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	stopNFTMetadataChangeTriggerTXKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	stopNFTMetadataChangeKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(stopNFTMetadataChangeKDAData, stopNFTMetadataChangeTriggerTXKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, true, stopNFTMetadataChangeKDAData.Attributes.IsNFTMetadataChangeStopped)

	//CHANGE Admin ######################################################

	changeAdminTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_ChangeAdmin,
		AssetID:     nftID,
		ToAddress:   []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg"),
	}

	changeAdminTriggerTX, _ := createTransactionMock(&changeAdminTrigger, transaction.TXContract_AssetTriggerContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(changeAdminTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, changeAdminTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)

	changeAdminKDABytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	changeAdminKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(changeAdminKDAData, changeAdminKDABytes)
	assert.Nil(t, err)

	assert.Equal(t, changeAdminTrigger.ToAddress, changeAdminKDAData.AdminAddress)
}

func TestTxProcessor_ProcessSetAccountNameOkValsShouldWork(t *testing.T) {
	t.Parallel()

	contract := transaction.SetAccountNameContract{
		Name: []byte("New Name"),
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SetAccountNameContractType, testOwnerAddress, 0)

	_, err := tx.RawData.Contract[0].GetSetAccountNameContract()
	assert.Nil(t, err)

	args := createArgsForTxProcessor()
	acntSrc := loadUserAccount(args.AccountsCacher, testOwnerAddress)
	acntSrc.AddToBalance(90, nil, true)
	_ = args.AccountsCacher.SaveAll()

	SetupKappController(t, &args)
	execTx, err := pTX.NewTxProcessor(args)
	assert.Nil(t, err)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)
	acntSrc = loadUserAccount(args.AccountsCacher, testOwnerAddress)
	assert.Equal(t, contract.Name, acntSrc.GetName())
}

func TestTxProcessor_ProcessProposalWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Proposal with invalid Parameter
	contract := transaction.ProposalContract{
		Parameters:     map[int32][]byte{9999: []byte("23")},
		EpochsDuration: 10,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_ProposalContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidParameter, err)

	//Proposal with invalid Value
	contract = transaction.ProposalContract{
		Parameters:     map[int32][]byte{int32(kapps.EnumParameter_FeePerDataByte): []byte("INVALID")},
		EpochsDuration: 10,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ProposalContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, "strconv.ParseInt: parsing \"INVALID\": invalid syntax", err.Error())

	//Proposal with epochs duration bigger than allowed
	contract = transaction.ProposalContract{
		Parameters:     map[int32][]byte{int32(kapps.EnumParameter_FeePerDataByte): []byte("100")},
		EpochsDuration: 1000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ProposalContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestTxProcessor_ProcessProposalOkValsShouldWork(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initProposalKapp(proposalKapp)

	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)

	userKDA := kapps.UserKDA{
		Balance:       12345,
		LastClaim:     &kapps.LastClaim{},
		FrozenBalance: 12345,
		Buckets:       make(map[string]*kapps.UserBucket),
	}

	userKDA.Buckets["MY_BUCKET"] = &kapps.UserBucket{
		StakedAt:      time.Now().Unix(),
		StakedEpoch:   0,
		UnstakedEpoch: core.DefaultUnstakedEpoch,
		Value:         12345,
		Delegation:    nil,
	}

	kfiBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(kfiKey, kfiBucket)
	assert.Nil(t, err)

	_ = ownerAcc.AddToBalance(1000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	proposalContract := transaction.ProposalContract{
		Parameters:     map[int32][]byte{0: []byte("19000")},
		Description:    []byte("Proposal to change feePerDataByte of transactions"),
		EpochsDuration: 5,
	}

	proposalTX, _ := createTransactionMock(&proposalContract, transaction.TXContract_ProposalContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(proposalTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, proposalTX)
	assert.Nil(t, err)

	proposalKapp = loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	controllerKey := kdautils.ToProposalKey(0)
	proposalKey := kdautils.ToProposalKey(1)

	controllerBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(controllerKey)
	assert.Nil(t, err)

	proposalBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(proposalKey)
	assert.Nil(t, err)

	controllerData := &kapps.ProposalController{}
	err = marshalizer.Unmarshal(controllerData, controllerBytes)
	assert.Nil(t, err)

	proposalData := &kapps.ProposalData{}
	err = marshalizer.Unmarshal(proposalData, proposalBytes)
	assert.Nil(t, err)

	assert.Equal(t, uint64(1), controllerData.ProposalCount)
	assert.Equal(t, kapps.EnumType_Int64, controllerData.ActiveParameters[0].Type)
	assert.Equal(t, []byte("4000"), controllerData.ActiveParameters[0].Value)
	assert.Equal(t, uint64(1), controllerData.ActiveProposals[5].ProposalIDs[0])

	assert.Equal(t, kapps.ProposalData_ActiveProposal, proposalData.ProposalStatus)
	assert.Equal(t, proposalContract.Parameters[0], proposalData.Parameters[0])
	assert.Equal(t, proposalContract.Description, proposalData.Description)
	assert.Equal(t, uint32(0), proposalData.EpochStart)
	assert.Equal(t, proposalContract.EpochsDuration, proposalData.EpochEnd)
	assert.Equal(t, int64(0), proposalData.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	assert.Equal(t, int64(0), proposalData.Votes[int32(kapps.ProposalData_VoteDetail_No)])
	assert.Equal(t, map[string]*kapps.ProposalData_VoteDetail(nil), proposalData.Voters)
}

func TestTxProcessor_ProcessVoteWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initProposalKapp(proposalKapp)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Vote with invalid Amount
	contract := transaction.VoteContract{
		Type:       transaction.VoteContract_EnumVoteType(kapps.ProposalData_VoteDetail_Yes),
		ProposalID: 1,
		Amount:     -1,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_VoteContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Vote with invalid proposalID
	contract = transaction.VoteContract{
		ProposalID: 99999,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_VoteContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrProposalNotFound, err)
}

func TestTxProcessor_ProcessVoteOkValsShouldWork(t *testing.T) {
	t.Parallel()

	testOwnerAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksf")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initProposalKapp(proposalKapp)

	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)

	userKDA := kapps.UserKDA{
		Balance:       12345,
		LastClaim:     &kapps.LastClaim{},
		FrozenBalance: 12345,
		Buckets:       make(map[string]*kapps.UserBucket),
	}

	userKDA.Buckets["MY_BUCKET"] = &kapps.UserBucket{
		StakedAt:      time.Now().Unix(),
		StakedEpoch:   0,
		UnstakedEpoch: core.DefaultUnstakedEpoch,
		Value:         12345,
		Delegation:    nil,
	}

	kfiBucket, err := marshalizer.Marshal(&userKDA)
	assert.Nil(t, err)

	err = ownerAcc.DataTrieTracker().SaveKeyValue(kfiKey, kfiBucket)
	assert.Nil(t, err)

	_ = ownerAcc.AddToBalance(1000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	proposalContract := transaction.ProposalContract{
		Parameters:     map[int32][]byte{0: []byte("19000")},
		Description:    []byte("Proposal to change feePerDataByte of transactions"),
		EpochsDuration: 5,
	}

	proposalTX, _ := createTransactionMock(&proposalContract, transaction.TXContract_ProposalContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(proposalTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, proposalTX)
	assert.Nil(t, err)

	voteContract := transaction.VoteContract{
		Type:       transaction.VoteContract_EnumVoteType(kapps.ProposalData_VoteDetail_Yes),
		ProposalID: 1,
		Amount:     1234,
	}

	voteTX, _ := createTransactionMock(&voteContract, transaction.TXContract_VoteContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(voteTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, voteTX)
	assert.Nil(t, err)

	proposalKapp = loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	controllerKey := kdautils.ToProposalKey(0)
	proposalKey := kdautils.ToProposalKey(1)

	controllerBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(controllerKey)
	assert.Nil(t, err)

	proposalBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(proposalKey)
	assert.Nil(t, err)

	controllerData := &kapps.ProposalController{}
	err = marshalizer.Unmarshal(controllerData, controllerBytes)
	assert.Nil(t, err)

	proposalData := &kapps.ProposalData{}
	err = marshalizer.Unmarshal(proposalData, proposalBytes)
	assert.Nil(t, err)

	assert.Equal(t, uint64(1), controllerData.ProposalCount)
	assert.Equal(t, kapps.EnumType_Int64, controllerData.ActiveParameters[0].Type)
	assert.Equal(t, []byte("4000"), controllerData.ActiveParameters[0].Value)
	require.Len(t, controllerData.ActiveProposals, 1)
	assert.Equal(t, uint64(1), controllerData.ActiveProposals[5].ProposalIDs[0])

	assert.Equal(t, kapps.ProposalData_ActiveProposal, proposalData.ProposalStatus)
	assert.Equal(t, proposalContract.Parameters[0], proposalData.Parameters[0])
	assert.Equal(t, proposalContract.Description, proposalData.Description)
	assert.Equal(t, uint32(0), proposalData.EpochStart)
	assert.Equal(t, proposalContract.EpochsDuration, proposalData.EpochEnd)
	assert.Equal(t, int64(1234), proposalData.Votes[int32(kapps.ProposalData_VoteDetail_Yes)])
	assert.Equal(t, int64(1234), proposalData.Voters[hex.EncodeToString(testOwnerAddress)].Amount)
}

func TestTxProcessor_ProcessConfigSetAndBuyITOWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initProposalKapp(proposalKapp)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Asset Creation to trigger
	kdaContract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("NFT"),
		Ticker:       []byte("NFT"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10_000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
		Staking: &transaction.StakingInfo{MinEpochsToUnstake: 0, MinEpochsToClaim: 0, APR: 16},
	}

	tx, _ := createTransactionMock(&kdaContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	assetID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), kdaContract.GetOwnerAddress(), ownerAcc.GetNonce(), kdaContract.GetTicker())

	//ConfigITO with invalid asset
	contract := transaction.ConfigITOContract{
		AssetID:         []byte("INVALID"),
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//ConfigITO with invalid receiver address
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: []byte("INVALID"),
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	//ConfigITO with invalid status
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          99,
		MaxAmount:       10000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrITOTypeInvalid, err)

	//ConfigITO with invalid maxAmount
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          99,
		MaxAmount:       -10000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigITO with invalid pack amount
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
		PackInfo:        map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: -1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigITO with invalid pack Price
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
		PackInfo:        map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: -1}}}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigITO with inexistent asset on a pack
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
		PackInfo:        map[string]*transaction.PackInfo{"INEXISTENT": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//ConfigITO with empty pack
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       10000,
		PackInfo:        map[string]*transaction.PackInfo{"KLV": {}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Config ITO invalid limit per address - more than max amount
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 1000000000000000000,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Config ITO invalid limit per address - negative
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: -10,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Config ITO invalid whitelist status
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistStatus:        5,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrWhitelistStatusInvalid, err)

	// Config ITO invalid whitelist address
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistInfo:          map[string]*transaction.WhitelistInfo{"INVALID": {Limit: 1}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, process.ErrInvalidWhitelistAddr, err)

	// Config ITO invalid whitelist address limit
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistInfo:          map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: -1}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Config ITO invalid ito whitelist start and end time
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistStatus:        transaction.ConfigITOContract_DefaultITO,
		WhitelistInfo:          map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: 1}},
		WhitelistStartTime:     time.Now().AddDate(0, 0, 2).Unix(),
		WhitelistEndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// Config ITO invalid ito start and end time
	contract = transaction.ConfigITOContract{
		AssetID:                assetID,
		ReceiverAddress:        testToAddress,
		Status:                 transaction.ConfigITOContract_ActiveITO,
		MaxAmount:              10000,
		PackInfo:               map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
		DefaultLimitPerAddress: 10,
		WhitelistStatus:        transaction.ConfigITOContract_DefaultITO,
		WhitelistInfo:          map[string]*transaction.WhitelistInfo{hex.EncodeToString(testWhitelistAddress): {Limit: 1}},
		WhitelistStartTime:     time.Now().AddDate(0, 0, 1).Unix(),
		WhitelistEndTime:       time.Now().AddDate(0, 0, 2).Unix(),
		StartTime:              time.Now().AddDate(0, 0, 2).Unix(),
		EndTime:                time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigITO correctly to test other ito transactions
	contract = transaction.ConfigITOContract{
		AssetID:         assetID,
		ReceiverAddress: testToAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       0,
		PackInfo:        map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, nil, err)

	//Pre fork tx executor for deprecated SetITOPrices
	argsTemp := createArgsForTxProcessor()

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                1000,
	}, epochNotifier)

	argsKappTemp := kappcontroller.ArgsNewKApp{
		Hasher:         argsTemp.Hasher,
		Marshalizer:    argsTemp.Marshalizer,
		PubkeyConv:     argsTemp.PubkeyConv,
		AccountsCacher: accCacher,
		ForkController: forkController,
		RatingsData:    argsTemp.RatingsData,
	}

	kAppController, err := kappcontroller.NewKappController(argsKappTemp)
	assert.Nil(t, err)

	controller := createProposalController()
	_ = kAppController.SetProposalController(controller)
	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)

	argsTemp.AccountsCacher = accCacher
	argsTemp.KAppController = kAppController
	argsTemp.ForkController = forkController

	execTxTemp := NewTXProcessor(t, argsTemp)

	//SetITOPrices with invalid asset
	setContract := transaction.SetITOPricesContract{
		AssetID:  []byte("INVALID"),
		PackInfo: map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//SetITOPrices with invalid pack amount
	setContract = transaction.SetITOPricesContract{
		AssetID:  assetID,
		PackInfo: map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: -1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//SetITOPrices with invalid pack price
	setContract = transaction.SetITOPricesContract{
		AssetID:  assetID,
		PackInfo: map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: -1}}}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//SetITOPrices with empty pack
	setContract = transaction.SetITOPricesContract{
		AssetID:  assetID,
		PackInfo: map[string]*transaction.PackInfo{"KLV": {}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//SetITOPrices with inexistent pack asset
	setContract = transaction.SetITOPricesContract{
		AssetID:  assetID,
		PackInfo: map[string]*transaction.PackInfo{"INEXISTENT": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//SetITOPrices without have permission
	setContract = transaction.SetITOPricesContract{
		AssetID:  assetID,
		PackInfo: map[string]*transaction.PackInfo{"KLV": {Packs: []*transaction.PackItem{{Amount: 1, Price: 1}}}},
	}

	tx, _ = createTransactionMock(&setContract, transaction.TXContract_SetITOPricesContractType, testToAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrRoleNotFound, err)

	//BuyITO with wrong type
	buyContract := transaction.BuyContract{
		BuyType:    999,
		ID:         assetID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrBuyTypeInvalid, err)

	//BuyITO with invalid asset
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         []byte("INVALID"),
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//BuyITO with invalid currency
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         assetID,
		CurrencyID: []byte("INVALID"),
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//BuyITO with invalid amount
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         assetID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     -1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestTxProcessor_ProcessConfigSetAndBuyITOOkValsShouldWork(t *testing.T) {
	t.Parallel()

	receiverAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg")

	fungibleContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("KDA"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: 1_000_000,
		MaxSupply:     1_000_000_000_000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	nftContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_NonFungible,
		Name:          []byte("NFT"),
		Ticker:        []byte("NFT"),
		OwnerAddress:  testOwnerAddress,
		Precision:     0,
		InitialSupply: 0,
		MaxSupply:     10000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	fungibleTX, _ := createTransactionMock(&fungibleContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)
	nftTX, _ := createTransactionMock(&nftContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc := loadUserAccount(accCacher, receiverAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	itoKapp := loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = ownerAcc.AddToBalance(1_000_000_000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(receiverAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(itoKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	//KDA creation ###################################################

	_, hash, err := execTx.PreProcessTransaction(fungibleTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTX)
	assert.Nil(t, err)

	fungibleID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), fungibleContract.GetTicker())

	_, hash, err = execTx.PreProcessTransaction(nftTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTX)
	assert.Nil(t, err)

	nftID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), nftContract.GetTicker())

	nftITOKey := kdautils.ToITOKey(nftID)
	fungibleITOKey := kdautils.ToITOKey(fungibleID)
	nftKey := kdautils.ToKDAKey(nftID, nil)
	fungibleKey := kdautils.ToKDAKey(fungibleID, nil)

	//Create NFT ITO ######################################################

	packInfo := make(map[string]*transaction.PackInfo)

	packInfo["KLV"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 1, Price: 50}, {Amount: 5, Price: 40}},
	}

	configITO := transaction.ConfigITOContract{
		AssetID:         nftID,
		ReceiverAddress: receiverAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       1_000_000_000,
		PackInfo:        packInfo,
	}

	nftConfigITOTX, _ := createTransactionMock(&configITO, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftConfigITOTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftConfigITOTX)
	assert.Nil(t, err)

	configITO.AssetID = fungibleID
	packInfo["KLV"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 500_000_000, Price: 500_000}, {Amount: 1_000_000_000, Price: 400_000}},
	}

	fungibleConfigITOTX, _ := createTransactionMock(&configITO, transaction.TXContract_ConfigITOContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleConfigITOTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleConfigITOTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	nftITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(nftITOKey)
	assert.Nil(t, err)

	nftITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(nftITOData, nftITODataBytes)
	assert.Nil(t, err)

	fungibleITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleITOData, fungibleITODataBytes)
	assert.Nil(t, err)

	nftKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftKDAData, nftKDADataBytes)
	assert.Nil(t, err)

	fungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleKDAData, fungibleKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, true, nftITOData.IsActive)
	assert.Equal(t, int64(1_000_000_000), nftITOData.MaxAmount)
	assert.Equal(t, int64(0), nftITOData.MintedAmount)
	assert.Equal(t, receiverAddress, nftITOData.ReceiverAddress)
	assert.Equal(t, int64(1), nftITOData.PackData["KLV"].Packs[0].Amount)
	assert.Equal(t, int64(50), nftITOData.PackData["KLV"].Packs[0].Price)
	assert.Equal(t, int64(5), nftITOData.PackData["KLV"].Packs[1].Amount)
	assert.Equal(t, int64(40), nftITOData.PackData["KLV"].Packs[1].Price)

	assert.Equal(t, true, fungibleITOData.IsActive)
	assert.Equal(t, int64(1_000_000_000), fungibleITOData.MaxAmount)
	assert.Equal(t, int64(0), fungibleITOData.MintedAmount)
	assert.Equal(t, receiverAddress, fungibleITOData.ReceiverAddress)
	assert.Equal(t, int64(500_000_000), fungibleITOData.PackData["KLV"].Packs[0].Amount)
	assert.Equal(t, int64(500_000), fungibleITOData.PackData["KLV"].Packs[0].Price)
	assert.Equal(t, int64(1_000_000_000), fungibleITOData.PackData["KLV"].Packs[1].Amount)
	assert.Equal(t, int64(400_000), fungibleITOData.PackData["KLV"].Packs[1].Price)

	assert.Equal(t, kapps.ITOKAppAddress, nftKDAData.Roles[0].Address)
	assert.Equal(t, true, nftKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, true, nftKDAData.Roles[0].HasRoleSetITOPrices)

	assert.Equal(t, kapps.ITOKAppAddress, fungibleKDAData.Roles[0].Address)
	assert.Equal(t, true, fungibleKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, true, fungibleKDAData.Roles[0].HasRoleSetITOPrices)

	//SetPrices Fungible ITO ######################################################

	//Pre fork tx executor for deprecated SetITOPrices
	argsTemp := createArgsForTxProcessor()

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                1000,
	}, epochNotifier)

	argsKappTemp := kappcontroller.ArgsNewKApp{
		Hasher:         argsTemp.Hasher,
		Marshalizer:    argsTemp.Marshalizer,
		PubkeyConv:     argsTemp.PubkeyConv,
		AccountsCacher: accCacher,
		ForkController: forkController,
		RatingsData:    argsTemp.RatingsData,
	}

	kAppController, err := kappcontroller.NewKappController(argsKappTemp)
	assert.Nil(t, err)

	controller := createProposalController()
	_ = kAppController.SetProposalController(controller)

	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)

	argsTemp.AccountsCacher = accCacher
	argsTemp.KAppController = kAppController
	argsTemp.ForkController = forkController

	execTxTemp := NewTXProcessor(t, argsTemp)

	packInfo["KFI"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 500, Price: 50}, {Amount: 1000, Price: 40}},
	}

	setITOPrices := transaction.SetITOPricesContract{
		AssetID:  fungibleID,
		PackInfo: packInfo,
	}

	fungibleSetITOPricesTX, _ := createTransactionMock(&setITOPrices, transaction.TXContract_SetITOPricesContractType, testOwnerAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(fungibleSetITOPricesTX)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(block, hash, fungibleSetITOPricesTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	fungibleSetITOPricesDataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleSetITOPricesData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleSetITOPricesData, fungibleSetITOPricesDataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(500), fungibleSetITOPricesData.PackData["KFI"].Packs[0].Amount)
	assert.Equal(t, int64(50), fungibleSetITOPricesData.PackData["KFI"].Packs[0].Price)
	assert.Equal(t, int64(1000), fungibleSetITOPricesData.PackData["KFI"].Packs[1].Amount)
	assert.Equal(t, int64(40), fungibleSetITOPricesData.PackData["KFI"].Packs[1].Price)

	//BUY NFT ITO ######################################################

	nftBuyITO := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         nftID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     5,
	}

	nftBuyTX, _ := createTransactionMock(&nftBuyITO, transaction.TXContract_BuyContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftBuyTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftBuyTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)

	nftBuyITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(nftITOKey)
	assert.Nil(t, err)

	nftBuyITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(nftBuyITOData, nftBuyITODataBytes)
	assert.Nil(t, err)

	nftBuyKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftBuyKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftBuyKDAData, nftBuyKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(5), nftBuyITOData.MintedAmount)
	assert.Equal(t, int64(5), nftBuyKDAData.MintedValue)
	assert.Equal(t, int64(999999800), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(200), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))

	//BUY FUNGIBLE ITO ######################################################

	fungibleBuyITO := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         fungibleID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     680_000_000,
	}

	fungibleBuyTX, _ := createTransactionMock(&fungibleBuyITO, transaction.TXContract_BuyContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleBuyTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleBuyTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)

	fungibleBuyITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleBuyITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleBuyITOData, fungibleBuyITODataBytes)
	assert.Nil(t, err)

	fungibleBuyKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleBuyKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleBuyKDAData, fungibleBuyKDADataBytes)
	assert.Nil(t, err)

	// 680 Un (Amount) * 0.4KLV (Price)
	// Cost = 680*0.4 = 272 KLV

	assert.Equal(t, int64(680_000_000), fungibleBuyITOData.MintedAmount)
	assert.Equal(t, int64(680_000_000+1_000_000), fungibleBuyKDAData.MintedValue)
	assert.Equal(t, int64(999_999_800-272_000_000), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(272_000_200), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTxProcessor_ProcessConfigSetAndBuyITOWithAdminOkValsShouldWork(t *testing.T) {
	t.Parallel()

	receiverAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg")

	fungibleContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("KDA"),
		OwnerAddress:  testOwnerAddress,
		AdminAddress:  testAdminAddress,
		Precision:     6,
		InitialSupply: 1_000_000,
		MaxSupply:     1_000_000_000_000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	nftContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_NonFungible,
		Name:          []byte("NFT"),
		Ticker:        []byte("NFT"),
		OwnerAddress:  testOwnerAddress,
		AdminAddress:  testAdminAddress,
		Precision:     0,
		InitialSupply: 0,
		MaxSupply:     10000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	fungibleTX, _ := createTransactionMock(&fungibleContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)
	nftTX, _ := createTransactionMock(&nftContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	adminAcc := loadUserAccount(accCacher, testAdminAddress)
	receiverAcc := loadUserAccount(accCacher, receiverAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	itoKapp := loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	_ = ownerAcc.AddToBalance(1_000_000_000, nil, true)
	_ = adminAcc.AddToBalance(1_000_000_000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(adminAcc)
	_ = userDB.SaveAccount(receiverAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(itoKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	//KDA creation ###################################################

	_, hash, err := execTx.PreProcessTransaction(fungibleTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleTX)
	assert.Nil(t, err)

	fungibleID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), fungibleContract.GetTicker())

	_, hash, err = execTx.PreProcessTransaction(nftTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTX)
	assert.Nil(t, err)

	nftID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), nftContract.GetTicker())

	nftITOKey := kdautils.ToITOKey(nftID)
	fungibleITOKey := kdautils.ToITOKey(fungibleID)
	nftKey := kdautils.ToKDAKey(nftID, nil)
	fungibleKey := kdautils.ToKDAKey(fungibleID, nil)

	//Create NFT ITO ######################################################

	packInfo := make(map[string]*transaction.PackInfo)

	packInfo["KLV"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 1, Price: 50}, {Amount: 5, Price: 40}},
	}

	configITO := transaction.ConfigITOContract{
		AssetID:         nftID,
		ReceiverAddress: receiverAddress,
		Status:          transaction.ConfigITOContract_ActiveITO,
		MaxAmount:       1_000_000_000,
		PackInfo:        packInfo,
	}

	nftConfigITOTX, _ := createTransactionMock(&configITO, transaction.TXContract_ConfigITOContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftConfigITOTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftConfigITOTX)
	assert.Nil(t, err)

	configITO.AssetID = fungibleID
	packInfo["KLV"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 500_000_000, Price: 500_000}, {Amount: 1_000_000_000, Price: 400_000}},
	}

	fungibleConfigITOTX, _ := createTransactionMock(&configITO, transaction.TXContract_ConfigITOContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleConfigITOTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleConfigITOTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	nftITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(nftITOKey)
	assert.Nil(t, err)

	nftITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(nftITOData, nftITODataBytes)
	assert.Nil(t, err)

	fungibleITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleITOData, fungibleITODataBytes)
	assert.Nil(t, err)

	nftKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftKDAData, nftKDADataBytes)
	assert.Nil(t, err)

	fungibleKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleKDAData, fungibleKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, true, nftITOData.IsActive)
	assert.Equal(t, int64(1_000_000_000), nftITOData.MaxAmount)
	assert.Equal(t, int64(0), nftITOData.MintedAmount)
	assert.Equal(t, receiverAddress, nftITOData.ReceiverAddress)
	assert.Equal(t, int64(1), nftITOData.PackData["KLV"].Packs[0].Amount)
	assert.Equal(t, int64(50), nftITOData.PackData["KLV"].Packs[0].Price)
	assert.Equal(t, int64(5), nftITOData.PackData["KLV"].Packs[1].Amount)
	assert.Equal(t, int64(40), nftITOData.PackData["KLV"].Packs[1].Price)

	assert.Equal(t, true, fungibleITOData.IsActive)
	assert.Equal(t, int64(1_000_000_000), fungibleITOData.MaxAmount)
	assert.Equal(t, int64(0), fungibleITOData.MintedAmount)
	assert.Equal(t, receiverAddress, fungibleITOData.ReceiverAddress)
	assert.Equal(t, int64(500_000_000), fungibleITOData.PackData["KLV"].Packs[0].Amount)
	assert.Equal(t, int64(500_000), fungibleITOData.PackData["KLV"].Packs[0].Price)
	assert.Equal(t, int64(1_000_000_000), fungibleITOData.PackData["KLV"].Packs[1].Amount)
	assert.Equal(t, int64(400_000), fungibleITOData.PackData["KLV"].Packs[1].Price)

	assert.Equal(t, kapps.ITOKAppAddress, nftKDAData.Roles[0].Address)
	assert.Equal(t, true, nftKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, true, nftKDAData.Roles[0].HasRoleSetITOPrices)

	assert.Equal(t, kapps.ITOKAppAddress, fungibleKDAData.Roles[0].Address)
	assert.Equal(t, true, fungibleKDAData.Roles[0].HasRoleMint)
	assert.Equal(t, true, fungibleKDAData.Roles[0].HasRoleSetITOPrices)

	//SetPrices Fungible ITO ######################################################

	//Pre fork tx executor for deprecated SetITOPrices
	argsTemp := createArgsForTxProcessor()

	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                1000,
	}, epochNotifier)

	argsKappTemp := kappcontroller.ArgsNewKApp{
		Hasher:         argsTemp.Hasher,
		Marshalizer:    argsTemp.Marshalizer,
		PubkeyConv:     argsTemp.PubkeyConv,
		AccountsCacher: accCacher,
		ForkController: forkController,
		RatingsData:    argsTemp.RatingsData,
	}

	kAppController, err := kappcontroller.NewKappController(argsKappTemp)
	assert.Nil(t, err)

	controller := createProposalController()
	_ = kAppController.SetProposalController(controller)

	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)

	argsTemp.AccountsCacher = accCacher
	argsTemp.KAppController = kAppController
	argsTemp.ForkController = forkController

	execTxTemp := NewTXProcessor(t, argsTemp)

	packInfo["KFI"] = &transaction.PackInfo{
		Packs: []*transaction.PackItem{{Amount: 500, Price: 50}, {Amount: 1000, Price: 40}},
	}

	setITOPrices := transaction.SetITOPricesContract{
		AssetID:  fungibleID,
		PackInfo: packInfo,
	}

	fungibleSetITOPricesTX, _ := createTransactionMock(&setITOPrices, transaction.TXContract_SetITOPricesContractType, testAdminAddress, 0)

	_, hash, err = execTxTemp.PreProcessTransaction(fungibleSetITOPricesTX)
	assert.Nil(t, err)

	err = execTxTemp.ProcessTransaction(block, hash, fungibleSetITOPricesTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)

	fungibleSetITOPricesDataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleSetITOPricesData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleSetITOPricesData, fungibleSetITOPricesDataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(500), fungibleSetITOPricesData.PackData["KFI"].Packs[0].Amount)
	assert.Equal(t, int64(50), fungibleSetITOPricesData.PackData["KFI"].Packs[0].Price)
	assert.Equal(t, int64(1000), fungibleSetITOPricesData.PackData["KFI"].Packs[1].Amount)
	assert.Equal(t, int64(40), fungibleSetITOPricesData.PackData["KFI"].Packs[1].Price)

	//BUY NFT ITO ######################################################

	nftBuyITO := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         nftID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     5,
	}

	nftBuyTX, _ := createTransactionMock(&nftBuyITO, transaction.TXContract_BuyContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftBuyTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftBuyTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testAdminAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)

	nftBuyITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(nftITOKey)
	assert.Nil(t, err)

	nftBuyITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(nftBuyITOData, nftBuyITODataBytes)
	assert.Nil(t, err)

	nftBuyKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftBuyKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftBuyKDAData, nftBuyKDADataBytes)
	assert.Nil(t, err)

	assert.Equal(t, int64(5), nftBuyITOData.MintedAmount)
	assert.Equal(t, int64(5), nftBuyKDAData.MintedValue)
	assert.Equal(t, int64(999999800), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(200), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))

	//BUY FUNGIBLE ITO ######################################################

	fungibleBuyITO := transaction.BuyContract{
		BuyType:    transaction.BuyContract_ITOBuy,
		ID:         fungibleID,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     680_000_000,
	}

	fungibleBuyTX, _ := createTransactionMock(&fungibleBuyITO, transaction.TXContract_BuyContractType, testAdminAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(fungibleBuyTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, fungibleBuyTX)
	assert.Nil(t, err)

	itoKapp = loadKAppAccount(accCacher, kapps.ITOKAppAddress)
	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testAdminAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)

	fungibleBuyITODataBytes, err := itoKapp.DataTrieTracker().RetrieveValue(fungibleITOKey)
	assert.Nil(t, err)

	fungibleBuyITOData := &kapps.ITOData{}
	err = marshalizer.Unmarshal(fungibleBuyITOData, fungibleBuyITODataBytes)
	assert.Nil(t, err)

	fungibleBuyKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(fungibleKey)
	assert.Nil(t, err)

	fungibleBuyKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(fungibleBuyKDAData, fungibleBuyKDADataBytes)
	assert.Nil(t, err)

	// 680 Un (Amount) * 0.4KLV (Price)
	// Cost = 680*0.4 = 272 KLV

	assert.Equal(t, int64(680_000_000), fungibleBuyITOData.MintedAmount)
	assert.Equal(t, int64(680_000_000+1_000_000), fungibleBuyKDAData.MintedValue)
	assert.Equal(t, int64(999_999_800-272_000_000), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(272_000_200), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTxProcessor_ProcessSellBuyClaimCancelMarketWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	peerAddress := []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc := loadUserAccount(accCacher, testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	peerAcc := loadPeerAccount(accCacher, peerAddress)
	_ = peerAcc.SetOwnerAddress(testToAddress)

	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	proposalKapp := loadKAppAccount(accCacher, kapps.ProposalKAppAddress)
	marketplaceKapp := loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initProposalKapp(proposalKapp)
	initMarketplaceKapp(marketplaceKapp)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = peerDB.SaveAccount(peerAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(proposalKapp)
	_ = kappDB.SaveAccount(marketplaceKapp)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	//Asset Creation to trigger
	kdaContract := transaction.CreateAssetContract{
		Type:         transaction.CreateAssetContract_NonFungible,
		Name:         []byte("NFT"),
		Ticker:       []byte("NFT"),
		OwnerAddress: testOwnerAddress,
		MaxSupply:    10_000,
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
		Staking: &transaction.StakingInfo{MinEpochsToUnstake: 0, MinEpochsToClaim: 0, APR: 16},
	}

	tx, _ := createTransactionMock(&kdaContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	block := createBlockHeader()

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Nil(t, err)

	assetID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), kdaContract.GetOwnerAddress(), ownerAcc.GetNonce(), kdaContract.GetTicker())

	//Mint NFT to test

	nftTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetID,
		ToAddress:   testOwnerAddress,
		Amount:      1,
	}

	nftTriggerTX, _ := createTransactionMock(&nftTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX)
	assert.Nil(t, err)

	nftKey := []byte(string(assetID) + kapps.Sp + "1")

	//Mint NFT to test

	nftTrigger2 := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     assetID,
		ToAddress:   testOwnerAddress,
		Amount:      1,
	}

	nftTriggerTX2, _ := createTransactionMock(&nftTrigger2, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(nftTriggerTX2)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX)
	assert.Nil(t, err)

	nftKey2 := []byte(string(assetID) + kapps.Sp + "2")

	//Sell with invalid market type
	contract := transaction.SellContract{
		MarketType:    999,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       nftKey,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         5,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Sell with invalid marketplaceID address
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: []byte("INVALID"),
		AssetID:       nftKey,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         10,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrNotFoundInKApp, err)

	//Sell with invalid asset
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       []byte("INVALID/1"),
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         10,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrAssetNotFound, err)

	//Sell with invalid price
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       nftKey,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         -10,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Sell with invalid reserve price
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       nftKey,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         10,
		ReservePrice:  -5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//Sell Actually auction NFT for test
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       nftKey,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         1000000,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, nil, err)

	marketIDAUC := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), 0, kdautils.MarketKeyLength)

	//Sell Actually buyItNow NFT for test
	contract = transaction.SellContract{
		MarketType:    transaction.SellContract_BuyItNowMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       nftKey2,
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         1000000,
		ReservePrice:  5,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, nil, err)

	marketIDBIN := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), 0, kdautils.MarketKeyLength)

	//Claim market with wrong type
	claimContract := transaction.ClaimContract{
		ClaimType: 999,
		ID:        marketIDAUC,
	}

	tx, _ = createTransactionMock(&claimContract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrClaimTypeInvalid, err)

	//Claim market without beign finished
	claimContract = transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_MarketClaim,
		ID:        marketIDAUC,
	}

	tx, _ = createTransactionMock(&claimContract, transaction.TXContract_ClaimContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//BuyMarket with wrong type
	buyContract := transaction.BuyContract{
		BuyType:    999,
		ID:         marketIDAUC,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrBuyTypeInvalid, err)

	//BuyMarket with invalid marketID
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         []byte("INVALID"),
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrNotFoundInKApp, err)

	//BuyMarket with invalid currency
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         marketIDAUC,
		CurrencyID: []byte("INVALID"),
		Amount:     1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//BuyMarket with invalid amount
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         marketIDAUC,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     -1,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//BuyMarket buyItNow with amount lower than price
	buyContract = transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         marketIDBIN,
		CurrencyID: kdautils.KLVIdentifier,
		Amount:     100,
	}

	tx, _ = createTransactionMock(&buyContract, transaction.TXContract_BuyContractType, testToAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(createBlockHeader(), hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestTxProcessor_ProcessSellBuyClaimCancelMarketOkValsShouldWork(t *testing.T) {
	t.Parallel()

	receiverAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksg")
	bidderAddress := []byte("klv1d05ju9jaj6u99zph0ant9jh7gkso")
	bidder2Address := []byte("klv1d05ju9jaj6u99zph0ant9jh7gksp")

	nftContract := transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_NonFungible,
		Name:          []byte("NFT"),
		Ticker:        []byte("NFT"),
		OwnerAddress:  testOwnerAddress,
		Precision:     0,
		InitialSupply: 0,
		MaxSupply:     10000,
		Royalties: &transaction.RoyaltiesInfo{
			MarketFixed:      1000,
			MarketPercentage: 100,
			TransferFixed:    1000,
		},
		Properties: &transaction.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	nftTX, _ := createTransactionMock(&nftContract, transaction.TXContract_CreateAssetContractType, testOwnerAddress, 0)

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc := loadUserAccount(accCacher, receiverAddress)
	bidderAcc := loadUserAccount(accCacher, bidderAddress)
	bidder2Acc := loadUserAccount(accCacher, bidder2Address)
	referralAcc := loadUserAccount(accCacher, testReferralAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	marketKapp := loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initMarketplaceKapp(marketKapp)

	_ = ownerAcc.AddToBalance(100000000, nil, true)
	_ = receiverAcc.AddToBalance(100000000, nil, true)
	_ = bidderAcc.AddToBalance(100000000, nil, true)
	_ = bidder2Acc.AddToBalance(100000000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(receiverAcc)
	_ = userDB.SaveAccount(referralAcc)
	_ = userDB.SaveAccount(bidderAcc)
	_ = userDB.SaveAccount(bidder2Acc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(marketKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	marketplaceBytes, err := marketKapp.DataTrieTracker().RetrieveValue(kdautils.ToMarketplaceKey(kdautils.KLVIdentifier))
	assert.Nil(t, err)

	marketplace := &kapps.Marketplace{}
	err = marshalizer.Unmarshal(marketplace, marketplaceBytes)
	assert.Nil(t, err)

	//KDA creation ###################################################

	hash, err := preProcessTransactionMock(nftTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTX)
	assert.Nil(t, err)

	nftID := kda.CreateNewAssetIdentifier(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), nftContract.GetTicker())

	//MINT NFT ######################################################

	nftTrigger := transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     nftID,
		ToAddress:   testOwnerAddress,
		Amount:      3,
	}

	nftTriggerTX, _ := createTransactionMock(&nftTrigger, transaction.TXContract_AssetTriggerContractType, testOwnerAddress, 0)

	hash, err = preProcessTransactionMock(nftTriggerTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, nftTriggerTX)
	assert.Nil(t, err)

	kdaKapp = loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	nftKey := kdautils.ToKDAKey(nftID, nil)
	userNFTKey1 := kdautils.ToKDAKey(nftID, []byte("1"))
	userNFTKey2 := kdautils.ToKDAKey(nftID, []byte("2"))
	userNFTKey3 := kdautils.ToKDAKey(nftID, []byte("3"))

	userNFTBytes1, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	userNFT1 := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFT1, userNFTBytes1)
	assert.Nil(t, err)

	userNFTBytes2, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	userNFT2 := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFT2, userNFTBytes2)
	assert.Nil(t, err)

	userNFTBytes3, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey3)
	assert.Nil(t, err)

	userNFT3 := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFT3, userNFTBytes3)
	assert.Nil(t, err)

	nftKDADataBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(nftKey)
	assert.Nil(t, err)

	nftKDAData := &kapps.KDAData{}
	err = marshalizer.Unmarshal(nftKDAData, nftKDADataBytes)
	assert.Nil(t, err)

	assert.Greater(t, len(userNFTBytes1), 0)
	assert.Greater(t, len(userNFTBytes2), 0)
	assert.Equal(t, []uint8([]byte(nil)), userNFT1.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFT2.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFT3.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFT1.Metadata)
	assert.Equal(t, []uint8([]byte(nil)), userNFT2.Metadata)
	assert.Equal(t, []uint8([]byte(nil)), userNFT3.Metadata)
	assert.Equal(t, int64(3), nftKDAData.MintedValue)
	assert.Equal(t, int64(3), nftKDAData.CirculatingSupply)

	assert.Equal(t, int64(100000000), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(100000000), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(0), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Sell BuyItNow ##############################################################

	sellBINMarket := transaction.SellContract{
		MarketType:    transaction.SellContract_BuyItNowMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       []byte(string(nftID) + "/1"),
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         123456,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	sellBINMarketTX, _ := createTransactionMock(&sellBINMarket, transaction.TXContract_SellContractType, testOwnerAddress, 1)
	hash, err = preProcessTransactionMock(sellBINMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, sellBINMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	BINMarketID := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), sellBINMarketTX.GetSender(), sellBINMarketTX.GetNonce(), 0, kdautils.MarketKeyLength)

	sellWithBINKey := kdautils.ToMarketOrderKey(BINMarketID)

	sellWithBINBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithBINKey)
	assert.Nil(t, err)

	sellWithBINData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(sellWithBINData, sellWithBINBytes)
	assert.Nil(t, err)

	userNFTBytesBIN, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	marketNFTBytesBIN, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	marketNFTDataBIN := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(marketNFTDataBIN, marketNFTBytesBIN)
	assert.Nil(t, err)

	assert.Equal(t, BINMarketID, sellWithBINData.ID)
	assert.Equal(t, kapps.MarketOrderData_BuyItNow, sellWithBINData.MarketType)
	assert.Equal(t, testOwnerAddress, sellWithBINData.OwnerAddress)
	assert.Equal(t, []byte("1"), sellWithBINData.AssetID)
	assert.Equal(t, nftID, sellWithBINData.CollectionID)
	assert.Equal(t, sellBINMarket.CurrencyID, sellWithBINData.CurrencyID)
	assert.Equal(t, sellBINMarket.Price, sellWithBINData.Price)
	assert.Equal(t, sellBINMarket.ReservePrice, sellWithBINData.ReservePrice)
	assert.Equal(t, 0, len(sellWithBINData.CurrentBidder))
	assert.Equal(t, int64(0), sellWithBINData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, sellWithBINData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, sellWithBINData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), sellWithBINData.StartTime)
	assert.Equal(t, sellBINMarket.EndTime, sellWithBINData.EndTime)
	assert.Equal(t, false, sellWithBINData.IsClaimed)

	assert.Equal(t, 0, len(userNFTBytesBIN))
	assert.Equal(t, 2, len(marketNFTBytesBIN))
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBIN.MIME)
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBIN.Metadata)

	assert.Equal(t, int64(99999000), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(100000000), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(0), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Sell Auction ##############################################################

	sellAuctionMarket := transaction.SellContract{
		MarketType:    transaction.SellContract_AuctionMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       []byte(string(nftID) + "/2"),
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         678900,
		ReservePrice:  123456,
		EndTime:       time.Now().AddDate(0, 0, 1).Add(time.Second).Unix(),
	}

	sellAuctionMarketTX, _ := createTransactionMock(&sellAuctionMarket, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	hash, err = preProcessTransactionMock(sellAuctionMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, sellAuctionMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	auctionMarketID := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), 0, kdautils.MarketKeyLength)

	sellWithAuctionKey := kdautils.ToMarketOrderKey(auctionMarketID)

	sellWithAuctionBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithAuctionKey)
	assert.Nil(t, err)

	sellWithAuctionData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(sellWithAuctionData, sellWithAuctionBytes)
	assert.Nil(t, err)

	userNFTBytesAuction, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	marketNFTBytesAuction, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	marketNFTDataAuction := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(marketNFTDataAuction, marketNFTBytesAuction)
	assert.Nil(t, err)

	assert.Equal(t, auctionMarketID, sellWithAuctionData.ID)
	assert.Equal(t, kapps.MarketOrderData_Auction, sellWithAuctionData.MarketType)
	assert.Equal(t, testOwnerAddress, sellWithAuctionData.OwnerAddress)
	assert.Equal(t, []byte("2"), sellWithAuctionData.AssetID)
	assert.Equal(t, nftID, sellWithAuctionData.CollectionID)
	assert.Equal(t, sellAuctionMarket.CurrencyID, sellWithAuctionData.CurrencyID)
	assert.Equal(t, sellAuctionMarket.Price, sellWithAuctionData.Price)
	assert.Equal(t, sellAuctionMarket.ReservePrice, sellWithAuctionData.ReservePrice)
	assert.Equal(t, 0, len(sellWithAuctionData.CurrentBidder))
	assert.Equal(t, int64(0), sellWithAuctionData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, sellWithAuctionData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, sellWithAuctionData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), sellWithAuctionData.StartTime)
	assert.Equal(t, sellAuctionMarket.EndTime, sellWithAuctionData.EndTime)
	assert.Equal(t, false, sellWithAuctionData.IsClaimed)

	assert.Equal(t, 0, len(userNFTBytesAuction))
	assert.Equal(t, 2, len(marketNFTBytesAuction))
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataAuction.MIME)
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataAuction.Metadata)

	assert.Equal(t, int64(99998000), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(100000000), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(0), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Buy BuyItNow ##############################################################

	buyBINMarket := transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         BINMarketID,
		Amount:     123456,
		CurrencyID: kdautils.KLVIdentifier,
	}

	buyBINMarketTX, _ := createTransactionMock(&buyBINMarket, transaction.TXContract_BuyContractType, receiverAddress, 0)

	hash, err = preProcessTransactionMock(buyBINMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, buyBINMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	buyWithBINBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithBINKey)
	assert.Nil(t, err)

	buyWithBINData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(buyWithBINData, buyWithBINBytes)
	assert.Nil(t, err)

	marketNFTBytesBuyBIN, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	userNFTBytesBuyBIN, err := receiverAcc.DataTrieTracker().RetrieveValue(userNFTKey1)
	assert.Nil(t, err)

	userNFTDataBuyBIN := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFTDataBuyBIN, userNFTBytesBuyBIN)
	assert.Nil(t, err)

	assert.Equal(t, BINMarketID, buyWithBINData.ID)
	assert.Equal(t, kapps.MarketOrderData_BuyItNow, buyWithBINData.MarketType)
	assert.Equal(t, testOwnerAddress, buyWithBINData.OwnerAddress)
	assert.Equal(t, []byte("1"), buyWithBINData.AssetID)
	assert.Equal(t, nftID, buyWithBINData.CollectionID)
	assert.Equal(t, sellBINMarket.CurrencyID, buyWithBINData.CurrencyID)
	assert.Equal(t, sellBINMarket.Price, buyWithBINData.Price)
	assert.Equal(t, sellBINMarket.ReservePrice, buyWithBINData.ReservePrice)
	assert.Equal(t, receiverAddress, buyWithBINData.CurrentBidder)
	assert.Equal(t, buyBINMarket.Amount, buyWithBINData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, buyWithBINData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, buyWithBINData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), buyWithBINData.StartTime)
	assert.Equal(t, block.GetTimestamp(), buyWithBINData.EndTime)
	assert.Equal(t, true, buyWithBINData.IsClaimed)

	assert.Equal(t, 0, len(marketNFTBytesBuyBIN))
	assert.Equal(t, 2, len(userNFTBytesBuyBIN))
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataBuyBIN.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataBuyBIN.Metadata)

	assert.Equal(t, int64(100122333), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(99876544), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(123), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Buy Auction ##############################################################

	//First bid - lower than the second
	buyAuctionMarketFirstBid := transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         auctionMarketID,
		Amount:     123456,
		CurrencyID: kdautils.KLVIdentifier,
	}

	buyAuctionMarketFirstBidTX, _ := createTransactionMock(&buyAuctionMarketFirstBid, transaction.TXContract_BuyContractType, bidderAddress, 0)

	hash, err = preProcessTransactionMock(buyAuctionMarketFirstBidTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, buyAuctionMarketFirstBidTX)
	assert.Nil(t, err)

	//Second bid - HIGHER
	buyAuctionSecondBidderMarket := transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         auctionMarketID,
		Amount:     123458,
		CurrencyID: kdautils.KLVIdentifier,
	}

	buyAuctionSecondBidderMarketTX, _ := createTransactionMock(&buyAuctionSecondBidderMarket, transaction.TXContract_BuyContractType, receiverAddress, 0)

	hash, err = preProcessTransactionMock(buyAuctionSecondBidderMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, buyAuctionSecondBidderMarketTX)
	assert.Nil(t, err)

	//Third bid - lower than the second
	buyAuctionThirdBidderMarket := transaction.BuyContract{
		BuyType:    transaction.BuyContract_MarketBuy,
		ID:         auctionMarketID,
		Amount:     123457,
		CurrencyID: kdautils.KLVIdentifier,
	}

	buyAuctionThirdBidderMarketTX, _ := createTransactionMock(&buyAuctionThirdBidderMarket, transaction.TXContract_BuyContractType, bidder2Address, 0)

	hash, err = preProcessTransactionMock(buyAuctionThirdBidderMarketTX)
	assert.Nil(t, err)

	//Must error - lower than the higher price offered
	err = execTx.ProcessTransaction(block, hash, buyAuctionThirdBidderMarketTX)
	assert.Equal(t, common.ErrInvalidValue, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	bidderAcc = loadUserAccount(accCacher, bidderAddress)
	bidder2Acc = loadUserAccount(accCacher, bidder2Address)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	buyWithAuctionBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithAuctionKey)
	assert.Nil(t, err)

	buyWithAuctionData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(buyWithAuctionData, buyWithAuctionBytes)
	assert.Nil(t, err)

	userNFTBytesBuyAuction, err := receiverAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	marketNFTBytesBuyAuction, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	marketNFTDataBuyAuction := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(marketNFTDataBuyAuction, marketNFTBytesBuyAuction)
	assert.Nil(t, err)

	assert.Equal(t, auctionMarketID, buyWithAuctionData.ID)
	assert.Equal(t, kapps.MarketOrderData_Auction, buyWithAuctionData.MarketType)
	assert.Equal(t, testOwnerAddress, buyWithAuctionData.OwnerAddress)
	assert.Equal(t, []byte("2"), buyWithAuctionData.AssetID)
	assert.Equal(t, nftID, buyWithAuctionData.CollectionID)
	assert.Equal(t, sellAuctionMarket.CurrencyID, buyWithAuctionData.CurrencyID)
	assert.Equal(t, sellAuctionMarket.Price, buyWithAuctionData.Price)
	assert.Equal(t, sellAuctionMarket.ReservePrice, buyWithAuctionData.ReservePrice)
	assert.Equal(t, receiverAddress, buyWithAuctionData.CurrentBidder)
	assert.Equal(t, buyAuctionSecondBidderMarket.Amount, buyWithAuctionData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, buyWithAuctionData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, buyWithAuctionData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), buyWithAuctionData.StartTime)
	assert.Equal(t, sellAuctionMarket.EndTime, buyWithAuctionData.EndTime)
	assert.Equal(t, false, buyWithAuctionData.IsClaimed)

	assert.Equal(t, 2, len(marketNFTBytesBuyAuction))
	assert.Equal(t, 0, len(userNFTBytesBuyAuction))
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBuyAuction.MIME)
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBuyAuction.Metadata)

	assert.Equal(t, int64(100122333), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(99753086), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(123), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Check bidders balance
	assert.Equal(t, int64(100000000), bidderAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(100000000), bidder2Acc.GetBalance(kdautils.KLVIdentifier, true))

	//Claim Auction ##############################################################

	startedTimestamp := block.Header.Timestamp
	block.Header.Timestamp = time.Unix(block.Header.Timestamp, 0).AddDate(0, 0, 1).Unix()

	claimAuctionMarket := transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_MarketClaim,
		ID:        auctionMarketID,
	}

	claimAuctionMarketTX, _ := createTransactionMock(&claimAuctionMarket, transaction.TXContract_ClaimContractType, receiverAddress, 0)

	hash, err = preProcessTransactionMock(claimAuctionMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, claimAuctionMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	claimWithAuctionBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithAuctionKey)
	assert.Nil(t, err)

	claimWithAuctionData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(claimWithAuctionData, claimWithAuctionBytes)
	assert.Nil(t, err)

	marketNFTBytesClaimAuction, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	userNFTBytesClaimAuction, err := receiverAcc.DataTrieTracker().RetrieveValue(userNFTKey2)
	assert.Nil(t, err)

	userNFTDataClaimAuction := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFTDataClaimAuction, userNFTBytesClaimAuction)
	assert.Nil(t, err)

	assert.Equal(t, auctionMarketID, claimWithAuctionData.ID)
	assert.Equal(t, kapps.MarketOrderData_Auction, claimWithAuctionData.MarketType)
	assert.Equal(t, testOwnerAddress, claimWithAuctionData.OwnerAddress)
	assert.Equal(t, []byte("2"), claimWithAuctionData.AssetID)
	assert.Equal(t, nftID, claimWithAuctionData.CollectionID)
	assert.Equal(t, sellAuctionMarket.CurrencyID, claimWithAuctionData.CurrencyID)
	assert.Equal(t, sellAuctionMarket.Price, claimWithAuctionData.Price)
	assert.Equal(t, sellAuctionMarket.ReservePrice, claimWithAuctionData.ReservePrice)
	assert.Equal(t, receiverAddress, claimWithAuctionData.CurrentBidder)
	assert.Equal(t, buyAuctionSecondBidderMarket.Amount, claimWithAuctionData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, claimWithAuctionData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, claimWithAuctionData.RoyaltiesFixedDeposit)
	assert.Equal(t, startedTimestamp, claimWithAuctionData.StartTime)
	assert.Equal(t, sellAuctionMarket.EndTime, claimWithAuctionData.EndTime)
	assert.Equal(t, true, claimWithAuctionData.IsClaimed)

	assert.Equal(t, 0, len(marketNFTBytesClaimAuction))
	assert.Equal(t, 2, len(userNFTBytesClaimAuction))
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataClaimAuction.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataClaimAuction.Metadata)

	assert.Equal(t, int64(100246668), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(99753086), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(246), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Sell BuyItNow for Cancel ##############################################################
	block.Header.Timestamp = startedTimestamp

	sellBINCCMarket := transaction.SellContract{
		MarketType:    transaction.SellContract_BuyItNowMarket,
		MarketplaceID: kdautils.KLVIdentifier,
		AssetID:       []byte(string(nftID) + "/3"),
		CurrencyID:    kdautils.KLVIdentifier,
		Price:         123456,
		EndTime:       time.Now().AddDate(0, 0, 1).Unix(),
	}

	sellBINCCMarketTX, _ := createTransactionMock(&sellBINCCMarket, transaction.TXContract_SellContractType, testOwnerAddress, 0)

	hash, err = preProcessTransactionMock(sellBINCCMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, sellBINCCMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	BINCCMarketID := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), 0, kdautils.MarketKeyLength)

	sellWithBINCCKey := kdautils.ToMarketOrderKey(BINCCMarketID)

	sellWithBINCCBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithBINCCKey)
	assert.Nil(t, err)

	sellWithBINCCData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(sellWithBINCCData, sellWithBINCCBytes)
	assert.Nil(t, err)

	userNFTBytesBINCC, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey3)
	assert.Nil(t, err)

	marketNFTBytesBINCC, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey3)
	assert.Nil(t, err)

	marketNFTDataBINCC := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(marketNFTDataBINCC, marketNFTBytesBINCC)
	assert.Nil(t, err)

	assert.Equal(t, BINCCMarketID, sellWithBINCCData.ID)
	assert.Equal(t, kapps.MarketOrderData_BuyItNow, sellWithBINCCData.MarketType)
	assert.Equal(t, testOwnerAddress, sellWithBINCCData.OwnerAddress)
	assert.Equal(t, []byte("3"), sellWithBINCCData.AssetID)
	assert.Equal(t, nftID, sellWithBINCCData.CollectionID)
	assert.Equal(t, sellBINCCMarket.CurrencyID, sellWithBINCCData.CurrencyID)
	assert.Equal(t, sellBINCCMarket.Price, sellWithBINCCData.Price)
	assert.Equal(t, sellBINCCMarket.ReservePrice, sellWithBINCCData.ReservePrice)
	assert.Equal(t, 0, len(sellWithBINCCData.CurrentBidder))
	assert.Equal(t, int64(0), sellWithBINCCData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, sellWithBINCCData.ReferralPercentage)
	assert.Equal(t, nftContract.Royalties.MarketFixed, sellWithBINCCData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), sellWithBINCCData.StartTime)
	assert.Equal(t, sellBINCCMarket.EndTime, sellWithBINCCData.EndTime)
	assert.Equal(t, false, sellWithBINCCData.IsClaimed)

	assert.Equal(t, 0, len(userNFTBytesBINCC))
	assert.Equal(t, 2, len(marketNFTBytesBINCC))
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBINCC.MIME)
	assert.Equal(t, []uint8([]byte(nil)), marketNFTDataBINCC.Metadata)

	assert.Equal(t, int64(100245668), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(99753086), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(246), referralAcc.GetBalance(kdautils.KLVIdentifier, true))

	//Cancel Market ##############################################################
	cancelBINMarket := transaction.CancelMarketOrderContract{
		OrderID: BINCCMarketID,
	}

	cancelBINMarketTX, _ := createTransactionMock(&cancelBINMarket, transaction.TXContract_CancelMarketOrderContractType, testOwnerAddress, 0)

	hash, err = preProcessTransactionMock(cancelBINMarketTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, cancelBINMarketTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)
	ownerAcc = loadUserAccount(accCacher, testOwnerAddress)
	receiverAcc = loadUserAccount(accCacher, receiverAddress)
	referralAcc = loadUserAccount(accCacher, testReferralAddress)

	cancelBINBytes, err := marketKapp.DataTrieTracker().RetrieveValue(sellWithBINCCKey)
	assert.Nil(t, err)

	cancelBINData := &kapps.MarketOrderData{}
	err = marshalizer.Unmarshal(cancelBINData, cancelBINBytes)
	assert.Nil(t, err)

	marketNFTBytesCancelBIN, err := marketKapp.DataTrieTracker().RetrieveValue(userNFTKey3)
	assert.Nil(t, err)

	userNFTBytesCancelBIN, err := ownerAcc.DataTrieTracker().RetrieveValue(userNFTKey3)
	assert.Nil(t, err)

	userNFTDataCancelBIN := &kapps.UserKDA{}
	err = marshalizer.Unmarshal(userNFTDataCancelBIN, userNFTBytesCancelBIN)
	assert.Nil(t, err)

	assert.Equal(t, BINCCMarketID, cancelBINData.ID)
	assert.Equal(t, kapps.MarketOrderData_BuyItNow, cancelBINData.MarketType)
	assert.Equal(t, testOwnerAddress, cancelBINData.OwnerAddress)
	assert.Equal(t, []byte("3"), cancelBINData.AssetID)
	assert.Equal(t, nftID, cancelBINData.CollectionID)
	assert.Equal(t, sellBINCCMarket.CurrencyID, cancelBINData.CurrencyID)
	assert.Equal(t, sellBINCCMarket.Price, cancelBINData.Price)
	assert.Equal(t, sellBINCCMarket.ReservePrice, cancelBINData.ReservePrice)
	assert.Equal(t, 0, len(cancelBINData.CurrentBidder))
	assert.Equal(t, int64(0), cancelBINData.CurrentBid)
	assert.Equal(t, testReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, marketplace.ReferralPercentage, cancelBINData.ReferralPercentage)
	assert.Equal(t, int64(0), cancelBINData.RoyaltiesFixedDeposit)
	assert.Equal(t, block.GetTimestamp(), cancelBINData.StartTime)
	assert.Equal(t, block.GetTimestamp(), cancelBINData.EndTime)
	assert.Equal(t, true, cancelBINData.IsClaimed)

	assert.Equal(t, 0, len(marketNFTBytesCancelBIN))
	assert.Equal(t, 2, len(userNFTBytesCancelBIN))
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataCancelBIN.MIME)
	assert.Equal(t, []uint8([]byte(nil)), userNFTDataCancelBIN.Metadata)

	assert.Equal(t, int64(100246668), ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(99753086), receiverAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, int64(246), referralAcc.GetBalance(kdautils.KLVIdentifier, true))
}

func TestTxProcessor_ProcessCreateConfigMarketplaceWrongValsShouldErr(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	toAcc := loadUserAccount(accCacher, testToAddress)
	referralAcc := loadUserAccount(accCacher, testReferralAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	marketKapp := loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initMarketplaceKapp(marketKapp)

	_ = ownerAcc.AddToBalance(100000000, nil, true)
	_ = toAcc.AddToBalance(100000000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = userDB.SaveAccount(referralAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(marketKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	//CreateMarketplace with invalid referral address
	contract := transaction.CreateMarketplaceContract{
		Name:               []byte("TEST"),
		ReferralAddress:    []byte("INVALID"),
		ReferralPercentage: 10,
	}

	tx, _ := createTransactionMock(&contract, transaction.TXContract_CreateMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//CreateMarketplace with invalid referral percentage
	contract = transaction.CreateMarketplaceContract{
		Name:               []byte("TEST"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 9999999,
	}

	tx, _ = createTransactionMock(&contract, transaction.TXContract_CreateMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigMarketplace with invalid marketplace id
	cfgContract := transaction.ConfigMarketplaceContract{
		MarketplaceID:      []byte("INVALID"),
		Name:               []byte("TEST"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 10,
	}

	tx, _ = createTransactionMock(&cfgContract, transaction.TXContract_ConfigMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrNotFoundInKApp, err)

	//ConfigMarketplace with invalid referral address
	cfgContract = transaction.ConfigMarketplaceContract{
		MarketplaceID:      kdautils.KLVIdentifier,
		Name:               []byte("TEST"),
		ReferralAddress:    []byte("INVALID"),
		ReferralPercentage: 10,
	}

	tx, _ = createTransactionMock(&cfgContract, transaction.TXContract_ConfigMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)

	//ConfigMarketplace with invalid referral percentage
	cfgContract = transaction.ConfigMarketplaceContract{
		MarketplaceID:      kdautils.KLVIdentifier,
		Name:               []byte("TEST"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 9999999,
	}

	tx, _ = createTransactionMock(&cfgContract, transaction.TXContract_ConfigMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(tx)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, tx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestTxProcessor_ProcessCreateConfigMarketplaceOkValsShouldWork(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	ownerAcc := loadUserAccount(accCacher, testOwnerAddress)
	toAcc := loadUserAccount(accCacher, testToAddress)
	referralAcc := loadUserAccount(accCacher, testReferralAddress)
	peerAcc := loadPeerAccount(accCacher, []byte("PEER"))
	kdaKapp := loadKAppAccount(accCacher, kapps.KDAKAppAddress)
	stakingKapp := loadKAppAccount(accCacher, kapps.StakingKAppAddress)
	marketKapp := loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	initMarketplaceKapp(marketKapp)

	_ = ownerAcc.AddToBalance(100000000, nil, true)
	_ = toAcc.AddToBalance(100000000, nil, true)

	_ = userDB.SaveAccount(ownerAcc)
	_ = userDB.SaveAccount(toAcc)
	_ = userDB.SaveAccount(referralAcc)
	_ = kappDB.SaveAccount(kdaKapp)
	_ = kappDB.SaveAccount(stakingKapp)
	_ = kappDB.SaveAccount(marketKapp)
	_ = peerDB.SaveAccount(peerAcc)

	args := createArgsForTxProcessor()
	args.AccountsCacher = accCacher
	execTx := NewTXProcessor(t, args)

	block := createBlockHeader()

	//Create Marketplace ######################################################
	createMarketplace := transaction.CreateMarketplaceContract{
		Name:               []byte("NewMarketplace"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 20,
	}

	createMarketplaceTX, _ := createTransactionMock(&createMarketplace, transaction.TXContract_CreateMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err := execTx.PreProcessTransaction(createMarketplaceTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, createMarketplaceTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	marketplaceID := kdautils.ToMarketID(args.Hasher, block.GetRandSeed(), testOwnerAddress, ownerAcc.GetNonce(), 0, kdautils.MarketKeyLength)

	marketplaceKey := kdautils.ToMarketplaceKey(marketplaceID)

	marketplaceBytes, err := marketKapp.DataTrieTracker().RetrieveValue(marketplaceKey)
	assert.Nil(t, err)

	marketplace := &kapps.Marketplace{}
	err = marshalizer.Unmarshal(marketplace, marketplaceBytes)
	assert.Nil(t, err)

	assert.Equal(t, marketplaceID, marketplace.ID)
	assert.Equal(t, createMarketplace.Name, marketplace.Name)
	assert.Equal(t, testOwnerAddress, marketplace.OwnerAddress)
	assert.Equal(t, createMarketplace.ReferralAddress, marketplace.ReferralAddress)
	assert.Equal(t, createMarketplace.ReferralPercentage, marketplace.ReferralPercentage)

	//Config Marketplace ######################################################
	configMarketplace := transaction.ConfigMarketplaceContract{
		MarketplaceID:      marketplaceID,
		Name:               []byte("AlteredName"),
		ReferralAddress:    testReferralAddress,
		ReferralPercentage: 30,
	}

	configMarketplaceTX, _ := createTransactionMock(&configMarketplace, transaction.TXContract_ConfigMarketplaceContractType, testOwnerAddress, 0)

	_, hash, err = execTx.PreProcessTransaction(configMarketplaceTX)
	assert.Nil(t, err)

	err = execTx.ProcessTransaction(block, hash, configMarketplaceTX)
	assert.Nil(t, err)

	marketKapp = loadKAppAccount(accCacher, kapps.MarketKAppAddress)

	configMarketplaceBytes, err := marketKapp.DataTrieTracker().RetrieveValue(marketplaceKey)
	assert.Nil(t, err)

	configMarketplaceData := &kapps.Marketplace{}
	err = marshalizer.Unmarshal(configMarketplaceData, configMarketplaceBytes)
	assert.Nil(t, err)

	assert.Equal(t, marketplaceID, configMarketplaceData.ID)
	assert.Equal(t, configMarketplace.Name, configMarketplaceData.Name)
	assert.Equal(t, testOwnerAddress, configMarketplaceData.OwnerAddress)
	assert.Equal(t, configMarketplace.ReferralAddress, configMarketplaceData.ReferralAddress)
	assert.Equal(t, configMarketplace.ReferralPercentage, configMarketplaceData.ReferralPercentage)
}
