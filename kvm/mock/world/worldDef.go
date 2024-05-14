package worldmock

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"

	"github.com/klever-io/klever-go/data/state/factory"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

// NewAddressMock allows tests to specify what new addresses to generate
type NewAddressMock struct {
	CreatorAddress []byte
	CreatorNonce   uint64
	NewAddress     []byte
}

// BlockInfo contains metadata about a mocked block
type BlockInfo struct {
	BlockTimestamp int64
	BlockNonce     uint64
	BlockSlot      uint64
	BlockEpoch     uint32
	RandomSeed     *[48]byte
}

// GetRandomSeedSlice retrieves the configured random seed or a slice of zeros.
// Always 48 bytes long, never nil.
func (bi *BlockInfo) GetRandomSeedSlice() []byte {
	if bi.RandomSeed == nil {
		bi.RandomSeed = &[48]byte{}
	}
	return bi.RandomSeed[:]
}

// MockWorld provides a mock representation of the blockchain to be used in VM tests.
type MockWorld struct {
	AccountsAdapter            state.AccountsAdapter
	KAppsAdapter               state.AccountsAdapter
	PeersAdapter               state.AccountsAdapter
	AccountsCacher             state.AccountsCacher
	PreviousBlockInfo          *BlockInfo
	CurrentBlockInfo           *BlockInfo
	Blockhashes                [][]byte
	NewAddressMocks            []*NewAddressMock
	StateRootHash              []byte
	Err                        error
	LastCreatedContractAddress []byte
	CompiledCode               map[string][]byte
	BuiltinFuncs               *BuiltinFunctionsWrapper
	ProvidedBlockchainHook     vmcommon.BlockchainHook
	OtherVMOutputMap           map[string]*vmcommon.VMOutput
	MockAsset                  *kapps.KDAData
}

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

func createAccountsDB(fact state.AccountFactory) *state.AccountsDB {
	hasher := &sha256.Sha256{}
	trieStorageManager, _ := trie.NewTrieStorageManagerWithoutPruning(createMemUnit())
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, fact, core.Normal)
	return adb
}

// NewMockWorld creates a new MockWorld instance
func NewMockWorld() *MockWorld {
	world := &MockWorld{}
	world.Clear() // reuse clear method to ensure same initialization on every test
	return world
}

func (b *MockWorld) CreateAccount(address []byte, world *MockWorld) *Account {
	newAccount := &Account{
		Exists:          true,
		Address:         make([]byte, len(address)),
		Nonce:           0,
		Balance:         big.NewInt(0),
		Storage:         make(map[string][]byte),
		Code:            nil,
		OwnerAddress:    nil,
		IsSmartContract: false,
		MockWorld:       world,
	}
	copy(newAccount.Address, address)
	b.PutAccount(newAccount)

	return newAccount
}

func (b *MockWorld) CreateSmartContractAccount(owner []byte, address []byte, code []byte, world *MockWorld) *Account {
	return b.CreateSmartContractAccountWithCodeHash(owner, address, code, nil, world)
}

func (b *MockWorld) CreateSmartContractAccountWithCodeHash(owner []byte, address []byte, code []byte, codeHash []byte, world *MockWorld) *Account {
	newAccount := b.CreateAccount(address, world)
	newAccount.Code = code
	if codeHash == nil {
		codeHash = code
	}
	newAccount.CodeHash = codeHash
	newAccount.IsSmartContract = true
	newAccount.OwnerAddress = owner

	metadata := &vmcommon.CodeMetadata{
		Payable: true,
	}

	newAccount.SetCodeAndMetadata(code, metadata)

	return newAccount
}

func convertMockWorldAccount(testAcct *Account) (state.UserAccountHandler, error) {
	acct := state.NewUserAccountNoCheck(testAcct.Address)

	acct.Nonce = testAcct.Nonce
	if testAcct.Balance != nil {
		acct.UserAccountData.Balance = testAcct.Balance.Int64()
	}
	acct.UserAccountData.Name = testAcct.Username
	acct.SetCode(testAcct.Code)
	acct.SetCodeMetadata(testAcct.CodeMetadata)
	acct.OwnerAddress = testAcct.OwnerAddress

	for key, value := range testAcct.Storage {
		err := acct.DataTrieTracker().SaveKeyValue([]byte(key), value)
		if err != nil {
			return nil, err
		}
	}

	return acct, nil
}

