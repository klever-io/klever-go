package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/httpserver"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("seednode/api")

// Route package keys, config route names, and served URL paths, kept as constants to avoid
// duplicating the string literals across registration and the fail-safe default.
const (
	logPackage   = "log"
	peersPackage = "peers"
	nodePackage  = "node"

	logRoute     = "/log"
	peersRoute   = "/peers"
	statusRoute  = "/status"
	metricsRoute = "/metrics"

	nodeStatusPath  = "/node/status"
	nodeMetricsPath = "/node/metrics"
)

// peerInfoProvider is the narrow surface the seednode API needs from the p2p
// messenger. Kept inside this package so handlers can be tested without
// pulling in the full libp2p stack.
type peerInfoProvider interface {
	Peers() []core.PeerID
	Addresses() []string
	ConnectedAddresses() []string
}

// server bundles the state needed to serve the seednode HTTP surface.
type server struct {
	marshalizer  marshal.Marshalizer
	messenger    peerInfoProvider
	version      string
	startTime    time.Time
	routesConfig config.APIRoutesConfig
}

// Start boots the gin server with the seednode routes. messenger and version
// are exposed by the /peers, /node/status and /node/metrics endpoints.
// startTime should reflect process start so uptime reflects the binary, not
// the API listener.
func Start(restAPIInterface string, marshalizer marshal.Marshalizer, messenger peerInfoProvider, version string, startTime time.Time, routesConfig config.APIRoutesConfig) error {
	srv := &server{
		marshalizer:  marshalizer,
		messenger:    messenger,
		version:      version,
		startTime:    startTime,
		routesConfig: routesConfig,
	}

	gin.SetMode(gin.ReleaseMode)

	// gin.New skips the access logger so /node/metrics scrapes don't spam stdout.
	ws := gin.New()
	ws.Use(gin.Recovery())
	ws.Use(cors.Default())

	srv.registerRoutes(ws)

	// Hardened http.Server instead of ws.Run: adds the ReadHeaderTimeout that
	// http.ListenAndServe lacks (slow-header DoS, GHSA-w4c6-7r69-w7j9).
	return httpserver.NewHardenedServer(restAPIInterface, ws.Handler()).ListenAndServe()
}

func (s *server) registerRoutes(ws *gin.Engine) {
	if s.routesConfig.IsRouteEnabled(logPackage, logRoute) {
		s.registerLoggerWsRoute(ws)
	}
	s.registerGet(ws, peersPackage, peersRoute, peersRoute, s.peers)
	s.registerGet(ws, nodePackage, statusRoute, nodeStatusPath, s.nodeStatus)
	s.registerGet(ws, nodePackage, metricsRoute, nodeMetricsPath, s.nodeMetrics)
}

// registerGet registers a GET endpoint when its config route (pkg/configName) is open, prepending
// Basic Authentication when that route is marked secured. path is the URL gin actually serves.
func (s *server) registerGet(ws *gin.Engine, pkg, configName, path string, handler gin.HandlerFunc) {
	if !s.routesConfig.IsRouteEnabled(pkg, configName) {
		return
	}

	handlers := []gin.HandlerFunc{handler}
	if s.routesConfig.IsRouteSecured(pkg, configName) {
		handlers = append([]gin.HandlerFunc{middleware.NewAuthenticationFunc(s.routesConfig)}, handlers...)
	}

	ws.GET(path, handlers...)
}

func (s *server) registerLoggerWsRoute(ws *gin.Engine) {
	upgrader := websocket.Upgrader{}

	// Only an authenticated (secured) /log may apply a client-supplied logger profile to the
	// process-global logger; on an unauthenticated /log profiles are ignored (GHSA-9v8p-frvj-2pcm).
	secured := s.routesConfig.IsRouteSecured(logPackage, logRoute)

	logHandler := func(c *gin.Context) {
		upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls, err := logs.NewLogSender(s.marshalizer, conn, log, secured)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls.StartSendingBlocking()
	}

	handlers := []gin.HandlerFunc{logHandler}
	if secured {
		// GHSA-9v8p-frvj-2pcm / KLC-2438: enforce authentication before the WebSocket
		// upgrade. Without this the seednode /log stream was always unauthenticated.
		handlers = append([]gin.HandlerFunc{middleware.NewAuthenticationFunc(s.routesConfig)}, handlers...)
	}

	ws.GET(logRoute, handlers...)
}

