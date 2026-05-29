package slot_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-2357 (M5): blacklist reason must be a fixed enumerator, never the raw
// err.Error() which the attacker controls.
func TestWorker_BlacklistReasonOnInvalidConsensusMessage_IsSanitized(t *testing.T) {
	t.Parallel()

	const injectionPayload = "\r\n2026-05-03 INFO  fake admin login from 10.0.0.1\r\n\x1b[31malert: pwn\x1b[0m"
	errPoison := errors.New("unmarshal failed: " + injectionPayload)

	var (
		mu      sync.Mutex
		reasons []string
	)
	antifloodHandler := &cMock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(p2p.MessageP2P, core.PeerID) error { return nil },
		CanProcessMessagesOnTopicCalled: func(core.PeerID, string, uint32, uint64, []byte) error {
			return nil
		},
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			reasons = append(reasons, reason)
		},
	}

	workerArgs := createDefaultWorkerArgs()
	workerArgs.AntifloodHandler = antifloodHandler
	workerArgs.Marshalizer = &cMock.MarshalizerStub{
		UnmarshalCalled: func(_ interface{}, _ []byte) error { return errPoison },
	}

	wrk, err := slot.NewWorker(workerArgs)
	require.NoError(t, err)

	msg := &cMock.P2PMessageMock{
		DataField: []byte("payload"),
		PeerField: core.PeerID("originator"),
	}

	procErr := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	require.Equal(t, errPoison, procErr)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, reasons, 2)
	for i, r := range reasons {
		assert.Equal(t, process.BlacklistReasonInvalidConsensus, r)
		assert.NotContains(t, r, "fake admin login", "reason[%d] must not carry attacker err string", i)
		assert.False(t, strings.ContainsAny(r, "\r\n\x1b"), "reason[%d] must not contain control chars", i)
	}
}
