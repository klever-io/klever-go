package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

type client struct {
	aliveLock sync.Mutex
	alive     bool
	hub       *SocketHub
	conn      *ws.Conn
	out       chan interface{}
	ctx       context.Context
	cancel    context.CancelFunc
	sem       chan struct{}
}

// NewClient create a new client, add into hub and start ping and watch
func NewClient(conn *ws.Conn, hub *SocketHub) *client {
	client := &client{conn: conn, hub: hub, out: make(chan interface{}, outChannelSize), alive: true, sem: make(chan struct{}, maxWorkers)}
	client.ctx, client.cancel = context.WithCancel(context.Background())
	go client.loopIn()
	go client.loopOut()

	return client
}

func (c *client) IsAlive() bool {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()
	return c.alive
}

// Done returns a channel that is closed once the client connection is torn down.
// It lets the connection owner (processSubscription) block for the connection
// lifetime so a per-connection resource slot can be released exactly on close.
func (c *client) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *client) send(msg interface{}) {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()
	if !c.alive {
		return
	}
	select {
	case c.out <- msg:
	default:
		c.hub.logSendDrop("ws.send", "client output buffer full, dropping message")
	}
}

// Close tears the connection down through the client, so its loopIn/loopOut goroutines
// see alive=false and stop. Always prefer it over closing the raw *ws.Conn: the extra
// close and the read-deadline call it strands both log an error nobody needs to see.
// Idempotent.
func (c *client) Close() {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()
	if c.alive {
		if err := c.conn.Close(); err != nil {
			// Usually local, but it is still one line per connection in the worst case and
			// it is the same teardown class as loopOut's write failures.
			c.hub.logWriteFailure("ws.close", err)
		}
		c.alive = false
		c.cancel()
		close(c.out)
		for len(c.out) > 0 {
			<-c.out
		}
	}
}

// unexpectedClose reports whether a read error ended the connection in a way that says
// anything about this node. The peer chooses the code it closes with, so no close frame is
// evidence of a problem here — listing "ordinary" codes and warning on the rest only moves
// the lever, because the peer just picks a code that is not on the list. 1006 is included:
// gorilla synthesises it when the connection drops without a close frame, which is what an
// abandoned socket looks like. What remains is a read that failed without any close frame
// at all, and even that is rate-limited at the call site.
func unexpectedClose(err error) bool {
	var closeErr *ws.CloseError
	if errors.As(err, &closeErr) {
		return false
	}

	// Only our own Close() produces this — a peer cannot make the server's socket report
	// itself closed — so it is teardown, not a read failure. Counting it would bill the
	// read budget for every shutdown, every write failure and every rejected insert, which
	// is exactly the cross-contamination the per-source budgets exist to prevent.
	return !errors.Is(err, net.ErrClosed)
}

// recoverPanic is the panic barrier for the goroutines this client owns. They are spawned
// outside the gin handler, so gin.Recovery() cannot see them: without this, an unrecovered
// panic here terminates the whole node process instead of dropping one connection
// (KLC-2596). It is this package's counterpart to shared.SafeRun, which guards the
// goroutines the API layer detaches, and takes a teardown func rather than an io.Closer
// because the per-request worker must answer its client instead of closing the connection.
//
// teardown, when set, runs before the panic is logged so it stays unconditional even if
// rendering the panic value misbehaves.
func (c *client) recoverPanic(op string, teardown func()) {
	r := recover()
	if r == nil {
		return
	}
	if teardown != nil {
		teardown()
	}
	c.hub.logRecoveredPanic(op, r)
}

