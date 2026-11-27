package trie

import (
	"bytes"
	"slices"
	"sync"
)

type pruningBuffer struct {
	mutOp  sync.RWMutex
	buffer [][]byte
	size   uint32
}

func newPruningBuffer(pruningBufferLen uint32) *pruningBuffer {
	return &pruningBuffer{
		buffer: make([][]byte, 0),
		size:   pruningBufferLen,
	}
}

func (pb *pruningBuffer) add(rootHash []byte) {
	pb.mutOp.Lock()
	defer pb.mutOp.Unlock()

	// A cancel is never dropped and supersedes a queued prune of the same key, which FIFO replay
	// would otherwise run first. A full buffer only drops prunes, which leak nodes at worst.
	if isCancelPrune(rootHash) {
		pruneEntry := append(bytes.Clone(rootHash[:len(rootHash)-1]), byte(prune))
		pb.buffer = slices.DeleteFunc(pb.buffer, func(entry []byte) bool { return bytes.Equal(entry, pruneEntry) })
		if pb.isFull() {
			log.Warn("pruning buffer is full, keeping cancel prune", "rootHash", rootHash, "len", len(pb.buffer))
		}
	} else if pb.isFull() {
		log.Warn("pruning buffer is full", "rootHash", rootHash)
		return
	}

	pb.buffer = append(pb.buffer, rootHash)
	log.Trace("pruning buffer add", "rootHash", rootHash)
}

func (pb *pruningBuffer) removeAll() [][]byte {
	pb.mutOp.Lock()
	defer pb.mutOp.Unlock()

	log.Trace("pruning buffer", "len", len(pb.buffer))

	buffer := make([][]byte, len(pb.buffer))
	copy(buffer, pb.buffer)
	pb.buffer = pb.buffer[:0]

	return buffer
}

func (pb *pruningBuffer) len() int {
	pb.mutOp.RLock()
	defer pb.mutOp.RUnlock()

	return len(pb.buffer)
}

func (pb *pruningBuffer) isFull() bool {
	return uint32(len(pb.buffer)) >= pb.size // #nosec G115
}

func isCancelPrune(bufferedHash []byte) bool {
	return len(bufferedHash) > 0 && pruningOperation(bufferedHash[len(bufferedHash)-1]) == cancelPrune
}
