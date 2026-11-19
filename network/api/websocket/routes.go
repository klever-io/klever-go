package websocket

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/klever-io/klever-go/websocket"
)

var log = logger.GetOrCreate("subscribe")

// subscribePanicWarn budgets the recovered-panic line of the subscription goroutine: the
// reconnect rate behind a panic a peer can reproduce is the peer's.
var subscribePanicWarn = websocket.NewDropWarner(websocket.PeerDrivenLogWindow)

const (
	subscribeReadTimeout = 30 * time.Second
	subscribeOp          = "ws.Subscribe"
)

// HandshakeTimeout bounds the write of the 101 upgrade response. Gorilla clears every deadline
// on the hijacked socket before that write and re-arms one only when this field is set, and the
// HTTP server's write timeout is deliberately unset, so without it a peer that leaves the send
// buffer full and then upgrades parks the handler inside Upgrade — holding the connection-limiter
// slot and the global throttler slot, with no deadline anywhere to reclaim either. The sender's
// own handshake deadline only starts once Upgrade has returned.
const HandshakeTimeout = 10 * time.Second

var upgrader = gorilla.Upgrader{
	// Origin isn't enforced here by design: the node runs headless behind an operator proxy
	// that owns origin/CORS policy (KLC-2450). Note this is a delegation, not an absence of
	// risk — the original rationale claimed /subscribe carries no ambient credentials, which
	// is wrong: api.go attaches Basic Auth when the route is `secured`, and browsers replay
	// cached credentials on a same-host WebSocket handshake. A secured /subscribe fronted by
	// a proxy that does not check Origin is CSWSH-exposed the way /log was before
	// AllowedOriginChecker below. Enforce origin at the proxy, or keep /subscribe unsecured
	// and public.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: HandshakeTimeout,
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
	limiter := NewConnLimiter(opt.MaxConnections, opt.MaxConnectionsPerIP)

	handlers := append([]gin.HandlerFunc{}, opt.AuthHandlers...)
	handlers = append(handlers, func(c *gin.Context) {
		handleSubscribe(c, hub, limiter)
	})
	ws.GET("/subscribe", handlers...)
}

func handleSubscribe(c *gin.Context, hub *websocket.SocketHub, limiter *ConnLimiter) {
	release, ok := limiter.Acquire(RemoteIP(c.Request))
	if !ok {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, map[string]string{"error": "too many websocket connections"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		release()
		// A plain GET with no upgrade headers lands here, so this is one line per request
		// for anyone who asks. Rate-limited, on its own budget so a flood of these cannot
		// hide a rarer failure elsewhere.
		hub.LogUpgradeFailure(subscribeOp, err)
		return
	}

	shared.SafeGo(log, subscribeOp, conn, subscribePanicWarn, func() { processSubscription(conn, hub, release) })
}

// rejectHandshake answers a bad subscribe with a reason and closes. The write is bounded
// like loopOut's: without a deadline a peer that provokes a rejection and then stops
// reading could park this goroutine — and the limiter slot it holds — for as long as the
// TCP stack keeps trying. One small frame on a fresh connection lands in the kernel buffer
// and returns in practice, so this is the guard rail rather than a live hole, but every
// write to a peer-controlled socket should carry one.
func rejectHandshake(conn *gorilla.Conn, reason string) {
	_ = conn.SetWriteDeadline(time.Now().Add(subscribeReadTimeout))
	_ = conn.WriteJSON(map[string]string{"error": reason})
	_ = conn.Close()
}

// RemoteIP returns the per-IP cap key for the connecting peer, derived from the raw remote
// address. It does not trust X-Forwarded-For, so the key cannot be spoofed (matches the
// existing sourceThrottler middleware), and it buckets IPv6 by /64 so a single routed
// allocation cannot masquerade as unlimited distinct sources.
func RemoteIP(r *http.Request) string {
	return shared.RemoteIPKey(r.RemoteAddr, shared.IPv6BucketBits)
}

// AllowedOriginChecker returns a gorilla CheckOrigin func for a route that may carry ambient
// credentials. A request with no Origin header is a non-browser client — the log viewer, curl,
// wscat. Origin is set by the browser and page script cannot forge it, so its absence means no
// web page is driving the connection, and the request is allowed. A request that does carry an
// Origin came from a page, and is allowed only if that origin is on the list: this is what stops
// a site an operator happens to visit from opening ws://localhost:8080/log and streaming node
// logs on their credentials (CSWSH).
//
// An empty allowlist therefore blocks every browser origin while leaving every non-browser
// client working, which is the right default for a headless node.
func AllowedOriginChecker(allowed []string) func(*http.Request) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(origin))] = struct{}{}
	}

	return func(r *http.Request) bool {
		// Values, not Get: Get returns the first value only and reports an explicitly empty
		// header the same as an absent one. Absent is the non-browser case and is allowed.
		// An empty value, or more than one, is neither what a browser sends nor what the
		// operator listed, and is refused rather than matched on its first entry.
		origins := r.Header.Values("Origin")
		if len(origins) == 0 {
			return true
		}
		if len(origins) != 1 || origins[0] == "" {
			return false
		}

		_, ok := allowedSet[strings.ToLower(origins[0])]
		return ok
	}
}

