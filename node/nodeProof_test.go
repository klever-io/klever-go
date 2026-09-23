package node

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/stretchr/testify/require"
)

func TestNode_GetProofDecodesAddressAndVerifies(t *testing.T) {
	t.Parallel()

	address := make([]byte, 32)
	address[0] = 4
	adb := newNodeProofAccounts(t)
	acc, err := adb.LoadAccount(address)
	require.NoError(t, err)
	require.NoError(t, acc.(state.UserAccountHandler).AddToBalance(8, nil, false))
	require.NoError(t, adb.SaveAccount(acc))
	root, err := adb.Commit()
	require.NoError(t, err)

	conv, err := pubkeyConverter.NewBech32PubkeyConverter(len(address))
	require.NoError(t, err)
	n := &Node{
		accounts:               adb,
		addressPubkeyConverter: conv,
	}

	proof, err := n.GetProof(conv.Encode(address))
	require.NoError(t, err)
	require.Equal(t, root, proof.RootHash)

	ok, err := n.VerifyProof(root, conv.Encode(address), proof.Proof)
	require.NoError(t, err)
	require.True(t, ok)

	_, err = n.GetProof("not-an-address")
	require.ErrorIs(t, err, state.ErrInvalidProofRequest)

	_, err = n.GetProofForRootHash([]byte{0x01, 0x02}, conv.Encode(address))
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)
}

func TestNode_GetProofRequiresProverAndConverter(t *testing.T) {
	t.Parallel()

	conv, err := pubkeyConverter.NewBech32PubkeyConverter(32)
	require.NoError(t, err)
	encoded := conv.Encode(make([]byte, 32))

	n := &Node{addressPubkeyConverter: conv}
	_, err = n.GetProof(encoded)
	require.ErrorIs(t, err, common.ErrNilAccountsAdapter)

	n.accounts = &mock.AccountsStub{}
	_, err = n.GetProof(encoded)
	require.EqualError(t, err, "accounts adapter does not support merkle proofs")

	_, err = n.GetProofForRootHash([]byte{0x01}, encoded)
	require.EqualError(t, err, "accounts adapter does not support merkle proofs")
	_, err = n.VerifyProof([]byte{0x01}, encoded, nil)
	require.EqualError(t, err, "accounts adapter does not support merkle proofs")

	_, err = n.GetProofForRootHash([]byte{0x01}, "not-an-address")
	require.ErrorIs(t, err, state.ErrInvalidProofRequest)
	_, err = n.VerifyProof([]byte{0x01}, "not-an-address", nil)
	require.ErrorIs(t, err, state.ErrInvalidProofRequest)
	_, err = n.GetProof("")
	require.ErrorIs(t, err, state.ErrInvalidProofRequest)

	n.accounts = nil
	n.addressPubkeyConverter = nil
	_, err = n.GetProof(encoded)
	require.ErrorIs(t, err, common.ErrNilPubkeyConverter)
}

func newNodeProofAccounts(t *testing.T) *state.AccountsDB {
	t.Helper()

	marshalizer := &mock.MarshalizerMock{}
	hsh := &mock.HasherMock{}
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hsh, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)
	return adb
}
