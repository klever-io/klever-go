package smartContract

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	commommock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	accountsKapp "github.com/klever-io/klever-go/core/kapp/accounts"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/postprocess"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	vmData "github.com/klever-io/klever-go/data/vm"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	integrationTestsMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/storage/txcache"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func createMockPubkeyConverter() *mock.PubkeyConverterMock {
	return mock.NewPubkeyConverterMock(32)
}

func createDefaultContext(tx data.TransactionHandler) kapp.KappContext {
	return kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: []byte("SRC"),
		ContractID:     0,
		ContractType:   transaction.TXContract_SmartContractType,
		Block:          &block.Block{},
		TxHash:         []byte("TX_HASH"),
		TX:             tx,
	})
}

func createAccounts(tx data.TransactionHandler) (state.UserAccountHandler, state.UserAccountHandler) {
	acntSrc, _ := state.NewUserAccount(tx.GetSender())

	smartContract, _ := tx.GetRaw().GetContract()[0].GetSmartContract()

	klvAmount := int64(0)
	if smartContract != nil && smartContract.CallValue != nil && smartContract.CallValue["KLV"] != nil {
		klvAmount = smartContract.CallValue["KLV"].Amount
	}

	acntSrc.Balance = klvAmount + int64(tx.GetGasLimit())

	var acntDst state.UserAccountHandler
	if smartContract != nil {
		acntDst, _ = state.NewUserAccount(smartContract.Address)
	}

	return acntSrc, acntDst
}

func initAccounts(tx data.TransactionHandler, accCacher state.AccountsCacher, acntSrcBalance int64) (state.UserAccountHandler, state.UserAccountHandler) {
	acntSrc, _ := accCacher.LoadUser(tx.GetSender())

	smartContract, _ := tx.GetRaw().GetContract()[0].GetSmartContract()

	_ = acntSrc.AddToBalance(acntSrcBalance, []byte("KLV"), false)

	var acntDst state.UserAccountHandler
	if smartContract != nil {
		acntDst, _ = accCacher.LoadUser(smartContract.Address)
	}

	return acntSrc, acntDst
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
	marshalizer := marshal.NewProtoMarshalizer()
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

func createTransactionMock(contract proto.Message, txType transaction.TXContract_ContractType, sender []byte, nonce uint64, data [][]byte) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{Signature: make([][]byte, 1)}

	serialized, err := marshal.NewProtoMarshalizer().Marshal(contract)
	if err != nil {
		return nil, errors.New("could not serialize contract")
	}

	tx.RawData = &transaction.Transaction_Raw{
		Data:   data,
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

func initKLVAndKFIintoKapps(accCacher state.AccountsCacher) {
	kdaKapp, _ := accCacher.LoadKApp(kapps.KDAKAppAddress)
	stakingKapp, _ := accCacher.LoadKApp(kapps.StakingKAppAddress)

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

	stakingData, _ := marshal.NewProtoMarshalizer().Marshal(&staking)
	aprStakingData, _ := marshal.NewProtoMarshalizer().Marshal(&aprStaking)

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

	klvData, _ := marshal.NewProtoMarshalizer().Marshal(&klv)
	kfiData, _ := marshal.NewProtoMarshalizer().Marshal(&kfi)
	aprData, _ := marshal.NewProtoMarshalizer().Marshal(&apr)

	_ = kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(kfiKey, kfiData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(aprKey, aprData)

	_ = accCacher.SaveAll()
}

func createMockSmartContractProcessorArguments() ArgsNewSmartContractProcessor {
	_, _, _, accCacher := createFullArgumentsForKAppsProcessing(createMemUnit())

	initKLVAndKFIintoKapps(accCacher)

	gasSchedule := make(map[string]map[string]uint64)
	gasSchedule[common.BaseOpsAPICost] = make(map[string]uint64)
	gasSchedule[common.BuiltInCost] = make(map[string]uint64)
	gasSchedule[common.BuiltInCost][core.BuiltInFunctionTransfer] = 2000
	gasSchedule[common.BaseOperationCost] = map[string]uint64{"StorePerByte": 1, "PersistPerByte": 1}

	txFeeHandler, _ := postprocess.NewFeeAccumulator()

	epochNotifier := &commommock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
		SmartContracts:        0,
	}, epochNotifier)

	return ArgsNewSmartContractProcessor{
		VmContainer:      &contextmock.VMContainerMock{},
		ArgsParser:       &contextmock.ArgumentParserMock{},
		Hasher:           &mock.HasherMock{},
		Marshalizer:      marshal.NewProtoMarshalizer(),
		AccountsCacher:   accCacher,
		BlockChainHook:   &contextmock.BlockchainHookStub{},
		BuiltInFunctions: builtInFunctions.NewBuiltInFunctionContainer(),
		PubkeyConv:       createMockPubkeyConverter(),
		TxFeeHandler:     txFeeHandler,
		TxLogsProcessor:  &contextmock.TxLogsProcessorStub{},
		EconomicsFee: &commommock.FeeHandlerStub{
			ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
				return &transaction.CostResponse{
					KAppFee:       tx.GetKAppFee(),
					BandwidthFee:  tx.GetBandwidthFee(),
					GasEstimated:  tx.GetTransaction().GetGasLimit(),
					GasMultiplier: tx.GetTransaction().GetGasLimit(),
				}, nil

			},
		},
		GasSchedule:        notifierMock.NewGasScheduleNotifierMock(gasSchedule),
		WasmVMChangeLocker: &sync.RWMutex{},
		VMOutputCacher:     txcache.NewDisabledCache(),
		ForkController:     forkController,
		AllowDeploy:        true,
	}
}

// ===================== TestNewSmartContractProcessor =====================
func TestNewSmartContractProcessorNilVM(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Nil(t, sc)
	require.Equal(t, process.ErrNoVM, err)
}

func TestNewSmartContractProcessorNilVMOutputCacher(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.VMOutputCacher = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Nil(t, sc)
	require.Equal(t, process.ErrNilCacher, err)
}

func TestNewSmartContractProcessorNilBuiltInFunctions(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.BuiltInFunctions = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Nil(t, sc)
	require.Equal(t, process.ErrNilBuiltInFunction, err)
}

func TestNewSmartContractProcessorNilArgsParser(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.ArgsParser = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilArgumentParser, err)
}

func TestNewSmartContractProcessorNilHasher(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.Hasher = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilHasher, err)
}

func TestNewSmartContractProcessorNilMarshalizer(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.Marshalizer = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewSmartContractProcessorNilAccountsDB(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.AccountsCacher = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilCacher, err)
}

func TestNewSmartContractProcessorNilAdrConv(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.PubkeyConv = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilPubkeyConverter, err)
}

func TestNewSmartContractProcessorNilFakeAccountsHandler(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.BlockChainHook = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilTemporaryAccountsHandler, err)
}

func TestNewSmartContractProcessor_ErrNilUnsignedTxHandlerMock(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.TxFeeHandler = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilUnsignedTxHandler, err)
}

func TestNewSmartContractProcessor_NilEnableEpochsHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.ForkController = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilEnableEpochsHandler, err)
}

func TestNewSmartContractProcessor_NilEconomicsFeeShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.EconomicsFee = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilEconomicsFeeHandler, err)
}

func TestNewSmartContractProcessor_NilGasScheduleShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.GasSchedule = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilGasSchedule, err)
}

func TestNewSmartContractProcessor_NilLatestGasScheduleShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.GasSchedule = notifierMock.NewGasScheduleNotifierMock(nil)
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilGasSchedule, err)
}

func TestNewSmartContractProcessor_NilTxLogsProcessorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.TxLogsProcessor = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilTxLogsProcessor, err)
}

func TestNewSmartContractProcessor_NilLockerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.WasmVMChangeLocker = nil
	sc, err := NewSmartContractProcessor(arguments)

	require.Nil(t, sc)
	require.Equal(t, process.ErrNilLocker, err)
}

func TestNewSmartContractProcessor_ShouldRegisterNotifiers(t *testing.T) {
	t.Parallel()

	gasScheduleRegisterCalled := false

	arguments := createMockSmartContractProcessorArguments()
	gasSchedule := notifierMock.NewGasScheduleNotifierMock(make(map[string]map[string]uint64))
	gasSchedule.RegisterNotifyHandlerCalled = func(handler core.GasScheduleSubscribeHandler) {
		gasScheduleRegisterCalled = true
	}
	arguments.GasSchedule = gasSchedule

	_, _ = NewSmartContractProcessor(arguments)

	require.True(t, gasScheduleRegisterCalled)
}

func TestNewSmartContractProcessor(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, err := NewSmartContractProcessor(arguments)

	require.NotNil(t, sc)
	require.Nil(t, err)
	require.False(t, sc.IsInterfaceNil())
}

func TestNewSmartContractProcessorVerifyAllMembers(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, _ := NewSmartContractProcessor(arguments)

	assert.Equal(t, arguments.VmContainer, sc.vmContainer)
	assert.Equal(t, arguments.ArgsParser, sc.argsParser)
	assert.Equal(t, arguments.Hasher, sc.hasher)
	assert.Equal(t, arguments.AccountsCacher, sc.accountsCacher)
	assert.Equal(t, arguments.BlockChainHook, sc.blockChainHook)
	assert.Equal(t, arguments.PubkeyConv, sc.pubkeyConv)
	assert.Equal(t, arguments.TxFeeHandler, sc.txFeeHandler)
	assert.Equal(t, arguments.EconomicsFee, sc.economicsFee)
	assert.Equal(t, arguments.TxLogsProcessor, sc.txLogsProcessor)
	assert.Equal(t, arguments.ForkController, sc.forkController)
}

// ===================== TestGasScheduleChange =====================

func TestGasScheduleChangeNoApiCostShouldNotChange(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, _ := NewSmartContractProcessor(arguments)

	gasSchedule := make(map[string]map[string]uint64)
	gasSchedule[common.BuiltInCost] = nil

	sc.GasScheduleChange(gasSchedule)
	require.Equal(t, sc.builtInGasCosts[core.BuiltInFunctionAssetTrigger], uint64(0))

	gasSchedule[common.BuiltInCost] = make(map[string]uint64)
	gasSchedule[common.BuiltInCost][core.BuiltInFunctionAssetTrigger] = 2000
	gasSchedule[common.BaseOperationCost] = map[string]uint64{"StorePerByte": 1, "PersistPerByte": 1}
	sc.GasScheduleChange(gasSchedule)
	require.Equal(t, sc.builtInGasCosts[core.BuiltInFunctionAssetTrigger], uint64(2000))
}

