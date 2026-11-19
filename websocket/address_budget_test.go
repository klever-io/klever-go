package websocket

import (
	"maps"
	"net"
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var accounts = []indexer.EventType{indexer.ACCOUNTS}

func ipClient(hub *SocketHub, ip string) *client {
	c := newTestClient(hub)
	c.ipKey = ip
	return c
}

// liveIPClient can go through handleClientDelete, which closes the conn.
func liveIPClient(t *testing.T, hub *SocketHub, ip string) *client {
	c := newConnClient(hub, upgradedConn(t))
	c.ipKey = ip
	return c
}

type budgetState struct {
	total     int
	perIP     map[string]int
	perClient int
	addrKeys  int
}

func snapshotBudget(hub *SocketHub, c *client) budgetState {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return budgetState{total: hub.totalAddresses, perIP: maps.Clone(hub.ipAddresses), perClient: len(hub.clientAddrs[c]), addrKeys: len(hub.addressSubscription)}
}

func TestAddressBudget_PerIPCapsClientsFromOneIP(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 4, MaxTotalAddresses: 100})
	c1, c2, c3 := ipClient(hub, "1.2.3.4"), ipClient(hub, "1.2.3.4"), ipClient(hub, "1.2.3.4")

	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, c1))
	// The same addresses on another connection are new pairs and count again.
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, c2))

	before := snapshotBudget(hub, c3)
	err := hub.HandleClientInsertion(accounts, []string{"c"}, c3)
	require.ErrorIs(t, err, ErrPerIPAddressBudgetExhausted)
	assert.Equal(t, before, snapshotBudget(hub, c3), "a refused insertion must leave every counter unchanged")
	hub.mu.RLock()
	_, stored := hub.addressSubscription["c"]
	_, registered := hub.clients[c3]
	hub.mu.RUnlock()
	assert.False(t, stored)
	assert.False(t, registered)

	other := ipClient(hub, "5.6.7.8")
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"c"}, other), "another IP has its own budget")

	s := snapshotBudget(hub, other)
	assert.Equal(t, 5, s.total)
	assert.Equal(t, map[string]int{"1.2.3.4": 4, "5.6.7.8": 1}, s.perIP)
}

func TestAddressBudget_GlobalCapRefusesPastTotal(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 4})

	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, "10.0.0.1")))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, "10.0.0.2")))

	c := ipClient(hub, "10.0.0.3")
	before := snapshotBudget(hub, c)
	err := hub.HandleClientInsertion(accounts, []string{"z"}, c)
	require.ErrorIs(t, err, ErrGlobalAddressBudgetExhausted)
	assert.Equal(t, before, snapshotBudget(hub, c), "a refused insertion must leave every counter unchanged")
	assert.Equal(t, 4, before.total)
}

func TestAddressBudget_CountersReleaseOnUnsubscribeAndTeardown(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3, MaxAddressesPerClient: 3, MaxAddressesPerIP: 5, MaxTotalAddresses: 6})
	c1, c2, c3 := liveIPClient(t, hub, "1.1.1.1"), liveIPClient(t, hub, "1.1.1.1"), liveIPClient(t, hub, "2.2.2.2")

	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS, indexer.USER_TRANSACTIONS}, []string{"a", "b", "c"}, c1))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, c2))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"x"}, c3))
	s := snapshotBudget(hub, c1)
	require.Equal(t, 6, s.total)
	require.Equal(t, map[string]int{"1.1.1.1": 5, "2.2.2.2": 1}, s.perIP)

	// Dropping one of two accept flags keeps the pair, so nothing is released.
	hub.HandleClientRemoval([]indexer.EventType{indexer.USER_TRANSACTIONS}, []string{"a"}, c1)
	assert.Equal(t, 6, snapshotBudget(hub, c1).total)

	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"a"}, c1)
	// Unknown or already-removed addresses release nothing.
	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"a", "nope"}, c1)
	s = snapshotBudget(hub, c1)
	assert.Equal(t, 5, s.total)
	assert.Equal(t, map[string]int{"1.1.1.1": 4, "2.2.2.2": 1}, s.perIP)

	hub.RemoveClient(c1)
	hub.RemoveClient(c1) // loopIn teardown can delete the same client again
	s = snapshotBudget(hub, c1)
	assert.Equal(t, 3, s.total)
	assert.Equal(t, map[string]int{"1.1.1.1": 2, "2.2.2.2": 1}, s.perIP)

	// The released budget is usable again.
	c4 := liveIPClient(t, hub, "1.1.1.1")
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"p", "q", "r"}, c4))
	hub.RemoveClient(c4)

	hub.handleClientDelete(c2)
	hub.handleClientDelete(c3)
	s = snapshotBudget(hub, c1)
	assert.Equal(t, 0, s.total)
	assert.Empty(t, s.perIP, "a fully torn-down IP must not keep a map entry")
	assert.Zero(t, s.addrKeys)
}

