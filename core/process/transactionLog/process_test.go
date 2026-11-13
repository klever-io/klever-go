package transactionLog_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTxLogProcessor_NilParameters(t *testing.T) {
	_, nilMarshalizer := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{},
	})

	require.Equal(t, process.ErrNilMarshalizer, nilMarshalizer)

	_, nilStorer := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})

	require.Equal(t, process.ErrNilStore, nilStorer)

	_, nilError := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:      &mock.StorerStub{},
		Marshalizer: &mock.MarshalizerMock{},
	})

	require.Nil(t, nilError)
}

func TestTxLogProcessor_SaveLogsNilTxHash(t *testing.T) {
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:      &mock.StorerStub{},
		Marshalizer: &mock.MarshalizerMock{},
	})

	err := txLogProcessor.SaveLog(nil, nil, nil, 0, make([]*vmcommon.LogEntry, 0))
	require.Equal(t, process.ErrNilTxHash, err)
}

func TestTxLogProcessor_SaveLogsEmptyLogsReturnsNil(t *testing.T) {
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:      &mock.StorerStub{},
		Marshalizer: &mock.MarshalizerMock{},
	})

	err := txLogProcessor.SaveLog([]byte("txhash"), []byte{}, nil, 0, make([]*vmcommon.LogEntry, 0))
	require.Nil(t, err)
}

func TestTxLogProcessor_Clean(t *testing.T) {
	t.Parallel()

	txLogsProc, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer:      &mock.StorerStub{},
		Marshalizer: &mock.MarshalizerMock{},
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("first log")},
	}
	err := txLogsProc.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs)
	require.Nil(t, err)
	require.Len(t, txLogsProc.GetAllCurrentLogs(), 1)

	txLogsProc.Clean()
	require.Len(t, txLogsProc.GetAllCurrentLogs(), 0)
}

func TestTxLogProcessor_SaveLogsMarshalErr(t *testing.T) {
	retErr := errors.New("marshal err")
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{},
		Marshalizer: &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) (bytes []byte, err error) {
				return nil, retErr
			},
		},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("first log")},
	}
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs))
	require.Equal(t, retErr, txLogProcessor.SaveLogsToStorage())
}

func TestTxLogProcessor_SaveLogsStoreErr(t *testing.T) {
	retErr := errors.New("put err")
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				return retErr
			},
		},
		Marshalizer: &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) (bytes []byte, err error) {
				return nil, nil
			},
		},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("first log")},
	}
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs))
	require.Equal(t, retErr, txLogProcessor.SaveLogsToStorage())
}

func TestTxLogProcessor_SaveLogsCallsPutWithMarshalBuff(t *testing.T) {
	buffExpected := []byte("marshaled log")
	buffActual := []byte("currently wrong value")
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				buffActual = data
				return nil
			},
		},
		Marshalizer: &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) (bytes []byte, err error) {
				return buffExpected, nil
			},
		},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("first log")},
	}
	_ = txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs)
	require.Nil(t, txLogProcessor.SaveLogsToStorage())

	require.Equal(t, buffExpected, buffActual)
}

// SaveLog runs during VM execution, before the checks that can still reject the block.
// It must leave the storer alone until CommitBlock flushes (KLR-69).
func TestTxLogProcessor_SaveLogDoesNotReachStorageBeforeFlush(t *testing.T) {
	putCalls := 0
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				putCalls++
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("first log")},
	}
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs))
	require.Equal(t, 0, putCalls)
	require.Len(t, txLogProcessor.GetAllCurrentLogs(), 1)

	require.Nil(t, txLogProcessor.SaveLogsToStorage())
	require.Equal(t, 1, putCalls)
}

// A rejected block is followed by CreateBlockStarted -> Clean, with no flush in
// between, so its logs must never be persisted (KLR-69).
func TestTxLogProcessor_CleanBeforeFlushDiscardsLogs(t *testing.T) {
	putCalls := 0
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				putCalls++
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{
		{Address: []byte("rejected log")},
	}
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs))

	txLogProcessor.Clean()

	require.Nil(t, txLogProcessor.SaveLogsToStorage())
	require.Equal(t, 0, putCalls)
}

