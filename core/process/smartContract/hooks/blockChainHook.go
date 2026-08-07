package hooks

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path"
	"reflect"
	"strconv"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/factory/containers"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/core/process/smartContract"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	factoryHasher "github.com/klever-io/klever-go/crypto/hashing/factory"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	dataRetriever "github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

var _ process.BlockChainHookHandler = (*BlockChainHookImpl)(nil)

var log = logger.GetOrCreate("process/smartcontract/blockchainhook")

const defaultCompiledSCPath = "compiledSCStorage"
const executeDurationAlarmThreshold = time.Duration(50) * time.Millisecond

// ArgBlockChainHook represents the arguments structure for the blockchain hook
type ArgBlockChainHook struct {
	AccountsCacher     state.AccountsCacher
	KAppController     kapp.KAppController
	PubkeyConv         core.PubkeyConverter
	StorageService     dataRetriever.StorageService
	DataPool           dataRetriever.PoolsHolder
	BlockChain         data.ChainHandler
	Marshalizer        marshal.Marshalizer
	Uint64Converter    typeConverters.Uint64ByteSliceConverter
	BuiltInFunctions   vmcommon.BuiltInFunctionContainer
	EnableEpochs       config.EnableEpochs
	CompiledSCPool     storage.Cacher
	ConfigSCStorage    config.StorageConfig
	EpochNotifier      process.EpochNotifier
	ForkController     core.ForkController
	GasSchedule        core.GasScheduleNotifier
	WorkingDir         string
	NilCompiledSCStore bool
	Counter            BlockChainHookCounter
}

// BlockChainHookImpl is a wrapper over AccountsAdapter that satisfy vmcommon.BlockchainHook interface
type BlockChainHookImpl struct {
	accountsCacher   state.AccountsCacher
	kappController   kapp.KAppController
	pubkeyConv       core.PubkeyConverter
	storageService   dataRetriever.StorageService
	blockChain       data.ChainHandler
	marshalizer      marshal.Marshalizer
	uint64Converter  typeConverters.Uint64ByteSliceConverter
	builtInFunctions vmcommon.BuiltInFunctionContainer
	vmContainer      process.VirtualMachinesContainer
	forkController   core.ForkController
	counter          BlockChainHookCounter

	mutCurrentHdr sync.RWMutex
	currentHdr    data.HeaderHandler

	compiledScPool     storage.Cacher
	compiledScStorage  storage.Storer
	configSCStorage    config.StorageConfig
	workingDir         string
	nilCompiledSCStore bool

	mapActivationEpochs map[uint32]struct{}

	mutGasLock sync.RWMutex
}

// NewBlockChainHookImpl creates a new BlockChainHookImpl instance
func NewBlockChainHookImpl(
	args ArgBlockChainHook,
) (*BlockChainHookImpl, error) {
	err := checkForNil(args)
	if err != nil {
		return nil, err
	}

	blockChainHookImpl := &BlockChainHookImpl{
		accountsCacher:     args.AccountsCacher,
		kappController:     args.KAppController,
		pubkeyConv:         args.PubkeyConv,
		storageService:     args.StorageService,
		blockChain:         args.BlockChain,
		marshalizer:        args.Marshalizer,
		uint64Converter:    args.Uint64Converter,
		builtInFunctions:   args.BuiltInFunctions,
		compiledScPool:     args.CompiledSCPool,
		configSCStorage:    args.ConfigSCStorage,
		workingDir:         args.WorkingDir,
		nilCompiledSCStore: args.NilCompiledSCStore,
		forkController:     args.ForkController,
		counter:            args.Counter,
	}

	err = blockChainHookImpl.makeCompiledSCStorage()
	if err != nil {
		return nil, err
	}

	blockChainHookImpl.ClearCompiledCodes()
	blockChainHookImpl.currentHdr = &block.Block{}
	blockChainHookImpl.mapActivationEpochs = createMapActivationEpochs(&args.EnableEpochs)
	blockChainHookImpl.vmContainer = containers.NewVirtualMachinesContainer()

	args.EpochNotifier.RegisterNotifyHandler(blockChainHookImpl)
	args.GasSchedule.RegisterNotifyHandler(blockChainHookImpl)

	return blockChainHookImpl, nil
}

