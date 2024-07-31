package process

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"sort"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/smartContractResult"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/vmcommon"
)

var log = logger.GetOrCreate("process")

// ForkInfo hold the data related to a detected fork
type ForkInfo struct {
	IsDetected bool
	Nonce      uint64
	Slot       uint64
	Hash       []byte
}

// GetHeaderFromPoolWithNonce method returns a meta block header from pool with a given nonce
func GetHeaderFromPoolWithNonce(
	nonce uint64,
	headersCacher retriever.HeadersPool,
) (*block.Block, []byte, error) {

	obj, hash, err := getHeaderFromPoolWithNonce(nonce, headersCacher)
	if err != nil {
		return nil, nil, err
	}

	hdr, ok := obj.(*block.Block)
	if !ok {
		return nil, nil, common.ErrWrongTypeAssertion
	}

	// check if signed
	if hdr.ProducerSignature == nil || hdr.Signature == nil {
		log.Trace("GetHeaderFromPoolWithNonce.RemoveHeaderByNonce", "nonce", nonce)
		headersCacher.RemoveHeaderByNonce(nonce)

		return nil, nil, common.ErrInvalidSignatureLength
	}

	return hdr, hash, nil
}

func getHeaderFromPoolWithNonce(
	nonce uint64,
	headersCacher retriever.HeadersPool,
) (interface{}, []byte, error) {

	if check.IfNil(headersCacher) {
		return nil, nil, common.ErrNilCacher
	}

	headers, hashes, err := headersCacher.GetHeadersByNonce(nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("%w : getHeaderFromPoolWithNonce nonce = %d",
			ErrMissingHeader, nonce)
	}

	//TODO what should we do when we get from pool more than one header with same nonce
	return headers[len(headers)-1], hashes[len(hashes)-1], nil
}

// GetHeader gets the header, which is associated with the given hash, from pool or storage
func GetHeader(
	hash []byte,
	headersCacher retriever.HeadersPool,
	marshalizer marshal.Marshalizer,
	storageService retriever.StorageService,
) (*block.Block, error) {

	err := checkGetHeaderParamsForNil(headersCacher, marshalizer, storageService)
	if err != nil {
		return nil, err
	}

	hdr, err := GetHeaderFromPool(hash, headersCacher)
	if err != nil {
		hdr, err = GetHeaderFromStorage(hash, marshalizer, storageService)
		if err != nil {
			return nil, err
		}
	}

	return hdr, nil
}

// GetHeaderFromPool gets the header, which is associated with the given hash, from pool
func GetHeaderFromPool(
	hash []byte,
	headersCacher retriever.HeadersPool,
) (*block.Block, error) {

	obj, err := getHeaderFromPool(hash, headersCacher)
	if err != nil {
		return nil, err
	}

	hdr, ok := obj.(*block.Block)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}

	return hdr, nil
}

func getHeaderFromPool(
	hash []byte,
	headersCacher retriever.HeadersPool,
) (interface{}, error) {

	if check.IfNil(headersCacher) {
		return nil, common.ErrNilCacher
	}

	obj, err := headersCacher.GetHeaderByHash(hash)
	if err != nil {
		return nil, fmt.Errorf("%w : getHeaderFromPool hash = %s",
			ErrMissingHeader, logger.DisplayByteSlice(hash))
	}

	return obj, nil
}

// GetHeaderFromStorage gets the header, which is associated with the given hash, from storage
func GetHeaderFromStorage(
	hash []byte,
	marshalizer marshal.Marshalizer,
	storageService retriever.StorageService,
) (*block.Block, error) {

	buffHdr, err := GetMarshalizedHeaderFromStorage(retriever.BlockUnit, hash, marshalizer, storageService)
	if err != nil {
		return nil, err
	}

	hdr := &block.Block{}
	err = marshalizer.Unmarshal(hdr, buffHdr)
	if err != nil {
		return nil, common.ErrUnmarshalWithoutSuccess
	}

	return hdr, nil
}

