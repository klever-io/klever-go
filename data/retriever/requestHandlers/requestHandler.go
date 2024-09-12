package requestHandlers

import (
	"fmt"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/tools/check"
)

var _ eventNotifier.RequestHandler = (*resolverRequestHandler)(nil)

var log = logger.GetOrCreate("data/retriever/requesthandlers")

const minHashesToRequest = 10
const timeToAccumulateTrieHashes = 100 * time.Millisecond

//TODO move the keys definitions that are whitelisted in core and use them in InterceptedData implementations, Identifiers() function

type resolverRequestHandler struct {
	epoch                 uint32
	maxTxsToRequest       int
	resolversFinder       retriever.ResolversFinder
	requestedItemsHandler retriever.RequestedItemsHandler
	whiteList             retriever.WhiteListHandler
	sweepTime             time.Time
	requestInterval       time.Duration
	mutSweepTime          sync.Mutex

	trieHashesAccumulator map[string]struct{}
	lastTrieRequestTime   time.Time
	mutexTrieHashes       sync.Mutex
}

// NewResolverRequestHandler creates a requestHandler interface implementation with request functions
func NewResolverRequestHandler(
	finder retriever.ResolversFinder,
	requestedItemsHandler retriever.RequestedItemsHandler,
	whiteList retriever.WhiteListHandler,
	maxTxsToRequest int,
	requestInterval time.Duration,
) (*resolverRequestHandler, error) {

	if check.IfNil(finder) {
		return nil, common.ErrNilResolverFinder
	}
	if check.IfNil(requestedItemsHandler) {
		return nil, common.ErrNilRequestedItemsHandler
	}
	if maxTxsToRequest < 1 {
		return nil, common.ErrInvalidMaxTxRequest
	}
	if check.IfNil(whiteList) {
		return nil, common.ErrNilWhiteListHandler
	}
	if requestInterval < time.Millisecond {
		return nil, fmt.Errorf("%w:request interval is smaller than a millisecond", common.ErrRequestIntervalTooSmall)
	}

	rrh := &resolverRequestHandler{
		resolversFinder:       finder,
		requestedItemsHandler: requestedItemsHandler,
		epoch:                 uint32(0), // will be updated after creation of the request handler
		maxTxsToRequest:       maxTxsToRequest,
		whiteList:             whiteList,
		requestInterval:       requestInterval,
		trieHashesAccumulator: make(map[string]struct{}),
	}

	rrh.sweepTime = time.Now()

	return rrh, nil
}

// SetEpoch will update the current epoch so the request handler will make requests for this received epoch
func (rrh *resolverRequestHandler) SetEpoch(epoch uint32) {
	rrh.epoch = epoch
}

// RequestTransaction method asks for transactions from the connected peers
func (rrh *resolverRequestHandler) RequestTransaction(txHashes [][]byte) {
	rrh.requestByHashes(txHashes, common.TransactionTopic, "")
}

func (rrh *resolverRequestHandler) RequestTransactionTo(txHashes [][]byte, peer core.PeerID) {
	rrh.requestByHashes(txHashes, common.TransactionTopic, peer)
}

func (rrh *resolverRequestHandler) requestByHashes(hashes [][]byte, topic string, peer core.PeerID) {
	unrequestedHashes := rrh.getUnrequestedHashes(hashes)
	if len(unrequestedHashes) == 0 {
		return
	}
	log.Debug("requesting transactions from network",
		"topic", topic,
		"num txs", len(unrequestedHashes),
	)
	resolver, err := rrh.resolversFinder.ChainResolver(topic)
	if err != nil {
		log.Error("requestByHashes.ChainResolver",
			"error", err.Error(),
			"topic", topic,
		)
		return
	}

	txResolver, ok := resolver.(HashSliceResolver)
	if !ok {
		log.Warn("wrong assertion type when creating transaction resolver")
		return
	}

	for _, txHash := range hashes {
		log.Trace("requestByHashes", "hash", txHash)
	}

	rrh.whiteList.Add(unrequestedHashes)

	if peer != "" {
		go rrh.requestHashesWithDataSplitTo(unrequestedHashes, txResolver, peer)
	} else {
		go rrh.requestHashesWithDataSplit(unrequestedHashes, txResolver)
	}

	rrh.addRequestedItems(unrequestedHashes)
}

