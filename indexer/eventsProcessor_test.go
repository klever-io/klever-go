package indexer

import (
	"encoding/hex"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	ptx "github.com/klever-io/klever-go/core/process/transaction"
	nodeData "github.com/klever-io/klever-go/data"
	dataBlock "github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/assert"
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

func createTestEventsProcessorWithAccountsDB(t *testing.T, accountsDB state.AccountsAdapter) *eventsProcessor {
	t.Helper()
	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		AccountsDB:               accountsDB,
	})
	require.NoError(t, err)
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

func drainTestQueue(ch chan Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// drainAllEvents drains up to 10 pending events from queue (comfortably more than any
// single SaveBlock call dispatches) into a slice, for tests that need to inspect which
// event types arrived without asserting on exact ordering.
func drainAllEvents(queue chan Event) []Event {
	events := make([]Event, 0, 10)
	for i := 0; i < 10; i++ {
		select {
		case ev := <-queue:
			events = append(events, ev)
		default:
			return events
		}
	}
	return events
}

// findEventType returns the first event of evType in events, or nil if none matches.
func findEventType(events []Event, evType EventType) *Event {
	for _, ev := range events {
		if ev.EvType == evType {
			evCopy := ev
			return &evCopy
		}
	}
	return nil
}

// pad32 returns a 32-byte slice with s copied into it, zero-padded.
func pad32(s string) []byte {
	b := make([]byte, 32)
	copy(b, s)
	return b
}

// bareTransactionWithReceipts creates a minimal transaction (no contracts) carrying
// the given receipts, exercising the receipt-based account-detection path.
func bareTransactionWithReceipts(senderBytes []byte, receipts ...*transaction.Transaction_Receipt) *transaction.Transaction {
	return &transaction.Transaction{
		Signature: [][]byte{[]byte("sig")},
		RawData:   &transaction.Transaction_Raw{Sender: senderBytes},
		Result:    transaction.Transaction_SUCCESS,
		Receipts:  receipts,
	}
}

// makeReceipt builds a Transaction_Receipt where data[0] is {receiptType, cID=0}
// followed by fields.
func makeReceipt(receiptType ptx.ReceiptType, fields ...[]byte) *transaction.Transaction_Receipt {
	d := make([][]byte, 0, 1+len(fields))
	d = append(d, []byte{byte(receiptType), 0})
	d = append(d, fields...)
	return &transaction.Transaction_Receipt{Data: d}
}

// runReceiptAddressTest is a shared helper for single-address receipt tests: it
// runs SaveBlock and asserts that expectedAddrBytes was loaded from accountsDB.
func runReceiptAddressTest(t *testing.T, tx *transaction.Transaction, expectedAddrBytes []byte) {
	t.Helper()
	testQueue := saveAndRestoreEventQueue(t, true)

	loaded := make(map[string]struct{})
	acc := createTestAccountStub()
	ep := createTestEventsProcessorWithAccountsDB(t, &mock.AccountsStub{
		GetExistingAccountCalled: func(addrBytes []byte) (state.AccountHandler, error) {
			loaded[hex.EncodeToString(addrBytes)] = struct{}{}
			return acc, nil
		},
	})

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"tx1": tx}}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})
	drainTestQueue(testQueue)

	require.Contains(t, loaded, hex.EncodeToString(expectedAddrBytes))
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
		require.NotNil(t, ep.logsAndEventsProc)
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
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

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

