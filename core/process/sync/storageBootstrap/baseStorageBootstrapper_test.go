package storageBootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
)

var marshalizer = &mock.MarshalizerMock{}

func bootStorerMock() (*mock.StorerStub, process.BootStorer) {

	slotInStorage := int64(5)

	storer := &mock.StorerStub{
		PutCalled: func(key, data []byte) error {
			rn := bootstrapStorage.SlotNum{}
			err := marshalizer.Unmarshal(&rn, data)
			slotInStorage = rn.Num
			if err != nil {
				fmt.Println(err.Error())
			}
			return nil
		},
		GetCalled: func(key []byte) ([]byte, error) {
			return marshalizer.Marshal(&bootstrapStorage.SlotNum{Num: slotInStorage})
		},
	}
	bt, _ := bootstrapStorage.NewBootstrapStorer(marshalizer, storer)
	return storer, bt
}

// Test getMinSlot
func TestStorageBootstrapper_GetMinSlot(t *testing.T) {
	t.Parallel()

	t.Run("with genesis header returns genesis slot", func(t *testing.T) {

		expectedSlot := uint64(42)

		mockChain := &mock.BlockChainMock{
			GetGenesisHeaderCalled: func() data.HeaderHandler {
				return &block.Block{
					Header: &block.BlockHeader{
						Slot: expectedSlot,
					},
				}
			},
		}

		sb := &storageBootstrapper{
			blkc: mockChain,
		}

		result := sb.getMinSlot()
		assert.Equal(t, expectedSlot, result)
	})

	t.Run("without genesis header returns 0", func(t *testing.T) {
		// Arrange
		mockChain := &mock.BlockChainMock{
			GetGenesisHeaderCalled: func() data.HeaderHandler {
				return nil
			},
		}

		sb := &storageBootstrapper{
			blkc: mockChain,
		}

		result := sb.getMinSlot()
		assert.Equal(t, uint64(0), result)
	})
}