func (rrh *resolverRequestHandler) requestHashesWithDataSplit(
	unrequestedHashes [][]byte,
	resolver HashSliceResolver,
) {
	dataSplit := &partitioning.DataSplit{}
	sliceBatches, err := dataSplit.SplitDataInChunks(unrequestedHashes, rrh.maxTxsToRequest)
	if err != nil {
		log.Debug("requestByHashes.SplitDataInChunks",
			"error", err.Error(),
			"num txs", len(unrequestedHashes),
			"max txs to request", rrh.maxTxsToRequest,
		)
	}

	for _, batch := range sliceBatches {
		err = resolver.RequestDataFromHashArray(batch, rrh.epoch)
		if err != nil {
			log.Debug("requestByHashes.RequestDataFromHashArray",
				"error", err.Error(),
				"epoch", rrh.epoch,
				"batch size", len(batch),
			)
		}
	}
}

func (rrh *resolverRequestHandler) requestHashesWithDataSplitTo(
	unrequestedHashes [][]byte,
	resolver HashSliceResolver,
	peer core.PeerID,
) {
	dataSplit := &partitioning.DataSplit{}
	sliceBatches, err := dataSplit.SplitDataInChunks(unrequestedHashes, rrh.maxTxsToRequest)
	if err != nil {
		log.Debug("requestByHashesTo.SplitDataInChunks",
			"error", err.Error(),
			"num txs", len(unrequestedHashes),
			"max txs to request", rrh.maxTxsToRequest,
		)
	}

	for _, batch := range sliceBatches {
		err = resolver.RequestDataFromHashArrayTo(batch, rrh.epoch, peer)
		if err != nil {
			log.Debug("requestByHashesTo.RequestDataFromHashArray",
				"error", err.Error(),
				"epoch", rrh.epoch,
				"batch size", len(batch),
				"peer", peer.Pretty(),
			)
		}
	}
}

// RequestBlock method asks for block from the connected peers
func (rrh *resolverRequestHandler) RequestBlock(blockHash []byte) {
	if !rrh.testIfRequestIsNeeded(blockHash) {
		return
	}

	log.Debug("requesting block from network",
		"topic", common.BlocksTopic,
		"hash", blockHash,
	)

	resolver, err := rrh.resolversFinder.ChainResolver(common.BlocksTopic)
	if err != nil {
		log.Error("RequestBlock.ChainResolver",
			"error", err.Error(),
			"topic", common.BlocksTopic,
		)
		return
	}

	rrh.whiteList.Add([][]byte{blockHash})

	err = resolver.RequestDataFromHash(blockHash, rrh.epoch)
	if err != nil {
		log.Debug("RequestBlock.RequestDataFromHash",
			"error", err.Error(),
			"epoch", rrh.epoch,
			"hash", blockHash,
		)
		return
	}

	rrh.addRequestedItems([][]byte{blockHash})
}

// RequestHeader method asks for meta header from the connected peers
func (rrh *resolverRequestHandler) RequestHeader(hash []byte) {
	if !rrh.testIfRequestIsNeeded(hash) {
		return
	}

	log.Debug("requesting meta header from network",
		"hash", hash,
	)

	resolver, err := rrh.getHeaderResolver()
	if err != nil {
		log.Error("RequestHeader.getHeaderResolver",
			"error", err.Error(),
			"hash", hash,
		)
		return
	}

	rrh.whiteList.Add([][]byte{hash})

	err = resolver.RequestDataFromHash(hash, rrh.epoch)
	if err != nil {
		log.Debug("RequestHeader.RequestDataFromHash",
			"error", err.Error(),
			"epoch", rrh.epoch,
			"hash", hash,
		)
		return
	}

	rrh.addRequestedItems([][]byte{hash})
}