// DefaultRoutesConfig is the fail-safe used when no API config file can be loaded: the read-only
// monitoring endpoints stay enabled so observability never silently breaks, while /log (which can
// stream logs and, when secured, mutate the logger profile) stays disabled rather than being
// exposed unauthenticated.
func DefaultRoutesConfig() config.APIRoutesConfig {
	return config.APIRoutesConfig{
		APIPackages: map[string]config.APIPackageConfig{
			peersPackage: {Routes: []config.RouteConfig{{Name: peersRoute, Open: true}}},
			nodePackage: {Routes: []config.RouteConfig{
				{Name: statusRoute, Open: true},
				{Name: metricsRoute, Open: true},
			}},
		},
	}
}

// peerSnapshot: connected count matches its slice; knownPeers is a separate read and may drift under churn.
type peerSnapshot struct {
	connectedAddrs []string
	knownPeers     int
	listenAddrs    []string
}

func (s *server) snapshot() peerSnapshot {
	connected := append([]string(nil), s.messenger.ConnectedAddresses()...)
	sort.Strings(connected)
	return peerSnapshot{
		connectedAddrs: connected,
		knownPeers:     len(s.messenger.Peers()),
		listenAddrs:    s.messenger.Addresses(),
	}
}

type peersResponse struct {
	ConnectedPeers     int      `json:"connectedPeers"`
	KnownPeers         int      `json:"knownPeers"`
	ListenAddresses    []string `json:"listenAddresses"`
	ConnectedAddresses []string `json:"connectedAddresses"`
}

func (s *server) peers(c *gin.Context) {
	snap := s.snapshot()
	c.JSON(http.StatusOK, peersResponse{
		ConnectedPeers:     len(snap.connectedAddrs),
		KnownPeers:         snap.knownPeers,
		ListenAddresses:    snap.listenAddrs,
		ConnectedAddresses: snap.connectedAddrs,
	})
}

type nodeStatusResponse struct {
	Version         string   `json:"version"`
	UptimeSeconds   uint64   `json:"uptimeSeconds"`
	ConnectedPeers  int      `json:"connectedPeers"`
	KnownPeers      int      `json:"knownPeers"`
	ListenAddresses []string `json:"listenAddresses"`
}

func (s *server) nodeStatus(c *gin.Context) {
	snap := s.snapshot()
	c.JSON(http.StatusOK, nodeStatusResponse{
		Version:         s.version,
		UptimeSeconds:   uint64(time.Since(s.startTime).Seconds()),
		ConnectedPeers:  len(snap.connectedAddrs),
		KnownPeers:      snap.knownPeers,
		ListenAddresses: snap.listenAddrs,
	})
}

var prometheusLabelEscaper = strings.NewReplacer(
	`\`, `\\`,
	"\n", `\n`,
	`"`, `\"`,
)

func (s *server) nodeMetrics(c *gin.Context) {
	snap := s.snapshot()
	var b strings.Builder
	fmt.Fprintf(&b, "seednode_connected_peers %d\n", len(snap.connectedAddrs))
	fmt.Fprintf(&b, "seednode_known_peers %d\n", snap.knownPeers)
	fmt.Fprintf(&b, "seednode_uptime_seconds %d\n", uint64(time.Since(s.startTime).Seconds()))
	if s.version != "" {
		fmt.Fprintf(&b, "klv_build_info{version=\"%s\",node_type=\"seednode\"} 1\n",
			prometheusLabelEscaper.Replace(s.version))
	}
	c.String(http.StatusOK, b.String())
}
