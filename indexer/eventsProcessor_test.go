package indexer

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/require"
)

type indexerStub struct {
	saveBlockCalled                    func(args *indexer.ArgsSaveBlockData)
	revertIndexedBlockCalled           func(header nodeData.HeaderHandler)
	saveEpochInfoCalled                func(epoch uint32, validators []kapp.ValidatorAccountInfoHandler)
	saveAccountsCalled                 func(blockTimestamp int64, acc []state.UserAccountHandler)
	savePeersAccountsCalled            func(validators []kapp.ValidatorAccountInfoHandler)
	updateProposalsAndParametersCalled func(proposalIDs []string)
	isNilIndexer                       bool
}

func (s *indexerStub) SaveBlock(args *indexer.ArgsSaveBlockData) {
	if s.saveBlockCalled != nil {
		s.saveBlockCalled(args)
	}
}
func (s *indexerStub) RevertIndexedBlock(header nodeData.HeaderHandler) {
	if s.revertIndexedBlockCalled != nil {
		s.revertIndexedBlockCalled(header)
	}
}
func (s *indexerStub) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
	if s.saveEpochInfoCalled != nil {
		s.saveEpochInfoCalled(epoch, validators)
	}
}
func (s *indexerStub) SaveAccounts(blockTimestamp int64, acc []state.UserAccountHandler) {
	if s.saveAccountsCalled != nil {
		s.saveAccountsCalled(blockTimestamp, acc)
	}
}
func (s *indexerStub) SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler) {
	if s.savePeersAccountsCalled != nil {
		s.savePeersAccountsCalled(validators)
	}
}
func (s *indexerStub) SaveAssets(_ []*kapps.KDAData) {}
func (s *indexerStub) Close() error                  { return nil }
func (s *indexerStub) IsInterfaceNil() bool          { return s == nil }
func (s *indexerStub) IsNilIndexer() bool            { return s.isNilIndexer }
func (s *indexerStub) UpdateProposalsAndParameters(proposalIDs []string) {
	if s.updateProposalsAndParametersCalled != nil {
		s.updateProposalsAndParametersCalled(proposalIDs)
	}
}

func createTestEventsProcessor(t *testing.T) *eventsProcessor {
	t.Helper()
	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  nil,
	})
	require.NoError(t, err)
	return ep
}

func createTestEventsProcessorWithIndexer(idx Indexer) *eventsProcessor {
	ep, _ := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  idx,
	})
	return ep
}

func createTestAccountStub() *mock.UserAccountHandlerStub {
	return &mock.UserAccountHandlerStub{
		AddressBytesCalled:    func() []byte { return []byte("testaddr") },
		GetNonceCalled:        func() uint64 { return 5 },
		GetNameCalled:         func() []byte { return []byte("test") },
		GetBalanceCalled:      func(_ []byte, _ bool) int64 { return 1000 },
		GetRootHashCalled:     func() []byte { return []byte("roothash") },
		GetAllowanceCalled:    func() int64 { return 50 },
		GetCodeHashCalled:     func() []byte { return []byte("codehash") },
		GetCodeMetadataCalled: func() []byte { return []byte("metadata") },
		GetPermissionsCalled:  func() []*state.Permission { return nil },
		GetUserKDACalled: func(_ []byte, _ []byte, _ bool) (*kapps.UserKDA, error) {
			return &kapps.UserKDA{
				FrozenBalance: 200,
				Buckets:       nil,
			}, nil
		},
	}
}

func saveAndRestoreEventQueue(t *testing.T, useQueue bool) chan Event {
	t.Helper()
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = useQueue
	t.Cleanup(func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	})
	return testQueue
}

func TestNewEventsProcessor(t *testing.T) {
	t.Parallel()

	t.Run("nil marshalizer should error", func(t *testing.T) {
		t.Parallel()
		_, err := NewEventsProcessor(ArgEventsProcessor{
			Marshalizer:              nil,
			Hasher:                   &mock.HasherMock{},
			AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
			ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		})
		require.NotNil(t, err)
	})

	t.Run("nil hasher should error", func(t *testing.T) {
		t.Parallel()
		_, err := NewEventsProcessor(ArgEventsProcessor{
			Marshalizer:              &mock.MarshalizerMock{},
			Hasher:                   nil,
			AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
			ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		})
		require.NotNil(t, err)
	})

	t.Run("nil address pubkey converter should error", func(t *testing.T) {
		t.Parallel()
		_, err := NewEventsProcessor(ArgEventsProcessor{
			Marshalizer:              &mock.MarshalizerMock{},
			Hasher:                   &mock.HasherMock{},
			AddressPubkeyConverter:   nil,
			ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		})
		require.NotNil(t, err)
	})

	t.Run("nil validator pubkey converter should error", func(t *testing.T) {
		t.Parallel()
		_, err := NewEventsProcessor(ArgEventsProcessor{
			Marshalizer:              &mock.MarshalizerMock{},
			Hasher:                   &mock.HasherMock{},
			AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
			ValidatorPubkeyConverter: nil,
		})
		require.NotNil(t, err)
	})

	t.Run("valid arguments should work", func(t *testing.T) {
		t.Parallel()
		ep, err := NewEventsProcessor(ArgEventsProcessor{
			Marshalizer:              &mock.MarshalizerMock{},
			Hasher:                   &mock.HasherMock{},
			AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
			ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		})
		require.Nil(t, err)
		require.NotNil(t, ep)
	})
}