func createMapActivationEpochs(enableEpochs *config.EnableEpochs) map[uint32]struct{} {
	mapActivationEpoch := make(map[uint32]struct{})

	reflectVal := reflect.ValueOf(enableEpochs).Elem()
	for i := 0; i < reflectVal.NumField(); i++ {
		f := reflectVal.Field(i)
		epoch, ok := f.Interface().(uint32)
		if !ok {
			continue
		}
		mapActivationEpoch[epoch] = struct{}{}
	}
	return mapActivationEpoch
}

func checkForNil(args ArgBlockChainHook) error {
	if check.IfNil(args.AccountsCacher) {
		return process.ErrNilAccountsAdapter
	}
	if check.IfNil(args.KAppController) {
		return common.ErrNilKAppController
	}
	if check.IfNil(args.PubkeyConv) {
		return process.ErrNilPubkeyConverter
	}
	if check.IfNil(args.StorageService) {
		return process.ErrNilStorage
	}
	if check.IfNil(args.BlockChain) {
		return process.ErrNilBlockChain
	}
	if check.IfNil(args.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(args.Uint64Converter) {
		return process.ErrNilUint64Converter
	}
	if check.IfNil(args.BuiltInFunctions) {
		return process.ErrNilBuiltInFunction
	}
	if check.IfNil(args.CompiledSCPool) {
		return process.ErrNilCacher
	}
	if check.IfNil(args.EpochNotifier) {
		return process.ErrNilEpochNotifier
	}
	if check.IfNil(args.ForkController) {
		return process.ErrNilEnableEpochsHandler
	}
	if check.IfNil(args.Counter) {
		return ErrNilBlockchainHookCounter
	}

	return nil
}

// GetKAppController returns the kapp controller instance
func (bh *BlockChainHookImpl) GetKAppController() kapp.KAppController {
	return bh.kappController
}

// GetCode returns the code for the given account
func (bh *BlockChainHookImpl) GetCode(account state.UserAccountHandler) []byte {
	if check.IfNil(account) {
		return nil
	}

	return bh.accountsCacher.GetCode(account.GetCodeHash())
}

// GetUserAccount returns the balance of a shard account
func (bh *BlockChainHookImpl) GetUserAccount(address []byte) (state.UserAccountHandler, error) {
	defer stopMeasure(startMeasure("GetUserAccount"))

	dstAccount, err := bh.accountsCacher.GetExistingUser(address)
	if err != nil {
		return nil, err
	}

	return dstAccount, nil
}

// GetStorageData returns the storage value of a variable held in account's data trie
func (bh *BlockChainHookImpl) GetStorageData(accountAddress []byte, index []byte) ([]byte, uint32, error) {
	defer stopMeasure(startMeasure("GetStorageData"))

	err := bh.processMaxReadsCounters()
	if err != nil {
		return nil, 0, err
	}

	userAcc, err := bh.GetUserAccount(accountAddress)
	if errors.Is(err, common.ErrAccNotFound) {
		return make([]byte, 0), 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	value, err := userAcc.DataTrieTracker().RetrieveValue(index)
	messages := []interface{}{
		"address", accountAddress,
		"rootHash", userAcc.GetRootHash(),
		"key", index,
		"value", value,
	}
	if err != nil {
		messages = append(messages, "error")
		messages = append(messages, err)
	}
	log.Trace("GetStorageData ", messages...)

	// returning nil here ensures backwards compatibility as the error wasn't taken into account by the previous versions
	// of the vm. Now, the VM take into account this error so the processMaxReadsCounters call can stop the execution of the contract
	return value, 0, nil
}

func (bh *BlockChainHookImpl) processMaxReadsCounters() error {
	return bh.counter.ProcessCrtNumberOfTrieReadsCounter()
}

// GetBlockHash returns the header hash for a requested nonce delta
func (bh *BlockChainHookImpl) GetBlockHash(nonce uint64) ([]byte, error) {
	defer stopMeasure(startMeasure("GetBlockHash"))

	hdr := bh.blockChain.GetCurrentBlockHeader()

	if check.IfNil(hdr) {
		return nil, process.ErrNilBlockHeader
	}
	if nonce > hdr.GetNonce() {
		return nil, process.ErrInvalidNonceRequest
	}
	if nonce == hdr.GetNonce() {
		return bh.blockChain.GetCurrentBlockHeaderHash(), nil
	}

	header, hash, err := process.GetHeaderFromStorageWithNonce(
		nonce,
		bh.storageService,
		bh.uint64Converter,
		bh.marshalizer,
	)
	if err != nil {
		return nil, err
	}

	if header.GetEpoch() != hdr.GetEpoch() {
		return nil, process.ErrInvalidBlockRequestOldEpoch
	}

	return hash, nil
}

// GetSFTMeta returns the metadata for a given asset ID and nonce
func (bh *BlockChainHookImpl) GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error) {
	sysKapp := bh.kappController.GetSystemAccountKApp()
	return sysKapp.SFTGetMeta(tokenID, []byte(fmt.Sprint(nonce)))
}

// LastNonce returns the nonce from the last committed block
func (bh *BlockChainHookImpl) LastNonce() uint64 {
	if !check.IfNil(bh.blockChain.GetCurrentBlockHeader()) {
		return bh.blockChain.GetCurrentBlockHeader().GetNonce()
	}
	return 0
}

// LastBlock returns the last committed block
func (bh *BlockChainHookImpl) LastBlock() data.HeaderHandler {
	return bh.blockChain.GetCurrentBlockHeader()
}

// LastSlot returns the slot from the last committed block
func (bh *BlockChainHookImpl) LastSlot() uint64 {
	if !check.IfNil(bh.blockChain.GetCurrentBlockHeader()) {
		return bh.blockChain.GetCurrentBlockHeader().GetSlot()
	}
	return 0
}

// LastTimeStamp returns the timeStamp from the last committed block
func (bh *BlockChainHookImpl) LastTimeStamp() int64 {
	if !check.IfNil(bh.blockChain.GetCurrentBlockHeader()) {
		return bh.blockChain.GetCurrentBlockHeader().GetTimestamp()
	}
	return 0
}

// LastRandomSeed returns the random seed from the last committed block
func (bh *BlockChainHookImpl) LastRandomSeed() []byte {
	if !check.IfNil(bh.blockChain.GetCurrentBlockHeader()) {
		return bh.blockChain.GetCurrentBlockHeader().GetRandSeed()
	}
	return make([]byte, 0)
}

// LastEpoch returns the epoch from the last committed block
func (bh *BlockChainHookImpl) LastEpoch() uint32 {
	if !check.IfNil(bh.blockChain.GetCurrentBlockHeader()) {
		return bh.blockChain.GetCurrentBlockHeader().GetEpoch()
	}
	return 0
}

// GetStateRootHash returns the state root hash from the last committed block
func (bh *BlockChainHookImpl) GetStateRootHash() []byte {
	rootHash := bh.blockChain.GetCurrentBlockRootHash()
	if len(rootHash) > 0 {
		return rootHash
	}

	return make([]byte, 0)
}

// CurrentNonce returns the nonce from the current block
func (bh *BlockChainHookImpl) CurrentNonce() uint64 {
	bh.mutCurrentHdr.RLock()
	defer bh.mutCurrentHdr.RUnlock()

	// check if the current header is valid, if not return from blockChain data
	if len(bh.currentHdr.GetParentHash()) == 0 {
		return bh.LastNonce()
	}

	return bh.currentHdr.GetNonce()
}

// CurrentSlot returns the slot from the current block
func (bh *BlockChainHookImpl) CurrentSlot() uint64 {
	bh.mutCurrentHdr.RLock()
	defer bh.mutCurrentHdr.RUnlock()

	// check if the current header is valid, if not return from blockChain data
	if len(bh.currentHdr.GetParentHash()) == 0 {
		return bh.LastSlot()
	}

	return bh.currentHdr.GetSlot()
}

// CurrentTimeStamp return the timestamp from the current block
func (bh *BlockChainHookImpl) CurrentTimeStamp() int64 {
	bh.mutCurrentHdr.RLock()
	defer bh.mutCurrentHdr.RUnlock()

	// check if the current header is valid, if not return from blockChain data
	if len(bh.currentHdr.GetParentHash()) == 0 {
		return bh.LastTimeStamp()
	}

	return bh.currentHdr.GetTimestamp()
}

// CurrentRandomSeed returns the random seed from the current header
func (bh *BlockChainHookImpl) CurrentRandomSeed() []byte {
	bh.mutCurrentHdr.RLock()
	defer bh.mutCurrentHdr.RUnlock()

	// check if the current header is valid, if not return from blockChain data
	if len(bh.currentHdr.GetParentHash()) == 0 {
		return bh.LastRandomSeed()
	}

	return bh.currentHdr.GetRandSeed()
}

// CurrentEpoch returns the current epoch
func (bh *BlockChainHookImpl) CurrentEpoch() uint32 {
	bh.mutCurrentHdr.RLock()
	defer bh.mutCurrentHdr.RUnlock()

	// check if the current header is valid, if not return from blockChain data
	if len(bh.currentHdr.GetParentHash()) == 0 {
		return bh.LastEpoch()
	}

	return bh.currentHdr.GetEpoch()
}

// NewAddress is a hook which creates a new smart contract address from the creators address and nonce
// The address is created by applied keccak256 on the appended value off creator address and nonce
// Prefix mask is applied for first 8 bytes 0, and for bytes 9-10 - VM type
// Suffix mask is applied - last 2 bytes are for the shard ID - mask is applied as suffix mask
func (bh *BlockChainHookImpl) NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte, randomSeed []byte) ([]byte, error) {
	addressLength := bh.pubkeyConv.Len()
	if len(creatorAddress) != addressLength {
		return nil, ErrAddressLengthNotCorrect
	}

	if len(vmType) != core.VMTypeLen {
		return nil, ErrVMTypeLengthIsNotCorrect
	}

	base := hashFromAddressAndNonce(creatorAddress, creatorNonce, randomSeed)
	prefixMask := createPrefixMask(vmType)
	suffixMask := createSuffixMask(creatorAddress)

	copy(base[:core.NumInitCharactersForScAddress], prefixMask)
	copy(base[len(base)-core.ShardIdentiferLen:], suffixMask)

	return base, nil
}

