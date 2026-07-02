package websocket

import "time"

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
)

// pingPeriod and pongWait are the /subscribe keepalive timings. They are vars only so
// tests can shorten them to exercise the idle-client read-deadline reclamation quickly;
// nothing in production mutates them. pongWait (the lifetime read deadline) must exceed
// pingPeriod so a live client can answer a server ping before it elapses.
var (
	pingPeriod = 15 * time.Second
	pongWait   = 30 * time.Second
)

// Limits bounds /subscribe resource usage. Zero-valued fields fall back to safe
// defaults, so a partial config can't disable the protection. The read limit is not a
// separate knob — it is derived from the per-subscribe cap (see resolve).
type Limits struct {
	MaxAddressesPerSubscribe int
	MaxAddressesPerClient    int
}

// resolvedLimits holds the effective limits after defaults are applied.
type resolvedLimits struct {
	maxAddressesPerSubscribe int
	maxAddressesPerClient    int
	maxMessageSize           int64
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

	return resolvedLimits{
		maxAddressesPerSubscribe: perSubscribe,
		maxAddressesPerClient:    perClient,
		maxMessageSize:           readLimit,
	}
}
