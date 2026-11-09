package websocket

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logArgCapture is a log observer and formatter in one: for lines with the given message
// it records the value that follows the given key, so a test asserts on what the logger
// actually emitted for that field.
type logArgCapture struct {
	message string
	key     string
	mut     sync.Mutex
	values  []string
}

func (c *logArgCapture) Write(p []byte) (int, error) { return len(p), nil }

func (c *logArgCapture) Output(line logger.LogLineHandler) []byte {
	if line.GetMessage() != c.message {
		return nil
	}
	// The logger carries argument values hex-encoded; decode so the assertions see the
	// text the operator would read.
	args := line.GetArgs()
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] != c.key {
			continue
		}
		value := args[i+1]
		if decoded, err := hex.DecodeString(value); err == nil {
			value = string(decoded)
		}
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

// TestMirrorFailureLogBoundsTheError pins the one line the mirror writes about a failed
// post: a mirror answering 500 with a 64 KiB body must not get that body into the node's
// log. The lastError field is bounded by loggableError (256 bytes plus its marker) and
// carries no line break, whatever the body held.
func TestMirrorFailureLogBoundsTheError(t *testing.T) {
	capture := &logArgCapture{message: postMirrorLogOp, key: "lastError"}
	require.NoError(t, logger.AddLogObserver(capture, capture))
	t.Cleanup(func() { _ = logger.RemoveLogObserver(capture) })

	hostileBody := strings.Repeat("X", 60000) + "\nWARN forged line\n" + strings.Repeat("Y", 5000)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(hostileBody))
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	stop := runHubServer(t, hub)
	defer stop()

	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})

	var lastError string
	require.Eventually(t, func() bool {
		v, ok := capture.first()
		lastError = v
		return ok
	}, 2*time.Second, 10*time.Millisecond, "the failed post must be logged")

	assert.LessOrEqual(t, len(lastError), 256+len("… (99999 bytes)"), "lastError must stay within loggableError's bound plus its marker")
	assert.Contains(t, lastError, "500 Internal Server Error", "the status must survive the bound")
	assert.Contains(t, lastError, " bytes)", "the cut must be marked with the original length")
	assert.NotContains(t, lastError, "\n", "no line break from the body may reach the log")
	assert.NotContains(t, lastError, "YYYY", "the tail of the body must not be logged")
}
