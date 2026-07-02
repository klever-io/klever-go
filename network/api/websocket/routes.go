package websocket

import (
	"fmt"
	"net"
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
	// Origin isn't enforced here by design: the node runs headless behind an operator proxy
	// that owns origin/CORS policy, and /subscribe carries no ambient credentials (KLC-2450).
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type subscribeRequest struct {
	Addresses []string `json:"addresses"`
	Types     []string `json:"subscribed_types"`
}

// SubscribeOptions configures the hardening applied to the /subscribe route.
type SubscribeOptions struct {
	// MaxConnections caps simultaneous live connections node-wide (0 = unlimited).
	MaxConnections uint32
	// MaxConnectionsPerIP caps simultaneous live connections per source IP (0 = unlimited).
	MaxConnectionsPerIP uint32
	// AuthHandlers run before the WebSocket upgrade when /subscribe is `secured`.
	AuthHandlers []gin.HandlerFunc
}

func SubscribeTopics(ws *gin.Engine, hub *websocket.SocketHub, opts ...SubscribeOptions) {
	var opt SubscribeOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// One limiter shared by every connection for the server lifetime.
	limiter := newConnLimiter(opt.MaxConnections, opt.MaxConnectionsPerIP)

	handlers := append([]gin.HandlerFunc{}, opt.AuthHandlers...)
	handlers = append(handlers, func(c *gin.Context) {
		handleSubscribe(c, hub, limiter)
	})
	ws.GET("/subscribe", handlers...)
}

func handleSubscribe(c *gin.Context, hub *websocket.SocketHub, limiter *connLimiter) {
	release, ok := limiter.acquire(remoteIP(c.Request))
	if !ok {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, map[string]string{"error": "too many websocket connections"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		release()
		log.Error(subscribeOp, "err", err.Error())
		return
	}

	go processSubscription(conn, hub, release)
}

// remoteIP returns the connecting peer's IP from the raw remote address. It does not
// trust X-Forwarded-For, so the per-IP connection cap cannot be spoofed (matches the
// existing sourceThrottler middleware).
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func processSubscription(conn *gorilla.Conn, hub *websocket.SocketHub, release func()) {
	// Held for the whole connection lifetime: fires on every early return below and, once
	// the client is live, after <-client.Done() unblocks at teardown.
	defer release()

	// Bound the handshake frame before it is read.
	conn.SetReadLimit(hub.MaxMessageSize())

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
	// Deadline intentionally left armed; loopIn re-arms a lifetime deadline immediately.

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

	if len(req.Addresses) > hub.MaxAddressesPerSubscribe() {
		_ = conn.WriteJSON(map[string]string{"error": "too many addresses in a single subscribe"})
		_ = conn.Close()
		return
	}

	log.Debug(subscribeOp, "types", fmt.Sprintf("%v", parsedTypes), "addressCount", len(req.Addresses))
	client := websocket.NewClient(conn, hub)
	if err := hub.HandleClientInsertion(parsedTypes, req.Addresses, client); err != nil {
		// Unreachable for a fresh client (resolve keeps per-client >= per-subscribe, and
		// req.Addresses was pre-checked); guard defensively.
		log.Warn(subscribeOp, "err", err.Error())
		_ = conn.Close()
		return
	}

	// Block until the connection is torn down so release() fires exactly at teardown.
	<-client.Done()
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
