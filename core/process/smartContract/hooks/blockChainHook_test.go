package hooks

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	builtinfunctions "github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/kapp/disabled"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
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
	kvmContextMock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBlockChainHookImpl_IsSmartContract(t *testing.T) {
	// Test cases
	tests := []struct {
		name          string
		args          string
		want          bool
		accountExists bool
		codeExists    bool
		codeMetadata  []byte
	}{
		{
			name:          "Should fail for invalid length",
			args:          "12345",
			want:          false,
			accountExists: false,
			codeExists:    false,
		},
		{
			name:          "Should fail for not enough leading zeros",
			args:          "000000000001000000005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          false,
			accountExists: false,
			codeExists:    false,
		},
		{
			name:          "Should fail for invalid VM type",
			args:          "000000000000000006005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          false,
			accountExists: false,
			codeExists:    false,
		},
		{
			name:          "Should be a valid address, but fail for not existing account",
			args:          "000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          false,
			accountExists: false,
			codeExists:    false,
		},
		{
			name:          "Should be a valid empty address, but fail for not existing account",
			args:          "0000000000000000000000000000000000000000000000000000000000000000",
			want:          false,
			accountExists: false,
			codeExists:    false,
		},
		{
			name:          "Should be a valid address, existing account, but fail for empty code",
			args:          "000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          false,
			accountExists: true,
			codeExists:    false,
		},
		{
			name:          "Should be a valid address, existing account, existing code and pass",
			args:          "000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          true,
			accountExists: true,
			codeExists:    true,
		},
		{
			name:          "Deleted contract, existing account, non existing code and pass",
			args:          "000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1",
			want:          true,
			accountExists: true,
			codeExists:    false,
			codeMetadata:  []byte{0, 0}, // Simulating deleted contract with empty code metadata
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// stub initialization
			cacherStub := &commonMock.AccountsCacherStub{}
			h := &BlockChainHookImpl{accountsCacher: cacherStub}
			cacherStub.GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
				if !tt.accountExists {
					return nil, fmt.Errorf("account not found")
				}
				accountStub := &commonMock.UserAccountHandlerStub{}
				accountStub.GetCodeHashCalled = func() []byte {
					return []byte("codeHash")
				}
				accountStub.GetCodeMetadataCalled = func() []byte {
					return tt.codeMetadata
				}
				return accountStub, nil
			}
			cacherStub.GetCodeCalled = func(codeHash []byte) []byte {
				if !tt.codeExists {
					return []byte{}
				}
				return []byte("code")
			}

			// perform test
			address, _ := hex.DecodeString(tt.args)
			if got := h.IsSmartContract(address); got != tt.want {
				t.Errorf("IsSmartContract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlockChainHookImpl_IsPayable(t *testing.T) {
	commonAddress := []byte("normalAddress1234567890123456789")
	scAddress, _ := addressConverter.Decode("klv1qqqqqqqqqqqqqpgqpg2ff85tljne96d2jwedj4mkrhsu3up5c0nq0x8g69")

	tests := []struct {
		name             string
		sndAddress       []byte
		recvAddress      []byte
		setupMocks       func(*commonMock.AccountsCacherStub)
		codeHashMock     []byte
		codeMetadataMock []byte
		codeMock         []byte
		getUserErrorMock error
		expectedPayable  bool
		expectedError    error
	}{
		{
			name:             "System account address - should return false",
			sndAddress:       commonAddress,
			recvAddress:      make([]byte, 32), // System address
			getUserErrorMock: errors.New("should not be called"),
			expectedPayable:  false,
			expectedError:    nil,
		},
		{
			name:             "Smart contract address format but not initialized - should return false",
			sndAddress:       commonAddress,
			recvAddress:      scAddress,
			getUserErrorMock: errors.New("account not found"),
			expectedPayable:  false,
			expectedError:    nil,
		},
		{
			name:            "Smart contract address format but no code - should return false",
			sndAddress:      commonAddress,
			recvAddress:     scAddress,
			codeHashMock:    []byte("codeHash"),
			codeMock:        []byte{}, // No code
			expectedPayable: false,
			expectedError:   nil,
		},
		{
			name:             "Normal address (not smart contract) - should return true",
			sndAddress:       commonAddress,
			recvAddress:      commonAddress,
			getUserErrorMock: errors.New("should not be called"),
			expectedPayable:  true,
			expectedError:    nil,
		},
		{
			name:             "Smart contract with GetUserAccount error - should return false without error",
			sndAddress:       commonAddress,
			recvAddress:      scAddress,
			getUserErrorMock: errors.New("database error"),
			expectedPayable:  false,
			expectedError:    nil,
		},
		{
			name:             "Smart contract payable - sender not SC - should return metadata.Payable",
			sndAddress:       commonAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{0, 2}, // Set Payable flag (bit 1 of second byte)
			codeMock:         []byte("some code"),
			expectedPayable:  true,
			expectedError:    nil,
		},
		{
			name:             "Smart contract not payable - sender not SC - should return false",
			sndAddress:       commonAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{0, 0}, // No flags set
			codeMock:         []byte("some code"),
			expectedPayable:  false,
			expectedError:    nil,
		},
		{
			name:             "Smart contract payable by SC - sender is SC - should return metadata.PayableBySC",
			sndAddress:       scAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{0, 4}, // Set PayableBySC
			codeMock:         []byte("some code"),
			expectedPayable:  true,
			expectedError:    nil,
		},
		{
			name:             "Smart contract not payable by SC - sender is SC - should return false",
			sndAddress:       scAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{0, 0}, // No flags set
			codeMock:         []byte("some code"),
			expectedPayable:  false,
			expectedError:    nil,
		},
		{
			name:             "Smart contract with both payable flags - sender is SC - should return PayableBySC",
			sndAddress:       scAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{0, 6}, // Payable = true, PayableBySC = true
			codeMock:         []byte("some code"),
			expectedPayable:  true,
			expectedError:    nil,
		},
		{
			name:             "Smart contract with empty metadata - should use default values",
			sndAddress:       commonAddress,
			recvAddress:      scAddress,
			codeHashMock:     []byte("codeHash"),
			codeMetadataMock: []byte{}, // Empty metadata
			codeMock:         []byte("some code"),
			expectedPayable:  false,
			expectedError:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			cacherStub := &commonMock.AccountsCacherStub{}
			accountStub := &commonMock.UserAccountHandlerStub{
				GetCodeHashCalled: func() []byte {
					return tt.codeHashMock
				},
				GetCodeMetadataCalled: func() []byte {
					return tt.codeMetadataMock
				},
			}
			cacherStub.GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
				return accountStub, tt.getUserErrorMock
			}
			cacherStub.GetCodeCalled = func(codeHash []byte) []byte {
				return tt.codeMock
			}

			h := &BlockChainHookImpl{
				accountsCacher: cacherStub,
			}

			// Execute test
			isPayable, err := h.IsPayable(tt.sndAddress, tt.recvAddress)

			// Verify results
			if tt.expectedError != nil {
				assert.Error(t, err)
				if err != nil {
					assert.Equal(t, tt.expectedError.Error(), err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedPayable, isPayable)
		})
	}
}

func TestBlockChainHookImpl_GetKDAToken(t *testing.T) {
	// Test data setup
	testAddress := []byte("testAddress123456789012345678901234")
	testAssetID := []byte("TESTKDA")
	testKDAData := &kapps.KDAData{
		ID:                testAssetID,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("Test KDA"),
		Ticker:            testAssetID,
		Precision:         6,
		InitialSupply:     1000000,
		CirculatingSupply: 500000,
	}
	testUserKDAData := &kapps.UserKDA{
		Balance:       1000,
		FrozenBalance: 100,
		LastClaim:     &kapps.LastClaim{Timestamp: 123456789},
		Buckets:       make(map[string]*kapps.UserBucket),
	}

	tests := []struct {
		name            string
		address         []byte
		assetID         []byte
		nonce           uint64
		getUserAccErr   bool
		getUserKDAErr   bool
		getKDAErr       bool
		expectedKDA     *kapps.KDAData
		expectedUserKDA *kapps.UserKDA
		expectError     bool
	}{
		{
			name:            "Success - nil address, valid asset",
			address:         nil,
			assetID:         testAssetID,
			nonce:           10,
			expectedKDA:     testKDAData,
			expectedUserKDA: &kapps.UserKDA{},
		},
		{
			name:            "Success - valid address with KDA data",
			address:         testAddress,
			assetID:         testAssetID,
			nonce:           10,
			expectedKDA:     testKDAData,
			expectedUserKDA: testUserKDAData,
		},
		{
			name:            "Error - GetUserAccount fails",
			address:         testAddress,
			assetID:         testAssetID,
			nonce:           10,
			getUserAccErr:   true,
			expectedKDA:     &kapps.KDAData{},
			expectedUserKDA: &kapps.UserKDA{},
			expectError:     true,
		},
		{
			name:            "Error - GetUserKDA fails",
			address:         testAddress,
			assetID:         testAssetID,
			nonce:           10,
			getUserKDAErr:   true,
			expectedKDA:     &kapps.KDAData{},
			expectedUserKDA: nil,
			expectError:     true,
		},
		{
			name:            "Error - GetKDA fails",
			address:         nil,
			assetID:         testAssetID,
			nonce:           10,
			getKDAErr:       true,
			expectedKDA:     nil,
			expectedUserKDA: &kapps.UserKDA{},
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create BlockChainHook with mocks configured for this test case
			accountsCacher := &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					if tt.getUserAccErr {
						return nil, errors.New("account not found")
					}
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							if tt.getUserKDAErr {
								return nil, errors.New("user KDA not found")
							}
							return testUserKDAData, nil
						},
					}, nil
				},
			}

			kappController := &stub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &stub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							if tt.getKDAErr {
								return nil, nil, errors.New("KDA not found")
							}
							return &commonMock.KAppAccountHandlerStub{}, testKDAData, nil
						},
					}
				},
			}

			forkController := commonMock.NewForkControllerStub()
			forkController.EnableSmartContractsValue = true

			bh := &BlockChainHookImpl{
				accountsCacher: accountsCacher,
				kappController: kappController,
				forkController: forkController,
			}

			// Execute
			actualKDA, actualUserKDA, actualError := bh.GetKDAToken(tt.address, tt.assetID, tt.nonce)

			// Verify
			if tt.expectError {
				assert.Error(t, actualError)
			} else {
				assert.NoError(t, actualError)
			}
			assert.Equal(t, tt.expectedKDA, actualKDA)
			assert.Equal(t, tt.expectedUserKDA, actualUserKDA)
		})
	}
}

