package node

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// newRealKAppAccount returns a KApp account backed by a real, empty Patricia trie, so that
// RetrieveValue follows the production path: a missing key yields (nil, nil), not an error.
func newRealKAppAccount(t *testing.T, address []byte) state.KAppAccountHandler {
	t.Helper()

	marshalizer := &mock.ProtobufMarshalizerMock{}
	ewl, err := mock.NewEvictionWaitingList(100, mock.NewMemDbMock(), marshalizer)
	require.NoError(t, err)
	tsm, err := trie.NewTrieStorageManager(
		mock.NewMemDbMock(), marshalizer, &mock.KeccakMock{},
		config.DBConfig{}, ewl,
		config.TrieStorageManagerConfig{PruningBufferLen: 1000, SnapshotsBufferLen: 10, MaxSnapshots: 2},
	)
	require.NoError(t, err)
	tr, err := trie.NewTrie(tsm, marshalizer, &mock.KeccakMock{}, 5)
	require.NoError(t, err)

	acc, err := state.NewKAppAccount(address)
	require.NoError(t, err)
	acc.SetDataTrie(tr)

	return acc
}

func newNodeWithKApp(acc state.KAppAccountHandler) *Node {
	conv, _ := pubkeyConverter.NewBech32PubkeyConverter(32)
	return &Node{
		kapps: &mock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) { return acc, nil },
		},
		internalMarshalizer:    marshal.NewProtoMarshalizer(),
		addressPubkeyConverter: conv,
	}
}

// A real trie returns (nil, nil) for a missing key, so GetAsset must map an empty value to
// ErrAssetNotFound (as readKDAData already does) instead of returning a zero-valued asset.
func TestNode_GetAsset_MissingAssetShouldErr(t *testing.T) {
	t.Parallel()

	n := newNodeWithKApp(newRealKAppAccount(t, kapps.KDAKAppAddress))

	asset, err := n.GetAsset("NOPE-1Q2W")
	require.ErrorIs(t, err, common.ErrAssetNotFound, "got asset %+v", asset)
}

// GetKDAFeePool builds on GetAsset: for a missing asset it must not report an empty pool as found.
func TestNode_GetKDAFeePool_MissingAssetShouldErr(t *testing.T) {
	t.Parallel()

	n := newNodeWithKApp(newRealKAppAccount(t, kapps.KDAKAppAddress))

	pool, err := n.GetKDAFeePool("NOPE-1Q2W")
	require.ErrorIs(t, err, common.ErrAssetNotFound, "got pool %+v", pool)
}

func TestNode_GetMarketplace_MissingMarketplaceShouldErr(t *testing.T) {
	t.Parallel()

	n := newNodeWithKApp(newRealKAppAccount(t, kapps.MarketKAppAddress))

	mkt, err := n.GetMarketplace("deadbeefdeadbeef")
	require.ErrorIs(t, err, common.ErrMarketplaceNotFound, "got marketplace %+v", mkt)
}
