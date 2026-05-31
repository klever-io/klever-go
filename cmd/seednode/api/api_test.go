package api

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/api/httpserver"
)

type stubMessenger struct {
	peers       []core.PeerID
	listenAddrs []string
	connected   []string
}

func (s *stubMessenger) Peers() []core.PeerID         { return s.peers }
func (s *stubMessenger) Addresses() []string          { return s.listenAddrs }
func (s *stubMessenger) ConnectedAddresses() []string { return s.connected }

func newTestServer(stub *stubMessenger, version string, started time.Time) *server {
	return &server{
		messenger: stub,
		version:   version,
		startTime: started,
	}
}

func setup(t *testing.T) (*gin.Engine, *stubMessenger) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stub := &stubMessenger{
		peers:       []core.PeerID{"peer-a", "peer-b", "peer-c"},
		listenAddrs: []string{"/ip4/127.0.0.1/tcp/1/p2p/A", "/ip4/10.0.0.1/tcp/1/p2p/A"},
		connected:   []string{"/ip4/2.2.2.2/tcp/1/p2p/Y", "/ip4/1.1.1.1/tcp/1/p2p/X"},
	}
	srv := newTestServer(stub, "v1.2.3", time.Now().Add(-90*time.Second))

	r := gin.New()
	srv.registerRoutes(r)
	return r, stub
}

// TestSeednodeAPI_HardenedServerDropsSlowHeader serves the real seednode routes through
// NewHardenedServer and confirms the seednode listener (the reporter's PoC path) drops a
// slow-header connection — GHSA-w4c6-7r69-w7j9, verified end-to-end, not just wired.
func TestSeednodeAPI_HardenedServerDropsSlowHeader(t *testing.T) {
	r, _ := setup(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := httpserver.NewHardenedServer(ln.Addr().String(), r.Handler())
	srv.ReadHeaderTimeout = 200 * time.Millisecond // tighten for a fast test
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Partial header, never terminated: the seednode listener must drop it.
	if _, err := conn.Write([]byte("GET /node/status HTTP/1.1\r\nHost: x\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	start := time.Now()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = bufio.NewReader(conn).ReadString('\n')
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("slow-header connection not dropped promptly: %v", elapsed)
	}
}

func TestPeers_returnsCountsAndSortedAddresses(t *testing.T) {
	r, _ := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var got peersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ConnectedPeers != 2 {
		t.Errorf("connectedPeers = %d, want 2", got.ConnectedPeers)
	}
	if got.KnownPeers != 3 {
		t.Errorf("knownPeers = %d, want 3", got.KnownPeers)
	}
	if len(got.ConnectedAddresses) != 2 || got.ConnectedAddresses[0] >= got.ConnectedAddresses[1] {
		t.Errorf("connectedAddresses not sorted: %v", got.ConnectedAddresses)
	}
	if len(got.ListenAddresses) != 2 {
		t.Errorf("listenAddresses = %v, want 2 entries", got.ListenAddresses)
	}
}

func TestNodeStatus_reportsUptimeAndVersion(t *testing.T) {
	r, _ := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/node/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var got nodeStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Version != "v1.2.3" {
		t.Errorf("version = %q, want v1.2.3", got.Version)
	}
	if got.UptimeSeconds < 89 || got.UptimeSeconds > 120 {
		t.Errorf("uptimeSeconds = %d, want ~90", got.UptimeSeconds)
	}
	if got.ConnectedPeers != 2 || got.KnownPeers != 3 {
		t.Errorf("counts = (%d,%d), want (2,3)", got.ConnectedPeers, got.KnownPeers)
	}
}

func TestNodeMetrics_prometheusFormat(t *testing.T) {
	r, _ := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/node/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	wantLines := []string{
		"seednode_connected_peers 2",
		"seednode_known_peers 3",
		`klv_build_info{version="v1.2.3",node_type="seednode"} 1`,
	}
	for _, want := range wantLines {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q\nfull body:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "seednode_uptime_seconds ") {
		t.Errorf("metrics body missing seednode_uptime_seconds line\nfull body:\n%s", body)
	}
}

func TestNodeMetrics_emptyVersionOmitsBuildInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubMessenger{}
	srv := newTestServer(stub, "", time.Now())
	r := gin.New()
	srv.registerRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/node/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if strings.Contains(w.Body.String(), "klv_build_info") {
		t.Errorf("expected klv_build_info to be omitted when version empty, got:\n%s", w.Body.String())
	}
}

func TestNodeMetrics_versionLabelEscaped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubMessenger{}
	srv := newTestServer(stub, `v"weird\ne`+"\n"+`xt`, time.Now())
	r := gin.New()
	srv.registerRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/node/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	// If escaping worked, every newline in the body is a line terminator —
	// no raw newline leaked from the version label into the middle of a line.
	if got, want := strings.Count(body, "\n"), countNonEmptyLines(body); got != want {
		t.Errorf("escaping leaked a raw newline into the label: \\n count = %d, lines = %d, body:\n%s", got, want, body)
	}
	if !strings.Contains(body, `\"`) || !strings.Contains(body, `\\`) {
		t.Errorf("expected escaped quote and backslash in label, body:\n%s", body)
	}
}

func TestSnapshot_doesNotMutateMessengerSlices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &stubMessenger{
		connected: []string{"/ip4/9/tcp/1/p2p/Z", "/ip4/1/tcp/1/p2p/A", "/ip4/5/tcp/1/p2p/M"},
	}
	srv := newTestServer(stub, "", time.Now())

	wantConnected := append([]string(nil), stub.connected...)

	for range 3 {
		_ = srv.snapshot()
	}

	for i, addr := range stub.connected {
		if addr != wantConnected[i] {
			t.Fatalf("snapshot mutated stub.connected[%d]: got %q, want %q (full got: %v)",
				i, addr, wantConnected[i], stub.connected)
		}
	}
}

func countNonEmptyLines(body string) int {
	n := 0
	for line := range strings.SplitSeq(body, "\n") {
		if line != "" {
			n++
		}
	}
	return n
}