func TestAddressBudget_DeleteAllClearsCounters(t *testing.T) {
	hub := newTestHub(nil)
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, liveIPClient(t, hub, "1.1.1.1")))
	require.Equal(t, 2, snapshotBudget(hub, nil).total)

	hub.deleteAll()

	s := snapshotBudget(hub, nil)
	assert.Equal(t, 0, s.total)
	assert.Empty(t, s.perIP)
}

func TestRemoteIPKey_BucketsIPv6By56(t *testing.T) {
	a := remoteIPKey(&net.TCPAddr{IP: net.ParseIP("2001:db8:1:201::1"), Port: 1111})
	b := remoteIPKey(&net.TCPAddr{IP: net.ParseIP("2001:db8:1:2ff:ffff::9"), Port: 2222})
	assert.Equal(t, "2001:db8:1:200::", a)
	assert.Equal(t, a, b, "two /64s inside one /56 are one address budget")
	assert.NotEqual(t, shared.IPBucket("2001:db8:1:201::1"), shared.IPBucket("2001:db8:1:2ff::9"), "the connection limiter still keys them apart")
	assert.NotEqual(t, a, remoteIPKey(&net.TCPAddr{IP: net.ParseIP("2001:db8:1:301::1"), Port: 1}))

	assert.Equal(t, "192.0.2.7", remoteIPKey(&net.TCPAddr{IP: net.ParseIP("192.0.2.7"), Port: 1}))
	assert.Equal(t, "192.0.2.7", remoteIPKey(&net.TCPAddr{IP: net.ParseIP("::ffff:192.0.2.7"), Port: 2}))
}

func TestAddressBudget_PerIPSharedAcrossOneIPv6Slash56(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 100})
	first := &net.TCPAddr{IP: net.ParseIP("2001:db8:1:201::1"), Port: 1}
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, remoteIPKey(first))))

	sibling := &net.TCPAddr{IP: net.ParseIP("2001:db8:1:202::1"), Port: 1}
	require.ErrorIs(t, hub.PrecheckAddressBudget(accounts, []string{"c"}, sibling), ErrPerIPAddressBudgetExhausted)
	require.ErrorIs(t, hub.HandleClientInsertion(accounts, []string{"c"}, ipClient(hub, remoteIPKey(sibling))), ErrPerIPAddressBudgetExhausted)

	outside := &net.TCPAddr{IP: net.ParseIP("2001:db8:1:301::1"), Port: 1}
	require.NoError(t, hub.PrecheckAddressBudget(accounts, []string{"c"}, outside))
}

func TestAddressBudget_RefusalsHideTheCaps(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 3})
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, "1.2.3.4")))

	perIP := hub.HandleClientInsertion(accounts, []string{"c"}, ipClient(hub, "1.2.3.4"))
	global := hub.HandleClientInsertion(accounts, []string{"c", "d"}, ipClient(hub, "5.6.7.8"))
	assert.True(t, perIP == ErrPerIPAddressBudgetExhausted, "got %v", perIP)
	assert.True(t, global == ErrGlobalAddressBudgetExhausted, "got %v", global)
	assert.NotErrorIs(t, perIP, ErrGlobalAddressBudgetExhausted)
	assert.NotErrorIs(t, global, ErrPerIPAddressBudgetExhausted)
	for _, err := range []error{perIP, global} {
		assert.Equal(t, "address subscription budget reached", err.Error(), "peers must not learn the cap or which budget is full")
	}
}

func TestLogRejectedInsertion_NamesTheBudget(t *testing.T) {
	var observed lockedBuffer
	require.NoError(t, logger.AddLogObserver(&observed, &logger.PlainFormatter{}))
	t.Cleanup(func() { assert.NoError(t, logger.RemoveLogObserver(&observed)) })

	newTestHub(nil).LogRejectedInsertion("ws.budgetProbe", ErrPerIPAddressBudgetExhausted)
	newTestHub(nil).LogRejectedInsertion("ws.budgetProbe", ErrGlobalAddressBudgetExhausted)

	out := observed.String()
	assert.Contains(t, out, "per-ip")
	assert.Contains(t, out, "node-wide")
}

// requireIndexConsistent checks addr is in clientAddrs[c] iff c is in addressSubscription[addr].
func requireIndexConsistent(t *testing.T, hub *SocketHub) {
	t.Helper()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	pairs := 0
	for addr, clients := range hub.addressSubscription {
		require.NotEmpty(t, clients, "an emptied address key must be reclaimed")
		for c := range clients {
			_, ok := hub.clientAddrs[c][addr]
			require.True(t, ok, "pair (%s, %p) missing from the client index", addr, c)
			pairs++
		}
	}
	indexed := 0
	for c, addrs := range hub.clientAddrs {
		require.NotEmpty(t, addrs, "an emptied client index must be dropped")
		for addr := range addrs {
			_, ok := hub.addressSubscription[addr][c]
			require.True(t, ok, "indexed pair (%s, %p) missing from addressSubscription", addr, c)
			indexed++
		}
	}
	require.Equal(t, pairs, indexed)
	require.Equal(t, pairs, hub.totalAddresses)
}

