package block

import (
	"testing"

	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/display"
	"github.com/stretchr/testify/assert"
)

func createGenesisBlock() *block.Block {
	rootHash := []byte("roothash")
	return &block.Block{
		Header: &block.BlockHeader{
			Nonce:        0,
			Slot:         0,
			RandSeed:     rootHash,
			PrevRandSeed: rootHash,
			TrieRoot:     rootHash,
			ParentHash:   rootHash,
			IsEpochStart: true,
		},
		ProducerSignature: rootHash,
	}
}

func TestDisplayBlock_DisplayTxBlockHeader(t *testing.T) {
	t.Parallel()

	lines := make([]*display.LineData, 0)
	blck := createGenesisBlock()
	blck.TxHashes = [][]byte{[]byte("hash1"), []byte("hash2"), []byte("hash3")}

	txCounter := NewTransactionCounter()
	lines = txCounter.displayTxBlockHeader(
		lines,
		blck,
	)

	assert.NotNil(t, lines)
	assert.Equal(t, len(blck.TxHashes), len(lines))
}
