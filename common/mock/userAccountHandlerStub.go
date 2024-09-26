package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

var _ state.UserAccountHandler = (*UserAccountHandlerStub)(nil)

// UserAccountHandlerStub -
type UserAccountHandlerStub struct {
	AddressData              []byte
	SetRootHashCalled        func([]byte)
	GetRootHashCalled        func() []byte
	SetNameCalled            func(userName []byte)
	GetNameCalled            func() []byte
	StartProposalsKAppCalled func(forks core.ForkController) (kapps.ActiveProposalController, error)
	GetNonceCalled           func() uint64
	IncreaseNonceCalled      func(nonce uint64)
	AddInternalKDACalled     func(assetID []byte, internalID []byte, data []byte) error
	SubInternalKDACalled     func(assetID []byte, internalID []byte) ([]byte, error)
	SetDataTrieCalled        func(trie data.Trie)
	DataTrieCalled           func() data.Trie
	DataTrieTrackerCalled    func() state.DataTrieTracker
	GetUserKDACalled         func(assetID []byte, nonce []byte, _ bool) (*kapps.UserKDA, error)
	GetBucketsCalled         func(assetID []byte, cdd bool) map[string]*kapps.UserBucket
}

// AddToBalance implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) AddToBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
	panic("unimplemented")
}

// AddToBalanceWithNonce implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) AddToBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	panic("unimplemented")
}

// AddressBytes implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) AddressBytes() []byte {
	return k.AddressData
}

// Claim implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Claim(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
	panic("unimplemented")
}

// ComputeAvailableClaim implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	panic("unimplemented")
}

// Delegate implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Delegate(bucketID []byte, delegation []byte, userKDA *kapps.UserKDA) (int64, error) {
	panic("unimplemented")
}

// Equal implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Equal(state.UserAccountHandler) bool {
	panic("unimplemented")
}

// Freeze implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Freeze(assetID []byte, bucketID []byte, value int64, blockEpoch uint32, blockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) error {
	panic("unimplemented")
}

// GetAllowance implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetAllowance() int64 {
	panic("unimplemented")
}

// GetBalance implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetBalance(assetID []byte, cdd bool) int64 {
	panic("unimplemented")
}

// GetBalanceWithNonce implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetBalanceWithNonce(assetID []byte, nonce []byte, cdd bool) int64 {
	panic("unimplemented")
}

// GetBuckets implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetBuckets(assetID []byte, cdd bool) map[string]*kapps.UserBucket {
	if k.GetBucketsCalled != nil {
		return k.GetBucketsCalled(assetID, cdd)
	}

	return nil
}

// GetCodeHash implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetCodeHash() []byte {
	panic("unimplemented")
}

// GetCodeMetadata implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetCodeMetadata() []byte {
	panic("unimplemented")
}

// GetFrozenBalance implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetFrozenBalance(assetID []byte, cdd bool) int64 {
	panic("unimplemented")
}

// GetOwnerAddress implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetOwnerAddress() []byte {
	panic("unimplemented")
}

// GetPermission implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetPermission(id int32) (*state.Permission, bool, error) {
	panic("unimplemented")
}

// GetPermissions implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) GetPermissions() []*state.Permission {
	panic("unimplemented")
}

// HasNewCode implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) HasNewCode() bool {
	panic("unimplemented")
}

// IsInterfaceNil implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) IsInterfaceNil() bool {
	panic("unimplemented")
}

// RetrieveValue implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) RetrieveValue(key []byte) ([]byte, error) {
	panic("unimplemented")
}

// SaveKeyValue implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SaveKeyValue(key []byte, value []byte) error {
	panic("unimplemented")
}

// SetCode implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetCode(code []byte) {
	panic("unimplemented")
}

// SetCodeHash implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetCodeHash([]byte) {
	panic("unimplemented")
}

// SetCodeMetadata implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetCodeMetadata(codeMetadata []byte) {
	panic("unimplemented")
}

