package shared_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closerStub struct {
	closed chan struct{}
}

func (c *closerStub) Close() error {
	close(c.closed)
	return errors.New("close failed") // SafeGo must swallow this; teardown is best-effort
}

func wait(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}

func TestSafeGo_RunsFnOnItsOwnGoroutine(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	done := make(chan struct{})
	returned := make(chan struct{})
	conn := &closerStub{closed: make(chan struct{})}
	log := &mock.LoggerStub{LogCalled: func(logger.LogLevel, string, ...interface{}) {
		t.Error("nothing should be logged on the happy path")
	}}

	// fn parks on a gate the test opens only after SafeGo has returned. Run inline instead
	// of detached, SafeGo would block on that gate and never return, so the first wait
	// below fails by name rather than the earlier version's "close(done)" passing either way.
	go func() {
		shared.SafeGo(log, "happy", conn, func() {
			<-gate
			close(done)
		})
		close(returned)
	}()

	wait(t, returned, "SafeGo must return before fn runs; fn is not running on its own goroutine")
	close(gate)
	wait(t, done, "fn never ran")
	select {
	case <-conn.closed:
		t.Fatal("conn must stay open when fn returns normally")
	default:
	}
}

func TestSafeGo_RecoversPanicLogsAndClosesConn(t *testing.T) {
	t.Parallel()

	logged := make(chan []interface{}, 1)
	log := &mock.LoggerStub{LogCalled: func(level logger.LogLevel, message string, args ...interface{}) {
		assert.Equal(t, logger.LogError, level)
		assert.Equal(t, "panic in detached websocket goroutine", message)
		logged <- args
	}}
	conn := &closerStub{closed: make(chan struct{})}
	deferRan := make(chan struct{})

	shared.SafeGo(log, "boom-routine", conn, func() {
		defer close(deferRan) // stands in for a limiter release; must run during the unwind
		panic("boom")
	})

	wait(t, deferRan, "defers inside fn must run before the recover")
	wait(t, conn.closed, "conn must be closed after a panic")
	select {
	case args := <-logged:
		require.Contains(t, args, "boom-routine")
		// Quoted, not raw: the panic value is rendered through QuoteForLog (KLC-2596).
		require.Contains(t, args, `"boom"`)
		require.Contains(t, args, "stack", "a recovered panic without its stack is not actionable")
	case <-time.After(2 * time.Second):
		t.Fatal("panic must be logged")
	}
}

// TestSafeGo_PanicValueCannotForgeALogLine covers the injection vector a panic value opens:
// a panic message routinely quotes what it was handed, so peer input reaches the log
// through it, and a raw newline there would close the entry and start an attacker-chosen
// one. QuoteForLog escapes it instead (KLC-2596).
func TestSafeGo_PanicValueCannotForgeALogLine(t *testing.T) {
	t.Parallel()

	logged := make(chan []interface{}, 1)
	log := &mock.LoggerStub{LogCalled: func(_ logger.LogLevel, _ string, args ...interface{}) {
		logged <- args
	}}
	conn := &closerStub{closed: make(chan struct{})}

	shared.SafeGo(log, "forge-routine", conn, func() {
		panic("bad key \"x\"\nERROR forged entry")
	})

	wait(t, conn.closed, "conn must be closed after a panic")
	select {
	case args := <-logged:
		var rendered string
		for _, arg := range args {
			if s, ok := arg.(string); ok && strings.Contains(s, "forged entry") {
				rendered = s
			}
		}
		require.NotEmpty(t, rendered, "the panic value must still reach the log")
		assert.NotContains(t, rendered, "\n", "a raw newline would forge a second log entry")
		assert.Contains(t, rendered, `\n`, "the newline must survive as an escape, not be dropped")
	case <-time.After(2 * time.Second):
		t.Fatal("panic must be logged")
	}
}
