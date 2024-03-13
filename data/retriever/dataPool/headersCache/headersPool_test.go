package headersCache_test

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever/dataPool/headersCache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHeadersCacher_AddHeadersInCache(t *testing.T) {
	t.Parallel()

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            1000,
			NumElementsToRemoveOnEviction: 100},
	)

	nonce := uint64(1)
	headerHash1 := []byte("hash1")
	headerHash2 := []byte("hash2")
	testHdr1 := &block.Block{Header: &block.BlockHeader{Nonce: nonce}}
	testHdr2 := &block.Block{Header: &block.BlockHeader{Nonce: nonce, Slot: 100}}

	headersCacher.AddHeader(headerHash1, testHdr1)
	headersCacher.AddHeader(headerHash2, testHdr2)

	header, err := headersCacher.GetHeaderByHash(headerHash1)
	require.Nil(t, err)
	require.Equal(t, testHdr1, header)

	header, err = headersCacher.GetHeaderByHash(headerHash2)
	require.Nil(t, err)
	require.Equal(t, testHdr2, header)

	expectedHeaders := []data.HeaderHandler{testHdr1, testHdr2}
	headers, _, err := headersCacher.GetHeadersByNonce(nonce)
	require.Nil(t, err)
	require.Equal(t, expectedHeaders, headers)
}

func Test_RemoveHeaderByHash(t *testing.T) {
	t.Parallel()

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            1000,
			NumElementsToRemoveOnEviction: 100},
	)

	nonce := uint64(1)
	headerHash1 := []byte("hash1")
	headerHash2 := []byte("hash2")
	testHdr1 := &block.Block{Header: &block.BlockHeader{Nonce: nonce}}
	testHdr2 := &block.Block{Header: &block.BlockHeader{Nonce: nonce, Slot: 100}}

	headersCacher.AddHeader(headerHash1, testHdr1)
	headersCacher.AddHeader(headerHash2, testHdr2)

	headersCacher.RemoveHeaderByHash(headerHash1)
	header, err := headersCacher.GetHeaderByHash(headerHash1)
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)

	headersCacher.RemoveHeaderByHash(headerHash2)
	header, err = headersCacher.GetHeaderByHash(headerHash2)
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)
}

func TestHeadersCacher_AddHeadersInCacheAndRemoveByNonce(t *testing.T) {
	t.Parallel()

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            1000,
			NumElementsToRemoveOnEviction: 100},
	)

	nonce := uint64(1)
	headerHash1 := []byte("hash1")
	headerHash2 := []byte("hash2")
	testHdr1 := &block.Block{Header: &block.BlockHeader{Nonce: nonce}}
	testHdr2 := &block.Block{Header: &block.BlockHeader{Nonce: nonce, Slot: 100}}

	headersCacher.AddHeader(headerHash1, testHdr1)
	headersCacher.AddHeader(headerHash2, testHdr2)

	headersCacher.RemoveHeaderByNonce(nonce)
	header, err := headersCacher.GetHeaderByHash(headerHash1)
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)

	header, err = headersCacher.GetHeaderByHash(headerHash2)
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)
}

func TestHeadersCacher_Eviction(t *testing.T) {
	t.Parallel()

	numHeadersToGenerate := 1001
	headers, headersHashes := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            900,
			NumElementsToRemoveOnEviction: 100},
	)

	for i := 0; i < numHeadersToGenerate; i++ {
		headersCacher.AddHeader(headersHashes[i], &headers[i])
	}

	// Cacher will do eviction 2 times, in items cache will be 801 items
	require.Equal(t, 801, headersCacher.GetNumHeaders())

	for i := 200; i < numHeadersToGenerate; i++ {
		header, err := headersCacher.GetHeaderByHash(headersHashes[i])
		require.Nil(t, err)
		require.Equal(t, &headers[i], header)
	}
}

