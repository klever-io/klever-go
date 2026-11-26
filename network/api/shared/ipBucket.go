package shared

import (
	"net"
	"net/netip"
)

// IPv6BucketBits is the IPv6 prefix IPBucket keys on.
const IPv6BucketBits = 64

// nat64WellKnown is the RFC 6052 well-known NAT64 prefix; the client IPv4 is the last 32 bits.
// Masking it by /64 would put every client behind one translator in a single bucket and let one
// of them 503 the rest, so it keys on the embedded IPv4 instead — the same key a native IPv4
// client at that address gets. The translator writes those bits, not the peer, so the key is not
// forgeable. Teredo (2001::/32) is deliberately not un-masked the same way: a Teredo client
// constructs its own address, so the embedded IPv4 would be attacker-selectable.
var nat64WellKnown = netip.MustParsePrefix("64:ff9b::/96")

// IPBucket returns the per-source rate-limiting key for a host: the exact address for IPv4, or
// the /64 prefix for IPv6. Keying IPv6 on the full /128 hands anyone with a routed allocation
// 2^64 distinct keys per /64 — enough to walk past any per-IP cap by picking a fresh source
// address per connection.
//
// /64 is a reduction, not an identity. It is the smallest prefix ISPs delegate, but /56 and /48
// are common, so one customer can still hold 256 to 65536 buckets. The /log per-IP cap is
// backstopped by the node-wide cap; the sameSourceRequests throttler is not, and there the quota
// is multiplied by the client's allocation size. 6to4 (2002::/16) embeds the IPv4 above the /64
// boundary and is not special-cased either.
//
// Zone identifiers (fe80::1%eth0, which is how Go renders a link-local peer's RemoteAddr) are
// dropped before bucketing: every self-assigned link-local address on the node's segment lands in
// the same fe80::/64 key instead of getting its own.
//
// Known remaining collapse: the RFC 8215 local-use prefix 64:ff9b:1::/48. The embedded IPv4's
// offset depends on the operator's deployed prefix length, which the address does not carry, so
// clients behind such a gateway do share one bucket.
//
// It takes a host, not a host:port, so callers that need to report a malformed remote address
// keep their own net.SplitHostPort error handling. An unparseable host is returned verbatim
// rather than collapsed, so distinct junk values never share a bucket.
func IPBucket(host string) string {
	return IPBucketBits(host, IPv6BucketBits)
}

// IPBucketBits is IPBucket with the IPv6 prefix length chosen by the caller; IPv4, NAT64 and
// unparseable hosts are keyed exactly as IPBucket keys them.
func IPBucketBits(host string, ipv6Bits int) string {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	addr = addr.Unmap().WithZone("")

	if addr.Is4() {
		return addr.String()
	}

	if nat64WellKnown.Contains(addr) {
		a := addr.As16()
		return netip.AddrFrom4([4]byte{a[12], a[13], a[14], a[15]}).String()
	}

	return netip.PrefixFrom(addr, ipv6Bits).Masked().Addr().String()
}

// RemoteIPKey buckets a raw remote address, "host:port" or a bare host, by IPBucketBits.
func RemoteIPKey(remoteAddr string, ipv6Bits int) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return IPBucketBits(remoteAddr, ipv6Bits)
	}
	return IPBucketBits(host, ipv6Bits)
}