// ProcessBuiltInFunction is the hook through which a smart contract can execute a built-in function
func (bh *BlockChainHookImpl) ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	defer stopMeasure(startMeasure("ProcessBuiltInFunction"))

	if input == nil {
		return nil, process.ErrNilVmInput
	}

	function, err := bh.builtInFunctions.Get(input.Function)
	if err != nil {
		return nil, err
	}

	// Every built-in registered in the container is a state-mutating contract type,
	// so a read-only execution context (VM query) must refuse all of them here.
	// Guarding at the dispatch point rather than inside each KApp also covers
	// ChangeOwnerAddress, which writes straight through the cacher without going
	// through the controller.
	if check.IfNil(bh.kappController) || bh.kappController.IsReadOnly() {
		return nil, process.ErrReadOnlyKAppMutation
	}

	err = bh.processMaxBuiltInCounters(input)
	if err != nil {
		return nil, err
	}

	ctx := bh.kappController.GetCurrentKAppContext()
	initialLen := len(ctx.Receipts().Get())

	vmOutput, err := function.ProcessBuiltinFunction(input)
	if err != nil {
		return nil, err
	}

	// load return from context
	vmOutput.ReturnData = ctx.GetAndClearReturnData()

	if !core.IsSmartContractAddress(input.CallerAddr) {
		return vmOutput, nil
	}

	index := uint32(1)

	for i := initialLen; i < len(ctx.Receipts().Get()); i++ {
		receipt := ctx.Receipts().Get()[i]

		if len(receipt.Data) == 0 || len(receipt.Data[0]) == 0 {
			continue
		}

		receiptType := receipt.Data[0][0]
		if txProcess.ReceiptType(receiptType) != txProcess.Transfer {
			continue
		}

		// check valid index data len
		if len(receipt.Data) < 7 {
			log.Warn("ProcessBuiltInFunction: missing transfer data", "len", len(receipt.Data))
			return nil, fmt.Errorf("ProcessBuiltInFunction: missing transfer data")
		}

		txValueInt, err := strconv.ParseInt(string(receipt.Data[3]), 10, 64)
		if err != nil {
			return nil, err
		}

		if len(receipt.Data[6]) == 0 {
			log.Warn("ProcessBuiltInFunction: missing asset type data", "len", len(receipt.Data[6]))
			return nil, fmt.Errorf("ProcessBuiltInFunction: missing token type")
		}

		tokenType := kapps.KDAData_EnumAssetType(receipt.Data[6][0])

		nonce := uint64(0)
		if tokenType != kapps.KDAData_Fungible {
			nonce, err = strconv.ParseUint(string(receipt.Data[5]), 10, 64)
			if err != nil {
				log.Warn("ProcessBuiltInFunction: invalid nonce", "nonce", string(receipt.Data[5]))
				return nil, err
			}
		}

		addOutputTransferToVMOutput(
			index,
			receipt.Data[1],
			receipt.Data[2],
			&vmcommon.KDATransfer{
				KDAValue:      big.NewInt(txValueInt),
				KDATokenName:  receipt.Data[4],
				KDATokenType:  uint32(tokenType), // #nosec G115 - type casting
				KDATokenNonce: nonce,
			},
			vmOutput,
		)

		index++
	}

	return vmOutput, nil
}