func (b *MockWorld) ExtractAccountStorage(acc state.UserAccountHandler) map[string][]byte {
	userStorage := make(map[string][]byte)
	// restore storage from trie
	if acc.DataTrie() != nil {
		leavesChannel, err := acc.DataTrie().GetAllLeavesOnChannel(acc.GetRootHash(), context.Background())
		if err == nil {
			for leaf := range leavesChannel {
				// remove key suffix from value
				suffix := leaf.Key()
				lenValue := len(leaf.Value())
				position := bytes.Index(leaf.Value(), suffix)
				if position > lenValue {
					userStorage[string(leaf.Key())] = leaf.Value()
					continue
				}
				userStorage[string(leaf.Key())] = leaf.Value()[:position]
			}
		}
	}
	return userStorage
}

func (b *MockWorld) ConvertAccountToWorldMock(acc state.UserAccountHandler) *Account {
	return &Account{
		Exists:  acc != nil,
		Address: acc.AddressBytes(),
		Nonce:   acc.GetNonce(),
		Balance: big.NewInt(acc.GetBalance(nil, true)),
		Storage: b.ExtractAccountStorage(acc),
		//Code:            acc.GetCode(),
		CodeMetadata:    acc.GetCodeMetadata(),
		OwnerAddress:    acc.GetOwnerAddress(),
		IsSmartContract: len(acc.GetCodeHash()) > 0,
		MockWorld:       nil,
	}
}

func (b *MockWorld) PutAccount(acct *Account) {
	acctHandler, _ := convertMockWorldAccount(acct)

	_ = b.AccountsCacher.SaveUser(acctHandler)
}

func (b MockWorld) PutAccounts(accounts []*Account) {
	for _, account := range accounts {
		b.PutAccount(account)
	}
}

func (b *MockWorld) GetAccount(address []byte) *Account {
	acctHandler, _ := b.AccountsCacher.LoadUser(address)

	return b.ConvertAccountToWorldMock(acctHandler)
}

func (b *MockWorld) CreateTestAssets() {
	KDATestTokenName := []byte("TTT-0101")
	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	kfiKey := kdautils.ToKDAKey(kdautils.KFIIdentifier, nil)
	aprKey := kdautils.ToKDAKey(KDATestTokenName, nil)

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
		ID:                KDATestTokenName,
		AssetType:         kapps.KDAData_Fungible,
		Name:              []byte("TTT TEST"),
		Ticker:            []byte("TTT"),
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

	kdaKappAccount, err := b.KAppsAdapter.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		panic(err)
	}
	kdaKapp, ok := kdaKappAccount.(state.KAppAccountHandler)
	if !ok {
		panic("KDA Kapp account not found")
	}

	_ = kdaKapp.DataTrieTracker().SaveKeyValue(klvKey, klvData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(kfiKey, kfiData)
	_ = kdaKapp.DataTrieTracker().SaveKeyValue(aprKey, aprData)

	err = b.KAppsAdapter.SaveAccount(kdaKappAccount)
	if err != nil {
		panic(err)
	}
}

func (b *MockWorld) GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error) {
	return &kapps.MetaV2{}, nil
}

func (b *MockWorld) MockTestAsset(asset *scenjsonmodel.KDAData) {
	kdaKappAccount, err := b.KAppsAdapter.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		panic(err)
	}
	kdaKapp, ok := kdaKappAccount.(state.KAppAccountHandler)
	if !ok {
		panic("KDA Kapp account not found")
	}

	assetId := asset.TokenIdentifier.Value

	for _, instance := range asset.Instances {
		assetType := kapps.KDAData_Fungible
		if instance.Nonce.Value != 0 {
			assetType = kapps.KDAData_NonFungible
		}

		nonce := []byte(strconv.FormatUint(instance.Nonce.Value, 10))

		tokenKey := kdautils.ToKDAKey(assetId, nonce)
		parentTokenKey := kdautils.ToKDAKey(assetId, nil)

		token := kapps.KDAData{
			ID:                assetId,
			AssetType:         assetType,
			Name:              assetId,
			Ticker:            assetId,
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

		tokenData, _ := marshalizer.Marshal(&token)

		_ = kdaKapp.DataTrieTracker().SaveKeyValue(tokenKey, tokenData)
		_ = kdaKapp.DataTrieTracker().SaveKeyValue(parentTokenKey, tokenData)
	}
	err = b.KAppsAdapter.SaveAccount(kdaKappAccount)
	if err != nil {
		panic(err)
	}
}

