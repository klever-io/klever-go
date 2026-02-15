package websocket

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/websocket"
)

var log = logger.GetOrCreate("subscribe")

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
		log.Error("ws.Subscribe", "err", err.Error())
		return
	}

	var req subscribeRequest
	if err = conn.ReadJSON(&req); err != nil {
		log.Error("ws.Subscribe", "err", err.Error())
		_ = conn.Close()
		return
	}

	parsedTypes, err := parseEventTypes(req.Types)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		_ = conn.Close()
		return
	}

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
