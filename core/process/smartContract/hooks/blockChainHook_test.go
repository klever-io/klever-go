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
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
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
