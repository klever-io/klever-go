package websocket

import (
	"time"
)

const (
	outChannelSize = 500
	maxWorkers     = 4

	// defaultMaxAddressesPerSubscribe caps addresses per subscribe call when unset.
	defaultMaxAddressesPerSubscribe = 10000

	// defaultMaxAddressesPerClient caps total addresses watched per connection when unset.
	defaultMaxAddressesPerClient = 50000

	// minMaxMessageSize is the floor (and default) for the inbound frame read limit;
	// gorilla's default is unlimited (GHSA-4fwh-wrm6-97xm, GAP#2). resolve raises it to
	// fit a larger per-subscribe address cap.
	minMaxMessageSize = 1 << 20 // 1 MiB

	// addressJSONOverhead is the per-address byte budget (address + JSON separators, with
	// headroom) used to derive the read limit from the address cap.
	addressJSONOverhead = 96

	// maxEncodedAddressLength caps each retained subscription address by byte size: a klv
	// bech32 address is 62 chars (matches the node's own encoded-address cap), so a longer
	// string can never match a real address and would only feed a per-connection
	// memory-amplification path (GHSA-4fwh-wrm6-97xm).
	maxEncodedAddressLength = 62

	// defaultPostWorkerCount bounds how many postWSConnection mirror requests run
	// concurrently, when PostWorkers is unset. Without a cap, a slow or unresponsive
	// mirror endpoint turns every account/tx event into an unbounded pile of in-flight
	// goroutines and sockets, scaling with chain throughput rather than with the mirror's
	// actual capacity.
	defaultPostWorkerCount = 8

	// defaultPostQueueSize bounds how many pending mirror sends can queue behind
	// postWorkerCount before new ones are dropped (mirroring the EventQueue drop-on-full
	// pattern in indexer/events.go), when PostQueueSize is unset — so a stalled mirror
	// endpoint sheds load instead of growing without bound.
	defaultPostQueueSize = 1000

	// defaultPingPeriod/defaultPongWait are the /subscribe keepalive timings when unset.
	// pongWait (the lifetime read deadline) must exceed pingPeriod so a live client can
	// answer a server ping before it elapses.
	defaultPingPeriod = 15 * time.Second
	defaultPongWait   = 30 * time.Second
)

// Limits bounds /subscribe resource usage. Zero-valued fields fall back to safe
// defaults, so a partial config can't disable the protection. The read limit is not a
// separate knob — it is derived from the per-subscribe cap (see resolve).
type Limits struct {
	MaxAddressesPerSubscribe int
	MaxAddressesPerClient    int
	// PostWorkers bounds concurrent postWSConnection mirror requests; 0 uses
	// defaultPostWorkerCount.
	PostWorkers int
	// PostQueueSize bounds pending mirror sends queued behind PostWorkers; 0 uses
	// defaultPostQueueSize.
	PostQueueSize int
	// PingPeriod/PongWait override the /subscribe keepalive timings (0 = defaults). A hub's
	// resolved values never change after construction, so client goroutines can read them
	// off c.hub.limits with no synchronization — tests that need short timings build a
	// dedicated hub instead of mutating shared state.
	PingPeriod time.Duration
	PongWait   time.Duration
}

// resolvedLimits holds the effective limits after defaults are applied.
type resolvedLimits struct {
	maxAddressesPerSubscribe int
	maxAddressesPerClient    int
	maxMessageSize           int64
	postWorkers              int
	postQueueSize            int
	pingPeriod               time.Duration
	pongWait                 time.Duration
}

func (l Limits) resolve() resolvedLimits {
	perSubscribe := l.MaxAddressesPerSubscribe
	if perSubscribe <= 0 {
		perSubscribe = defaultMaxAddressesPerSubscribe
	}
	perClient := l.MaxAddressesPerClient
	if perClient <= 0 {
		perClient = defaultMaxAddressesPerClient
	}
	// A per-connection cap below the per-call cap is incoherent (one max subscribe could
	// never fit); clamp it up so a fresh client's first subscribe always fits.
	if perClient < perSubscribe {
		perClient = perSubscribe
	}

	readLimit := int64(minMaxMessageSize)
	if derived := int64(perSubscribe)*addressJSONOverhead + 1024; derived > readLimit {
		readLimit = derived
	}

	postWorkers := l.PostWorkers
	if postWorkers <= 0 {
		postWorkers = defaultPostWorkerCount
	}
	postQueueSize := l.PostQueueSize
	if postQueueSize <= 0 {
		postQueueSize = defaultPostQueueSize
	}

	pingPeriod := l.PingPeriod
	if pingPeriod <= 0 {
		pingPeriod = defaultPingPeriod
	}
	pongWait := l.PongWait
	if pongWait <= 0 {
		pongWait = defaultPongWait
	}
	// An incoherent override (pongWait <= pingPeriod) is clamped up rather than left to
	// reclaim connections that are still answering pings on time.
	if pongWait <= pingPeriod {
		pongWait = pingPeriod * 2
	}

	return resolvedLimits{
		maxAddressesPerSubscribe: perSubscribe,
		maxAddressesPerClient:    perClient,
		maxMessageSize:           readLimit,
		postWorkers:              postWorkers,
		postQueueSize:            postQueueSize,
		pingPeriod:               pingPeriod,
		pongWait:                 pongWait,
	}
}
