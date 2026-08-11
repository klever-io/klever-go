package node

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

func TestNode_computeAccountTotals(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	leaves := make([]data.KeyValueHolder, 0)
	// A real account is stored at its address, so its inline Address == the leaf key.
	add := func(key string, balance, allowance int64) {
		raw, err := marshalizer.Marshal(&state.UserAccountData{Address: []byte(key), Balance: balance, Allowance: allowance})
		require.NoError(t, err)
		leaves = append(leaves, keyValStorage.NewKeyValStorage([]byte(key), raw))
	}
	add("addr-a", 1000, 50)
	add("addr-b", 2500, 0)
	add("addr-c", 0, 700)
	// an undecodable leaf is skipped.
	leaves = append(leaves, keyValStorage.NewKeyValStorage([]byte("garbage"), []byte{0xff, 0xff, 0xff, 0xff}))
	// a code leaf decodes cleanly into UserAccountData, but its Address (bytecode) != the leaf key
	// (code hash), so it must be skipped — its 9999 balance must NOT be summed.
	codeRaw, err := marshalizer.Marshal(&state.UserAccountData{Address: []byte("wasm-bytecode-blob"), Balance: 9999})
	require.NoError(t, err)
	leaves = append(leaves, keyValStorage.NewKeyValStorage([]byte("codehash"), codeRaw))

	n := &Node{
		accounts: &mock.AccountsStub{
			RootHashCalled: func() ([]byte, error) { return []byte("root"), nil },
			GetAllLeavesCalled: func(_ []byte) (*data.TrieIteratorChannels, error) {
				return data.NewCompletedTrieIteratorChannels(leaves...), nil
			},
		},
		internalMarshalizer: marshalizer,
	}

	totals, err := n.computeAccountTotals()
	require.NoError(t, err)
	require.Equal(t, int64(3), totals.AccountCount)     // code leaf skipped
	require.Equal(t, int64(3500), totals.BalanceTotal)  // 1000 + 2500 + 0
	require.Equal(t, int64(750), totals.AllowanceTotal) // 50 + 0 + 700
}

func TestNode_computeAccountTotals_truncatedWalk(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	raw, err := marshalizer.Marshal(&state.UserAccountData{Address: []byte("addr-a"), Balance: 1000, Allowance: 50})
	require.NoError(t, err)

	expectedErr := errors.New("trie iteration failed")
	n := &Node{
		accounts: &mock.AccountsStub{
			RootHashCalled: func() ([]byte, error) { return []byte("root"), nil },
			GetAllLeavesCalled: func(_ []byte) (*data.TrieIteratorChannels, error) {
				return data.NewFailedTrieIteratorChannels(
					expectedErr,
					keyValStorage.NewKeyValStorage([]byte("addr-a"), raw),
				), nil
			},
		},
		internalMarshalizer: marshalizer,
	}

	// The partial sum of the leaves that did arrive must not be handed back as the totals.
	totals, err := n.computeAccountTotals()
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, totals)
}

func TestNode_GetAccountTotals(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	raw, err := marshalizer.Marshal(&state.UserAccountData{Address: []byte("addr-a"), Balance: 1000, Allowance: 50})
	require.NoError(t, err)

	n := &Node{
		blkc: &mock.BlockChainMock{GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil }},
		accounts: &mock.AccountsStub{
			RootHashCalled: func() ([]byte, error) { return []byte("root"), nil },
			GetAllLeavesCalled: func(_ []byte) (*data.TrieIteratorChannels, error) {
				return data.NewCompletedTrieIteratorChannels(keyValStorage.NewKeyValStorage([]byte("addr-a"), raw)), nil
			},
		},
		internalMarshalizer: marshalizer,
	}

	totals, err := n.GetAccountTotals()
	require.NoError(t, err)
	require.Equal(t, int64(1), totals.AccountCount)
	require.Equal(t, int64(1000), totals.BalanceTotal)
	require.Equal(t, int64(50), totals.AllowanceTotal)

	cached, err := n.GetAccountTotals()
	require.NoError(t, err)
	require.Same(t, totals, cached) // memoized within the block
}

func TestNode_loadAccumulatedFeesTotal(t *testing.T) {
	t.Parallel()

	mkPeer := func(fees int64) state.PeerAccountHandler {
		p := state.NewEmptyPeerAccount()
		p.AddToAccumulatedFees(fees)
		return p
	}

	t.Run("sums peer accumulated fees", func(t *testing.T) {
		t.Parallel()
		n := &Node{validatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte { return []byte("root") },
			ListPeerAccountsCalled: func(_ []byte) ([]state.PeerAccountHandler, error) {
				return []state.PeerAccountHandler{mkPeer(100), mkPeer(250), mkPeer(0)}, nil
			},
		}}
		total, err := n.loadAccumulatedFeesTotal()
		require.NoError(t, err)
		require.Equal(t, int64(350), total) // 100 + 250 + 0
	})

	t.Run("returns 0 with no error before the first finalized block", func(t *testing.T) {
		t.Parallel()
		n := &Node{validatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte { return nil },
		}}
		total, err := n.loadAccumulatedFeesTotal()
		require.NoError(t, err)
		require.Zero(t, total)
	})

	t.Run("propagates peer-read error", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("peer read failed")
		n := &Node{validatorStatistics: &mock.ValidatorStatisticsProcessorStub{
			LastFinalizedRootHashCalled: func() []byte { return []byte("root") },
			ListPeerAccountsCalled: func(_ []byte) ([]state.PeerAccountHandler, error) {
				return nil, expectedErr
			},
		}}
		total, err := n.loadAccumulatedFeesTotal()
		require.ErrorIs(t, err, expectedErr)
		require.Zero(t, total)
	})
}