func TestEventsProcessor_Enabled(t *testing.T) {
	t.Run("disabled when no indexer and no event queue", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		UseEventQueue = false
		defer func() { UseEventQueue = originalUseEventQueue }()

		ep := createTestEventsProcessor(t)
		require.False(t, ep.Enabled())
	})

	t.Run("enabled when event queue is active", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		UseEventQueue = true
		defer func() { UseEventQueue = originalUseEventQueue }()

		ep := createTestEventsProcessor(t)
		require.True(t, ep.Enabled())
	})

	t.Run("enabled when indexer is active", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		UseEventQueue = false
		defer func() { UseEventQueue = originalUseEventQueue }()

		ep := createTestEventsProcessorWithIndexer(&indexerStub{isNilIndexer: false})
		require.True(t, ep.Enabled())
	})
}

func TestEventsProcessor_SaveBlock_DispatchesWhenEnabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: &indexer.Pool{},
	})

	select {
	case event := <-testQueue:
		require.Equal(t, BLOCKS, event.EvType)
		require.NotNil(t, event.Message)
	default:
		t.Fatal("expected block event to be dispatched")
	}
}

func TestEventsProcessor_SaveBlock_SkipsWhenDisabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, false)
	ep := createTestEventsProcessor(t)

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: &indexer.Pool{},
	})

	select {
	case <-testQueue:
		t.Fatal("expected no event to be dispatched when UseEventQueue is false")
	default:
	}
}

func TestEventsProcessor_SaveBlock_DispatchesTransactionEvents(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, _ := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}
	pool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{
			"txHash1": tx,
		},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: pool,
	})

	blockEvent := <-testQueue
	require.Equal(t, BLOCKS, blockEvent.EvType)

	userTxEvent := <-testQueue
	require.Equal(t, USER_TRANSACTIONS, userTxEvent.EvType)
	txs, ok := userTxEvent.Message.([]*data.Transaction)
	require.True(t, ok)
	require.Len(t, txs, 1)
	require.NotEmpty(t, txs[0].Contracts)

	txEvent := <-testQueue
	require.Equal(t, TRANSACTIONS, txEvent.EvType)
}

func TestEventsProcessor_SaveBlock_SkipsWebsocketWhenIndexerActive(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	var saveBlockCalled int32
	idx := &indexerStub{
		isNilIndexer: false,
		saveBlockCalled: func(_ *indexer.ArgsSaveBlockData) {
			atomic.AddInt32(&saveBlockCalled, 1)
		},
	}
	ep := createTestEventsProcessorWithIndexer(idx)

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: &indexer.Pool{},
	})

	select {
	case <-testQueue:
		t.Fatal("expected no websocket events when indexer is active")
	default:
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&saveBlockCalled))
}

func TestEventsProcessor_SaveBlock_NilPool(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: nil,
	})

	event := <-testQueue
	require.Equal(t, BLOCKS, event.EvType)

	select {
	case <-testQueue:
		t.Fatal("expected no tx events with nil pool")
	default:
	}
}

func TestEventsProcessor_SaveAccounts_DispatchesWhenEnabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)
	acc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case event := <-testQueue:
		require.Equal(t, ACCOUNTS, event.EvType)
		accountsMap, ok := event.Message.(map[string]*data.AccountInfo)
		require.True(t, ok)
		require.Len(t, accountsMap, 1)
		for _, info := range accountsMap {
			require.Equal(t, uint64(5), info.Nonce)
			require.Equal(t, "test", info.Name)
			require.Equal(t, int64(1000), info.Balance)
			require.Equal(t, int64(200), info.FrozenBalance)
			require.Equal(t, int64(50), info.Allowance)
		}
	default:
		t.Fatal("expected account event to be dispatched")
	}
}

