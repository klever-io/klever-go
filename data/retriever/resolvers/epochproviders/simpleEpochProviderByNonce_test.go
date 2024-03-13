package epochproviders

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func TestSimpleEpochProviderByNonce_EpochForNonce(t *testing.T) {
	t.Parallel()

	epoch := uint32(1)
	sep := NewSimpleEpochProviderByNonce(&mock.EpochHandlerStub{
		EpochCalled: func() uint32 {
			return epoch
		},
	})
	require.False(t, check.IfNil(sep))

	resEpoch, err := sep.EpochForNonce(0)
	require.Nil(t, err)
	require.Equal(t, epoch, resEpoch)
}
