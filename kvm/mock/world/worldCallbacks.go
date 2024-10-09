package worldmock

import (
	"bytes"
	"errors"
	"math/big"
	"strconv"

	"github.com/klever-io/klever-go/data/transaction"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ vmcommon.BlockchainHook = (*MockWorld)(nil)
var _ process.BlockChainHookHandler = (*MockWorld)(nil)

// ErrBuiltinFuncWrapperNotInitialized means that the builtin function wrapper was used before initialization.
var ErrBuiltinFuncWrapperNotInitialized = errors.New("builtin function not found or container not initialized")

// NewAddress provides the address for a new account.
// It looks up the explicit new address mocks, if none found generates one using a fake but realistic algorithm.
func (b *MockWorld) NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte) ([]byte, error) {
	// custom error
	if b.Err != nil {
		return nil, b.Err
	}

	// explicit new address mocks
	// matched by creator address and nonce
	for _, newAddressMock := range b.NewAddressMocks {
		if bytes.Equal(creatorAddress, newAddressMock.CreatorAddress) && creatorNonce == newAddressMock.CreatorNonce {
			b.LastCreatedContractAddress = newAddressMock.NewAddress
			return newAddressMock.NewAddress, nil
		}
	}

	// If a mock address wasn't registered for the specified creatorAddress, generate one automatically.
	// This is not the real algorithm but it's simple and close enough.
	result := GenerateMockAddress(creatorAddress, creatorNonce, vmType)
	b.LastCreatedContractAddress = result
	return result, nil
}

// GetStorageData yields the storage value for a certain account and storage key.
// Should return an empty byte array if the key is missing from the account storage
func (b *MockWorld) GetStorageData(accountAddress []byte, key []byte) ([]byte, uint32, error) {
	// custom error
	if b.Err != nil {
		return nil, 0, b.Err
	}

	userAcc, err := b.AccountsCacher.GetExistingUser(accountAddress)
	if errors.Is(err, common.ErrAccNotFound) {
		return make([]byte, 0), 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	value, err := userAcc.DataTrieTracker().RetrieveValue(key)
	if err == common.ErrNilTrie {
		return make([]byte, 0), 0, nil

	}

	return value, 0, err
}

// GetBlockHash should return the hash of the nth previous blockchain.
// Offset specifies how many blocks we need to look back.
func (b *MockWorld) GetBlockHash(nonce uint64) ([]byte, error) {
	if b.Err != nil {
		return nil, b.Err
	}
	currentNonce := b.CurrentNonce()
	if nonce > currentNonce {
		return nil, errors.New("requested nonce is greater than current nonce")
	}
	offsetInt32 := int(currentNonce - nonce) // #nosec G115
	if offsetInt32 >= len(b.Blockhashes) {
		return nil, errors.New("requested nonce is older than the oldest available block nonce")
	}
	return b.Blockhashes[offsetInt32], nil
}

// LastNonce returns the nonce from from the last committed block
func (b *MockWorld) LastNonce() uint64 {
	if b.PreviousBlockInfo == nil {
		return 0
	}
	return b.PreviousBlockInfo.BlockNonce
}

// LastSlot returns the Slot from the last committed block
func (b *MockWorld) LastSlot() uint64 {
	if b.PreviousBlockInfo == nil {
		return 0
	}
	return b.PreviousBlockInfo.BlockSlot
}

// LastTimeStamp returns the timeStamp from the last committed block
func (b *MockWorld) LastTimeStamp() int64 {
	if b.PreviousBlockInfo == nil {
		return 0
	}
	return b.PreviousBlockInfo.BlockTimestamp
}

// LastRandomSeed returns the random seed from the last committed block
func (b *MockWorld) LastRandomSeed() []byte {
	if b.PreviousBlockInfo == nil {
		return nil
	}
	return b.PreviousBlockInfo.GetRandomSeedSlice()
}

// LastEpoch returns the epoch from the last committed block
func (b *MockWorld) LastEpoch() uint32 {
	if b.PreviousBlockInfo == nil {
		return 0
	}
	return b.PreviousBlockInfo.BlockEpoch
}

// GetStateRootHash returns the state root hash from the last committed block
func (b *MockWorld) GetStateRootHash() []byte {
	return b.StateRootHash
}

// CurrentNonce returns the nonce from the current block
func (b *MockWorld) CurrentNonce() uint64 {
	if b.CurrentBlockInfo == nil {
		return 0
	}
	return b.CurrentBlockInfo.BlockNonce
}

// CurrentSlot returns the Slot from the current block
func (b *MockWorld) CurrentSlot() uint64 {
	if b.CurrentBlockInfo == nil {
		return 0
	}
	return b.CurrentBlockInfo.BlockSlot
}

// CurrentTimeStamp return the timestamp from the current block
func (b *MockWorld) CurrentTimeStamp() int64 {
	if b.CurrentBlockInfo == nil {
		return 0
	}
	return b.CurrentBlockInfo.BlockTimestamp
}

// CurrentRandomSeed returns the random seed from the current header
func (b *MockWorld) CurrentRandomSeed() []byte {
	if b.CurrentBlockInfo == nil {
		return nil
	}
	return b.CurrentBlockInfo.GetRandomSeedSlice()
}

// CurrentEpoch returns the current epoch
func (b *MockWorld) CurrentEpoch() uint32 {
	if b.CurrentBlockInfo == nil {
		return 0
	}
	return b.CurrentBlockInfo.BlockEpoch
}

// ProcessBuiltInFunction -
func (b *MockWorld) ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	// custom error
	if b.Err != nil {
		return nil, b.Err
	}

	if b.BuiltinFuncs == nil {
		return nil, ErrBuiltinFuncWrapperNotInitialized
	}

	return b.BuiltinFuncs.ProcessBuiltInFunction(input)
}

