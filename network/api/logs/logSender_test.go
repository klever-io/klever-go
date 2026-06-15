package logs_test

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/stretchr/testify/assert"
)

func removeWriterFromLogSubsystem(w io.Writer) {
	_ = logger.RemoveLogObserver(w)
}

func createMockLogSender() (*logs.LogSender, *mock.WsConnStub, io.Writer) {
	conn := &mock.WsConnStub{}
	conn.SetCloseHandler(func() error {
		return nil
	})
	conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
		profile := logger.Profile{LogLevelPatterns: "*:INFO"}
		profileJson, _ := profile.Marshal()
		return websocket.TextMessage, profileJson, nil
	})

	ls, _ := logs.NewLogSender(
		&mock.MarshalizerStub{},
		conn,
		&mock.LoggerStub{},
		false,
	)
	removeWriterFromLogSubsystem(ls.Writer())
	ls.SetWriter(logs.NewLogWriter())

	lsender := &logs.LogSender{}
	lsender.Set(ls)
	return lsender, conn, ls.Writer()
}

//------- NewLogSender

func TestNewLogSender_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	ls, err := logs.NewLogSender(nil, &mock.WsConnStub{}, &mock.LoggerStub{}, false)

	assert.Nil(t, ls)
	assert.Equal(t, logs.ErrNilMarshalizer, err)
}

func TestNewLogSender_NilConnectionShouldErr(t *testing.T) {
	t.Parallel()

	ls, err := logs.NewLogSender(&mock.MarshalizerStub{}, nil, &mock.LoggerStub{}, false)

	assert.Nil(t, ls)
	assert.Equal(t, logs.ErrNilWsConn, err)
}

func TestNewLogSender_NilLoggerShouldErr(t *testing.T) {
	t.Parallel()

	ls, err := logs.NewLogSender(&mock.MarshalizerStub{}, &mock.WsConnStub{}, nil, false)

	assert.Nil(t, ls)
	assert.Equal(t, logs.ErrNilLogger, err)
}

func TestNewLogSender_ShouldWork(t *testing.T) {
	t.Parallel()

	ls, err := logs.NewLogSender(&mock.MarshalizerStub{}, &mock.WsConnStub{}, &mock.LoggerStub{}, false)

	assert.NotNil(t, ls)
	assert.Nil(t, err)
	assert.NotNil(t, ls.Writer())

	removeWriterFromLogSubsystem(ls.Writer())
}

//------- StartSendingBlocking

func TestLogSender_StartSendingBlockingConnReadMessageErrShouldCloseConn(t *testing.T) {
	t.Parallel()

	closeCalled := false
	conn := &mock.WsConnStub{}
	conn.SetCloseHandler(func() error {
		closeCalled = true
		return nil
	})
	conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
		return websocket.TextMessage, nil, errors.New("")
	})
	ls, _ := logs.NewLogSender(
		&mock.MarshalizerStub{},
		conn,
		&mock.LoggerStub{},
		false,
	)
	removeWriterFromLogSubsystem(ls.Writer())

	ls.StartSendingBlocking()

	assert.True(t, closeCalled)
}

func TestLogSender_StartSendingBlockingWrongPatternShouldCloseConn(t *testing.T) {
	t.Parallel()

	closeCalled := false
	conn := &mock.WsConnStub{}
	conn.SetCloseHandler(func() error {
		closeCalled = true
		return nil
	})
	conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
		return websocket.TextMessage, []byte("wrong log pattern"), nil
	})
	ls, _ := logs.NewLogSender(
		&mock.MarshalizerStub{},
		conn,
		&mock.LoggerStub{},
		false,
	)
	removeWriterFromLogSubsystem(ls.Writer())

	ls.StartSendingBlocking()

	assert.True(t, closeCalled)
}

func TestLogSender_StartSendingBlockingSendsMessage(t *testing.T) {
	t.Parallel()

	ls, conn, writer := createMockLogSender()
	data := []byte("random data")
	var retrievedData []byte
	conn.SetWriteMessageHandler(func(messageType int, data []byte) error {
		retrievedData = data
		return nil
	})

	go func() {
		//watchdog function
		time.Sleep(time.Millisecond * 10)

		_ = ls.Writer().Close()
	}()

	_, err := writer.Write(data)
	ls.StartSendingBlocking()

	assert.Nil(t, err)
	assert.Equal(t, data, retrievedData)
}

