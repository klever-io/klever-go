package mock

import (
	"bytes"
	"math/big"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ vmhost.BlockchainContext = (*BlockchainContextMock)(nil)

// BlockchainContextMock -
type BlockchainContextMock struct {
}

// InitState -
func (b *BlockchainContextMock) InitState() {
}

// PushState -
func (b *BlockchainContextMock) PushState() {
}

// PopSetActiveState -
func (b *BlockchainContextMock) PopSetActiveState() {
}

// PopDiscard -
func (b *BlockchainContextMock) PopDiscard() {
}

// ClearStateStack -
func (b *BlockchainContextMock) ClearStateStack() {
}

// NewAddress -
func (b *BlockchainContextMock) NewAddress(creatorAddress []byte) ([]byte, error) {
	return creatorAddress, nil
}

// AccountExists -
func (b *BlockchainContextMock) AccountExists(_ []byte) bool {
	return true
}

// GetBalance -
func (b *BlockchainContextMock) GetBalance(_ []byte) []byte {
	return make([]byte, 0)
}

// GetBalanceBigInt -
func (b *BlockchainContextMock) GetBalanceBigInt(_ []byte) *big.Int {
	return big.NewInt(0)
}

// GetNonce -
func (b *BlockchainContextMock) GetNonce(_ []byte) (uint64, error) {
	return 0, nil
}

// CurrentEpoch -
func (b *BlockchainContextMock) CurrentEpoch() uint32 {
	return 0
}

// GetStateRootHash -
func (b *BlockchainContextMock) GetStateRootHash() []byte {
	return bytes.Repeat([]byte{1}, 32)
}

// LastTimeStamp -
func (b *BlockchainContextMock) LastTimeStamp() int64 {
	return 0
}

// LastNonce -
func (b *BlockchainContextMock) LastNonce() uint64 {
	return 0
}

// LastSlot -
func (b *BlockchainContextMock) LastSlot() uint64 {
	return 0
}

// LastEpoch -
func (b *BlockchainContextMock) LastEpoch() uint32 {
	return 0
}

// CurrentSlot -
func (b *BlockchainContextMock) CurrentSlot() uint64 {
	return 0
}

// CurrentNonce -
func (b *BlockchainContextMock) CurrentNonce() uint64 {
	return 0
}

// CurrentTimeStamp -
func (b *BlockchainContextMock) CurrentTimeStamp() int64 {
	return 0
}

// CurrentRandomSeed -
func (b *BlockchainContextMock) CurrentRandomSeed() []byte {
	return bytes.Repeat([]byte{1}, 32)
}

// LastRandomSeed -
func (b *BlockchainContextMock) LastRandomSeed() []byte {
	return bytes.Repeat([]byte{1}, 32)
}

// IncreaseNonce -
func (b *BlockchainContextMock) IncreaseNonce(_ []byte) {
}

// GetCodeHash -
func (b *BlockchainContextMock) GetCodeHash(addr []byte) []byte {
	return addr
}

// GetCode -
func (b *BlockchainContextMock) GetCode(addr []byte) ([]byte, error) {
	return addr, nil
}

// GetCodeSize -
func (b *BlockchainContextMock) GetCodeSize(_ []byte) (int32, error) {
	return 10, nil
}

// BlockHash -
func (b *BlockchainContextMock) BlockHash(_ uint64) []byte {
	return bytes.Repeat([]byte{1}, 32)
}

// GetOwnerAddress -
func (b *BlockchainContextMock) GetOwnerAddress() ([]byte, error) {
	return bytes.Repeat([]byte{1}, 32), nil
}

// IsSmartContract -
func (b *BlockchainContextMock) IsSmartContract(_ []byte) bool {
	return true
}

// IsPayable -
func (b *BlockchainContextMock) IsPayable(_, _ []byte) (bool, error) {
	return true, nil
}

// SaveCompiledCode -
func (b *BlockchainContextMock) SaveCompiledCode(_ []byte, _ []byte) {
}

// GetCompiledCode -
func (b *BlockchainContextMock) GetCompiledCode(_ []byte) (bool, []byte) {
	return true, make([]byte, 0)
}

// GetKDAToken -
func (b *BlockchainContextMock) GetKDAToken(_ []byte, _ []byte, _ uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
	return &kapps.KDAData{}, &kapps.UserKDA{}, nil
}

// GetUserAccount -
func (b *BlockchainContextMock) GetUserAccount(_ []byte) (state.UserAccountHandler, error) {
	return nil, nil
}

// ProcessBuiltInFunction -
func (b *BlockchainContextMock) ProcessBuiltInFunction(_ *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	return &vmcommon.VMOutput{}, nil
}

// GetSnapshot -
func (b *BlockchainContextMock) GetSnapshot() int {
	return 0
}

// RevertToSnapshot -
func (b *BlockchainContextMock) RevertToSnapshot(_ int) {
}

// IsLimitedTransfer -
func (b *BlockchainContextMock) IsLimitedTransfer(_ []byte) bool {
	return false
}

// IsPaused -
func (b *BlockchainContextMock) IsPaused(_ []byte) bool {
	return false
}

// ClearCompiledCodes -
func (b *BlockchainContextMock) ClearCompiledCodes() {
}

// ExecuteSmartContractCallOnOtherVM -
func (b *BlockchainContextMock) ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	return nil, nil
}

func (b *BlockchainContextMock) TransferValueOnly(destination []byte, sender []byte, value *big.Int) error {
	return nil
}

func (b *BlockchainContextMock) KDATransfer(sender []byte, tc *transaction.TransferContract) error {
	return nil
}
