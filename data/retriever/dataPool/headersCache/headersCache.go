package headersCache

import (
	"bytes"
	"time"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

type headersCache struct {
	headersNonceCache listOfHeadersByNonces

	headersByHash  headersByHashMap
	headersCounter int64

	numHeadersToRemove int
	maxHeadersPerShard int
}

func newHeadersCache(numMaxHeaderPerShard int, numHeadersToRemove int) *headersCache {
	return &headersCache{
		headersNonceCache:  make(listOfHeadersByNonces, 0),
		headersCounter:     0,
		headersByHash:      make(headersByHashMap),
		numHeadersToRemove: numHeadersToRemove,
		maxHeadersPerShard: numMaxHeaderPerShard,
	}
}

func (cache *headersCache) addHeader(headerHash []byte, header data.HeaderHandler) bool {
	if check.IfNil(header) || len(headerHash) == 0 {
		return false
	}

	headerNonce := header.GetNonce()

	cache.tryToDoEviction()

	hdrInfo := headerInfo{headerNonce: headerNonce}
	added := cache.headersByHash.addElement(headerHash, hdrInfo)
	if added {
		return false
	}

	shard := cache.getShardMap()
	shard.appendHeaderToList(headerHash, header)

	cache.headersCounter++

	return true
}

// tryToDoEviction will check if pool is full and if it is will do eviction
func (cache *headersCache) tryToDoEviction() {
	numHeaders := cache.getNumHeaders()
	if int(numHeaders) >= cache.maxHeadersPerShard {
		cache.lruEviction()
	}
}

func (cache *headersCache) lruEviction() {
	if cache.headersNonceCache == nil {
		return
	}

	nonces := cache.headersNonceCache.getNoncesSortedByTimestamp()

	numHashes := 0
	maxItemsToRemove := tools.MinInt(cache.numHeadersToRemove, len(nonces))
	for i := 0; i < maxItemsToRemove; i++ {
		numHashes += cache.removeHeaderByNonce(nonces[i])

		if numHashes >= maxItemsToRemove {
			break
		}
	}
}

func (cache *headersCache) getShardMap() listOfHeadersByNonces {
	if cache.headersNonceCache == nil {
		cache.headersNonceCache = make(listOfHeadersByNonces)
	}

	return cache.headersNonceCache
}

func (cache *headersCache) getNumHeaders() int64 {
	return cache.headersCounter
}

func (cache *headersCache) removeHeaderByNonce(headerNonce uint64) int {
	headers, ok := cache.headersNonceCache.getHeadersByNonce(headerNonce)
	if !ok {
		return 0
	}
	headersHashes := headers.getHashes()

	for _, hash := range headersHashes {
		log.Trace("removeHeaderByNonce",
			"nonce", headerNonce,
			"hash", hash,
		)
	}

	//remove items from nonce map
	cache.headersNonceCache.removeListOfHeaders(headerNonce)
	//remove elements from hashes map
	cache.headersByHash.deleteBulk(headersHashes)

	cache.headersCounter -= int64(len(headersHashes))

	return len(headersHashes)
}

func (cache *headersCache) removeHeaderByHash(hash []byte) {
	if len(hash) == 0 {
		return
	}

	info, ok := cache.headersByHash.getElement(hash)
	if !ok {
		return
	}

	log.Trace("removeHeaderByHash",
		"nonce", info.headerNonce,
		"hash", hash,
	)

	cache.removeHeaderFromNonceMap(info, hash)
	cache.headersByHash.deleteElement(hash)
}

// removeHeaderFromNonceMap will remove a header from headerWithTimestamp
// when a header is removed by hash we need to remove also header from the map where is stored with nonce
func (cache *headersCache) removeHeaderFromNonceMap(headerInfo headerInfo, headerHash []byte) {
	if cache.headersNonceCache == nil {
		return
	}

	headers, ok := cache.headersNonceCache.getHeadersByNonce(headerInfo.headerNonce)
	if !ok {
		return
	}

	for index, header := range headers.items {
		if !bytes.Equal(header.headerHash, headerHash) {
			continue
		}

		headers.removeHeader(index)
		cache.headersCounter--

		if headers.isEmpty() {
			cache.headersNonceCache.removeListOfHeaders(headerInfo.headerNonce)
			return
		}

		cache.headersNonceCache.setListOfHeaders(headerInfo.headerNonce, headers)
		return
	}
}

func (cache *headersCache) getHeaderByHash(hash []byte) (data.HeaderHandler, error) {
	info, ok := cache.headersByHash.getElement(hash)
	if !ok {
		return nil, ErrHeaderNotFound
	}

	if cache.headersNonceCache == nil {
		return nil, ErrHeaderNotFound
	}

	headers := cache.headersNonceCache.getListOfHeaders(info.headerNonce)
	if headers.isEmpty() {
		return nil, ErrHeaderNotFound
	}

	headers.timestamp = time.Now()
	cache.headersNonceCache.setListOfHeaders(info.headerNonce, headers)

	if header, hashExists := headers.findHeaderByHash(hash); hashExists {
		return header, nil
	}

	return nil, ErrHeaderNotFound
}

func (cache *headersCache) getHeadersByNonce(headerNonce uint64) ([]headerDetails, bool) {

	if cache.headersNonceCache == nil {
		return nil, false
	}

	headersList, ok := cache.headersNonceCache.getHeadersByNonce(headerNonce)
	if !ok {
		return nil, false
	}

	return headersList.items, true
}

func (cache *headersCache) getHeadersAndHashesByNonce(nonce uint64) ([]data.HeaderHandler, [][]byte, bool) {
	headersList, ok := cache.getHeadersByNonce(nonce)
	if !ok || len(headersList) == 0 {
		return nil, nil, false
	}

	headers := make([]data.HeaderHandler, 0, len(headersList))
	hashes := make([][]byte, 0, len(headersList))
	for _, hdrDetails := range headersList {
		headers = append(headers, hdrDetails.header)
		hashes = append(hashes, hdrDetails.headerHash)
	}

	return headers, hashes, true
}

func (cache *headersCache) keys() []uint64 {
	shardMap := cache.getShardMap()

	return shardMap.keys()
}

func (cache *headersCache) totalHeaders() int {
	return int(cache.headersCounter)
}

func (cache *headersCache) clear() {
	cache.headersNonceCache = listOfHeadersByNonces{}
	cache.headersCounter = 0
	cache.headersByHash = make(headersByHashMap)
}
