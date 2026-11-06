package utils_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleResult struct {
	Name string `json:"name"`
}

func TestURLHandlesHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		status     int
		body       string
		wantErr    bool
		wantName   string
		wantSubstr []string
	}{
		{
			name:     "GET decodes success body",
			method:   http.MethodGet,
			status:   http.StatusOK,
			body:     `{"name":"klever"}`,
			wantName: "klever",
		},
		{
			name:       "GET surfaces server error",
			method:     http.MethodGet,
			status:     http.StatusInternalServerError,
			body:       `{"name":"boom"}`,
			wantErr:    true,
			wantSubstr: []string{"500", "boom"},
		},
		{
			name:       "GET rejects not found",
			method:     http.MethodGet,
			status:     http.StatusNotFound,
			body:       "not found",
			wantErr:    true,
			wantSubstr: []string{"404"},
		},
		{
			name:       "GET rejects multiple choices",
			method:     http.MethodGet,
			status:     http.StatusMultipleChoices,
			body:       "choices",
			wantErr:    true,
			wantSubstr: []string{"300"},
		},
		{
			name:     "POST decodes success body",
			method:   http.MethodPost,
			status:   http.StatusOK,
			body:     `{"name":"klever"}`,
			wantName: "klever",
		},
		{
			name:       "POST surfaces server error",
			method:     http.MethodPost,
			status:     http.StatusInternalServerError,
			body:       `{"name":"boom"}`,
			wantErr:    true,
			wantSubstr: []string{"500", "boom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			var got sampleResult
			var err error
			if tt.method == http.MethodPost {
				err = utils.PostURL(srv.URL, `{}`, nil, &got)
			} else {
				err = utils.GetURL(srv.URL, &got)
			}

			if tt.wantErr {
				require.Error(t, err)
				for _, substr := range tt.wantSubstr {
					assert.Contains(t, err.Error(), substr)
				}
				assert.Empty(t, got.Name)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantName, got.Name)
		})
	}
}

func TestGetURLSurfacesEOFOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	assert.True(t, errors.Is(err, io.EOF), "empty 2xx body must still surface io.EOF for not-found callers")
}

// TestErrorBodyIsReportedWhole pins that a failed call's body reaches the operator intact
// past the old 512-byte snippet (which cut the node's `{"error":"..."}` reason mid-string),
// and that only a body past the 64 KiB cap is cut, with the message saying so and no
// artefact of the cut (split rune, stray spaces). GetURL and PostURL share checkStatus, so
// both are driven through the same table.
func TestErrorBodyIsReportedWhole(t *testing.T) {
	const capBytes = 64 << 10
	reason := `{"error":"invalid argument: ` + strings.Repeat("the fee estimate needs a valid sender; ", 40) + `","code":"internal_issue"}`
	require.Greater(t, len(reason), 512, "the realistic body must exceed the old snippet or the test proves nothing")

	tests := []struct {
		name          string
		body          string
		wantWhole     string // must appear verbatim in the error
		wantZ         int    // -1 when the body is not made of Z
		wantTruncated bool
	}{
		{name: "realistic node error past 512 bytes", body: reason, wantWhole: reason, wantZ: -1},
		{name: "2000 bytes", body: strings.Repeat("Z", 2000), wantZ: 2000},
		{name: "exactly at the cap", body: strings.Repeat("Z", capBytes), wantZ: capBytes},
		{name: "past the cap is cut and says so", body: strings.Repeat("Z", capBytes+1000), wantZ: capBytes, wantTruncated: true},
		// A cut that lands inside a multi-byte rune must drop the rune, not leave U+FFFD in
		// front of the marker: 2-byte "é" after its lead byte, 3-byte "€" after two bytes,
		// 4-byte "😀" after one and after three bytes.
		{name: "cut inside a 2-byte rune drops it", body: strings.Repeat("Z", capBytes-1) + "é" + strings.Repeat("Z", 1000), wantZ: capBytes - 1, wantTruncated: true},
		{name: "cut inside a 3-byte rune drops it", body: strings.Repeat("Z", capBytes-2) + "€" + strings.Repeat("Z", 1000), wantZ: capBytes - 2, wantTruncated: true},
		{name: "cut after one byte of a 4-byte rune drops it", body: strings.Repeat("Z", capBytes-1) + "😀" + strings.Repeat("Z", 1000), wantZ: capBytes - 1, wantTruncated: true},
		{name: "cut after three bytes of a 4-byte rune drops it", body: strings.Repeat("Z", capBytes-3) + "😀" + strings.Repeat("Z", 1000), wantZ: capBytes - 3, wantTruncated: true},
		// A rune that ends exactly at the cap is complete and must be kept before the marker.
		{name: "rune ending exactly at the cap is kept", body: strings.Repeat("Z", capBytes-2) + "é" + strings.Repeat("Z", 1000), wantZ: capBytes - 2, wantTruncated: true, wantWhole: "é ... (response body truncated"},
		// 64 KiB of whitespace sanitises to nothing: the marker stands alone, no stray spaces.
		{name: "whitespace past the cap leaves only the marker", body: strings.Repeat(" ", capBytes+1000), wantZ: 0, wantTruncated: true, wantWhole: "502 Bad Gateway: (response body truncated at 65536 bytes)"},
	}

	for _, tt := range tests {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(tt.name+"/"+method, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(tt.body))
				}))
				defer srv.Close()

				var got sampleResult
				var err error
				if method == http.MethodGet {
					err = utils.GetURL(srv.URL, &got)
				} else {
					err = utils.PostURL(srv.URL, `{}`, nil, &got)
				}
				require.Error(t, err)
				assert.Contains(t, err.Error(), "502")
				assert.NotContains(t, err.Error(), "�", "a byte-boundary cut must not leave a replacement character")
				if tt.wantWhole != "" {
					assert.Contains(t, err.Error(), tt.wantWhole, "the full reason must survive into the error")
				}
				if tt.wantZ >= 0 {
					assert.Equal(t, tt.wantZ, strings.Count(err.Error(), "Z"))
				}
				if tt.wantTruncated {
					assert.Contains(t, err.Error(), "response body truncated at 65536 bytes")
				} else {
					assert.NotContains(t, err.Error(), "truncated")
				}
			})
		}
	}
}

func TestErrorStripsControlCharsFromServerBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("line1\nline2\x1b[2Kfaked-success"))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\n", "newlines must be stripped to prevent log line forging")
	assert.NotContains(t, err.Error(), "\x1b", "escape sequences must be stripped to prevent terminal spoofing")
	assert.Contains(t, err.Error(), "line1line2[2Kfaked-success")
}

// TestErrorLeadsWithStatusAndBody pins the field order the node's websocket mirror relies
// on: it logs this error through a 256-byte bound, so with a URL longer than that the
// status and the reason must still sit inside the first 256 bytes, ahead of the URL.
func TestErrorLeadsWithStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"mirror rejected the event"}`))
	}))
	defer srv.Close()

	longURL := srv.URL + "/" + strings.Repeat("p", 300)
	err := utils.PostURL(longURL, `{}`, nil, nil)
	require.Error(t, err)
	require.Greater(t, len(err.Error()), 256)

	head := err.Error()[:256]
	assert.Contains(t, head, "502 Bad Gateway")
	assert.Contains(t, head, "mirror rejected the event")
	assert.Contains(t, err.Error(), "from POST "+longURL, "method and URL still close the message")
}

// TestErrorStripsFormatCharsFromServerBody pins that the characters which change how text
// is displayed without being control characters are dropped too: a bidi override that
// would show the reason reversed, the line/paragraph separators some terminals treat as
// newlines, and a zero-width space that could hide a split in a value.
func TestErrorStripsFormatCharsFromServerBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("fee ‮001 KLV‬ to klv1​abc"))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	for _, forbidden := range []string{"‮", "‬", " ", " ", "​"} {
		assert.NotContains(t, err.Error(), forbidden)
	}
	assert.Contains(t, err.Error(), "fee 001 KLVtoklv1abc", "the visible text must survive with only the invisible characters removed")
}

func TestPostURLReturnsErrorOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.PostURL(srv.URL, `{}`, nil, &got)
	require.Error(t, err)
}

// newHijackingServer declares far more body than it actually sends, then closes the raw
// connection — the client's Read on the error body sees an unexpected EOF instead of a
// clean stream, exercising checkStatus's io.ReadAll failure path.
func newHijackingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusInternalServerError)
		hj, ok := w.(http.Hijacker)
		require.True(t, ok, "test server response writer must support hijacking")
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		_, _ = conn.Write([]byte("short"))
		_ = conn.Close()
	}))
}

// TestErrorBodyReadFailureIsWrapped covers checkStatus's io.ReadAll failure path from
// both callers that share it: the partial "short" body it did manage to read before the
// failure must survive into the error message (previously discarded via
// `snippet, _ := io.ReadAll(...)`), for GetURL and PostURL alike.
func TestErrorBodyReadFailureIsWrapped(t *testing.T) {
	t.Run("GetURL", func(t *testing.T) {
		srv := newHijackingServer(t)
		defer srv.Close()

		var got sampleResult
		err := utils.GetURL(srv.URL, &got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "failed to read response body")
		assert.Contains(t, err.Error(), "short", "partial body read before the failure must not be discarded")
	})

	t.Run("PostURL", func(t *testing.T) {
		srv := newHijackingServer(t)
		defer srv.Close()

		err := utils.PostURL(srv.URL, `{}`, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
		assert.Contains(t, err.Error(), "failed to read response body")
		assert.Contains(t, err.Error(), "short", "partial body read before the failure must not be discarded")
	})
}

func TestPostURLNilTargetReturnsNilOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"ignored"}`))
	}))
	defer srv.Close()

	err := utils.PostURL(srv.URL, `{}`, nil, nil)
	assert.NoError(t, err)
}
