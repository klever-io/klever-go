package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

	const n = 5000
	addresses := make([]string, n)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("klv-addr-%d", i)
	}

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, addresses, c))

	hub.mu.RLock()
	require.Equal(t, n, len(hub.addressSubscription))
	require.Equal(t, n, len(hub.clientAddrs[c]))
	hub.mu.RUnlock()

	// newTestClient has a nil conn and nil cancel; flip alive=false first so the
	// c.Close() inside handleClientDelete is a no-op.
	killClient(c)

	hub.handleClientDelete(c)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, len(hub.addressSubscription), "outer address keys must be reclaimed on disconnect")
	_, hasCount := hub.clientAddrs[c]
	assert.False(t, hasCount, "per-client address count must be cleared on disconnect")
}

func TestHandleClientInsertion_ClosedClientIsNotRetained(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1a"}, c))

	killClient(c)
	hub.handleClientDelete(c)

	err := hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS, indexer.BLOCKS}, []string{"klv1b"}, c)
	require.ErrorIs(t, err, ErrClientClosed)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, len(hub.addressSubscription), "a torn-down client must not be re-added to the address map")
	_, hasCount := hub.clientAddrs[c]
	assert.False(t, hasCount, "a torn-down client must not regain a per-client address count")
	_, hasBlocks := hub.blockSubscription[c]
	assert.False(t, hasBlocks, "a torn-down client must not be re-added to the blocks subscription")
	_, hasTxs := hub.transactionSubscription[c]
	assert.False(t, hasTxs, "a torn-down client must not be re-added to the transactions subscription")
}

// TestHandleClientInsertion_RejectsAfterHubShutdown covers the teardown path the
// per-client liveness gate cannot see: a client created but not yet inserted is unknown
// to the hub, so deleteAll never closes it and IsAlive() still reports true. Registering
// it into a hub whose StartServer loop has returned leaks the map entries (and the
// connection-limiter slot) for as long as the peer holds the socket.
func TestHandleClientInsertion_RejectsAfterHubShutdown(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.deleteAll()
	require.True(t, c.IsAlive(), "a client not yet in any subscription map survives deleteAll")

	err := hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS, indexer.BLOCKS}, []string{"klv1a"}, c)
	require.ErrorIs(t, err, ErrHubClosed)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 0, len(hub.addressSubscription), "a shut-down hub must not retain address subscriptions")
	_, hasCount := hub.clientAddrs[c]
	assert.False(t, hasCount, "a shut-down hub must not retain a per-client address count")
	_, hasBlocks := hub.blockSubscription[c]
	assert.False(t, hasBlocks, "a shut-down hub must not retain a blocks subscription")
}

// TestHandleDynamicSubscribe_HubShutdownReportsClientClosed pins the downgrade on the
// client-facing path: a subscribe that loses the race with hub teardown is answered with
// the state of the client's own connection, not with the node's shutdown state.
func TestHandleDynamicSubscribe_HubShutdownReportsClientClosed(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.deleteAll()

	params, _ := json.Marshal(SubscribeParams{Types: []string{"blocks"}, Addresses: []string{"klv1a"}})
	resp := sendRequest(hub, c, WSRequest{ID: "sub-closed", Method: MethodSubscribe, Params: params})

	assert.Equal(t, "sub-closed", resp.ID)
	assert.Equal(t, ErrClientClosed.Error(), resp.Error)
}

// TestDeleteAll_ClosesClientHeldInNoSubscriptionMap pins the client the subscription maps
// cannot see: an address-scoped subscribe with an empty address list (and, equally, an
// unsubscribe that empties the last one) writes to no map, so a shutdown that discovers
// clients by walking those maps leaves the socket, both client goroutines and the
// connection-limiter slot alive for as long as the peer holds the connection.
func TestDeleteAll_ClosesClientHeldInNoSubscriptionMap(t *testing.T) {
	hub := newTestHub(nil)
	c := NewClient(upgradedConn(t), hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, nil, c))

	hub.mu.RLock()
	require.Empty(t, hub.addressSubscription, "an empty address list must not create map entries")
	require.Empty(t, hub.blockSubscription)
	hub.mu.RUnlock()

	hub.deleteAll()

	assert.False(t, c.IsAlive(), "hub shutdown must close a client that holds no subscription entry")
}