// GetKDAToken -
func (b *MockWorld) GetKDAToken(address []byte, tokenIdentifier []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
	// custom error
	if b.Err != nil {
		return nil, nil, b.Err
	}

	// mock for unit tests
	if b.MockAsset != nil && bytes.Equal(tokenIdentifier, b.MockAsset.ID) {
		return b.MockAsset, nil, nil
	}

	if b.BuiltinFuncs == nil {
		return nil, nil, ErrBuiltinFuncWrapperNotInitialized
	}

	// convert nonce
	nonceBytes := []byte(strconv.FormatUint(nonce, 10))

	user, err := b.AccountsCacher.GetExistingUser(address)
	if err != nil {
		return nil, nil, err
	}
	userKda, err := user.GetUserKDA(tokenIdentifier, nonceBytes, true)
	if err != nil {
		return nil, nil, err
	}
	// klv does not save its balance in a KDA instance
	if bytes.Equal(tokenIdentifier, kdautils.KLVIdentifier) {
		userKda.Balance = user.GetBalance(kdautils.KLVIdentifier, true)
	}
	kdaData, err := b.GetKDAData(tokenIdentifier, nonceBytes)
	if err != nil {
		return nil, nil, err
	}

	return kdaData, userKda, nil
}

// GetBuiltinFunctionNames -
func (b *MockWorld) GetBuiltinFunctionNames() vmcommon.FunctionNames {
	return b.BuiltinFuncs.GetBuiltinFunctionNames()
}

// GetAllState simply returns the storage as-is.
func (b *MockWorld) GetAllState(accountAddress []byte) (map[string][]byte, error) {
	return b.ProvidedBlockchainHook.GetAllState(accountAddress)
}

// GetUserAccount retrieves account info from map, or error if not found.
func (b *MockWorld) GetUserAccount(address []byte) (state.UserAccountHandler, error) {
	if b.Err != nil {
		return nil, b.Err
	}
	return b.AccountsCacher.GetExistingUser(address)
}

// GetCode retrieves the code from the given account, or nil if not found
func (b *MockWorld) GetCode(acc state.UserAccountHandler) []byte {
	return b.AccountsCacher.GetCode(acc.GetCodeHash())
}

// IsSmartContract -
func (b *MockWorld) IsSmartContract(address []byte) bool {
	return IsSmartContractAddress(address)
}