func (b *MockWorld) GetKDAData(tokenIdentifier []byte, nonce []byte) (*kapps.KDAData, error) {
	tokenKey := kdautils.ToKDAKey(tokenIdentifier, nonce)

	// load kdaKapp
	kdaKappAccount, err := b.KAppsAdapter.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}
	kdaKapp, ok := kdaKappAccount.(state.KAppAccountHandler)
	if !ok {
		return nil, fmt.Errorf("KDA Kapp account not found")
	}

	// retrieve token info
	kdaRaw, err := kdaKapp.DataTrieTracker().RetrieveValue(tokenKey)
	if err != nil {
		return nil, err
	}
	kda := kapps.KDAData{}
	err = marshalizer.Unmarshal(&kda, kdaRaw)
	if err != nil {
		return nil, err
	}
	return &kda, nil
}

func (b *MockWorld) SetKDAData(tokenIdentifier []byte, nonce []byte, data *kapps.KDAData) error {
	tokenKey := kdautils.ToKDAKey(tokenIdentifier, nonce)

	// load kdaKapp
	kdaKappAccount, err := b.KAppsAdapter.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return err
	}
	kdaKapp, ok := kdaKappAccount.(state.KAppAccountHandler)
	if !ok {
		return fmt.Errorf("KDA Kapp account not found")
	}

	// set token info
	rawData, err := marshalizer.Marshal(data)
	if err != nil {
		return err
	}
	err = kdaKapp.DataTrieTracker().SaveKeyValue(tokenKey, rawData)
	if err != nil {
		return err
	}

	return b.KAppsAdapter.SaveAccount(kdaKappAccount)
}

// SetProvidedBlockchainHook -
func (b *MockWorld) SetProvidedBlockchainHook(bh vmcommon.BlockchainHook) {
	b.ProvidedBlockchainHook = bh
}

// InitBuiltinFunctions initializes the inner BuiltinFunctionsWrapper, required
// for calling builtin functions.
func (b *MockWorld) InitBuiltinFunctions(gasMap config.GasScheduleMap, forkController core.ForkController) error {
	wrapper, err := NewBuiltinFunctionsWrapper(b, gasMap, forkController)
	if err != nil {
		return err
	}

	b.BuiltinFuncs = wrapper
	return nil
}

// Clear resets all mock data between tests.
func (b *MockWorld) Clear() {
	b.AccountsAdapter = createAccountsDB(factory.NewAccountCreator())
	b.KAppsAdapter = createAccountsDB(factory.NewKAppAccountCreator())
	b.PeersAdapter = createAccountsDB(factory.NewPeerAccountCreator())
	b.PreviousBlockInfo = nil
	b.CurrentBlockInfo = nil
	b.Blockhashes = nil
	b.NewAddressMocks = nil
	b.CompiledCode = make(map[string][]byte)
	b.BuiltinFuncs = nil
	b.OtherVMOutputMap = make(map[string]*vmcommon.VMOutput)

	accCacher, err := NewMockWorldAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: b.AccountsAdapter,
			Kapps:    b.KAppsAdapter,
			Peers:    b.PeersAdapter,
		},
	)
	if err != nil {
		panic(err)
	}
	accCacher.ResetAll(true)

	b.AccountsCacher = accCacher

	b.CreateTestAssets()
	if err = b.CommitChangesToDB(); err != nil {
		panic(err)
	}
}

// SetCurrentBlockHash -
func (b *MockWorld) SetCurrentBlockHash(blockHash []byte) {
	if b.CurrentBlockInfo == nil {
		b.CurrentBlockInfo = &BlockInfo{}
	}
	b.Blockhashes = [][]byte{blockHash}
}

// GetSnapshot -
func (b *MockWorld) GetSnapshot() int {
	b.CreateStateBackup()
	return b.AccountsAdapter.JournalLen()
}

// RevertToSnapshot -
func (b *MockWorld) RevertToSnapshot(snapshot int) error {
	return b.AccountsAdapter.RevertToSnapshot(snapshot)
}

// ExecuteSmartContractCallOnOtherVM -
func (b *MockWorld) ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	vmType, err := vmcommon.ParseVMTypeFromContractAddress(input.RecipientAddr)
	if err != nil {
		return nil, err
	}
	vmOutput := b.OtherVMOutputMap[string(vmType)]
	if vmOutput == nil {
		return &vmcommon.VMOutput{}, nil
	}

	return vmOutput, nil
}