func TestClosedClientIsNotRetainedInSubscriptions(t *testing.T) {
	const iterations = 300
	inserted := 0

	for i := 0; i < iterations; i++ {
		hub := newTestHub(nil)
		c := newTestClient(hub)

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start
			killClient(c)
			hub.handleClientDelete(c)
		}()

		var insertErr error
		go func() {
			defer wg.Done()
			<-start
			insertErr = hub.HandleClientInsertion(
				[]indexer.EventType{indexer.ACCOUNTS, indexer.USER_TRANSACTIONS, indexer.BLOCKS, indexer.TRANSACTIONS},
				[]string{"klv1a", "klv1b", "klv1c"},
				c,
			)
		}()

		close(start)
		wg.Wait()

		if insertErr == nil {
			inserted++
		}

		hub.mu.RLock()
		addressCount := len(hub.addressSubscription)
		_, hasCount := hub.clientAddrs[c]
		_, hasBlocks := hub.blockSubscription[c]
		_, hasTxs := hub.transactionSubscription[c]
		hub.mu.RUnlock()

		require.Equal(t, 0, addressCount, "iteration %d: address subscriptions must not outlive the client", i)
		require.False(t, hasCount, "iteration %d: per-client address count must not outlive the client", i)
		require.False(t, hasBlocks, "iteration %d: blocks subscription must not outlive the client", i)
		require.False(t, hasTxs, "iteration %d: transactions subscription must not outlive the client", i)
	}

	// The post-state above is empty under both interleavings, so it would also hold for a
	// hub that rejected every insertion. Requiring a winner keeps the insert-then-delete
	// ordering — the one that actually exercises handleClientDelete's cleanup — covered.
	//
	// The race itself decides whether that ordering ever happens, and the inserter does more
	// pre-lock work than the killer, so the bias runs against it. Rather than let coverage
	// depend on the scheduler, run that ordering once deterministically and count it.
	inserted += insertThenDeleteOnce(t)

	require.NotZero(t, inserted, "no insertion ever won the race, so only the rejection path was exercised")
}

// insertThenDeleteOnce runs the insert-then-delete ordering with no concurrency, so the
// racing test above has that path covered whatever the scheduler does. Returns 1 when the
// insertion succeeded, so it can be folded into the caller's count.
func insertThenDeleteOnce(t *testing.T) int {
	t.Helper()

	hub := newTestHub(nil)
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion(
		[]indexer.EventType{indexer.ACCOUNTS, indexer.USER_TRANSACTIONS, indexer.BLOCKS, indexer.TRANSACTIONS},
		[]string{"klv1a", "klv1b", "klv1c"},
		c,
	))

	killClient(c)
	hub.handleClientDelete(c)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	require.Empty(t, hub.addressSubscription, "address subscriptions must not outlive the client")
	require.NotContains(t, hub.clientAddrs, c, "per-client address count must not outlive the client")
	require.NotContains(t, hub.blockSubscription, c, "blocks subscription must not outlive the client")
	require.NotContains(t, hub.transactionSubscription, c, "transactions subscription must not outlive the client")

	return 1
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
	assert.Equal(t, 0, len(hub.clientAddrs[c]), "a rejected subscribe must not consume the address budget")
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
	assert.Equal(t, 1, len(hub.clientAddrs[c]))
}

func TestHandleClientInsertion_RejectsPerClientCap(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 5, MaxAddressesPerClient: 5})
	c := newTestClient(hub)

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "b", "c", "d", "e"}, c))

	hub.mu.RLock()
	require.Equal(t, 5, len(hub.clientAddrs[c]))
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
	assert.Equal(t, 3, len(hub.clientAddrs[c]), "a duplicated new address must count once")
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
	assert.Equal(t, 0, len(hub.clientAddrs[c]), "non-address-scoped subscribe must not consume the address budget")
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
	assert.Equal(t, defaultPingPeriod, r.pingPeriod)
	assert.Equal(t, defaultPongWait, r.pongWait)
}

func TestLimits_Resolve_OverridesPostWorkerLimits(t *testing.T) {
	r := Limits{PostWorkers: 3, PostQueueSize: 42}.resolve()
	assert.Equal(t, 3, r.postWorkers)
	assert.Equal(t, 42, r.postQueueSize)
}