// GetHeaderFromStorageWithNonce method returns a meta block header from storage with a given nonce
func GetHeaderFromStorageWithNonce(
	nonce uint64,
	storageService retriever.StorageService,
	uint64Converter typeConverters.Uint64ByteSliceConverter,
	marshalizer marshal.Marshalizer,
) (*block.Block, []byte, error) {

	hash, err := getHeaderHashFromStorageWithNonce(
		nonce,
		storageService,
		uint64Converter,
		marshalizer,
		retriever.HdrNonceHashDataUnit)
	if err != nil {
		return nil, nil, err
	}

	hdr, err := GetHeaderFromStorage(hash, marshalizer, storageService)
	if err != nil {
		return nil, nil, err
	}

	return hdr, hash, nil
}

// GetMarshalizedHeaderFromStorage gets the marshalized header, which is associated with the given hash, from storage
func GetMarshalizedHeaderFromStorage(
	blockUnit retriever.UnitType,
	hash []byte,
	marshalizer marshal.Marshalizer,
	storageService retriever.StorageService,
) ([]byte, error) {

	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(storageService) {
		return nil, common.ErrNilStorage
	}

	hdrStore := storageService.GetStorer(blockUnit)
	if check.IfNil(hdrStore) {
		return nil, common.ErrNilHeadersStorage
	}

	buffHdr, err := hdrStore.Get(hash)
	if err != nil {
		return nil, fmt.Errorf("%w : GetMarshalizedHeaderFromStorage hash = %s",
			ErrMissingHeader, logger.DisplayByteSlice(hash))
	}

	return buffHdr, nil
}

func checkGetHeaderParamsForNil(
	cacher retriever.HeadersPool,
	marshalizer marshal.Marshalizer,
	storageService retriever.StorageService,
) error {

	if cacher == nil || cacher.IsInterfaceNil() {
		return common.ErrNilCacher
	}
	if marshalizer == nil || marshalizer.IsInterfaceNil() {
		return ErrNilMarshalizer
	}
	if storageService == nil || storageService.IsInterfaceNil() {
		return common.ErrNilStorage
	}

	return nil
}

// NewForkInfo creates a new ForkInfo object
func NewForkInfo() *ForkInfo {
	return &ForkInfo{IsDetected: false, Nonce: math.MaxUint64, Slot: math.MaxUint64, Hash: nil}
}

func getHeaderHashFromStorageWithNonce(
	nonce uint64,
	storageService retriever.StorageService,
	uint64Converter typeConverters.Uint64ByteSliceConverter,
	marshalizer marshal.Marshalizer,
	blockUnit retriever.UnitType,
) ([]byte, error) {

	if storageService == nil || storageService.IsInterfaceNil() {
		return nil, common.ErrNilStorage
	}
	if uint64Converter == nil || uint64Converter.IsInterfaceNil() {
		return nil, ErrNilUint64Converter
	}
	if marshalizer == nil || marshalizer.IsInterfaceNil() {
		return nil, ErrNilMarshalizer
	}

	headerStore := storageService.GetStorer(blockUnit)
	if headerStore == nil {
		return nil, common.ErrNilHeadersStorage
	}

	nonceToByteSlice := uint64Converter.ToByteSlice(nonce)
	hash, err := headerStore.Get(nonceToByteSlice)
	if err != nil {
		return nil, ErrMissingHashForHeaderNonce
	}

	return hash, nil
}

// IsInProperSlot checks if the given slot index satisfies the slot modulus trigger
func IsInProperSlot(index int64) bool {
	return index%SlotModulusTrigger == 0
}

// AddHeaderToBlackList adds a hash to black list handler. Logs if the operation did not succeed
func AddHeaderToBlackList(blackListHandler TimeCacher, hash []byte) {
	blackListHandler.Sweep()
	err := blackListHandler.Add(string(hash))
	if err != nil {
		log.Trace("blackListHandler.Add", "error", err.Error())
	}

	log.Debug("header has been added to blacklist",
		"hash", hash)
}

