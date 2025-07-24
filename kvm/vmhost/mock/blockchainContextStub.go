package mock

import (
	"math/big"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/vmcommon"
)

type BlockchainContextStub struct {
	InitStateCalled                         func()
	PushStateCalled                         func()
	PopSetActiveStateCalled                 func()
	PopDiscardCalled                        func()
	ClearStateStackCalled                   func()
	NewAddressCalled                        func(creatorAddress []byte) ([]byte, error)
	AccountExistsCalled                     func([]byte) bool
	GetBalanceCalled                        func([]byte) []byte
	GetBalanceBigIntCalled                  func([]byte) *big.Int
	GetNonceCalled                          func([]byte) (uint64, error)
	CurrentEpochCalled                      func() uint32
	GetStateRootHashCalled                  func() []byte
	LastTimeStampCalled                     func() int64
	LastNonceCalled                         func() uint64
	LastSlotCalled                          func() uint64
	LastEpochCalled                         func() uint32
	CurrentSlotCalled                       func() uint64
	CurrentNonceCalled                      func() uint64
	CurrentTimeStampCalled                  func() int64
	CurrentRandomSeedCalled                 func() []byte
	LastRandomSeedCalled                    func() []byte
	IncreaseNonceCalled                     func([]byte)
	GetCodeHashCalled                       func([]byte) []byte
	GetCodeCalled                           func([]byte) ([]byte, error)
	GetCodeSizeCalled                       func([]byte) (int32, error)
	BlockHashCalled                         func(uint64) []byte
	GetOwnerAddressCalled                   func() ([]byte, error)
	IsSmartContractCalled                   func([]byte) bool
	IsPayableCalled                         func([]byte, []byte) (bool, error)
	SaveCompiledCodeCalled                  func([]byte, []byte)
	GetCompiledCodeCalled                   func([]byte) (bool, []byte)
	GetKDATokenCalled                       func([]byte, []byte, uint64) (*kapps.KDAData, *kapps.UserKDA, error)
	GetSFTMetaCalled                        func([]byte, uint64) (*kapps.MetaV2, error)
	GetUserAccountCalled                    func([]byte) (state.UserAccountHandler, error)
	ProcessBuiltInFunctionCalled            func(*vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	GetSnapshotCalled                       func() int
	RevertToSnapshotCalled                  func(int)
	IsLimitedTransferCalled                 func([]byte) bool
	IsPausedCalled                          func([]byte) bool
	ClearCompiledCodesCalled                func()
	ExecuteSmartContractCallOnOtherVMCalled func(*vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	TransferValueOnlyCalled                 func(destination []byte, sender []byte, value *big.Int) error
	KDATransferCalled                       func(sender []byte, tc *transaction.TransferContract) error
	GetKAppControllerCalled                 func() kapp.KAppController
}

func (stub *BlockchainContextStub) InitState() {
	if stub.InitStateCalled != nil {
		stub.InitStateCalled()
	}
}

func (stub *BlockchainContextStub) PushState() {
	if stub.PushStateCalled != nil {
		stub.PushStateCalled()
	}
}

func (stub *BlockchainContextStub) PopSetActiveState() {
	if stub.PopSetActiveStateCalled != nil {
		stub.PopSetActiveStateCalled()
	}
}

func (stub *BlockchainContextStub) PopDiscard() {
	if stub.PopDiscardCalled != nil {
		stub.PopDiscardCalled()
	}
}

func (stub *BlockchainContextStub) ClearStateStack() {
	if stub.ClearStateStackCalled != nil {
		stub.ClearStateStackCalled()
	}
}

func (stub *BlockchainContextStub) NewAddress(creatorAddress []byte) ([]byte, error) {
	if stub.NewAddressCalled != nil {
		return stub.NewAddressCalled(creatorAddress)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) AccountExists(address []byte) bool {
	if stub.AccountExistsCalled != nil {
		return stub.AccountExistsCalled(address)
	}
	return false
}

func (stub *BlockchainContextStub) GetBalance(address []byte) []byte {
	if stub.GetBalanceCalled != nil {
		return stub.GetBalanceCalled(address)
	}
	return nil
}

func (stub *BlockchainContextStub) GetBalanceBigInt(address []byte) *big.Int {
	if stub.GetBalanceBigIntCalled != nil {
		return stub.GetBalanceBigIntCalled(address)
	}
	return big.NewInt(0)
}

func (stub *BlockchainContextStub) GetNonce(address []byte) (uint64, error) {
	if stub.GetNonceCalled != nil {
		return stub.GetNonceCalled(address)
	}
	return 0, nil
}

func (stub *BlockchainContextStub) CurrentEpoch() uint32 {
	if stub.CurrentEpochCalled != nil {
		return stub.CurrentEpochCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) GetStateRootHash() []byte {
	if stub.GetStateRootHashCalled != nil {
		return stub.GetStateRootHashCalled()
	}
	return nil
}

func (stub *BlockchainContextStub) LastTimeStamp() int64 {
	if stub.LastTimeStampCalled != nil {
		return stub.LastTimeStampCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) LastNonce() uint64 {
	if stub.LastNonceCalled != nil {
		return stub.LastNonceCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) LastSlot() uint64 {
	if stub.LastSlotCalled != nil {
		return stub.LastSlotCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) LastEpoch() uint32 {
	if stub.LastEpochCalled != nil {
		return stub.LastEpochCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) CurrentSlot() uint64 {
	if stub.CurrentSlotCalled != nil {
		return stub.CurrentSlotCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) CurrentNonce() uint64 {
	if stub.CurrentNonceCalled != nil {
		return stub.CurrentNonceCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) CurrentTimeStamp() int64 {
	if stub.CurrentTimeStampCalled != nil {
		return stub.CurrentTimeStampCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) CurrentRandomSeed() []byte {
	if stub.CurrentRandomSeedCalled != nil {
		return stub.CurrentRandomSeedCalled()
	}
	return nil
}

func (stub *BlockchainContextStub) LastRandomSeed() []byte {
	if stub.LastRandomSeedCalled != nil {
		return stub.LastRandomSeedCalled()
	}
	return nil
}

func (stub *BlockchainContextStub) IncreaseNonce(address []byte) {
	if stub.IncreaseNonceCalled != nil {
		stub.IncreaseNonceCalled(address)
	}
}

func (stub *BlockchainContextStub) GetCodeHash(address []byte) []byte {
	if stub.GetCodeHashCalled != nil {
		return stub.GetCodeHashCalled(address)
	}
	return nil
}

func (stub *BlockchainContextStub) GetCode(address []byte) ([]byte, error) {
	if stub.GetCodeCalled != nil {
		return stub.GetCodeCalled(address)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) GetCodeSize(address []byte) (int32, error) {
	if stub.GetCodeSizeCalled != nil {
		return stub.GetCodeSizeCalled(address)
	}
	return 0, nil
}

func (stub *BlockchainContextStub) BlockHash(nonce uint64) []byte {
	if stub.BlockHashCalled != nil {
		return stub.BlockHashCalled(nonce)
	}
	return nil
}

func (stub *BlockchainContextStub) GetOwnerAddress() ([]byte, error) {
	if stub.GetOwnerAddressCalled != nil {
		return stub.GetOwnerAddressCalled()
	}
	return nil, nil
}

func (stub *BlockchainContextStub) IsSmartContract(address []byte) bool {
	if stub.IsSmartContractCalled != nil {
		return stub.IsSmartContractCalled(address)
	}
	return false
}

func (stub *BlockchainContextStub) IsPayable(address []byte, scAddress []byte) (bool, error) {
	if stub.IsPayableCalled != nil {
		return stub.IsPayableCalled(address, scAddress)
	}
	return false, nil
}

func (stub *BlockchainContextStub) SaveCompiledCode(codeHash []byte, code []byte) {
	if stub.SaveCompiledCodeCalled != nil {
		stub.SaveCompiledCodeCalled(codeHash, code)
	}
}

func (stub *BlockchainContextStub) GetCompiledCode(codeHash []byte) (bool, []byte) {
	if stub.GetCompiledCodeCalled != nil {
		return stub.GetCompiledCodeCalled(codeHash)
	}
	return false, nil
}

func (stub *BlockchainContextStub) GetKDAToken(assetID []byte, userAddress []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
	if stub.GetKDATokenCalled != nil {
		return stub.GetKDATokenCalled(assetID, userAddress, nonce)
	}
	return nil, nil, nil
}

func (stub *BlockchainContextStub) GetSFTMeta(assetID []byte, nonce uint64) (*kapps.MetaV2, error) {
	if stub.GetSFTMetaCalled != nil {
		return stub.GetSFTMetaCalled(assetID, nonce)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) GetUserAccount(address []byte) (state.UserAccountHandler, error) {
	if stub.GetUserAccountCalled != nil {
		return stub.GetUserAccountCalled(address)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if stub.ProcessBuiltInFunctionCalled != nil {
		return stub.ProcessBuiltInFunctionCalled(input)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) GetSnapshot() int {
	if stub.GetSnapshotCalled != nil {
		return stub.GetSnapshotCalled()
	}
	return 0
}

func (stub *BlockchainContextStub) RevertToSnapshot(snapshot int) {
	if stub.RevertToSnapshotCalled != nil {
		stub.RevertToSnapshotCalled(snapshot)
	}
}

func (stub *BlockchainContextStub) IsLimitedTransfer(tokenID []byte) bool {
	if stub.IsLimitedTransferCalled != nil {
		return stub.IsLimitedTransferCalled(tokenID)
	}
	return false
}

func (stub *BlockchainContextStub) IsPaused(tokenID []byte) bool {
	if stub.IsPausedCalled != nil {
		return stub.IsPausedCalled(tokenID)
	}
	return false
}

func (stub *BlockchainContextStub) ClearCompiledCodes() {
	if stub.ClearCompiledCodesCalled != nil {
		stub.ClearCompiledCodesCalled()
	}
}

func (stub *BlockchainContextStub) ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	if stub.ExecuteSmartContractCallOnOtherVMCalled != nil {
		return stub.ExecuteSmartContractCallOnOtherVMCalled(input)
	}
	return nil, nil
}

func (stub *BlockchainContextStub) TransferValueOnly(destination []byte, sender []byte, value *big.Int) error {
	if stub.TransferValueOnlyCalled != nil {
		return stub.TransferValueOnlyCalled(destination, sender, value)
	}
	return nil
}

func (stub *BlockchainContextStub) KDATransfer(sender []byte, tc *transaction.TransferContract) error {
	if stub.KDATransferCalled != nil {
		return stub.KDATransferCalled(sender, tc)
	}
	return nil
}

func (stub *BlockchainContextStub) GetKAppController() kapp.KAppController {
	if stub.GetKAppControllerCalled != nil {
		return stub.GetKAppControllerCalled()
	}
	return nil
}