// RequestTrieNodes method asks for trie nodes from the connected peers
func (rrh *resolverRequestHandler) RequestTrieNodes(hashes [][]byte, topic string) {
	unrequestedHashes := rrh.getUnrequestedHashes(hashes)
	if len(unrequestedHashes) == 0 {
		return
	}

	rrh.mutexTrieHashes.Lock()
	defer rrh.mutexTrieHashes.Unlock()

	for _, hash := range unrequestedHashes {
		rrh.trieHashesAccumulator[string(hash)] = struct{}{}
	}

	index := 0
	itemsToRequest := make([][]byte, len(rrh.trieHashesAccumulator))
	for hash := range rrh.trieHashesAccumulator {
		itemsToRequest[index] = []byte(hash)
		index++
	}

	rrh.whiteList.Add(itemsToRequest)

	elapsedTime := time.Since(rrh.lastTrieRequestTime)
	if len(rrh.trieHashesAccumulator) < minHashesToRequest && elapsedTime < timeToAccumulateTrieHashes {
		return
	}

	log.Debug("requesting trie nodes from network",
		"topic", topic,
		"num nodes", len(rrh.trieHashesAccumulator),
		"firstHash", unrequestedHashes[0],
	)

	resolver, err := rrh.resolversFinder.ChainResolver(topic)
	if err != nil {
		log.Error("requestByHash.Resolver",
			"error", err.Error(),
			"topic", topic,
		)
		return
	}

	trieResolver, ok := resolver.(retriever.TrieNodesResolver)
	if !ok {
		log.Warn("wrong assertion type when creating a trie nodes resolver")
		return
	}

	for _, txHash := range rrh.trieHashesAccumulator {
		log.Trace("requestByHashes", "hash", txHash)
	}

	go rrh.requestHashesWithDataSplit(itemsToRequest, trieResolver)

	rrh.addRequestedItems(itemsToRequest)
	rrh.lastTrieRequestTime = time.Now()
	rrh.trieHashesAccumulator = make(map[string]struct{})
}

// ResetRequests clean all requests
func (rrh *resolverRequestHandler) ResetRequests() {
	rrh.requestedItemsHandler.ResetAll()
}

// RequestHeaderByNonce method asks for meta header from the connected peers by nonce
func (rrh *resolverRequestHandler) RequestHeaderByNonce(nonce uint64) {
	key := []byte(fmt.Sprintf("nonce-%d", nonce))
	if !rrh.testIfRequestIsNeeded(key) {
		return
	}

	log.Debug("requesting meta header by nonce from network",
		"nonce", nonce,
		"epoch", rrh.epoch,
	)

	headerResolver, err := rrh.getHeaderResolver()
	if err != nil {
		log.Error("RequestHeaderByNonce.getHeaderResolver",
			"error", err.Error(),
		)
		return
	}

	rrh.whiteList.Add([][]byte{key})

	err = headerResolver.RequestDataFromNonce(nonce, rrh.epoch)
	if err != nil {
		log.Debug("RequestHeaderByNonce.RequestDataFromNonce",
			"error", err.Error(),
			"epoch", rrh.epoch,
			"nonce", nonce,
		)
		return
	}

	rrh.addRequestedItems([][]byte{key})
}

func (rrh *resolverRequestHandler) testIfRequestIsNeeded(key []byte) bool {
	rrh.sweepIfNeeded()

	if rrh.requestedItemsHandler.Has(string(key)) {
		log.Trace("item already requested",
			"key", key)
		return false
	}

	return true
}

func (rrh *resolverRequestHandler) addRequestedItems(keys [][]byte) {
	for _, key := range keys {
		err := rrh.requestedItemsHandler.Add(string(key))
		if err != nil {
			log.Trace("addRequestedItems",
				"error", err.Error(),
				"key", key)
			continue
		}
	}
}

