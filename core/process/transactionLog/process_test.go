package transactionLog_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"

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
	err := txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs)
	require.Equal(t, retErr, err)
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
	err := txLogProcessor.SaveLog([]byte("txhash"), []byte{}, &transaction.SmartContract{}, 0, logs)
	require.Equal(t, retErr, err)
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

	require.Equal(t, buffExpected, buffActual)
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
