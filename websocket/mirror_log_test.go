package websocket

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/operator/utils"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logArgCapture is a log observer and formatter in one: for lines with the given message
// (and, when msg is set, the given "msg" field) it records the value that follows key, so
// a test asserts on what the logger actually emitted for that field.
type logArgCapture struct {
	message string
	msg     string
	key     string
	mut     sync.Mutex
	values  []string
}

func (c *logArgCapture) Write(p []byte) (int, error) { return len(p), nil }

func (c *logArgCapture) Output(line logger.LogLineHandler) []byte {
	if line.GetMessage() != c.message {
		return nil
	}
	fields := map[string]string{}
	args := line.GetArgs()
	for i := 0; i+1 < len(args); i += 2 {
		fields[args[i]] = decodeLogArg(args[i+1])
	}
	if c.msg != "" && fields["msg"] != c.msg {
		return nil
	}
	if value, ok := fields[c.key]; ok {
		c.mut.Lock()
		c.values = append(c.values, value)
		c.mut.Unlock()
	}
	return []byte{1}
}

func (c *logArgCapture) IsInterfaceNil() bool { return c == nil }

func (c *logArgCapture) first() (string, bool) {
	c.mut.Lock()
	defer c.mut.Unlock()
	if len(c.values) == 0 {
		return "", false
	}
	return c.values[0], true
}

// decodeLogArg undoes the logger's hex encoding of text values. A value that does not
// decode to printable text (a plain number such as "10" also parses as hex) is returned as
// it came.
func decodeLogArg(value string) string {
	decoded, err := hex.DecodeString(value)
	if err != nil || !utf8.Valid(decoded) {
		return value
	}
	for _, r := range string(decoded) {
		if !unicode.IsPrint(r) && r != ' ' {
			return value
		}
	}
	return string(decoded)
}

// captureMirrorField registers an observer for the mirror's log line with the given "msg"
// and returns a getter for the first value of key it logged.
func captureMirrorField(t *testing.T, msg, key string) func() (string, bool) {
	t.Helper()
	capture := &logArgCapture{message: postMirrorLogOp, msg: msg, key: key}
	require.NoError(t, logger.AddLogObserver(capture, capture))
	t.Cleanup(func() { assert.NoError(t, logger.RemoveLogObserver(capture)) })
	return capture.first
}

// TestMirrorFailureLogBoundsTheBody pins the line the mirror writes about a failed post:
// a mirror answering 500 with a 64 KiB body must not get that body into the node's log.
// The body is its own field, bounded by loggableText (256 bytes plus its marker), with no
// line break, no split rune in front of the marker, and none of the body's tail.
func TestMirrorFailureLogBoundsTheBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantHead string
	}{
		{
			name:     "ascii body with a forged log line",
			body:     strings.Repeat("X", 60000) + "\nWARN forged line\n" + strings.Repeat("Y", 5000),
			wantHead: "XXXXXXXX",
		},
		{
			// 3-byte runes: byte 256 of the field falls inside one unless the cut steps back.
			name:     "multi-byte body",
			body:     strings.Repeat("€", 30000),
			wantHead: "€€€€",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loggedBody := captureMirrorField(t, "failed to post to mirror", "body")

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			hub := NewHub(ts.URL, "key", nil)
			startServerEnvWith(t, hub)

			hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})

			var body string
			require.Eventually(t, func() bool {
				v, ok := loggedBody()
				body = v
				return ok
			}, 2*time.Second, 10*time.Millisecond, "the failed post must be logged")

			assert.LessOrEqual(t, len(body), maxLoggableErrorLength+len("… (99999 bytes)"), "body must stay within loggableText's bound plus its marker")
			assert.Contains(t, body, tt.wantHead, "the head of the body must have been received and kept")
			assert.Contains(t, body, " bytes)", "the cut must be marked with the original length")
			assert.NotContains(t, body, "\n", "no line break from the body may reach the log")
			assert.NotContains(t, body, "YYYY", "the tail of the body must not be logged")
			assert.NotContains(t, body, string(utf8.RuneError), "the cut must land on a rune boundary")
		})
	}
}