// blockChainHookCounterPassthroughStub is a minimal BlockChainHookCounter that
// reports no errors and no-ops everything else, used to isolate higher-layer
// branches under test.
type blockChainHookCounterPassthroughStub struct{}

func (s *blockChainHookCounterPassthroughStub) ProcessCrtNumberOfTrieReadsCounter() error {
	return nil
}
func (s *blockChainHookCounterPassthroughStub) ProcessMaxBuiltInCounters(_ *vmcommon.ContractCallInput) error {
	return nil
}
func (s *blockChainHookCounterPassthroughStub) ResetCounters()                       {}
func (s *blockChainHookCounterPassthroughStub) SetMaximumValues(_ map[string]uint64) {}
func (s *blockChainHookCounterPassthroughStub) GetCounterValues() map[string]uint64  { return nil }
func (s *blockChainHookCounterPassthroughStub) IsInterfaceNil() bool                 { return s == nil }

func TestGetStorageData_AccNotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"direct ErrAccNotFound", common.ErrAccNotFound},
		{"wrapped ErrAccNotFound", fmt.Errorf("load: %w", common.ErrAccNotFound)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bh := &BlockChainHookImpl{
				counter: &blockChainHookCounterPassthroughStub{},
				accountsCacher: &commonMock.AccountsCacherStub{
					GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
						return nil, c.err
					},
				},
			}

			value, code, err := bh.GetStorageData([]byte("addr"), []byte("key"))
			assert.NoError(t, err)
			assert.Equal(t, uint32(0), code)
			assert.Equal(t, []byte{}, value)
		})
	}
}

