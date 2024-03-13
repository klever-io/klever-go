package sync

import (
	"bytes"
	"math"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
	"github.com/klever-io/klever-go/update"
)

var _ update.HeaderSyncHandler = (*headersToSync)(nil)

const waitTimeForHeaders = time.Minute

type headersToSync struct {
	mutMeta                sync.Mutex
	epochStartMetaBlock    *block.Block
	unFinishedMetaBlocks   map[string]*block.Block
	firstPendingMetaBlocks map[string]*block.Block
	missingMetaBlocks      map[string]struct{}
	missingMetaNonces      map[uint64]struct{}
	foundMetaNonces        map[uint64]string
	chReceivedAll          chan bool
	store                  retriever.StorageService
	metaBlockPool          retriever.HeadersPool
	epochHandler           update.EpochStartVerifier
	marshalizer            marshal.Marshalizer
	hasher                 hashing.Hasher
	stopSyncing            bool
	epochToSync            uint32
	requestHandler         process.RequestHandler
	uint64Converter        typeConverters.Uint64ByteSliceConverter
}

// ArgsNewHeadersSyncHandler defines the arguments needed for the new header syncer
type ArgsNewHeadersSyncHandler struct {
	StorageService  retriever.StorageService
	Cache           retriever.HeadersPool
	Marshalizer     marshal.Marshalizer
	Hasher          hashing.Hasher
	EpochHandler    update.EpochStartVerifier
	RequestHandler  process.RequestHandler
	Uint64Converter typeConverters.Uint64ByteSliceConverter
}

