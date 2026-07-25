package websocket

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleClientDelete_ReclaimsAddressKeys is the regression test for the Impact C
// leak (GHSA-4fwh-wrm6-97xm): a disconnecting client must release every address-map
// outer key it created, not just remove itself from the inner maps.
func TestHandleClientDelete_ReclaimsAddressKeys(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	// Mark the client dead before inserting: newTestClient has a nil conn, so the
	// c.close() inside handleClientDelete must be a no-op. killClient flips alive=false
	// so close() skips the (nil) conn, letting us drive handleClientDelete directly.
	killClient(c)

	const n = 5000
	addresses := make([]string, n)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("klv-addr-%d", i)
	}

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, addresses, c))

	hub.mu.RLock()
	require.Equal(t, n, len(hub.addressSubscription))
	require.Equal(t, n, hub.clientAddresses[c])
	hub.mu.RUnlock()

	hub.handleClientDelete(c)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, len(hub.addressSubscription), "outer address keys must be reclaimed on disconnect")
	_, hasCount := hub.clientAddresses[c]
	assert.False(t, hasCount, "per-client address count must be cleared on disconnect")
}

func TestHandleClientInsertion_RejectsOversizedSubscribe(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3})
	c := newTestClient(hub)

	err := hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "b", "c", "d"}, c)
	require.Error(t, err)

	hub.mu.RLock()
	assert.Equal(t, 0, len(hub.addressSubscription), "rejected subscribe must not mutate the hub")
	hub.mu.RUnlock()
}

// TestHandleClientInsertion_RejectsOversizedAddress guards the memory-amplification path
// (GHSA-4fwh-wrm6-97xm): an address longer than a real klv bech32 address can never match,
// so it must be rejected before it is retained as a subscription key rather than counting
// only toward the count caps.
func TestHandleClientInsertion_RejectsOversizedAddress(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	oversized := strings.Repeat("a", maxEncodedAddressLength+1)
	err := hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{oversized}, c)
	require.Error(t, err)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, len(hub.addressSubscription), "an oversized address must not be retained as a key")
	assert.Equal(t, 0, hub.clientAddresses[c], "a rejected subscribe must not consume the address budget")
}

// TestHandleClientInsertion_AcceptsMaxLengthAddress confirms the length cap is inclusive:
// a real, full-length (62-char) address is accepted.
func TestHandleClientInsertion_AcceptsMaxLengthAddress(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	addr := strings.Repeat("a", maxEncodedAddressLength)
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{addr}, c))

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 1, hub.clientAddresses[c])
}

func TestHandleClientInsertion_RejectsPerClientCap(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 5, MaxAddressesPerClient: 5})
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "b", "c", "d", "e"}, c))

	hub.mu.RLock()
	require.Equal(t, 5, hub.clientAddresses[c])
	hub.mu.RUnlock()

	// One more new address (across a second call) must be rejected.
	err := hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"one-too-many"}, c)
	require.Error(t, err)

	hub.mu.RLock()
	_, exists := hub.addressSubscription["one-too-many"]
	hub.mu.RUnlock()
	assert.False(t, exists)
}

// TestHandleClientInsertion_DuplicateAddressesCountOnce guards the per-client cap
// gate: a duplicated NEW address in one call must count once, so a client near the cap
// is not falsely rejected. Cap 3, already holding 2; a {"x","x"} call adds one (total
// 3, accepted) — counting the duplicate twice would wrongly reject it as 4.
func TestHandleClientInsertion_DuplicateAddressesCountOnce(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3, MaxAddressesPerClient: 3})
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "b"}, c))
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"x", "x"}, c))

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 3, hub.clientAddresses[c], "a duplicated new address must count once")
}

// TestHandleClientInsertion_NonAddressScopedIgnoresAddresses verifies that a
// blocks/transactions-only subscribe neither counts nor stores its addresses:
// such entries would never match (both accept flags false) yet would otherwise
// burn the per-connection address budget (GHSA-4fwh-wrm6-97xm, Impact C).
func TestHandleClientInsertion_NonAddressScopedIgnoresAddresses(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3, MaxAddressesPerClient: 3})
	c := newTestClient(hub)

	// BLOCKS-only with an oversized address list attached: addresses are irrelevant
	// here, so neither the per-subscribe cap nor the per-client budget applies.
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.BLOCKS}, []string{"a", "b", "c", "d", "e"}, c))

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, hub.clientAddresses[c], "non-address-scoped subscribe must not consume the address budget")
	assert.Equal(t, 0, len(hub.addressSubscription), "non-address-scoped subscribe must not store addresses")
	_, subscribed := hub.blockSubscription[c]
	assert.True(t, subscribed, "the global BLOCKS subscription must still be registered")
}

