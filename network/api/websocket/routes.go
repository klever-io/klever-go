package websocket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/websocket"
)

var log = logger.GetOrCreate("subscribe")

const (
	subscribeReadTimeout = 30 * time.Second
	subscribeOp          = "ws.Subscribe"
)

var upgrader = gorilla.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type subscribeRequest struct {
	Addresses []string `json:"addresses"`
	Types     []string `json:"subscribed_types"`
}

func SubscribeTopics(ws *gin.Engine, hub *websocket.SocketHub) {
	ws.GET("/subscribe", func(c *gin.Context) {
		handleSubscribe(c, hub)
	})
}

func handleSubscribe(c *gin.Context, hub *websocket.SocketHub) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error(subscribeOp, "err", err.Error())
		return
	}

	go processSubscription(conn, hub)
}

func processSubscription(conn *gorilla.Conn, hub *websocket.SocketHub) {
	if err := conn.SetReadDeadline(time.Now().Add(subscribeReadTimeout)); err != nil {
		log.Error(subscribeOp, "err", err.Error())
		_ = conn.Close()
		return
	}

	var req subscribeRequest
	if err := conn.ReadJSON(&req); err != nil {
		log.Error(subscribeOp, "err", err.Error())
		_ = conn.Close()
		return
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Error(subscribeOp, "err", err.Error())
		_ = conn.Close()
		return
	}

	if len(req.Types) == 0 {
		_ = conn.WriteJSON(map[string]string{"error": "subscribed_types must not be empty"})
		_ = conn.Close()
		return
	}

	parsedTypes, err := parseEventTypes(req.Types)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		_ = conn.Close()
		return
	}

	log.Debug(subscribeOp, "types", fmt.Sprintf("%v", parsedTypes), "addresses", fmt.Sprintf("%v", req.Addresses))
	client := websocket.NewClient(conn, hub)
	hub.HandleClientInsertion(parsedTypes, req.Addresses, client)
}

func parseEventTypes(types []string) ([]indexer.EventType, error) {
	var parsed []indexer.EventType
	seen := make(map[indexer.EventType]struct{})

	for _, evType := range types {
		et, err := indexer.NewEventTypeStrict(evType)
		if err != nil {
			return nil, fmt.Errorf("invalid subscription type: %s", evType)
		}
		if _, ok := seen[et]; ok {
			continue
		}
		seen[et] = struct{}{}
		parsed = append(parsed, et)
	}

	return parsed, nil
}