func (rrh *resolverRequestHandler) getHeaderResolver() (retriever.HeaderResolver, error) {
	resolver, err := rrh.resolversFinder.ChainResolver(common.BlocksTopic)
	if err != nil {
		err = fmt.Errorf("%w, topic: %s",
			err, common.BlocksTopic)
		return nil, err
	}

	headerResolver, ok := resolver.(retriever.HeaderResolver)
	if !ok {
		err = fmt.Errorf("%w, topic: %s, expected HeaderResolver",
			common.ErrWrongTypeInContainer, common.BlocksTopic)
		return nil, err
	}

	return headerResolver, nil
}

// RequestStartOfEpochBlock method asks for the start of epoch metablock from the connected peers
func (rrh *resolverRequestHandler) RequestStartOfEpochBlock(epoch uint32) {
	epochStartIdentifier := []byte(core.EpochStartIdentifier(epoch))
	if !rrh.testIfRequestIsNeeded(epochStartIdentifier) {
		return
	}

	baseTopic := common.BlocksTopic
	log.Debug("requesting header by epoch",
		"topic", baseTopic,
		"epoch", epoch,
		"hash", epochStartIdentifier,
	)

	resolver, err := rrh.resolversFinder.ChainResolver(baseTopic)
	if err != nil {
		log.Error("RequestStartOfEpochBlock.ChainResolver",
			"error", err.Error(),
			"topic", baseTopic,
		)
		return
	}

	headerResolver, ok := resolver.(retriever.HeaderResolver)
	if !ok {
		log.Warn("wrong assertion type when creating header resolver")
		return
	}

	rrh.whiteList.Add([][]byte{epochStartIdentifier})

	err = headerResolver.RequestDataFromEpoch(epochStartIdentifier)
	if err != nil {
		log.Debug("RequestStartOfEpochBlock.RequestDataFromEpoch",
			"error", err.Error(),
			"epochStartIdentifier", epochStartIdentifier,
		)
		return
	}

	rrh.addRequestedItems([][]byte{epochStartIdentifier})
}

// RequestInterval returns the request interval between sending the same request
func (rrh *resolverRequestHandler) RequestInterval() time.Duration {
	return rrh.requestInterval
}

// IsInterfaceNil returns true if there is no value under the interface
func (rrh *resolverRequestHandler) IsInterfaceNil() bool {
	return rrh == nil
}

func (rrh *resolverRequestHandler) getUnrequestedHashes(hashes [][]byte) [][]byte {
	unrequestedHashes := make([][]byte, 0)

	rrh.sweepIfNeeded()

	for _, hash := range hashes {
		if !rrh.requestedItemsHandler.Has(string(hash)) {
			unrequestedHashes = append(unrequestedHashes, hash)
		}
	}

	return unrequestedHashes
}

func (rrh *resolverRequestHandler) sweepIfNeeded() {
	rrh.mutSweepTime.Lock()
	defer rrh.mutSweepTime.Unlock()

	if time.Since(rrh.sweepTime) <= rrh.requestInterval {
		return
	}

	rrh.sweepTime = time.Now()
	rrh.requestedItemsHandler.Sweep()
}

// SetNumPeersToQuery will set the number of intra shard and cross shard number of peers to query
// for a given resolver
func (rrh *resolverRequestHandler) SetNumPeersToQuery(key string, intra int, cross int) error {
	resolver, err := rrh.resolversFinder.Get(key)
	if err != nil {
		return err
	}

	resolver.SetNumPeersToQuery(intra, cross)
	return nil
}

// GetNumPeersToQuery will return the number of intra shard and cross shard number of peers to query
// for a given resolver
func (rrh *resolverRequestHandler) GetNumPeersToQuery(key string) (int, int, error) {
	resolver, err := rrh.resolversFinder.Get(key)
	if err != nil {
		return 0, 0, err
	}

	intra, cross := resolver.NumPeersToQuery()
	return intra, cross, nil
}
