package block

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
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
		TxHashes:  [][]byte{hasher.Compute("valid")},
		TxResults: []uint32{0},
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
		TxHashes:  [][]byte{hasher.Compute("valid")},
		TxResults: []uint32{0},
	}

	txRootHash, _ := validBlock.ComputeRootHash(hasher)
	validBlock.Header.TxRootHash = txRootHash

	if len(txHashes) > 0 { // change tx hash to invalidate TxRootHash
		validBlock.TxHashes = txHashes
	}

	return validBlock
}

func getBlockWithTxHashesSplitAtWrongWidth(hasher hashing.Hasher) *block.Block {
	blk := &block.Block{
		Header: &block.BlockHeader{
			TxCount: 2,
		},
		TxHashes:  [][]byte{hasher.Compute("first"), hasher.Compute("second")},
		TxResults: []uint32{0, 0},
	}

	txRootHash, _ := blk.ComputeRootHash(hasher)
	blk.Header.TxRootHash = txRootHash

	joined := append(append([]byte{}, blk.TxHashes[0]...), blk.TxHashes[1]...)
	blk.TxHashes = [][]byte{joined[:hasher.Size()-1], joined[hasher.Size()-1:]}

	return blk
}

func Test_validateTxRootHash(t *testing.T) {
	hasher := &blake2b.Blake2b{HashSize: 32}

	bp := baseProcessor{
		hasher:         hasher,
		forkController: mock.NewForkControllerStub(),
	}

	tests := []struct {
		name         string
		block        *block.Block
		disableFork  bool
		wantErr      bool
		err          error
		bodyMismatch bool
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
				TxResults: []uint32{0},
			},
			wantErr:      true,
			err:          process.ErrTxRootHashDoesNotMatch,
			bodyMismatch: true,
		},
		{
			name: "should error because empty txRootHash",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount:    1,
					TxRootHash: []byte(""),
				},
				TxHashes: [][]byte{
					hasher.Compute("valid"),
				},
				TxResults: []uint32{0},
			},
			wantErr:      true,
			err:          process.ErrTxRootHashDoesNotMatch,
			bodyMismatch: true,
		},
		{
			name: "should error because empty hash in TxHashes",
			block: getInvalidBlock(hasher, [][]byte{
				[]byte(""),
			}),
			wantErr:      true,
			err:          process.ErrInvalidTxHashLength,
			bodyMismatch: true,
		},
		{
			name:         "should error when tx hashes are not hasher sized even if the root matches",
			block:        getBlockWithTxHashesSplitAtWrongWidth(hasher),
			wantErr:      true,
			err:          process.ErrInvalidTxHashLength,
			bodyMismatch: true,
		},
		{
			name: "should error when the tx count does not match the tx hashes",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount: 2,
				},
				TxHashes:  [][]byte{hasher.Compute("valid")},
				TxResults: []uint32{0},
			},
			wantErr:      true,
			err:          process.ErrInvalidTXCount,
			bodyMismatch: true,
		},
		{
			name: "should error when tx results do not pair up with the tx hashes",
			block: func() *block.Block {
				blk := getValidBlock(hasher)
				blk.TxResults = nil
				return blk
			}(),
			wantErr:      true,
			err:          process.ErrInvalidTXResultsCount,
			bodyMismatch: true,
		},
		{
			name: "should error when the tx count is zero but the block has tx hashes",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount: 0,
				},
				TxHashes:  [][]byte{hasher.Compute("valid")},
				TxResults: []uint32{0},
			},
			wantErr:      true,
			err:          process.ErrInvalidTXCount,
			bodyMismatch: true,
		},
		{
			name: "should error when a block without transactions carries tx results",
			block: &block.Block{
				Header: &block.BlockHeader{
					TxCount: 0,
				},
				TxResults: []uint32{0},
			},
			wantErr:      true,
			err:          process.ErrInvalidTXResultsCount,
			bodyMismatch: true,
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
				assert.ErrorIs(err, tt.err)
				assert.Equal(tt.bodyMismatch, errors.Is(err, process.ErrBlockBodyNotMatchingHeader))
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

type slotTimestampStub struct {
	consensusMock.SlotManagerMock
	valid bool
}

func (s *slotTimestampStub) ValidateSlotTimestamp(_ int64, _ int64) bool {
	return s.valid
}

func Test_checkBlockValidity_FirstBlockAgainstGenesis(t *testing.T) {
	t.Parallel()

	hasher := &blake2b.Blake2b{HashSize: 32}
	genesisHash := []byte("genesis-hash")
	genesisSeed := []byte("genesis-rand-seed")
	newGenesis := func() *block.Block {
		return &block.Block{Header: &block.BlockHeader{Timestamp: 1000, RandSeed: genesisSeed}}
	}
	validFirstBlock := func() *block.Block {
		return &block.Block{Header: &block.BlockHeader{
			Nonce:        1,
			Slot:         2,
			Timestamp:    1008,
			ParentHash:   genesisHash,
			PrevRandSeed: genesisSeed,
			TxFees:       100,
			TxBurnedFees: 100,
		}}
	}
	newProcessor := func(genesis data.HeaderHandler, v7Epoch uint32, validSlotTimestamp bool) *baseProcessor {
		forkController := mock.NewForkControllerStub()
		forkController.FixAuditChangesV7Epoch = &v7Epoch

		return &baseProcessor{
			hasher:                       hasher,
			forkController:               forkController,
			validatorStatisticsProcessor: &mock.ValidatorStatisticsProcessorStub{},
			slotManager: &slotTimestampStub{
				SlotManagerMock: consensusMock.SlotManagerMock{IndexCalled: func() int64 { return 10 }},
				valid:           validSlotTimestamp,
			},
			blockChain: &mock.BlockChainMock{
				GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil },
				GetGenesisHeaderCalled:      func() data.HeaderHandler { return genesis },
				GetGenesisHeaderHashCalled:  func() []byte { return genesisHash },
			},
		}
	}

	tests := []struct {
		name   string
		mutate func(b *block.Block)
		err    error
	}{
		{name: "valid first block", mutate: func(_ *block.Block) {}},
		{name: "slot not after genesis", mutate: func(b *block.Block) { b.Header.Slot = 0 }, err: process.ErrLowerSlotInBlock},
		{name: "timestamp before genesis", mutate: func(b *block.Block) { b.Header.Timestamp = 999 }, err: process.ErrInvalidBlockTimestamp},
		{name: "wrong prev rand seed", mutate: func(b *block.Block) { b.Header.PrevRandSeed = []byte("forged") }, err: process.ErrRandSeedDoesNotMatch},
		{name: "epoch differs from genesis", mutate: func(b *block.Block) { b.Header.Epoch = 1 }, err: process.ErrEpochDoesNotMatch},
		{name: "forged burned fees", mutate: func(b *block.Block) { b.Header.TxBurnedFees = 1 }, err: process.ErrInvalidTXFees},
		{name: "root hash on an empty block", mutate: func(b *block.Block) { b.Header.TxRootHash = []byte("forged") }, err: process.ErrTxRootHashInvalidForEmptyBlock},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			blk := validFirstBlock()
			tt.mutate(blk)

			err := newProcessor(newGenesis(), 0, true).checkBlockValidity(blk)
			if tt.err == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.err)
		})
	}

	t.Run("timestamp outside its slot", func(t *testing.T) {
		t.Parallel()

		err := newProcessor(newGenesis(), 0, false).checkBlockValidity(validFirstBlock())
		require.ErrorIs(t, err, process.ErrInvalidBlockTimestamp)
	})

	t.Run("nil genesis header", func(t *testing.T) {
		t.Parallel()

		err := newProcessor(nil, 0, true).checkBlockValidity(validFirstBlock())
		require.ErrorIs(t, err, process.ErrNilBlockHeader)
	})

	t.Run("live network block 1 passes with every fork off", func(t *testing.T) {
		t.Parallel()

		// the shape of block 1 on mainnet, testnet and devnet: slot 1, genesis+4s, no txs
		historicalFirstBlock := func(txRootHash []byte) *block.Block {
			return &block.Block{Header: &block.BlockHeader{
				Nonce:        1,
				Slot:         1,
				Timestamp:    1004,
				ParentHash:   genesisHash,
				PrevRandSeed: genesisSeed,
				TxRootHash:   txRootHash,
			}}
		}

		bp := newProcessor(newGenesis(), 0, true)
		forksOff := mock.NewForkControllerStub()
		forksOff.SetAll(false)
		bp.forkController = forksOff

		// the real slot manager with 4s slots from genesis, as the nodes run it
		genesisTime := time.Unix(newGenesis().GetTimestamp(), 0)
		slotManager, err := slot.NewSlotManager(genesisTime, genesisTime.Add(40*time.Second), 4*time.Second, &consensusMock.SyncTimerMock{}, 0)
		require.NoError(t, err)
		bp.slotManager = slotManager

		require.NoError(t, bp.checkBlockValidity(historicalFirstBlock(nil)))
		require.NoError(t, bp.checkBlockValidity(historicalFirstBlock(hasher.EmptyHash())))

		blk := historicalFirstBlock(nil)
		blk.Header.PrevRandSeed = []byte("forged")
		require.ErrorIs(t, bp.checkBlockValidity(blk), process.ErrRandSeedDoesNotMatch)

		blk = historicalFirstBlock(nil)
		blk.Header.Timestamp = 1008
		require.ErrorIs(t, bp.checkBlockValidity(blk), process.ErrInvalidBlockTimestamp, "slot 1 ends at genesis+8s")
	})
}