func TestEventsProcessor_SaveBlock_DispatchesBlocksEventWithIndexerActive(t *testing.T) {
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

	ev := <-testQueue
	require.Equal(t, BLOCKS, ev.EvType)

	select {
	case extra := <-testQueue:
		t.Fatalf("unexpected extra event: %s", extra.EvType)
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

func TestEventsProcessor_SaveBlock_DispatchesLogEvents(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

	logHandler := &transaction.Log{
		Address:    []byte("contractaddr"),
		ContractID: 7,
		Events: []*transaction.Event{
			{
				Address:     []byte("eventaddr"),
				Identifier:  []byte("transfer"),
				Topics:      [][]byte{[]byte("topic1")},
				Data:        [][]byte{[]byte("data1")},
				IsSystemLog: true,
			},
		},
	}

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Txs:  map[string]nodeData.TransactionHandler{"txHash1": tx},
		Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txHash1"}},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	logsEvent := findEventType(drainAllEvents(testQueue), LOGS)
	require.NotNil(t, logsEvent, "expected a LOGS event to be dispatched")
	logsDB, ok := logsEvent.Message.([]*data.Logs)
	require.True(t, ok)
	require.Len(t, logsDB, 1)

	// Full payload contract, not just "some log arrived": ID is what the hub uses as the
	// envelope hash, Caller/Status/ResultCode resolve from txsMap (keyed by the same raw
	// tx-hash bytes as pool.Txs — if that keying is ever "unified" to hex elsewhere, this
	// must catch it going blank), and addresses/topics/data are hex-encoded.
	entry := logsDB[0]
	assert.Equal(t, hex.EncodeToString([]byte("txHash1")), entry.ID)
	assert.Equal(t, hex.EncodeToString([]byte("contractaddr")), entry.Address)
	assert.Equal(t, hex.EncodeToString([]byte("sender")), entry.Caller, "Caller must resolve from txsMap")
	assert.Equal(t, int32(7), entry.ContractID)
	assert.Equal(t, "success", entry.Status)
	assert.Equal(t, transaction.Transaction_Ok.String(), entry.ResultCode)
	require.Len(t, entry.Events, 1)
	assert.Equal(t, hex.EncodeToString([]byte("eventaddr")), entry.Events[0].Address)
	assert.Equal(t, "transfer", entry.Events[0].Identifier)
	assert.Equal(t, []string{hex.EncodeToString([]byte("topic1"))}, entry.Events[0].Topics)
	assert.Equal(t, []string{hex.EncodeToString([]byte("data1"))}, entry.Events[0].Data)
	assert.True(t, entry.Events[0].IsSystemLog)
}

// TestEventsProcessor_SaveBlock_DispatchesLogEvents_WhenPrepareIsSkipped: LOGS must still
// dispatch when prepared == nil (empty Txs forces prepare()'s early-return path), with
// Caller/Status/ResultCode degrading to zero values on the resulting nil txsMap instead of
// panicking.
func TestEventsProcessor_SaveBlock_DispatchesLogEvents_WhenPrepareIsSkipped(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	logHandler := &transaction.Log{
		Address:    []byte("contractaddr"),
		ContractID: 7,
		Events: []*transaction.Event{
			{
				Address:    []byte("eventaddr"),
				Identifier: []byte("transfer"),
			},
		},
	}

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txHash1"}},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	logsEvent := findEventType(drainAllEvents(testQueue), LOGS)
	require.NotNil(t, logsEvent, "LOGS must still dispatch even when prepared == nil")
	logsDB, ok := logsEvent.Message.([]*data.Logs)
	require.True(t, ok)
	require.Len(t, logsDB, 1)

	entry := logsDB[0]
	assert.Equal(t, hex.EncodeToString([]byte("contractaddr")), entry.Address)
	assert.Equal(t, int32(7), entry.ContractID)
	assert.Empty(t, entry.Caller, "txsMap is nil: Caller must degrade to zero value, not panic")
	assert.Empty(t, entry.Status)
	assert.Empty(t, entry.ResultCode)
}

// TestEventsProcessor_SaveBlock_SkipsLogConversionWithNoSubscriber guards the fix for a
// real perf/consensus-timing concern: dispatchLogEvents used to always pay the full
// bech32/hex-encoding PrepareLogsForDB conversion on the block-commit goroutine whenever
// UseEventQueue was on, even when no client subscribed to LOGS and no mirror was
// configured — the hub's own subscriber gate only skipped cost later, per entry, after
// conversion had already run for all of them. LogsSubscriberChecker lets the commit
// goroutine skip the conversion (and the LOGS event) entirely in that case.
func TestEventsProcessor_SaveBlock_SkipsLogConversionWithNoSubscriber(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	original := LogsSubscriberChecker
	LogsSubscriberChecker = func() bool { return false }
	t.Cleanup(func() { LogsSubscriberChecker = original })

	ep := createTestEventsProcessor(t)

	logHandler := &transaction.Log{Address: []byte("contractaddr"), ContractID: 7}
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txHash1"}},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	logsEvent := findEventType(drainAllEvents(testQueue), LOGS)
	assert.Nil(t, logsEvent, "LOGS must not dispatch when LogsSubscriberChecker reports nobody is listening")
}

func TestEventsProcessor_SaveBlock_NoLogsNoOp(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{"txHash1": tx},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	logsEvent := findEventType(drainAllEvents(testQueue), LOGS)
	require.Nil(t, logsEvent, "expected no LOGS event when pool.Logs is empty")
}

func TestEventsProcessor_SaveBlock_NilPool_SkipsLogEvents(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}

	require.NotPanics(t, func() {
		ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: nil})
	})

	event := <-testQueue
	require.Equal(t, BLOCKS, event.EvType)

	select {
	case ev := <-testQueue:
		require.NotEqual(t, LOGS, ev.EvType, "expected no LOGS event with nil pool")
	default:
	}
}

