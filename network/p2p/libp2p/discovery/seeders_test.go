package discovery_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery"
	"github.com/stretchr/testify/assert"
)

func TestWithoutOwnPeerID(t *testing.T) {
	t.Parallel()

	self := newTestPeerID(t)
	other := "/ip4/10.0.0.1/tcp/37373/p2p/" + newTestPeerID(t).String()
	malformed := "10.0.0.2:37373"
	own := "/ip4/10.0.0.3/tcp/37373/p2p/" + self.String()

	filtered := discovery.WithoutOwnPeerID([]string{other, own, malformed, own}, self)

	// only the entries naming this node are dropped; the rest keep their order
	assert.Equal(t, []string{other, malformed}, filtered)
}