///////////////////////////////////
// Read-only VM query wiring     //
///////////////////////////////////

const readOnlyOwnerStartBalance = int64(10_000_000_000)

// readOnlySeedCommittedTries builds fresh trie-backed adapters and commits a funded
// owner account plus the KLV KDA into them, returning the committed state that both
// the writable and read-only query cachers will read from.
func readOnlySeedCommittedTries(t *testing.T) (state.AccountsAdapter, state.AccountsAdapter, state.AccountsAdapter) {
	t.Helper()

	hasher := &sha256.Sha256{}
	tsm, err := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	require.NoError(t, err)

	userDB := createAccountsDB(hasher, marshalizer, factory.NewAccountCreator(), tsm)
	kappDB := createAccountsDB(hasher, marshalizer, factory.NewKAppAccountCreator(), tsm)
	peerDB := createAccountsDB(hasher, marshalizer, factory.NewPeerAccountCreator(), tsm)

	seed, err := state.NewAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	seed.ResetAll(true)

	owner := loadUserAccount(seed, testOwnerAddress)
	kdaKapp := loadKAppAccount(seed, kapps.KDAKAppAddress)
	initKLVintoKapps(kdaKapp)
	require.NoError(t, owner.AddToBalance(readOnlyOwnerStartBalance, nil, true))

	// commit funded owner + KLV KDA to the underlying tries
	require.NoError(t, seed.SaveAll())

	return userDB, kappDB, peerDB
}