func TestGasScheduleChangeShouldWork(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, _ := NewSmartContractProcessor(arguments)

	gasSchedule := make(map[string]map[string]uint64)
	gasSchedule[common.BuiltInCost] = make(map[string]uint64)
	gasSchedule[common.BuiltInCost][core.BuiltInFunctionTransfer] = 20
	gasSchedule[common.BaseOperationCost] = map[string]uint64{"StorePerByte": 3, "PersistPerByte": 4}

	sc.GasScheduleChange(gasSchedule)

	require.Equal(t, sc.builtInGasCosts[core.BuiltInFunctionTransfer], uint64(20))
	require.Equal(t, uint64(3), sc.storePerByte)
	require.Equal(t, uint64(4), sc.persistPerByte)
}

func TestGasScheduleChangeWithMissingPerByteCostKeepsPrevious(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, _ := NewSmartContractProcessor(arguments)
	storePerByte, persistPerByte := sc.storePerByte, sc.persistPerByte

	for _, missing := range []string{"StorePerByte", "PersistPerByte"} {
		gasSchedule := map[string]map[string]uint64{
			common.BuiltInCost:       {core.BuiltInFunctionTransfer: 20},
			common.BaseOperationCost: {"StorePerByte": 3, "PersistPerByte": 4},
		}
		delete(gasSchedule[common.BaseOperationCost], missing)

		err := CheckGasSchedule(gasSchedule)
		require.ErrorIs(t, err, common.ErrInvalidGasSchedule)
		require.Contains(t, err.Error(), missing)

		sc.GasScheduleChange(gasSchedule)
		require.Equal(t, storePerByte, sc.storePerByte)
		require.Equal(t, persistPerByte, sc.persistPerByte)
		require.NotEqual(t, uint64(20), sc.builtInGasCosts[core.BuiltInFunctionTransfer])
	}
}

// ===================== TestDeploySmartContract =====================
func mockParserAndArguments() (*contextmock.ArgumentParserMock, ArgsNewSmartContractProcessor) {
	argParser := &contextmock.ArgumentParserMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = &contextmock.VMContainerMock{}
	arguments.ArgsParser = argParser

	return argParser, arguments

}
func TestScProcessor_DeploySmartContractBadParse(t *testing.T) {
	t.Parallel()

	argParser, arguments := mockParserAndArguments()
	parseError := fmt.Errorf("fooError")
	argParser.ParseDeployDataCalled = func(data string) (*parsers.DeployArgs, error) {
		return nil, parseError
	}

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	acntSrc, _ := createAccounts(tx)

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	returnCode, err := sc.DeploySmartContract(ctx, tc)

	require.Equal(t, parseError, err)
	require.Equal(t, vmcommon.VMUserError.ResultCode(), returnCode.ResultCode())
	require.True(t, acntSrc.GetBalance(kdautils.KLVKey, false) == 0)
}

func TestScProcessor_DeploySmartContractWithAddressShouldFail(t *testing.T) {
	t.Parallel()

	_, arguments := mockParserAndArguments()

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	acntSrc, _ := createAccounts(tx)

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	returnCode, err := sc.DeploySmartContract(ctx, tc)

	require.Equal(t, process.ErrInvalidRcvAddr, err)
	require.Equal(t, vmcommon.VMContractInvalid.ResultCode(), returnCode.ResultCode())
	require.True(t, acntSrc.GetBalance(kdautils.KLVKey, false) == 0)
}

func TestScProcessor_DeploySmartContractRunError(t *testing.T) {
	t.Parallel()

	vmContainer := &contextmock.VMContainerMock{}
	argParser := NewArgumentParser()
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	_, _ = createAccounts(tx)

	vm := &contextmock.VMExecutionHandlerStub{}

	createError := fmt.Errorf("fooError")
	vm.RunSmartContractCreateCalled = func(input *vmcommon.ContractCreateInput) (output *vmcommon.VMOutput, e error) {
		return nil, createError
	}

	vmContainer.GetCalled = func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
		return vm, nil
	}

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	returnCode, err := sc.DeploySmartContract(ctx, tc)

	require.Equal(t, createError, err)
	require.Equal(t, vmcommon.VMUserError.ResultCode(), returnCode.ResultCode())
}

func TestScProcessor_DeploySmartContractDisabled(t *testing.T) {
	t.Parallel()

	vmContainer := &contextmock.VMContainerMock{}
	argParser := NewArgumentParser()
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	arguments.ForkController = &integrationTestsMock.ForkControllerStub{
		EnableSmartContractsCalled: func() bool {
			return false
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	_, _ = createAccounts(tx)

	vm := &contextmock.VMExecutionHandlerStub{}
	vmContainer.GetCalled = func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
		return vm, nil
	}

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	returnCode, err := sc.DeploySmartContract(ctx, tc)
	require.Equal(t, common.ErrInvalidContract, err)

	require.Equal(t, vmcommon.VMContractInvalid.ResultCode(), returnCode.ResultCode())
}

func TestScProcessor_DeploySmartContractNilTx(t *testing.T) {
	t.Parallel()

	vm := &contextmock.VMContainerMock{}
	argParser := &contextmock.ArgumentParserMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, _ := NewSmartContractProcessor(arguments)

	_, err := sc.DeploySmartContract(nil, nil)
	require.Equal(t, process.ErrNilTransaction, err)
}

func TestScProcessor_DeploySmartContractCalculateHashFails(t *testing.T) {
	t.Parallel()

	vm := &contextmock.VMContainerMock{}
	argParser := &contextmock.ArgumentParserMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	arguments.Marshalizer = &mock.MarshalizerMock{
		Fail: true,
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	_, err = tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Equal(t, "MarshalizerMock generic error", err.Error())
}

func TestScProcessor_DeploySmartContractVmContainerGetFails(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("expected error")

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	vm.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
		return nil, expectedError
	}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, _ := NewSmartContractProcessor(arguments)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	_, _ = createAccounts(tx)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	code, err := sc.DeploySmartContract(ctx, tc)
	require.Equal(t, expectedError, err)
	require.Equal(t, vmcommon.VMUserError.ResultCode(), code.ResultCode())
}

func TestScProcessor_DeploySmartContractVmExecuteCreateSCFails(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("expected error")

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	vmExecutor := &contextmock.VMExecutionHandlerStub{
		RunSmartContractCreateCalled: func(input *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
			return nil, expectedError
		},
	}
	vm.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
		return vmExecutor, nil
	}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, _ := NewSmartContractProcessor(arguments)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	_, _ = createAccounts(tx)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	code, err := sc.DeploySmartContract(ctx, tc)
	require.Equal(t, expectedError, err)
	require.Equal(t, vmcommon.VMUserError.ResultCode(), code.ResultCode())
}

func TestScProcessor_DeploySmartContractVmExecuteCreateSCVMOutputNil(t *testing.T) {
	t.Parallel()

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	vmExecutor := &contextmock.VMExecutionHandlerStub{
		RunSmartContractCreateCalled: func(input *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
			return nil, nil
		},
	}
	vm.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
		return vmExecutor, nil
	}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, _ := NewSmartContractProcessor(arguments)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	_, _ = createAccounts(tx)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	code, err := sc.DeploySmartContract(ctx, tc)
	require.Equal(t, process.ErrNilVMOutput, err)
	require.Equal(t, vmcommon.VMUserError.ResultCode(), code.ResultCode())
}

func TestScProcessor_DeploySmartContractVmOutputReturnCodeNotOk(t *testing.T) {
	t.Parallel()

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	vmExecutor := &contextmock.VMExecutionHandlerStub{
		RunSmartContractCreateCalled: func(input *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
			return &vmcommon.VMOutput{ReturnCode: vmcommon.VMCallStackOverFlow, ReturnMessage: "returnMessage"}, nil
		},
	}
	vm.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
		return vmExecutor, nil
	}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, _ := NewSmartContractProcessor(arguments)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	_, _ = createAccounts(tx)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	code, err := sc.DeploySmartContract(ctx, tc)
	require.Equal(t, vmcommon.VMCallStackOverFlow.String(), err.Error())
	require.Equal(t, vmcommon.VMUserError.ResultCode(), code.ResultCode())
}

// KLC-2722: from FixAuditChangesV7 a deploy records its execution time and keeps the VM's
// VMExecutionFailed on a host timeout, so validateTransactionResult can judge it like a call.
// Before the fork every VM error is VMUserError and the execution time stays unset.
func TestScProcessor_DeploySmartContractTimeoutResultAndExecutionTime(t *testing.T) {
	t.Parallel()

	// Every mock takes measurable time, so "execution time recorded" never hinges on timer resolution.
	vmTimeout := func(_ *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
		time.Sleep(time.Millisecond)
		// what hostCore.finalizeTimeout hands back
		return &vmcommon.VMOutput{
			ReturnCode:    vmcommon.VMExecutionFailed,
			ReturnMessage: vmhost.ErrExecutionFailedWithTimeout.Error(),
		}, vmhost.ErrExecutionFailedWithTimeout
	}
	vmPanicNoOutput := func(_ *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
		time.Sleep(time.Millisecond)
		return nil, vmhost.ErrExecutionPanicked
	}
	vmErrorOtherCode := func(_ *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
		time.Sleep(time.Millisecond)
		return &vmcommon.VMOutput{ReturnCode: vmcommon.VMOutOfGas}, errors.New("vm error")
	}
	vmSuccess := func(_ *vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error) {
		time.Sleep(time.Millisecond)
		return &vmcommon.VMOutput{}, nil
	}

	// The execution time is recorded exactly when V7 is on, whatever the outcome.
	tests := []struct {
		name         string
		fixV7        bool
		run          func(*vmcommon.ContractCreateInput) (*vmcommon.VMOutput, error)
		expectedCode vmcommon.ReturnCode
		expectedErr  string
	}{
		{"V7 timeout keeps VMExecutionFailed", true, vmTimeout, vmcommon.VMExecutionFailed, vmhost.ErrExecutionFailedWithTimeout.Error()},
		{"V7 error without output stays VMUserError", true, vmPanicNoOutput, vmcommon.VMUserError, vmhost.ErrExecutionPanicked.Error()},
		{"V7 error with another code stays VMUserError", true, vmErrorOtherCode, vmcommon.VMUserError, "vm error"},
		{"V7 success", true, vmSuccess, vmcommon.Ok, ""},
		{"pre-V7 timeout is VMUserError", false, vmTimeout, vmcommon.VMUserError, vmhost.ErrExecutionFailedWithTimeout.Error()},
		{"pre-V7 error without output is VMUserError", false, vmPanicNoOutput, vmcommon.VMUserError, vmhost.ErrExecutionPanicked.Error()},
		{"pre-V7 success", false, vmSuccess, vmcommon.Ok, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &contextmock.VMContainerMock{}
			vmExecutor := &contextmock.VMExecutionHandlerStub{RunSmartContractCreateCalled: tt.run}
			vm.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
				return vmExecutor, nil
			}
			forkController := commommock.NewForkControllerStub()
			forkController.FixAuditChangesV7Value = tt.fixV7

			arguments := createMockSmartContractProcessorArguments()
			arguments.VmContainer = vm
			arguments.ArgsParser = NewArgumentParser()
			arguments.ForkController = forkController
			sc, err := NewSmartContractProcessor(arguments)
			require.Nil(t, err)

			contract := transaction.SmartContract{
				Type:      transaction.SmartContract_SCDeploy,
				CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
			}
			tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
			_, _ = createAccounts(tx)

			computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
			require.Nil(t, err)

			ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: tx.GetSender(),
				ContractID:     0,
				ContractType:   tx.RawData.GetContract()[0].Type,
				Block:          &block.Block{},
				TxHash:         computedHash,
				TX:             tx,
				IsScSimulation: true,
			})

			tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
			require.Nil(t, err)

			code, err := sc.DeploySmartContract(ctx, tc)
			if tt.expectedErr == "" {
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tt.expectedErr)
			}
			require.Equal(t, tt.expectedCode, code)
			require.Equal(t, tt.fixV7, ctx.GetExecutionTime() > 0)
		})
	}
}

