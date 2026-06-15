package websocket

import (
	"math"
	"sync"
)

// connLimiter caps the number of simultaneous live /subscribe WebSocket connections,
// both node-wide and per source IP. The gin global throttler cannot do this: it
// releases its slot at the HTTP->WS upgrade, so live connections are uncounted
// (GHSA-4fwh-wrm6-97xm, GAP#3). A slot is acquired before the upgrade and released
// exactly once when the connection is torn down.
//
// A zero maximum means "unlimited" for that dimension. Per-IP must be tunable to 0
// because behind a reverse proxy every client shares the proxy's source IP.
type connLimiter struct {
	mu        sync.Mutex
	global    int
	maxGlobal int
	perIP     map[string]int
	maxPerIP  int
}

func newConnLimiter(maxGlobal, maxPerIP uint32) *connLimiter {
	return &connLimiter{
		maxGlobal: clampUint32ToInt(maxGlobal),
		maxPerIP:  clampUint32ToInt(maxPerIP),
		perIP:     make(map[string]int),
	}
}

// clampUint32ToInt converts a uint32 cap to int without the negative wrap a direct
// conversion would cause on 32-bit platforms — a negative max would silently DISABLE
// the cap (treated as unlimited) instead of enforcing it.
func clampUint32ToInt(v uint32) int {
	if uint64(v) > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(v)
}

// acquire reserves a connection slot for ip. It returns an idempotent release func
// and true on success, or nil and false when a cap is reached.
func (l *connLimiter) acquire(ip string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.maxGlobal > 0 && l.global >= l.maxGlobal {
		return nil, false
	}
	if l.maxPerIP > 0 && l.perIP[ip] >= l.maxPerIP {
		return nil, false
	}

	l.global++
	if l.maxPerIP > 0 {
		l.perIP[ip]++
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.global--
			if l.maxPerIP > 0 {
				if l.perIP[ip] <= 1 {
					delete(l.perIP, ip)
				} else {
					l.perIP[ip]--
				}
			}
		})
	}, true
}