func TestHeadersCacher_ConcurrentRequests_NoEviction(t *testing.T) {
	t.Parallel()

	numHeadersToGenerate := 50

	headers, headersHashes := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            numHeadersToGenerate + 1,
			NumElementsToRemoveOnEviction: 10},
	)

	var waitgroup sync.WaitGroup
	for i := 0; i < numHeadersToGenerate; i++ {
		waitgroup.Add(1)
		go func(index int) {
			headersCacher.AddHeader(headersHashes[index], &headers[index])
			header, err := headersCacher.GetHeaderByHash(headersHashes[index])

			assert.Nil(t, err)
			assert.Equal(t, &headers[index], header)
			waitgroup.Done()
		}(i)
	}
	waitgroup.Wait()
}

func TestHeadersCacher_ConcurrentRequests_WithEviction(t *testing.T) {
	cacheSize := 2
	numHeadersToGenerate := 50

	headers, headersHashes := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 1},
	)

	var waitgroup sync.WaitGroup
	for i := 0; i < numHeadersToGenerate; i++ {
		waitgroup.Add(1)
		go func(index int) {
			headersCacher.AddHeader(headersHashes[index], &headers[index])
			waitgroup.Done()
		}(i)
	}
	waitgroup.Wait()
	// cache size after all eviction is finish should be 2
	require.Equal(t, 2, headersCacher.GetNumHeaders())

	numHeadersToGenerate = 3
	headers, headersHashes = createASliceOfHeaders(3)
	for i := 0; i < numHeadersToGenerate; i++ {
		headersCacher.AddHeader(headersHashes[i], &headers[i])
		time.Sleep(time.Microsecond)
	}

	require.Equal(t, 2, headersCacher.GetNumHeaders())
	header1, err := headersCacher.GetHeaderByHash(headersHashes[1])
	require.Nil(t, err)
	require.Equal(t, &headers[1], header1)

	header2, err := headersCacher.GetHeaderByHash(headersHashes[2])
	require.Nil(t, err)
	require.Equal(t, &headers[2], header2)
}

func TestHeadersCacher_AddHeadersWithSameNonceShouldBeRemovedAtEviction(t *testing.T) {
	t.Parallel()

	cacheSize := 2

	hash1, hash2, hash3 := []byte("hash1"), []byte("hash2"), []byte("hash3")
	header1, header2, header3 := &block.Block{Header: &block.BlockHeader{Nonce: 0}}, &block.Block{Header: &block.BlockHeader{Nonce: 0}}, &block.Block{Header: &block.BlockHeader{Nonce: 1}}

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 1},
	)
	headersCacher.AddHeader(hash1, header1)
	headersCacher.AddHeader(hash2, header2)
	headersCacher.AddHeader(hash3, header3)

	require.Equal(t, 1, headersCacher.GetNumHeaders())

	header, err := headersCacher.GetHeaderByHash(hash3)
	require.Nil(t, err)
	require.Equal(t, header3, header)
}

func TestHeadersCacher_AddALotOfHeadersAndCheckEviction(t *testing.T) {
	t.Parallel()

	cacheSize := 100
	numHeaders := 200
	headers, headersHash := createASliceOfHeaders(numHeaders)

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 50},
	)

	var waitgroup sync.WaitGroup
	for i := 0; i < numHeaders; i++ {
		waitgroup.Add(1)
		go func(index int) {
			headersCacher.AddHeader(headersHash[index], &headers[index])
			waitgroup.Done()
		}(i)
	}

	waitgroup.Wait()
	assert.Equal(t, 100, headersCacher.GetNumHeaders())
}

func TestHeadersCacher_BigCacheALotOfHeaders(t *testing.T) {
	t.Parallel()

	cacheSize := 100000
	numHeadersToGenerate := cacheSize

	headers, headersHash := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 50},
	)

	start := time.Now()
	for i := 0; i < numHeadersToGenerate; i++ {
		headersCacher.AddHeader(headersHash[i], &headers[i])
	}
	elapsed := time.Since(start)
	fmt.Printf("insert %d took %s \n", numHeadersToGenerate, elapsed)

	start = time.Now()
	header, _ := headersCacher.GetHeaderByHash(headersHash[100])
	elapsed = time.Since(start)
	require.Equal(t, &headers[100], header)
	fmt.Printf("get header by hash took %s \n", elapsed)

	start = time.Now()
	d, _, _ := headersCacher.GetHeadersByNonce(uint64(100))
	elapsed = time.Since(start)
	fmt.Printf("get header by shard id and nonce took %s \n", elapsed)
	require.Equal(t, &headers[100], d[0])

	start = time.Now()
	headersCacher.RemoveHeaderByNonce(uint64(500))
	elapsed = time.Since(start)
	fmt.Printf("remove header by shard id and nonce took %s \n", elapsed)

	header, err := headersCacher.GetHeaderByHash(headersHash[500])
	require.Nil(t, header)
	require.Error(t, headersCache.ErrHeaderNotFound, err)

	start = time.Now()
	headersCacher.RemoveHeaderByHash(headersHash[2012])
	elapsed = time.Since(start)
	fmt.Printf("remove header by hash took %s \n", elapsed)

	header, err = headersCacher.GetHeaderByHash(headersHash[2012])
	require.Nil(t, header)
	require.Error(t, headersCache.ErrHeaderNotFound, err)
}

