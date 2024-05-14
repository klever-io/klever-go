package mock

import (
	"errors"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// ErrNegativeValue -
var ErrNegativeValue = errors.New("negative value provided")

// UserAccountMock -
type UserAccountMock struct {
	BaseAccountMock
	rootHash     []byte
	BalanceField int64
}

// SetRootHash -
func (uam *UserAccountMock) SetRootHash(bytes []byte) {
	uam.rootHash = bytes
}

// GetRootHash -
func (uam *UserAccountMock) GetRootHash() []byte {
	return uam.rootHash
}

// SetDataTrie -
func (uam *UserAccountMock) SetDataTrie(_ data.Trie) {
}

// DataTrie -
func (uam *UserAccountMock) DataTrie() data.Trie {
	return nil
}

// DataTrieTracker -
func (uam *UserAccountMock) DataTrieTracker() state.DataTrieTracker {
	return nil
}

// AddToBalance -
func (uam *UserAccountMock) AddToBalance(value int64, assetID []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrNegativeValue
	}

	uam.BalanceField += value

	return nil
}

// SubFromBalance -
func (uam *UserAccountMock) SubFromBalance(value int64, assetID []byte, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrNegativeValue
	}

	uam.BalanceField -= value

	return nil
}

func (uam *UserAccountMock) AddToBalanceWithNonce(value int64, assetID []byte, nonce []byte, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrNegativeValue
	}

	uam.BalanceField += value

	return nil
}

func (uam *UserAccountMock) SubFromBalanceWithNonce(value int64, assetID []byte, nonce []byte, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrNegativeValue
	}

	uam.BalanceField -= value

	return nil
}

// GetBalance -
func (uam *UserAccountMock) GetBalance(assetID []byte) int64 {
	return uam.BalanceField
}

// ChangeOwnerAddress -
func (uam *UserAccountMock) ChangeOwnerAddress(_ []byte, _ []byte) error {
	return nil
}

// SetOwnerAddress -
func (uam *UserAccountMock) SetOwnerAddress(_ []byte) {
}

// GetOwnerAddress -
func (uam *UserAccountMock) GetOwnerAddress() []byte {
	return nil
}

// SetUserName -
func (uam *UserAccountMock) SetUserName(_ []byte) {
}

// GetUserName -
func (uam *UserAccountMock) GetUserName() []byte {
	return nil
}