func TestEventsProcessor_SaveBlock_SkipsLogEventsWhenDisabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, false)
	ep := createTestEventsProcessor(t)

	logHandler := &transaction.Log{Address: []byte("contractaddr")}
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txHash1"}},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	select {
	case ev := <-testQueue:
		t.Fatalf("expected no events when UseEventQueue is false, got %s", ev.EvType)
	default:
	}
}

func TestEventsProcessor_SaveBlock_DispatchesAccountEvents(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			return acc, nil
		},
	}
	ep := createTestEventsProcessorWithAccountsDB(t, accountsDB)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

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

	var accountEvent *Event
	for i := 0; i < 10; i++ {
		select {
		case ev := <-testQueue:
			if ev.EvType == ACCOUNTS {
				evCopy := ev
				accountEvent = &evCopy
			}
		default:
			i = 10
		}
	}
	require.NotNil(t, accountEvent, "expected an ACCOUNTS event to be dispatched")
	accountsMap, ok := accountEvent.Message.(map[string]*data.AccountInfo)
	require.True(t, ok)
	require.NotEmpty(t, accountsMap)
}

func TestEventsProcessor_SaveBlock_SkipsAccountEventsWhenNoAccountsDB(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)
	ep := createTestEventsProcessor(t)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

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

	for i := 0; i < 10; i++ {
		select {
		case ev := <-testQueue:
			require.NotEqual(t, ACCOUNTS, ev.EvType, "expected no ACCOUNTS event when accountsDB is nil")
		default:
			i = 10
		}
	}
}

func TestEventsProcessor_SaveBlock_DispatchesAllEventsAndPreparesIndexer(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			return acc, nil
		},
	}

	var receivedPrepared any
	var saveBlockCount int32
	idx := &indexerStub{
		isNilIndexer: false,
		saveBlockCalled: func(args *indexer.ArgsSaveBlockData) {
			atomic.AddInt32(&saveBlockCount, 1)
			receivedPrepared = args.Prepared
		},
	}

	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  idx,
		AccountsDB:               accountsDB,
	})
	require.NoError(t, err)

	contract := transaction.TransferContract{
		ToAddress: []byte("receiver"),
		Amount:    100,
	}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)
	header := &dataBlock.Block{
		Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100},
	}
	pool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{"txHash1": tx},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: pool,
	})

	seen := map[EventType]bool{}
	for i := 0; i < 8; i++ {
		select {
		case ev := <-testQueue:
			seen[ev.EvType] = true
		default:
			i = 8
		}
	}
	require.True(t, seen[BLOCKS], "BLOCKS event must fire with indexer active")
	require.True(t, seen[USER_TRANSACTIONS], "USER_TRANSACTIONS event must fire with indexer active")
	require.True(t, seen[TRANSACTIONS], "TRANSACTIONS event must fire with indexer active")
	require.True(t, seen[ACCOUNTS], "ACCOUNTS event must fire with indexer active")

	require.Equal(t, int32(1), atomic.LoadInt32(&saveBlockCount))
	prepared, ok := receivedPrepared.(*data.PreparedBlockData)
	require.True(t, ok, "indexer must receive *data.PreparedBlockData via args.Prepared")
	require.NotNil(t, prepared)
	require.NotEmpty(t, prepared.Txs)
	require.NotNil(t, prepared.Altered)
}

