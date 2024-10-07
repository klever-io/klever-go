package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

type UserAccountHandlerStub struct {
	SetCodeCalled                 func(code []byte)
	SetCodeMetadataCalled         func(codeMetadata []byte)
	GetCodeMetadataCalled         func() []byte
	SetCodeHashCalled             func([]byte)
	GetCodeHashCalled             func() []byte
	GetOwnerAddressCalled         func() []byte
	SetOwnerAddressCalled         func([]byte)
	SaveKeyValueCalled            func(key []byte, value []byte) error
	RetrieveValueCalled           func(key []byte) ([]byte, error)
	GetNonceCalled                func() uint64
	GetUserKDACalled              func(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error)
	SetUserKDACalled              func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error
	SetRootHashCalled             func([]byte)
	GetRootHashCalled             func() []byte
	AddToBalanceCalled            func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error
	AddToBalanceWithNonceCalled   func(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error
	SubFromBalanceCalled          func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error
	SubFromBalanceWithNonceCalled func(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error
	AddInternalKDACalled          func(assetID []byte, internalID []byte, data []byte) error
	SubInternalKDACalled          func(assetID []byte, internalID []byte) ([]byte, error)
	AddToAllowanceCalled          func(value int64) error
	GetBalanceCalled              func(assetID []byte, cdd bool) int64
	GetBalanceWithNonceCalled     func(assetID []byte, nonce []byte, cdd bool) int64
	GetAllowanceCalled            func() int64
	GetFrozenBalanceCalled        func(assetID []byte, cdd bool) int64
	GetBucketsCalled              func(assetID []byte, cdd bool) map[string]*kapps.UserBucket
	FreezeCalled                  func(assetID, bucketID []byte, value int64, blockEpoch uint32, blockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) error
	UnfreezeCalled                func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error)
	DelegateCalled                func(bucketID, delegation []byte, userKDA *kapps.UserKDA) (int64, error)
	UndelegateCalled              func(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error)
	ClaimCalled                   func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error)
	ComputeAvailableClaimCalled   func(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error)
	WithdrawCalled                func(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error)
	GetPermissionCalled           func(id int32) (*state.Permission, bool, error)
	SetPermissionsCalled          func([]*state.Permission)
	GetPermissionsCalled          func() []*state.Permission
	SetNameCalled                 func(name []byte)
	GetNameCalled                 func() []byte
	SetDataTrieCalled             func(trie data.Trie)
	DataTrieCalled                func() data.Trie
	HasNewCodeCalled              func() bool
	DataTrieTrackerCalled         func() state.DataTrieTracker
	AccountDataHandlerCalled      func() state.AccountDataHandler
	EqualCalled                   func(state.UserAccountHandler) bool
	state.AccountHandler
}

// Methods

func (u *UserAccountHandlerStub) SetCode(code []byte) {
	if u.SetCodeCalled != nil {
		u.SetCodeCalled(code)
	}
}

func (u *UserAccountHandlerStub) SetCodeMetadata(codeMetadata []byte) {
	if u.SetCodeMetadataCalled != nil {
		u.SetCodeMetadataCalled(codeMetadata)
	}
}

