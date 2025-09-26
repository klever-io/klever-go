package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("seednode/api")

// Start will boot up the api and appropriate routes, handlers and validators
func Start(restAPIInterface string, marshalizer marshal.Marshalizer) error {
	ws := gin.Default()
	ws.Use(cors.Default())

	registerRoutes(ws, marshalizer)

	return ws.Run(restAPIInterface)
}

func registerRoutes(ws *gin.Engine, marshalizer marshal.Marshalizer) {
	registerLoggerWsRoute(ws, marshalizer)
}

func registerLoggerWsRoute(ws *gin.Engine, marshalizer marshal.Marshalizer) {
	upgrader := websocket.Upgrader{}

	ws.GET("/log", func(c *gin.Context) {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls, err := logs.NewLogSender(marshalizer, conn, log)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls.StartSendingBlocking()
	})
}