func TestEventsProcessor_SaveAccounts_SkipsWhenDisabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, false)
	ep := createTestEventsProcessor(t)
	acc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case <-testQueue:
		t.Fatal("expected no event when UseEventQueue is false")
	default:
	}
}

func TestEventsProcessor_SaveAccounts_SkipsWebsocketWhenIndexerActive(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	var called int32
	idx := &indexerStub{
		isNilIndexer: false,
		saveAccountsCalled: func(_ int64, _ []state.UserAccountHandler) {
			atomic.AddInt32(&called, 1)
		},
	}
	ep := createTestEventsProcessorWithIndexer(idx)
	acc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case <-testQueue:
		t.Fatal("expected no websocket account events when indexer is active")
	default:
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&called))
}

func TestEventsProcessor_SaveAccounts_GetUserKDAError(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	failAcc := &mock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return []byte("failaddr") },
		GetUserKDACalled: func(_ []byte, _ []byte, _ bool) (*kapps.UserKDA, error) {
			return nil, errors.New("kda error")
		},
	}
	goodAcc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{failAcc, goodAcc})

	select {
	case event := <-testQueue:
		accountsMap, ok := event.Message.(map[string]*data.AccountInfo)
		require.True(t, ok)
		require.Len(t, accountsMap, 1)
	default:
		t.Fatal("expected account event to be dispatched")
	}
}

func TestEventsProcessor_SaveAccounts_EmptySlice(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	ep.SaveAccounts(100, []state.UserAccountHandler{})

	select {
	case <-testQueue:
		t.Fatal("expected no event for empty accounts")
	default:
	}
}

func TestEventsProcessor_SaveAccounts_WithPermissions(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	acc := createTestAccountStub()
	acc.GetPermissionsCalled = func() []*state.Permission {
		return []*state.Permission{
			{
				ID:             1,
				Type:           state.Permission_Owner,
				PermissionName: "owner",
				Threshold:      1,
				Operations:     []byte{0x01, 0x02},
				Signers: []*state.Key{
					{
						Address: []byte("signeraddr"),
						Weight:  10,
					},
				},
			},
		}
	}

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case event := <-testQueue:
		accountsMap, ok := event.Message.(map[string]*data.AccountInfo)
		require.True(t, ok)
		require.Len(t, accountsMap, 1)
		for _, info := range accountsMap {
			require.Len(t, info.Permissions, 1)
			require.Equal(t, "owner", info.Permissions[0].PermissionName)
			require.Len(t, info.Permissions[0].Signers, 1)
		}
	default:
		t.Fatal("expected account event to be dispatched")
	}
}

func TestEventsProcessor_RevertIndexedBlock(t *testing.T) {
	t.Run("nil indexer does nothing", func(t *testing.T) {
		ep := createTestEventsProcessor(t)
		ep.RevertIndexedBlock(&dataBlock.Block{
			Header: &dataBlock.BlockHeader{Nonce: 1},
		})
	})

	t.Run("delegates to indexer", func(t *testing.T) {
		var called int32
		idx := &indexerStub{
			isNilIndexer: false,
			revertIndexedBlockCalled: func(_ nodeData.HeaderHandler) {
				atomic.AddInt32(&called, 1)
			},
		}
		ep := createTestEventsProcessorWithIndexer(idx)

		ep.RevertIndexedBlock(&dataBlock.Block{
			Header: &dataBlock.BlockHeader{Nonce: 1},
		})

		require.Equal(t, int32(1), atomic.LoadInt32(&called))
	})
}

func TestEventsProcessor_SaveValidatorsRating(t *testing.T) {
	t.Run("nil indexer does nothing", func(t *testing.T) {
		ep := createTestEventsProcessor(t)
		ep.SaveValidatorsRating(nil)
	})

	t.Run("delegates to indexer", func(t *testing.T) {
		var called int32
		idx := &indexerStub{
			isNilIndexer: false,
			savePeersAccountsCalled: func(_ []kapp.ValidatorAccountInfoHandler) {
				atomic.AddInt32(&called, 1)
			},
		}
		ep := createTestEventsProcessorWithIndexer(idx)

		ep.SaveValidatorsRating(nil)

		require.Equal(t, int32(1), atomic.LoadInt32(&called))
	})
}

func TestEventsProcessor_SaveEpochInfo(t *testing.T) {
	t.Run("nil indexer does nothing", func(t *testing.T) {
		ep := createTestEventsProcessor(t)
		ep.SaveEpochInfo(1, nil)
	})

	t.Run("delegates to indexer", func(t *testing.T) {
		var called int32
		idx := &indexerStub{
			isNilIndexer: false,
			saveEpochInfoCalled: func(_ uint32, _ []kapp.ValidatorAccountInfoHandler) {
				atomic.AddInt32(&called, 1)
			},
		}
		ep := createTestEventsProcessorWithIndexer(idx)

		ep.SaveEpochInfo(1, nil)

		require.Equal(t, int32(1), atomic.LoadInt32(&called))
	})
}

