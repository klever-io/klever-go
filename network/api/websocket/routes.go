package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/websocket"
)

var log = logger.GetOrCreate("subscribe")

func SubscribeTopics(ws *gin.Engine, hub *websocket.SocketHub) {
	ws.GET("/subscribe", func(c *gin.Context) {
		upgrader := gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error("ws.Subscribe", "err", err.Error())
			return
		}

		var req struct {
			Addresses []string `json:"addresses"`
			Types     []string `json:"subcribed_types"`
		}

		err = conn.ReadJSON(&req)
		if err != nil {
			log.Error("ws.Subscribe", "err", err.Error())
			return
		}

		var parsedTypes []indexer.EventType
		for _, evType := range req.Types {
			parsed := indexer.NewEventType(evType)

			var has bool
			for _, v := range parsedTypes {
				if v == parsed {
					has = true
					break
				}
			}

			if !has {
				parsedTypes = append(parsedTypes, parsed)
			}
		}

		client := websocket.NewClient(conn, hub)
		hub.HandleClientInsertion(parsedTypes, req.Addresses, client)
	})
}
