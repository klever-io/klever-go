package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

// ErrStateRootUnavailable is returned when the requested state root is not in trie
// storage. Pruning, and a node that never stored that root, both surface as this error.
var ErrStateRootUnavailable = errors.New("state root is not available")

// ErrInvalidProofRequest is returned when a proof call is missing its key or root.
var ErrInvalidProofRequest = errors.New("invalid proof request")

// MerkleProof is an inclusion proof for one key under a state root.
//
// External verification uses the chain hasher (blake2b in config/node/config.yaml) over
// the raw encoded trie node bytes: hasher.Compute(string(node)). The first node hashes
// to RootHash, and each following node hashes to the child selected by the key. The key
// is the raw account address, with nibbles reversed and a hex terminator appended
// (data/trie keyBytesToHex). Value is the leaf bytes stored at that key, which for an
// account is the protobuf-encoded account. A valid proof shows inclusion under RootHash
// only; RootHash itself must come from a finalized block header.
type MerkleProof struct {
	RootHash []byte
	Value    []byte
	Proof    [][]byte
}

// MerkleProver builds and checks account-trie proofs without replacing the live trie.
type MerkleProver interface {
	GetMerkleProof(key []byte) (*MerkleProof, error)
	GetMerkleProofAtRoot(rootHash []byte, key []byte) (*MerkleProof, error)
	VerifyMerkleProof(rootHash []byte, key []byte, proof [][]byte) (bool, error)
}

var _ MerkleProver = (*AccountsDB)(nil)

// GetMerkleProof returns a proof of key under the current accounts root.
func (adb *AccountsDB) GetMerkleProof(key []byte) (*MerkleProof, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("%w in GetMerkleProof", common.ErrNilAddress)
	}

	adb.mutOp.Lock()
	defer adb.mutOp.Unlock()

	if check.IfNil(adb.mainTrie) {
		return nil, common.ErrNilTrie
	}
	return proveOnTrie(adb.mainTrie, nil, key)
}

// GetMerkleProofAtRoot returns a proof of key under rootHash.
// It recreates a trie for that root and does not call RecreateTrie, so the live
// accounts trie and its journal stay as they were.
func (adb *AccountsDB) GetMerkleProofAtRoot(rootHash []byte, key []byte) (*MerkleProof, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("%w in GetMerkleProofAtRoot", common.ErrNilAddress)
	}

	var proof *MerkleProof
	err := adb.withTrieAtRoot(rootHash, func(tr data.Trie) error {
		var proveErr error
		proof, proveErr = proveOnTrie(tr, rootHash, key)
		return proveErr
	})
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// VerifyMerkleProof reports whether proof shows key included under rootHash.
// A well-formed request with a non-matching proof returns false. A root that is not
// in storage returns ErrStateRootUnavailable.
func (adb *AccountsDB) VerifyMerkleProof(rootHash []byte, key []byte, proof [][]byte) (bool, error) {
	if len(key) == 0 {
		return false, fmt.Errorf("%w in VerifyMerkleProof", common.ErrNilAddress)
	}

	var ok bool
	err := adb.withTrieAtRoot(rootHash, func(tr data.Trie) error {
		var verifyErr error
		ok, verifyErr = tr.VerifyProof(key, proof)
		return verifyErr
	})
	return ok, err
}

// withTrieAtRoot runs fn on the live trie when rootHash is the current root, and on a
// trie from Trie.Recreate otherwise. The accounts lock is held only while the live trie
// is in use. RecreateTrie is not called.
func (adb *AccountsDB) withTrieAtRoot(rootHash []byte, fn func(data.Trie) error) error {
	if len(rootHash) == 0 {
		return fmt.Errorf("%w: empty root hash", ErrInvalidProofRequest)
	}

	adb.mutOp.Lock()
	if check.IfNil(adb.mainTrie) {
		adb.mutOp.Unlock()
		return common.ErrNilTrie
	}

	current, err := adb.mainTrie.RootHash()
	if err != nil {
		adb.mutOp.Unlock()
		return err
	}
	if bytes.Equal(current, rootHash) {
		err = fn(adb.mainTrie)
		adb.mutOp.Unlock()
		return err
	}

	tr, err := adb.mainTrie.Recreate(rootHash)
	adb.mutOp.Unlock()
	if err != nil {
		if errors.Is(err, data.ErrHashNotFound) {
			return fmt.Errorf("%w: %w", ErrStateRootUnavailable, err)
		}
		return err
	}
	if check.IfNil(tr) {
		return ErrStateRootUnavailable
	}
	return fn(tr)
}

func proveOnTrie(tr data.Trie, root []byte, key []byte) (*MerkleProof, error) {
	actual, err := tr.RootHash()
	if err != nil {
		return nil, err
	}
	if len(root) > 0 && !bytes.Equal(actual, root) {
		return nil, fmt.Errorf("%w: recreated root does not match the requested root", ErrStateRootUnavailable)
	}

	value, err := tr.Get(key)
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return nil, common.ErrAccNotFound
	}

	nodes, err := tr.GetProof(key)
	if err != nil {
		return nil, err
	}

	return &MerkleProof{
		RootHash: cloneBytes(actual),
		Value:    cloneBytes(value),
		Proof:    cloneProof(nodes),
	}, nil
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	return append([]byte(nil), b...)
}

func cloneProof(proof [][]byte) [][]byte {
	if proof == nil {
		return nil
	}
	out := make([][]byte, len(proof))
	for i, node := range proof {
		out[i] = cloneBytes(node)
	}
	return out
}
