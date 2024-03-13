package sync

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/dataPool/headersCache"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/require"
)

func createMockHeadersSyncHandlerArgs() ArgsNewHeadersSyncHandler {
	return ArgsNewHeadersSyncHandler{
		StorageService: &cMock.ChainStorerMock{},
		Cache:          &cMock.HeadersCacherStub{},
		Marshalizer:    &cMock.MarshalizerFake{},
		Hasher:         &cMock.HasherMock{},
		EpochHandler: &cMock.EpochStartTriggerStub{
			IsEpochStartCalled: func() bool { return true },
		},
		RequestHandler:  &cMock.RequestHandlerStub{},
		Uint64Converter: &mock.Uint64ByteSliceConverterStub{},
	}
}

func generateTestCache() storage.Cacher {
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000, Shards: 1, SizeInBytes: 0})
	return cache
}

func generateTestUnit() storage.Storer {
	storer, _ := storageUnit.NewStorageUnit(
		generateTestCache(),
		memorydb.New(),
	)

	return storer
}

func initStore() *retriever.ChainStorer {
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.TransactionUnit, generateTestUnit())
	store.AddStorer(retriever.BlockUnit, generateTestUnit())
	store.AddStorer(retriever.HdrNonceHashDataUnit, generateTestUnit())
	return store
}

func TestHeadersSyncHandler(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.NotNil(t, headersSyncHandler)
	require.Nil(t, err)
	require.False(t, headersSyncHandler.IsInterfaceNil())
}

func TestHeadersSyncHandler_NilStorageErr(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()
	args.StorageService = nil

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, headersSyncHandler)
	require.Equal(t, common.ErrNilStorage, err)
}

func TestHeadersSyncHandler_NilCacheErr(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()
	args.Cache = nil

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, headersSyncHandler)
	require.Equal(t, common.ErrNilCacher, err)
}

func TestHeadersSyncHandler_NilEpochHandlerErr(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()
	args.EpochHandler = nil

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, headersSyncHandler)
	require.Equal(t, common.ErrNilEpochHandler, err)
}

func TestHeadersSyncHandler_NilMarshalizerEr(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()
	args.Marshalizer = nil

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, headersSyncHandler)
	require.Equal(t, common.ErrNilMarshalizer, err)
}

func TestHeadersSyncHandler_NilRequestHandlerEr(t *testing.T) {
	t.Parallel()

	args := createMockHeadersSyncHandlerArgs()
	args.RequestHandler = nil

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, headersSyncHandler)
	require.Equal(t, common.ErrNilRequestHandler, err)
}

func TestSyncEpochStartMetaHeader_MetaBlockInStorage(t *testing.T) {
	t.Parallel()

	meta := &block.Block{
		Header: &block.BlockHeader{
			Epoch:        1,
			IsEpochStart: true,
		},
	}
	args := createMockHeadersSyncHandlerArgs()
	args.StorageService = &cMock.ChainStorerMock{GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
		return &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return json.Marshal(meta)
			},
		}
	}}

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, err)

	err = headersSyncHandler.syncEpochStartMetaHeader(1, time.Second)
	require.Nil(t, err)

	metaBlock, err := headersSyncHandler.GetEpochStartMetaBlock()
	require.Nil(t, err)
	require.Equal(t, meta, metaBlock)
}

func TestSyncEpochStartMetaHeader_MissingHeaderTimeout(t *testing.T) {
	t.Parallel()

	localErr := errors.New("not found")
	args := createMockHeadersSyncHandlerArgs()
	args.StorageService = &cMock.ChainStorerMock{GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
		return &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return nil, localErr
			},
			GetFromEpochCalled: func(key []byte, epoch uint32) (bytes []byte, err error) {
				return nil, localErr
			},
		}
	}}

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, err)

	err = headersSyncHandler.syncEpochStartMetaHeader(1, time.Second)
	require.Equal(t, process.ErrTimeIsOut, err)
}

func TestSyncEpochStartMetaHeader_ReceiveWrongHeaderTimeout(t *testing.T) {
	t.Parallel()

	localErr := errors.New("not found")
	metaHash := []byte("metaHash")
	meta := &block.Block{Header: &block.BlockHeader{Epoch: 1}}
	args := createMockHeadersSyncHandlerArgs()
	args.Cache, _ = headersCache.NewHeadersPool(config.HeadersPoolConfig{
		MaxHeadersPerShard:            1000,
		NumElementsToRemoveOnEviction: 1,
	})
	args.EpochHandler = &cMock.EpochStartTriggerStub{IsEpochStartCalled: func() bool {
		return true
	}}

	args.StorageService = &cMock.ChainStorerMock{GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
		return &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return nil, localErr
			},
			GetFromEpochCalled: func(key []byte, epoch uint32) (bytes []byte, err error) {
				return nil, localErr
			},
		}
	}}

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, err)

	go func() {
		time.Sleep(100 * time.Millisecond)
		headersSyncHandler.metaBlockPool.AddHeader(metaHash, meta)
	}()

	err = headersSyncHandler.syncEpochStartMetaHeader(1, time.Second)
	require.Equal(t, process.ErrTimeIsOut, err)
}

func TestSyncEpochStartMetaHeader_ReceiveHeaderOk(t *testing.T) {
	t.Parallel()

	metaHash := []byte("epochStartBlock_0")
	meta := &block.Block{Header: &block.BlockHeader{Epoch: 1, IsEpochStart: true}}
	args := createMockHeadersSyncHandlerArgs()
	args.Cache, _ = headersCache.NewHeadersPool(config.HeadersPoolConfig{
		MaxHeadersPerShard:            1000,
		NumElementsToRemoveOnEviction: 1,
	})

	args.EpochHandler = &cMock.EpochStartTriggerStub{
		IsEpochStartCalled: func() bool {
			return true
		},
		EpochStartHdrHashCalled: func() []byte {
			return metaHash
		},
	}

	metaBytes, _ := args.Marshalizer.Marshal(meta)
	args.StorageService = &cMock.ChainStorerMock{GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
		return &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return metaBytes, nil
			},
			GetFromEpochCalled: func(key []byte, epoch uint32) (bytes []byte, err error) {
				return metaBytes, nil
			},
		}
	}}

	headersSyncHandler, err := NewHeadersSyncHandler(args)
	require.Nil(t, err)

	go func() {
		time.Sleep(100 * time.Millisecond)
		headersSyncHandler.metaBlockPool.AddHeader(metaHash, meta)
	}()

	err = headersSyncHandler.syncEpochStartMetaHeader(1, 2*time.Second)
	require.Nil(t, err)

	metaBlockSync, err := headersSyncHandler.GetEpochStartMetaBlock()
	require.Nil(t, err)
	require.Equal(t, meta, metaBlockSync)

}
