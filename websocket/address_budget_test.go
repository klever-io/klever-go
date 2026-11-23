package websocket

import (
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	indexer "github.com/klever-io/klever-go/indexer"
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
	newTestHub(nil).LogRejectedInsertion("ws.budgetProbe", errPrefix48BudgetExhausted)

	out := observed.String()
	assert.Contains(t, out, "per-ip")
	assert.Contains(t, out, "node-wide")
	assert.Contains(t, out, "per-ipv6-48")
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
	for addr, peak := range hub.addrSubPeak {
		clients, ok := hub.addressSubscription[addr]
		require.True(t, ok, "address peak %s outlived its subscriber map", addr)
		require.GreaterOrEqual(t, peak, len(clients))
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
	for c, peak := range hub.clientAddrPeak {
		addrs, ok := hub.clientAddrs[c]
		require.True(t, ok, "a peak must not outlive its client index")
		require.GreaterOrEqual(t, peak, len(addrs))
	}
}

func TestClientAddrs_ChurnRebuildsDrainedIndex(t *testing.T) {
	hub := newTestHub(nil)
	c := ipClient(hub, "1.2.3.4")
	const n, keep = 5000, 3
	addrs := make([]string, n)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("addr-%d", i)
	}
	require.NoError(t, hub.HandleClientInsertion(accounts, addrs, c))
	hub.mu.RLock()
	require.Equal(t, n, hub.clientAddrPeak[c])
	before := fmt.Sprintf("%p", hub.clientAddrs[c])
	hub.mu.RUnlock()

	for i := keep; i < n; i += 500 {
		hub.HandleClientRemoval(accounts, addrs[i:min(i+500, n)], c)
		requireIndexConsistent(t, hub)
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Len(t, hub.clientAddrs[c], keep)
	assert.Less(t, hub.clientAddrPeak[c], clientAddrsRebuildFloor, "the drained index must have been rebuilt")
	assert.NotEqual(t, before, fmt.Sprintf("%p", hub.clientAddrs[c]), "the rebuild must allocate a fresh map")
	assert.Equal(t, keep, hub.totalAddresses)
}

func TestClientAddrs_SmallIndexIsNotRebuilt(t *testing.T) {
	hub := newTestHub(nil)
	c := ipClient(hub, "1.2.3.4")
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b", "c", "d", "e"}, c))
	hub.HandleClientRemoval(accounts, []string{"a", "b", "c", "d"}, c)
	hub.mu.RLock()
	assert.Equal(t, 5, hub.clientAddrPeak[c], "below the floor a drained index keeps its peak")
	hub.mu.RUnlock()

	hub.HandleClientRemoval(accounts, []string{"e"}, c)
	requireIndexConsistent(t, hub)
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.NotContains(t, hub.clientAddrPeak, c, "an emptied index drops its peak")
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

// addrClient keys a client by remote as NewClient does.
func addrClient(hub *SocketHub, remote net.Addr) *client {
	c := newTestClient(hub)
	c.ipKey, c.prefix48Key = remoteIPKey(remote), remotePrefix48Key(remote)
	return c
}

func v6(host string) net.Addr { return &net.TCPAddr{IP: net.ParseIP(host), Port: 1} }

func TestAddressBudget_IPv6Prefix48CapsSibling56s(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxAddressesPerIPv6Prefix48: 6, MaxTotalAddresses: 100})
	var held []*client
	for _, host := range []string{"2001:db8:1:100::1", "2001:db8:1:200::1", "2001:db8:1:300::1"} {
		c := addrClient(hub, v6(host))
		require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, c), host)
		held = append(held, c)
	}

	fourth := v6("2001:db8:1:400::1")
	require.ErrorIs(t, hub.PrecheckAddressBudget(accounts, []string{"a"}, fourth), errPrefix48BudgetExhausted)
	c4 := addrClient(hub, fourth)
	before := snapshotBudget(hub, c4)
	err := hub.HandleClientInsertion(accounts, []string{"a"}, c4)
	require.ErrorIs(t, err, errPrefix48BudgetExhausted)
	require.ErrorIs(t, err, ErrPerIPAddressBudgetExhausted, "peers see the per-IP refusal")
	assert.Equal(t, "address subscription budget reached", err.Error())
	assert.Equal(t, before, snapshotBudget(hub, c4), "a refused insertion must leave every counter unchanged")

	otherPrefix48 := v6("2001:db8:2:100::1")
	require.NoError(t, hub.PrecheckAddressBudget(accounts, []string{"a"}, otherPrefix48))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a"}, addrClient(hub, otherPrefix48)), "another /48 has its own budget")

	hub.mu.RLock()
	assert.Equal(t, map[string]int{"2001:db8:1::": 6, "2001:db8:2::": 1}, hub.prefix48Addresses)
	hub.mu.RUnlock()

	// A released pair frees /48 budget for a sibling /56.
	hub.HandleClientRemoval(accounts, []string{"a"}, held[0])
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a"}, c4))
	hub.mu.RLock()
	assert.Equal(t, 6, hub.prefix48Addresses["2001:db8:1::"])
	hub.mu.RUnlock()
}

