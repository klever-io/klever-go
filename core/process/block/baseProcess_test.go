package block

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/data"
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
		hasher:         hasher,
		forkController: mock.NewForkControllerStub(),
	}

	tests := []struct {
		name        string
		block       *block.Block
		disableFork bool
		wantErr     bool
		err         error
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
		{
			name: "allow zero txRootHash when there are no transactions prior to fork",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount: 0,
					// 0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8
					TxRootHash: hasher.EmptyHash(),
				},
				TxHashes: [][]byte{},
			},
			disableFork: true,
			wantErr:     false,
			err:         nil,
		},
		{
			name: "error zero txRootHash when there are no transactions prior to fork wrong hash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    0,
					TxRootHash: []byte{0x00, 0x57, 0x51, 0xc0, 0x26, 0xe5, 0x43, 0xb2, 0xe8, 0xab, 0x2e, 0xb0, 0x60, 0x99, 0xda, 0xa1, 0xd1, 0xe5, 0xdf, 0x47, 0x77, 0x8f, 0x77, 0x87, 0xfa, 0xab, 0x45, 0xcd, 0xf1, 0x2f, 0xe3, 0xa8},
				},
				TxHashes: [][]byte{},
			},
			disableFork: true,
			wantErr:     true,
			err:         process.ErrTxRootHashDoesNotMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			bp.forkController.(*mock.ForkControllerStub).SetFork("EnableSmartContracts", !tt.disableFork)

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

func Test_updateStateStorage_ImportDbMode_SkipsCheckpoint(t *testing.T) {
	t.Parallel()

	checkpointCalled := false
	accountsStub := &mock.AccountsStub{
		IsPruningEnabledCalled: func() bool {
			return true
		},
		SetStateCheckpointCalled: func(rootHash []byte) {
			checkpointCalled = true
		},
		CancelPruneCalled: func(rootHash []byte, identifier data.TriePruningIdentifier) {},
		PruneTrieCalled:   func(rootHash []byte, identifier data.TriePruningIdentifier) {},
	}

	bp := baseProcessor{
		processingMode:         core.ImportDb,
		stateCheckpointModulus: 1, // checkpoint every block
	}

	header := &mock.HeaderHandlerStub{
		GetNonceCalled: func() uint64 {
			return 100 // divisible by modulus, would trigger checkpoint
		},
	}

	bp.updateStateStorage(header, []byte("rootHash"), []byte("prevRootHash"), accountsStub)

	assert.False(t, checkpointCalled, "SetStateCheckpoint should NOT be called in import-db mode")
}

func Test_updateStateStorage_NormalMode_CallsCheckpoint(t *testing.T) {
	t.Parallel()

	checkpointCalled := false
	accountsStub := &mock.AccountsStub{
		IsPruningEnabledCalled: func() bool {
			return true
		},
		SetStateCheckpointCalled: func(rootHash []byte) {
			checkpointCalled = true
		},
		CancelPruneCalled: func(rootHash []byte, identifier data.TriePruningIdentifier) {},
		PruneTrieCalled:   func(rootHash []byte, identifier data.TriePruningIdentifier) {},
	}

	bp := baseProcessor{
		processingMode:         core.Normal,
		stateCheckpointModulus: 1, // checkpoint every block
	}

	header := &mock.HeaderHandlerStub{
		GetNonceCalled: func() uint64 {
			return 100 // divisible by modulus, should trigger checkpoint
		},
	}

	bp.updateStateStorage(header, []byte("rootHash"), []byte("prevRootHash"), accountsStub)

	assert.True(t, checkpointCalled, "SetStateCheckpoint should be called in normal mode")
}