// TestEventsProcessor_SaveBlock_PreparesLogsResultsWhenIndexerActive guards the fix for a
// real data race: SaveTransactions (elasticProcessor, invoked asynchronously via
// indexer.SaveBlock -> dispatcher.Add) used to call ExtractDataFromLogs itself, mutating
// tx.HasLogs/tx.HasOperations on the same *data.Transaction pointers the websocket hub
// concurrently json.Marshals via dispatchTransactionEvents on a different goroutine
// (caught under -race). SaveBlock must now run ExtractDataFromLogs synchronously, before
// either consumer can touch prepared.Txs, and stash the result on prepared.LogsResults
// (plus PrepareLogsForDB's own result on prepared.LogsDB) for the elastic worker to reuse
// instead of recomputing.
func TestEventsProcessor_SaveBlock_PreparesLogsResultsWhenIndexerActive(t *testing.T) {
	saveAndRestoreEventQueue(t, true)

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) { return acc, nil },
	}

	var receivedPrepared any
	idx := &indexerStub{
		isNilIndexer:    false,
		saveBlockCalled: func(args *indexer.ArgsSaveBlockData) { receivedPrepared = args.Prepared },
	}

	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  idx,
		AccountsDB:               accountsDB,
	})
	require.NoError(t, err)

	contract := transaction.TransferContract{ToAddress: []byte("receiver"), Amount: 100}
	tx, err := createTransactionHandlerMock(&contract, transaction.TXContract_TransferContractType, []byte("sender"))
	require.NoError(t, err)

	logHandler := &transaction.Log{Address: []byte("contractaddr"), ContractID: 7}
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Txs:  map[string]nodeData.TransactionHandler{"txHash1": tx},
		Logs: []*nodeData.LogData{{LogHandler: logHandler, TxHash: "txHash1"}},
	}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	prepared, ok := receivedPrepared.(*data.PreparedBlockData)
	require.True(t, ok)
	require.NotNil(t, prepared.LogsResults, "LogsResults must be computed synchronously when both ws and indexer are active")
	require.NotNil(t, prepared.LogsDB, "LogsDB must be stashed for the elastic worker to reuse")
	require.Len(t, prepared.LogsDB, 1)
	require.NotEmpty(t, prepared.TxsMap)
	for _, dbTx := range prepared.TxsMap {
		assert.True(t, dbTx.HasLogs, "ExtractDataFromLogs must have already set HasLogs before args.Prepared was handed off")
	}
}

