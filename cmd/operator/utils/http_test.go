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

func TestGetURLTruncatesBodyInError(t *testing.T) {
	longBody := strings.Repeat("Z", 2000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(longBody))
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.Equal(t, 512, strings.Count(err.Error(), "Z"), "body snippet must be capped at 512 bytes")
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

func TestPostURLReturnsErrorOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.PostURL(srv.URL, `{}`, nil, &got)
	require.Error(t, err)
}

func TestErrorBodyReadFailureIsWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Declare far more body than we actually send, then close the raw connection —
		// the client's Read on the error body sees an unexpected EOF instead of a clean
		// stream, exercising checkStatus's io.ReadAll failure path (previously discarded
		// via `snippet, _ := io.ReadAll(...)`).
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusInternalServerError)
		hj, ok := w.(http.Hijacker)
		require.True(t, ok, "test server response writer must support hijacking")
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		_, _ = conn.Write([]byte("short"))
		_ = conn.Close()
	}))
	defer srv.Close()

	var got sampleResult
	err := utils.GetURL(srv.URL, &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "failed to read response body")
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