func TestAddressBudget_IPv6Prefix48ReleasesOnTeardown(t *testing.T) {
	hub := newTestHub(nil)
	c1 := newConnClient(hub, upgradedConn(t))
	c1.ipKey, c1.prefix48Key = remoteIPKey(v6("2001:db8:1:100::1")), remotePrefix48Key(v6("2001:db8:1:100::1"))
	c2 := newConnClient(hub, upgradedConn(t))
	c2.ipKey, c2.prefix48Key = remoteIPKey(v6("2001:db8:1:200::1")), remotePrefix48Key(v6("2001:db8:1:200::1"))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, c1))
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a"}, c2))

	hub.handleClientDelete(c1)
	hub.mu.RLock()
	assert.Equal(t, map[string]int{"2001:db8:1::": 1}, hub.prefix48Addresses)
	hub.mu.RUnlock()

	hub.deleteAll()
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Empty(t, hub.prefix48Addresses)
	assert.Empty(t, hub.clientAddrPeak)
}

func TestAddressBudget_IPv4SkipsThePrefix48Tier(t *testing.T) {
	for _, host := range []string{"192.0.2.7", "::ffff:192.0.2.7", "64:ff9b::c000:207"} {
		assert.Empty(t, remotePrefix48Key(v6(host)), host)
	}
	assert.Equal(t, "2001:db8:1::", remotePrefix48Key(v6("2001:db8:1:2ff::9")))
	assert.Empty(t, remotePrefix48Key(nil))

	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxAddressesPerIPv6Prefix48: 2, MaxTotalAddresses: 100})
	for _, host := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, addrClient(hub, v6(host))), host)
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.Empty(t, hub.prefix48Addresses)
	assert.Equal(t, 6, hub.totalAddresses)
}

func TestNewClient_SetsIPKeyFromRemoteAddr(t *testing.T) {
	_, c := dialLiveClient(t, newTestHub(nil))
	t.Cleanup(c.Close)
	assert.Equal(t, "127.0.0.1", c.ipKey)
	assert.Empty(t, c.prefix48Key)
}

func TestLimits_Resolve_AddressBudgetDefaultsAndClamps(t *testing.T) {
	r := Limits{}.resolve()
	assert.Equal(t, defaultMaxAddressesPerIP, r.maxAddressesPerIP)
	assert.Equal(t, defaultMaxTotalAddresses, r.maxTotalAddresses)
	assert.Equal(t, 100000, defaultMaxAddressesPerIP)
	assert.Equal(t, 2000000, defaultMaxTotalAddresses)
	assert.Equal(t, 400000, r.maxAddressesPerIPv6Prefix48)

	r = Limits{MaxAddressesPerSubscribe: 1, MaxAddressesPerClient: 1, MaxAddressesPerIP: 7, MaxTotalAddresses: 9}.resolve()
	assert.Equal(t, 7, r.maxAddressesPerIP)
	assert.Equal(t, 9, r.maxTotalAddresses)
	assert.Equal(t, 9, r.maxAddressesPerIPv6Prefix48, "an unset /48 budget is capped at the total")

	// perIP <= per-/48 <= total: a low /48 is raised to perIP, a high one is clamped to the total.
	r = Limits{MaxAddressesPerSubscribe: 1, MaxAddressesPerClient: 1, MaxAddressesPerIP: 7, MaxAddressesPerIPv6Prefix48: 3, MaxTotalAddresses: 9}.resolve()
	assert.Equal(t, 7, r.maxAddressesPerIPv6Prefix48)
	r = Limits{MaxAddressesPerSubscribe: 1, MaxAddressesPerClient: 1, MaxAddressesPerIP: 7, MaxAddressesPerIPv6Prefix48: 12, MaxTotalAddresses: 9}.resolve()
	assert.Equal(t, 9, r.maxAddressesPerIPv6Prefix48)
	assert.Equal(t, 9, r.maxTotalAddresses)

	// The total is never raised: every inner tier is clamped down to it.
	r = Limits{MaxAddressesPerSubscribe: 10, MaxAddressesPerClient: 20, MaxAddressesPerIP: 5, MaxTotalAddresses: 3}.resolve()
	assert.Equal(t, 3, r.maxAddressesPerSubscribe)
	assert.Equal(t, 3, r.maxAddressesPerClient)
	assert.Equal(t, 3, r.maxAddressesPerIP)
	assert.Equal(t, 3, r.maxAddressesPerIPv6Prefix48)
	assert.Equal(t, 3, r.maxTotalAddresses)

	// Shipped config with only the total lowered: the explicit /48 must not lift it back.
	r = Limits{MaxAddressesPerIP: 100000, MaxAddressesPerIPv6Prefix48: 400000, MaxTotalAddresses: 200000}.resolve()
	assert.Equal(t, 200000, r.maxAddressesPerIPv6Prefix48)
	assert.Equal(t, 200000, r.maxTotalAddresses)
	r = Limits{MaxTotalAddresses: 60000}.resolve()
	assert.Equal(t, 50000, r.maxAddressesPerClient)
	assert.Equal(t, 60000, r.maxAddressesPerIP)
	assert.Equal(t, 60000, r.maxTotalAddresses)

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

func addrSubState(hub *SocketHub, addr string) (string, int, int) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	m := hub.addressSubscription[addr]
	return fmt.Sprintf("%p", m), len(m), hub.addrSubPeak[addr]
}