func (bh *BlockChainHookImpl) processMaxBuiltInCounters(input *vmcommon.ContractCallInput) error {
	return bh.counter.ProcessMaxBuiltInCounters(input)
}

// IsSmartContract returns whether the address points to a smart contract
func (bh *BlockChainHookImpl) IsSmartContract(address []byte) bool {
	if !core.IsSmartContractAddress(address) {
		return false
	}
	scAccount, err := bh.GetUserAccount(address)
	if err != nil { // common.ErrAccNotFound
		return false
	}

	// deleted contracts still have code metadata, and still need to be
	// considered as smart contracts
	if len(scAccount.GetCodeMetadata()) > 0 {
		return true
	}
	// fallback, if no code metadata, check if the code is present
	return len(bh.accountsCacher.GetCode(scAccount.GetCodeHash())) > 0
}

// IsPayable checks whether the provided address can receive KDA or not
func (bh *BlockChainHookImpl) IsPayable(sndAddress []byte, recvAddress []byte) (bool, error) {
	if core.IsSystemAccountAddress(recvAddress) {
		return false, nil
	}
	// redundant because of the code below, but worth keeping for performance
	// the other checks uses cacher/storage to do so.
	if !core.IsSmartContractAddress(recvAddress) {
		return true, nil
	}

	if core.IsSmartContractAddress(recvAddress) && !bh.IsSmartContract(recvAddress) {
		// This is a smart contract address format but not properly initialized
		// blocking transfers to avoid unwanted scaddress initialization
		return false, nil
	}

	if !bh.IsSmartContract(recvAddress) {
		return true, nil
	}

	userAcc, err := bh.GetUserAccount(recvAddress)
	if err != nil {
		return false, err
	}

	metadata := vmcommon.CodeMetadataFromBytes(userAcc.GetCodeMetadata())
	if bh.IsSmartContract(sndAddress) {
		return metadata.PayableBySC, nil
	}

	return metadata.Payable, nil
}

