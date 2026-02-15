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
			log.Error("ws.close", "err", err.Error())
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

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				log.Error("ws.loopIn", "err", err.Error())
			}
			break
		}

		switch messageType {
		case ws.CloseMessage:
			return
		case ws.PingMessage:
			err = c.conn.WriteControl(ws.PongMessage, nil, time.Now().Add(pingPeriod))
			if err != nil {
				log.Error("ws.loopIn", "err", err.Error())
				return
			}
		case ws.TextMessage:
			var req WSRequest
			if err := json.Unmarshal(message, &req); err != nil {
				c.send(WSResponse{Error: "invalid json: " + err.Error()})
				continue
			}
			c.sem <- struct{}{}
			go func() {
				defer func() { <-c.sem }()
				c.hub.HandleClientRequest(c, req)
			}()
		}
	}
}

func (c *client) loopOut() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case m := <-c.out:
			err := c.conn.WriteJSON(m)
			if err != nil {
				log.Error("ws.loopOut", "err", err.Error())
				c.close()
			}
		}
	}
}
