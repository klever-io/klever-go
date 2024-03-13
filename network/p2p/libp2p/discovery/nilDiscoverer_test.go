package discovery_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/p2p/libp2p/discovery"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNilDiscoverer(t *testing.T) {
	t.Parallel()

	nd := discovery.NewNilDiscoverer()

	assert.False(t, check.IfNil(nd))
	assert.Equal(t, discovery.NullName, nd.Name())
	assert.Nil(t, nd.Bootstrap())
	assert.Equal(t, 0, len(nd.ReconnectToNetwork()))
}
