package mock

import (
	"reflect"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

var _ state.UserAccountHandler = (*AccountWrapMock)(nil)

// AccountWrapMock -
type AccountWrapMock struct {
	AccountWrapMockData
	dataTrie          data.Trie
	RootHash          []byte
	address           []byte
	trackableDataTrie state.DataTrieTracker
}

func (awm *AccountWrapMock) GetPermissions() []*state.Permission {
	return nil
}

// SetName -
func (awm *AccountWrapMock) SetName(_ []byte) {
}

// GetName -
func (awm *AccountWrapMock) GetName() []byte {
	return nil
}

// GetNonce -
func (awm *AccountWrapMock) GetNonce() uint64 {
	return 0
}

// IncreaseNonce -
func (awm *AccountWrapMock) IncreaseNonce(_ uint64) {
}

// GetUserKDA -
func (awm *AccountWrapMock) GetUserKDA(assetID []byte, nonce []byte) (*kapps.UserKDA, error) {
	return nil, nil
}

// SetUserKDA
func (awm *AccountWrapMock) SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
	return nil
}

// AddToBalance -
func (awm *AccountWrapMock) AddToBalance(_ int64, _ []byte, _ ...*kapps.UserKDA) error {
	return nil
}

func (awm *AccountWrapMock) AddToBalanceWithNonce(_ int64, _ []byte, _ []byte, _ ...*kapps.UserKDA) error {
	return nil
}

// SubFromBalance -
func (awm *AccountWrapMock) SubFromBalance(_ int64, _ []byte, _ ...*kapps.UserKDA) error {
	return nil
}

func (awm *AccountWrapMock) SubFromBalanceWithNonce(_ int64, _ []byte, _ []byte, _ ...*kapps.UserKDA) error {
	return nil
}

// AddInternalKDA -
func (awm *AccountWrapMock) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	return nil
}

// SubInternalKDA -
func (awm *AccountWrapMock) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	return nil, nil
}

func (awm *AccountWrapMock) AddToAllowance(_ int64) error {
	return nil
}

// GetBalance -
func (awm *AccountWrapMock) GetBalance(_ []byte) int64 {
	return 0
}

func (awm *AccountWrapMock) GetBalanceWithNonce(_ []byte, _ []byte) int64 {
	return 0
}

// GetAllowance -
func (awm *AccountWrapMock) GetAllowance() int64 {
	return 0
}

// GetFrozenBalance -
func (awm *AccountWrapMock) GetFrozenBalance(_ []byte) int64 {
	return 0
}

// GetBuckets -
func (awm *AccountWrapMock) GetBuckets(assetID []byte) map[string]*kapps.UserBucket {
	return nil
}

// Freeze -
func (awm *AccountWrapMock) Freeze(assetID []byte, bucketID []byte, value int64, epoch uint32, lockTime int64, staking *kapps.StakingData, userKDA *kapps.UserKDA, _ bool) error {
	return nil
}

// Unfreeze -
func (awm *AccountWrapMock) Unfreeze(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, _ bool) ([]byte, int64, error) {
	return nil, 0, nil
}

// Delegate -
func (awm *AccountWrapMock) Delegate(bucketID []byte, delegation []byte, userKDA *kapps.UserKDA) (int64, error) {
	return 0, nil
}

// Undelegate -
func (awm *AccountWrapMock) Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
	return nil, 0, nil
}

// Withdraw -
func (awm *AccountWrapMock) Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error) {
	return 0, nil
}

// Claim -
func (awm *AccountWrapMock) Claim(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
	return nil, nil
}

// ChangeOwnerAddress -
func (awm *AccountWrapMock) ChangeOwnerAddress([]byte, []byte) error {
	return nil
}

// SetOwnerAddress -
func (awm *AccountWrapMock) SetOwnerAddress([]byte) {

}

// GetOwnerAddress -
func (awm *AccountWrapMock) GetOwnerAddress() []byte {
	return nil
}

// NewAccountWrapMock -
func NewAccountWrapMock(adr []byte) *AccountWrapMock {
	return &AccountWrapMock{
		address:           adr,
		trackableDataTrie: state.NewTrackableDataTrie([]byte("identifier"), nil),
	}
}

// IsInterfaceNil -
func (awm *AccountWrapMock) IsInterfaceNil() bool {
	return awm == nil
}

// GetRootHash -
func (awm *AccountWrapMock) GetRootHash() []byte {
	return awm.RootHash
}

// SetRootHash -
func (awm *AccountWrapMock) SetRootHash(rootHash []byte) {
	awm.RootHash = rootHash
}

// AddressBytes -
func (awm *AccountWrapMock) AddressBytes() []byte {
	return awm.address
}

// DataTrie -
func (awm *AccountWrapMock) DataTrie() data.Trie {
	return awm.dataTrie
}

// SetDataTrie -
func (awm *AccountWrapMock) SetDataTrie(trie data.Trie) {
	awm.dataTrie = trie
	awm.trackableDataTrie.SetDataTrie(trie)
}

// DataTrieTracker -
func (awm *AccountWrapMock) DataTrieTracker() state.DataTrieTracker {
	return awm.trackableDataTrie
}

func (awm *AccountWrapMock) ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	return nil, nil
}

func (awm *AccountWrapMock) GetPermission(id int32) (*state.Permission, bool, error) {
	return nil, false, nil
}

func (awm *AccountWrapMock) SetPermissions([]*state.Permission) {}

func (awm *AccountWrapMock) AccountDataHandler() state.AccountDataHandler {
	return nil
}

func (awm *AccountWrapMock) GetCodeHash() []byte {
	return nil
}

func (awm *AccountWrapMock) GetCodeMetadata() []byte {
	return nil
}

func (awm *AccountWrapMock) HasNewCode() bool {
	return false
}

func (awm *AccountWrapMock) RetrieveValue(key []byte) ([]byte, error) {
	return nil, nil
}

func (awm *AccountWrapMock) SaveKeyValue(key []byte, value []byte) error {
	return nil
}

func (awm *AccountWrapMock) SetCode(code []byte) {

}

func (awm *AccountWrapMock) SetCodeHash(hash []byte) {

}

func (awm *AccountWrapMock) SetCodeMetadata(metadata []byte) {

}

func (awm *AccountWrapMock) Equal(toCompare state.UserAccountHandler) bool {
	return reflect.DeepEqual(awm, toCompare)
}
