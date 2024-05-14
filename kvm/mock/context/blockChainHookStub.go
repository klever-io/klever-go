package mock

import (
	"math/big"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ vmcommon.BlockchainHook = (*BlockchainHookStub)(nil)

// BlockchainHookStub is used in tests to check that interface methods were called
type BlockchainHookStub struct {
	NewAddressCalled                        func(creatorAddress []byte, creatorNonce uint64, vmType []byte) ([]byte, error)
	GetStorageDataCalled                    func(accountsAddress []byte, index []byte) ([]byte, uint32, error)
	GetBlockHashCalled                      func(nonce uint64) ([]byte, error)
	LastNonceCalled                         func() uint64
	LastSlotCalled                          func() uint64
	LastTimeStampCalled                     func() int64
	LastRandomSeedCalled                    func() []byte
	LastEpochCalled                         func() uint32
	GetStateRootHashCalled                  func() []byte
	CurrentNonceCalled                      func() uint64
	CurrentSlotCalled                       func() uint64
	CurrentTimeStampCalled                  func() int64
	CurrentRandomSeedCalled                 func() []byte
	CurrentEpochCalled                      func() uint32
	ProcessBuiltInFunctionCalled            func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	GetBuiltinFunctionNamesCalled           func() vmcommon.FunctionNames
	GetAllStateCalled                       func(address []byte) (map[string][]byte, error)
	GetUserAccountCalled                    func(address []byte) (state.UserAccountHandler, error)
	IsSmartContractCalled                   func(address []byte) bool
	IsPayableCalled                         func(address []byte) (bool, error)
	GetCompiledCodeCalled                   func(codeHash []byte) (bool, []byte)
	SaveCompiledCodeCalled                  func(codeHash []byte, code []byte)
	GetCodeCalled                           func(account state.UserAccountHandler) []byte
	GetKDATokenCalled                       func(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error)
	GetSnapshotCalled                       func() int
	RevertToSnapshotCalled                  func(snapshot int) error
	ExecuteSmartContractCallOnOtherVMCalled func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	TransferValueOnlyCalled                 func(destination []byte, sender []byte, value *big.Int) error
	KDATransferCalled                       func(sender []byte, tc *transaction.TransferContract) error
	DeleteCompiledCodeCalled                func(codeHash []byte)
	FilterCodeMetadataForUpgradeCalled      func(input []byte) ([]byte, error)
	GetBuiltinFunctionsContainerCalled      func() vmcommon.BuiltInFunctionContainer
	ResetCountersCalled                     func()
	GetCounterValuesCalled                  func() map[string]uint64
	GetKAppControllerCalled                 func() kapp.KAppController
	LastBlockCalled                         func() data.HeaderHandler
	SetCurrentHeaderCalled                  func(hdr data.HeaderHandler)
	GetSFTMetaCalled                        func(tokenID []byte, nonce uint64) (*kapps.MetaV2, error)
}

func (b *BlockchainHookStub) GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error) {
	if b.GetSFTMetaCalled != nil {
		return b.GetSFTMetaCalled(tokenID, nonce)
	}

	return &kapps.MetaV2{}, nil
}

func (b *BlockchainHookStub) GetBlockHash(nonce uint64) ([]byte, error) {
	if b.GetBlockHashCalled != nil {
		return b.GetBlockHashCalled(nonce)
	}

	return []byte{1}, nil
}

func (b *BlockchainHookStub) TransferValueOnly(destination []byte, sender []byte, value *big.Int) error {
	if b.TransferValueOnlyCalled != nil {
		return b.TransferValueOnlyCalled(destination, sender, value)
	}
	return nil
}

// NewAddress mocked method
func (b *BlockchainHookStub) NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte) ([]byte, error) {
	if b.NewAddressCalled != nil {
		return b.NewAddressCalled(creatorAddress, creatorNonce, vmType)
	}
	return []byte("newAddress"), nil
}

// GetStorageData mocked method
func (b *BlockchainHookStub) GetStorageData(accountAddress []byte, index []byte) ([]byte, uint32, error) {
	if b.GetStorageDataCalled != nil {
		return b.GetStorageDataCalled(accountAddress, index)
	}
	return nil, 0, nil
}

