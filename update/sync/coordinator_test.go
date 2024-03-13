package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/require"
)

func createHeaderSyncHandler(retErr bool) update.HeaderSyncHandler {
	meta := &block.Block{
		Header: &block.BlockHeader{
			Nonce: 1, Epoch: 1, TrieRoot: []byte("metaRootHash"), IsEpochStart: true,
		},
	}
	args := createMockHeadersSyncHandlerArgs()
	args.StorageService = &cMock.ChainStorerMock{GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
		return &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				if retErr {
					return nil, errors.New("err")
				}

				return json.Marshal(meta)
			},
		}
	}}

	if !retErr {
		args.StorageService = initStore()
		byteArray := args.Uint64Converter.ToByteSlice(meta.Header.Nonce)
		_ = args.StorageService.Put(retriever.HdrNonceHashDataUnit, byteArray, []byte("firstPending"))
		marshaledData, _ := json.Marshal(meta)
		_ = args.StorageService.Put(retriever.BlockUnit, []byte("firstPending"), marshaledData)

		_ = args.StorageService.Put(retriever.BlockUnit, []byte(core.EpochStartIdentifier(meta.Header.Epoch)), marshaledData)
	}

	headersSyncHandler, _ := NewHeadersSyncHandler(args)
	return headersSyncHandler
}

func createPendingTxSyncHandler() update.PendingTransactionsSyncHandler {
	args := createMockArgs()
	args.Storages = &cMock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{
				GetCalled: func(key []byte) (bytes []byte, err error) {
					tx := &dataTransaction.Transaction{
						RawData: &dataTransaction.Transaction_Raw{
							Sender: []byte("snd"),
						},
					}
					return json.Marshal(tx)
				},
			}
		},
	}

	pendingTxsSyncer, _ := NewPendingTransactionsSyncer(args)
	return pendingTxsSyncer
}

func createSyncTrieState(retErr bool) update.EpochStartTriesSyncHandler {
	args := ArgsNewSyncAccountsDBsHandler{
		AccountsDBsSyncers: &mock.AccountsDBSyncersStub{
			GetCalled: func(key string) (syncer update.AccountsDBSyncer, err error) {
				return &mock.AccountsDBSyncerStub{
					SyncAccountsCalled: func(rootHash []byte) error {
						if retErr {
							return errors.New("err")
						}
						return nil
					},
				}, nil
			},
		},
		ActiveAccountsDBs: make(map[state.AccountsDbIdentifier]state.AccountsAdapter),
	}

	args.ActiveAccountsDBs[state.UserAccountsState] = &cMock.AccountsStub{
		RecreateAllTriesCalled: func(rootHash []byte) (map[string]data.Trie, error) {
			tries := make(map[string]data.Trie)
			tries[string(rootHash)] = &mock.TrieStub{
				CommitCalled: func() error {
					if retErr {
						return errors.New("err")
					}
					return nil
				},
			}
			return tries, nil
		},
	}

	args.ActiveAccountsDBs[state.PeerAccountsState] = &cMock.AccountsStub{
		RecreateAllTriesCalled: func(rootHash []byte) (map[string]data.Trie, error) {
			tries := make(map[string]data.Trie)
			tries[string(rootHash)] = &mock.TrieStub{
				CommitCalled: func() error {
					if retErr {
						return errors.New("err")
					}
					return nil
				},
			}
			return tries, nil
		},
	}

	triesSyncHandler, _ := NewSyncAccountsDBsHandler(args)
	return triesSyncHandler
}

func TestNewSyncState_Ok(t *testing.T) {
	t.Parallel()

	args := ArgsNewSyncState{
		Headers:      createHeaderSyncHandler(false),
		Tries:        createSyncTrieState(false),
		Transactions: createPendingTxSyncHandler(),
	}

	ss, err := NewSyncState(args)
	require.Nil(t, err)
	require.False(t, ss.IsInterfaceNil())

	err = ss.SyncAllState(1)
	require.Nil(t, err)
}

func TestNewSyncState_CannotSyncHeaderErr(t *testing.T) {
	t.Parallel()

	args := ArgsNewSyncState{
		Headers:      createHeaderSyncHandler(true),
		Tries:        createSyncTrieState(false),
		Transactions: createPendingTxSyncHandler(),
	}

	ss, err := NewSyncState(args)
	require.Nil(t, err)

	err = ss.SyncAllState(1)
	require.NotNil(t, err)
}

func TestNewSyncState_CannotSyncTriesErr(t *testing.T) {
	t.Parallel()

	args := ArgsNewSyncState{
		Headers:      createHeaderSyncHandler(false),
		Tries:        createSyncTrieState(true),
		Transactions: createPendingTxSyncHandler(),
	}

	ss, err := NewSyncState(args)
	require.Nil(t, err)

	err = ss.SyncAllState(1)
	require.NotNil(t, err)
}

func TestSyncState_SyncAllStateSyncTxsErr(t *testing.T) {
	t.Parallel()

	localErr := errors.New("err")
	args := ArgsNewSyncState{
		Headers: &mock.HeaderSyncHandlerStub{
			SyncUnFinishedMetaHeadersCalled: func(epoch uint32) error {
				return nil
			},
			GetEpochStartMetaBlockCalled: func() (metaBlock *block.Block, err error) {
				return &block.Block{
					Header: &block.BlockHeader{},
				}, nil
			},
		},
		Tries: &mock.EpochStartTriesSyncHandlerMock{},
		Transactions: &mock.PendingTransactionsSyncHandlerMock{
			SyncPendingTransactionsCalled: func(miniBlocks *block.Block, epoch uint32, ctx context.Context) error {
				return localErr
			},
		},
	}

	ss, err := NewSyncState(args)
	require.Nil(t, err)

	err = ss.SyncAllState(0)
	require.True(t, errors.Is(err, localErr))
}
