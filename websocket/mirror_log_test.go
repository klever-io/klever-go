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
	"unicode/utf8"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/operator/utils"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logArgCapture is a log observer and formatter in one: for the mirror's log line with the
// given "msg" field it records the first value that follows key, exactly as the logger
// emits it, so a test asserts on what the operator would read. The node's logger renders
// a string value holding any non-ASCII byte as hex, so a value that is meant to be
// readable must come out as plain text here.
type logArgCapture struct {
	msg   string
	key   string
	mut   sync.Mutex
	value string
	seen  bool
}

func (c *logArgCapture) Write(p []byte) (int, error) { return len(p), nil }

func (c *logArgCapture) Output(line logger.LogLineHandler) []byte {
	if line.GetMessage() != postMirrorLogOp {
		return nil
	}
	fields := map[string]string{}
	args := line.GetArgs()
	for i := 0; i+1 < len(args); i += 2 {
		fields[args[i]] = args[i+1]
	}
	if fields["msg"] != c.msg {
		return nil
	}
	if value, ok := fields[c.key]; ok {
		c.mut.Lock()
		if !c.seen {
			c.value, c.seen = value, true
		}
		c.mut.Unlock()
	}
	return []byte{1}
}

func (c *logArgCapture) IsInterfaceNil() bool { return c == nil }

func (c *logArgCapture) first() (string, bool) {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.value, c.seen
}

// captureMirrorField registers an observer for the mirror's log line with the given "msg"
// and returns a getter for the first value of key it logged.
func captureMirrorField(t *testing.T, msg, key string) func() (string, bool) {
	t.Helper()
	capture := &logArgCapture{msg: msg, key: key}
	require.NoError(t, logger.AddLogObserver(capture, capture))
	t.Cleanup(func() { assert.NoError(t, logger.RemoveLogObserver(capture)) })
	return capture.first
}

const loggedMarkerMax = len("... (99999 bytes)")

// TestMirrorFailureLogBoundsTheBody pins the line the mirror writes about a failed post:
// a mirror answering 500 with a 64 KiB body must not get that body into the node's log.
// The body is its own field, bounded by loggableText (256 bytes plus its marker), with no
// line break, no split rune in front of the marker, and none of the body's tail. An ASCII
// body must reach the log as readable text, not as the hex the logger uses for non-ASCII.
func TestMirrorFailureLogBoundsTheBody(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		hexed    bool // the logger renders a non-ASCII value as hex; decode before asserting
		wantHead string
	}{
		{
			// The forged line sits inside the first 256 bytes, so the bound alone cannot
			// hide it: it must have been flattened to one line.
			name:     "ascii body with a forged log line stays readable",
			body:     "head\nWARN forged line\n" + strings.Repeat("X", 60000) + strings.Repeat("Y", 5000),
			wantHead: "head WARN forged line XXXXXXXX",
		},
		{
			// 3-byte runes: byte 256 of the field falls inside one unless the cut steps back.
			name:     "multi-byte body",
			body:     strings.Repeat("€", 30000),
			hexed:    true,
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

			if tt.hexed {
				decoded, err := hex.DecodeString(body)
				require.NoError(t, err, "a non-ASCII value is logged as hex")
				body = string(decoded)
			}

			assert.LessOrEqual(t, len(body), maxLoggableErrorLength+loggedMarkerMax, "body must stay within loggableText's bound plus its marker")
			assert.Contains(t, body, tt.wantHead, "the head of the body must have been received and kept, as readable text")
			assert.Contains(t, body, " bytes)", "the cut must be marked with the original length")
			assert.NotContains(t, body, "\n", "no line break from the body may reach the log")
			assert.NotContains(t, body, "YYYY", "the tail of the body must not be logged")
			assert.NotContains(t, body, string(utf8.RuneError), "the cut must land on a rune boundary")
		})
	}
}

// TestMirrorFailureFields drives the field builder directly: a plain error is one bounded,
// scrubbed lastError; a typed status error, even wrapped, is taken apart into status, url
// and body, with readError and bodyTruncated only when they apply, and the body last so a
// body that imitates a field has no real field after it to displace.
func TestMirrorFailureFields(t *testing.T) {
	longLine := strings.Repeat("k", 300)
	statusErr := &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/hook", Status: "502 Bad Gateway", Body: `{"error":"x"}`}
	readErr := &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/hook", Status: "500 Internal Server Error", Body: "short", ReadErr: "unexpected EOF"}

	tests := []struct {
		name string
		err  error
		want map[string]interface{}
	}{
		{
			name: "plain error is one bounded scrubbed lastError",
			err:  errors.New("dial tcp: " + longLine + "\nWARN forged"),
			want: map[string]interface{}{"lastError": loggableText("dial tcp: "+longLine+"\nWARN forged", maxLoggableErrorLength)},
		},
		{
			name: "status error is taken apart",
			err:  statusErr,
			want: map[string]interface{}{"status": "502 Bad Gateway", "body": `{"error":"x"}`, "url": "https://mirror.example/hook"},
		},
		{
			name: "wrapped status error is still found",
			err:  fmt.Errorf("mirror: %w", statusErr),
			want: map[string]interface{}{"status": "502 Bad Gateway", "body": `{"error":"x"}`, "url": "https://mirror.example/hook"},
		},
		{
			name: "read failure adds readError",
			err:  readErr,
			want: map[string]interface{}{"status": "500 Internal Server Error", "body": "short", "readError": "unexpected EOF", "url": "https://mirror.example/hook"},
		},
		{
			name: "truncated body is flagged",
			err:  &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/hook", Status: "502 Bad Gateway", Body: "x", Truncated: true},
			want: map[string]interface{}{"status": "502 Bad Gateway", "body": "x", "bodyTruncated": true, "url": "https://mirror.example/hook"},
		},
		{
			name: "long url is bounded too",
			err:  &utils.HTTPStatusError{Method: "POST", URL: "https://mirror.example/" + longLine, Status: "502 Bad Gateway"},
			want: map[string]interface{}{"status": "502 Bad Gateway", "body": "", "url": loggableText("https://mirror.example/"+longLine, maxLoggableErrorLength)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := mirrorFailureFields(tt.err)
			require.Equal(t, 0, len(fields)%2, "fields come in key/value pairs")
			got := map[string]interface{}{}
			for i := 0; i < len(fields); i += 2 {
				got[fields[i].(string)] = fields[i+1]
			}
			assert.Equal(t, tt.want, got)
			if _, isStatus := got["status"]; isStatus {
				assert.Equal(t, "body", fields[len(fields)-2], "the body, the only server-written text, must be the last field")
			}
			if lastError, ok := got["lastError"].(string); ok {
				assert.NotContains(t, lastError, "\n")
				assert.LessOrEqual(t, len(lastError), maxLoggableErrorLength+loggedMarkerMax)
			}
		})
	}
}
