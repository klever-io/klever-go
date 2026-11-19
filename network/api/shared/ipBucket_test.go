package shared_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/api/shared"
	"github.com/stretchr/testify/assert"
)

func TestIPBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{name: "IPv4 keys on the exact address", host: "203.0.113.7", expected: "203.0.113.7"},
		{name: "IPv4 loopback", host: "127.0.0.1", expected: "127.0.0.1"},
		{name: "IPv6 collapses to its /64", host: "2001:db8:abcd:1234::1", expected: "2001:db8:abcd:1234::"},
		// The point of the whole helper: keying on the full /128 would hand anyone holding a
		// routed /64 2^64 distinct keys and make every per-IP cap decorative.
		{name: "top of a /64 shares the key with its bottom", host: "2001:db8:abcd:1234:ffff:ffff:ffff:ffff", expected: "2001:db8:abcd:1234::"},
		{name: "a neighbouring /64 is a different key", host: "2001:db8:abcd:1235::1", expected: "2001:db8:abcd:1235::"},
		{name: "IPv6 loopback", host: "::1", expected: "::"},
		{name: "IPv4-mapped IPv6 keys as the IPv4", host: "::ffff:203.0.113.7", expected: "203.0.113.7"},
		// Go renders a link-local peer's RemoteAddr with its zone ([fe80::1%eth0]:1234).
		// net.ParseIP rejects that form, so returning it verbatim would give every
		// self-assigned link-local address its own key and reopen the bypass.
		{name: "zoned link-local collapses to fe80::/64", host: "fe80::1%eth0", expected: "fe80::"},
		{name: "a second zoned link-local shares the key", host: "fe80::2%eth0", expected: "fe80::"},
		{name: "the same address on another zone shares the key", host: "fe80::1%en0", expected: "fe80::"},
		// NAT64 keeps client identity in the low bits, so the /64 mask would put every client
		// behind one translator in a single bucket and let one of them 503 the rest.
		{name: "NAT64 returns the embedded IPv4", host: "64:ff9b::cb00:7107", expected: "203.0.113.7"},
		{name: "a second NAT64 client gets its own bucket", host: "64:ff9b::cb00:7108", expected: "203.0.113.8"},
		// Teredo is not un-masked: the client builds its own address, so the embedded IPv4
		// is attacker-selectable and would make the key forgeable.
		{name: "Teredo buckets by /64 like any other IPv6", host: "2001:0:0:0:0:0:34ff:8ef8", expected: "2001::"},
		// Distinct malformed hosts must not collide either: collapsing them to one key would
		// let one bad address throttle unrelated callers.
		{name: "unparseable host is returned verbatim", host: "bad address", expected: "bad address"},
		{name: "a different unparseable host keeps its own key", host: "other junk", expected: "other junk"},
		{name: "empty host", host: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, shared.IPBucket(tt.host))
		})
	}
}

func TestIPBucketBits(t *testing.T) {
	t.Parallel()

	// Two /64s inside one /56 share a /56 key but not a /64 one.
	assert.Equal(t, "2001:db8:abcd:1200::", shared.IPBucketBits("2001:db8:abcd:1201::1", 56))
	assert.Equal(t, "2001:db8:abcd:1200::", shared.IPBucketBits("2001:db8:abcd:12ff::9", 56))
	assert.Equal(t, "2001:db8:abcd:1300::", shared.IPBucketBits("2001:db8:abcd:1301::1", 56))
	assert.Equal(t, shared.IPBucket("2001:db8:abcd:1201::1"), shared.IPBucketBits("2001:db8:abcd:1201::1", shared.IPv6BucketBits))

	// IPv4, IPv4-mapped, NAT64 and junk ignore the IPv6 prefix length.
	assert.Equal(t, "203.0.113.7", shared.IPBucketBits("203.0.113.7", 56))
	assert.Equal(t, "203.0.113.7", shared.IPBucketBits("::ffff:203.0.113.7", 56))
	assert.Equal(t, "203.0.113.7", shared.IPBucketBits("64:ff9b::cb00:7107", 56))
	assert.Equal(t, "bad address", shared.IPBucketBits("bad address", 56))
}

func TestRemoteIPKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, remote, expected string
		bits                   int
	}{
		{name: "IPv4 host:port", remote: "203.0.113.7:1234", bits: 64, expected: "203.0.113.7"},
		{name: "IPv4 ignores the IPv6 bits", remote: "203.0.113.7:1234", bits: 56, expected: "203.0.113.7"},
		{name: "IPv6 host:port by /64", remote: "[2001:db8:abcd:1201::1]:1", bits: 64, expected: "2001:db8:abcd:1201::"},
		{name: "IPv6 host:port by /56", remote: "[2001:db8:abcd:1201::1]:1", bits: 56, expected: "2001:db8:abcd:1200::"},
		{name: "zoned link-local host:port", remote: "[fe80::1%eth0]:1", bits: 64, expected: "fe80::"},
		{name: "bare IPv4 host", remote: "203.0.113.7", bits: 64, expected: "203.0.113.7"},
		{name: "bare IPv6 host", remote: "2001:db8:abcd:1201::1", bits: 56, expected: "2001:db8:abcd:1200::"},
		{name: "junk is returned verbatim", remote: "bad address", bits: 64, expected: "bad address"},
		{name: "empty", remote: "", bits: 64, expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, shared.RemoteIPKey(tt.remote, tt.bits))
		})
	}
}