func TestLimits_Resolve_ClampsPerClientToPerSubscribe(t *testing.T) {
	// An incoherent config (per-connection cap below per-call cap) is clamped up so a
	// single maximal subscribe always fits the per-connection budget.
	r := Limits{MaxAddressesPerSubscribe: 100, MaxAddressesPerClient: 10}.resolve()
	assert.Equal(t, 100, r.maxAddressesPerSubscribe)
	assert.Equal(t, 100, r.maxAddressesPerClient, "per-client cap clamped up to per-subscribe")
}

func TestLimits_Resolve_AppliesDefaults(t *testing.T) {
	r := Limits{}.resolve()
	assert.Equal(t, defaultMaxAddressesPerSubscribe, r.maxAddressesPerSubscribe)
	assert.Equal(t, defaultMaxAddressesPerClient, r.maxAddressesPerClient)
	assert.Equal(t, int64(minMaxMessageSize), r.maxMessageSize, "default read limit is the floor")
	assert.Equal(t, defaultPostWorkerCount, r.postWorkers)
	assert.Equal(t, defaultPostQueueSize, r.postQueueSize)
}

func TestLimits_Resolve_OverridesPostWorkerLimits(t *testing.T) {
	r := Limits{PostWorkers: 3, PostQueueSize: 42}.resolve()
	assert.Equal(t, 3, r.postWorkers)
	assert.Equal(t, 42, r.postQueueSize)
}

func TestLimits_Resolve_DerivesReadLimitFromAddressCap(t *testing.T) {
	// A per-subscribe cap large enough to exceed the floor must raise the read limit so
	// a maximal subscribe still fits while staying bounded.
	r := Limits{MaxAddressesPerSubscribe: 100000}.resolve()
	assert.Equal(t, 100000, r.maxAddressesPerSubscribe)
	assert.Equal(t, int64(100000)*addressJSONOverhead+1024, r.maxMessageSize)
	assert.Greater(t, r.maxMessageSize, int64(minMaxMessageSize))
}

func TestClientAddresses_DecrementOnRemoval(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "b", "c"}, c))
	hub.mu.RLock()
	require.Equal(t, 3, hub.clientAddresses[c])
	hub.mu.RUnlock()

	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"a"}, c)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 2, hub.clientAddresses[c], "removing an address must lower the per-client count")
	_, hasA := hub.addressSubscription["a"]
	assert.False(t, hasA, "fully unsubscribed address key must be reclaimed")
}

// TestHandleClientInsertion_ReSubscribeDoesNotDoubleCount ensures re-subscribing to an
// address already watched by the client does not inflate the per-client count.
func TestHandleClientInsertion_ReSubscribeDoesNotDoubleCount(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"x"}, c))
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.USER_TRANSACTIONS}, []string{"x"}, c))

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 1, hub.clientAddresses[c])
}

// setKeepaliveForTest shortens the /subscribe keepalive timings so the read-deadline
// reclamation fires in milliseconds, and returns a restore func. Swaps the (ping, pong)
// pair as a single atomic pointer rather than two independent stores, because a client
// whose teardown this test just triggered may still have its loopIn/loopOut goroutines
// mid-exit and reading the current value (e.g. loopOut's ticker branch) when the restore
// fires — two separate stores would let such a reader observe a torn combination of an
// old ping with a new pong (or vice versa).
func setKeepaliveForTest(ping, pong time.Duration) func() {
	orig := keepalive.Swap(&keepaliveTimings{ping: ping, pong: pong})
	return func() {
		keepalive.Store(orig)
	}
}

// TestClient_IdleConnectionReclaimedAtPongWait covers the core new defense
// (GHSA-4fwh-wrm6-97xm): a silent/dead client that never answers server pings must have
// its connection torn down when the lifetime read deadline (pongWait) elapses, so the
// per-connection slot the owner holds via Done() is released instead of leaking.
func TestClient_IdleConnectionReclaimedAtPongWait(t *testing.T) {
	defer setKeepaliveForTest(20*time.Millisecond, 60*time.Millisecond)()

	hub := newTestHub(nil)
	upgrader := ws.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	released := make(chan struct{})
	// Buffered so the handler goroutine (not the test goroutine — calling t.Fatal there
	// would violate testing's FailNow-must-run-on-the-test's-own-goroutine contract)
	// never blocks reporting an upgrade failure back to the test.
	upgradeErrCh := make(chan error, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			upgradeErrCh <- err
			return
		}
		c := NewClient(conn, hub)
		// Mirror processSubscription: the owner blocks on Done() and frees the slot at teardown.
		go func() {
			<-c.Done()
			close(released)
		}()
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Never read from the connection: gorilla only answers server pings while the app reads,
	// so a client that never reads is indistinguishable from a dead one and never pongs.
	select {
	case <-released:
		// Read deadline elapsed and reclaimed the idle client — the slot is freed.
	case err := <-upgradeErrCh:
		t.Fatalf("server failed to upgrade the websocket connection: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("idle client was not reclaimed at pongWait; connection slot leaked")
	}
}