// TestMirrorFailureLogReportsReadFailure pins the readError field: a mirror that declares
// a body it never delivers gets the read failure logged beside the partial body.
func TestMirrorFailureLogReportsReadFailure(t *testing.T) {
	loggedReadError := captureMirrorField(t, "failed to post to mirror", "readError")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusInternalServerError)
		hj, ok := w.(http.Hijacker)
		require.True(t, ok, "test server response writer must support hijacking")
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		_, _ = conn.Write([]byte("short"))
		_ = conn.Close()
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	startServerEnvWith(t, hub)
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})

	var readError string
	require.Eventually(t, func() bool {
		v, ok := loggedReadError()
		readError = v
		return ok
	}, 2*time.Second, 10*time.Millisecond, "the failed post must be logged with its read failure")
	assert.Contains(t, readError, "EOF")
}

// TestMirrorFailureLogKeepsAttributionApart pins that the mirror URL and the status are
// their own fields, so a body that imitates them changes nothing a reader relies on.
func TestMirrorFailureLogKeepsAttributionApart(t *testing.T) {
	loggedStatus := captureMirrorField(t, "failed to post to mirror", "status")
	loggedURL := captureMirrorField(t, "failed to post to mirror", "url")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"x"} status = 200 OK url = https://other-host/`))
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	startServerEnvWith(t, hub)
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})

	var status, url string
	require.Eventually(t, func() bool {
		s, okStatus := loggedStatus()
		u, okURL := loggedURL()
		status, url = s, u
		return okStatus && okURL
	}, 2*time.Second, 10*time.Millisecond, "the failed post must be logged with both status and url")
	assert.Equal(t, "502 Bad Gateway", status)
	assert.Equal(t, ts.URL, url)
}

// TestMirrorFailureFields drives the field builder directly: a plain error is one bounded,
// scrubbed lastError; a typed status error, even wrapped, is taken apart into status, body
// and url, with readError only when the body was not read whole.
func TestMirrorFailureFields(t *testing.T) {
	longLine := strings.Repeat("k", 300)
	statusErr := &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/hook", StatusCode: 502, Status: "502 Bad Gateway", Body: `{"error":"x"}`}
	readErr := &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/hook", StatusCode: 500, Status: "500 Internal Server Error", Body: "short", ReadErr: "unexpected EOF"}

	tests := []struct {
		name string
		err  error
		want map[string]string
	}{
		{
			name: "plain error is one bounded scrubbed lastError",
			err:  errors.New("dial tcp: " + longLine + "\nWARN forged"),
			want: map[string]string{"lastError": loggableText("dial tcp: "+longLine+"\nWARN forged", maxLoggableErrorLength)},
		},
		{
			name: "status error is taken apart",
			err:  statusErr,
			want: map[string]string{"status": "502 Bad Gateway", "body": `{"error":"x"}`, "url": "https://mirror.example/hook"},
		},
		{
			name: "wrapped status error is still found",
			err:  fmt.Errorf("mirror: %w", statusErr),
			want: map[string]string{"status": "502 Bad Gateway", "body": `{"error":"x"}`, "url": "https://mirror.example/hook"},
		},
		{
			name: "read failure adds readError",
			err:  readErr,
			want: map[string]string{"status": "500 Internal Server Error", "body": "short", "readError": "unexpected EOF", "url": "https://mirror.example/hook"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := mirrorFailureFields(tt.err)
			require.Equal(t, 0, len(fields)%2, "fields come in key/value pairs")
			got := map[string]string{}
			for i := 0; i < len(fields); i += 2 {
				got[fields[i].(string)] = fields[i+1].(string)
			}
			assert.Equal(t, tt.want, got)
			if lastError, ok := got["lastError"]; ok {
				assert.NotContains(t, lastError, "\n")
				assert.LessOrEqual(t, len(lastError), maxLoggableErrorLength+len("… (99999 bytes)"))
			}
		})
	}
}
