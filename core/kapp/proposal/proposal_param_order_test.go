package proposal

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

// TestValidateNewParameters_FirstErrorInKeyOrder checks that from FixAuditChangesV7 a proposal's parameters
// are checked in key order: an unknown parameter (ErrInvalidParameter) sorts before a known one with an
// empty value (ErrInvalidValue), so the error is the same on every run instead of following Go's random
// map order (KLC-2770).
func TestValidateNewParameters_FirstErrorInKeyOrder(t *testing.T) {
	t.Parallel()

	params := map[int32][]byte{
		-1: []byte("1"),
		int32(kapps.EnumParameter_KAppFeeTransfer): {},
	}
	newKApp := func(v7 bool) *proposalKapp {
		return &proposalKapp{forkController: mock.NewForkControllerStub().SetFork("FixAuditChangesV7", v7)}
	}

	t.Run("first failing parameter in order wins from FixAuditChangesV7", func(t *testing.T) {
		p := newKApp(true)
		for i := 0; i < 100; i++ {
			require.Equal(t, common.ErrInvalidParameter, p.validateNewParameters(params, nil), "run %d", i)
		}
	})

	t.Run("either error may come first before FixAuditChangesV7", func(t *testing.T) {
		p := newKApp(false)
		seen := make(map[error]bool)
		for i := 0; i < 100; i++ {
			seen[p.validateNewParameters(params, nil)] = true
		}
		require.Equal(t, map[error]bool{common.ErrInvalidParameter: true, common.ErrInvalidValue: true}, seen,
			"pre-fork order must stay Go's map order")
	})
}
