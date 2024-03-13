package worldmock

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"

	"github.com/klever-io/klever-go/vmcommon"
)

// ErrOperationNotPermitted indicates an operation rejected due to insufficient
// permissions.
var ErrOperationNotPermitted = errors.New("operation not permitted")

// ErrInvalidAddressLength indicates an incorrect length given for an address.
var ErrInvalidAddressLength = errors.New("invalid address length")

// Account holds the account info
type Account struct {
	Exists          bool
	Address         []byte
	Nonce           uint64
	Balance         *big.Int
	Storage         map[string][]byte
	RootHash        []byte
	Code            []byte
	CodeHash        []byte
	CodeMetadata    []byte
	OwnerAddress    []byte
	CallData        string
	Username        []byte
	IsSmartContract bool
	MockWorld       *MockWorld
}

func (a *Account) GetUserKDA(assetID []byte, nonce []byte) (*kapps.UserKDA, error) {
	kdaData := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	// check assetID for NFT
	key := kdautils.ToKDAKey(assetID, nonce)

	value, ok := a.Storage[string(key)]
	if !ok {
		return nil, common.ErrAssetNotFound
	}
	if len(value) == 0 {
		return kdaData, nil
	}

	marshalizer := marshal.NewProtoMarshalizer()

	err := marshalizer.Unmarshal(kdaData, value)
	if err != nil {
		return nil, err
	}

	return kdaData, nil
}

func (a *Account) SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	marshalizer := marshal.NewProtoMarshalizer()

	data, err := marshalizer.Marshal(userKDA)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nonce)

	a.Storage[string(key)] = data

	return nil
}

// MOCK world is been refactor to use the new world, is is a mock only for SFT assets
func (a *Account) GetBalanceWithNonce(_ []byte, _ []byte) int64 {
	return 0
}

func (a *Account) AddToBalanceWithNonce(_ int64, _ []byte, _ []byte, _ ...*kapps.UserKDA) error {
	return fmt.Errorf("not implemented")
}

func (a *Account) SubFromBalanceWithNonce(_ int64, _ []byte, _ []byte, _ ...*kapps.UserKDA) error {
	return fmt.Errorf("not implemented")
}

func (a *Account) AddToBalance(value int64, assetID []byte, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return common.ErrInvalidValue
	}

	if assetID == nil || bytes.Equal(assetID, kdautils.KLVIdentifier) {
		newBalance := a.Balance.Add(a.Balance, big.NewInt(value))
		if newBalance.Cmp(big.NewInt(0)) < 0 {
			return common.ErrBalance
		}

		a.Balance = newBalance
		return nil
	}

	var userKDA *kapps.UserKDA
	var err error
	if len(userKDAopts) == 1 {
		userKDA = userKDAopts[0]
	}

	if userKDA == nil {
		userKDA, err = a.GetUserKDA(assetID, nil)
		if err != nil {
			return err
		}
	}

	newBalance := userKDA.Balance + value
	if newBalance < 0 {
		return common.ErrBalance
	}
	userKDA.Balance = newBalance

	return a.SetUserKDA(assetID, nil, userKDA)
}

func (a *Account) SetTokenBalanceUint64(tokenIdentifier []byte, nonce uint64, balance uint64) error {
	if balance < 0 {
		return common.ErrInvalidValue
	}

	if len(tokenIdentifier) == 0 ||
		bytes.Equal(tokenIdentifier, kdautils.KLVIdentifier) {
		a.Balance = big.NewInt(int64(balance))
		return nil
	}

	kda, err := a.GetUserKDA(tokenIdentifier, []byte(fmt.Sprintf("%d", nonce)))
	if err != nil {
		return err
	}

	kda.Balance = int64(balance)

	return a.SetUserKDA(tokenIdentifier, []byte(fmt.Sprintf("%d", nonce)), kda)
}

func (a *Account) GetTokenBalanceUint64(tokenIdentifier []byte, nonce uint64) (uint64, error) {
	kda, err := a.GetUserKDA(tokenIdentifier, []byte(fmt.Sprintf("%d", nonce)))
	if err != nil {
		return 0, err
	}

	return uint64(kda.Balance), nil
}

func (a *Account) SubFromBalance(value int64, assetID []byte, userKDA ...*kapps.UserKDA) error {
	//TODO implement me
	panic("implement me")
}

