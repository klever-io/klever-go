package blockchain_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewBlockChain_ShouldWork(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	assert.False(t, check.IfNil(bc))
}

func TestBlockChain_SetNilAppStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetAppStatusHandler(nil)

	assert.Equal(t, blockchain.ErrNilAppStatusHandler, err)
}

func TestBlockChain_SetAppStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	ash := &mock.AppStatusHandlerStub{}
	err := bc.SetAppStatusHandler(ash)

	assert.Nil(t, err)
}

func TestBlockChain_SettersAndGetters(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 4,
		},
	}
	genesis := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 0,
		},
	}
	hdrHash := []byte("hash")
	genesisHash := []byte("genesis hash")

	bc.SetCurrentBlockHeaderHash(hdrHash)
	bc.SetGenesisHeaderHash(genesisHash)

	err := bc.SetGenesisHeader(genesis)
	assert.Nil(t, err)

	err = bc.SetCurrentBlockHeader(hdr)
	assert.Nil(t, err)

	assert.Equal(t, hdr.Clone(), bc.GetCurrentBlockHeader())
	assert.False(t, hdr == bc.GetCurrentBlockHeader())

	assert.Equal(t, genesis.Clone(), bc.GetGenesisHeader())
	assert.False(t, genesis == bc.GetGenesisHeader())

	assert.Equal(t, hdrHash, bc.GetCurrentBlockHeaderHash())
	assert.Equal(t, genesisHash, bc.GetGenesisHeaderHash())
}

func TestBlockChain_SettersAndGettersNilValues(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetGenesisHeader(nil)
	assert.Nil(t, err)

	err = bc.SetCurrentBlockHeader(nil)
	assert.Nil(t, err)

	assert.Nil(t, bc.GetGenesisHeader())
	assert.Nil(t, bc.GetCurrentBlockHeader())
}

func TestBlockChain_CreateNewHeader(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	assert.Equal(t, &block.Block{}, bc.CreateNewHeader())
}

func TestBlockChain_SettersWithInvalidBlock(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	err := bc.SetCurrentBlockHeader(&mock.HeaderHandlerStub{})
	assert.Equal(t, common.ErrInvalidHeaderType, err)

	err = bc.SetGenesisHeader(&mock.HeaderHandlerStub{})
	assert.Equal(t, common.ErrInvalidHeaderType, err)
}

func TestBlockChain_GetCurrentBlockRootHash(t *testing.T) {
	t.Parallel()

	bc := blockchain.NewBlockChain()

	// empty blockchain should return empty root hash
	assert.Equal(t, []byte(nil), bc.GetCurrentBlockRootHash())

	// set a block header and check the root hash
	hdr := &block.Block{
		Header: &block.BlockHeader{
			TrieRoot: []byte("root hash"),
		},
	}
	bc.SetCurrentBlockHeader(hdr)

	assert.Equal(t, []byte("root hash"), bc.GetCurrentBlockRootHash())
}