func readOnlyArgsOver(userDB, kappDB, peerDB state.AccountsAdapter) state.ArgsAcccountCacher {
	return state.ArgsAcccountCacher{
		Accounts: userDB,
		Kapps:    kappDB,
		Peers:    peerDB,
	}
}

// readOnlyBuildController builds a KAppController bound to the given cacher, mirroring
// how the query VM element wires it (see cmd/node/sc.go).
func readOnlyBuildController(t *testing.T, cacher state.AccountsCacher, readOnly bool) kapp.KAppController {
	t.Helper()

	args := createArgsForTxProcessor()
	controller, err := kappcontroller.NewKappController(kappcontroller.ArgsNewKApp{
		Hasher:         args.Hasher,
		Marshalizer:    args.Marshalizer,
		PubkeyConv:     args.PubkeyConv,
		ForkController: args.ForkController,
		RatingsData:    args.RatingsData,
		ReadOnly:       readOnly,
	})
	require.NoError(t, err)
	require.NoError(t, controller.InitKApps(cacher))

	controller.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		ContractID:   0,
		ContractType: transaction.TXContract_SmartContractType,
		Block:        createBlockHeader(),
	}))

	return controller
}

// readOnlyCommittedKLV reads the owner's KLV balance from committed state via a fresh
// cacher, so it reflects only what actually persisted to the tries.
func readOnlyCommittedKLV(t *testing.T, userDB, kappDB, peerDB state.AccountsAdapter, address []byte) int64 {
	t.Helper()

	verifier, err := state.NewAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	verifier.ResetAll(true)

	acc, err := verifier.GetExistingUser(address)
	require.NoError(t, err)

	return acc.GetBalance(kdautils.KLVIdentifier, true)
}