func TestScProcessor_DeploySmartContract(t *testing.T) {
	t.Parallel()

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	_, _ = createAccounts(tx)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	returnCode, err := sc.DeploySmartContract(ctx, tc)
	require.Nil(t, err)
	require.Equal(t, vmcommon.Ok.ResultCode(), returnCode.ResultCode())
}

func TestScProcessor_ExecuteSmartContractTransactionNilAccount(t *testing.T) {
	t.Parallel()

	argParser := NewArgumentParser()
	vm := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	_, err = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.Nil(t, err)

	acntDst = nil

	_, err = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.Equal(t, process.ErrNilSCDestAccount, err)
}

func TestScProcessor_ExecuteSmartContractTransactionBadParser(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vm := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vm
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	acntDst.SetCode([]byte("code"))
	tmpError := errors.New("error")
	called := false
	argParser.ParseCallDataCalled = func(data string) (string, [][]byte, error) {
		called = true
		return "", nil, tmpError
	}

	_, err = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.True(t, called)
	require.Equal(t, tmpError, err)
}

func TestScProcessor_CheckUpgradePermission(t *testing.T) {
	t.Parallel()

	owner := []byte("owner-address")
	other := []byte("other-address")
	upgradeable := (&vmcommon.CodeMetadata{Upgradeable: true}).ToBytes()
	notUpgradeable := (&vmcommon.CodeMetadata{Upgradeable: false}).ToBytes()

	newContract := func(codeMetadata []byte) state.UserAccountHandler {
		acnt, err := state.NewUserAccount([]byte("contract-address"))
		require.Nil(t, err)
		acnt.SetCodeMetadata(codeMetadata)
		acnt.SetOwnerAddress(owner)
		return acnt
	}

	tests := []struct {
		name     string
		function string
		contract state.UserAccountHandler
		caller   []byte
		expected error
	}{
		{
			name:     "not an upgrade call",
			function: "doSomething",
			contract: nil,
			caller:   other,
			expected: nil,
		},
		{
			name:     "existing account without code metadata is not a contract",
			function: upgradeFunctionName,
			contract: newContract(nil),
			caller:   owner,
			expected: process.ErrUpgradeContractNotFound,
		},
		{
			name:     "metadata shorter than two bytes decodes as not upgradeable",
			function: upgradeFunctionName,
			contract: newContract([]byte{vmcommon.MetadataUpgradeable}),
			caller:   owner,
			expected: process.ErrUpgradeContractNotUpgradeable,
		},
		{
			name:     "metadata longer than two bytes decodes as not upgradeable",
			function: upgradeFunctionName,
			contract: newContract([]byte{vmcommon.MetadataUpgradeable, 0x00, 0x00}),
			caller:   owner,
			expected: process.ErrUpgradeContractNotUpgradeable,
		},
		{
			name:     "upgradeable contract with no owner refuses any caller",
			function: upgradeFunctionName,
			contract: func() state.UserAccountHandler {
				acnt := newContract(upgradeable)
				acnt.SetOwnerAddress(nil)
				return acnt
			}(),
			caller:   other,
			expected: process.ErrUpgradeCallerNotOwner,
		},
		{
			name:     "owner on non-upgradeable contract",
			function: upgradeFunctionName,
			contract: newContract(notUpgradeable),
			caller:   owner,
			expected: process.ErrUpgradeContractNotUpgradeable,
		},
		{
			name:     "non-owner on upgradeable contract",
			function: upgradeFunctionName,
			contract: newContract(upgradeable),
			caller:   other,
			expected: process.ErrUpgradeCallerNotOwner,
		},
		{
			name:     "non-owner on non-upgradeable contract reports not upgradeable",
			function: upgradeFunctionName,
			contract: newContract(notUpgradeable),
			caller:   other,
			expected: process.ErrUpgradeContractNotUpgradeable,
		},
		{
			name:     "owner on upgradeable contract",
			function: upgradeFunctionName,
			contract: newContract(upgradeable),
			caller:   owner,
			expected: nil,
		},
	}

	sc, err := NewSmartContractProcessor(createMockSmartContractProcessorArguments())
	require.Nil(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmInput := &vmcommon.ContractCallInput{Function: tt.function}
			vmInput.CallerAddr = tt.caller

			err := sc.checkUpgradePermission(tt.contract, vmInput)
			require.Equal(t, tt.expected, err)
		})
	}
}

// The refusal text travels through ProcessIfError to the caller unchanged, with
// the result code still VMUserError.
func TestScProcessor_ExecuteSmartContractTransactionUpgradeNotUpgradeable(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("upgradeContract@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	// the sender owns the contract, but it was deployed without the upgradeable flag
	acntDst.SetCode([]byte("code"))
	acntDst.SetCodeMetadata((&vmcommon.CodeMetadata{Upgradeable: false}).ToBytes())
	acntDst.SetOwnerAddress(tx.GetSender())
	argParser.ParseCallDataCalled = func(data string) (string, [][]byte, error) {
		return upgradeFunctionName, nil, nil
	}
	vmCalled := false
	vmContainer.GetCalled = func(key []byte) (vmcommon.VMExecutionHandler, error) {
		vmCalled = true
		return nil, errors.New("vm must not be reached")
	}

	returnCode, err := sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.False(t, vmCalled)
	require.Equal(t, vmcommon.VMUserError, returnCode)
	require.EqualError(t, err, process.ErrUpgradeContractNotUpgradeable.Error())
}

func TestScProcessor_ExecuteSmartContractTransactionVMRunError(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	acntDst.SetCode([]byte("code"))
	tmpError := errors.New("error")
	vm := &contextmock.VMExecutionHandlerStub{}
	called := false
	vm.RunSmartContractCallCalled = func(input *vmcommon.ContractCallInput) (output *vmcommon.VMOutput, e error) {
		called = true
		return nil, tmpError
	}
	vmContainer.GetCalled = func(key []byte) (handler vmcommon.VMExecutionHandler, e error) {
		return vm, nil
	}

	_, err = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.True(t, called)
	require.Equal(t, tmpError, err)
}

func TestScProcessor_ExecuteSmartContractTransaction(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	acntDst.SetCode([]byte("code"))
	_, err = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.Nil(t, err)
}

func TestScProcessor_ExecuteSmartContractTransactionSaveLogCalled(t *testing.T) {
	t.Parallel()

	slCalled := false

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	arguments.TxLogsProcessor = &contextmock.TxLogsProcessorStub{
		SaveLogCalled: func(txHash []byte, sender []byte, tc data.SmartContractHandler, contractID int, vmLogs []*vmcommon.LogEntry) error {
			slCalled = true
			return nil
		},
	}

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)

	acntDst.SetCode([]byte("code"))
	_, _ = sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
	require.True(t, slCalled)
}

func TestScProcessor_CreateVMCallInputWrongCode(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	tmpError := errors.New("error")
	argParser.ParseCallDataCalled = func(data string) (string, [][]byte, error) {
		return "", nil, tmpError
	}
	input, err := sc.createVMCallInput(createDefaultContext(tx), tx.GasLimit, []byte{}, contract.CallValue)
	require.Nil(t, input)
	require.Equal(t, tmpError, err)
}

func TestScProcessor_CreateVMCallInput(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	input, err := sc.createVMCallInput(createDefaultContext(tx), tx.GasLimit, []byte{}, contract.CallValue)
	require.NotNil(t, input)
	require.Nil(t, err)
}

func TestScProcessor_CreateVMDeployBadCode(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, nil)

	badCodeError := errors.New("fooError")
	argParser.ParseDeployDataCalled = func(data string) (*parsers.DeployArgs, error) {
		return nil, badCodeError
	}

	input, vmType, err := sc.createVMDeployInput(createDefaultContext(tx), tx.GasLimit, nil)
	require.Nil(t, vmType)
	require.Nil(t, input)
	require.Equal(t, badCodeError, err)
}

func TestScProcessor_CreateVMDeployInput(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("foobar")})

	expectedVMType := []byte{5, 6}
	expectedCodeMetadata := vmcommon.CodeMetadata{Upgradeable: true}
	argParser.ParseDeployDataCalled = func(data string) (*parsers.DeployArgs, error) {
		return &parsers.DeployArgs{
			Code:         []byte("code"),
			VMType:       expectedVMType,
			CodeMetadata: expectedCodeMetadata,
			Arguments:    nil,
		}, nil
	}

	input, vmType, err := sc.createVMDeployInput(createDefaultContext(tx), tx.GasLimit, contract.CallValue)
	require.NotNil(t, input)
	require.Nil(t, err)
	require.Equal(t, vmData.DirectCall, input.CallType)
	require.True(t, bytes.Equal(expectedVMType, vmType))
	require.Equal(t, expectedCodeMetadata.ToBytes(), input.ContractCodeMetadata)
	require.Nil(t, err)
}