// Regression: SaveBlock must not drain TransactionsPool.Txs — the work item
// later calls ComputeSizeOfTxs and the elastic fallback re-preps.
func TestEventsProcessor_SaveBlock_DoesNotMutatePool(t *testing.T) {
	saveAndRestoreEventQueue(t, true)

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) { return acc, nil },
	}
	idx := &indexerStub{isNilIndexer: false, saveBlockCalled: func(_ *indexer.ArgsSaveBlockData) {}}

	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  idx,
		AccountsDB:               accountsDB,
	})
	require.NoError(t, err)

	tx1, err := createTransactionHandlerMock(
		&transaction.TransferContract{ToAddress: []byte("rcv1"), Amount: 100},
		transaction.TXContract_TransferContractType, []byte("snd1"))
	require.NoError(t, err)
	tx2, err := createTransactionHandlerMock(
		&transaction.TransferContract{ToAddress: []byte("rcv2"), Amount: 200},
		transaction.TXContract_TransferContractType, []byte("snd2"))
	require.NoError(t, err)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{
		Txs: map[string]nodeData.TransactionHandler{
			"h1": tx1,
			"h2": tx2,
		},
	}
	require.Len(t, pool.Txs, 2)

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	require.Len(t, pool.Txs, 2, "SaveBlock must not drain TransactionsPool.Txs — work item still reads it for ComputeSizeOfTxs")
}

// When ws is disabled and only the indexer is enabled, SaveBlock must NOT
// prep on the commit goroutine — the worker re-preps via the fallback in
// elasticProcessor.SaveTransactions, keeping commit-thread cost flat.
func TestEventsProcessor_SaveBlock_IndexerOnlySkipsCommitThreadPrep(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, false)

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			return acc, nil
		},
	}

	var receivedPrepared any
	idx := &indexerStub{
		isNilIndexer:    false,
		saveBlockCalled: func(args *indexer.ArgsSaveBlockData) { receivedPrepared = args.Prepared },
	}

	ep, err := NewEventsProcessor(ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		Indexer:                  idx,
		AccountsDB:               accountsDB,
	})
	require.NoError(t, err)

	tx, err := createTransactionHandlerMock(&transaction.TransferContract{ToAddress: []byte("rcv"), Amount: 1},
		transaction.TXContract_TransferContractType, []byte("snd"))
	require.NoError(t, err)
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"h": tx}}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})

	select {
	case ev := <-testQueue:
		t.Fatalf("expected no events when UseEventQueue=false, got %s", ev.EvType)
	default:
	}

	require.Nil(t, receivedPrepared, "Prepared must be nil when ws is disabled — worker re-preps on its own goroutine")
}

func TestEventsProcessor_SaveBlock_NoOpWhenNothingEnabled(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, false)

	ep := createTestEventsProcessor(t)
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"h": nil}}

	require.NotPanics(t, func() {
		ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})
	})
	require.Len(t, testQueue, 0)
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

func TestEventsProcessor_SaveAccounts_DispatchesWebsocketAndForwardsToIndexer(t *testing.T) {
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

	ev := <-testQueue
	require.Equal(t, ACCOUNTS, ev.EvType)
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

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
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

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
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

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
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

		result := getAllowanceWithPendingRewards(ep.kappsController, userAccount)
		require.Equal(t, int64(4000), result)
	})
}

func TestEventsProcessor_DispatchAccountEventsFromAlteredAccounts_IncludesAllAddresses(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	var loadCount int32
	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			atomic.AddInt32(&loadCount, 1)
			return acc, nil
		},
	}
	ep := createTestEventsProcessorWithAccountsDB(t, accountsDB)

	senderAddr := hex.EncodeToString([]byte("sender"))
	fromAddr := hex.EncodeToString([]byte("from_sc"))
	toAddr := hex.EncodeToString([]byte("receiver"))

	alteredAccounts := data.NewAlteredAccounts()
	alteredAccounts.Add(senderAddr, &data.AlteredAccount{IsSender: true, BalanceChange: true})
	alteredAccounts.Add(fromAddr, &data.AlteredAccount{IsSender: true, BalanceChange: true})
	alteredAccounts.Add(toAddr, &data.AlteredAccount{BalanceChange: true})

	ep.dispatchAccountEventsFromAlteredAccounts(100, alteredAccounts)

	require.Equal(t, int32(3), atomic.LoadInt32(&loadCount), "expected LoadAccount called for sender, from, and to")

	var accountEvent *Event
	for i := 0; i < 10; i++ {
		select {
		case ev := <-testQueue:
			if ev.EvType == ACCOUNTS {
				evCopy := ev
				accountEvent = &evCopy
			}
		default:
			i = 10
		}
	}
	require.NotNil(t, accountEvent, "expected an ACCOUNTS event to be dispatched")
	_, ok := accountEvent.Message.(map[string]*data.AccountInfo)
	require.True(t, ok)
}

