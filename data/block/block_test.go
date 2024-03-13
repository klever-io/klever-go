package block_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

func TestCalcularRootHash_EmptySlice(t *testing.T) {
	hasher := &mock.HasherMock{}

	blk := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: make([][]byte, 0),
	}

	root, err := blk.ComputeRootHash(hasher)
	assert.Nil(t, err)
	assert.Equal(t, hasher.EmptyHash(), root)
}

func TestCalcularRootHash_OneLeaf(t *testing.T) {
	hasher := &mock.HasherMock{}

	info := make([][]byte, 0)
	hash, _ := hex.DecodeString("82c42480e36a15c1d794e8d4e19f46fe6c9be6b6240c8dc8744b219aaa845c91")
	info = append(info, hash)

	blk := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: info,
	}

	root, err := blk.ComputeRootHash(hasher)
	assert.Nil(t, err)
	assert.Equal(t, "b4a334ae3a983557c74626d307b94bcb8188ad583bd5b3a0084fb51e28dbe4b4", hex.EncodeToString(root))
}

func TestCalcularRootHash_TwoLeaves(t *testing.T) {
	hasher := &mock.HasherMock{}

	info := []string{
		"3603015cebfeba9e2995cfa834d587d56407ffd860423b2fa9c4cb677aeaf8ec",
		"7d08d15b913a3567381a59b6ae68d9bb04efc2ac74fec27041f6f55492574533",
	}
	data := make([][]byte, 0)
	for _, st := range info {
		hash, _ := hex.DecodeString(st)
		data = append(data, hash)
	}

	blk := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: data,
	}

	root, err := blk.ComputeRootHash(hasher)
	assert.Nil(t, err)
	assert.Equal(t, "727dcb43f3db4b398ea8b928505626b84b03f4a8e6f7e027b3717122e41af788", hex.EncodeToString(root))
}

func TestCalcularRootHash_ThreeLeaves(t *testing.T) {
	hasher := &mock.HasherMock{}

	info := []string{
		"3603015cebfeba9e2995cfa834d587d56407ffd860423b2fa9c4cb677aeaf8ec",
		"7d08d15b913a3567381a59b6ae68d9bb04efc2ac74fec27041f6f55492574533",
		"6359f0868171b1d194cbee1af2f16ea598ae8fad666d9b012c8ed2b79a236ec4",
	}
	data := make([][]byte, 0)
	for _, st := range info {
		hash, _ := hex.DecodeString(st)
		data = append(data, hash)
	}

	blk := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: data,
	}

	root, err := blk.ComputeRootHash(hasher)
	assert.Nil(t, err)
	assert.Equal(t, "0c33813e014ba60590e8090e2fe71b23b89ca023f76bea63b46aac4c80473251", hex.EncodeToString(root))
}

func TestCalcularRootHash(t *testing.T) {
	hasher := &mock.HasherMock{}

	info := []string{
		"8c14f0db3df150123e6f3dbbf30f8b955a8249b62ac1d1ff16284aefa3d06d87",
		"fff2525b8931402dd09222c50775608f75787bd2b87e56995a7bdd30f79702c4",
		"6359f0868171b1d194cbee1af2f16ea598ae8fad666d9b012c8ed2b79a236ec4",
		"e9a66845e05d5abc0ad04ec80f774a7e585c6e8db975962d069a522137b80c1d",
	}
	data := make([][]byte, 0)
	for _, st := range info {
		// hash := hasher.Compute(st)
		hash, _ := hex.DecodeString(st)
		data = append(data, hash)
	}

	blk := &block.Block{
		Header:   &block.BlockHeader{},
		TxHashes: data,
	}

	root, err := blk.ComputeRootHash(hasher)
	assert.Nil(t, err)
	assert.Equal(t, "2f4bf3a18b541d7ae702a42c6c017448496bdd477ed43638b656114916b8d604", hex.EncodeToString(root))
}
