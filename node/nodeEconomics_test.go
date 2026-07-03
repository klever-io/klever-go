package node

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// newEconomicsTestNode builds a minimal Node whose KApp adapter returns an account whose data
// trie yields the given key->value records (values delivered via GetStorage, already trimmed).
func newEconomicsTestNode(records map[string][]byte) *Node {
	leaves := make([]data.KeyValueHolder, 0, len(records))
	for key := range records {
		leaves = append(leaves, keyValStorage.NewKeyValStorage([]byte(key), nil))
	}
	trieStub := &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
			ch := make(chan data.KeyValueHolder, len(leaves))
			for _, l := range leaves {
				ch <- l
			}
			close(ch)
			return ch, nil
		},
	}
	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled:    func() data.Trie { return trieStub },
		GetRootHashCalled: func() []byte { return []byte("root") },
		GetStorageCalled:  func(key []byte) []byte { return records[string(key)] },
	}
	return &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		},
		internalMarshalizer: marshal.NewProtoMarshalizer(),
	}
}

func TestNode_loadFeesPoolKLVTotal(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	records := make(map[string][]byte)
	// NEG exercises the negative guard: a corrupt balance is skipped, not summed.
	for id, klv := range map[string]int64{"KLV": 1000, "ABC-1q2w": 2500, "ZERO": 0, "NEG": -50} {
		raw, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{KLVBalance: klv})
		require.NoError(t, err)
		records[id] = raw
	}

	n := newEconomicsTestNode(records)
	total, err := n.loadFeesPoolKLVTotal()
	require.NoError(t, err)
	require.Equal(t, int64(3500), total) // 1000 + 2500 + 0, NEG skipped
}

func TestNode_loadFPRPoolKLVTotal(t *testing.T) {
	t.Parallel()

	marshalizer := marshal.NewProtoMarshalizer()
	// total per asset = CurrentFPRAmount + Σ(TotalAmount-TotalClaimed) where the diff is > 0.
	// TotalAmount/CurrentFPRAmount are KLV; the KDAS map (non-KLV rewards) must be excluded.
	stakings := map[string]*kapps.StakingData{
		"A": {CurrentFPRAmount: 100, FPR: []*kapps.FPRData{{
			TotalAmount: 500, TotalClaimed: 200,
			// non-KLV reward (9999) must NOT be added to the KLV total.
			KDAS: map[string]*kapps.KDAFPRData{"OTHER-1q2w": {TotalAmount: 9999, TotalClaimed: 0}},
		}}}, // 100 + 300 = 400 (KDAS excluded)
		"B": {CurrentFPRAmount: 50, FPR: []*kapps.FPRData{{TotalAmount: 1000, TotalClaimed: 1000}}}, // 50 + 0 = 50
		"C": {CurrentFPRAmount: 0, FPR: []*kapps.FPRData{{TotalAmount: 100, TotalClaimed: 150}}},    // 0 + skip(neg) = 0
		"D": {CurrentFPRAmount: 25, FPR: []*kapps.FPRData{
			{TotalAmount: 300, TotalClaimed: 100},
			{TotalAmount: 200, TotalClaimed: 200},
		}}, // 25 + 200 + 0 = 225
	}
	records := make(map[string][]byte)
	for id, s := range stakings {
		raw, err := marshalizer.Marshal(s)
		require.NoError(t, err)
		records[string(kdautils.ToKDAKey([]byte(id), nil))] = raw
	}
	// a non-KDA-prefixed leaf must be filtered out, not decoded into the total
	records["garbage-key"] = []byte{0xff, 0xfe, 0xfd}

	n := newEconomicsTestNode(records)
	total, err := n.loadFPRPoolKLVTotal()
	require.NoError(t, err)
	require.Equal(t, int64(675), total) // 400 + 50 + 0 + 225
}

func TestNode_scanKAppDataTrie_nilTrie(t *testing.T) {
	t.Parallel()

	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled: func() data.Trie { return nil },
	}
	n := &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return app, nil },
		},
		internalMarshalizer: marshal.NewProtoMarshalizer(),
	}

	called := false
	err := n.scanKAppDataTrie([]byte("addr"), nil, func(_ []byte) error { called = true; return nil })
	require.NoError(t, err)
	require.False(t, called)
}
