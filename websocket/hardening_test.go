package websocket

import (
	"fmt"
	"testing"

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
