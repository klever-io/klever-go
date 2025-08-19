package hostCore_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)
var testOwnerAddress, _ = addressConverter.Decode("klv10gq6xsegedacd084vmpr2xus950j3d6lhqjfe8ue2xkmfwtkzavqnqhz99")
var testToAddress, _ = addressConverter.Decode("klv15zssmvht00ugvge5le9n885kahc5ykxzvmxx6xwz5ya2an562yyssfa0c5")

var marshalizer = marshal.NewProtoMarshalizer()

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

func loadUsersAccounts(t *testing.T, kappDB state.AccountsAdapter, accCacher state.AccountsCacher) {
	ownerAcc, _ := accCacher.LoadUser(testOwnerAddress)
	_ = ownerAcc.AddToBalance(100_000_000, nil, true)

	toAcc, _ := accCacher.LoadUser(testToAddress)
	_ = toAcc.AddToBalance(100_000_000, nil, true)

	kdaKapp, _ := accCacher.LoadKApp(kapps.KDAKAppAddress)
	require.NoError(t, kappDB.SaveAccount(kdaKapp))

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

func createFullArgumentsForKAppsProcessingMemory(t *testing.T) state.AccountsCacher {
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

	loadUsersAccounts(t, kappAccountsDB, accCacher)

	return accCacher
}

func TestExecuteKDATransfer(t *testing.T) {
	hostParams := makeHostParameters()

	accCacher := createFullArgumentsForKAppsProcessingMemory(t)

	mockWorld := worldmock.NewMockWorld()
	mockWorld.AccountsCacher = accCacher

	_, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	_, err = accCacher.LoadUser(testToAddress)
	require.NoError(t, err)
	err = accCacher.SaveAll()
	require.NoError(t, err)

	require.NoError(t, mockWorld.InitBuiltinFunctions(hostParams.GasSchedule, hostParams.ForkController))

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParams)
	require.NoError(t, err)

	value := int64(100)
	vmHost.SetRuntimeContext(&contextmock.RuntimeContextMock{})
	transfers := []*vmcommon.KDATransfer{
		{
			KDATokenName: kdautils.KLVIdentifier,
			KDATokenType: uint32(core.Fungible),
			KDAValue:     big.NewInt(value),
		},
		{
			KDATokenName: kdautils.KLVIdentifier,
			KDATokenType: uint32(core.Fungible),
			KDAValue:     big.NewInt(value),
		},
	}

	runtime := vmHost.Runtime()
	metering := vmHost.Metering()

	oneTransferCost := metering.GasSchedule().BuiltInCost.Transfer
	gasProvided := oneTransferCost*uint64(len(transfers)) + 1
	input := &vmcommon.ContractCallInput{
		RecipientAddr: testToAddress,
		VMInput: vmcommon.VMInput{
			CallerAddr:  testOwnerAddress,
			GasProvided: gasProvided,
		},
	}

	runtime.SetVMInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)

	args := &vmhost.KDATransfersArgs{
		Sender:         testOwnerAddress,
		Destination:    testToAddress,
		OriginalCaller: runtime.GetOriginalCallerAddress(),
		Transfers:      transfers,
	}

	vmOutput, gasConsumed, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
	require.Nil(t, err)
	require.NotNil(t, vmOutput)
	require.Equal(t, uint64(2), gasConsumed)
}