func TestHeadersCacher_AddHeadersWithDifferentOnMultipleGoroutines(t *testing.T) {
	t.Parallel()

	cacheSize := 51
	numHdrsToGenerate := 50

	headersShard0, hashesShad0 := createASliceOfHeadersNonce0(numHdrsToGenerate)
	headersShard1, hashesShad1 := createASliceOfHeaders(numHdrsToGenerate)
	headersShard2, hashesShad2 := createASliceOfHeaders(numHdrsToGenerate)
	numElemsToRemove := 25
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: numElemsToRemove},
	)

	var waitgroup sync.WaitGroup
	start := time.Now()
	for i := 0; i < numHdrsToGenerate; i++ {
		waitgroup.Add(5)
		go func(index int) {
			headersCacher.AddHeader(hashesShad0[index], &headersShard0[index])
			waitgroup.Done()
		}(i)

		go func(index int) {
			headersCacher.AddHeader(hashesShad1[index], &headersShard1[index])
			go func(index int) {
				headersCacher.RemoveHeaderByHash(hashesShad1[index])
				waitgroup.Done()
			}(index)
			waitgroup.Done()
		}(i)

		go func(index int) {
			headersCacher.AddHeader(hashesShad2[index], &headersShard2[index])
			go func(index int) {
				headersCacher.RemoveHeaderByHash(hashesShad2[index])
				waitgroup.Done()
			}(index)
			waitgroup.Done()
		}(i)
	}

	waitgroup.Wait()

	for i := 0; i < numHdrsToGenerate; i++ {
		waitgroup.Add(1)
		go func(index int) {
			headersCacher.RemoveHeaderByHash(hashesShad0[index])
			waitgroup.Done()
		}(i)
	}
	waitgroup.Wait()

	elapsed := time.Since(start)
	fmt.Printf("time need to add %d in cache %s \n", numHdrsToGenerate, elapsed)

	require.Equal(t, 0, headersCacher.GetNumHeaders())
	require.Equal(t, 0, headersCacher.GetNumHeaders())
	require.Equal(t, 0, headersCacher.GetNumHeaders())
}

func TestHeadersCacher_TestEvictionRemoveCorrectHeader(t *testing.T) {
	t.Parallel()

	cacheSize := 2
	numHeadersToGenerate := 3

	headers, headersHashes := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 1},
	)

	for i := 0; i < numHeadersToGenerate-1; i++ {
		headersCacher.AddHeader(headersHashes[i], &headers[i])
		time.Sleep(time.Microsecond)
	}

	header, err := headersCacher.GetHeaderByHash(headersHashes[0])
	require.Nil(t, err)
	require.Equal(t, &headers[0], header)

	headersCacher.AddHeader(headersHashes[2], &headers[2])

	header, err = headersCacher.GetHeaderByHash(headersHashes[0])
	require.Nil(t, err)
	require.Equal(t, &headers[0], header)

	header, err = headersCacher.GetHeaderByHash(headersHashes[2])
	require.Nil(t, err)
	require.Equal(t, &headers[2], header)

	header, err = headersCacher.GetHeaderByHash(headersHashes[1])
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)
}