// FilterCodeMetadataForUpgrade will filter the provided input bytes as a correctly constructed vmcommon.CodeMetadata bytes
// taking into account the activation flags for the future flags. This should be used in the upgrade SC process
func (bh *BlockChainHookImpl) FilterCodeMetadataForUpgrade(input []byte) ([]byte, error) {
	raw := vmcommon.CodeMetadataFromBytes(input)
	if bytes.Equal(input, raw.ToBytes()) {
		return raw.ToBytes(), nil
	}

	return nil, parsers.ErrInvalidCodeMetadata
}

// GetBuiltinFunctionNames returns the built-in function names
func (bh *BlockChainHookImpl) GetBuiltinFunctionNames() vmcommon.FunctionNames {
	return bh.builtInFunctions.Keys()
}

// GetBuiltinFunctionsContainer returns the built-in functions container
func (bh *BlockChainHookImpl) GetBuiltinFunctionsContainer() vmcommon.BuiltInFunctionContainer {
	return bh.builtInFunctions
}

// GetAllState returns the underlying state of a given account
func (bh *BlockChainHookImpl) GetAllState(_ []byte) (map[string][]byte, error) {
	return nil, nil
}

// GetKDAToken returns the unmarshalled kda data for the given key
func (bh *BlockChainHookImpl) GetKDAToken(address []byte, assetID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
	kda := &kapps.KDAData{}
	userKDA := &kapps.UserKDA{}
	var err error
	if address != nil {
		acc, err := bh.GetUserAccount(address)
		if err != nil {
			return kda, userKDA, err
		}

		userKDA, err = acc.GetUserKDA(assetID, []byte(fmt.Sprint(nonce)), bh.forkController.EnableSmartContracts())
		if err != nil {
			return kda, userKDA, err
		}
	}

	_, kda, err = bh.kappController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return kda, userKDA, err
	}

	return kda, userKDA, nil
}