// Test loadAndApplyBlocks
func TestStorageBootstrapper_LoadAndApplyBlocks(t *testing.T) {
	t.Run("handles boot storer get error", func(t *testing.T) {
		storer, mockBoot := bootStorerMock()
		expectedErr := errors.New("get error")

		storer.GetCalled = func(key []byte) ([]byte, error) {
			return nil, expectedErr
		}

		sb := &storageBootstrapper{
			bootStorer: mockBoot,
		}

		result, err := sb.loadAndApplyBlocks(100)
		assert.Empty(t, result)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("handles corrupt bootstrap data", func(t *testing.T) {
		storer, mockBoot := bootStorerMock()
		bootstrapData := &bootstrapStorage.BootstrapData{
			LastSlot: 100,
		}

		storer.GetCalled = func(key []byte) ([]byte, error) {
			return marshalizer.Marshal(bootstrapData)
		}

		sb := &storageBootstrapper{
			bootStorer: mockBoot,
		}

		result, err := sb.loadAndApplyBlocks(100)
		assert.Empty(t, result)
		assert.Equal(t, sync.ErrCorruptBootstrapFromStorageDb, err)
	})

	t.Run("handles corrupt bootstrap data with result", func(t *testing.T) {
		storer, mockBoot := bootStorerMock()
		bootstrapData := &bootstrapStorage.BootstrapData{
			LastSlot: 100,
		}

		storer.GetCalled = func(key []byte) ([]byte, error) {
			return marshalizer.Marshal(bootstrapData)
		}

		sb := &storageBootstrapper{
			bootStorer: mockBoot,
		}

		result, err := sb.loadAndApplyBlocks(101)
		assert.Len(t, result, 1)
		assert.Equal(t, int64(100), result[0].LastSlot)
		assert.Equal(t, sync.ErrCorruptBootstrapFromStorageDb, err)
	})

	t.Run("handles valid bootstrap data wrong chainID", func(t *testing.T) {
		bootstrapData := &bootstrapStorage.BootstrapData{
			LastSlot: 100,
			LastHeader: &bootstrapStorage.BootstrapHeaderInfo{
				Hash:  []byte{1, 2, 3},
				Nonce: 100,
			},
			HighestFinalBlockNonce: 100,
		}

		sb := newValidStorageBootstrapper(bootstrapData)
		sb.chainID = "wrongChainID"

		result, err := sb.loadAndApplyBlocks(98)
		assert.Len(t, result, 1)
		assert.Equal(t, int64(101), result[0].LastSlot)
		assert.Equal(t, sync.ErrCorruptBootstrapFromStorageDb, err)
	})

	t.Run("handles valid bootstrap data", func(t *testing.T) {
		bootstrapData := &bootstrapStorage.BootstrapData{
			LastSlot: 100,
			LastHeader: &bootstrapStorage.BootstrapHeaderInfo{
				Hash:  []byte{1, 2, 3},
				Nonce: 100,
			},
			HighestFinalBlockNonce: 101,
		}

		var headerCalled []byte

		sb := newValidStorageBootstrapper(bootstrapData)
		sb.blkc.(*mock.BlockChainMock).SetCurrentBlockHeaderAndHashCalled = func(_ data.HeaderHandler, hash []byte) error {
			headerCalled = hash
			return nil
		}

		result, err := sb.loadAndApplyBlocks(98)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, uint64(101), result[0].HighestFinalBlockNonce)
		assert.Equal(t, bootstrapData.LastHeader.Hash, headerCalled)
	})
}

func newValidStorageBootstrapper(bootstrapData *bootstrapStorage.BootstrapData) *storageBootstrapper {
	storer, mockBoot := bootStorerMock()

	storer.GetCalled = func(key []byte) ([]byte, error) {

		if bytes.Equal(key, []byte(core.HighestSlotFromBootStorage)) {
			return marshalizer.Marshal(&bootstrapStorage.SlotNum{Num: int64(bootstrapData.LastSlot)})
		}

		return marshalizer.Marshal(bootstrapStorage.BootstrapData{
			LastSlot:               bootstrapData.LastSlot + 1,
			LastHeader:             bootstrapData.LastHeader,
			HighestFinalBlockNonce: bootstrapData.HighestFinalBlockNonce,
		})
	}

	store := &mock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) ([]byte, error) {
					if bytes.Equal(key, bootstrapData.LastHeader.Hash) {
						return marshalizer.Marshal(bootstrapData.LastHeader)
					}
					return nil, errors.New("not found")
				},
			}
		},
	}

	sb := &storageBootstrapper{
		bootStorer:         mockBoot,
		bootstrapSlotIndex: 98,

		marshalizer: marshalizer,
		store:       store,
		blkExecutor: &consensusMock.BlockProcessorMock{
			RevertStateToBlockCalled: func(block data.HeaderHandler) error {
				return nil
			},
		},
		blkc: &mock.BlockChainMock{
			SetCurrentBlockHeaderCalled: func(header data.HeaderHandler) error {
				return nil
			},
			SetCurrentBlockHeaderHashCalled: func(hash []byte) {

			},
		},
		forkDetector: &mock.ForkDetectorMock{
			AddHeaderCalled: func(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
				return nil
			},
		},
		nodesCoordinator: &mock.NodesCoordinatorMock{},
		epochStartTrigger: &mock.EpochStartTriggerStub{
			LoadStateCalled: func(key []byte) error {
				return nil
			},
		},
	}

	msBoot := &metaStorageBootstrapper{
		storageBootstrapper: sb,
	}

	sb.bootstrapper = msBoot

	return sb
}

// Test handleBlockLoadingError
func TestStorageBootstrapper_HandleBlockLoadingError(t *testing.T) {
	t.Run("restores to genesis and returns error", func(t *testing.T) {
		var headerCleared bool
		var headerHashCleared bool

		sb := newValidStorageBootstrapper(&bootstrapStorage.BootstrapData{})
		sb.blkc.(*mock.BlockChainMock).GetGenesisHeaderCalled = func() data.HeaderHandler {
			return &block.Block{}
		}
		sb.blkc.(*mock.BlockChainMock).SetCurrentBlockHeaderAndHashCalled = func(header data.HeaderHandler, hash []byte) error {
			if header == nil {
				headerCleared = true
			}
			if hash == nil {
				headerHashCleared = true
			}
			return nil
		}

		testErr := errors.New("test error")

		err := sb.handleBlockLoadingError(testErr)
		assert.Equal(t, process.ErrNotEnoughValidBlocksInStorage, err)
		assert.True(t, headerCleared)
		assert.True(t, headerHashCleared)
		assert.Equal(t, int64(0), sb.bootStorer.GetHighestSlot())

	})
}