func TestScProcessor_CreateVMDeployInputNotEnoughArguments(t *testing.T) {
	t.Parallel()

	argParser := NewArgumentParser()
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data@0000")})

	input, vmType, err := sc.createVMDeployInput(createDefaultContext(tx), tx.GasLimit, contract.CallValue)
	require.Nil(t, input)
	require.Nil(t, vmType)
	require.Equal(t, parsers.ErrInvalidDeployArguments, err)
}

func TestScProcessor_CreateVMDeployInputWrongArgument(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	tmpError := errors.New("fooError")
	argParser.ParseDeployDataCalled = func(data string) (*parsers.DeployArgs, error) {
		return nil, tmpError
	}
	input, vmType, err := sc.createVMDeployInput(createDefaultContext(tx), tx.GasLimit, contract.CallValue)
	require.Nil(t, input)
	require.Nil(t, vmType)
	require.Equal(t, tmpError, err)
}

func TestScProcessor_InitializeVMInputFromTx(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	vmInput := &vmcommon.VMInput{}
	err = sc.initializeVMInputFromTx(tx.GetSender(), vmInput, contract.CallValue)
	require.Nil(t, err)
}

func TestScProcessor_processVMOutputNilSndAcc(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, nil, 0, nil)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	vmOutput := &vmcommon.VMOutput{
		GasRemaining: 0,
	}

	err = sc.processVMOutput(ctx, vmOutput)
	require.Nil(t, err)
}

func TestScProcessor_processVMOutputNilDstAcc(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, nil)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	vmOutput := &vmcommon.VMOutput{
		GasRemaining: 0,
	}

	err = sc.processVMOutput(ctx, vmOutput)
	require.Nil(t, err)
}

func TestScProcessor_ProcessSCPaymentNotEnoughBalance(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	currBalance := int64(10)
	acntSrc, _ := initAccounts(tx, arguments.AccountsCacher, currBalance)

	err = sc.processSCPayment(tc, acntSrc)
	require.Equal(t, "result code: 1, insufficient funds", err.Error())
	require.Equal(t, currBalance, acntSrc.GetBalance(nil, true))
}

func newNFTAccountsKappForCallValue(
	t *testing.T,
	forkController core.ForkController,
	sender, recipient, assetID, internalID []byte,
	subInternalCalled, addInternalCalled *bool,
) kapp.AccountsKapp {
	t.Helper()

	nftRecord := commommock.NFTHolderRecord(1)
	take := func(gotAssetID, gotInternalID []byte) ([]byte, error) {
		assert.Equal(t, assetID, gotAssetID)
		assert.Equal(t, internalID, gotInternalID)
		*subInternalCalled = true
		return nftRecord, nil
	}

	src := &commommock.UserAccountHandlerStub{
		AddressBytesCalled:   func() []byte { return sender },
		SubInternalKDACalled: take,
		SubFromBalanceWithNonceCalled: func(value int64, _, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
			t.Fatalf("balance path called for true NFT with value %d", value)
			return nil
		},
	}
	dst := &commommock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return recipient },
		AddInternalKDACalled: func(gotAssetID, gotInternalID, data []byte) error {
			assert.Equal(t, assetID, gotAssetID)
			assert.Equal(t, internalID, gotInternalID)
			assert.Equal(t, nftRecord, data)
			*addInternalCalled = true
			return nil
		},
		AddToBalanceWithNonceCalled: func(value int64, _, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
			t.Fatalf("balance path called for true NFT with value %d", value)
			return nil
		},
	}

	accKapp, err := accountsKapp.NewAccountKApp(&accountsKapp.ArgsNewAccountKApp{
		Marshalizer:    &commommock.ProtoMarshalizerMock{},
		PubkeyConv:     commommock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	})
	require.NoError(t, err)

	require.NoError(t, accKapp.SetAccountsCacher(&commommock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			switch {
			case bytes.Equal(address, sender):
				return src, nil
			case bytes.Equal(address, recipient):
				return dst, nil
			default:
				return nil, fmt.Errorf("unexpected account load %x", address)
			}
		},
		UpdateUserCalled: func(state.AccountHandler) error { return nil },
	}))
	require.NoError(t, accKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: sender,
				ContractID:     0,
				ContractType:   transaction.TXContract_SmartContractType,
				Block:          &block.Block{},
				TX:             &transaction.Transaction{GasLimit: 1_000_000},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(gotAssetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					assert.Equal(t, assetID, gotAssetID)
					return nil, &kapps.KDAData{
						AssetType:  kapps.KDAData_NonFungible,
						Attributes: &kapps.AttributesData{},
						Properties: &kapps.PropertiesData{},
						Royalties:  &kapps.RoyaltiesData{},
					}, nil
				},
			}
		},
	}))

	return accKapp
}

// newNFTForkController returns a fork controller with FixAuditChangesV3 toggled on or off.
func newNFTForkController(t *testing.T, fixV3On bool) core.ForkController {
	t.Helper()
	cfg := config.EnableEpochs{}
	if !fixV3On {
		cfg.FixAuditChangesV3 = 1000 // not yet active at epoch 0
	}
	forkController, err := fork.NewForkController(cfg, &commommock.EpochNotifierStub{})
	require.NoError(t, err)
	return forkController
}

func TestScProcessor_ProcessSCPayment_NonCanonicalNFTAmount(t *testing.T) {
	sender := bytes.Repeat([]byte{0x11}, 32)
	recipient := bytes.Repeat([]byte{0x22}, 32)
	assetID := []byte("NFT-1234")
	internalID := []byte("7")
	const inflated = int64(1_000_000)

	tests := []struct {
		description string
		fixV3On     bool
	}{
		{description: "fork on: non-canonical call value is rejected", fixV3On: true},
		{description: "fork off: legacy inflated call value accepted", fixV3On: false},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var subInternalCalled, addInternalCalled bool
			forkController := newNFTForkController(t, tt.fixV3On)
			accKapp := newNFTAccountsKappForCallValue(t, forkController, sender, recipient, assetID, internalID,
				&subInternalCalled, &addInternalCalled)

			sc := &scProcessor{
				blockChainHook: &contextmock.BlockchainHookStub{
					GetKAppControllerCalled: func() kapp.KAppController {
						return &kvmStub.KAppControllerStub{
							GetAccountsKAppCalled: func() kapp.AccountsKapp { return accKapp },
						}
					},
				},
				forkController: forkController,
			}

			tc := &transaction.SmartContract{
				Type:    transaction.SmartContract_SCInvoke,
				Address: recipient,
				CallValue: map[string]*transaction.CallValue{
					"NFT-1234/7": {Amount: inflated},
				},
			}
			acntSnd := &commommock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return sender },
			}

			err := sc.processSCPayment(tc, acntSnd)

			if tt.fixV3On {
				// Post-fork: the non-canonical NFT amount is rejected and the
				// internal KDA move never happens.
				require.Error(t, err)
				require.Contains(t, err.Error(), common.ErrInvalidValue.Error())
				require.False(t, subInternalCalled, "internal KDA path (sub) must not run on rejection")
				require.False(t, addInternalCalled, "internal KDA path (add) must not run on rejection")
				return
			}

			// Pre-fork: legacy behaviour moves the NFT and preserves the inflated call value.
			require.NoError(t, err)
			require.True(t, subInternalCalled && addInternalCalled, "true NFT was not moved through the internal path")
			assert.Equal(t, inflated, tc.GetCallValue()["NFT-1234/7"].Amount)
		})
	}
}

func TestScProcessor_PrepareExecution_NonCanonicalNFTCallValue(t *testing.T) {
	sender := bytes.Repeat([]byte{0x11}, 32)
	recipient := bytes.Repeat([]byte{0x22}, 32)
	assetID := []byte("NFT-1234")
	internalID := []byte("7")
	const inflated = int64(1_000_000)

	tests := []struct {
		description string
		fixV3On     bool
	}{
		{description: "fork on: non-canonical call value rejected before VM input", fixV3On: true},
		{description: "fork off: VM input exposes inflated value", fixV3On: false},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var subInternalCalled, addInternalCalled bool
			forkController := newNFTForkController(t, tt.fixV3On)
			accKapp := newNFTAccountsKappForCallValue(t, forkController, sender, recipient, assetID, internalID,
				&subInternalCalled, &addInternalCalled)

			sc := &scProcessor{
				blockChainHook: &contextmock.BlockchainHookStub{
					GetKAppControllerCalled: func() kapp.KAppController {
						return &kvmStub.KAppControllerStub{
							GetAccountsKAppCalled: func() kapp.AccountsKapp { return accKapp },
						}
					},
				},
				argsParser: &contextmock.ArgumentParserMock{
					ParseCallDataCalled: func(data string) (string, [][]byte, error) {
						return "credit", [][]byte{[]byte("attacker")}, nil
					},
				},
				builtInGasCosts: map[string]uint64{transferValueGasSchedule: 0},
				forkController:  forkController,
			}

			tx := &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{
					Sender: sender,
					Data:   [][]byte{[]byte("credit@attacker")},
				},
				GasLimit: 1_000_000,
			}
			ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: sender,
				ContractID:     0,
				ContractType:   transaction.TXContract_SmartContractType,
				Block:          &block.Block{},
				TX:             tx,
			})
			tc := &transaction.SmartContract{
				Type:    transaction.SmartContract_SCInvoke,
				Address: recipient,
				CallValue: map[string]*transaction.CallValue{
					"NFT-1234/7": {Amount: inflated},
				},
			}

			returnCode, vmInput, err := sc.prepareExecution(
				ctx,
				tc,
				&commommock.UserAccountHandlerStub{AddressBytesCalled: func() []byte { return sender }},
				&commommock.UserAccountHandlerStub{},
			)

			if tt.fixV3On {
				// Post-fork: preparation fails on the non-canonical NFT call value,
				// so no VM input is produced and the internal move never happens.
				require.Error(t, err)
				require.Contains(t, err.Error(), common.ErrInvalidValue.Error())
				require.Equal(t, vmcommon.VMContractInvalid, returnCode)
				require.Nil(t, vmInput)
				require.False(t, subInternalCalled, "internal KDA path (sub) must not run on rejection")
				require.False(t, addInternalCalled, "internal KDA path (add) must not run on rejection")
				return
			}

			// Pre-fork: preparation succeeds and the VM input exposes the inflated value.
			require.NoError(t, err)
			require.Equal(t, vmcommon.Ok, returnCode)
			require.True(t, subInternalCalled && addInternalCalled, "true NFT was not moved through the internal path")
			require.NotNil(t, vmInput)
			require.Len(t, vmInput.KDATransfers, 1)

			transfer := vmInput.KDATransfers[0]
			assert.Equal(t, 0, transfer.KDAValue.Cmp(big.NewInt(inflated)), "got KDAValue %s want %d", transfer.KDAValue, inflated)
			assert.Equal(t, uint64(7), transfer.KDATokenNonce)
			assert.Equal(t, uint32(core.NonFungible), transfer.KDATokenType)
			assert.True(t, transfer.IsExecuted(), "VM input transfer should be marked executed after SC payment")
		})
	}
}

