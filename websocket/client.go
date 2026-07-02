package websocket

import (
	"context"
	"encoding/json"
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
		log.Warn("ws.send", "msg", "client output buffer full, dropping message")
	}
}

// close call this function to close client connection and remove from hub
func (c *client) close() {
	c.aliveLock.Lock()
	defer c.aliveLock.Unlock()
	if c.alive {
		if err := c.conn.Close(); err != nil {
			log.Warn("ws.close", "err", err.Error())
		}
		c.alive = false
		c.cancel()
		close(c.out)
		for len(c.out) > 0 {
			<-c.out
		}
	}
}

// watch this function read client messages to check if client cancel the conn or send a ping
func (c *client) loopIn() {
	defer func() {
		c.close()
		c.hub.handleClientDelete(c)
	}()

	// Bound every inbound frame and keep a lifetime read deadline, refreshed by the pong
	// handler and each read. loopOut's pings keep a live client warm while a dead/idle one
	// is reclaimed at pongWait (GHSA-4fwh-wrm6-97xm).
	c.conn.SetReadLimit(c.hub.limits.maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				log.Warn("ws.loopIn", "err", err.Error())
			}
			break
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

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
	// Ping the client periodically; its pong refreshes loopIn's read deadline, keeping
	// passive-but-live subscribers up while dead ones time out. WriteControl is safe to
	// call concurrently with the other writer.
	ticker := time.NewTicker(pingPeriod)
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
			if err := c.conn.SetWriteDeadline(time.Now().Add(pingPeriod)); err != nil {
				log.Warn("ws.loopOut", "err", err.Error())
				c.close()
				return
			}
			if err := c.conn.WriteJSON(m); err != nil {
				log.Warn("ws.loopOut", "err", err.Error())
				c.close()
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(ws.PingMessage, nil, time.Now().Add(pingPeriod)); err != nil {
				log.Warn("ws.loopOut.ping", "err", err.Error())
				c.close()
				return
			}
		}
	}
}
