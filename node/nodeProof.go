package node

import (
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
)

// GetProof returns a Merkle proof of address under the current accounts root.
func (n *Node) GetProof(address string) (*state.MerkleProof, error) {
	key, err := n.decodeProofAddress(address)
	if err != nil {
		return nil, err
	}
	prover, err := n.merkleProver()
	if err != nil {
		return nil, err
	}
	return prover.GetMerkleProof(key)
}

// GetProofForRootHash returns a Merkle proof of address under rootHash.
// rootHash is the raw state root. The live accounts trie is not replaced.
func (n *Node) GetProofForRootHash(rootHash []byte, address string) (*state.MerkleProof, error) {
	key, err := n.decodeProofAddress(address)
	if err != nil {
		return nil, err
	}
	prover, err := n.merkleProver()
	if err != nil {
		return nil, err
	}
	return prover.GetMerkleProofAtRoot(rootHash, key)
}

// VerifyProof reports whether proof is a valid inclusion proof of address under rootHash.
func (n *Node) VerifyProof(rootHash []byte, address string, proof [][]byte) (bool, error) {
	key, err := n.decodeProofAddress(address)
	if err != nil {
		return false, err
	}
	prover, err := n.merkleProver()
	if err != nil {
		return false, err
	}
	return prover.VerifyMerkleProof(rootHash, key, proof)
}

func (n *Node) decodeProofAddress(address string) ([]byte, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return nil, fmt.Errorf("%w for addressPubkeyConverter", common.ErrNilPubkeyConverter)
	}
	addr, err := n.addressPubkeyConverter.Decode(address)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", state.ErrInvalidProofRequest, err.Error())
	}
	if len(addr) == 0 {
		return nil, fmt.Errorf("%w: empty address", state.ErrInvalidProofRequest)
	}
	return addr, nil
}

func (n *Node) merkleProver() (state.MerkleProver, error) {
	if check.IfNil(n.accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	prover, ok := n.accounts.(state.MerkleProver)
	if !ok {
		return nil, errors.New("accounts adapter does not support merkle proofs")
	}
	return prover, nil
}