// SetOwnerAddress implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetOwnerAddress([]byte) {
	panic("unimplemented")
}

// SetPermissions implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetPermissions([]*state.Permission) {
	panic("unimplemented")
}

// SetUserKDA implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
	panic("unimplemented")
}

// SubFromBalance implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SubFromBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
	panic("unimplemented")
}

// SubFromBalanceWithNonce implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) SubFromBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	panic("unimplemented")
}

// Undelegate implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
	panic("unimplemented")
}

// Unfreeze implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Unfreeze(assetID []byte, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
	panic("unimplemented")
}

// Withdraw implements state.UserAccountHandler.
func (k *UserAccountHandlerStub) Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error) {
	panic("unimplemented")
}

func (k *UserAccountHandlerStub) GetUserKDA(assetID []byte, nonce []byte, _ bool) (*kapps.UserKDA, error) {
	if k.GetUserKDACalled != nil {
		return k.GetUserKDACalled(assetID, nonce, true)
	}

	return nil, nil
}

func (k *UserAccountHandlerStub) GetStorage(key []byte) []byte {
	return nil
}

func (k *UserAccountHandlerStub) SetStorage(key []byte, value []byte) error {
	return nil
}

// SetRootHash -
func (k *UserAccountHandlerStub) SetRootHash(rootHash []byte) {
	if k.SetRootHashCalled != nil {
		k.SetRootHashCalled(rootHash)
	}
}

// GetRootHash -
func (k *UserAccountHandlerStub) GetRootHash() []byte {
	if k.GetRootHashCalled != nil {
		return k.GetRootHashCalled()
	}
	return []byte("roothash")
}

// SetName -
func (k *UserAccountHandlerStub) SetName(name []byte) {
	if k.SetNameCalled != nil {
		k.SetNameCalled(name)
	}
}

// GetName -
func (k *UserAccountHandlerStub) GetName() []byte {
	if k.GetNameCalled != nil {
		return k.GetNameCalled()
	}
	return []byte("roothash")
}

// StartProposalsKApp -
func (k *UserAccountHandlerStub) StartProposalsKApp(forks core.ForkController) (kapps.ActiveProposalController, error) {
	if k.StartProposalsKAppCalled != nil {
		return k.StartProposalsKAppCalled(forks)
	}

	return kapps.NewProposalController(forks)
}

// GetNonce -
func (k *UserAccountHandlerStub) GetNonce() uint64 {
	if k.GetNonceCalled != nil {
		return k.GetNonceCalled()
	}
	return uint64(0)
}

// IncreaseNonce -
func (k *UserAccountHandlerStub) IncreaseNonce(nonce uint64) {
	if k.IncreaseNonceCalled != nil {
		k.IncreaseNonceCalled(nonce)
	}
}

// AddInternalKDA -
func (k *UserAccountHandlerStub) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	if k.AddInternalKDACalled != nil {
		return k.AddInternalKDACalled(assetID, internalID, data)
	}
	return nil
}

// SubInternalKDA -
func (k *UserAccountHandlerStub) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	if k.SubInternalKDACalled != nil {
		return k.SubInternalKDACalled(assetID, internalID)
	}
	return []byte("kda"), nil
}

// SetDataTrie -
func (k *UserAccountHandlerStub) SetDataTrie(tr data.Trie) {
	if k.SetDataTrieCalled != nil {
		k.SetDataTrieCalled(tr)
	}
}

// DataTrie -
func (k *UserAccountHandlerStub) DataTrie() data.Trie {
	if k.DataTrieCalled != nil {
		return k.DataTrieCalled()
	}

	return nil
}

// DataTrieTracker -
func (k *UserAccountHandlerStub) DataTrieTracker() state.DataTrieTracker {
	if k.DataTrieTrackerCalled != nil {
		return k.DataTrieTrackerCalled()
	}

	return nil
}

func (k *UserAccountHandlerStub) AccountDataHandler() state.AccountDataHandler {
	return nil
}

func (k *UserAccountHandlerStub) AddToAllowance(int64) error {
	return nil
}
