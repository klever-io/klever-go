package websocket_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	wsocket "github.com/klever-io/klever-go/network/api/websocket"
	socket "github.com/klever-io/klever-go/websocket"
	"github.com/stretchr/testify/assert"
)

func TestSubscribeTopics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws := gin.New()
	ws.Use(cors.Default())
	hub := socket.NewHub("", "", nil)
	wsocket.SubscribeTopics(ws, hub)

	srv := &http.Server{
		Addr:              ":23456",
		Handler:           ws,
		ReadHeaderTimeout: 1 * time.Second,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	socket, _, err := websocket.DefaultDialer.Dial("ws://localhost:23456/subscribe", nil)
	assert.NoError(t, err)
	assert.NotNil(t, socket)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	assert.NoError(t, err)
}