// GetBlockhash mocked method
func (b *BlockchainHookStub) GetBlockhash(nonce uint64) ([]byte, error) {
	if b.GetBlockHashCalled != nil {
		return b.GetBlockHashCalled(nonce)
	}
	return []byte("roothash"), nil
}

// LastNonce mocked method
func (b *BlockchainHookStub) LastNonce() uint64 {
	if b.LastNonceCalled != nil {
		return b.LastNonceCalled()
	}
	return 0
}

// LastSlot mocked method
func (b *BlockchainHookStub) LastSlot() uint64 {
	if b.LastSlotCalled != nil {
		return b.LastSlotCalled()
	}
	return 0
}

// LastTimeStamp mocked method
func (b *BlockchainHookStub) LastTimeStamp() int64 {
	if b.LastTimeStampCalled != nil {
		return b.LastTimeStampCalled()
	}
	return 0
}

// LastRandomSeed mocked method
func (b *BlockchainHookStub) LastRandomSeed() []byte {
	if b.LastRandomSeedCalled != nil {
		return b.LastRandomSeedCalled()
	}
	return []byte("seed")
}

// LastEpoch mocked method
func (b *BlockchainHookStub) LastEpoch() uint32 {
	if b.LastEpochCalled != nil {
		return b.LastEpochCalled()
	}
	return 0
}

// GetStateRootHash mocked method
func (b *BlockchainHookStub) GetStateRootHash() []byte {
	if b.GetStateRootHashCalled != nil {
		return b.GetStateRootHashCalled()
	}
	return []byte("roothash")
}

// CurrentNonce mocked method
func (b *BlockchainHookStub) CurrentNonce() uint64 {
	if b.CurrentNonceCalled != nil {
		return b.CurrentNonceCalled()
	}
	return 0
}

// CurrentSlot mocked method
func (b *BlockchainHookStub) CurrentSlot() uint64 {
	if b.CurrentSlotCalled != nil {
		return b.CurrentSlotCalled()
	}
	return 0
}

// CurrentTimeStamp mocked method
func (b *BlockchainHookStub) CurrentTimeStamp() int64 {
	if b.CurrentTimeStampCalled != nil {
		return b.CurrentTimeStampCalled()
	}
	return 0
}

// CurrentRandomSeed mocked method
func (b *BlockchainHookStub) CurrentRandomSeed() []byte {
	if b.CurrentRandomSeedCalled != nil {
		return b.CurrentRandomSeedCalled()
	}
	return []byte("seed")
}

// CurrentEpoch mocked method
func (b *BlockchainHookStub) CurrentEpoch() uint32 {
	if b.CurrentEpochCalled != nil {
		return b.CurrentEpochCalled()
	}
	return 0
}

// ProcessBuiltInFunction mocked method
func (b *BlockchainHookStub) ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if b.ProcessBuiltInFunctionCalled != nil {
		return b.ProcessBuiltInFunctionCalled(input)
	}
	return &vmcommon.VMOutput{}, nil
}

// GetBuiltinFunctionNames mocked method
func (b *BlockchainHookStub) GetBuiltinFunctionNames() vmcommon.FunctionNames {
	if b.GetBuiltinFunctionNamesCalled != nil {
		return b.GetBuiltinFunctionNamesCalled()
	}
	return make(vmcommon.FunctionNames)
}

// GetAllState mocked method
func (b *BlockchainHookStub) GetAllState(address []byte) (map[string][]byte, error) {
	if b.GetAllStateCalled != nil {
		return b.GetAllStateCalled(address)
	}
	return nil, nil
}

// GetUserAccount mocked method
func (b *BlockchainHookStub) GetUserAccount(address []byte) (state.UserAccountHandler, error) {
	if b.GetUserAccountCalled != nil {
		return b.GetUserAccountCalled(address)
	}
	return nil, nil
}

// GetKDAToken mocked method
func (b *BlockchainHookStub) GetKDAToken(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
	if b.GetKDATokenCalled != nil {
		return b.GetKDATokenCalled(address, tokenID, nonce)
	}
	return &kapps.KDAData{}, &kapps.UserKDA{}, nil
}