// GetTransactionHandlerFromPool gets the transaction from pool with a given txHash
func GetTransactionHandlerFromPool(
	txHash []byte,
	shardedDataCacherNotifier retriever.ShardedDataCacherNotifier,
) (data.TransactionHandler, error) {

	if shardedDataCacherNotifier == nil {
		return nil, ErrNilShardedDataCacherNotifier
	}

	var val interface{}
	ok := false

	txStore := shardedDataCacherNotifier.ShardDataStore("0")
	if txStore == nil {
		return nil, ErrNilStorage
	}

	val, ok = txStore.Peek(txHash)

	if !ok {
		return nil, ErrTxNotFound
	}

	tx, ok := val.(data.TransactionHandler)
	if !ok {
		return nil, ErrInvalidTxInPool
	}

	return tx, nil
}

// GetTransactionHandlerFromPool gets the transaction from pool with a given txHash
func CheckIfInTxPool(
	txHash []byte,
	shardedDataCacherNotifier retriever.ShardedDataCacherNotifier,
) error {
	txStore := shardedDataCacherNotifier.ShardDataStore("0")
	if txStore == nil {
		return ErrNilStorage
	}

	if txStore.Has(txHash) {
		return nil
	}

	return ErrTxNotFound
}

// SortVMOutputInsideData returns the output accounts as a sorted list
func SortVMOutputInsideData(vmOutput *vmcommon.VMOutput) []*vmcommon.OutputAccount {
	sort.Slice(vmOutput.DeletedAccounts, func(i, j int) bool {
		return bytes.Compare(vmOutput.DeletedAccounts[i], vmOutput.DeletedAccounts[j]) < 0
	})

	outPutAccounts := make([]*vmcommon.OutputAccount, len(vmOutput.OutputAccounts))
	i := 0
	for _, outAcc := range vmOutput.OutputAccounts {
		outPutAccounts[i] = outAcc
		i++
	}

	sort.Slice(outPutAccounts, func(i, j int) bool {
		return bytes.Compare(outPutAccounts[i].Address, outPutAccounts[j].Address) < 0
	})

	return outPutAccounts
}

// GetSortedStorageUpdates returns the storage updates as a sorted list
func GetSortedStorageUpdates(account *vmcommon.OutputAccount) []*vmcommon.StorageUpdate {
	storageUpdates := make([]*vmcommon.StorageUpdate, len(account.StorageUpdates))
	i := 0
	for _, update := range account.StorageUpdates {
		storageUpdates[i] = update
		i++
	}

	sort.Slice(storageUpdates, func(i, j int) bool {
		return bytes.Compare(storageUpdates[i].Offset, storageUpdates[j].Offset) < 0
	})

	return storageUpdates
}

// DisplayProcessTxDetails displays information related to the tx which should be executed
func DisplayProcessTxDetails(
	message string,
	accountHandler state.AccountHandler,
	scr *smartContractResult.SmartContractResult,
	txHash []byte,
	addressPubkeyConverter core.PubkeyConverter,
) {
	if !check.IfNil(accountHandler) {
		account, ok := accountHandler.(state.UserAccountHandler)
		if ok {
			log.Trace(message,
				"nonce", account.GetNonce(),
				"balance", account.GetBalance(nil, true),
			)
		}
	}

	if check.IfNil(addressPubkeyConverter) {
		return
	}
	if check.IfNil(scr) {
		return
	}

	receiver := ""
	if len(scr.GetRcvAddr()) == addressPubkeyConverter.Len() {
		receiver = addressPubkeyConverter.Encode(scr.GetRcvAddr())
	}

	sender := ""
	if len(scr.GetSndAddr()) == addressPubkeyConverter.Len() {
		sender = addressPubkeyConverter.Encode(scr.GetSndAddr())
	}

	log.Trace("executing transaction",
		"txHash", txHash,
		"nonce", scr.GetNonce(),
		"value", scr.GetValue(),
		"gas limit", scr.GetGasLimit(),
		"gas multiplier", scr.GetGasMultiplier(),
		"data", hex.EncodeToString(scr.GetSCData()),
		"sender", sender,
		"receiver", receiver)
}