func (a *Account) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (a *Account) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) AddToAllowance(value int64) error {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetBalance(assetID []byte) int64 {
	return a.Balance.Int64()
}

func (a *Account) GetAllowance() int64 {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetFrozenBalance(assetID []byte) int64 {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetBuckets(assetID []byte) map[string]*kapps.UserBucket {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Freeze(assetID, bucketID []byte, value int64, blockEpoch uint32, blockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) error {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Unfreeze(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Delegate(bucketID, delegation []byte, userKDA *kapps.UserKDA) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Claim(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetPermission(id int32) (*state.Permission, bool, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) SetPermissions(permissions []*state.Permission) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetPermissions() []*state.Permission {
	//TODO implement me
	panic("implement me")
}

func (a *Account) SetName(name []byte) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) GetName() []byte {
	//TODO implement me
	panic("implement me")
}

func (a *Account) SetDataTrie(trie data.Trie) {
	//TODO implement me
	panic("implement me")
}

func (a *Account) DataTrie() data.Trie {
	acc, _ := a.MockWorld.AccountsCacher.LoadUser(a.Address)
	return acc.DataTrie()
}

func (a *Account) DataTrieTracker() state.DataTrieTracker {
	acc, _ := a.MockWorld.AccountsCacher.LoadUser(a.Address)
	return acc.DataTrieTracker()
}

var storageDefaultValue = make([]byte, 0)

// StorageValue yields the storage value for key, default 0
func (a *Account) StorageValue(key string) []byte {
	value, found := a.Storage[key]
	if !found {
		return storageDefaultValue
	}
	return value
}

// SetCodeAndMetadata changes the account code, as well as all fields depending on it:
// CodeHash, IsSmartContract, CodeMetadata.
// The code metadata must be given explicitly.
func (a *Account) SetCodeAndMetadata(code []byte, codeMetadata *vmcommon.CodeMetadata) {
	a.Code = code
	hash := DefaultHasher.Compute(string(a.Code))

	a.CodeHash = hash
	a.IsSmartContract = true
	a.CodeMetadata = codeMetadata.ToBytes()
}

// AddressBytes -
func (a *Account) AddressBytes() []byte {
	return a.Address
}

// GetNonce -
func (a *Account) GetNonce() uint64 {
	return a.Nonce
}

// GetCode -
func (a *Account) GetCode() []byte {
	return a.Code
}

// HasNewCode -
func (a *Account) HasNewCode() bool {
	return false
}

// GetCodeMetadata -
func (a *Account) GetCodeMetadata() []byte {
	return a.CodeMetadata
}

// GetCodeHash -
func (a *Account) GetCodeHash() []byte {
	return a.CodeHash
}

// GetRootHash -
func (a *Account) GetRootHash() []byte {
	return a.RootHash
}

// SetBalance -
func (a *Account) SetBalance(balance int64) {
	a.Balance = big.NewInt(balance)
}

// GetOwnerAddress -
func (a *Account) GetOwnerAddress() []byte {
	return a.OwnerAddress
}

// GetUserName -
func (a *Account) GetUserName() []byte {
	return a.Username
}

// IsInterfaceNil -
func (a *Account) IsInterfaceNil() bool {
	return a == nil
}

// SetCode -
func (a *Account) SetCode(code []byte) {
	a.Code = code
	a.CodeHash = DefaultHasher.Compute(string(code))
	a.IsSmartContract = true
}

// SetCodeMetadata -
func (a *Account) SetCodeMetadata(codeMetadata []byte) {
	a.CodeMetadata = codeMetadata
}

// SetCodeHash -
func (a *Account) SetCodeHash(hash []byte) {
	a.CodeHash = hash
}

// SetRootHash -
func (a *Account) SetRootHash(hash []byte) {
	a.RootHash = hash
}

// AccountDataHandler -
func (a *Account) AccountDataHandler() state.AccountDataHandler {
	return a
}

// ChangeOwnerAddress -
func (a *Account) ChangeOwnerAddress(sender []byte, newAddress []byte) error {
	if !bytes.Equal(sender, a.OwnerAddress) {
		return ErrOperationNotPermitted
	}
	if len(newAddress) != len(a.Address) {
		return ErrInvalidAddressLength
	}

	a.OwnerAddress = newAddress

	return nil
}

// SetOwnerAddress -
func (a *Account) SetOwnerAddress(address []byte) {
	a.OwnerAddress = address
}

// SetUserName -
func (a *Account) SetUserName(userName []byte) {
	a.Username = make([]byte, len(userName))
	copy(a.Username, userName)
}

// IncreaseNonce -
func (a *Account) IncreaseNonce(nonce uint64) {
	a.Nonce += nonce
}

// RetrieveValue -
func (a *Account) RetrieveValue(key []byte) ([]byte, error) {
	return a.Storage[string(key)], nil
}

// SaveKeyValue -
func (a *Account) SaveKeyValue(key []byte, value []byte) error {
	a.Storage[string(key)] = value
	if a.MockWorld == nil {
		return ErrNilWorldMock
	}
	a.MockWorld.CreateStateBackup()
	return nil
}

// ClearDataCaches -
func (a *Account) ClearDataCaches() {
}

// DirtyData -
func (a *Account) DirtyData() map[string][]byte {
	return a.Storage
}

// Clone -
func (a *Account) Clone() *Account {
	return &Account{
		Exists:          a.Exists,
		Address:         a.Address,
		Nonce:           a.Nonce,
		Balance:         big.NewInt(0).Set(a.Balance),
		Storage:         a.cloneStorage(),
		RootHash:        cloneBytes(a.RootHash),
		Code:            cloneBytes(a.Code),
		CodeHash:        cloneBytes(a.CodeHash),
		CodeMetadata:    cloneBytes(a.CodeMetadata),
		OwnerAddress:    cloneBytes(a.OwnerAddress),
		Username:        cloneBytes(a.Username),
		IsSmartContract: a.IsSmartContract,
		MockWorld:       a.MockWorld,
	}
}

func (a *Account) cloneStorage() map[string][]byte {
	clone := make(map[string][]byte, len(a.Storage))
	for key, value := range a.Storage {
		clone[key] = cloneBytes(value)
	}

	return clone
}

func (a *Account) Equal(b state.UserAccountHandler) bool {
	return reflect.DeepEqual(a, b)
}

func cloneBytes(b []byte) []byte {
	clone := make([]byte, len(b))
	copy(clone, b)
	return clone
}