func TestLimits_Resolve_OverridesKeepaliveTimings(t *testing.T) {
	r := Limits{PingPeriod: 20 * time.Millisecond, PongWait: 60 * time.Millisecond}.resolve()
	assert.Equal(t, 20*time.Millisecond, r.pingPeriod)
	assert.Equal(t, 60*time.Millisecond, r.pongWait)
}

func TestLimits_Resolve_ClampsIncoherentPongWait(t *testing.T) {
	// A pongWait at or below pingPeriod would reclaim a client that's still answering
	// pings on time; clamp it up instead of honoring the incoherent override.
	r := Limits{PingPeriod: 10 * time.Millisecond, PongWait: 5 * time.Millisecond}.resolve()
	assert.Equal(t, 10*time.Millisecond, r.pingPeriod)
	assert.Greater(t, r.pongWait, r.pingPeriod)
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
	require.Equal(t, 3, len(hub.clientAddrs[c]))
	hub.mu.RUnlock()

	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"a"}, c)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Equal(t, 2, len(hub.clientAddrs[c]), "removing an address must lower the per-client count")
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
	assert.Equal(t, 1, len(hub.clientAddrs[c]))
}

// TestClient_IdleConnectionReclaimedAtPongWait covers the core new defense
// (GHSA-4fwh-wrm6-97xm): a silent/dead client that never answers server pings must have
// its connection torn down when the lifetime read deadline (pongWait) elapses, so the
// per-connection slot the owner holds via Done() is released instead of leaking. Uses a
// dedicated hub with shortened keepalive timings (via Limits) instead of mutating shared
// state, so the read-deadline reclamation fires in milliseconds with nothing else to
// synchronize.
func TestClient_IdleConnectionReclaimedAtPongWait(t *testing.T) {
	hub := NewHub("", "", nil, Limits{PingPeriod: 20 * time.Millisecond, PongWait: 60 * time.Millisecond})
	upgrader := ws.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	released := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// t.Errorf (unlike Fatal/FailNow) is safe to call from any goroutine.
			t.Errorf("server failed to upgrade the websocket connection: %v", err)
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
	case <-time.After(2 * time.Second):
		t.Fatal("idle client was not reclaimed at pongWait; connection slot leaked")
	}
}

// upgradedConn returns the server side of a live websocket connection. The peer end is
// never read from and is closed with the test.
func upgradedConn(t *testing.T) *ws.Conn {
	t.Helper()
	conns := make(chan *ws.Conn, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&ws.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return // the dial below fails too, and that is where the test reports it
		}
		conns <- conn
	}))
	t.Cleanup(s.Close)

	peer, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	return <-conns
}

// newConnClient builds a client over conn without starting loopIn/loopOut, so a test can
// drive one loop synchronously.
func newConnClient(hub *SocketHub, conn *ws.Conn) *client {
	c := newTestClient(hub)
	c.conn = conn
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return c
}

// TestClient_LoopInReturnsWhenConnIsClosedBeforeStart pins the rejected-insertion race:
// processSubscription may Close() the client before loopIn is even scheduled, so its
// very first SetReadDeadline fails and the goroutine must tear the client down instead
// of entering the read loop.
func TestClient_LoopInReturnsWhenConnIsClosedBeforeStart(t *testing.T) {
	conn := upgradedConn(t)
	require.NoError(t, conn.Close())

	c := NewClient(conn, newTestHub(nil))
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("loopIn must tear the client down when its connection is already closed")
	}
	assert.False(t, c.IsAlive())
}

// TestClient_LoopOutClosesOnWriteFailure covers loopOut's failure exits: a write or a
// keepalive ping failing must tear the client down rather than leave it half-dead in
// the hub. (gorilla's SetWriteDeadline only stores the deadline and cannot fail, so that
// guard is not reachable from a test.)
func TestClient_LoopOutClosesOnWriteFailure(t *testing.T) {
	t.Run("write on a closed conn", func(t *testing.T) {
		conn := upgradedConn(t)
		require.NoError(t, conn.Close())
		c := newConnClient(newTestHub(nil), conn)
		c.out <- WSResponse{}

		assertReturnsQuickly(t, 2*time.Second, "loopOut must exit when WriteJSON fails", c.loopOut)
		assert.False(t, c.IsAlive())
	})

	t.Run("ping on a closed conn", func(t *testing.T) {
		hub := NewHub("", "", nil, Limits{PingPeriod: 20 * time.Millisecond, PongWait: 60 * time.Millisecond})
		conn := upgradedConn(t)
		require.NoError(t, conn.Close())
		c := newConnClient(hub, conn)

		assertReturnsQuickly(t, 2*time.Second, "loopOut must exit when the keepalive ping fails", c.loopOut)
		assert.False(t, c.IsAlive())
	})
}

