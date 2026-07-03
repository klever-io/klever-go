package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/klever-io/klever-go/statusHandler/presenter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		raw        string
		useWss     bool
		wantHost   string
		wantSecure bool
	}{
		{"bare host", "localhost:8801", false, "localhost:8801", false},
		{"bare host with -w", "localhost:8801", true, "localhost:8801", true},
		{"http scheme", "http://localhost:8801", false, "localhost:8801", false},
		{"https scheme without -w", "https://localhost:8801", false, "localhost:8801", true},
		{"http scheme with -w override", "http://localhost:8801", true, "localhost:8801", true},
		{"https scheme with -w", "https://localhost:8801", true, "localhost:8801", true},
		{"ws scheme", "ws://localhost:8801", false, "localhost:8801", false},
		{"wss scheme", "wss://localhost:8801", false, "localhost:8801", true},
		{"uppercase http scheme", "HTTP://localhost:8801", false, "localhost:8801", false},
		{"uppercase https scheme", "HTTPS://localhost:8801", false, "localhost:8801", true},
		{"trailing slash", "http://localhost:8801/", false, "localhost:8801", false},
		{"https trailing slash", "https://localhost:8801/", false, "localhost:8801", true},
		{"host and port preserved", "http://127.0.0.1:9090", false, "127.0.0.1:9090", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeAddress(tc.raw, tc.useWss)
			assert.Equal(t, tc.wantHost, got.host)
			assert.Equal(t, tc.wantSecure, got.secure)

			if tc.wantSecure {
				assert.Equal(t, wss, got.wsScheme())
				assert.Equal(t, schemeHTTPS, got.httpScheme())
			} else {
				assert.Equal(t, ws, got.wsScheme())
				assert.Equal(t, "http", got.httpScheme())
			}
		})
	}
}

func TestNewStatusMetricsProviderBuildsSchemeAwareBaseURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		address  string
		useWss   bool
		wantBase string
	}{
		{"bare host", "localhost:8801", false, "http://localhost:8801"},
		{"http scheme", "http://localhost:8801", false, "http://localhost:8801"},
		{"https scheme without -w", "https://localhost:8801", false, "https://localhost:8801"},
		{"http scheme with -w override", "http://localhost:8801", true, "https://localhost:8801"},
		{"trailing slash stripped", "http://localhost:8801/", false, "http://localhost:8801"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			psh := presenter.NewPresenterStatusHandler()
			smp, err := NewStatusMetricsProvider(psh, tc.address, 1000, tc.useWss)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBase, smp.nodeAddress)
		})
	}
}

func TestLogWebSocketURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		scheme string
		host   string
		want   string
	}{
		{"ws", ws, "localhost:8801", "ws://localhost:8801/log"},
		{"wss", wss, "localhost:8801", "wss://localhost:8801/log"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := logWebSocketURL(tc.scheme, tc.host)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "http://")
		})
	}
}

func TestLogWebSocketURLFromSchemePrefixedAddressHasNoDoubleScheme(t *testing.T) {
	t.Parallel()

	addr := normalizeAddress("http://localhost:8801", false)
	got := logWebSocketURL(addr.wsScheme(), addr.host)
	assert.Equal(t, "ws://localhost:8801/log", got)
}

func TestStatusMetricsProviderFetchesWithSchemePrefixedAddress(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, statusMetricsUrlSuffix, r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"metrics":{"klv_chain_id":"stub"}},"error":"","code":""}`))
	}))
	defer srv.Close()

	psh := presenter.NewPresenterStatusHandler()
	smp, err := NewStatusMetricsProvider(psh, srv.URL, 1000, false)
	require.NoError(t, err)

	metrics, err := smp.loadMetricsFromApi()
	require.NoError(t, err)
	assert.Equal(t, "stub", metrics["klv_chain_id"])
}

func TestOpenWebSocketDialsWithSchemePrefixedAddress(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _, _ = conn.ReadMessage()
	}))
	defer srv.Close()

	addr := normalizeAddress(srv.URL, false)
	conn, err := openWebSocket(addr.wsScheme(), addr.host)
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()
}
