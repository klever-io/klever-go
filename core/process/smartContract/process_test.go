package smartContract

import (
	"bytes"
	"errors"
	"fmt"
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
		GasSchedule:         notifierMock.NewGasScheduleNotifierMock(gasSchedule),
		WasmVMChangeLocker:  &sync.RWMutex{},
		VMOutputCacher:      txcache.NewDisabledCache(),
		ForkController:      forkController,
		IsGenesisProcessing: true,
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

	sc.GasScheduleChange(gasSchedule)

	require.Equal(t, sc.builtInGasCosts[core.BuiltInFunctionTransfer], uint64(20))
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

	address := []byte("output-account-address")
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