func TestScProcessor_ProcessSCPayment(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
		Address:   core.ZeroAddress,
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	require.Nil(t, err)

	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		GetKAppControllerCalled: func() kapp.KAppController {
			kappArgs := kappcontroller.ArgsNewKApp{
				Hasher:         arguments.Hasher,
				Marshalizer:    arguments.Marshalizer,
				PubkeyConv:     arguments.PubkeyConv,
				ForkController: arguments.ForkController,
				AccountsCacher: arguments.AccountsCacher,
				RatingsData:    &commommock.RatingsInfoMock{},
			}

			kappCtrl, err := kappcontroller.NewKappController(kappArgs)
			require.Nil(t, err)
			_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
			_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
			_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)

			kappCtrl.SetCurrentKAppContext(ctx)

			return kappCtrl
		},
	}

	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	currBalance := int64(100)
	acntSrc, _ := initAccounts(tx, arguments.AccountsCacher, currBalance)

	modifiedBalance := currBalance - contract.CallValue["KLV"].Amount - int64(tx.GasLimit)*int64(tx.GasLimit)

	err = sc.processSCPayment(tc, acntSrc)
	require.Nil(t, err)
	require.Equal(t, modifiedBalance, acntSrc.GetBalance(nil, true))
}

func TestScProcessor_processVMOutput(t *testing.T) {
	t.Parallel()

	argParser := &contextmock.ArgumentParserMock{}
	vmContainer := &contextmock.VMContainerMock{}
	arguments := createMockSmartContractProcessorArguments()
	arguments.VmContainer = vmContainer
	arguments.ArgsParser = argParser
	sc, err := NewSmartContractProcessor(arguments)
	require.NotNil(t, sc)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		Address:   core.ZeroAddress,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, nil)

	computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
	require.Nil(t, err)

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     0,
		ContractType:   tx.RawData.GetContract()[0].Type,
		Block:          &block.Block{},
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	vmOutput := &vmcommon.VMOutput{
		GasRemaining: 0,
	}

	err = sc.processVMOutput(ctx, vmOutput)
	require.Nil(t, err)
}

func TestScProcessor_DeleteAccounts(t *testing.T) {
	t.Parallel()

	addrOk := "addr1"
	addrNotFound := "addr2"
	addrNoCode := "addr3"
	addrMult1 := "addr4"
	addrMult2 := "addr5"
	mockCodes := map[string][]byte{
		addrOk:    []byte("code"),
		addrMult1: []byte("code"),
		addrMult2: []byte("code"),
	}

	makeMock := func(addr string) state.UserAccountHandler {
		acc, err := state.NewUserAccount([]byte(addr))
		require.Nil(t, err)

		if code, ok := mockCodes[addr]; ok {
			acc.SetCode(code)
		}

		return acc
	}

	mockAccounts := map[string]state.UserAccountHandler{
		addrOk:     makeMock(addrOk),
		addrMult1:  makeMock(addrMult1),
		addrMult2:  makeMock(addrMult2),
		addrNoCode: makeMock(addrNoCode),
	}

	tests := []struct {
		name        string
		deletedAccs [][]byte
		expectedErr error
	}{
		{
			name:        "fail account not found",
			deletedAccs: [][]byte{[]byte(addrNotFound)},
			expectedErr: common.ErrAccNotFound,
		},
		{
			name:        "no code account should success",
			deletedAccs: [][]byte{[]byte(addrNoCode)},
			expectedErr: nil,
		},
		{
			name:        "success",
			deletedAccs: [][]byte{[]byte(addrOk)},
			expectedErr: nil,
		},
		{
			name:        "success multiple accounts",
			deletedAccs: [][]byte{[]byte(addrMult1), []byte(addrMult2)},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := createMockSmartContractProcessorArguments()
			sc, _ := NewSmartContractProcessor(args)

			// load accounts
			for _, account := range mockAccounts {
				err := args.AccountsCacher.SaveUser(account)
				require.Nil(t, err)
			}

			err := sc.deleteAccounts(tt.deletedAccs)
			assert.Equal(t, tt.expectedErr, err)

			// check if account code is removed
			if err == nil {
				for _, addr := range tt.deletedAccs {
					acc, err := args.AccountsCacher.GetExistingUser(addr)
					require.Nil(t, err)
					assert.Len(t, acc.GetCodeHash(), 0)
				}
			}

		})
	}
}

// sharedCodeProcessor drives deploy and delete VM outputs through processVMOutput on a
// real trie-backed accounts cacher, one transaction at a time, and reads the reference
// count of the code entry shared by every contract deployed from the same bytecode.
type sharedCodeProcessor struct {
	t           *testing.T
	sc          *scProcessor
	adb         *state.AccountsDB
	trieGet     func([]byte) ([]byte, error)
	cacher      state.AccountsCacher
	marshalizer marshal.Marshalizer
	code        []byte
	codeHash    []byte
	metadata    []byte
}

func newSharedCodeProcessor(t *testing.T, fixV6 bool) *sharedCodeProcessor {
	hasher := &sha256.Sha256{}
	marshalizer := marshal.NewProtoMarshalizer()
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	require.Nil(t, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hasher, 5)
	require.Nil(t, err)
	adb, err := state.NewAccountsDB(tr, hasher, marshalizer, factory.NewAccountCreator(), core.Normal)
	require.Nil(t, err)
	cacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{Accounts: adb, Kapps: adb, Peers: adb})
	require.Nil(t, err)

	args := createMockSmartContractProcessorArguments()
	args.AccountsCacher = cacher
	args.ForkController = commommock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixV6)
	args.BlockChainHook = &contextmock.BlockchainHookStub{
		FilterCodeMetadataForUpgradeCalled: func(input []byte) ([]byte, error) {
			return input, nil
		},
	}
	sc, err := NewSmartContractProcessor(args)
	require.Nil(t, err)

	code := []byte("shared contract bytecode")
	return &sharedCodeProcessor{
		t:           t,
		sc:          sc,
		adb:         adb,
		trieGet:     tr.Get,
		cacher:      cacher,
		marshalizer: marshalizer,
		code:        code,
		codeHash:    hasher.Compute(string(code)),
		metadata:    []byte{vmcommon.MetadataUpgradeable, 0},
	}
}

func sharedCodeSCAddress(id byte) []byte {
	address := make([]byte, 32)
	copy(address[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	address[31] = id
	return address
}

func (p *sharedCodeProcessor) processTx(vmOutput *vmcommon.VMOutput) {
	p.cacher.ResetAll(true)
	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{Block: &block.Block{}, IsScSimulation: true})
	require.Nil(p.t, p.sc.processVMOutput(ctx, vmOutput))
	require.Nil(p.t, p.cacher.SaveAll())
}

func (p *sharedCodeProcessor) deploy(address, owner []byte) {
	p.processTx(&vmcommon.VMOutput{
		OutputAccounts: map[string]*vmcommon.OutputAccount{
			string(address): {
				Address:             address,
				Code:                p.code,
				CodeMetadata:        p.metadata,
				CodeDeployerAddress: owner,
			},
		},
	})
}

func (p *sharedCodeProcessor) delete(deleted ...[]byte) {
	p.processTx(&vmcommon.VMOutput{DeletedAccounts: deleted})
}

// references returns NumReferences of the shared code entry, or 0 once it is removed.
func (p *sharedCodeProcessor) references() uint32 {
	raw, err := p.trieGet(p.codeHash)
	require.Nil(p.t, err)
	if len(raw) == 0 {
		return 0
	}

	entry := &state.CodeEntry{}
	require.Nil(p.t, p.marshalizer.Unmarshal(entry, raw))
	return entry.NumReferences
}

func (p *sharedCodeProcessor) codeLoadable(address []byte) bool {
	p.cacher.ResetAll(true)
	acc, err := p.cacher.GetExistingUser(address)
	require.Nil(p.t, err)
	return len(p.cacher.GetCode(acc.GetCodeHash())) != 0
}

func TestScProcessor_DeleteAccountsSharedCode(t *testing.T) {
	t.Parallel()

	first := sharedCodeSCAddress(1)
	second := sharedCodeSCAddress(2)
	firstOwner := []byte("first-owner-address-00000000000")
	secondOwner := []byte("second-owner-address-0000000000")

	newDeployed := func(t *testing.T, fixV6 bool) *sharedCodeProcessor {
		p := newSharedCodeProcessor(t, fixV6)
		p.deploy(first, firstOwner)
		p.deploy(second, secondOwner)
		require.Equal(t, uint32(2), p.references())
		return p
	}

	t.Run("single delete decrements the shared entry once", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		p.delete(second)

		require.Equal(t, uint32(1), p.references())
		require.True(t, p.codeLoadable(first))
		require.False(t, p.codeLoadable(second))
	})

	t.Run("address listed more than once decrements once", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		p.delete(second, second, second)

		require.Equal(t, uint32(1), p.references())
		require.True(t, p.codeLoadable(first))
	})

	t.Run("deleting again in later transactions has no effect", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		for i := 0; i < 3; i++ {
			p.delete(second)
		}

		require.Equal(t, uint32(1), p.references())
		require.True(t, p.codeLoadable(first))
	})

	t.Run("repeated deploy and delete keeps the count", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		for i := byte(0); i < 5; i++ {
			other := sharedCodeSCAddress(10 + i)
			p.deploy(other, secondOwner)
			p.delete(other)
			require.Equal(t, uint32(2), p.references())
		}
		require.True(t, p.codeLoadable(first))
	})

	t.Run("revert restores the shared entry", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		snapshot := p.adb.JournalLen()
		p.delete(second)
		require.Equal(t, uint32(1), p.references())

		require.Nil(t, p.adb.RevertToSnapshot(snapshot))

		require.Equal(t, uint32(2), p.references())
		require.True(t, p.codeLoadable(second))
	})

	t.Run("deleting the last reference removes the entry", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, true)
		p.delete(second)
		p.delete(first)

		require.Equal(t, uint32(0), p.references())
		require.False(t, p.codeLoadable(first))
	})

	// Below FixAuditChangesV6 the legacy accounting is reproduced so that pre-fork
	// blocks replay to the same state.
	t.Run("before FixAuditChangesV6 the legacy accounting is kept", func(t *testing.T) {
		t.Parallel()

		p := newDeployed(t, false)
		p.delete(second)

		require.Equal(t, uint32(0), p.references())
	})
}

