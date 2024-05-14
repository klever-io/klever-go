package node_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/node/blockAPI"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockByHash_InvalidShardShouldErr(t *testing.T) {
	t.Parallel()

	n, _ := node.NewNode()

	blk, err := n.GetBlockByHash("invalidHash", false)
	assert.Error(t, err)
	assert.Nil(t, blk)
}

func TestGetBlockByHashFromNormalNode(t *testing.T) {
	t.Parallel()

	uint64Converter := mock.NewNonceHashConverterMock()

	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")
	storerMock := mock.NewStorerMock("", 0)
	n, _ := node.NewNode(
		node.WithInternalMarshalizer(&mock.MarshalizerFake{}),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
		}),
		node.WithUint64ByteSliceConverter(uint64Converter),
	)

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
			Epoch: epoch,
		},
	}
	headerBytes, _ := json.Marshal(header)
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
		Status: blockAPI.BlockStatusPending,
	}

	blk, err := n.GetBlockByHash(hex.EncodeToString(headerHash), false)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock, blk)
}

func TestGetBlockByNonceFromNormalNode(t *testing.T) {
	t.Parallel()

	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := "d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00"
	n, _ := node.NewNode(
		node.WithUint64ByteSliceConverter(mock.NewNonceHashConverterMock()),
		node.WithInternalMarshalizer(&mock.MarshalizerFake{}),
		node.WithDataStore(&mock.ChainStorerMock{
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				if unitType == retriever.HdrNonceHashDataUnit {
					return hex.DecodeString(headerHash)
				}
				blk := &block.Block{
					Header: &block.BlockHeader{
						Nonce: nonce,
						Slot:  slot,
						Epoch: epoch,
					},
				}
				blockBytes, _ := json.Marshal(blk)
				return blockBytes, nil
			},
		}),
	)

	expectedBlock := &api.Block{
		Block: &block.Block{
			Header: &block.BlockHeader{
				Nonce: nonce,
				Slot:  slot,
				Epoch: epoch,
			},
		},
		Hash:   headerHash,
		Status: blockAPI.BlockStatusOnChain,
	}

	blk, err := n.GetBlockByNonce(1, false)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock, blk)
}

func TestGetBlockByHashFromHistoryNode_StatusReverted(t *testing.T) {
	t.Parallel()

	/*historyProc := &testscommon.HistoryRepositoryStub{
		IsEnabledCalled: func() bool {
			return true
		},
		GetEpochByHashCalled: func(hash []byte) (uint32, error) {
			return 1, nil
		},
	}*/
	nonce := uint64(1)
	slot := uint64(2)
	epoch := uint32(1)
	headerHash := []byte("d08089f2ab739520598fd7aeed08c427460fe94f286383047f3f61951afc4e00")

	uint64Converter := mock.NewNonceHashConverterMock()
	storerMock := mock.NewStorerMock("", 0)
	n, _ := node.NewNode(
		node.WithInternalMarshalizer(&mock.MarshalizerFake{}),
		//node.WithHistoryRepository(historyProc),
		node.WithDataStore(&mock.ChainStorerMock{
			GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
				return storerMock
			},
			GetCalled: func(unitType retriever.UnitType, key []byte) ([]byte, error) {
				return storerMock.Get(key)
			},
		}),
		node.WithUint64ByteSliceConverter(uint64Converter),
	)

	header := &block.Block{
		Header: &block.BlockHeader{
			Nonce: nonce,
			Slot:  slot,
			Epoch: epoch,
		},
	}
	blockBytes, _ := json.Marshal(header)
	_ = storerMock.Put(headerHash, blockBytes)

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
		Status: blockAPI.BlockStatusReverted,
	}

	blk, err := n.GetBlockByHash(hex.EncodeToString(headerHash), false)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlock, blk)
}
