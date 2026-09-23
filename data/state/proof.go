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

// MerkleProof is an inclusion proof for one key under a state root. A client can check
// it without the node:
//
//  1. Key: take the raw account address, reverse its bytes, and emit each byte's low
//     nibble then its high nibble; append the terminator nibble 16
//     (data/trie keyBytesToHex).
//  2. Hash: each node must hash to the expected hash, starting with RootHash. The hash
//     is unkeyed blake2b-256 (config/node/config.yaml hasher) over the full node bytes.
//  3. Decode: the last byte of a node is its type (0 extension, 1 leaf, 2 branch); the
//     bytes before it are the protobuf CollapsedEn{Key, EncodedChild},
//     CollapsedLn{Key, Value} or CollapsedBn{EncodedChildren} (data/trie/proto/node.proto).
//  4. Walk: a branch always has 17 EncodedChildren slots; the next hash is
//     EncodedChildren[key[0]] and one nibble is consumed. An extension's Key must prefix
//     the remaining key; the next hash is EncodedChild and len(Key) nibbles are consumed.
//     A leaf ends the proof; its Key must equal the remaining key, terminator included.
//  5. Value: the leaf Value is the protobuf-encoded account and must equal Value.
//     VerifyMerkleProof checks steps 1-4 only.
//
// A valid proof shows inclusion under RootHash only; RootHash itself must come from a
// finalized block header.
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

// GetMerkleProof returns a proof of key under the last committed accounts root, so
// uncommitted changes are never proven.
func (adb *AccountsDB) GetMerkleProof(key []byte) (*MerkleProof, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("%w in GetMerkleProof", common.ErrNilAddress)
	}

	adb.mutOp.Lock()
	committed := cloneBytes(adb.lastRootHash)
	adb.mutOp.Unlock()

	if len(committed) == 0 {
		return nil, ErrStateRootUnavailable
	}
	return adb.GetMerkleProofAtRoot(committed, key)
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
// trie recreated from the main trie DB otherwise; snapshots are never used, since
// recreating from one copies the whole trie into the main DB. RecreateTrie is not
// called. The accounts lock is held until fn returns, so PruneTrie cannot remove nodes
// of a historical root mid-walk.
func (adb *AccountsDB) withTrieAtRoot(rootHash []byte, fn func(data.Trie) error) error {
	if len(rootHash) == 0 {
		return fmt.Errorf("%w: empty root hash", ErrInvalidProofRequest)
	}

	adb.mutOp.Lock()
	defer adb.mutOp.Unlock()

	if check.IfNil(adb.mainTrie) {
		return common.ErrNilTrie
	}

	current, err := adb.mainTrie.RootHash()
	if err != nil {
		return err
	}
	if bytes.Equal(current, rootHash) {
		return fn(adb.mainTrie)
	}

	tr, err := adb.mainTrie.RecreateFromMainDb(rootHash)
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