func TestEventsProcessor_UpdateProposalsAndParameters(t *testing.T) {
	t.Run("nil indexer does nothing", func(t *testing.T) {
		ep := createTestEventsProcessor(t)
		ep.UpdateProposalsAndParameters([]string{"1"})
	})

	t.Run("delegates to indexer", func(t *testing.T) {
		var called int32
		idx := &indexerStub{
			isNilIndexer: false,
			updateProposalsAndParametersCalled: func(_ []string) {
				atomic.AddInt32(&called, 1)
			},
		}
		ep := createTestEventsProcessorWithIndexer(idx)

		ep.UpdateProposalsAndParameters([]string{"1"})

		require.Equal(t, int32(1), atomic.LoadInt32(&called))
	})
}

func TestEventsProcessor_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var ep *eventsProcessor
	require.True(t, ep.IsInterfaceNil())

	ep = createTestEventsProcessor(t)
	require.False(t, ep.IsInterfaceNil())
}

func TestTrySendEvent_QueueFull(t *testing.T) {
	originalEventQueue := EventQueue
	fullQueue := make(chan Event, 1)
	fullQueue <- Event{EvType: BLOCKS, Message: "filler"}
	EventQueue = fullQueue
	defer func() { EventQueue = originalEventQueue }()

	trySendEvent(Event{EvType: TRANSACTIONS, Message: "dropped"})

	event := <-fullQueue
	require.Equal(t, BLOCKS, event.EvType)
}

func createTestEventsProcessorWithKApp(t *testing.T, ctrl kapp.KAppController) *eventsProcessor {
	t.Helper()
	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  nil,
		KAppController:           ctrl,
	})
	require.NoError(t, err)
	return ep
}

func TestEventsProcessor_GetAllowanceWithPendingRewards(t *testing.T) {
	t.Parallel()

	t.Run("nil kappsController returns base allowance", func(t *testing.T) {
		t.Parallel()

		ep := createTestEventsProcessor(t)

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 1000
			},
		}

		result := ep.getAllowanceWithPendingRewards(userAccount)
		require.Equal(t, int64(1000), result)
	})

	t.Run("with pending rewards adds to allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 500, nil
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		ep := createTestEventsProcessorWithKApp(t, kappController)

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 2000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := ep.getAllowanceWithPendingRewards(userAccount)
		require.Equal(t, int64(2500), result)
	})

	t.Run("zero pending rewards returns base allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 0, nil
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		ep := createTestEventsProcessorWithKApp(t, kappController)

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 3000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := ep.getAllowanceWithPendingRewards(userAccount)
		require.Equal(t, int64(3000), result)
	})

	t.Run("error getting pending rewards returns base allowance", func(t *testing.T) {
		t.Parallel()

		validatorsKapp := &mock.ValidatorsKAppStub{
			GetPendingRewardsCalled: func(address []byte) (int64, error) {
				return 0, errors.New("some error")
			},
		}

		kappController := &stub.KAppControllerStub{
			GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
				return validatorsKapp
			},
		}

		ep := createTestEventsProcessorWithKApp(t, kappController)

		userAccount := &mock.UserAccountHandlerStub{
			GetAllowanceCalled: func() int64 {
				return 4000
			},
			AddressBytesCalled: func() []byte {
				return []byte("testaddress")
			},
		}

		result := ep.getAllowanceWithPendingRewards(userAccount)
		require.Equal(t, int64(4000), result)
	})
}

func TestEventsProcessor_SaveAccounts_AllowanceIncludesPendingRewards(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	validatorsKapp := &mock.ValidatorsKAppStub{
		GetPendingRewardsCalled: func(address []byte) (int64, error) {
			return 100, nil
		},
	}

	kappController := &stub.KAppControllerStub{
		GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
			return validatorsKapp
		},
	}

	ep := createTestEventsProcessorWithKApp(t, kappController)
	acc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case event := <-testQueue:
		require.Equal(t, ACCOUNTS, event.EvType)
		accountsMap, ok := event.Message.(map[string]*data.AccountInfo)
		require.True(t, ok)
		require.Len(t, accountsMap, 1)
		for _, info := range accountsMap {
			require.Equal(t, int64(150), info.Allowance)
		}
	default:
		t.Fatal("expected account event to be dispatched")
	}
}