func TestExecution_DeleteContract(t *testing.T) {
	hostParams := makeHostParameters()
	accCacher := createFullArgumentsForKAppsProcessingMemory(t)
	mockWorld := worldmock.NewMockWorld()
	mockWorld.AccountsCacher = accCacher

	_, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	err = accCacher.SaveAll()
	require.NoError(t, err)

	require.NoError(t, mockWorld.InitBuiltinFunctions(hostParams.GasSchedule, hostParams.ForkController))

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParams)
	require.NoError(t, err)

	deleteGasCost := vmHost.Metering().GasSchedule().BaseOpsAPICost.CreateContract
	extraGas := uint64(1000)

	tests := []struct {
		name                 string
		callerAddr           []byte
		gasProvided          uint64
		expectedMsg          string
		shouldDelete         bool
		expectedGasRemaining uint64
	}{
		{
			name:                 "successful delete with extra gas",
			callerAddr:           testOwnerAddress,
			gasProvided:          deleteGasCost + extraGas,
			expectedMsg:          "",
			shouldDelete:         true,
			expectedGasRemaining: extraGas,
		},
		{
			name:                 "exact gas provided for delete",
			callerAddr:           testOwnerAddress,
			gasProvided:          deleteGasCost,
			expectedMsg:          "",
			shouldDelete:         true,
			expectedGasRemaining: 0,
		},
		{
			name:                 "fail delete with insufficient gas",
			callerAddr:           testOwnerAddress,
			gasProvided:          deleteGasCost - 1,
			expectedMsg:          vmhost.ErrNotEnoughGas.Error(),
			shouldDelete:         false,
			expectedGasRemaining: 0,
		},
		{
			name:                 "fail delete from non-owner",
			callerAddr:           []byte("not-owner"),
			gasProvided:          deleteGasCost + extraGas,
			expectedMsg:          vmhost.ErrUpgradeNotAllowed.Error(),
			shouldDelete:         false,
			expectedGasRemaining: 0,
		},
	}

	scAddress := []byte("scAddress")
	scAccount, err := accCacher.LoadUser(scAddress)
	require.NoError(t, err)

	scAccount.SetCode([]byte("dummy code"))
	scAccount.SetOwnerAddress(testOwnerAddress)
	scAccount.SetCodeMetadata([]byte{
		vmcommon.MetadataUpgradeable,
		0,
	})
	err = accCacher.SaveUser(scAccount)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &vmcommon.ContractCallInput{
				VMInput: vmcommon.VMInput{
					CallerAddr:  tt.callerAddr,
					GasProvided: tt.gasProvided,
					CallType:    vm.DirectCall,
				},
				RecipientAddr: scAddress,
				Function:      vmhost.DeleteFunctionName,
			}

			vmOutput, err := vmHost.RunSmartContractCall(input)
			require.NoError(t, err)
			require.NotNil(t, vmOutput)

			if tt.shouldDelete {
				require.Contains(t, vmOutput.DeletedAccounts, scAddress)
				require.Equal(t, tt.expectedGasRemaining, vmOutput.GasRemaining)
			} else {
				require.Equal(t, tt.expectedMsg, vmOutput.ReturnMessage)
				require.Empty(t, vmOutput.DeletedAccounts)
			}
		})
	}
}

func TestExecution_CreateContract(t *testing.T) {
	hostParams := makeHostParameters()
	accCacher := createFullArgumentsForKAppsProcessingMemory(t)
	mockWorld := worldmock.NewMockWorld()
	mockWorld.AccountsCacher = accCacher

	_, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	err = accCacher.SaveAll()
	require.NoError(t, err)

	require.NoError(t, mockWorld.InitBuiltinFunctions(hostParams.GasSchedule, hostParams.ForkController))

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParams)
	require.NoError(t, err)

	tests := []struct {
		name              string
		callerAddr        []byte
		contractCode      []byte
		klvTransferAmount int64
		expectedMsg       string
		shouldCreate      bool
	}{
		{
			name:         "fail with invalid contract code",
			callerAddr:   testOwnerAddress,
			contractCode: []byte("invalid code"),
			expectedMsg:  "invalid contract code",
			shouldCreate: false,
		},
		{
			name:         "successful create",
			callerAddr:   testOwnerAddress,
			contractCode: testcommon.GetTestSCCode("empty", "../../"),
			expectedMsg:  "",
			shouldCreate: true,
		},
		{
			name:              "fail create, init not payable",
			callerAddr:        testOwnerAddress,
			contractCode:      testcommon.GetTestSCCode("empty", "../../"),
			klvTransferAmount: 100,
			expectedMsg:       "function does not accept KDA payment",
			shouldCreate:      false,
		},
		{
			name:              "success create, init payable",
			callerAddr:        testOwnerAddress,
			contractCode:      testcommon.GetTestSCCode("init-payable", "../../"),
			klvTransferAmount: 100,
			expectedMsg:       "",
			shouldCreate:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &vmcommon.ContractCreateInput{
				VMInput: vmcommon.VMInput{
					CallerAddr:  tt.callerAddr,
					GasProvided: 10000,
					CallType:    vm.DirectCall,
				},
				ContractCode:         tt.contractCode,
				ContractCodeMetadata: []byte{0, 0},
			}
			if tt.klvTransferAmount > 0 {
				input.VMInput.KDATransfers = []*vmcommon.KDATransfer{
					{
						KDATokenName: kdautils.KLVIdentifier,
						KDATokenType: uint32(core.Fungible),
						KDAValue:     big.NewInt(tt.klvTransferAmount),
					},
				}
			}

			vmOutput, err := vmHost.RunSmartContractCreate(input)
			require.NoError(t, err)
			require.NotNil(t, vmOutput)

			if tt.shouldCreate {
				require.Len(t, vmOutput.OutputAccounts, 1)
			} else {
				require.Equal(t, tt.expectedMsg, vmOutput.ReturnMessage)
				require.Empty(t, vmOutput.DeletedAccounts)
			}
		})
	}
}

