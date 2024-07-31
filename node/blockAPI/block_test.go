package blockAPI

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
)

func createMockMetaAPIProcessor(
	blockHeaderHash []byte,
	storerMock *mock.StorerMock,
	withHistory bool,
	withKey bool,
) *apiBlockProcessor {
	return NewAPIBlockProcessor(
		&APIBlockProcessorArg{
			Marshalizer: &mock.ProtoMarshalizerMock{},
			Store: &mock.ChainStorerMock{
				GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
					return storerMock
				},
				GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
					if unitType == retriever.BlockUnit {
						return storerMock.Get(key)
					}
					if withKey {
						return storerMock.Get(key)
					}
					return blockHeaderHash, nil
				},
			},
			Uint64ByteSliceConverter: mock.NewNonceHashConverterMock(),
			/* FIXME: HistoryRepo: &testscommon.HistoryRepositoryStub{
				GetEpochByHashCalled: func(hash []byte) (uint32, error) {
					return 1, nil
				},
				IsEnabledCalled: func() bool {
					return withHistory
				},
			},*/
		},
	)
}

func TestMetaAPIBlockProcessor_GetBlockByHashInvalidHashShouldErr(t *testing.T) {
	t.Parallel()

	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	storerMock := mock.NewStorerMock("", 0)

	metaAPIBlockProcessor := createMockMetaAPIProcessor(
		headerHash,
		storerMock,
		true,
		false,
	)

	blk, err := metaAPIBlockProcessor.GetBlockByHash([]byte("invalidHash"), false)
	assert.Nil(t, blk)
	assert.Error(t, err)
}

func TestMetaAPIBlockProcessor_GetBlockByNonceInvalidNonceShouldErr(t *testing.T) {
	t.Parallel()

	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	storerMock := mock.NewStorerMock("", 0)

	metaAPIBlockProcessor := createMockMetaAPIProcessor(
		headerHash,
		storerMock,
		true,
		false,
	)

	blk, err := metaAPIBlockProcessor.GetBlockByNonce(100, false)
	assert.Nil(t, blk)
	assert.Error(t, err)
}

func TestMetaAPIBlockProcessor_GetBlockByHashFromHistoryNode(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtoMarshalizerMock{}

	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	storerMock := mock.NewStorerMock("", 0)
	uint64Converter := mock.NewNonceHashConverterMock()

	metaAPIBlockProcessor := createMockMetaAPIProcessor(
		headerHash,
		storerMock,
		false,
		false,
	)

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
			Epoch: epoch,
		},
	}
	headerBytes, _ := marshalizer.Marshal(header)
	_ = storerMock.Put(headerHash, headerBytes)

	nonceBytes := uint64Converter.ToByteSlice(nonce)
	_ = storerMock.Put(nonceBytes, headerHash)

	expectedBlock := &api.Block{
		Block: &block.Block{
			Header: &block.BlockHeader{
				Nonce: nonce,
				Slot:  slot,
				Epoch: epoch,
			},
		},
		Hash:   hex.EncodeToString(headerHash),
		Status: BlockStatusPending,
	}

	blk, err := metaAPIBlockProcessor.GetBlockByHash(headerHash, false)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock.Hash, blk.Hash)
}

func TestMetaAPIBlockProcessor_GetBlockByNonceFromHistoryNode(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtoMarshalizerMock{}

	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	storerMock := mock.NewStorerMock("", 0)

	metaAPIBlockProcessor := createMockMetaAPIProcessor(
		headerHash,
		storerMock,
		true,
		false,
	)

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
			Epoch: epoch,
		},
	}

	headerBytes, _ := marshalizer.Marshal(header)
	_ = storerMock.Put(headerHash, headerBytes)

	expectedBlock := &api.Block{
		Block: &block.Block{
			Header: &block.BlockHeader{
				Nonce: nonce,
				Slot:  slot,
				Epoch: epoch,
			},
		},
		Hash:         hex.EncodeToString(headerHash),
		Status:       BlockStatusOnChain,
		Transactions: make([]*api.Transaction, 0),
	}

	blk, err := metaAPIBlockProcessor.GetBlockByNonce(1, true)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock.Hash, blk.Hash)
}

func TestMetaAPIBlockProcessor_GetBlockByHashFromHistoryNodeStatusReverted(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtoMarshalizerMock{}

	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	storerMock := mock.NewStorerMock("", 0)
	uint64Converter := mock.NewNonceHashConverterMock()

	metaAPIBlockProcessor := createMockMetaAPIProcessor(
		headerHash,
		storerMock,
		true,
		true,
	)

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
			Epoch: epoch,
		},
	}
	headerBytes, _ := marshalizer.Marshal(header)
	_ = storerMock.Put(headerHash, headerBytes)

	nonceBytes := uint64Converter.ToByteSlice(nonce)
	correctHash := []byte("correct-hash")
	_ = storerMock.Put(nonceBytes, correctHash)

	expectedBlock := &api.Block{
		Block: &block.Block{
			Header: &block.BlockHeader{
				Nonce: nonce,
				Slot:  slot,
				Epoch: epoch,
			},
		},
		Hash:   hex.EncodeToString(headerHash),
		Status: BlockStatusReverted,
	}

	blk, err := metaAPIBlockProcessor.GetBlockByHash(headerHash, true)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock.Hash, blk.Hash)
}