func TestEventsProcessor_DispatchAccountEventsFromAlteredAccounts_SkipsZeroAddress(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	var loadCount int32
	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			atomic.AddInt32(&loadCount, 1)
			return acc, nil
		},
	}
	ep := createTestEventsProcessorWithAccountsDB(t, accountsDB)

	alteredAccounts := data.NewAlteredAccounts()
	alteredAccounts.Add(ZeroAddressDecoded, &data.AlteredAccount{IsSender: true})
	alteredAccounts.Add(hex.EncodeToString([]byte("validaddr1234567")), &data.AlteredAccount{IsSender: false})

	ep.dispatchAccountEventsFromAlteredAccounts(100, alteredAccounts)

	require.Equal(t, int32(1), atomic.LoadInt32(&loadCount), "zero address must not trigger a trie lookup")

	for {
		select {
		case ev := <-testQueue:
			if ev.EvType == ACCOUNTS {
				accountsMap, ok := ev.Message.(map[string]*data.AccountInfo)
				require.True(t, ok)
				for addr := range accountsMap {
					require.NotEqual(t, ZeroAddressDecoded, addr, "zero address must not appear in dispatched accounts")
				}
			}
		default:
			return
		}
	}
}

func TestEventsProcessor_DispatchAccountEventsFromAlteredAccounts_ErrAccNotFoundIgnoredQuietly(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) {
			return nil, common.ErrAccNotFound
		},
	}
	ep := createTestEventsProcessorWithAccountsDB(t, accountsDB)

	alteredAccounts := data.NewAlteredAccounts()
	alteredAccounts.Add(hex.EncodeToString([]byte("missingaddr12345")), &data.AlteredAccount{IsSender: true})

	ep.dispatchAccountEventsFromAlteredAccounts(100, alteredAccounts)

	select {
	case ev := <-testQueue:
		require.NotEqual(t, ACCOUNTS, ev.EvType, "missing account must not produce an ACCOUNTS event")
	default:
	}
}

func TestEventsProcessor_SaveBlock_TransferDispatchesSenderAndRecipient(t *testing.T) {
	testQueue := saveAndRestoreEventQueue(t, true)

	senderAddr := pad32("sender")
	recipientAddr := pad32("recipient")

	loaded := make(map[string]struct{})
	acc := createTestAccountStub()
	ep := createTestEventsProcessorWithAccountsDB(t, &mock.AccountsStub{
		GetExistingAccountCalled: func(addrBytes []byte) (state.AccountHandler, error) {
			loaded[hex.EncodeToString(addrBytes)] = struct{}{}
			return acc, nil
		},
	})

	tx := bareTransactionWithReceipts(senderAddr,
		makeReceipt(ptx.Transfer,
			senderAddr, recipientAddr,
			[]byte("100"), []byte("KLV"),
			[]byte(nil), []byte{0}, []byte(nil), []byte(nil),
		),
	)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}
	pool := &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"tx1": tx}}

	ep.SaveBlock(&indexer.ArgsSaveBlockData{Header: header, TransactionsPool: pool})
	drainTestQueue(testQueue)

	require.Contains(t, loaded, hex.EncodeToString(senderAddr), "sender must be dispatched")
	require.Contains(t, loaded, hex.EncodeToString(recipientAddr), "recipient must be dispatched")
}

func TestEventsProcessor_SaveBlock_UpdateValidatorDispatchesValidatorAddress(t *testing.T) {
	validatorAddr := pad32("validator")
	tx := bareTransactionWithReceipts(pad32("sender"),
		makeReceipt(ptx.UpdateValidator, validatorAddr),
	)
	runReceiptAddressTest(t, tx, validatorAddr)
}