// The query element must not reach the production KAppController. Mirrors the wiring
// in cmd/node/sc.go createScQueryElement (which is package main and not directly
// testable): the hook is handed a dedicated read-only controller over a read-only
// cacher, distinct from the node's production controller.
func TestBlockChainHook_QueryWiring_UsesDedicatedReadOnlyController(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB := readOnlySeedCommittedTries(t)

	prodCacher, err := state.NewAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	prodCacher.ResetAll(true)
	productionController := readOnlyBuildController(t, prodCacher, false)

	queryCacher, err := state.NewReadOnlyAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	queryCacher.ResetAll(true)
	queryController := readOnlyBuildController(t, queryCacher, true)

	hookImpl := &BlockChainHookImpl{kappController: queryController}

	require.NotSame(t, productionController, hookImpl.GetKAppController(),
		"the query hook must not be handed the production KAppController")
	require.True(t, hookImpl.GetKAppController().IsReadOnly(),
		"the controller reachable from the query hook must be read-only")
	require.False(t, productionController.IsReadOnly(),
		"the production controller must stay writable")
}

// The read-only guard sits at the dispatch point, so the built-in is refused before
// it ever runs - this is what covers the ~25 built-ins that do not check the flag.
func TestBlockChainHook_ProcessBuiltInFunction_ReadOnly_RefusesBeforeDispatch(t *testing.T) {
	t.Parallel()

	dispatched := false
	container := builtinfunctions.NewBuiltInFunctionContainer()
	require.NoError(t, container.Add("Transfer", &kvmContextMock.BuiltInFunctionStub{
		ProcessBuiltinFunctionCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			dispatched = true
			return &vmcommon.VMOutput{}, nil
		},
	}))

	hook := &BlockChainHookImpl{
		builtInFunctions: container,
		counter:          counters.NewDisabledCounter(),
		kappController: &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return disabled.NewDisabledKappContext() },
			IsReadOnlyCalled:            func() bool { return true },
		},
	}

	vmOutput, err := hook.ProcessBuiltInFunction(&vmcommon.ContractCallInput{
		VMInput:       vmcommon.VMInput{CallerAddr: []byte("caller")},
		RecipientAddr: []byte("recipient"),
		Function:      "Transfer",
	})

	require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation)
	require.Nil(t, vmOutput)
	require.False(t, dispatched, "a read-only query must not dispatch the built-in at all")
}

// A nil controller is an unknown execution context and must be refused, not assumed
// writable - and it must not nil-deref the way the pre-fix code did.
func TestBlockChainHook_ProcessBuiltInFunction_NilController_FailsClosedNoPanic(t *testing.T) {
	t.Parallel()

	container := builtinfunctions.NewBuiltInFunctionContainer()
	require.NoError(t, container.Add("Transfer", &kvmContextMock.BuiltInFunctionStub{
		ProcessBuiltinFunctionCalled: func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
			return &vmcommon.VMOutput{}, nil
		},
	}))

	hook := &BlockChainHookImpl{
		builtInFunctions: container,
		counter:          counters.NewDisabledCounter(),
	}

	require.NotPanics(t, func() {
		vmOutput, err := hook.ProcessBuiltInFunction(&vmcommon.ContractCallInput{
			VMInput:       vmcommon.VMInput{CallerAddr: []byte("caller")},
			RecipientAddr: []byte("recipient"),
			Function:      "Transfer",
		})
		require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation)
		require.Nil(t, vmOutput)
	})
}

