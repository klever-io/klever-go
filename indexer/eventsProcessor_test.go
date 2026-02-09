package indexer

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

func createTestEventsProcessor() *eventsProcessor {
	ep, _ := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  nil,
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

		ep := createTestEventsProcessor()
		require.False(t, ep.Enabled())
	})

	t.Run("enabled when event queue is active", func(t *testing.T) {
		originalUseEventQueue := UseEventQueue
		UseEventQueue = true
		defer func() { UseEventQueue = originalUseEventQueue }()

		ep := createTestEventsProcessor()
		require.True(t, ep.Enabled())
	})
}

func TestEventsProcessor_SaveBlock_DispatchesWhenEnabled(t *testing.T) {
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = true
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	ep := createTestEventsProcessor()

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}
	pool := &indexer.Pool{}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: pool,
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
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = false
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	ep := createTestEventsProcessor()

	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}
	pool := &indexer.Pool{}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: pool,
	})

	select {
	case <-testQueue:
		t.Fatal("expected no event to be dispatched when UseEventQueue is false")
	default:
	}
}

func TestEventsProcessor_SaveBlock_DispatchesTransactionEvents(t *testing.T) {
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = true
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	ep := createTestEventsProcessor()

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
	require.Equal(t, USER_TRANSACTION, userTxEvent.EvType)
	txs, ok := userTxEvent.Message.([]*data.Transaction)
	require.True(t, ok)
	require.Len(t, txs, 1)
	require.NotEmpty(t, txs[0].Contracts)

	txEvent := <-testQueue
	require.Equal(t, TRANSACTION, txEvent.EvType)
}

func TestEventsProcessor_SaveAccounts_DispatchesWhenEnabled(t *testing.T) {
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = true
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	ep := createTestEventsProcessor()
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
	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 10)
	EventQueue = testQueue
	UseEventQueue = false
	defer func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
	}()

	ep := createTestEventsProcessor()
	acc := createTestAccountStub()

	ep.SaveAccounts(100, []state.UserAccountHandler{acc})

	select {
	case <-testQueue:
		t.Fatal("expected no event when UseEventQueue is false")
	default:
	}
}

func TestEventsProcessor_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var ep *eventsProcessor
	require.True(t, ep.IsInterfaceNil())

	ep = createTestEventsProcessor()
	require.False(t, ep.IsInterfaceNil())
}