// NewHeadersSyncHandler creates a new header syncer
func NewHeadersSyncHandler(args ArgsNewHeadersSyncHandler) (*headersToSync, error) {
	if check.IfNil(args.StorageService) {
		return nil, common.ErrNilStorage
	}
	if check.IfNil(args.Cache) {
		return nil, common.ErrNilCacher
	}
	if check.IfNil(args.EpochHandler) {
		return nil, common.ErrNilEpochHandler
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.RequestHandler) {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(args.Uint64Converter) {
		return nil, common.ErrNilUint64Converter
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}

	h := &headersToSync{
		mutMeta:                sync.Mutex{},
		epochStartMetaBlock:    &block.Block{},
		chReceivedAll:          make(chan bool),
		store:                  args.StorageService,
		metaBlockPool:          args.Cache,
		epochHandler:           args.EpochHandler,
		stopSyncing:            true,
		requestHandler:         args.RequestHandler,
		marshalizer:            args.Marshalizer,
		hasher:                 args.Hasher,
		unFinishedMetaBlocks:   make(map[string]*block.Block),
		firstPendingMetaBlocks: make(map[string]*block.Block),
		missingMetaBlocks:      make(map[string]struct{}),
		missingMetaNonces:      make(map[uint64]struct{}),
		uint64Converter:        args.Uint64Converter,
	}

	h.metaBlockPool.RegisterHandler(h.receivedMetaBlockFirstPending)
	h.metaBlockPool.RegisterHandler(h.receivedUnFinishedMetaBlocks)

	return h, nil
}

func (h *headersToSync) receivedMetaBlockFirstPending(headerHandler data.HeaderHandler, hash []byte) {
	h.mutMeta.Lock()
	if h.stopSyncing || len(h.missingMetaBlocks) == 0 {
		h.mutMeta.Unlock()
		return
	}

	metaHeader, ok := headerHandler.(*block.Block)
	if !ok {
		h.mutMeta.Unlock()
		return
	}

	if _, ok = h.missingMetaBlocks[string(hash)]; !ok {
		h.mutMeta.Unlock()
		return
	}

	delete(h.missingMetaBlocks, string(hash))
	h.firstPendingMetaBlocks[string(hash)] = metaHeader

	if len(h.missingMetaBlocks) > 0 {
		h.mutMeta.Unlock()
		return
	}

	h.mutMeta.Unlock()
	h.chReceivedAll <- true
}

func (h *headersToSync) receivedUnFinishedMetaBlocks(headerHandler data.HeaderHandler, hash []byte) {
	h.mutMeta.Lock()
	if h.stopSyncing || len(h.missingMetaNonces) == 0 {
		h.mutMeta.Unlock()
		return
	}

	meta, ok := headerHandler.(*block.Block)
	if !ok {
		h.mutMeta.Unlock()
		return
	}

	if _, ok = h.missingMetaNonces[meta.GetNonce()]; !ok {
		h.mutMeta.Unlock()
		return
	}

	attestingHash, okHash := h.foundMetaNonces[meta.GetNonce()+1]
	attestingHdr, okHdr := h.unFinishedMetaBlocks[attestingHash]

	isTheNeededMeta := okHash && okHdr && bytes.Equal(attestingHdr.GetParentHash(), hash)
	if !isTheNeededMeta {
		h.requestHandler.RequestHeaderByNonce(meta.GetNonce())
		h.requestHandler.RequestHeaderByNonce(meta.GetNonce() + 1)
		h.mutMeta.Unlock()
		return
	}

	delete(h.missingMetaNonces, meta.GetNonce())
	h.unFinishedMetaBlocks[string(hash)] = meta
	h.foundMetaNonces[meta.GetNonce()] = string(hash)

	if len(h.missingMetaNonces) > 0 {
		h.mutMeta.Unlock()
		return
	}

	h.mutMeta.Unlock()
	h.chReceivedAll <- true
}

// SyncUnFinishedMetaHeaders syncs and validates all the unFinished metaHeaders
func (h *headersToSync) SyncUnFinishedMetaHeaders(epoch uint32) error {
	// TODO: do this with context.Context
	err := h.syncEpochStartMetaHeader(epoch, waitTimeForHeaders)
	if err != nil {
		return err
	}

	err = h.syncAllNeededMetaHeaders(waitTimeForHeaders)
	if err != nil {
		return err
	}

	return nil
}

// SyncEpochStartMetaHeader syncs and validates an epoch start metaHeader
func (h *headersToSync) syncEpochStartMetaHeader(epoch uint32, waitTime time.Duration) error {
	defer func() {
		h.mutMeta.Lock()
		h.stopSyncing = true
		h.mutMeta.Unlock()
	}()

	h.epochToSync = epoch
	epochStartId := core.EpochStartIdentifier(epoch)
	meta, err := process.GetHeaderFromStorage([]byte(epochStartId), h.marshalizer, h.store)
	if err != nil {
		h.mutMeta.Lock()
		h.stopSyncing = false
		h.requestHandler.RequestStartOfEpochBlock(epoch)
		h.mutMeta.Unlock()

		startTime := time.Now()
		for {
			time.Sleep(time.Millisecond)
			elapsedTime := time.Since(startTime)
			if elapsedTime > waitTime {
				return process.ErrTimeIsOut
			}

			if !h.epochHandler.IsEpochStart() {
				continue
			}

			meta, err = process.GetHeaderFromStorage([]byte(epochStartId), h.marshalizer, h.store)
			if err != nil {
				continue
			}

			h.mutMeta.Lock()
			h.epochStartMetaBlock = meta
			h.mutMeta.Unlock()

			break
		}

		err = WaitFor(h.chReceivedAll, waitTime)
		if err != nil {
			log.Warn("timeOut for requesting epoch metaHdr")
			return err
		}

		return nil
	}

	h.mutMeta.Lock()
	h.epochStartMetaBlock = meta
	h.mutMeta.Unlock()

	return nil
}

func (h *headersToSync) syncAllNeededMetaHeaders(waitTime time.Duration) error {
	defer func() {
		h.mutMeta.Lock()
		h.stopSyncing = true
		h.mutMeta.Unlock()
	}()

	h.mutMeta.Lock()

	err := h.computeMissingNonce(h.epochStartMetaBlock)
	if err != nil {
		h.mutMeta.Unlock()
		return err
	}

	_ = tools.EmptyChannel(h.chReceivedAll)
	for nonce := range h.missingMetaNonces {
		h.stopSyncing = false
		h.requestHandler.RequestHeaderByNonce(nonce)
	}

	requested := len(h.missingMetaNonces) > 0
	h.mutMeta.Unlock()

	if requested {
		errWaitFor := WaitFor(h.chReceivedAll, waitTime)
		if errWaitFor != nil {
			log.Warn("timeOut for requesting all unFinished metaBlocks")
			return errWaitFor
		}
	}

	return nil
}

func (h *headersToSync) computeMissingNonce(epochStart *block.Block) error {
	h.missingMetaNonces = make(map[uint64]struct{})
	h.foundMetaNonces = make(map[uint64]string)

	epochStartNonce := epochStart.GetNonce()
	epochStartHash, err := tools.CalculateHash(h.marshalizer, h.hasher, epochStart.Header)
	if err != nil {
		return err
	}

	h.foundMetaNonces[epochStartNonce] = string(epochStartHash)
	h.unFinishedMetaBlocks[string(epochStartHash)] = epochStart

	for hash, meta := range h.firstPendingMetaBlocks {
		h.unFinishedMetaBlocks[hash] = meta
		h.foundMetaNonces[meta.GetNonce()] = hash
	}

	if len(h.firstPendingMetaBlocks) == 0 {
		return nil
	}

	lowestPendingNonce := h.lowestPendingNonceFrom(h.firstPendingMetaBlocks)
	for nonce := epochStartNonce - 1; nonce >= lowestPendingNonce+1; nonce-- {
		_, ok := h.foundMetaNonces[nonce]
		if ok {
			continue
		}

		attestingHash, ok := h.foundMetaNonces[nonce+1]
		if !ok {
			h.missingMetaNonces[nonce] = struct{}{}
			continue
		}
		attestingMeta, ok := h.unFinishedMetaBlocks[attestingHash]
		if !ok {
			h.missingMetaNonces[nonce] = struct{}{}
			continue
		}
		metaHdr, errGetMetaHeader := process.GetHeader(attestingMeta.GetParentHash(), h.metaBlockPool, h.marshalizer, h.store)
		if errGetMetaHeader != nil {
			h.missingMetaNonces[nonce] = struct{}{}
			continue
		}

		h.foundMetaNonces[nonce] = string(attestingMeta.GetParentHash())
		h.unFinishedMetaBlocks[string(attestingMeta.GetParentHash())] = metaHdr
	}

	return nil
}

func (h *headersToSync) lowestPendingNonceFrom(metaBlocks map[string]*block.Block) uint64 {
	lowestNonce := uint64(math.MaxUint64)
	for _, metaBlock := range metaBlocks {
		if lowestNonce > metaBlock.GetNonce() {
			lowestNonce = metaBlock.GetNonce()
		}
	}
	return lowestNonce
}

// GetEpochStartMetaBlock returns the synced epoch start metaBlock
func (h *headersToSync) GetEpochStartMetaBlock() (*block.Block, error) {
	h.mutMeta.Lock()
	meta := h.epochStartMetaBlock
	h.mutMeta.Unlock()

	if meta.GetIsEpochStart() || meta.GetNonce() == 0 {
		return meta, nil
	}

	return nil, update.ErrNotSynced
}

// GetUnFinishedMetaBlocks returns the synced metablock
func (h *headersToSync) GetUnFinishedMetaBlocks() (map[string]*block.Block, error) {
	h.mutMeta.Lock()
	unFinished := make(map[string]*block.Block)
	for hash, meta := range h.unFinishedMetaBlocks {
		unFinished[hash] = meta
	}
	h.mutMeta.Unlock()

	return unFinished, nil
}

// IsInterfaceNil returns true if underlying object is nil
func (h *headersToSync) IsInterfaceNil() bool {
	return h == nil
}