// Bound to a dedicated read-only cacher, a hook transfer neither corrupts the
// production cached object nor reaches committed state.
func TestBlockChainHook_TransferViaReadOnlyCacher_LeavesCommittedStateUntouched(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB := readOnlySeedCommittedTries(t)

	// Represents live production state: owner is loaded and cached here.
	prodCacher, err := state.NewAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	prodCacher.ResetAll(true)
	prodOwner, err := prodCacher.GetExistingUser(testOwnerAddress)
	require.NoError(t, err)
	require.Equal(t, readOnlyOwnerStartBalance, prodOwner.GetBalance(kdautils.KLVIdentifier, true))

	// The query VM element uses its own read-only cacher over the same tries.
	queryCacher, err := state.NewReadOnlyAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	queryCacher.ResetAll(true)

	hookImpl := &BlockChainHookImpl{kappController: readOnlyBuildController(t, queryCacher, false)}

	err = hookImpl.TransferValueOnly(testToAddress, testOwnerAddress, big.NewInt(1))
	require.NoError(t, err)

	// A read-only SaveAll must never reach the tries.
	require.NoError(t, queryCacher.SaveAll())

	// Production's cached object is untouched (the query mutated only its own copy).
	require.Equal(t, readOnlyOwnerStartBalance, prodOwner.GetBalance(kdautils.KLVIdentifier, true),
		"read-only query transfer must not corrupt the production cached account")

	// Committed state is untouched.
	require.Equal(t, readOnlyOwnerStartBalance, readOnlyCommittedKLV(t, userDB, kappDB, peerDB, testOwnerAddress),
		"read-only query transfer must not persist to committed state")
}

// Read-only MODE alone refuses the transfer, even against a fully writable cacher,
// so the guard does not depend on the cacher wiring being right.
func TestBlockChainHook_TransferInReadOnlyMode_FailsClosed(t *testing.T) {
	t.Parallel()

	userDB, kappDB, peerDB := readOnlySeedCommittedTries(t)

	prodCacher, err := state.NewAccountsCacher(readOnlyArgsOver(userDB, kappDB, peerDB))
	require.NoError(t, err)
	prodCacher.ResetAll(true)

	hookImpl := &BlockChainHookImpl{kappController: readOnlyBuildController(t, prodCacher, true)}

	err = hookImpl.TransferValueOnly(testToAddress, testOwnerAddress, big.NewInt(1))
	require.Error(t, err)
	require.ErrorContains(t, err, process.ErrReadOnlyKAppMutation.Error())

	require.Equal(t, readOnlyOwnerStartBalance, readOnlyCommittedKLV(t, userDB, kappDB, peerDB, testOwnerAddress),
		"read-only mode must refuse the transfer, leaving committed state untouched")
}

////////////////////////////////////////
// ProcessBuiltInFunction dispatch     //
////////////////////////////////////////

// builtInSCAddress builds an address IsSmartContractAddress accepts, so the
// receipt -> OutputTransfer loop in ProcessBuiltInFunction is reachable.
func builtInSCAddress(suffix string) []byte {
	addr := make([]byte, 0, 32)
	addr = append(addr, make([]byte, core.NumInitCharactersForScAddress-core.VMTypeLen)...)
	addr = append(addr, common.WasmVirtualMachine...)
	addr = append(addr, []byte(suffix)...)

	return append(addr, make([]byte, 32-len(addr))...)
}

// transferReceipt builds a receipt in the layout ProcessBuiltInFunction decodes:
// [type][sender][recipient][value][token][nonce][tokenType].
func transferReceipt(sender, recipient []byte, value, token, nonce string, tokenType kapps.KDAData_EnumAssetType) *transaction.Transaction_Receipt {
	return &transaction.Transaction_Receipt{
		Data: [][]byte{
			{byte(pTX.Transfer)},
			sender,
			recipient,
			[]byte(value),
			[]byte(token),
			[]byte(nonce),
			{byte(tokenType)},
		},
	}
}

// dispatchHook wires a hook whose controller is writable, so the guard lets the
// built-in through and the rest of the function runs.
func dispatchHook(t *testing.T, ctx kapp.KappContext, fn func(*vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)) *BlockChainHookImpl {
	t.Helper()

	container := builtinfunctions.NewBuiltInFunctionContainer()
	require.NoError(t, container.Add("Transfer", &kvmContextMock.BuiltInFunctionStub{
		ProcessBuiltinFunctionCalled: fn,
	}))

	return &BlockChainHookImpl{
		builtInFunctions: container,
		counter:          counters.NewDisabledCounter(),
		kappController: &stub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
			IsReadOnlyCalled:            func() bool { return false },
		},
	}
}

func dispatchInput(caller []byte) *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput:       vmcommon.VMInput{CallerAddr: caller},
		RecipientAddr: []byte("recipient"),
		Function:      "Transfer",
	}
}