func TestHeadersCacher_TestEvictionRemoveCorrectHeader2(t *testing.T) {
	t.Parallel()

	cacheSize := 99
	numHeadersToGenerate := 100

	headers, headersHashes := createASliceOfHeaders(numHeadersToGenerate)
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: 1},
	)

	for i := 0; i < numHeadersToGenerate-1; i++ {
		headersCacher.AddHeader(headersHashes[i], &headers[i])
		time.Sleep(time.Microsecond)
	}

	headersFromCache, _, err := headersCacher.GetHeadersByNonce(0)
	require.Nil(t, err)
	require.Equal(t, &headers[0], headersFromCache[0])

	headersCacher.AddHeader(headersHashes[numHeadersToGenerate-1], &headers[numHeadersToGenerate-1])

	header, err := headersCacher.GetHeaderByHash(headersHashes[0])
	require.Nil(t, err)
	require.Equal(t, &headers[0], header)

	header, err = headersCacher.GetHeaderByHash(headersHashes[1])
	require.Nil(t, header)
	require.Equal(t, headersCache.ErrHeaderNotFound, err)

	for i := 2; i <= cacheSize; i++ {
		header, err := headersCacher.GetHeaderByHash(headersHashes[i])
		require.Nil(t, err)
		require.Equal(t, &headers[i], header)
	}
}

func TestHeadersPool_Nonces(t *testing.T) {
	t.Parallel()

	numHeadersToGenerate := 1000
	cacheSize := 1000
	numHeadersToRemove := 100
	headersShard0, headersHashesShard0 := createASliceOfHeaders(numHeadersToGenerate)

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            cacheSize,
			NumElementsToRemoveOnEviction: numHeadersToRemove},
	)

	for i := 0; i < numHeadersToGenerate; i++ {
		headersCacher.AddHeader(headersHashesShard0[i], &headersShard0[i])
	}

	require.Equal(t, cacheSize, headersCacher.MaxSize())
	require.Equal(t, numHeadersToGenerate, headersCacher.Len())

	// get all keys and sort then to can verify if are ok
	nonces := headersCacher.Nonces()
	sort.Slice(nonces, func(i, j int) bool {
		return nonces[i] < nonces[j]
	})

	for i := uint64(0); i < uint64(len(nonces)); i++ {
		require.Equal(t, i, nonces[i])
	}
}

func TestHeadersPool_RegisterHandler(t *testing.T) {
	t.Parallel()

	wasCalled := false
	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            1000,
			NumElementsToRemoveOnEviction: 100},
	)
	wg := sync.WaitGroup{}
	wg.Add(1)
	handler := func(header data.HeaderHandler, hash []byte) {
		wasCalled = true
		wg.Done()
	}
	headersCacher.RegisterHandler(handler)
	header, hash := createASliceOfHeaders(1)
	headersCacher.AddHeader(hash[0], &header[0])

	wg.Wait()

	assert.True(t, wasCalled)
}

func TestHeadersPool_Clear(t *testing.T) {
	t.Parallel()

	headersCacher, _ := headersCache.NewHeadersPool(
		config.HeadersPoolConfig{
			MaxHeadersPerShard:            1000,
			NumElementsToRemoveOnEviction: 10},
	)
	header, hash := createASliceOfHeaders(1)
	headersCacher.AddHeader(hash[0], &header[0])

	headersCacher.Clear()

	require.Equal(t, 0, headersCacher.Len())
	require.Equal(t, 0, headersCacher.GetNumHeaders())
}

func createASliceOfHeaders(numHeaders int) ([]block.Block, [][]byte) {
	headers := make([]block.Block, 0)
	headersHashes := make([][]byte, 0)
	for i := 0; i < numHeaders; i++ {
		headers = append(headers, block.Block{Header: &block.BlockHeader{Nonce: uint64(i)}})
		headersHashes = append(headersHashes, []byte(fmt.Sprintf("%d", i)))
	}

	return headers, headersHashes
}

func createASliceOfHeadersNonce0(numHeaders int) ([]block.Block, [][]byte) {
	headers := make([]block.Block, 0)
	headersHashes := make([][]byte, 0)
	for i := 0; i < numHeaders; i++ {
		headers = append(headers, block.Block{Header: &block.BlockHeader{Nonce: 0}})
		headersHashes = append(headersHashes, []byte(fmt.Sprintf("nonce_%d", i)))
	}

	return headers, headersHashes
}
