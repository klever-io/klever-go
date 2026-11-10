package discovery

import (
	"fmt"
	"net"

	"github.com/klever-io/klever-go/network/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// seeder is one peer ID of the initial peers list, with every address the list gives for it
type seeder struct {
	addresses []string
	addrInfo  peer.AddrInfo
	// ips are the configured IP addresses
	ips []net.IP
	// matchesAnyIP is set when an entry names a DNS host: then any connection under the seeder's
	// peer ID counts as a connection to it
	matchesAnyIP bool
}

// CheckInitialPeersList returns an error if an entry of the initial peers list is not a
// /<transport>/.../p2p/<peerID> address or names the local peer ID
func CheckInitialPeersList(initialPeersList []string, self peer.ID) error {
	_, err := parseSeeders(initialPeersList, self)
	return err
}

// WithoutOwnPeerID returns the initial peers list without the entries that name the local peer
// ID, logging a warning for each one it drops. Entries it can not parse are kept as they are.
func WithoutOwnPeerID(initialPeersList []string, self peer.ID) []string {
	filtered := make([]string, 0, len(initialPeersList))
	for _, address := range initialPeersList {
		addrInfo, err := parseAddrInfo(address)
		if err == nil && addrInfo.ID == self {
			log.Warn("ignoring an initial peer that is this node's own peer ID", "address", address)
			continue
		}

		filtered = append(filtered, address)
	}

	return filtered
}

// parseSeeders parses the initial peers list into one seeder per peer ID, in the order each peer
// ID first appears; entries that repeat a peer ID add their address to that seeder
func parseSeeders(initialPeersList []string, self peer.ID) ([]seeder, error) {
	seeders := make([]seeder, 0, len(initialPeersList))
	indexByID := make(map[peer.ID]int, len(initialPeersList))
	for _, address := range initialPeersList {
		addrInfo, err := parseAddrInfo(address)
		if err != nil {
			return nil, fmt.Errorf("%w for initial peer %q: %s", p2p.ErrInvalidValue, address, err.Error())
		}
		if addrInfo.ID == self {
			return nil, fmt.Errorf("%w: initial peer %q is this node's own peer ID", p2p.ErrInvalidValue, address)
		}

		idx, isKnown := indexByID[addrInfo.ID]
		if !isKnown {
			idx = len(seeders)
			indexByID[addrInfo.ID] = idx
			seeders = append(seeders, seeder{addrInfo: peer.AddrInfo{ID: addrInfo.ID}})
		}
		seeders[idx].addAddress(address, addrInfo.Addrs[0])
	}

	return seeders, nil
}

func parseAddrInfo(address string) (*peer.AddrInfo, error) {
	multiAddr, err := multiaddr.NewMultiaddr(address)
	if err != nil {
		return nil, err
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
	if err != nil {
		return nil, err
	}
	if len(addrInfo.Addrs) == 0 {
		return nil, fmt.Errorf("no transport address before /p2p")
	}

	return addrInfo, nil
}

func (s *seeder) addAddress(address string, transportAddr multiaddr.Multiaddr) {
	s.addresses = append(s.addresses, address)
	s.addrInfo.Addrs = append(s.addrInfo.Addrs, transportAddr)

	ip, err := manet.ToIP(transportAddr)
	if err != nil {
		s.matchesAnyIP = true
		return
	}
	s.ips = append(s.ips, ip)
}

// matchesIP returns true if a connection from ip counts as a connection to the seeder
func (s *seeder) matchesIP(ip net.IP) bool {
	if s.matchesAnyIP {
		return true
	}
	for _, configured := range s.ips {
		if configured.Equal(ip) {
			return true
		}
	}

	return false
}