// TestStartServer_ClearsClosedSoAHubCanBeRestarted pins that the shutdown flag does not
// outlive the shutdown that set it. deleteAll sets closed and nothing else clears it, so
// without this a hub whose context was cancelled would reject every later insertion with
// ErrHubClosed forever. No production caller restarts a hub today — api.go builds a fresh
// one per registration — but the type is exported and its own flush comment already
// reasons about hub reuse.
func TestStartServer_ClearsClosedSoAHubCanBeRestarted(t *testing.T) {
	hub := newTestHub(nil)

	// Joining the first StartServer rather than just waiting for closed is the point: the
	// documented contract is restart-after-return, not overlapping starts, so a test that
	// began the second run while the first was still tearing down would be asserting a
	// guarantee the code does not make.
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(stopped)
		hub.StartServer(ctx)
	}()
	cancel()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("StartServer did not return after its context was cancelled")
	}

	hub.mu.RLock()
	closedAfterShutdown := hub.closed
	hub.mu.RUnlock()
	require.True(t, closedAfterShutdown, "deleteAll must mark the hub closed on shutdown")

	require.ErrorIs(t,
		hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1a"}, newTestClient(hub)),
		ErrHubClosed,
		"a shut-down hub must reject insertions")

	restartCtx, stopRestart := context.WithCancel(context.Background())
	go hub.StartServer(restartCtx)

	c := newTestClient(hub)
	require.Eventually(t, func() bool {
		return hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1a"}, c) == nil
	}, time.Second, time.Millisecond, "a restarted hub must accept insertions again")

	// newTestClient has a nil conn, so it must not still be registered when the deferred
	// shutdown reaches deleteAll's c.Close() — the same invariant killClient exists for.
	killClient(c)
	hub.handleClientDelete(c)
	stopRestart()
}

// TestValidateSubscription_AppliesTheSameCapsAsInsertion pins that the exported validator
// routes.go calls up front and the check HandleClientInsertion repeats cannot drift apart.
func TestValidateSubscription_AppliesTheSameCapsAsInsertion(t *testing.T) {
	hub := newTestHub(nil)
	addressScoped := []indexer.EventType{indexer.ACCOUNTS}

	oversized := strings.Repeat("k", maxEncodedAddressLength+1)
	atCap := strings.Repeat("k", maxEncodedAddressLength)

	require.Error(t, hub.ValidateSubscription(addressScoped, []string{oversized}))
	require.NoError(t, hub.ValidateSubscription(addressScoped, []string{atCap}),
		"exactly the cap must still be accepted")

	// Insertion must agree, or the route would pre-accept something the hub then rejects.
	require.Error(t, hub.HandleClientInsertion(addressScoped, []string{oversized}, newTestClient(hub)))
	require.NoError(t, hub.HandleClientInsertion(addressScoped, []string{atCap}, newTestClient(hub)))

	// Addresses are meaningless for a blocks-only subscribe, so neither path inspects them.
	require.NoError(t, hub.ValidateSubscription([]indexer.EventType{indexer.BLOCKS}, []string{oversized}))
}