func hashFromAddressAndNonce(creatorAddress []byte, creatorNonce uint64, randomSeed []byte) []byte {
	buffNonce := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffNonce, creatorNonce)
	adrAndNonce := append(creatorAddress, buffNonce...)
	adrAndNonce = append(adrAndNonce, randomSeed...)

	hasher, err := factoryHasher.NewHasher("keccak")
	if err != nil {
		return []byte{}
	}

	scAddress := hasher.Compute(string(adrAndNonce))

	return scAddress
}

func createPrefixMask(vmType []byte) []byte {
	prefixMask := make([]byte, core.NumInitCharactersForScAddress-core.VMTypeLen)
	prefixMask = append(prefixMask, vmType...)

	return prefixMask
}

func createSuffixMask(creatorAddress []byte) []byte {
	return creatorAddress[len(creatorAddress)-2:]
}

// SetCurrentHeader sets current header to be used by smart contracts
func (bh *BlockChainHookImpl) SetCurrentHeader(hdr data.HeaderHandler) {
	if check.IfNil(hdr) {
		return
	}

	bh.mutCurrentHdr.Lock()
	bh.currentHdr = hdr
	bh.mutCurrentHdr.Unlock()
}

// SaveCompiledCode saves the compiled code to cache and storage
func (bh *BlockChainHookImpl) SaveCompiledCode(codeHash []byte, code []byte) {
	bh.compiledScPool.Put(codeHash, code, len(code))
	err := bh.compiledScStorage.Put(codeHash, code)
	if err != nil {
		log.Debug("BlockChainHookImpl.SaveCompiledCode: compiledScStorage.Put",
			"error", err, "codeHash", codeHash)
	}
}

