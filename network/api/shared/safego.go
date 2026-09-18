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
			// The stack is what makes a recovered panic actionable: without it the log says
			// only that some goroutine died, and the frame that panicked — the whole reason
			// to recover rather than crash — is gone (KLC-2596).
			//
			// The panic value gets QuoteForLog because a panic message routinely quotes what
			// it was handed, so peer input reaches the log through it exactly as it does
			// through an error — and a value of "\nERROR ..." would otherwise forge a log
			// line. The stack needs no such treatment: a goroutine dump carries function
			// names and hex arguments, never peer bytes. It is also left unbounded, unlike
			// the hub's in-package barrier: a panic here closes the connection, so repeating
			// it costs a full reconnect against the per-IP and global connection caps rather
			// than one more frame on an open socket.
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