func TestAddrSub_UnsubscribeChurnRebuildsSharedAddress(t *testing.T) {
	hub := newTestHub(nil)
	clients := make([]*client, 64)
	for i := range clients {
		clients[i] = ipClient(hub, fmt.Sprintf("10.0.0.%d", i))
		require.NoError(t, hub.HandleClientInsertion(accounts, []string{"shared"}, clients[i]))
	}
	before, _, peak := addrSubState(hub, "shared")
	require.Equal(t, 64, peak)

	for _, c := range clients[1:] {
		hub.HandleClientRemoval(accounts, []string{"shared"}, c)
	}
	after, live, peak := addrSubState(hub, "shared")
	assert.Equal(t, 1, live)
	assert.Less(t, peak, addrSubRebuildFloor, "a drained subscriber map must be rebuilt")
	assert.NotEqual(t, before, after, "the rebuild must swap in a fresh map")
	requireIndexConsistent(t, hub)
}

func TestAddrSub_TeardownChurnRebuildsSharedAddress(t *testing.T) {
	hub := newTestHub(nil)
	clients := make([]*client, 32)
	for i := range clients {
		clients[i] = liveIPClient(t, hub, fmt.Sprintf("10.0.1.%d", i))
		require.NoError(t, hub.HandleClientInsertion(accounts, []string{"shared"}, clients[i]))
	}
	before, _, _ := addrSubState(hub, "shared")

	for _, c := range clients[1:] {
		hub.RemoveClient(c)
	}
	after, live, peak := addrSubState(hub, "shared")
	assert.Equal(t, 1, live)
	assert.Less(t, peak, addrSubRebuildFloor)
	assert.NotEqual(t, before, after)
	requireIndexConsistent(t, hub)

	hub.RemoveClient(clients[0])
	_, live, peak = addrSubState(hub, "shared")
	assert.Zero(t, live)
	assert.Zero(t, peak, "the peak must go with the emptied address")
	requireIndexConsistent(t, hub)
}

func TestHandleDynamicSubscribe_LogsBudgetRefusal(t *testing.T) {
	var observed lockedBuffer
	require.NoError(t, logger.AddLogObserver(&observed, &logger.PlainFormatter{}))
	t.Cleanup(func() { assert.NoError(t, logger.RemoveLogObserver(&observed)) })

	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 2})
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b"}, ipClient(hub, "1.2.3.4")))

	params, err := json.Marshal(SubscribeParams{Types: []string{"accounts"}, Addresses: []string{"c"}})
	require.NoError(t, err)
	resp := sendRequest(hub, ipClient(hub, "5.6.7.8"), WSRequest{ID: "dyn", Method: MethodSubscribe, Params: params})

	require.Equal(t, ErrGlobalAddressBudgetExhausted.Error(), resp.Error)
	out := observed.String()
	assert.Contains(t, out, opDynamicSubscribe)
	assert.Contains(t, out, "node-wide")
}

func TestAddressBudget_PublishesTotalAddressesGauge(t *testing.T) {
	hub := NewHub("", "", nil, Limits{MaxAddressesPerSubscribe: 3, MaxAddressesPerClient: 3, MaxAddressesPerIP: 6, MaxTotalAddresses: 100})
	var last uint64
	hub.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			if key == core.MetricWSSubscribedAddresses {
				last = value
			}
		},
	})
	c1, c2 := liveIPClient(t, hub, "1.2.3.4"), liveIPClient(t, hub, "1.2.3.4")

	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "b", "a"}, c1))
	assert.Equal(t, uint64(2), last, "duplicates within a call count once")
	require.NoError(t, hub.HandleClientInsertion(accounts, []string{"a", "c"}, c2))
	assert.Equal(t, uint64(4), last)

	hub.HandleClientRemoval(accounts, []string{"a"}, c2)
	assert.Equal(t, uint64(3), last)
	hub.handleClientDelete(c1)
	assert.Equal(t, uint64(1), last)
	hub.deleteAll()
	assert.Equal(t, uint64(0), last)
}