func TestLogSender_StartSendingBlockingSendsMessageAndStopsWhenReadClose(t *testing.T) {
	t.Parallel()

	ls, conn, writer := createMockLogSender()
	data := []byte("random data")
	var retrievedData []byte
	conn.SetWriteMessageHandler(func(messageType int, data []byte) error {
		retrievedData = data
		return nil
	})

	go func() {
		//watchdog function
		time.Sleep(time.Millisecond * 10)

		conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
			return websocket.CloseMessage, []byte(""), nil
		})
	}()

	_, err := writer.Write(data)
	ls.StartSendingBlocking()

	assert.Nil(t, err)
	assert.Equal(t, data, retrievedData)
}

// TestLogSender_UnauthenticatedClientProfileIsIgnored is the anti-PoC regression for
// GHSA-9v8p-frvj-2pcm / KLC-2438: on an unauthenticated /log (allowProfileApply=false) a profile
// sent by a client as the first frame must NOT mutate the process-global logger.
// Not parallel: it asserts on global logger state.
func TestLogSender_UnauthenticatedClientProfileIsIgnored(t *testing.T) {
	err := logger.SetLogLevel("*:INFO")
	assert.Nil(t, err)
	baseline := logger.GetLogLevelPattern()
	assert.NotEqual(t, "*:NONE", baseline)
	t.Cleanup(func() { _ = logger.SetLogLevel("*:INFO") })

	mutePayload := []byte(`{"LogLevelPatterns":"*:NONE","WithCorrelation":false,"WithLoggerName":false}`)

	readCount := 0
	conn := &mock.WsConnStub{}
	conn.SetCloseHandler(func() error { return nil })
	conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
		readCount++
		if readCount == 1 {
			// Attacker handshake frame: a global mute profile.
			return websocket.TextMessage, mutePayload, nil
		}
		// End the stream: monitorConnection returns on CloseMessage and closes the
		// writer, which unblocks doSendContinuously so StartSendingBlocking returns.
		return websocket.CloseMessage, nil, nil
	})

	// allowProfileApply=false => unauthenticated connection.
	ls, _ := logs.NewLogSender(&mock.MarshalizerStub{}, conn, &mock.LoggerStub{}, false)

	ls.StartSendingBlocking()

	assert.Equal(t, baseline, logger.GetLogLevelPattern(), "unauthenticated client profile must not change the global log level")
	assert.NotEqual(t, "*:NONE", logger.GetLogLevelPattern())
}

// TestLogSender_AuthenticatedClientProfileIsAppliedAndReverted verifies that on a secured /log
// (allowProfileApply=true) an authenticated operator CAN apply a logger profile while connected
// — the useful debugging capability — and that it is reverted to the prior profile on disconnect.
// Not parallel: it asserts on global logger state.
func TestLogSender_AuthenticatedClientProfileIsAppliedAndReverted(t *testing.T) {
	err := logger.SetLogLevel("*:INFO")
	assert.Nil(t, err)
	baseline := logger.GetLogLevelPattern()
	assert.NotEqual(t, "*:NONE", baseline)
	t.Cleanup(func() { _ = logger.SetLogLevel("*:INFO") })

	mutePayload := []byte(`{"LogLevelPatterns":"*:NONE","WithCorrelation":false,"WithLoggerName":false}`)

	var midLevel string
	readCount := 0
	conn := &mock.WsConnStub{}
	conn.SetCloseHandler(func() error { return nil })
	conn.SetReadMessageHandler(func() (messageType int, p []byte, err error) {
		readCount++
		if readCount == 1 {
			return websocket.TextMessage, mutePayload, nil
		}
		// This read happens in monitorConnection, after waitForProfile applied the profile:
		// capture the live (mid-connection) global level before ending the stream.
		if midLevel == "" {
			midLevel = logger.GetLogLevelPattern()
		}
		return websocket.CloseMessage, nil, nil
	})

	// allowProfileApply=true => authenticated connection.
	ls, _ := logs.NewLogSender(&mock.MarshalizerStub{}, conn, &mock.LoggerStub{}, true)

	ls.StartSendingBlocking()

	assert.Equal(t, "*:NONE", midLevel, "authenticated client profile should be applied while connected")
	assert.Equal(t, baseline, logger.GetLogLevelPattern(), "profile should be reverted to baseline on disconnect")
}

func TestLogSender_StartSendingBlockingConnWriteFailsShouldStop(t *testing.T) {
	t.Parallel()

	ls, conn, writer := createMockLogSender()
	data := []byte("random data")
	closeCalled := false
	conn.SetWriteMessageHandler(func(messageType int, data []byte) error {
		return errors.New("")
	})
	conn.SetCloseHandler(func() error {
		closeCalled = true
		return nil
	})

	_, _ = writer.Write(data)
	ls.StartSendingBlocking()

	assert.True(t, closeCalled)
}