// TestSetVMExecutionMode tests setting the execution mode
func TestSetVMExecutionMode(t *testing.T) {
	t.Parallel()

	t.Run("successfully calls SetVMExecutionMode without panic", func(t *testing.T) {
		args := createMockSmartContractProcessorArguments()
		sc, err := NewSmartContractProcessor(args)
		require.Nil(t, err)

		// Test setting different modes - just verify no panic
		sc.SetVMExecutionMode(vmcommon.ExecutionModeValidator)
		sc.SetVMExecutionMode(vmcommon.ExecutionModeLeader)
		sc.SetVMExecutionMode(vmcommon.ExecutionModeQuery)
	})
}

// TestGetVMExecutionMode tests getting the execution mode
func TestGetVMExecutionMode(t *testing.T) {
	t.Parallel()

	t.Run("gets execution mode without panic", func(t *testing.T) {
		args := createMockSmartContractProcessorArguments()
		sc, err := NewSmartContractProcessor(args)
		require.Nil(t, err)

		mode := sc.GetVMExecutionMode()
		// Should return some valid mode (just verify no panic and valid mode returned)
		assert.True(t, mode >= vmcommon.ExecutionModeLeader && mode <= vmcommon.ExecutionModeQuery)
	})
}

func TestIsProtectedStorageKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		key  []byte
		want bool
	}{
		{"kda balance key", []byte("KDA/FAKE"), true},
		{"klv key", []byte("KLV/anything"), true},
		{"kfi key", []byte("KFI/anything"), true},
		{"bare klever key", []byte("KLEVERanything"), true},
		{"vm internal key", []byte("KLEVERVM@box"), false},
		{"regular contract key", []byte("AAA/FAKE"), false},
		{"empty key", []byte(""), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isProtectedStorageKey(tc.key))
		})
	}
}

func TestScProcessor_ProcessSCOutputAccountsRejectsProtectedKeyWrite(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, err := NewSmartContractProcessor(arguments)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 0}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})
	ctx := createDefaultContext(tx)

	address := []byte("output-account-address-32-bytes!")
	buildAccounts := func(offset []byte, written bool) []*vmcommon.OutputAccount {
		return []*vmcommon.OutputAccount{
			{
				Address: address,
				StorageUpdates: map[string]*vmcommon.StorageUpdate{
					string(offset): {Offset: offset, Data: []byte("forged"), Written: written},
				},
			},
		}
	}

	t.Run("written protected kda key is rejected", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts([]byte("KDA/FAKE"), true))
		require.Equal(t, process.ErrStoreProtectedKey, err)
	})

	t.Run("read-only protected kda key is not rejected", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts([]byte("KDA/FAKE"), false))
		require.NoError(t, err)
	})

	t.Run("regular contract key is persisted", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts([]byte("AAA/FAKE"), true))
		require.NoError(t, err)
	})

	t.Run("vm internal key is persisted", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts([]byte(kapps.ProtectedKleverKeyPrefix+"VM@box"), true))
		require.NoError(t, err)
	})
}

func TestScProcessor_ProcessSCOutputAccountsRejectsOverLimitStorageKeyWrite(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	sc, err := NewSmartContractProcessor(arguments)
	require.Nil(t, err)

	preForkArguments := createMockSmartContractProcessorArguments()
	preForkArguments.ForkController, err = fork.NewForkController(config.EnableEpochs{FixAuditChangesV6: 100}, &commommock.EpochNotifierStub{})
	require.Nil(t, err)
	preForkSc, err := NewSmartContractProcessor(preForkArguments)
	require.Nil(t, err)

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 0}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})
	ctx := createDefaultContext(tx)

	address := []byte("output-account-address-32-bytes!")
	buildAccounts := func(offset []byte, data []byte, written bool) []*vmcommon.OutputAccount {
		return []*vmcommon.OutputAccount{
			{
				Address: address,
				StorageUpdates: map[string]*vmcommon.StorageUpdate{
					string(offset): {Offset: offset, Data: data, Written: written},
				},
			},
		}
	}

	isPersisted := func(processor *scProcessor, key []byte) bool {
		acc, errLoad := processor.getAccountFromAddress(address)
		require.Nil(t, errLoad)
		_, found := acc.DataTrieTracker().DirtyData()[string(key)]
		return found
	}

	keyAtLimit := bytes.Repeat([]byte("a"), common.MaxStorageKeyLength)
	keyOverLimit := bytes.Repeat([]byte("b"), common.MaxStorageKeyLength+1)
	readOnlyKeyOverLimit := bytes.Repeat([]byte("c"), common.MaxStorageKeyLength+1)
	readOnlyKey := []byte("regular")

	t.Run("written over-limit key is rejected", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(keyOverLimit, []byte("data"), true))
		require.Equal(t, process.ErrStorageKeyTooLong, err)
		require.False(t, isPersisted(sc, keyOverLimit))
	})

	t.Run("read-only over-limit key is not persisted", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(readOnlyKeyOverLimit, []byte("data"), false))
		require.NoError(t, err)
		require.False(t, isPersisted(sc, readOnlyKeyOverLimit))
	})

	t.Run("read-only regular key is not re-saved", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(readOnlyKey, []byte("data"), false))
		require.NoError(t, err)
		require.False(t, isPersisted(sc, readOnlyKey))
	})

	t.Run("deleted over-limit key is not rejected", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(keyOverLimit, nil, true))
		require.NoError(t, err)
		require.True(t, isPersisted(sc, keyOverLimit))
	})

	t.Run("key at the limit is persisted", func(t *testing.T) {
		err := sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(keyAtLimit, []byte("data"), true))
		require.NoError(t, err)
		require.True(t, isPersisted(sc, keyAtLimit))
	})

	t.Run("over-limit key is persisted before FixAuditChangesV6", func(t *testing.T) {
		err := preForkSc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(keyOverLimit, []byte("data"), true))
		require.NoError(t, err)
		require.True(t, isPersisted(preForkSc, keyOverLimit))
	})

	t.Run("read-only key is re-saved before FixAuditChangesV6", func(t *testing.T) {
		err := preForkSc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, buildAccounts(readOnlyKeyOverLimit, []byte("data"), false))
		require.NoError(t, err)
		require.True(t, isPersisted(preForkSc, readOnlyKeyOverLimit))
	})
}

// TestScProcessor_SaveStorageUpdatesFirstFailureInKeyOrder pins which guard rejects an output account carrying
// two bad updates: from FixAuditChangesV7 the updates are visited in key order, so the error is the same on
// every run instead of following Go's random map order (KLC-2770).
func TestScProcessor_SaveStorageUpdatesFirstFailureInKeyOrder(t *testing.T) {
	t.Parallel()

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCDeploy,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 0}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})
	ctx := createDefaultContext(tx)

	// "KDA/..." sorts before the over-limit "a..." key, so the protected-key guard must fire first
	protectedKey := []byte("KDA/FAKE")
	keyOverLimit := bytes.Repeat([]byte("a"), common.MaxStorageKeyLength+1)
	outputAccounts := []*vmcommon.OutputAccount{
		{
			Address: []byte("output-account-address-32-bytes!"),
			StorageUpdates: map[string]*vmcommon.StorageUpdate{
				string(keyOverLimit): {Offset: keyOverLimit, Data: []byte("data"), Written: true},
				string(protectedKey): {Offset: protectedKey, Data: []byte("forged"), Written: true},
			},
		},
	}

	t.Run("first failing key in order wins from FixAuditChangesV7", func(t *testing.T) {
		sc, err := NewSmartContractProcessor(createMockSmartContractProcessorArguments())
		require.Nil(t, err)
		require.True(t, sc.forkController.FixAuditChangesV7())

		for i := 0; i < 100; i++ {
			err = sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, outputAccounts)
			require.Equal(t, process.ErrStoreProtectedKey, err, "run %d", i)
		}
	})

	t.Run("either guard may fire before FixAuditChangesV7", func(t *testing.T) {
		arguments := createMockSmartContractProcessorArguments()
		forkController, err := fork.NewForkController(config.EnableEpochs{FixAuditChangesV7: 100}, &commommock.EpochNotifierStub{})
		require.Nil(t, err)
		arguments.ForkController = forkController
		sc, err := NewSmartContractProcessor(arguments)
		require.Nil(t, err)

		seen := make(map[error]bool)
		for i := 0; i < 100; i++ {
			err = sc.processSCOutputAccounts(ctx, &vmcommon.VMOutput{}, outputAccounts)
			require.True(t, errors.Is(err, process.ErrStoreProtectedKey) || errors.Is(err, process.ErrStorageKeyTooLong), "unexpected error %v", err)
			seen[err] = true
		}
		require.Len(t, seen, 2, "pre-fork order must stay Go's map order")
	})
}