func (u *UserAccountHandlerStub) GetCodeMetadata() []byte {
	if u.GetCodeMetadataCalled != nil {
		return u.GetCodeMetadataCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) SetCodeHash(hash []byte) {
	if u.SetCodeHashCalled != nil {
		u.SetCodeHashCalled(hash)
	}
}

func (u *UserAccountHandlerStub) GetCodeHash() []byte {
	if u.GetCodeHashCalled != nil {
		return u.GetCodeHashCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) GetOwnerAddress() []byte {
	if u.GetOwnerAddressCalled != nil {
		return u.GetOwnerAddressCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) SetOwnerAddress(ownerAddress []byte) {
	if u.SetOwnerAddressCalled != nil {
		u.SetOwnerAddressCalled(ownerAddress)
	}
}

func (u *UserAccountHandlerStub) SaveKeyValue(key []byte, value []byte) error {
	if u.SaveKeyValueCalled != nil {
		return u.SaveKeyValueCalled(key, value)
	}
	return nil
}

func (u *UserAccountHandlerStub) RetrieveValue(key []byte) ([]byte, error) {
	if u.RetrieveValueCalled != nil {
		return u.RetrieveValueCalled(key)
	}
	return nil, nil
}

func (u *UserAccountHandlerStub) GetNonce() uint64 {
	if u.GetNonceCalled != nil {
		return u.GetNonceCalled()
	}
	return 0
}

func (u *UserAccountHandlerStub) GetUserKDA(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
	if u.GetUserKDACalled != nil {
		return u.GetUserKDACalled(assetID, nonce, checkDirtData)
	}
	return nil, nil
}

func (u *UserAccountHandlerStub) SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
	if u.SetUserKDACalled != nil {
		return u.SetUserKDACalled(assetID, nonce, userKDA)
	}
	return nil
}

func (u *UserAccountHandlerStub) SetRootHash(rootHash []byte) {
	if u.SetRootHashCalled != nil {
		u.SetRootHashCalled(rootHash)
	}
}

func (u *UserAccountHandlerStub) GetRootHash() []byte {
	if u.GetRootHashCalled != nil {
		return u.GetRootHashCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) AddToBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
	if u.AddToBalanceCalled != nil {
		return u.AddToBalanceCalled(value, assetID, cdd, userKDA...)
	}
	return nil
}

func (u *UserAccountHandlerStub) AddToBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if u.AddToBalanceWithNonceCalled != nil {
		return u.AddToBalanceWithNonceCalled(value, assetID, nonce, cdd, userKDAopts...)
	}
	return nil
}

func (u *UserAccountHandlerStub) SubFromBalance(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
	if u.SubFromBalanceCalled != nil {
		return u.SubFromBalanceCalled(value, assetID, cdd, userKDA...)
	}
	return nil
}

func (u *UserAccountHandlerStub) SubFromBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if u.SubFromBalanceWithNonceCalled != nil {
		return u.SubFromBalanceWithNonceCalled(value, assetID, nonce, cdd, userKDAopts...)
	}
	return nil
}

func (u *UserAccountHandlerStub) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	if u.AddInternalKDACalled != nil {
		return u.AddInternalKDACalled(assetID, internalID, data)
	}
	return nil
}

func (u *UserAccountHandlerStub) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	if u.SubInternalKDACalled != nil {
		return u.SubInternalKDACalled(assetID, internalID)
	}
	return nil, nil
}

func (u *UserAccountHandlerStub) AddToAllowance(value int64) error {
	if u.AddToAllowanceCalled != nil {
		return u.AddToAllowanceCalled(value)
	}
	return nil
}

func (u *UserAccountHandlerStub) GetBalance(assetID []byte, cdd bool) int64 {
	if u.GetBalanceCalled != nil {
		return u.GetBalanceCalled(assetID, cdd)
	}
	return 0
}

func (u *UserAccountHandlerStub) GetBalanceWithNonce(assetID []byte, nonce []byte, cdd bool) int64 {
	if u.GetBalanceWithNonceCalled != nil {
		return u.GetBalanceWithNonceCalled(assetID, nonce, cdd)
	}
	return 0
}

func (u *UserAccountHandlerStub) GetAllowance() int64 {
	if u.GetAllowanceCalled != nil {
		return u.GetAllowanceCalled()
	}
	return 0
}

func (u *UserAccountHandlerStub) GetFrozenBalance(assetID []byte, cdd bool) int64 {
	if u.GetFrozenBalanceCalled != nil {
		return u.GetFrozenBalanceCalled(assetID, cdd)
	}
	return 0
}

