package trie

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/stretchr/testify/require"
)

func interceptNode(t *testing.T, n node) error {
	marsh, hsh := getTestMarshalizerAndHasher()
	require.Nil(t, n.setHash())
	encoded, err := n.getEncodedNode()
	require.Nil(t, err)

	intercepted, err := NewInterceptedTrieNode(encoded, marsh, hsh)
	require.Nil(t, err)

	return intercepted.CheckValidity()
}

func TestInterceptedLeafNodeKeyLengthBound(t *testing.T) {
	t.Parallel()

	marsh, hsh := getTestMarshalizerAndHasher()

	atBound := keyBytesToHex(bytes.Repeat([]byte{0xab}, common.MaxStorageKeyLength))
	leaf, err := newLeafNode(atBound, []byte("value"), marsh, hsh)
	require.Nil(t, err)
	require.Nil(t, interceptNode(t, leaf))

	overBound := append([]byte{1}, atBound...)
	leaf, err = newLeafNode(overBound, []byte("value"), marsh, hsh)
	require.Nil(t, err)
	require.Equal(t, ErrInvalidNode, interceptNode(t, leaf))
}

func TestInterceptedExtensionNodeKeyLengthBound(t *testing.T) {
	t.Parallel()

	marsh, hsh := getTestMarshalizerAndHasher()
	child, err := newLeafNode([]byte{2, hexTerminator}, []byte("value"), marsh, hsh)
	require.Nil(t, err)

	atBound := bytes.Repeat([]byte{7}, maxTriePathLen)
	ext, err := newExtensionNode(atBound, child, marsh, hsh)
	require.Nil(t, err)
	require.Nil(t, interceptNode(t, ext))

	overBound := bytes.Repeat([]byte{7}, maxTriePathLen+1)
	ext, err = newExtensionNode(overBound, child, marsh, hsh)
	require.Nil(t, err)
	require.Equal(t, ErrInvalidNode, interceptNode(t, ext))
}

// A data-trie leaf for the longest storage key a contract may write still passes the interceptor.
func TestInterceptedMaxStorageKeyLeafFromTriePasses(t *testing.T) {
	t.Parallel()

	tr, _, _ := newEmptyTrie()
	key := bytes.Repeat([]byte{0xff}, common.MaxStorageKeyLength)
	require.Nil(t, tr.Update(key, []byte("value")))
	require.Nil(t, tr.Commit())

	leaf, ok := tr.root.(*leafNode)
	require.True(t, ok)
	require.Len(t, leaf.Key, maxTriePathLen)
	require.Nil(t, interceptNode(t, leaf))
}