func TestHandleClientDelete_RemovesOnlyThatClientsPairs(t *testing.T) {
	hub := newTestHub(nil)
	c1, c2 := liveIPClient(t, hub, "1.1.1.1"), liveIPClient(t, hub, "2.2.2.2")
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS, indexer.USER_TRANSACTIONS}, []string{"shared", "only1", "only1b"}, c1))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"shared", "only2"}, c2))
	hub.HandleClientRemoval(accounts, []string{"only1b"}, c1) // keeps the pair: USER_TRANSACTIONS remains
	requireIndexConsistent(t, hub)

	hub.handleClientDelete(c1)
	requireIndexConsistent(t, hub)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for addr, clients := range hub.addressSubscription {
		_, has := clients[c1]
		assert.False(t, has, "teardown left c1 on %s", addr)
	}
	assert.NotContains(t, hub.addressSubscription, "only1")
	assert.NotContains(t, hub.addressSubscription, "only1b")
	assert.Equal(t, map[*client]userOptions{c2: {acceptAccount: true}}, hub.addressSubscription["shared"])
	assert.Equal(t, map[*client]userOptions{c2: {acceptAccount: true}}, hub.addressSubscription["only2"])
	assert.NotContains(t, hub.clientAddrs, c1)
	assert.Equal(t, map[string]struct{}{"shared": {}, "only2": {}}, hub.clientAddrs[c2])
	assert.Equal(t, map[string]int{"2.2.2.2": 2}, hub.ipAddresses)
}

func TestNewClient_SetsIPKeyFromRemoteAddr(t *testing.T) {
	_, c := dialLiveClient(t, newTestHub(nil))
	t.Cleanup(c.Close)
	assert.Equal(t, "127.0.0.1", c.ipKey)
}

func TestLimits_Resolve_AddressBudgetDefaultsAndClamps(t *testing.T) {
	r := Limits{}.resolve()
	assert.Equal(t, defaultMaxAddressesPerIP, r.maxAddressesPerIP)
	assert.Equal(t, defaultMaxTotalAddresses, r.maxTotalAddresses)
	assert.Equal(t, 100000, defaultMaxAddressesPerIP)
	assert.Equal(t, 2000000, defaultMaxTotalAddresses)

	r = Limits{MaxAddressesPerSubscribe: 1, MaxAddressesPerClient: 1, MaxAddressesPerIP: 7, MaxTotalAddresses: 9}.resolve()
	assert.Equal(t, 7, r.maxAddressesPerIP)
	assert.Equal(t, 9, r.maxTotalAddresses)

	// perClient <= perIP <= total, raised from below.
	r = Limits{MaxAddressesPerSubscribe: 10, MaxAddressesPerClient: 20, MaxAddressesPerIP: 5, MaxTotalAddresses: 3}.resolve()
	assert.Equal(t, 20, r.maxAddressesPerIP)
	assert.Equal(t, 20, r.maxTotalAddresses)

	// An unset per-IP budget below a large per-client cap follows it up.
	r = Limits{MaxAddressesPerClient: 600000}.resolve()
	assert.Equal(t, 600000, r.maxAddressesPerIP)
	assert.Equal(t, defaultMaxTotalAddresses, r.maxTotalAddresses)
}

func TestAddressBudget_HeldPairsDoNotCountAgain(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3, MaxAddressesPerClient: 3, MaxAddressesPerIP: 3, MaxTotalAddresses: 3})
	c := ipClient(hub, "1.2.3.4")

	// Duplicates inside one call count once; the client now holds 2 of the IP's 3.
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "a", "b"}, c))
	// Fill the IP budget from another client, then re-subscribe held pairs with a new flag.
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"z"}, ipClient(hub, "1.2.3.4")))
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.USER_TRANSACTIONS}, []string{"a", "b"}, c),
		"pairs the client already holds must not be refused at budget")

	s := snapshotBudget(hub, c)
	assert.Equal(t, 3, s.total)
	assert.Equal(t, map[string]int{"1.2.3.4": 3}, s.perIP)
	assert.Equal(t, 2, s.perClient)
}

func TestPrecheckAddressBudget(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 3})
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, "1.2.3.4")))

	same := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 1}
	other := &net.TCPAddr{IP: net.ParseIP("5.6.7.8"), Port: 1}
	require.ErrorIs(t, hub.PrecheckAddressBudget(accounts, []string{"c"}, same), ErrPerIPAddressBudgetExhausted)
	require.ErrorIs(t, hub.PrecheckAddressBudget(accounts, []string{"c", "d"}, other), ErrGlobalAddressBudgetExhausted)
	require.NoError(t, hub.PrecheckAddressBudget(accounts, []string{"c", "c"}, other), "duplicates count once")
	require.NoError(t, hub.PrecheckAddressBudget([]indexer.EventType{indexer.BLOCKS}, []string{"c"}, same), "blocks-only ignores addresses")
}