func TestEventsProcessor_SaveBlock_UpdateAccountPermissionDispatchesTargetAddress(t *testing.T) {
	targetAddr := pad32("permission_target_ab")
	tx := bareTransactionWithReceipts(pad32("sender"),
		makeReceipt(ptx.UpdateAccountPermission, targetAddr),
	)
	runReceiptAddressTest(t, tx, targetAddr)
}

func TestEventsProcessor_SaveBlock_UpdateMetadataDispatchesOwnerAddress(t *testing.T) {
	ownerAddr := pad32("nft_owner_address_ab")
	tx := bareTransactionWithReceipts(pad32("sender"),
		makeReceipt(ptx.UpdateMetadata, ownerAddr, []byte("KDA-1"), []byte("1")),
	)
	runReceiptAddressTest(t, tx, ownerAddr)
}

func TestEventsProcessor_SaveBlock_SCTriggerDispatchesContractAddress(t *testing.T) {
	contractAddr := pad32("smart_contract_12345")
	tx := bareTransactionWithReceipts(pad32("sender"),
		makeReceipt(ptx.SCTrigger, []byte("0"), pad32("from_addr_12345678"), contractAddr),
	)
	runReceiptAddressTest(t, tx, contractAddr)
}

func TestEventsProcessor_SaveBlock_SetAccountNameDispatchesTargetAddress(t *testing.T) {
	targetAddr := pad32("account_owner_12345a")
	tx := bareTransactionWithReceipts(pad32("sender"),
		makeReceipt(ptx.SetAccountName, []byte("myname"), targetAddr),
	)
	runReceiptAddressTest(t, tx, targetAddr)
}

func TestEventsProcessor_WebsocketAndIndexerProduceSameAddressSet(t *testing.T) {
	senderBytes := pad32("sender_address_12345")
	validatorBytes := pad32("validator_address_ab")

	makeTx := func() *transaction.Transaction {
		return bareTransactionWithReceipts(senderBytes,
			makeReceipt(ptx.UpdateValidator, validatorBytes),
		)
	}
	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}

	// Step 1: collect the indexer's altered account set.
	proc := newTxDatabaseProcessor(
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		mock.NewPubkeyConverterMock(32),
		mock.NewPubkeyConverterMock(32),
		false,
	)
	_, _, ad, err := proc.prepareTransactionsForDatabase(header,
		&indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"tx1": makeTx()}},
	)
	require.NoError(t, err)

	indexerAddrs := make(map[string]struct{})
	for addr := range ad.Accounts.GetAll() {
		indexerAddrs[addr] = struct{}{}
	}

	// Step 2: collect addresses loaded by the websocket fallback path.
	testQueue := saveAndRestoreEventQueue(t, true)
	wsLoadedAddrs := make(map[string]struct{})
	acc := createTestAccountStub()
	ep := createTestEventsProcessorWithAccountsDB(t, &mock.AccountsStub{
		GetExistingAccountCalled: func(addrBytes []byte) (state.AccountHandler, error) {
			wsLoadedAddrs[hex.EncodeToString(addrBytes)] = struct{}{}
			return acc, nil
		},
	})

	ep.SaveBlock(&indexer.ArgsSaveBlockData{
		Header:           header,
		TransactionsPool: &indexer.Pool{Txs: map[string]nodeData.TransactionHandler{"tx1": makeTx()}},
	})
	drainTestQueue(testQueue)

	require.Equal(t, indexerAddrs, wsLoadedAddrs,
		"websocket fallback must query exactly the same accounts as the indexer path")
}

