package mock

import (
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

var _ data.HeaderHandler = (*HeaderHandlerStub)(nil)

// HeaderHandlerStub --
type HeaderHandlerStub struct {
	GetPrevRandSeedCalled       func() []byte
	SetPrevRandSeedCalled       func([]byte)
	GetRandSeedCalled           func() []byte
	SetRandSeedCalled           func([]byte)
	GetParentHashCalled         func() []byte
	CheckChainIDCalled          func(reference []byte) error
	GetNonceCalled              func() uint64
	GetIsEpochStartCalled       func() bool
	GetPrevEpochStartSlotCalled func() uint64
	GetEpochCaled               func() uint32
	GetSlotCalled               func() uint64
	GetTxRootHashCalled         func() []byte
	GetProducerPublicKeyCalled  func() []byte
	GetBlockHeaderCalled        func() interface{}
}

// GetProducerID -
func (hhs *HeaderHandlerStub) GetProducerID() uint64 {
	panic("implement me")
}

// SetProducerID -
func (hhs *HeaderHandlerStub) SetProducerID(_ uint64) {
	panic("implement me")
}

func (hhs *HeaderHandlerStub) GetBurnedUnclaimed() int64 {
	panic("implement me")
}

func (hhs *HeaderHandlerStub) SetBurnedUnclaimed(burned int64) {
	panic("implement me")
}

// GetProducerPublicKey -
func (hhs *HeaderHandlerStub) GetProducerPublicKey() []byte {
	if hhs.GetProducerPublicKeyCalled != nil {
		return hhs.GetProducerPublicKeyCalled()
	}

	return make([]byte, 0)
}

// SetProducerPublicKey -
func (hhs *HeaderHandlerStub) ComputeRootHash(_ hashing.Hasher) ([]byte, error) {
	panic("implement me")
}

// SetProducerPublicKey -
func (hhs *HeaderHandlerStub) SetProducerPublicKey(_ []byte) {
	panic("implement me")
}

// GetAccumulatedFees --
func (hhs *HeaderHandlerStub) GetAccumulatedFees() int64 {
	panic("implement me")
}

// GetDeveloperFees --
func (hhs *HeaderHandlerStub) GetDeveloperFees() int64 {
	panic("implement me")
}

// GetBlockHeader --
func (hhs *HeaderHandlerStub) GetBlockHeader() interface{} {
	if hhs.GetBlockHeaderCalled != nil {
		return hhs.GetBlockHeaderCalled()
	}

	return &block.Block{Header: &block.BlockHeader{}}
}

// GetTxFees --
func (hhs *HeaderHandlerStub) GetTxFees() int64 {
	panic("implement me")
}

// GetTxFees --
func (hhs *HeaderHandlerStub) GetTxBurnedFees() int64 {
	panic("implement me")
}

// GetKAppFees --
func (hhs *HeaderHandlerStub) GetKAppFees() int64 {
	panic("implement me")
}

// GetBlockRewards --
func (hhs *HeaderHandlerStub) GetBlockRewards() int64 {
	panic("implement me")
}

// GetStakingRewards --
func (hhs *HeaderHandlerStub) GetStakingRewards() int64 {
	panic("implement me")
}

// SetAccumulatedFees --
func (hhs *HeaderHandlerStub) SetAccumulatedFees(_ int64) {
	panic("implement me")
}

// SetDeveloperFees --
func (hhs *HeaderHandlerStub) SetDeveloperFees(int64) {
	panic("implement me")
}

// SetKFIFees --
func (hhs *HeaderHandlerStub) SetKFIFees(int64) {
	panic("implement me")
}

// GetTxHashes --
func (hhs *HeaderHandlerStub) GetTxHashes() [][]byte {
	return nil
}

// Clone --
func (hhs *HeaderHandlerStub) Clone() data.HeaderHandler {
	panic("implement me")
}

// GetIsEpochStart --
func (hhs *HeaderHandlerStub) GetIsEpochStart() bool {
	if hhs.GetIsEpochStartCalled != nil {
		return hhs.GetIsEpochStartCalled()
	}

	return false
}

// GetPrevEpochStartSlot --
func (hhs *HeaderHandlerStub) GetPrevEpochStartSlot() uint64 {
	if hhs.GetPrevEpochStartSlotCalled != nil {
		return hhs.GetPrevEpochStartSlotCalled()
	}

	return 0
}

// GetNonce -
func (hhs *HeaderHandlerStub) GetNonce() uint64 {
	if hhs.GetNonceCalled != nil {
		return hhs.GetNonceCalled()
	}
	return 1
}

// GetEpoch -
func (hhs *HeaderHandlerStub) GetEpoch() uint32 {
	if hhs.GetEpochCaled != nil {
		return hhs.GetEpochCaled()
	}

	return 0
}

// GetSlot -
func (hhs *HeaderHandlerStub) GetSlot() uint64 {
	if hhs.GetSlotCalled != nil {
		return hhs.GetSlotCalled()
	}
	return 1
}

// GetTimestamp -
func (hhs *HeaderHandlerStub) GetTimestamp() int64 {
	panic("implement me")
}

// GetParentHash -
func (hhs *HeaderHandlerStub) GetParentHash() []byte {
	if hhs.GetParentHashCalled != nil {
		return hhs.GetParentHashCalled()
	}

	return make([]byte, 0)
}

// GetPubKeysBitmap -
func (hhs *HeaderHandlerStub) GetPubKeysBitmap() []byte {
	panic("implement me")
}

// SetPubKeysBitmap -
func (hhs *HeaderHandlerStub) SetPubKeysBitmap(_ []byte) {
	panic("implement me")
}

// GetSignature -
func (hhs *HeaderHandlerStub) GetSignature() []byte {
	panic("implement me")
}

// SetSignature -
func (hhs *HeaderHandlerStub) SetSignature(_ []byte) {
	panic("implement me")
}

// GetProducerSignature -
func (hhs *HeaderHandlerStub) GetProducerSignature() []byte {
	panic("implement me")
}

// SetProducerSignature -
func (hhs *HeaderHandlerStub) SetProducerSignature(_ []byte) {
	panic("implement me")
}

// GetChainID -
func (hhs *HeaderHandlerStub) GetChainID() []byte {
	panic("implement me")
}

// CheckChainID -
func (hhs *HeaderHandlerStub) CheckChainID(reference []byte) error {
	if hhs.CheckChainIDCalled != nil {
		return hhs.CheckChainIDCalled(reference)
	}
	return nil
}

// GetTxCount -
func (hhs *HeaderHandlerStub) GetTxCount() uint32 {
	return 0
}

// GetReserved -
func (hhs *HeaderHandlerStub) GetReserved() []byte {
	return nil
}

// SetNonce -
func (hhs *HeaderHandlerStub) SetNonce(_ uint64) {
	panic("implement me")
}

// SetEpoch -
func (hhs *HeaderHandlerStub) SetEpoch(_ uint32) {
	panic("implement me")
}

// SetSlot -
func (hhs *HeaderHandlerStub) SetSlot(_ uint64) {
	panic("implement me")
}

// SetTimestamp -
func (hhs *HeaderHandlerStub) SetTimestamp(_ int64) {
	panic("implement me")
}

// SetParentHash -
func (hhs *HeaderHandlerStub) SetParentHash(_ []byte) {
	panic("implement me")
}

// SetChainID -
func (hhs *HeaderHandlerStub) SetChainID(_ []byte) {
	panic("implement me")
}

// SetTxCount -
func (hhs *HeaderHandlerStub) SetTxCount(_ uint32) {
	panic("implement me")
}

// IsInterfaceNil returns true if there is no value under the interface
func (hhs *HeaderHandlerStub) IsInterfaceNil() bool {
	return hhs == nil
}

// GetEpochStartMetaHash -
func (hhs *HeaderHandlerStub) GetEpochStartMetaHash() []byte {
	panic("implement me")
}

// GetSoftwareVersion -
func (hhs *HeaderHandlerStub) GetSoftwareVersion() []byte {
	return []byte("v1.0")
}

// SetSoftwareVersion -
func (hhs *HeaderHandlerStub) SetSoftwareVersion(_ []byte) {
}

// GetTxRootHash -
func (hhs *HeaderHandlerStub) GetTxRootHash() []byte {
	if hhs.GetTxRootHashCalled != nil {
		return hhs.GetTxRootHashCalled()
	}
	return make([]byte, 0)
}

// GetRandSeed --
func (hhs *HeaderHandlerStub) GetRandSeed() []byte {
	if hhs.GetRandSeedCalled != nil {
		return hhs.GetRandSeedCalled()
	}

	return make([]byte, 0)
}

// GetPrevRandSeed --
func (hhs *HeaderHandlerStub) GetPrevRandSeed() []byte {
	if hhs.GetPrevRandSeedCalled != nil {
		return hhs.GetPrevRandSeedCalled()
	}

	return make([]byte, 0)
}

// SetPrevRandSeed --
func (hhs *HeaderHandlerStub) SetPrevRandSeed(data []byte) {
	if hhs.SetPrevRandSeedCalled != nil {
		hhs.SetPrevRandSeedCalled(data)
	}
}

// SetRandSeed --
func (hhs *HeaderHandlerStub) SetRandSeed(data []byte) {
	if hhs.SetRandSeedCalled != nil {
		hhs.SetRandSeedCalled(data)
	}
}

// GetTrieRoot -
func (hhs *HeaderHandlerStub) GetTrieRoot() []byte {
	panic("implement me")
}

// SetTrieRoot -
func (hhs *HeaderHandlerStub) SetTrieRoot(_ []byte) {
	panic("implement me")
}

// GetAssetAccountsTrieRoot -
func (hhs *HeaderHandlerStub) GetAssetAccountsTrieRoot() []byte {
	panic("implement me")
}

// GetAssetTrieRoot -
func (hhs *HeaderHandlerStub) GetAssetTrieRoot() []byte {
	panic("implement me")
}

// GetStakingTrieRoot -
func (hhs *HeaderHandlerStub) GetStakingTrieRoot() []byte {
	panic("implement me")
}

// GetValidatorsTrieRoot -
func (hhs *HeaderHandlerStub) GetValidatorsTrieRoot() []byte {
	return nil
}

// GetKAppsTrieRoot -
func (hhs *HeaderHandlerStub) GetKAppsTrieRoot() []byte {
	panic("implement me")
}

// GetKAppsTrieRoot -
func (hhs *HeaderHandlerStub) SetKAppsTrieRoot([]byte) {
	panic("implement me")
}