func (u *UserAccountHandlerStub) GetBuckets(assetID []byte, cdd bool) map[string]*kapps.UserBucket {
	if u.GetBucketsCalled != nil {
		return u.GetBucketsCalled(assetID, cdd)
	}
	return nil
}

func (u *UserAccountHandlerStub) Freeze(assetID, bucketID []byte, value int64, blockEpoch uint32, blockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) error {
	if u.FreezeCalled != nil {
		return u.FreezeCalled(assetID, bucketID, value, blockEpoch, blockTime, staking, userKDA, newStakingFlow)
	}
	return nil
}

func (u *UserAccountHandlerStub) Unfreeze(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
	if u.UnfreezeCalled != nil {
		return u.UnfreezeCalled(assetID, bucketID, blockEpoch, staking, userKDA, newStakingFlow)
	}
	return nil, 0, nil
}

func (u *UserAccountHandlerStub) Delegate(bucketID, delegation []byte, userKDA *kapps.UserKDA) (int64, error) {
	if u.DelegateCalled != nil {
		return u.DelegateCalled(bucketID, delegation, userKDA)
	}
	return 0, nil
}

func (u *UserAccountHandlerStub) Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
	if u.UndelegateCalled != nil {
		return u.UndelegateCalled(bucketID, userKDA)
	}
	return nil, 0, nil
}

func (u *UserAccountHandlerStub) Claim(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
	if u.ClaimCalled != nil {
		return u.ClaimCalled(claimType, assetID, epoch, blockTime, staking, kda, userKDA, forkController)
	}
	return nil, nil
}

func (u *UserAccountHandlerStub) ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	if u.ComputeAvailableClaimCalled != nil {
		return u.ComputeAvailableClaimCalled(assetID, epoch, blockTime, userKDA, staking, forkController)
	}
	return nil, nil
}

func (u *UserAccountHandlerStub) Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error) {
	if u.WithdrawCalled != nil {
		return u.WithdrawCalled(assetID, epoch, minEpochsToWithdraw, userKDA)
	}
	return 0, nil
}

func (u *UserAccountHandlerStub) GetPermission(id int32) (*state.Permission, bool, error) {
	if u.GetPermissionCalled != nil {
		return u.GetPermissionCalled(id)
	}
	return nil, false, nil
}

func (u *UserAccountHandlerStub) SetPermissions(permissions []*state.Permission) {
	if u.SetPermissionsCalled != nil {
		u.SetPermissionsCalled(permissions)
	}
}

func (u *UserAccountHandlerStub) GetPermissions() []*state.Permission {
	if u.GetPermissionsCalled != nil {
		return u.GetPermissionsCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) SetName(name []byte) {
	if u.SetNameCalled != nil {
		u.SetNameCalled(name)
	}
}

func (u *UserAccountHandlerStub) GetName() []byte {
	if u.GetNameCalled != nil {
		return u.GetNameCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) SetDataTrie(trie data.Trie) {
	if u.SetDataTrieCalled != nil {
		u.SetDataTrieCalled(trie)
	}
}

func (u *UserAccountHandlerStub) DataTrie() data.Trie {
	if u.DataTrieCalled != nil {
		return u.DataTrieCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) HasNewCode() bool {
	if u.HasNewCodeCalled != nil {
		return u.HasNewCodeCalled()
	}
	return false
}

func (u *UserAccountHandlerStub) DataTrieTracker() state.DataTrieTracker {
	if u.DataTrieTrackerCalled != nil {
		return u.DataTrieTrackerCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) AccountDataHandler() state.AccountDataHandler {
	if u.AccountDataHandlerCalled != nil {
		return u.AccountDataHandlerCalled()
	}
	return nil
}

func (u *UserAccountHandlerStub) Equal(other state.UserAccountHandler) bool {
	if u.EqualCalled != nil {
		return u.EqualCalled(other)
	}
	return false
}
func (u *UserAccountHandlerStub) AddressBytes() []byte {
	return []byte{0}
}
