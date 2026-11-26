package trie

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultPruningBuffer() *pruningBuffer {
	sb := newPruningBuffer(100)
	sb.add([]byte("0"))
	sb.add([]byte("1"))

	return sb
}

func TestPruningBuffer_NewPruningBuffer(t *testing.T) {
	assert.NotNil(t, newPruningBuffer(100))
}

func TestSnapshotsBuffer_Add(t *testing.T) {
	sb := defaultPruningBuffer()

	assert.Equal(t, 2, len(sb.buffer))
}

func TestSnapshotBuffer_Len(t *testing.T) {
	sb := defaultPruningBuffer()

	assert.Equal(t, 2, sb.len())
}

func TestSnapshotsBuffer_RemoveAll(t *testing.T) {
	sb := defaultPruningBuffer()

	buffer := sb.removeAll()
	assert.Equal(t, []byte("0"), buffer[0])
	assert.Equal(t, []byte("1"), buffer[1])

	assert.Equal(t, 0, len(sb.buffer))
}

func TestPruningBuffer_FullDropsPruneButKeepsCancelPrune(t *testing.T) {
	t.Parallel()

	pb := newPruningBuffer(1)
	pb.add([]byte{'a', byte(prune)})
	pb.add([]byte{'b', byte(prune)})
	pb.add([]byte{'c', byte(cancelPrune)})

	assert.Equal(t, [][]byte{{'a', byte(prune)}, {'c', byte(cancelPrune)}}, pb.removeAll())
}

func TestPruningBuffer_CancelPruneSupersedesQueuedPruneOfSameKey(t *testing.T) {
	t.Parallel()

	pb := newPruningBuffer(10)
	pb.add([]byte{'a', 1, byte(prune)})
	pb.add([]byte{'a', 2, byte(prune)})
	pb.add([]byte{'b', 1, byte(prune)})
	pb.add([]byte{'a', 1, byte(cancelPrune)})

	expected := [][]byte{{'a', 2, byte(prune)}, {'b', 1, byte(prune)}, {'a', 1, byte(cancelPrune)}}
	assert.Equal(t, expected, pb.removeAll())
}

func TestPruningBuffer_PruneQueuedAfterCancelPruneIsKept(t *testing.T) {
	t.Parallel()

	pb := newPruningBuffer(10)
	pb.add([]byte{'a', 1, byte(prune)})
	pb.add([]byte{'a', 1, byte(cancelPrune)})
	pb.add([]byte{'a', 1, byte(prune)})

	assert.Equal(t, [][]byte{{'a', 1, byte(cancelPrune)}, {'a', 1, byte(prune)}}, pb.removeAll())
}

func TestPruningBuffer_ZeroSizeKeepsOnlyCancelPrune(t *testing.T) {
	t.Parallel()

	pb := newPruningBuffer(0)
	pb.add([]byte{'a', byte(prune)})
	pb.add([]byte{'b', byte(cancelPrune)})

	assert.Equal(t, [][]byte{{'b', byte(cancelPrune)}}, pb.removeAll())
}