// watch this function read client messages to check if client cancel the conn or send a ping
func (c *client) loopIn() {
	// Registered before the teardown defer so it runs after it: the connection is closed and
	// deregistered exactly as on a clean exit, and a panic raised inside that teardown is
	// caught here too.
	defer c.recoverPanic(opLoopIn, nil)
	defer func() {
		c.Close()
		c.hub.handleClientDelete(c)
	}()
	label := opLoopIn
	// Bound every inbound frame and keep a lifetime read deadline, refreshed by the pong
	// handler and each read. loopOut's pings keep a live client warm while a dead/idle one
	// is reclaimed at pongWait (GHSA-4fwh-wrm6-97xm).
	c.conn.SetReadLimit(c.hub.limits.maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(c.hub.limits.pongWait)); err != nil {
		// Debug, not Warn: a conn this new only fails here when the owner tore it down
		// before this goroutine got scheduled (a rejected insertion) — and a peer can drive
		// that at will, so it must not be a log-amplification lever.
		log.Debug(label, "err", err.Error())
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.hub.limits.pongWait))
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if unexpectedClose(err) {
				c.hub.logReadFailure(label, err)
			}
			break
		}
		if err := c.conn.SetReadDeadline(time.Now().Add(c.hub.limits.pongWait)); err != nil {
			// Debug, not Warn: this only fires when a concurrent c.Close() lands between a
			// successful read and this call — normal teardown, not an anomaly.
			log.Debug(label, "err", err.Error())
			break
		}

		// ReadMessage only yields data frames; gorilla answers client ping and close
		// control frames through its default handlers (close surfaces as a read error
		// above). Only text frames carry client requests.
		if messageType != ws.TextMessage {
			continue
		}

		var req WSRequest
		if err := json.Unmarshal(message, &req); err != nil {
			c.send(WSResponse{Error: "invalid json: " + err.Error()})
			continue
		}
		// Acquire a worker slot, but abandon the read loop if the connection is torn
		// down meanwhile, so loopIn never lingers blocked on a full semaphore at close.
		select {
		case c.sem <- struct{}{}:
		case <-c.ctx.Done():
			return
		}
		ctx := c.ctx
		go func(ctx context.Context, req WSRequest) {
			// This goroutine runs the attacker-supplied req through the facade — the same
			// methods REST serves behind gin.Recovery() — so it is the widest unauthenticated
			// panic surface in the node. Registered before the semaphore release so it unwinds
			// last, keeping the recover in place while the other defers run. The slot release
			// itself is order-independent: a receive on a live channel cannot panic, and Go
			// runs the remaining defers even when a deferred call panics.
			defer c.recoverPanic(opHandleClientRequest, func() {
				c.send(WSResponse{ID: req.ID, Error: errInternal})
			})
			defer func() { <-c.sem }()
			select {
			case <-ctx.Done():
				return
			default:
			}
			c.hub.HandleClientRequest(c, req)
		}(ctx, req)
	}
}

func (c *client) loopOut() {
	// loopOut owns the only writer for this connection, so a panic here leaves it unusable:
	// close it rather than leaking a half-live client. loopIn then observes the closed conn
	// and deregisters from the hub, exactly as on a write error below.
	defer c.recoverPanic(opLoopOut, c.Close)

	// Ping the client periodically; its pong refreshes loopIn's read deadline, keeping
	// passive-but-live subscribers up while dead ones time out. WriteControl is safe to
	// call concurrently with the other writer.
	ticker := time.NewTicker(c.hub.limits.pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case m, ok := <-c.out:
			if !ok {
				return
			}
			// Bound the write: a client that re-arms its read deadline but never drains its
			// socket would otherwise park this goroutine here indefinitely (GHSA-4fwh-wrm6-97xm).
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.hub.limits.pingPeriod)); err != nil {
				c.hub.logWriteFailure("ws.loopOut", err)
				c.Close()
				return
			}
			if err := c.conn.WriteJSON(m); err != nil {
				c.hub.logWriteFailure("ws.loopOut", err)
				c.Close()
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(ws.PingMessage, nil, time.Now().Add(c.hub.limits.pingPeriod)); err != nil {
				c.hub.logWriteFailure("ws.loopOut.ping", err)
				c.Close()
				return
			}
		}
	}
}