// One failed Put must not drop the rest of the block's logs.
func TestTxLogProcessor_SaveLogsToStorageContinuesAfterError(t *testing.T) {
	retErr := errors.New("put error")
	stored := make(map[string]bool)
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				if string(key) == "txhash1" {
					return retErr
				}
				stored[string(key)] = true
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})

	logs := []*vmcommon.LogEntry{{Address: []byte("log")}}
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash1"), []byte{}, &transaction.SmartContract{}, 0, logs))
	require.Nil(t, txLogProcessor.SaveLog([]byte("txhash2"), []byte{}, &transaction.SmartContract{}, 0, logs))

	require.Equal(t, retErr, txLogProcessor.SaveLogsToStorage())
	require.True(t, stored["txhash2"])
}

func TestTxLogProcessor_GetLogErrNotFound(t *testing.T) {
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return nil, errors.New("storer error")
			},
		},
		Marshalizer:          &mock.MarshalizerStub{},
		SaveInStorageEnabled: true,
	})

	_, err := txLogProcessor.GetLog([]byte("texhash"))

	require.Equal(t, process.ErrLogNotFound, err)
}

func TestTxLogProcessor_GetLogUnmarshalErr(t *testing.T) {
	retErr := errors.New("marshal error")
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			GetCalled: func(key []byte) (bytes []byte, err error) {
				return make([]byte, 0), nil
			},
		},
		Marshalizer: &mock.MarshalizerStub{
			UnmarshalCalled: func(obj interface{}, buff []byte) error {
				return retErr
			},
		},
		SaveInStorageEnabled: true,
	})

	_, err := txLogProcessor.GetLog([]byte("texhash"))

	require.Equal(t, retErr, err)
}

func TestTxLogProcessor_GetLogFromCache(t *testing.T) {
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				return nil
			},
		},
		Marshalizer: &mock.MarshalizerMock{},
	})
	txLogProcessor.EnableLogToBeSavedInCache()
	_ = txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, []*vmcommon.LogEntry{{}})

	logData, found := txLogProcessor.GetLogFromCache([]byte("txhash"))
	require.True(t, found)
	require.Equal(t, "txhash", logData.TxHash)
}

func TestTxLogProcessor_GetLogFromCacheNotInCacheShouldReturnFromStorage(t *testing.T) {
	t.Parallel()

	logs := []*vmcommon.LogEntry{{
		Address: []byte("my-addr"),
	}}

	txLog := &transaction.Log{
		Address: []byte("add"),
	}

	marshalizer := &mock.MarshalizerMock{}
	txLogProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				logsBytes, _ := marshalizer.Marshal(txLog)
				return logsBytes, nil
			},
		},
		Marshalizer: marshalizer,
	})
	_ = txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs)

	_, found := txLogProcessor.GetLogFromCache([]byte("txhash"))
	require.True(t, found)
}

// KLC-2891 helpers: a processor whose storer records the keys flushed to it.
func newRemoveLogProcessor(t *testing.T) (process.TransactionLogProcessor, *[]string) {
	tlp, putKeys, _ := newRemoveLogProcessorWithStored(t, nil)
	return tlp, putKeys
}

// stored seeds the storer, as a log left behind by a rolled-back block would be.
func newRemoveLogProcessorWithStored(t *testing.T, stored map[string][]byte) (process.TransactionLogProcessor, *[]string, *[]string) {
	putKeys := make([]string, 0)
	removedKeys := make([]string, 0)
	tlp, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				putKeys = append(putKeys, string(key))
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				if buff, ok := stored[string(key)]; ok {
					return buff, nil
				}
				return nil, errors.New("not in storage")
			},
			RemoveCalled: func(key []byte) error {
				removedKeys = append(removedKeys, string(key))
				delete(stored, string(key))
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})
	require.Nil(t, err)

	return tlp, &putKeys, &removedKeys
}

func saveLogFor(t *testing.T, tlp process.TransactionLogProcessor, txHash string, address string) {
	logs := []*vmcommon.LogEntry{{Identifier: []byte(address), Address: []byte(address)}}
	require.Nil(t, tlp.SaveLog([]byte(txHash), []byte{}, &transaction.SmartContract{Address: []byte(address)}, 0, logs))
}

func currentLogHashes(tlp process.TransactionLogProcessor) []string {
	hashes := make([]string, 0)
	for _, logData := range tlp.GetAllCurrentLogs() {
		hashes = append(hashes, logData.TxHash)
	}

	return hashes
}

