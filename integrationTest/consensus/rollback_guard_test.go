package consensus

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	processSync "github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/require"
)

// TestRollbackForkDetector_RemoveHeaderAllowsRetry covers the ordinary removal
// used by sync error recovery, which must not permanently blacklist a header.
func TestRollbackForkDetector_RemoveHeaderAllowsRetry(t *testing.T) {
	realDetector, err := processSync.NewMetaForkDetector(
		&consensusMock.SlotManagerMock{SlotIndex: 1, TimeDurationCalled: func() time.Duration { return 0 }},
		&mock.BlackListHandlerStub{}, 0,
	)
	require.NoError(t, err)
	fd := &rollbackForkDetector{ForkDetector: realDetector, discarded: make(map[string]struct{})}
	header := &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 1}}
	hash := []byte("retryable header")
	require.NoError(t, fd.AddHeader(header, hash, process.BHReceived, nil, nil))
	require.Equal(t, uint64(1), fd.ProbableHighestNonce())

	fd.RemoveHeader(1, hash)
	require.Equal(t, uint64(0), fd.ProbableHighestNonce())
	require.NoError(t, fd.AddHeader(header, hash, process.BHReceived, nil, nil))
	require.Equal(t, uint64(1), fd.ProbableHighestNonce())
}