func newDispatchContext() kapp.KappContext {
	return kapp.NewKappContext(kapp.ArgsNewKAppContext{
		ContractID:   0,
		ContractType: transaction.TXContract_SmartContractType,
		Block:        createBlockHeader(),
	})
}

// A writable controller must let the built-in run, and its return data must be
// taken from the KApp context rather than from the built-in's own VMOutput.
func TestBlockChainHook_ProcessBuiltInFunction_Writable_DispatchesAndTakesReturnData(t *testing.T) {
	t.Parallel()

	ctx := newDispatchContext()
	ctx.AddReturnData([]byte("asset-id"))

	dispatched := false
	hook := dispatchHook(t, ctx, func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
		dispatched = true
		return &vmcommon.VMOutput{ReturnCode: vmcommon.Ok}, nil
	})

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput([]byte("not-a-contract")))
	require.NoError(t, err)
	require.True(t, dispatched, "a writable controller must dispatch the built-in")
	require.Equal(t, [][]byte{[]byte("asset-id")}, vmOutput.ReturnData)

	// GetAndClearReturnData must have drained the context.
	require.Empty(t, ctx.GetAndClearReturnData())
}

// A non-contract caller returns right after the return data is copied: receipts
// are not turned into output transfers.
func TestBlockChainHook_ProcessBuiltInFunction_NonContractCaller_SkipsTransfers(t *testing.T) {
	t.Parallel()

	ctx := newDispatchContext()
	hook := dispatchHook(t, ctx, func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
		ctx.Receipts().Add(transferReceipt(
			[]byte("sender"), []byte("recipient"), "5", "KLV", "0", kapps.KDAData_Fungible))
		return &vmcommon.VMOutput{}, nil
	})

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput([]byte("not-a-contract")))
	require.NoError(t, err)
	require.Empty(t, vmOutput.OutputAccounts,
		"only smart contract callers get receipts translated into output transfers")
}

// Receipts emitted by the built-in become OutputTransfers on the VM output, and
// only receipts added by THIS call are considered.
func TestBlockChainHook_ProcessBuiltInFunction_ContractCaller_BuildsOutputTransfers(t *testing.T) {
	t.Parallel()

	caller := builtInSCAddress("caller")
	recipient := []byte("recipient-addr")

	ctx := newDispatchContext()
	// A receipt from an earlier call must be ignored (initialLen bookkeeping).
	ctx.Receipts().Add(transferReceipt(
		[]byte("old-sender"), []byte("old-recipient"), "999", "OLD", "0", kapps.KDAData_Fungible))

	hook := dispatchHook(t, ctx, func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
		ctx.Receipts().Add(transferReceipt(
			[]byte("sender-addr"), recipient, "7", "KLV", "0", kapps.KDAData_Fungible))
		ctx.Receipts().Add(transferReceipt(
			[]byte("sender-addr"), recipient, "3", "NFT-1", "42", kapps.KDAData_NonFungible))
		return &vmcommon.VMOutput{}, nil
	})

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput(caller))
	require.NoError(t, err)

	outAcc, ok := vmOutput.OutputAccounts[string(recipient)]
	require.True(t, ok, "an output account must be created for the transfer recipient")
	require.Len(t, outAcc.OutputTransfers, 2, "the pre-existing receipt must not be included")

	fungible := outAcc.OutputTransfers[0]
	require.Equal(t, uint32(1), fungible.Index)
	require.Equal(t, []byte("sender-addr"), fungible.SenderAddress)
	require.Equal(t, big.NewInt(7), fungible.KDATransfers.KDAValue)
	require.Equal(t, []byte("KLV"), fungible.KDATransfers.KDATokenName)
	require.Equal(t, uint64(0), fungible.KDATransfers.KDATokenNonce,
		"a fungible transfer must not parse the nonce field")

	nonFungible := outAcc.OutputTransfers[1]
	require.Equal(t, uint32(2), nonFungible.Index)
	require.Equal(t, uint64(42), nonFungible.KDATransfers.KDATokenNonce,
		"a non-fungible transfer must carry its nonce")
}