// benchSaveBlock runs ep.SaveBlock against a synthetic pool of transferTxs
// transfer txs. Use it to anchor commit-thread cost of the orchestrator
// (prepareTransactionsForDatabase + dispatch + indexer enqueue).
//
//	go test -bench=BenchmarkEventsProcessor_SaveBlock -benchmem ./indexer/
//
// Read each result as cost-per-block, not per-tx; b.N is the number of blocks
// processed in the run, each containing transferTxs synthetic transactions.
func benchSaveBlock(b *testing.B, transferTxs int, wsEnabled bool, indexerEnabled bool) {
	b.Helper()

	originalUseEventQueue := UseEventQueue
	originalEventQueue := EventQueue
	testQueue := make(chan Event, 1024)
	UseEventQueue = wsEnabled
	EventQueue = testQueue
	b.Cleanup(func() {
		UseEventQueue = originalUseEventQueue
		EventQueue = originalEventQueue
		// Close the local channel (not the global, which may have been swapped)
		// so the drain goroutine exits cleanly instead of leaking across benches.
		close(testQueue)
	})
	go func() {
		// drain the local channel so trySendEvent's non-blocking send doesn't
		// fall back to drop logging. Ranging over testQueue (local) instead of
		// the package-level EventQueue avoids racing with the cleanup-time write.
		for range testQueue {
		}
	}()

	acc := createTestAccountStub()
	accountsDB := &mock.AccountsStub{
		GetExistingAccountCalled: func(_ []byte) (state.AccountHandler, error) { return acc, nil },
	}

	args := ArgEventsProcessor{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: mock.NewPubkeyConverterMock(32),
		AccountsDB:               accountsDB,
	}
	if indexerEnabled {
		args.Indexer = &indexerStub{
			isNilIndexer:    false,
			saveBlockCalled: func(_ *indexer.ArgsSaveBlockData) {},
		}
	}
	ep, err := NewEventsProcessor(args)
	require.NoError(b, err)

	header := &dataBlock.Block{Header: &dataBlock.BlockHeader{Nonce: 1, Timestamp: 100}}

	// Pre-build the pool template once; cloning into a fresh map per iter is the
	// only mutation the orchestrator could perform on it (it shouldn't, post-fix —
	// but we want the bench to measure steady-state, not first-iteration warmup).
	contract := transaction.TransferContract{ToAddress: pad32("rcv"), Amount: 1}
	template := make(map[string]nodeData.TransactionHandler, transferTxs)
	for i := 0; i < transferTxs; i++ {
		tx, err := createTransactionHandlerMock(&contract,
			transaction.TXContract_TransferContractType, pad32("snd"))
		require.NoError(b, err)
		template[hex.EncodeToString([]byte{byte(i >> 8), byte(i)})] = tx
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool := make(map[string]nodeData.TransactionHandler, transferTxs)
		for k, v := range template {
			pool[k] = v
		}
		ep.SaveBlock(&indexer.ArgsSaveBlockData{
			Header:           header,
			TransactionsPool: &indexer.Pool{Txs: pool},
		})
	}
}

func BenchmarkEventsProcessor_SaveBlock_Empty_WSOnly(b *testing.B) { benchSaveBlock(b, 0, true, false) }
func BenchmarkEventsProcessor_SaveBlock_50tx_WSOnly(b *testing.B)  { benchSaveBlock(b, 50, true, false) }
func BenchmarkEventsProcessor_SaveBlock_500tx_WSOnly(b *testing.B) {
	benchSaveBlock(b, 500, true, false)
}
func BenchmarkEventsProcessor_SaveBlock_50tx_IndexerOnly(b *testing.B) {
	benchSaveBlock(b, 50, false, true)
}
func BenchmarkEventsProcessor_SaveBlock_500tx_IndexerOnly(b *testing.B) {
	benchSaveBlock(b, 500, false, true)
}
func BenchmarkEventsProcessor_SaveBlock_50tx_Both(b *testing.B)  { benchSaveBlock(b, 50, true, true) }
func BenchmarkEventsProcessor_SaveBlock_500tx_Both(b *testing.B) { benchSaveBlock(b, 500, true, true) }