// GetCode mocked method
func (b *BlockchainHookStub) GetCode(account state.UserAccountHandler) []byte {
	if b.GetCodeCalled != nil {
		return b.GetCodeCalled(account)
	}
	return nil
}

// IsSmartContract mocked method
func (b *BlockchainHookStub) IsSmartContract(address []byte) bool {
	if b.IsSmartContractCalled != nil {
		return b.IsSmartContractCalled(address)
	}
	return false
}

// IsPayable mocked method
func (b *BlockchainHookStub) IsPayable(_, address []byte) (bool, error) {
	if b.IsPayableCalled != nil {
		return b.IsPayableCalled(address)
	}
	return true, nil
}

// SaveCompiledCode mocked method
func (b *BlockchainHookStub) SaveCompiledCode(codeHash []byte, code []byte) {
	if b.SaveCompiledCodeCalled != nil {
		b.SaveCompiledCodeCalled(codeHash, code)
	}
}

// GetCompiledCode mocked method
func (b *BlockchainHookStub) GetCompiledCode(codeHash []byte) (bool, []byte) {
	if b.GetCompiledCodeCalled != nil {
		return b.GetCompiledCodeCalled(codeHash)
	}
	return false, nil
}

func (b *BlockchainHookStub) KDATransfer(sender []byte, tc *transaction.TransferContract) error {
	if b.KDATransferCalled != nil {
		return b.KDATransfer(sender, tc)
	}

	return nil
}

// ClearCompiledCodes mocked method
func (b *BlockchainHookStub) ClearCompiledCodes() {
}

// GetSnapshot mocked method
func (b *BlockchainHookStub) GetSnapshot() int {
	if b.GetSnapshotCalled != nil {
		return b.GetSnapshotCalled()
	}
	return 1
}

// RevertToSnapshot mocked method
func (b *BlockchainHookStub) RevertToSnapshot(snapshot int) error {
	if b.RevertToSnapshotCalled != nil {
		return b.RevertToSnapshotCalled(snapshot)
	}
	return nil
}

// ExecuteSmartContractCallOnOtherVM -
func (b *BlockchainHookStub) ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if b.ExecuteSmartContractCallOnOtherVMCalled != nil {
		return b.ExecuteSmartContractCallOnOtherVMCalled(input)
	}
	return nil, nil
}

func (b *BlockchainHookStub) DeleteCompiledCode(codeHash []byte) {
	if b.DeleteCompiledCodeCalled != nil {
		b.DeleteCompiledCodeCalled(codeHash)
	}
}

func (b *BlockchainHookStub) FilterCodeMetadataForUpgrade(input []byte) ([]byte, error) {
	if b.FilterCodeMetadataForUpgradeCalled != nil {
		return b.FilterCodeMetadataForUpgradeCalled(input)
	}

	return make([]byte, 0), nil
}

func (b *BlockchainHookStub) GetBuiltinFunctionsContainer() vmcommon.BuiltInFunctionContainer {
	if b.GetBuiltinFunctionsContainerCalled != nil {
		return b.GetBuiltinFunctionsContainerCalled()
	}
	return nil
}

func (b *BlockchainHookStub) ResetCounters() {
	if b.ResetCountersCalled != nil {
		b.ResetCountersCalled()
	}
}

func (b *BlockchainHookStub) GetCounterValues() map[string]uint64 {
	if b.GetCounterValuesCalled != nil {
		return b.GetCounterValuesCalled()
	}

	return make(map[string]uint64)
}

func (b *BlockchainHookStub) GetKAppController() kapp.KAppController {
	if b.GetKAppControllerCalled != nil {
		return b.GetKAppControllerCalled()
	}

	return nil
}

func (b *BlockchainHookStub) LastBlock() data.HeaderHandler {
	if b.LastBlockCalled != nil {
		return b.LastBlockCalled()
	}

	return nil

}

func (b *BlockchainHookStub) SetCurrentHeader(hdr data.HeaderHandler) {
	if b.SetCurrentHeaderCalled != nil {
		b.SetCurrentHeaderCalled(hdr)
	}
}

func (b *BlockchainHookStub) Close() error {
	return nil
}

// IsInterfaceNil mocked method
func (b *BlockchainHookStub) IsInterfaceNil() bool {
	return b == nil
}