func requireLogAddress(t *testing.T, tlp process.TransactionLogProcessor, txHash string, address string) {
	txLog, err := tlp.GetLog([]byte(txHash))
	require.Nil(t, err)
	require.Equal(t, []byte(address), txLog.GetAddress())
}

// KLC-2891: removing a middle log keeps the indices of the others, and the flush skips it.
func TestTxLogProcessor_RemoveLogMiddleEntry(t *testing.T) {
	t.Parallel()

	tlp, putKeys := newRemoveLogProcessor(t)
	saveLogFor(t, tlp, "tx1", "addr1")
	saveLogFor(t, tlp, "tx2", "addr2")
	saveLogFor(t, tlp, "tx3", "addr3")

	tlp.RemoveLog([]byte("tx2"))

	require.Equal(t, []string{"tx1", "tx3"}, currentLogHashes(tlp))
	requireLogAddress(t, tlp, "tx1", "addr1")
	requireLogAddress(t, tlp, "tx3", "addr3")
	_, err := tlp.GetLog([]byte("tx2"))
	require.Equal(t, process.ErrLogNotFound, err)

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx1", "tx3"}, *putKeys)
}

func TestTxLogProcessor_RemoveLogMissingHashIsNoOp(t *testing.T) {
	t.Parallel()

	tlp, _ := newRemoveLogProcessor(t)
	saveLogFor(t, tlp, "tx1", "addr1")
	saveLogFor(t, tlp, "tx2", "addr2")

	tlp.RemoveLog([]byte("missing"))
	tlp.RemoveLog(nil)

	require.Equal(t, []string{"tx1", "tx2"}, currentLogHashes(tlp))
	requireLogAddress(t, tlp, "tx1", "addr1")
	requireLogAddress(t, tlp, "tx2", "addr2")
}

func TestTxLogProcessor_RemoveLogThenSaveAgain(t *testing.T) {
	t.Parallel()

	tlp, putKeys := newRemoveLogProcessor(t)
	saveLogFor(t, tlp, "tx1", "addr1")
	saveLogFor(t, tlp, "tx2", "addr2")

	tlp.RemoveLog([]byte("tx1"))
	saveLogFor(t, tlp, "tx1", "addr1-new")

	require.Equal(t, []string{"tx2", "tx1"}, currentLogHashes(tlp))
	requireLogAddress(t, tlp, "tx1", "addr1-new")
	requireLogAddress(t, tlp, "tx2", "addr2")

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx2", "tx1"}, *putKeys)
}

// SaveLog appends, so one tx can hold several entries; RemoveLog drops them all.
func TestTxLogProcessor_RemoveLogDropsEveryEntryOfTheTx(t *testing.T) {
	t.Parallel()

	tlp, putKeys := newRemoveLogProcessor(t)
	saveLogFor(t, tlp, "tx1", "addr1")
	saveLogFor(t, tlp, "tx2", "addr2")
	saveLogFor(t, tlp, "tx1", "addr1-again")

	tlp.RemoveLog([]byte("tx1"))

	require.Equal(t, []string{"tx2"}, currentLogHashes(tlp))
	requireLogAddress(t, tlp, "tx2", "addr2")

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx2"}, *putKeys)
}

// KLC-2891: a replayed tx discarded after a rollback must not surface the log its old block stored.
func TestTxLogProcessor_RemoveLogHidesAndDeletesStoredLogAtCommit(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	oldLog, err := marshalizer.Marshal(&transaction.Log{Address: []byte("old")})
	require.Nil(t, err)
	stored := map[string][]byte{"tx1": oldLog}

	tlp, putKeys, removedKeys := newRemoveLogProcessorWithStored(t, stored)
	requireLogAddress(t, tlp, "tx1", "old")

	saveLogFor(t, tlp, "tx1", "replayed")
	tlp.RemoveLog([]byte("tx1"))

	_, err = tlp.GetLog([]byte("tx1"))
	require.Equal(t, process.ErrLogNotFound, err)
	require.Empty(t, *removedKeys, "nothing is deleted before commit")

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx1"}, *removedKeys)
	require.Empty(t, *putKeys)
	require.NotContains(t, stored, "tx1")
}