// IsPayable -
func (b *MockWorld) IsPayable(sndAddress []byte, rcvAddress []byte) (bool, error) {
	if !b.IsSmartContract(rcvAddress) {
		return true, nil
	}

	account, err := b.AccountsCacher.GetExistingUser(rcvAddress)
	if err == common.ErrAccNotFound {
		// if new contract, allow deployer to deposit
		return true, nil
	}
	if err != nil {
		return false, err
	}

	metadata := vmcommon.CodeMetadataFromBytes(account.GetCodeMetadata())
	if IsSmartContractAddress(sndAddress) {
		return metadata.PayableBySC, nil
	}

	return metadata.Payable, nil
}

// SaveCompiledCode -
func (b *MockWorld) SaveCompiledCode(codeHash []byte, code []byte) {
	b.CompiledCode[string(codeHash)] = code
}

// GetCompiledCode -
func (b *MockWorld) GetCompiledCode(codeHash []byte) (bool, []byte) {
	code, found := b.CompiledCode[string(codeHash)]
	return found, code
}

// ClearCompiledCodes -
func (b *MockWorld) ClearCompiledCodes() {
	b.CompiledCode = make(map[string][]byte)
}

// TransferValueOnly -
func (b *MockWorld) TransferValueOnly(to []byte, from []byte, value *big.Int) error {
	tc := &transaction.TransferContract{
		ToAddress: to,
		AssetID:   kdautils.KLVIdentifier,
		Amount:    value.Int64(),
	}
	return b.KDATransfer(from, tc)
}

func (b *MockWorld) KDATransfer(sender []byte, tc *transaction.TransferContract) error {
	fromAccount, err := b.AccountsCacher.LoadUser(sender)
	if err != nil {
		return err
	}
	toAccount, err := b.AccountsCacher.LoadUser(tc.ToAddress)
	if err != nil {
		return err
	}
	err = fromAccount.SubFromBalance(tc.Amount, tc.AssetID, true)
	if err != nil {
		return err
	}
	err = toAccount.AddToBalance(tc.Amount, tc.AssetID, true)
	if err != nil {
		return err
	}

	return nil
}

// IncreaseNonce -
func (b *MockWorld) IncreaseNonce(address []byte) error {
	account, err := b.AccountsCacher.LoadUser(address)
	if err != nil {
		return err
	}

	account.IncreaseNonce(1)
	return nil
}

// Close implements process.BlockChainHookHandler.
func (b *MockWorld) Close() error {
	panic("unimplemented")
}

// DeleteCompiledCode implements process.BlockChainHookHandler.
func (b *MockWorld) DeleteCompiledCode(codeHash []byte) {
	panic("unimplemented")
}

// FilterCodeMetadataForUpgrade implements process.BlockChainHookHandler.
func (b *MockWorld) FilterCodeMetadataForUpgrade(input []byte) ([]byte, error) {
	panic("unimplemented")
}

// GetBuiltinFunctionsContainer implements process.BlockChainHookHandler.
func (b *MockWorld) GetBuiltinFunctionsContainer() vmcommon.BuiltInFunctionContainer {
	panic("unimplemented")
}

// GetCounterValues implements process.BlockChainHookHandler.
func (b *MockWorld) GetCounterValues() map[string]uint64 {
	panic("unimplemented")
}

// GetKAppController implements process.BlockChainHookHandler.
func (b *MockWorld) GetKAppController() kapp.KAppController {
	return b.KAppController
}

// LastBlock implements process.BlockChainHookHandler.
func (b *MockWorld) LastBlock() data.HeaderHandler {
	panic("unimplemented")
}

// ResetCounters implements process.BlockChainHookHandler.
func (b *MockWorld) ResetCounters() {
	panic("unimplemented")
}

// SetCurrentHeader implements process.BlockChainHookHandler.
func (b *MockWorld) SetCurrentHeader(hdr data.HeaderHandler) {
	panic("unimplemented")
}

// IsInterfaceNil returns true if underlying implementation is nil
func (b *MockWorld) IsInterfaceNil() bool {
	return b == nil
}