// TestUnexpectedClose_TreatsEveryPeerChosenCodeAsOrdinary pins the property that makes this
// predicate safe: the peer picks the code it closes with, so no close frame — whatever the
// code — may cost a log line. An earlier version listed the "ordinary" codes and warned on
// the rest, which only moved the lever: a peer that wants a log line just closes with 1002
// or 4000 instead of 1000. The codes below are exactly that bypass.
func TestUnexpectedClose_TreatsEveryPeerChosenCodeAsOrdinary(t *testing.T) {
	peerChosen := map[string]int{
		"normal closure":     ws.CloseNormalClosure,
		"going away":         ws.CloseGoingAway,
		"no status received": ws.CloseNoStatusReceived,
		// synthesised locally when the peer vanishes without a close frame
		"abnormal closure": ws.CloseAbnormalClosure,
		// the bypass codes: nothing stops a peer choosing these
		"protocol error":   ws.CloseProtocolError,
		"message too big":  ws.CloseMessageTooBig,
		"internal error":   ws.CloseInternalServerErr,
		"application code": 4000,
	}
	for name, code := range peerChosen {
		require.Falsef(t, unexpectedClose(&ws.CloseError{Code: code}),
			"%s is peer-chosen, so it must not cost a log line", name)
	}

	// A read that fails with no close frame at all is the only thing left that can say
	// something about this node — and the call site still rate-limits even that.
	require.True(t, unexpectedClose(errors.New("read tcp: connection reset by peer")),
		"a failure that is not a close frame must still be reportable")
}

// TestLoggableHash_KeepsPeerContentOutOfTheLog pins the sanitiser on the one peer-supplied
// string that reached a log line raw. It is capped only by the inbound message limit, so
// without this a query could write a megabyte into the log — or, worse, embed newlines and
// forge log entries of its own.
func TestLoggableHash_KeepsPeerContentOutOfTheLog(t *testing.T) {
	realHash := "a3f1" + strings.Repeat("0", 60)
	require.Len(t, realHash, 64)
	require.Equal(t, realHash, loggableHash(realHash), "a well-formed hash must still be readable")
	require.Equal(t, "ABCDEF0123", loggableHash("ABCDEF0123"), "upper-case hex is a hash too")
	require.Equal(t, "", loggableHash(""), "an empty hash carries nothing to sanitise")

	// Anything a hash cannot be must not reach the log as bytes.
	forged := "deadbeef\nWARN  everything is fine, nothing to see here"
	require.NotContains(t, loggableHash(forged), "\n",
		"a newline must never survive into a log line")
	require.NotContains(t, loggableHash(forged), "nothing to see here")

	require.Equal(t, "<1048576 bytes>", loggableHash(strings.Repeat("a", 1<<20)),
		"an oversized value must be reduced to its length")
	require.Equal(t, "<5 bytes, not hex>", loggableHash("zzzzz"))
}

// TestPeerDrivenBudgets_EachSourceKeepsItsOwn is the discriminating version of a test I got
// wrong twice. Asserting on the counters directly, or driving every source the same number
// of times, passes against an implementation that routes them all to one budget or swaps
// two of them. Every source is driven through its own logging method a distinct number of
// times, so each destination's count identifies which callers reached it.
//
// It matters because only the caller that opens a window has its message logged. If two
// sources share a budget, a flood of the cheap one hides the identity of the rare one; if
// two are swapped, a summary is attributed to the wrong subsystem.
func TestPeerDrivenBudgets_EachSourceKeepsItsOwn(t *testing.T) {
	hub := newTestHub(nil)

	boom := errors.New("boom")
	drive := map[string]struct {
		warner *dropWarner
		times  int
		call   func()
	}{
		"upgrade":   {&hub.upgradeFailWarn, 3, func() { hub.LogUpgradeFailure("ws.Subscribe", boom) }},
		"handshake": {&hub.handshakeFailWarn, 5, func() { hub.LogHandshakeFailure("ws.Subscribe", boom) }},
		"read":      {&hub.readFailWarn, 7, func() { hub.logReadFailure("ws.loopIn", boom) }},
		"sendDrop":  {&hub.sendDropWarn, 11, func() { hub.logSendDrop("ws.send", "buffer full") }},
		"write":     {&hub.writeFailWarn, 13, func() { hub.logWriteFailure("ws.loopOut", boom) }},
		"query":     {&hub.queryFailWarn, 17, func() { hub.logQueryFailure("ws.handleGetBlock", "hash", "abc", boom) }},
	}

	// Open every window first, so each subsequent call folds instead of reporting.
	for name, source := range drive {
		_, reported := source.warner.fire()
		require.Truef(t, reported, "%s: the first occurrence must open the window", name)
	}

	for _, source := range drive {
		for i := 0; i < source.times; i++ {
			source.call()
		}
	}

	// Distinct counts, so a budget holding the wrong number names the miswiring.
	for name, source := range drive {
		count, pending := source.warner.flush()
		require.Truef(t, pending, "%s: folded occurrences must still be pending", name)
		require.EqualValuesf(t, source.times, count,
			"%s budget holds %d occurrences, expected %d — a source is wired to the wrong budget",
			name, count, source.times)
	}
}