func TestExecuteKDATransfer_SCValidations(t *testing.T) {
	hostParams := makeHostParameters()
	accCacher := createFullArgumentsForKAppsProcessingMemory(t)
	mockWorld := worldmock.NewMockWorld()
	mockWorld.AccountsCacher = accCacher

	_, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	err = accCacher.SaveAll()
	require.NoError(t, err)

	require.NoError(t, mockWorld.InitBuiltinFunctions(hostParams.GasSchedule, hostParams.ForkController))

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParams)
	require.NoError(t, err)

	value := int64(100)
	transfers := []*vmcommon.KDATransfer{
		{
			KDATokenName: kdautils.KLVIdentifier,
			KDATokenType: uint32(core.Fungible),
			KDAValue:     big.NewInt(value),
		},
	}

	runtime := vmHost.Runtime()
	metering := vmHost.Metering()

	oneTransferCost := metering.GasSchedule().BuiltInCost.Transfer
	gasProvided := oneTransferCost*uint64(len(transfers)) + 1

	setupTest := func(recipientAddr []byte) {
		input := &vmcommon.ContractCallInput{
			RecipientAddr: recipientAddr,
			VMInput: vmcommon.VMInput{
				CallerAddr:  testOwnerAddress,
				GasProvided: gasProvided,
			},
		}
		runtime.SetVMInput(input)
		metering.InitStateFromContractCallInput(&input.VMInput)
	}

	t.Run("Transfer to non-SC address", func(t *testing.T) {
		setupTest(testToAddress)

		args := &vmhost.KDATransfersArgs{
			Sender:         testOwnerAddress,
			Destination:    testToAddress,
			OriginalCaller: runtime.GetOriginalCallerAddress(),
			Transfers:      transfers,
		}

		vmOutput, gasConsumed, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
		require.Nil(t, err)
		require.NotNil(t, vmOutput)
		require.Greater(t, gasConsumed, uint64(0))
	})

	t.Run("Transfer to non-existent SC address", func(t *testing.T) {
		scAddress, _ := addressConverter.Decode("klv1qqqqqqqqqqqqqpgqpg2ff85tljne96d2jwedj4mkrhsu3up5c0nq0x8g69")

		setupTest(scAddress)

		args := &vmhost.KDATransfersArgs{
			Sender:         testOwnerAddress,
			Destination:    scAddress,
			OriginalCaller: runtime.GetOriginalCallerAddress(),
			Transfers:      transfers,
		}

		vmOutput, _, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
		require.Error(t, err)
		require.Equal(t, common.ErrAccNotFound, err)
		require.Nil(t, vmOutput)
	})

	t.Run("Transfer to SC address without code", func(t *testing.T) {
		scAddress := make([]byte, 32)
		copy(scAddress[:4], []byte{0, 0, 0, 0})

		scAccount, _ := accCacher.LoadUser(scAddress)
		require.NoError(t, accCacher.SaveUser(scAccount))

		setupTest(scAddress)

		args := &vmhost.KDATransfersArgs{
			Sender:         testOwnerAddress,
			Destination:    scAddress,
			OriginalCaller: runtime.GetOriginalCallerAddress(),
			Transfers:      transfers,
		}

		vmOutput, _, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
		require.Error(t, err)
		require.Equal(t, vmhost.ErrAccountNotPayable, err)
		require.Nil(t, vmOutput)
	})

	t.Run("Transfer to valid SC address", func(t *testing.T) {
		scAddress := make([]byte, 32)
		copy(scAddress[:4], []byte{0, 0, 0, 0})

		scAccount, _ := accCacher.LoadUser(scAddress)
		scAccount.SetCode([]byte("dummy code"))
		scAccount.SetCodeMetadata([]byte{
			vmcommon.MetadataPayable,
			2,
		})
		require.NoError(t, accCacher.SaveUser(scAccount))

		setupTest(scAddress)

		args := &vmhost.KDATransfersArgs{
			Sender:         testOwnerAddress,
			Destination:    scAddress,
			OriginalCaller: runtime.GetOriginalCallerAddress(),
			Transfers:      transfers,
		}

		vmOutput, _, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
		require.Nil(t, err)
		require.NotNil(t, vmOutput)
	})

	t.Run("Transfer to non-payable SC address", func(t *testing.T) {
		scAddress := make([]byte, 32)
		copy(scAddress[:4], []byte{0, 0, 0, 0})

		scAccount, _ := accCacher.LoadUser(scAddress)
		scAccount.SetCode([]byte("dummy code"))
		scAccount.SetCodeMetadata([]byte{
			vmcommon.MetadataPayableBySC,
			4,
		})
		require.NoError(t, accCacher.SaveUser(scAccount))

		scAccount.SetCodeMetadata([]byte{
			vmcommon.MetadataPayableBySC,
			4,
		})
		require.NoError(t, accCacher.SaveUser(scAccount))

		setupTest(scAddress)

		args := &vmhost.KDATransfersArgs{
			Sender:         testOwnerAddress,
			Destination:    scAddress,
			OriginalCaller: runtime.GetOriginalCallerAddress(),
			Transfers:      transfers,
		}

		vmOutput, _, err := vmHost.ExecuteKDATransfer(args, vm.DirectCall)
		require.Error(t, err)
		require.Equal(t, vmhost.ErrAccountNotPayable, err)
		require.Nil(t, vmOutput)
	})
}