// TestScProcessor_PrintScDeployedOnlyWrittenCodeInAddressOrder pins the contracts a deploy records an SCTrigger
// receipt for: from FixAuditChangesV7 only the contracts whose code it writes, in address order on every node;
// before, every contract account in the output, in map order (KLC-2770).
func TestScProcessor_PrintScDeployedOnlyWrittenCodeInAddressOrder(t *testing.T) {
	t.Parallel()

	scAddress := func(last byte) []byte {
		addr := make([]byte, 32)
		copy(addr[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
		addr[31] = last
		return addr
	}
	deployedLow, deployedHigh, touched := scAddress(0x01), scAddress(0xf0), scAddress(0x80)
	user := bytes.Repeat([]byte{0x42}, 32)
	vmOutput := &vmcommon.VMOutput{OutputAccounts: map[string]*vmcommon.OutputAccount{
		string(deployedHigh): {Address: deployedHigh, Code: []byte("code")},
		string(touched):      {Address: touched},
		string(deployedLow):  {Address: deployedLow, Code: []byte("code")},
		string(user):         {Address: user},
	}}

	t.Run("written code only, in address order, from FixAuditChangesV7", func(t *testing.T) {
		sc, err := NewSmartContractProcessor(createMockSmartContractProcessorArguments())
		require.Nil(t, err)
		require.True(t, sc.forkController.FixAuditChangesV7())

		for i := 0; i < 50; i++ {
			require.Equal(t, [][]byte{deployedLow, deployedHigh}, sc.printScDeployed([]byte("sender"), vmOutput), "run %d", i)
		}
	})

	t.Run("every contract account, in map order, before FixAuditChangesV7", func(t *testing.T) {
		arguments := createMockSmartContractProcessorArguments()
		forkController, err := fork.NewForkController(config.EnableEpochs{FixAuditChangesV7: 100}, &commommock.EpochNotifierStub{})
		require.Nil(t, err)
		arguments.ForkController = forkController
		sc, err := NewSmartContractProcessor(arguments)
		require.Nil(t, err)

		sorted := [][]byte{deployedLow, touched, deployedHigh}
		unsorted := false
		for i := 0; i < 50; i++ {
			deployed := sc.printScDeployed([]byte("sender"), vmOutput)
			require.ElementsMatch(t, sorted, deployed)
			unsorted = unsorted || !assert.ObjectsAreEqual(sorted, deployed)
		}
		require.True(t, unsorted, "pre-fork order must stay Go's map order")
	})
}

func newOutputAccountAddressProcessor(t *testing.T, fixV6 bool) (*scProcessor, ArgsNewSmartContractProcessor) {
	arguments := createMockSmartContractProcessorArguments()
	if !fixV6 {
		forkController, err := fork.NewForkController(config.EnableEpochs{FixAuditChangesV6: 100}, &commommock.EpochNotifierStub{})
		require.Nil(t, err)
		arguments.ForkController = forkController
	}
	sc, err := NewSmartContractProcessor(arguments)
	require.Nil(t, err)

	return sc, arguments
}

// userAccountExists reports whether a user account is held under address, first in the
// accounts cacher and then, once flushed and reset, in the accounts trie
func userAccountExists(t *testing.T, cacher state.AccountsCacher, address []byte) (bool, bool) {
	_, errCached := cacher.GetExistingUser(address)
	require.Nil(t, cacher.SaveAll())
	cacher.ResetAll(true)
	_, errStored := cacher.GetExistingUser(address)

	return errCached == nil, errStored == nil
}

func TestScProcessor_ProcessSCOutputAccountsRejectsInvalidAddressLength(t *testing.T) {
	t.Parallel()

	contract := transaction.SmartContract{
		Type:      transaction.SmartContract_SCInvoke,
		CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 0}},
	}
	tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("data")})

	buildAccounts := func(address []byte) []*vmcommon.OutputAccount {
		return []*vmcommon.OutputAccount{
			{
				Address: address,
				StorageUpdates: map[string]*vmcommon.StorageUpdate{
					"key": {Offset: []byte("key"), Data: []byte("data"), Written: true},
				},
			},
		}
	}

	for _, addressLen := range []int{31, 33} {
		address := bytes.Repeat([]byte{0x01}, addressLen)

		t.Run(fmt.Sprintf("%d-byte account is rejected after FixAuditChangesV6", addressLen), func(t *testing.T) {
			t.Parallel()

			sc, arguments := newOutputAccountAddressProcessor(t, true)
			err := sc.processSCOutputAccounts(createDefaultContext(tx), &vmcommon.VMOutput{}, buildAccounts(address))
			require.ErrorIs(t, err, process.ErrInvalidOutputAccountAddressLength)

			cached, stored := userAccountExists(t, arguments.AccountsCacher, address)
			require.False(t, cached)
			require.False(t, stored)
		})

		t.Run(fmt.Sprintf("%d-byte account is persisted before FixAuditChangesV6", addressLen), func(t *testing.T) {
			t.Parallel()

			sc, arguments := newOutputAccountAddressProcessor(t, false)
			err := sc.processSCOutputAccounts(createDefaultContext(tx), &vmcommon.VMOutput{}, buildAccounts(address))
			require.NoError(t, err)

			cached, stored := userAccountExists(t, arguments.AccountsCacher, address)
			require.True(t, cached)
			require.True(t, stored)
		})
	}

	for _, fixV6 := range []bool{true, false} {
		t.Run(fmt.Sprintf("32-byte account is persisted (FixAuditChangesV6=%v)", fixV6), func(t *testing.T) {
			t.Parallel()

			address := bytes.Repeat([]byte{0x01}, 32)
			sc, arguments := newOutputAccountAddressProcessor(t, fixV6)
			err := sc.processSCOutputAccounts(createDefaultContext(tx), &vmcommon.VMOutput{}, buildAccounts(address))
			require.NoError(t, err)

			cached, stored := userAccountExists(t, arguments.AccountsCacher, address)
			require.True(t, cached)
			require.True(t, stored)
		})
	}
}

func TestScProcessor_ExecuteSmartContractTransactionInvalidOutputAccountAddressLength(t *testing.T) {
	t.Parallel()

	const gasLimit = uint64(1000)
	const gasRemaining = uint64(400)

	execute := func(t *testing.T, fixV6 bool, isSimulation bool, outputAddress []byte) (vmcommon.ReturnCode, kapp.KappContext, state.AccountsCacher, error) {
		_, arguments := newOutputAccountAddressProcessor(t, fixV6)
		arguments.ArgsParser = &contextmock.ArgumentParserMock{}
		arguments.VmContainer = &contextmock.VMContainerMock{
			GetCalled: func(key []byte) (vmcommon.VMExecutionHandler, error) {
				return &contextmock.VMExecutionHandlerStub{
					RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
						return &vmcommon.VMOutput{
							ReturnCode:   vmcommon.Ok,
							GasRemaining: gasRemaining,
							OutputAccounts: map[string]*vmcommon.OutputAccount{
								string(outputAddress): {Address: outputAddress},
							},
						}, nil
					},
				}, nil
			},
		}

		contract := transaction.SmartContract{
			Type:      transaction.SmartContract_SCInvoke,
			CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
			Address:   core.ZeroAddress,
		}
		tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
		tx.GasLimit = gasLimit

		computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
		require.Nil(t, err)

		ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
			OriginalSender: tx.GetSender(),
			ContractID:     0,
			ContractType:   tx.RawData.GetContract()[0].Type,
			Block:          &block.Block{},
			TxHash:         computedHash,
			TX:             tx,
			IsScSimulation: isSimulation,
		})

		tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
		require.Nil(t, err)

		arguments.BlockChainHook = &contextmock.BlockchainHookStub{
			GetKAppControllerCalled: func() kapp.KAppController {
				kappCtrl, errCtrl := kappcontroller.NewKappController(kappcontroller.ArgsNewKApp{
					Hasher:         arguments.Hasher,
					Marshalizer:    arguments.Marshalizer,
					PubkeyConv:     arguments.PubkeyConv,
					ForkController: arguments.ForkController,
					AccountsCacher: arguments.AccountsCacher,
					RatingsData:    &commommock.RatingsInfoMock{},
				})
				require.Nil(t, errCtrl)
				_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
				_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
				_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
				_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)
				kappCtrl.SetCurrentKAppContext(ctx)

				return kappCtrl
			},
		}

		sc, err := NewSmartContractProcessor(arguments)
		require.Nil(t, err)

		acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)
		acntDst.SetCode([]byte("code"))

		returnCode, err := sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
		return returnCode, ctx, arguments.AccountsCacher, err
	}

	shortAddress := bytes.Repeat([]byte{0x01}, 31)

	// the address check runs before processSCOutputAccounts' simulation guard, so each case runs on
	// both paths: in a simulation, and in a real execution, which also saves storage updates
	for _, isSimulation := range []bool{true, false} {
		mode := fmt.Sprintf("simulation=%v", isSimulation)

		t.Run(mode+"/short output account fails the transaction and charges the full gas after FixAuditChangesV6", func(t *testing.T) {
			t.Parallel()

			returnCode, ctx, cacher, err := execute(t, true, isSimulation, shortAddress)
			require.Equal(t, vmcommon.VMExecutionFailed, returnCode)
			require.EqualError(t, err, process.ErrInvalidOutputAccountAddressLength.Error())
			require.Zero(t, ctx.GetGasLimit())

			cached, stored := userAccountExists(t, cacher, shortAddress)
			require.False(t, cached)
			require.False(t, stored)
		})

		t.Run(mode+"/short output account succeeds before FixAuditChangesV6", func(t *testing.T) {
			t.Parallel()

			returnCode, ctx, cacher, err := execute(t, false, isSimulation, shortAddress)
			require.Equal(t, vmcommon.Ok, returnCode)
			require.NoError(t, err)
			require.Equal(t, gasRemaining, ctx.GetGasLimit())

			cached, stored := userAccountExists(t, cacher, shortAddress)
			require.True(t, cached)
			require.True(t, stored)
		})

		t.Run(mode+"/32-byte output account succeeds after FixAuditChangesV6", func(t *testing.T) {
			t.Parallel()

			address := bytes.Repeat([]byte{0x01}, 32)
			returnCode, ctx, cacher, err := execute(t, true, isSimulation, address)
			require.Equal(t, vmcommon.Ok, returnCode)
			require.NoError(t, err)
			require.Equal(t, gasRemaining, ctx.GetGasLimit())

			cached, stored := userAccountExists(t, cacher, address)
			require.True(t, cached)
			require.True(t, stored)
		})
	}
}