func TestTxLogProcessor_CleanDropsStagedRemovals(t *testing.T) {
	t.Parallel()

	tlp, _, removedKeys := newRemoveLogProcessorWithStored(t, nil)
	tlp.RemoveLog([]byte("tx1"))
	tlp.Clean()

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Empty(t, *removedKeys)
}

func TestTxLogProcessor_SaveLogsToStorageReturnsRemoveError(t *testing.T) {
	t.Parallel()

	errRemove := errors.New("remove failed")
	putKeys := make([]string, 0)
	tlp, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				putKeys = append(putKeys, string(key))
				return nil
			},
			RemoveCalled: func(key []byte) error {
				return errRemove
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})
	require.Nil(t, err)

	tlp.RemoveLog([]byte("tx1"))
	saveLogFor(t, tlp, "tx2", "addr2")

	require.Equal(t, errRemove, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx2"}, putKeys, "a failed remove does not stop the writes")
}

// One processor shared by concurrent block processing, removals, lookups and flushes; run with -race.
func TestTxLogProcessor_ConcurrentRemoveLookupAndFlush(t *testing.T) {
	t.Parallel()

	var mutStored sync.Mutex
	stored := make(map[string][]byte)
	tlp, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				mutStored.Lock()
				stored[string(key)] = data
				mutStored.Unlock()
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				mutStored.Lock()
				defer mutStored.Unlock()
				if buff, ok := stored[string(key)]; ok {
					return buff, nil
				}
				return nil, errors.New("not in storage")
			},
			RemoveCalled: func(key []byte) error {
				mutStored.Lock()
				delete(stored, string(key))
				mutStored.Unlock()
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})
	require.Nil(t, err)

	tlp.RemoveLog([]byte("discarded"))

	const workers, rounds = 8, 50
	start := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < rounds; i++ {
				txHash := []byte(fmt.Sprintf("tx-%d-%d", w, i))
				entries := []*vmcommon.LogEntry{{Identifier: txHash, Address: txHash}}
				assert.Nil(t, tlp.SaveLog(txHash, []byte{}, &transaction.SmartContract{Address: txHash}, 0, entries))
				if i%2 == 0 {
					tlp.RemoveLog(txHash)
				}
				_, _ = tlp.GetLog(txHash)
				_, _ = tlp.GetLog([]byte("discarded"))
				_ = tlp.GetAllCurrentLogs()
				assert.Nil(t, tlp.SaveLogsToStorage())
			}
		}(w)
	}
	close(start)
	wg.Wait()
	// a flush may snapshot another worker's log before its RemoveLog; the next flush settles it
	require.Nil(t, tlp.SaveLogsToStorage())

	_, err = tlp.GetLog([]byte("discarded"))
	require.Equal(t, process.ErrLogNotFound, err)
	for w := 0; w < workers; w++ {
		for i := 0; i < rounds; i++ {
			txHash := []byte(fmt.Sprintf("tx-%d-%d", w, i))
			_, err = tlp.GetLog(txHash)
			if i%2 == 0 {
				require.Equal(t, process.ErrLogNotFound, err, string(txHash))
				mutStored.Lock()
				_, inStorage := stored[string(txHash)]
				mutStored.Unlock()
				require.False(t, inStorage, string(txHash))
			} else {
				require.Nil(t, err, string(txHash))
			}
		}
	}
}

type multiEpochStorerStub struct {
	*mock.StorerStub
	removeFromAllCalled func(key []byte) error
}

func (s *multiEpochStorerStub) RemoveFromAllActivePersisters(key []byte) error {
	return s.removeFromAllCalled(key)
}

// An epoch-pruned storer must drop the log from every epoch, not only the newest.
func TestTxLogProcessor_SaveLogsToStorageRemovesFromAllEpochs(t *testing.T) {
	t.Parallel()

	removedFromAll := make([]string, 0)
	tlp, err := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Storer: &multiEpochStorerStub{
			StorerStub: &mock.StorerStub{
				RemoveCalled: func(key []byte) error {
					require.Fail(t, "single-epoch Remove must not be used")
					return nil
				},
			},
			removeFromAllCalled: func(key []byte) error {
				removedFromAll = append(removedFromAll, string(key))
				return nil
			},
		},
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: true,
	})
	require.Nil(t, err)

	tlp.RemoveLog([]byte("tx1"))

	require.Nil(t, tlp.SaveLogsToStorage())
	require.Equal(t, []string{"tx1"}, removedFromAll)
}