// GetCompiledCode returns the compiled code if it is found in the cache or storage
func (bh *BlockChainHookImpl) GetCompiledCode(codeHash []byte) (bool, []byte) {
	val, found := bh.compiledScPool.Get(codeHash)
	if found {
		compiledCode, ok := val.([]byte)
		if ok {
			return true, compiledCode
		}
	}

	compiledCode, err := bh.compiledScStorage.Get(codeHash)
	if err != nil || len(compiledCode) == 0 {
		return false, nil
	}

	bh.compiledScPool.Put(codeHash, compiledCode, len(compiledCode))

	return true, compiledCode
}

// DeleteCompiledCode deletes the compiled code from storage and cache
func (bh *BlockChainHookImpl) DeleteCompiledCode(codeHash []byte) {
	bh.compiledScPool.Remove(codeHash)
	err := bh.compiledScStorage.Remove(codeHash)
	if err != nil {
		log.Debug("BlockChainHookImpl.DeleteCompiledCode: compiledScStorage.Remove",
			"error", err, "codeHash", codeHash)
	}
}

// Close closes/cleans up the blockchain hook
func (bh *BlockChainHookImpl) Close() error {
	bh.compiledScPool.Clear()
	return bh.compiledScStorage.DestroyUnit()
}

// ClearCompiledCodes deletes the compiled codes from storage and cache
func (bh *BlockChainHookImpl) ClearCompiledCodes() {
	bh.compiledScPool.Clear()
	err := bh.compiledScStorage.DestroyUnit()
	if err != nil {
		log.Debug("BlockChainHookImpl.ClearCompiledCodes: compiledScStorage.DestroyUnit", "error", err)
	}

	err = bh.makeCompiledSCStorage()
	if err != nil {
		log.Debug("BlockChainHookImpl.ClearCompiledCodes: makeCompiledSCStorage", "error", err)
	}
}

func (bh *BlockChainHookImpl) makeCompiledSCStorage() error {
	if bh.nilCompiledSCStore {
		bh.compiledScStorage = storageUnit.NewNilStorer()
		return nil
	}

	dbConfig := factory.GetDBFromConfig(bh.configSCStorage.DB)
	dbConfig.FilePath = path.Join(bh.workingDir, defaultCompiledSCPath, bh.configSCStorage.DB.FilePath)
	store, err := storageUnit.NewStorageUnitFromConf(
		factory.GetCacherFromConfig(bh.configSCStorage.Cache),
		dbConfig,
		factory.GetBloomFromConfig(bh.configSCStorage.Bloom),
	)
	if err != nil {
		return err
	}

	bh.compiledScStorage = store
	return nil
}

// GetSnapshot gets the number of entries in the journal as a snapshot id
func (bh *BlockChainHookImpl) GetSnapshot() int {
	return 0 //not needed as we use accounts cacher
}

// RevertToSnapshot reverts snapshots up to the specified one
func (bh *BlockChainHookImpl) RevertToSnapshot(snapshot int) error {
	return nil ////not needed as we use accounts cacher
}

// TransferValueOnly transfers KLV from one account to another
func (bh *BlockChainHookImpl) TransferValueOnly(destination []byte, sender []byte, value *big.Int) error {
	log.Trace("TransferValueOnly", "destination", destination, "sender", sender, "value", value)

	tc := &transaction.TransferContract{
		ToAddress: destination,
		AssetID:   kdautils.KLVIdentifier,
		Amount:    value.Int64(),
	}

	return bh.KDATransfer(sender, tc)
}

func (bh *BlockChainHookImpl) KDATransfer(sender []byte, tc *transaction.TransferContract) error {
	accKapp := bh.GetKAppController().GetAccountsKApp()

	resultCode, err := accKapp.Transfer(transaction.TXContract_TransferContractType, sender, tc)
	if err != nil || resultCode != transaction.Transaction_Ok {
		return fmt.Errorf("result code: %d, %v", resultCode, err)
	}

	return nil
}

