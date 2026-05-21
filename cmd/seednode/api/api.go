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
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/logs"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("seednode/api")

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
	marshalizer marshal.Marshalizer
	messenger   peerInfoProvider
	version     string
	startTime   time.Time
}

// Start boots the gin server with the seednode routes. messenger and version
// are exposed by the /peers, /node/status and /node/metrics endpoints.
// startTime should reflect process start so uptime reflects the binary, not
// the API listener.
func Start(restAPIInterface string, marshalizer marshal.Marshalizer, messenger peerInfoProvider, version string, startTime time.Time) error {
	srv := &server{
		marshalizer: marshalizer,
		messenger:   messenger,
		version:     version,
		startTime:   startTime,
	}

	ws := gin.Default()
	ws.Use(cors.Default())

	srv.registerRoutes(ws)

	return ws.Run(restAPIInterface)
}

func (s *server) registerRoutes(ws *gin.Engine) {
	s.registerLoggerWsRoute(ws)
	ws.GET("/peers", s.peers)
	ws.GET("/node/status", s.nodeStatus)
	ws.GET("/node/metrics", s.nodeMetrics)
}

func (s *server) registerLoggerWsRoute(ws *gin.Engine) {
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

		ls, err := logs.NewLogSender(s.marshalizer, conn, log)
		if err != nil {
			log.Error(err.Error())
			return
		}

		ls.StartSendingBlocking()
	})
}

// snapshot reads the peer view in one shot so the response cannot tear
// across separate getter calls under churn.
type peerSnapshot struct {
	connectedAddrs []string
	knownPeers     int
	listenAddrs    []string
}

func (s *server) snapshot() peerSnapshot {
	connected := s.messenger.ConnectedAddresses()
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