// Receipts that are not transfers, or that carry no data, are skipped instead of
// being mistaken for transfers.
func TestBlockChainHook_ProcessBuiltInFunction_SkipsNonTransferReceipts(t *testing.T) {
	t.Parallel()

	ctx := newDispatchContext()
	hook := dispatchHook(t, ctx, func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
		ctx.Receipts().Add(&transaction.Transaction_Receipt{Data: [][]byte{}})
		ctx.Receipts().Add(&transaction.Transaction_Receipt{Data: [][]byte{{}}})
		ctx.Receipts().AddError(0, "boom")
		return &vmcommon.VMOutput{}, nil
	})

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput(builtInSCAddress("caller")))
	require.NoError(t, err)
	require.Empty(t, vmOutput.OutputAccounts)
}

// A malformed transfer receipt must surface as an error rather than producing a
// half-built output transfer.
func TestBlockChainHook_ProcessBuiltInFunction_MalformedTransferReceipts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		receipt *transaction.Transaction_Receipt
	}{
		{
			name:    "truncated data",
			receipt: &transaction.Transaction_Receipt{Data: [][]byte{{byte(pTX.Transfer)}, []byte("s"), []byte("r")}},
		},
		{
			name: "unparsable value",
			receipt: transferReceipt(
				[]byte("sender"), []byte("recipient"), "not-a-number", "KLV", "0", kapps.KDAData_Fungible),
		},
		{
			name: "missing asset type",
			receipt: &transaction.Transaction_Receipt{Data: [][]byte{
				{byte(pTX.Transfer)}, []byte("s"), []byte("r"), []byte("1"), []byte("KLV"), []byte("0"), {},
			}},
		},
		{
			name: "unparsable nonce on non-fungible",
			receipt: transferReceipt(
				[]byte("sender"), []byte("recipient"), "1", "NFT", "not-a-nonce", kapps.KDAData_NonFungible),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := newDispatchContext()
			hook := dispatchHook(t, ctx, func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
				ctx.Receipts().Add(tt.receipt)
				return &vmcommon.VMOutput{}, nil
			})

			vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput(builtInSCAddress("caller")))
			require.Error(t, err)
			require.Nil(t, vmOutput)
		})
	}
}

// An unknown function must report that, not the read-only error: the guard sits
// after the container lookup precisely so this diagnostic is preserved.
func TestBlockChainHook_ProcessBuiltInFunction_UnknownFunction(t *testing.T) {
	t.Parallel()

	hook := &BlockChainHookImpl{
		builtInFunctions: builtinfunctions.NewBuiltInFunctionContainer(),
		counter:          counters.NewDisabledCounter(),
		kappController: &stub.KAppControllerStub{
			IsReadOnlyCalled: func() bool { return true },
		},
	}

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput([]byte("caller")))
	require.Error(t, err)
	require.NotErrorIs(t, err, process.ErrReadOnlyKAppMutation,
		"an unknown function must not be masked by the read-only guard")
	require.Nil(t, vmOutput)
}

// A nil input is rejected before anything else is touched.
func TestBlockChainHook_ProcessBuiltInFunction_NilInput(t *testing.T) {
	t.Parallel()

	hook := &BlockChainHookImpl{}

	vmOutput, err := hook.ProcessBuiltInFunction(nil)
	require.ErrorIs(t, err, process.ErrNilVmInput)
	require.Nil(t, vmOutput)
}

// An error from the built-in itself propagates unchanged.
func TestBlockChainHook_ProcessBuiltInFunction_BuiltInError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("built-in blew up")
	hook := dispatchHook(t, newDispatchContext(), func(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
		return nil, expectedErr
	})

	vmOutput, err := hook.ProcessBuiltInFunction(dispatchInput([]byte("caller")))
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, vmOutput)
}