// IncreaseNonce increase the nonce of given address
func (bh *BlockChainHookImpl) IncreaseNonce(address []byte) error {
	user, err := bh.kappController.GetAccountsKApp().GetAccountsCacher().GetExistingUser(address)
	if err != nil {
		return err
	}
	user.IncreaseNonce(1)
	return nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (bh *BlockChainHookImpl) EpochConfirmed(epoch uint32) {
	_, ok := bh.mapActivationEpochs[epoch]
	if ok {
		bh.ClearCompiledCodes()
	}
}

// ExecuteSmartContractCallOnOtherVM on another VM
func (bh *BlockChainHookImpl) ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	vmExec, err := smartContract.FindVMByScAddress(bh.vmContainer, input.RecipientAddr)
	if err != nil {
		return nil, err
	}
	return vmExec.RunSmartContractCall(input)
}

// GasScheduleChange sets the new gas schedule where it is needed
func (bh *BlockChainHookImpl) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	maxPerTransaction := bh.getMaxPerTransactionValues(gasSchedule)
	if maxPerTransaction == nil {
		log.Error("maxPerTransaction definition is missing in the current gas schedule, using old values")
		return
	}

	bh.counter.SetMaximumValues(maxPerTransaction)
}

func (bh *BlockChainHookImpl) getMaxPerTransactionValues(gasSchedule map[string]map[string]uint64) map[string]uint64 {
	bh.mutGasLock.Lock()
	defer bh.mutGasLock.Unlock()

	maxPerTransaction := gasSchedule[common.MaxPerTransaction]
	if maxPerTransaction == nil {
		return nil
	}

	result := make(map[string]uint64)
	for key, value := range maxPerTransaction {
		result[key] = value
	}

	return result
}

// ResetCounters resets the state counters for the blockchain hook
func (bh *BlockChainHookImpl) ResetCounters() {
	bh.counter.ResetCounters()
}

// GetCounterValues returns the current counter values
func (bh *BlockChainHookImpl) GetCounterValues() map[string]uint64 {
	return bh.counter.GetCounterValues()
}

// IsInterfaceNil returns true if there is no value under the interface
func (bh *BlockChainHookImpl) IsInterfaceNil() bool {
	return bh == nil
}

func startMeasure(hook string) (string, *tools.StopWatch) {
	sw := tools.NewStopWatch()
	sw.Start(hook)
	return hook, sw
}

func stopMeasure(hook string, sw *tools.StopWatch) {
	sw.Stop(hook)

	duration := sw.GetMeasurement(hook)
	if duration > executeDurationAlarmThreshold {
		log.Debug(fmt.Sprintf("%s took > %s", hook, executeDurationAlarmThreshold), "duration", duration)
	} else {
		log.Trace(hook, "duration", duration)
	}
}

func addOutputTransferToVMOutput(
	index uint32,
	senderAddress []byte,
	recipient []byte,
	kdaTransfer *vmcommon.KDATransfer,
	vmOutput *vmcommon.VMOutput,
) {
	outTransfer := vmcommon.OutputTransfer{
		Index:         index,
		SenderAddress: senderAddress,
		RcvAddr:       recipient,
		KDATransfers:  kdaTransfer.Clone(),
	}

	if vmOutput.OutputAccounts == nil {
		vmOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
	}

	outAcc, ok := vmOutput.OutputAccounts[string(recipient)]
	if !ok {
		vmOutput.OutputAccounts[string(recipient)] = &vmcommon.OutputAccount{
			Address:         recipient,
			OutputTransfers: []vmcommon.OutputTransfer{outTransfer},
		}
		return
	}

	outAcc.OutputTransfers = append(outAcc.OutputTransfers, outTransfer)
}
