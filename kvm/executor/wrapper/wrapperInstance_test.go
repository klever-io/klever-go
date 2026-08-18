package executorwrapper

import (
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/require"
)

// tableSizeInstance embeds executor.Instance so the 25-method interface is
// satisfied without hand-writing stubs for methods this test never calls.
// kvm/mock/context cannot be used here: it imports this package, so depending
// on it would create an import cycle.
type tableSizeInstance struct {
	executor.Instance
	size uint32
}

func (inst *tableSizeInstance) MaxDeclaredTableSize() uint32 { return inst.size }

// TestWrapperInstance_MaxDeclaredTableSize checks that the wrapper forwards the
// declared table maximum unchanged. The value gates contract acceptance
// (KLC-2526), so a wrapper that dropped or altered it would silently widen what
// the node accepts wherever the wrapping executor is in use.
func TestWrapperInstance_MaxDeclaredTableSize(t *testing.T) {
	for _, size := range []uint32{0, 1, 1000, ^uint32(0)} {
		wrapped := &WrapperInstance{
			wrappedInstance: &tableSizeInstance{size: size},
		}
		require.Equal(t, size, wrapped.MaxDeclaredTableSize())
	}
}