// TestLoggableError_KeepsPeerContentOutOfTheLog covers the hole the hash sanitiser alone
// left open. Bounding the fields we choose to log is not enough, because the error text
// carries peer input back out by itself.
func TestLoggableError_KeepsPeerContentOutOfTheLog(t *testing.T) {
	// The real shape: a storage miss quotes the whole key it was handed, so a 100KB hash
	// reaches the log through err even though loggableHash reduced the hash field.
	hugeHash := strings.Repeat("ab", 50_000)
	storageMiss := fmt.Errorf("key %s not found in %s", hugeHash, "TransactionsUnit")
	rendered := loggableError(storageMiss)
	require.NotContains(t, rendered, hugeHash, "the full key must not survive into the log")
	require.Less(t, len(rendered), 400, "a bounded error must stay bounded")
	require.Contains(t, rendered, fmt.Sprintf("(%d bytes)", len(storageMiss.Error())),
		"the size that was dropped must still be stated")

	// A websocket close reason arrives verbatim in CloseError.Error(); the logger passes
	// CR/LF through, so a newline here would forge a log line of its own.
	forging := &ws.CloseError{Code: ws.CloseNormalClosure, Text: "\nWARN  all clear, nothing to see"}
	rendered = loggableError(forging)
	require.NotContains(t, rendered, "\n", "a newline must never survive into a log line")
	require.NotContains(t, rendered, "\r")
	require.Contains(t, rendered, "all clear", "the text itself stays readable")

	// The Unicode line and paragraph separators are not control runes, and some log
	// aggregators break lines on them just the same.
	separators := loggableError(errors.New("before\u2028WARN forged\u2029after"))
	require.NotContains(t, separators, "\u2028")
	require.NotContains(t, separators, "\u2029")

	require.Equal(t, "", loggableError(nil))
}

// TestUnexpectedClose_ANodeSideCloseIsNotAReadFailure pins the one non-close-frame error
// that must not count: our own Close() makes the blocked ReadMessage return net.ErrClosed,
// which is not a *CloseError. Counting it billed the read budget for every shutdown, every
// write failure and every rejected insert — the cross-contamination the per-source budgets
// exist to prevent, reintroduced from the inside. The socket is real so the error is the
// one gorilla actually produces, not a hand-built stand-in.
func TestUnexpectedClose_ANodeSideCloseIsNotAReadFailure(t *testing.T) {
	readErrs := make(chan error, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&ws.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			_, _, rerr := conn.ReadMessage()
			readErrs <- rerr
		}()
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close() // the node's side, as client.Close() does
	}))
	defer s.Close()

	peer, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http"), nil)
	require.NoError(t, err)
	defer peer.Close()

	rerr := <-readErrs
	require.ErrorIs(t, rerr, net.ErrClosed, "a local close must surface as net.ErrClosed")
	require.False(t, unexpectedClose(rerr),
		"a close the node performed itself must not be counted as a read failure")

	// A peer that simply goes quiet still is: an idle read deadline is a genuine signal,
	// and it stays on the read budget where it is bounded.
	require.True(t, unexpectedClose(errors.New("i/o timeout")))
}

// TestNewDropWarner_ClampsNonPositiveWindow pins the constructor: the zero value of dropWarner is
// not a rate limiter, since a zero window lets every occurrence through, so a caller that passes
// a zero or sub-second window must still get one that suppresses.
func TestNewDropWarner_ClampsNonPositiveWindow(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(1), NewDropWarner(0).w.windowSecs, "a zero window would let every occurrence through")
	assert.Equal(t, int64(1), NewDropWarner(-time.Second).w.windowSecs)
	assert.Equal(t, int64(1), NewDropWarner(500*time.Millisecond).w.windowSecs)
	assert.Equal(t, int64(10), NewDropWarner(10*time.Second).w.windowSecs)
}