// NewUpgrader builds the upgrader for a route that may carry ambient credentials: the origin
// allowlist above, and the handshake write bound every upgrader here must carry. Built once at
// registration and never mutated afterwards; assigning CheckOrigin per request raced with
// Upgrade reading it.
func NewUpgrader(allowedOrigins []string) gorilla.Upgrader {
	return gorilla.Upgrader{
		CheckOrigin:      AllowedOriginChecker(allowedOrigins),
		HandshakeTimeout: HandshakeTimeout,
	}
}

func processSubscription(conn *gorilla.Conn, hub *websocket.SocketHub, release func()) {
	// Held for the whole connection lifetime: fires on every early return below, once the
	// client is live after <-client.Done() unblocks at teardown, and on the unwind if anything
	// below panics (SafeGo recovers above this frame).
	defer release()

	// Bound the handshake frame before it is read.
	conn.SetReadLimit(hub.MaxMessageSize())

	if err := conn.SetReadDeadline(time.Now().Add(subscribeReadTimeout)); err != nil {
		hub.LogHandshakeFailure(subscribeOp, err)
		_ = conn.Close()
		return
	}

	var req subscribeRequest
	if err := conn.ReadJSON(&req); err != nil {
		// Malformed JSON, a wrong-typed field or an early close all arrive here, before any
		// of the caps below can apply — so this is the cheapest log lever on the route.
		hub.LogHandshakeFailure(subscribeOp, err)
		_ = conn.Close()
		return
	}
	// Deadline intentionally left armed; loopIn re-arms a lifetime deadline immediately.

	if len(req.Types) == 0 {
		rejectHandshake(conn, "subscribed_types must not be empty")
		return
	}

	parsedTypes, err := parseEventTypes(req.Types)
	if err != nil {
		rejectHandshake(conn, err.Error())
		return
	}

	if len(req.Addresses) > hub.MaxAddressesPerSubscribe() {
		rejectHandshake(conn, "too many addresses in a single subscribe")
		return
	}

	// Apply the hub's own input caps here, while this goroutine still owns the raw
	// connection and can answer with a reason. Past NewClient the writer goroutine owns the
	// socket, so a rejection can only be an abrupt close the peer cannot diagnose — and the
	// per-address byte cap is reachable on any fresh connection, not a theoretical branch.
	if err := hub.ValidateSubscription(parsedTypes, req.Addresses); err != nil {
		rejectHandshake(conn, err.Error())
		return
	}

	// Best effort: a full per-IP or node-wide budget is a normal outcome now, so answer it here.
	if err := hub.PrecheckAddressBudget(parsedTypes, req.Addresses, conn.RemoteAddr()); err != nil {
		rejectHandshake(conn, err.Error())
		return
	}

	log.Debug(subscribeOp, "types", fmt.Sprintf("%v", parsedTypes), "addressCount", len(req.Addresses))
	client := websocket.NewClient(conn, hub)
	if err := hub.HandleClientInsertion(parsedTypes, req.Addresses, client); err != nil {
		// Pre-validated above, so what reaches here is a teardown race or a budget filled
		// since the precheck. The hub picks the level and rate-limits the Warn, so this cannot become a
		// log-amplification lever.
		hub.LogRejectedInsertion(subscribeOp, err)
		// Close through the client: NewClient already started loopIn/loopOut, and closing
		// the raw conn under them logs two warnings per rejected connection.
		client.Close()
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