// Test loadBlocks
func TestStorageBootstrapper_LoadBlocks(t *testing.T) {
	t.Run("starts from genesis when highest slot <= min slot", func(t *testing.T) {
		sb := newValidStorageBootstrapper(&bootstrapStorage.BootstrapData{})

		err := sb.loadBlocks()
		assert.Equal(t, process.ErrNotEnoughValidBlocksInStorage, err)
	})

	t.Run("handles load and apply error", func(t *testing.T) {
		sb := newValidStorageBootstrapper(&bootstrapStorage.BootstrapData{
			LastSlot: 100,
			LastHeader: &bootstrapStorage.BootstrapHeaderInfo{
				Hash:  []byte{1, 2, 3},
				Nonce: 100,
			},
			HighestFinalBlockNonce: 101,
		})

		err := sb.loadBlocks()
		assert.Equal(t, process.ErrNotEnoughValidBlocksInStorage, err)
	})

	t.Run("handles load success", func(t *testing.T) {
		sb := newValidStorageBootstrapper(&bootstrapStorage.BootstrapData{
			LastSlot: 95,
			LastHeader: &bootstrapStorage.BootstrapHeaderInfo{
				Hash:  []byte{1, 2, 3},
				Nonce: 95,
			},
			HighestFinalBlockNonce: 95,
		})

		err := sb.loadBlocks()
		assert.NoError(t, err)
	})
}

func newStorageBootstrapperAfterRollBack(t *testing.T, raiseFinalNonce bool) (*storageBootstrapper, *int, *[]uint64) {
	t.Helper()

	bootStorer, err := bootstrapStorage.NewBootstrapStorer(marshalizer, mock.NewStorerMock("", 0))
	assert.NoError(t, err)

	headers := make(map[string]*block.Block)
	for nonce := uint64(3); nonce <= 5; nonce++ {
		hash := []byte(fmt.Sprintf("hash%d", nonce))
		headers[string(hash)] = &block.Block{Header: &block.BlockHeader{Nonce: nonce, Slot: nonce + 5}}
		err = bootStorer.Put(int64(nonce)+5, &bootstrapStorage.BootstrapData{
			LastHeader:             &bootstrapStorage.BootstrapHeaderInfo{Nonce: nonce, Hash: hash},
			HighestFinalBlockNonce: nonce - 1,
		})
		assert.NoError(t, err)
	}

	if raiseFinalNonce {
		err = bootStorer.RaiseHighestFinalBlockNonce(9, 4, []byte("hash4"))
		assert.NoError(t, err)
	}
	err = bootStorer.SaveLastSlot(9)
	assert.NoError(t, err)

	numSetFinalToLastCheckpoint := 0
	addedNonces := make([]uint64, 0)

	sb := newValidStorageBootstrapper(&bootstrapStorage.BootstrapData{})
	sb.bootStorer = bootStorer
	sb.store = &mock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) ([]byte, error) {
					header, ok := headers[string(key)]
					if !ok {
						return nil, errors.New("not found")
					}
					return marshalizer.Marshal(header)
				},
			}
		},
	}
	sb.forkDetector = &mock.ForkDetectorMock{
		AddHeaderCalled: func(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
			addedNonces = append(addedNonces, header.GetNonce())
			return nil
		},
		SetFinalToLastCheckpointCalled: func() {
			numSetFinalToLastCheckpoint++
		},
	}

	return sb, &numSetFinalToLastCheckpoint, &addedNonces
}

func TestStorageBootstrapper_ApplyBootInfos(t *testing.T) {
	t.Run("final block equal to head sets final to last checkpoint", func(t *testing.T) {
		sb, numSetFinalToLastCheckpoint, addedNonces := newStorageBootstrapperAfterRollBack(t, true)

		headerInfo, err := sb.bootStorer.Get(sb.bootStorer.GetHighestSlot())
		assert.NoError(t, err)
		assert.Equal(t, uint64(4), headerInfo.LastHeader.Nonce)
		assert.Equal(t, uint64(4), headerInfo.HighestFinalBlockNonce)

		bootInfos, err := sb.getBootInfos(headerInfo)
		assert.NoError(t, err)
		assert.Len(t, bootInfos, 1)

		err = sb.applyBootInfos(bootInfos)
		assert.NoError(t, err)
		assert.Equal(t, 1, *numSetFinalToLastCheckpoint)
		assert.Equal(t, []uint64{4}, *addedNonces)
	})

	t.Run("final block behind head keeps final from applied headers", func(t *testing.T) {
		sb, numSetFinalToLastCheckpoint, addedNonces := newStorageBootstrapperAfterRollBack(t, false)

		headerInfo, err := sb.bootStorer.Get(sb.bootStorer.GetHighestSlot())
		assert.NoError(t, err)
		assert.Equal(t, uint64(4), headerInfo.LastHeader.Nonce)
		assert.Equal(t, uint64(3), headerInfo.HighestFinalBlockNonce)

		bootInfos, err := sb.getBootInfos(headerInfo)
		assert.NoError(t, err)
		assert.Len(t, bootInfos, 2)

		err = sb.applyBootInfos(bootInfos)
		assert.NoError(t, err)
		assert.Equal(t, 0, *numSetFinalToLastCheckpoint)
		assert.Equal(t, []uint64{3, 4}, *addedNonces)
	})
}
