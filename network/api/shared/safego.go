package shared

import (
	"fmt"
	"io"
	"runtime/debug"

	logger "github.com/klever-io/klever-go-logger"
)

// SafeRun runs fn on the calling goroutine under a recover. A goroutine detached from the
// request is outside gin.Recovery(), so an unrecovered panic in it takes the node down instead
// of the one connection; SafeRun logs it and closes conn so the connection's own teardown still
// runs. Defers inside fn — a limiter release, say — run during the unwind, before the recover.
// It returns only once that recovery has finished, so a caller that needs to know the goroutine
// is entirely done — a join before releasing a connection slot — wraps SafeRun rather than
// deferring inside fn, where the accounting would run before the recovery.
func SafeRun(log logger.Logger, name string, conn io.Closer, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			// Quote the panic value: it can echo peer input and forge a log line (KLC-2596).
			// The stack is this goroutine's trace, logged in full; Close ends the connection.
			log.Error("panic in detached websocket goroutine",
				"goroutine", name,
				"recover", QuoteForLog(fmt.Sprintf("%v", r)),
				"stack", string(debug.Stack()),
			)
			_ = conn.Close()
		}
	}()

	fn()
}

// SafeGo runs fn on its own goroutine under SafeRun.
func SafeGo(log logger.Logger, name string, conn io.Closer, fn func()) {
	go SafeRun(log, name, conn, fn)
}
