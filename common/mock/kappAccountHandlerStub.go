package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// KAppAccountHandlerStub -
type KAppAccountHandlerStub struct {
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
	state.AccountHandler
}

func (k *KAppAccountHandlerStub) GetUserKDA(assetID []byte, nonce []byte, _ bool) (*kapps.UserKDA, error) {
	return nil, nil
}

func (k *KAppAccountHandlerStub) GetStorage(key []byte) []byte {
	return nil
}

func (k *KAppAccountHandlerStub) SetStorage(key []byte, value []byte) error {
	return nil
}

// SetRootHash -
func (k *KAppAccountHandlerStub) SetRootHash(rootHash []byte) {
	if k.SetRootHashCalled != nil {
		k.SetRootHashCalled(rootHash)
	}
}

// GetRootHash -
func (k *KAppAccountHandlerStub) GetRootHash() []byte {
	if k.GetRootHashCalled != nil {
		return k.GetRootHashCalled()
	}
	return []byte("roothash")
}

// SetName -
func (k *KAppAccountHandlerStub) SetName(name []byte) {
	if k.SetNameCalled != nil {
		k.SetNameCalled(name)
	}
}

// GetName -
func (k *KAppAccountHandlerStub) GetName() []byte {
	if k.GetNameCalled != nil {
		return k.GetNameCalled()
	}
	return []byte("roothash")
}

// StartProposalsKApp -
func (k *KAppAccountHandlerStub) StartProposalsKApp(forks core.ForkController) (kapps.ActiveProposalController, error) {
	if k.StartProposalsKAppCalled != nil {
		return k.StartProposalsKAppCalled(forks)
	}

	return kapps.NewProposalController(forks)
}

// GetNonce -
func (k *KAppAccountHandlerStub) GetNonce() uint64 {
	if k.GetNonceCalled != nil {
		return k.GetNonceCalled()
	}
	return uint64(0)
}

// IncreaseNonce -
func (k *KAppAccountHandlerStub) IncreaseNonce(nonce uint64) {
	if k.IncreaseNonceCalled != nil {
		k.IncreaseNonceCalled(nonce)
	}
}

// AddInternalKDA -
func (k *KAppAccountHandlerStub) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	if k.AddInternalKDACalled != nil {
		return k.AddInternalKDACalled(assetID, internalID, data)
	}
	return nil
}

// SubInternalKDA -
func (k *KAppAccountHandlerStub) SubInternalKDA(assetID []byte, internalID []byte) ([]byte, error) {
	if k.SubInternalKDACalled != nil {
		return k.SubInternalKDACalled(assetID, internalID)
	}
	return []byte("kda"), nil
}

// SetDataTrie -
func (k *KAppAccountHandlerStub) SetDataTrie(tr data.Trie) {
	k.SetDataTrieCalled(tr)
}

// DataTrie -
func (k *KAppAccountHandlerStub) DataTrie() data.Trie {
	return k.DataTrieCalled()
}

// DataTrieTracker -
func (k *KAppAccountHandlerStub) DataTrieTracker() state.DataTrieTracker {
	return k.DataTrieTrackerCalled()
}
