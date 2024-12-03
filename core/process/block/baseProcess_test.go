package block

import (
	"testing"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getValidBlock(hasher hashing.Hasher) *block.Block {
	validBlock := &block.Block{
		Header: &block.BlockHeader{
			TxCount: 1,
		},
		TxHashes: [][]byte{hasher.Compute("valid")},
	}

	txRootHash, _ := validBlock.ComputeRootHash(hasher)
	validBlock.Header.TxRootHash = txRootHash

	return validBlock
}

func getInvalidBlock(hasher hashing.Hasher, txHashes [][]byte) *block.Block {
	validBlock := &block.Block{
		Header: &block.BlockHeader{
			TxCount: 1,
		},
		TxHashes: [][]byte{hasher.Compute("valid")},
	}

	txRootHash, _ := validBlock.ComputeRootHash(hasher)
	validBlock.Header.TxRootHash = txRootHash

	if len(txHashes) > 0 { // change tx hash to invalidate TxRootHash
		validBlock.TxHashes = txHashes
	}

	return validBlock
}

func Test_validateTxRootHash(t *testing.T) {
	hasher := &blake2b.Blake2b{HashSize: 32}

	bp := baseProcessor{
		hasher: hasher,
	}

	tests := []struct {
		name    string
		block   *block.Block
		wantErr bool
		err     error
	}{
		{
			name: "should ok when doesn't have rootHash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxRootHash: nil,
				},
			},
			wantErr: false,
			err:     nil,
		}, {
			name:    "should ok when send a valid rootHash",
			block:   getValidBlock(hasher),
			wantErr: false,
			err:     nil,
		},
		{
			name: "should error because empty blocks doesn't have txRootHash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    0,
					TxRootHash: []byte("invalid"),
				},
			},
			wantErr: true,
			err:     process.ErrTxRootHashInvalidForEmptyBlock,
		},
		{
			name: "should error because computed wrong txRootHash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    1,
					TxRootHash: []byte("invalid"),
				},
				TxHashes: [][]byte{
					hasher.Compute("valid"),
				},
			},
			wantErr: true,
			err:     process.ErrTxRootHashDoesNotMatch,
		},
		{
			name: "should error because empty txRootHash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    1,
					TxRootHash: []byte(""),
				},
			},
			wantErr: true,
			err:     process.ErrTxRootHashDoesNotMatch,
		},
		{
			name: "should error because empty hash in TxHashes",
			block: getInvalidBlock(hasher, [][]byte{
				[]byte(""),
			}),
			wantErr: true,
			err:     process.ErrTxRootHashDoesNotMatch,
		},
		{
			name: "should error when using EmptyHash without transactions",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    0,
					TxRootHash: hasher.EmptyHash(),
				},
				TxHashes: [][]byte{},
			},
			wantErr: true,
			err:     process.ErrTxRootHashInvalidForEmptyBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			err := bp.validateTxRootHash(tt.block)
			if tt.wantErr {
				require.Error(err)
				assert.Equal(tt.err, err)
			} else {
				require.NoError(err)
			}

		})
	}
}
