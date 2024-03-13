package mock

import (
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
)

var _ data.HeaderHandler = (*HeaderHandlerStub)(nil)

// HeaderHandlerStub --
type HeaderHandlerStub struct {
	GetPrevRandSeedCalled       func() []byte
	SetPrevRandSeedCalled       func([]byte)
	GetRandSeedCalled           func() []byte
	SetRandSeedCalled           func([]byte)
	GetIsEpochStartCalled       func() bool
	GetPrevEpochStartSlotCalled func() uint64
	GetEpochCaled               func() uint32
	GetTxRootHashCalled         func() []byte
	ComputeRootHashCalled       func(_ hashing.Hasher) ([]byte, error)
}

func (hhs *HeaderHandlerStub) GetTxBurnedFees() int64 {
	panic("implement me")
}

func (hhs *HeaderHandlerStub) GetBurnedUnclaimed() int64 {
	panic("implement me")
}

func (hhs *HeaderHandlerStub) SetBurnedUnclaimed(burned int64) {
	panic("implement me")
}

// GetProducerID -
func (hhs *HeaderHandlerStub) GetProducerID() uint64 {
	panic("implement me")
}

// SetProducerID -
func (hhs *HeaderHandlerStub) SetProducerID(_ uint64) {
	panic("implement me")
}

// GetProducerPublicKey -
func (hhs *HeaderHandlerStub) GetProducerPublicKey() []byte {
	panic("implement me")
}

// SetProducerPublicKey -
func (hhs *HeaderHandlerStub) SetProducerPublicKey(_ []byte) {
	panic("implement me")
}

// GetTxFees --
func (hhs *HeaderHandlerStub) GetTxFees() int64 {
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

// SetTxFees --
func (hhs *HeaderHandlerStub) SetTxFees(_ int64) {
	panic("implement me")
}

// SetBlockRewards --
func (hhs *HeaderHandlerStub) SetBlockRewards(int64) {
	panic("implement me")
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
	panic("implement me")
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
	panic("implement me")
}

// GetTimestamp -
func (hhs *HeaderHandlerStub) GetTimestamp() int64 {
	panic("implement me")
}

// GetParentHash -
func (hhs *HeaderHandlerStub) GetParentHash() []byte {
	panic("implement me")
}

// GetPubKeysBitmap -
func (hhs *HeaderHandlerStub) GetPubKeysBitmap() []byte {
	panic("implement me")
}

// SetPubKeysBitmap -
func (hhs *HeaderHandlerStub) SetPubKeysBitmap(signature []byte) {
	panic("implement me")
}

// SetProducerSignature -
func (hhs *HeaderHandlerStub) SetProducerSignature(signature []byte) {
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

// GetChainID -
func (hhs *HeaderHandlerStub) GetChainID() []byte {
	panic("implement me")
}

// GetTxCount -
func (hhs *HeaderHandlerStub) GetTxCount() uint32 {
	panic("implement me")
}

// GetReserved -
func (hhs *HeaderHandlerStub) GetReserved() []byte {
	return nil
}

// GetBlockHeader -
func (hhs *HeaderHandlerStub) GetBlockHeader() interface{} {
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

// GetTxRootHash -
func (hhs *HeaderHandlerStub) GetTxRootHash() []byte {
	if hhs.GetTxRootHashCalled != nil {
		return hhs.GetTxRootHashCalled()
	}
	return make([]byte, 0)
}

// GetTrieRoot -
func (hhs *HeaderHandlerStub) GetTrieRoot() []byte {
	panic("implement me")
}

// SetTrieRoot -
func (hhs *HeaderHandlerStub) SetTrieRoot(_ []byte) {
	panic("implement me")
}

// GetAssetTrieRoot -
func (hhs *HeaderHandlerStub) GetAssetTrieRoot() []byte {
	panic("implement me")
}

// GetKAppsTrieRoot -
func (hhs *HeaderHandlerStub) GetKAppsTrieRoot() []byte {
	panic("implement me")
}

// GetKAppsTrieRoot -
func (hhs *HeaderHandlerStub) SetKAppsTrieRoot([]byte) {
	panic("implement me")
}

// GetValidatorsTrieRoot -
func (hhs *HeaderHandlerStub) GetValidatorsTrieRoot() []byte {
	panic("implement me")
}

// GetTxHashes -
func (hhs *HeaderHandlerStub) GetTxHashes() [][]byte {
	panic("implement me")
}

func (hhs *HeaderHandlerStub) ComputeRootHash(_ hashing.Hasher) ([]byte, error) {
	return make([]byte, 0), nil
}

// GetStakingRewards -
func (hhs *HeaderHandlerStub) GetStakingRewards() int64 {
	return 0
}