func TestScProcessor_UpdateSmartContractCodeRejectsOverLimitCode(t *testing.T) {
	t.Parallel()

	arguments := createMockSmartContractProcessorArguments()
	arguments.BlockChainHook = &contextmock.BlockchainHookStub{
		FilterCodeMetadataForUpgradeCalled: func(input []byte) ([]byte, error) {
			return input, nil
		},
	}
	sc, err := NewSmartContractProcessor(arguments)
	require.Nil(t, err)

	preForkArguments := createMockSmartContractProcessorArguments()
	preForkArguments.ForkController, err = fork.NewForkController(config.EnableEpochs{FixAuditChangesV7: 100}, &commommock.EpochNotifierStub{})
	require.Nil(t, err)
	preForkSc, err := NewSmartContractProcessor(preForkArguments)
	require.Nil(t, err)

	scAddress := sharedCodeSCAddress(1)
	require.True(t, core.IsSmartContractAddress(scAddress))
	deployer := []byte("deployer-address-32-bytes-long!!")
	existingCode := []byte("existing-code")
	codeAtLimit := bytes.Repeat([]byte("c"), int(common.MaxLeafSize))
	codeOverLimit := bytes.Repeat([]byte("d"), int(common.MaxLeafSize)+1)

	newAccount := func(owner []byte, metadata vmcommon.CodeMetadata) state.UserAccountHandler {
		acc, errNew := state.NewUserAccount(scAddress)
		require.Nil(t, errNew)
		acc.SetOwnerAddress(owner)
		acc.SetCodeMetadata(metadata.ToBytes())
		return acc
	}
	// stores the code so the account is a real upgrade target, with a code hash GetCode resolves
	deployedContractWithCode := func(cacher state.AccountsCacher, code []byte) state.UserAccountHandler {
		acc, errLoad := cacher.LoadUser(scAddress)
		require.Nil(t, errLoad)
		acc.SetOwnerAddress(deployer)
		metadata := vmcommon.CodeMetadata{Upgradeable: true}
		acc.SetCodeMetadata(metadata.ToBytes())
		acc.SetCode(code)
		require.Nil(t, cacher.SaveAll())
		cacher.ResetAll(true)

		acc, errLoad = cacher.GetExistingUser(scAddress)
		require.Nil(t, errLoad)
		require.Equal(t, code, cacher.GetCode(acc.GetCodeHash()))
		return acc
	}
	deployedContract := func(cacher state.AccountsCacher) state.UserAccountHandler {
		return deployedContractWithCode(cacher, existingCode)
	}
	outputAccount := func(code []byte) *vmcommon.OutputAccount {
		return &vmcommon.OutputAccount{
			Address:             scAddress,
			Code:                code,
			CodeMetadata:        []byte{1, 0},
			CodeDeployerAddress: deployer,
		}
	}

	t.Run("deploy over the limit is rejected", func(t *testing.T) {
		acc := newAccount(nil, vmcommon.CodeMetadata{})
		metadataBefore := acc.GetCodeMetadata()
		vmOutput := &vmcommon.VMOutput{}
		err := sc.updateSmartContractCode(vmOutput, acc, outputAccount(codeOverLimit))
		require.Equal(t, process.ErrContractCodeTooLarge, err)
		require.Empty(t, acc.GetOwnerAddress())
		require.Equal(t, metadataBefore, acc.GetCodeMetadata())
		require.False(t, acc.HasNewCode())
		require.Empty(t, vmOutput.Logs)
	})

	t.Run("upgrade over the limit is rejected", func(t *testing.T) {
		acc := deployedContract(arguments.AccountsCacher)
		metadataBefore := acc.GetCodeMetadata()
		vmOutput := &vmcommon.VMOutput{}
		err := sc.updateSmartContractCode(vmOutput, acc, outputAccount(codeOverLimit))
		require.Equal(t, process.ErrContractCodeTooLarge, err)
		require.Equal(t, deployer, acc.GetOwnerAddress())
		require.Equal(t, metadataBefore, acc.GetCodeMetadata())
		require.Equal(t, existingCode, arguments.AccountsCacher.GetCode(acc.GetCodeHash()))
		require.False(t, acc.HasNewCode())
		require.Empty(t, vmOutput.Logs)
	})

	t.Run("deploy at the limit is accepted", func(t *testing.T) {
		acc := newAccount(nil, vmcommon.CodeMetadata{})
		vmOutput := &vmcommon.VMOutput{}
		err := sc.updateSmartContractCode(vmOutput, acc, outputAccount(codeAtLimit))
		require.NoError(t, err)
		require.Equal(t, deployer, acc.GetOwnerAddress())
		require.True(t, acc.HasNewCode())
		require.Len(t, vmOutput.Logs, 1)
	})

	t.Run("upgrade at the limit is accepted", func(t *testing.T) {
		acc := deployedContract(arguments.AccountsCacher)
		err := sc.updateSmartContractCode(&vmcommon.VMOutput{}, acc, outputAccount(codeAtLimit))
		require.NoError(t, err)
		require.True(t, acc.HasNewCode())
	})

	t.Run("metadata-only upgrade of over-limit code is accepted", func(t *testing.T) {
		acc := deployedContractWithCode(arguments.AccountsCacher, codeOverLimit)
		metadata := vmcommon.CodeMetadata{Upgradeable: true, Payable: true}
		acc.SetCodeMetadata(metadata.ToBytes())
		vmOutput := &vmcommon.VMOutput{}
		err := sc.updateSmartContractCode(vmOutput, acc, outputAccount(codeOverLimit))
		require.NoError(t, err)
		require.Equal(t, outputAccount(codeOverLimit).CodeMetadata, acc.GetCodeMetadata())
		require.Len(t, vmOutput.Logs, 1)
	})

	t.Run("over-limit code that is neither a deploy nor an upgrade is ignored", func(t *testing.T) {
		acc := newAccount([]byte("another-owner"), vmcommon.CodeMetadata{Upgradeable: true})
		err := sc.updateSmartContractCode(&vmcommon.VMOutput{}, acc, outputAccount(codeOverLimit))
		require.NoError(t, err)
		require.False(t, acc.HasNewCode())
	})

	t.Run("deploy over the limit is accepted before FixAuditChangesV7", func(t *testing.T) {
		acc := newAccount(nil, vmcommon.CodeMetadata{})
		err := preForkSc.updateSmartContractCode(&vmcommon.VMOutput{}, acc, outputAccount(codeOverLimit))
		require.NoError(t, err)
		require.Equal(t, deployer, acc.GetOwnerAddress())
		require.True(t, acc.HasNewCode())
	})

	t.Run("upgrade over the limit is accepted before FixAuditChangesV7", func(t *testing.T) {
		acc := deployedContract(preForkArguments.AccountsCacher)
		err := preForkSc.updateSmartContractCode(&vmcommon.VMOutput{}, acc, outputAccount(codeOverLimit))
		require.NoError(t, err)
		require.True(t, acc.HasNewCode())
	})
}

func TestScProcessor_ExecuteSmartContractTransactionOverLimitIndirectDeploy(t *testing.T) {
	t.Parallel()

	const gasLimit = uint64(1000)
	const gasRemaining = uint64(400)

	newScAddress := sharedCodeSCAddress(2)

	// the VM output of a CreateContract hook: a new contract account carrying the code to deploy
	execute := func(t *testing.T, fixV7 bool) (vmcommon.ReturnCode, kapp.KappContext, state.AccountsCacher, error) {
		arguments := createMockSmartContractProcessorArguments()
		if !fixV7 {
			forkController, err := fork.NewForkController(config.EnableEpochs{FixAuditChangesV7: 100}, &commommock.EpochNotifierStub{})
			require.Nil(t, err)
			arguments.ForkController = forkController
		}
		arguments.VmContainer = &contextmock.VMContainerMock{
			GetCalled: func(key []byte) (vmcommon.VMExecutionHandler, error) {
				return &contextmock.VMExecutionHandlerStub{
					RunSmartContractCallCalled: func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
						return &vmcommon.VMOutput{
							ReturnCode:   vmcommon.Ok,
							GasRemaining: gasRemaining,
							OutputAccounts: map[string]*vmcommon.OutputAccount{
								string(newScAddress): {
									Address:             newScAddress,
									Code:                bytes.Repeat([]byte("d"), int(common.MaxLeafSize)+1),
									CodeMetadata:        []byte{1, 0},
									CodeDeployerAddress: input.RecipientAddr,
								},
							},
						}, nil
					},
				}, nil
			},
		}

		contract := transaction.SmartContract{
			Type:      transaction.SmartContract_SCInvoke,
			CallValue: map[string]*transaction.CallValue{"KLV": {Amount: 45}},
			Address:   core.ZeroAddress,
		}
		tx, _ := createTransactionMock(&contract, transaction.TXContract_SmartContractType, []byte("SRC"), 0, [][]byte{[]byte("abba@0500@0000")})
		tx.GasLimit = gasLimit

		computedHash, err := tools.CalculateHash(arguments.Marshalizer, arguments.Hasher, tx.RawData)
		require.Nil(t, err)

		ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
			OriginalSender: tx.GetSender(),
			ContractType:   tx.RawData.GetContract()[0].Type,
			Block:          &block.Block{},
			TxHash:         computedHash,
			TX:             tx,
		})

		tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
		require.Nil(t, err)

		arguments.BlockChainHook = &contextmock.BlockchainHookStub{
			GetKAppControllerCalled: func() kapp.KAppController {
				kappCtrl, errCtrl := kappcontroller.NewKappController(kappcontroller.ArgsNewKApp{
					Hasher:         arguments.Hasher,
					Marshalizer:    arguments.Marshalizer,
					PubkeyConv:     arguments.PubkeyConv,
					ForkController: arguments.ForkController,
					AccountsCacher: arguments.AccountsCacher,
					RatingsData:    &commommock.RatingsInfoMock{},
				})
				require.Nil(t, errCtrl)
				_ = kappCtrl.GetAccountsKApp().SetAccountsCacher(arguments.AccountsCacher)
				_ = kappCtrl.GetAccountsKApp().SetKAppController(kappCtrl)
				_ = kappCtrl.GetKDAKApp().SetAccountsCacher(arguments.AccountsCacher)
				_ = kappCtrl.GetKDAKApp().SetKAppController(kappCtrl)
				kappCtrl.SetCurrentKAppContext(ctx)

				return kappCtrl
			},
		}

		sc, err := NewSmartContractProcessor(arguments)
		require.Nil(t, err)

		acntSrc, acntDst := initAccounts(tx, arguments.AccountsCacher, contract.CallValue["KLV"].Amount*2)
		acntDst.SetCode([]byte("code"))

		returnCode, err := sc.ExecuteSmartContractTransaction(ctx, tc, acntSrc, acntDst)
		return returnCode, ctx, arguments.AccountsCacher, err
	}

	// flushes first, so a rejected deploy shows as no code in the trie rather than only as a failed lookup
	storedCode := func(t *testing.T, cacher state.AccountsCacher) []byte {
		require.Nil(t, cacher.SaveAll())
		cacher.ResetAll(true)

		acc, err := cacher.LoadUser(newScAddress)
		require.Nil(t, err)
		return cacher.GetCode(acc.GetCodeHash())
	}

	t.Run("over-limit indirect deploy fails the transaction and charges the full gas after FixAuditChangesV7", func(t *testing.T) {
		t.Parallel()

		returnCode, ctx, cacher, err := execute(t, true)
		require.Equal(t, vmcommon.VMExecutionFailed, returnCode)
		require.EqualError(t, err, process.ErrContractCodeTooLarge.Error())
		require.Zero(t, ctx.GetGasLimit())
		require.Empty(t, storedCode(t, cacher))
	})

	t.Run("over-limit indirect deploy succeeds before FixAuditChangesV7", func(t *testing.T) {
		t.Parallel()

		returnCode, ctx, cacher, err := execute(t, false)
		require.Equal(t, vmcommon.Ok, returnCode)
		require.NoError(t, err)
		require.Equal(t, gasRemaining, ctx.GetGasLimit())
		require.Len(t, storedCode(t, cacher), int(common.MaxLeafSize)+1)
	})
}
