//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. block.proto

package block

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
	"google.golang.org/protobuf/proto"
)

var _ data.HeaderHandler = (*Block)(nil)

// IsInterfaceNil returns true if there is no value under the interface
func (b *Block) IsInterfaceNil() bool {
	return b == nil
}

// Clone will return a clone of the object
func (b *Block) Clone() data.HeaderHandler {
	return proto.Clone(b).(data.HeaderHandler)
}

// GetNonce -
func (b *Block) GetNonce() uint64 {
	if b.Header != nil {
		return b.Header.Nonce
	}
	return 0
}

// GetParentHash -
func (b *Block) GetParentHash() []byte {
	if b.Header != nil {
		return b.Header.ParentHash
	}
	return nil
}

// SetParentHash -
func (b *Block) SetParentHash(parentHash []byte) {
	b.Header.ParentHash = parentHash
}

// GetTimestamp -
func (b *Block) GetTimestamp() int64 {
	if b.Header != nil {
		return b.Header.Timestamp
	}
	return 0
}

// SetTimestamp -
func (b *Block) SetTimestamp(timestamp int64) {
	b.Header.Timestamp = timestamp
}

// GetSlot -
func (b *Block) GetSlot() uint64 {
	if b.Header != nil {
		return b.Header.Slot
	}
	return 0
}

// GetEpoch -
func (b *Block) GetEpoch() uint32 {
	if b.Header != nil {
		return b.Header.Epoch
	}
	return 0
}

// GetIsEpochStart -
func (b *Block) GetIsEpochStart() bool {
	if b.Header != nil {
		return b.Header.IsEpochStart
	}
	return false
}

// GetPrevEpochStartSlot -
func (b *Block) GetPrevEpochStartSlot() uint64 {
	if b.Header != nil {
		return b.Header.PrevEpochStartSlot
	}
	return 0
}

// GetTxRootHash -
func (b *Block) GetTxRootHash() []byte {
	if b.Header != nil {
		return b.Header.TxRootHash
	}
	return nil
}

// GetTrieRoot -
func (b *Block) GetTrieRoot() []byte {
	if b.Header != nil {
		return b.Header.TrieRoot
	}
	return nil
}

// SetTrieRoot -
func (b *Block) SetTrieRoot(hash []byte) {
	b.Header.TrieRoot = hash
}

// GetValidatorsTrieRoot -
func (b *Block) GetValidatorsTrieRoot() []byte {
	if b.Header != nil {
		return b.Header.ValidatorsTrieRoot
	}
	return nil
}

// GetKAppsTrieRoot -
func (b *Block) GetKAppsTrieRoot() []byte {
	if b.Header != nil {
		return b.Header.KAppsTrieRoot
	}
	return nil
}

// SetKAppsTrieRoot -
func (b *Block) SetKAppsTrieRoot(hash []byte) {
	b.Header.KAppsTrieRoot = hash
}

// GetPrevRandSeed -
func (b *Block) GetPrevRandSeed() []byte {
	if b.Header != nil {
		return b.Header.PrevRandSeed
	}
	return nil
}

// SetPrevRandSeed -
func (b *Block) SetPrevRandSeed(prevRandSeed []byte) {
	b.Header.PrevRandSeed = prevRandSeed
}

// GetRandSeed -
func (b *Block) GetRandSeed() []byte {
	if b.Header != nil {
		return b.Header.RandSeed
	}
	return nil
}

// SetRandSeed -
func (b *Block) SetRandSeed(randSeed []byte) {
	b.Header.RandSeed = randSeed
}

// GetTxCount -
func (b *Block) GetTxCount() uint32 {
	if b.Header != nil {
		return b.Header.TxCount
	}
	return 0
}

// GetSoftwareVersion -
func (b *Block) GetSoftwareVersion() []byte {
	if b.Header != nil {
		return b.Header.SoftwareVersion
	}
	return nil
}

// SetSoftwareVersion -
func (b *Block) SetSoftwareVersion(softwareVersion []byte) {
	b.Header.SoftwareVersion = softwareVersion
}

// GetChainID -
func (b *Block) GetChainID() []byte {
	if b.Header != nil {
		return b.Header.ChainID
	}
	return nil
}

// SetChainID -
func (b *Block) SetChainID(chainID []byte) {
	b.Header.ChainID = chainID
}

// SetPubKeysBitmap -
func (b *Block) SetPubKeysBitmap(signature []byte) {
	b.PubKeysBitmap = signature
}

// SetProducerSignature -
func (b *Block) SetProducerSignature(signature []byte) {
	b.ProducerSignature = signature
}

// SetSignature -
func (b *Block) SetSignature(signature []byte) {
	b.Signature = signature
}

// GetTxFees --
func (b *Block) GetTxFees() int64 {
	if b.Header != nil {
		return b.Header.TxFees
	}
	return 0
}

// GetTxBurnedFees --
func (b *Block) GetTxBurnedFees() int64 {
	if b.Header != nil {
		return b.Header.TxBurnedFees
	}
	return 0
}

// GetKAppFees --
func (b *Block) GetKAppFees() int64 {
	if b.Header != nil {
		return b.Header.KAppFees
	}
	return 0
}

// GetBlockRewards --
func (b *Block) GetBlockRewards() int64 {
	if b.Header != nil {
		return b.Header.BlockRewards
	}
	return 0
}

func (b *Block) GetBurnedUnclaimed() int64 {
	if b.Header != nil {
		return b.Header.BurnedUnclaimed
	}
	return 0
}

func (b *Block) SetBurnedUnclaimed(burned int64) {
	if b.Header != nil {
		b.Header.BurnedUnclaimed = burned
	}
}

// GetStakingRewards --
func (b *Block) GetStakingRewards() int64 {
	if b.Header != nil {
		return b.Header.StakingRewards
	}
	return 0
}

// GetReserved -
func (b *Block) GetReserved() []byte {
	if b.Header != nil {
		return b.Header.Reserved
	}
	return nil
}

// GetBlockHeader -
func (b *Block) GetBlockHeader() interface{} {
	return b.GetHeader()
}

// IsInterfaceNil returns true if there is no value under the interface
func (b *BlockHeader) IsInterfaceNil() bool {
	return b == nil
}

// Clone will return a clone of the object
func (b *BlockHeader) Clone() *BlockHeader {
	return proto.Clone(b).(*BlockHeader)
}

type merkletree struct {
	nodes  []node
	hasher hashing.Hasher
}

type node struct {
	left  []byte
	right []byte
}

func (n *node) Hash(hasher hashing.Hasher) []byte {
	return hasher.Compute(string(append(n.left, n.right...)))
}

func (mt *merkletree) GetRootHash() ([]byte, error) {
	if len(mt.nodes) == 0 {
		return mt.hasher.EmptyHash(), nil
	}
	if len(mt.nodes) == 1 {
		return mt.nodes[0].Hash(mt.hasher), nil
	}

	var nodes []node
	for i := 0; i < len(mt.nodes); i += 2 {
		left, right := i, i+1
		if i+1 == len(mt.nodes) {
			right = i
		}

		n := node{
			left:  mt.nodes[left].Hash(mt.hasher),
			right: mt.nodes[right].Hash(mt.hasher),
		}
		if len(mt.nodes) == 2 {
			return n.Hash(mt.hasher), nil
		}
		nodes = append(nodes, n)
	}

	root := &merkletree{
		nodes:  nodes,
		hasher: mt.hasher,
	}

	return root.GetRootHash()
}

// UpdateTxRootHash compute and update root hash
func (b *Block) UpdateTxRootHash(hasher hashing.Hasher) error {
	rootHash, err := b.ComputeRootHash(hasher)
	if err != nil {
		return err
	}

	b.Header.TxRootHash = rootHash
	return nil
}

// ComputeRootHash calculates root hash of slice
func (b *Block) ComputeRootHash(hasher hashing.Hasher) ([]byte, error) {
	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}

	mt := &merkletree{
		hasher: hasher,
		nodes:  make([]node, 0),
	}

	var size = len(b.TxHashes)

	for i := 0; i < size; i += 2 {
		left, right := i, i+1
		if i+1 == size {
			right = i
		}

		mt.nodes = append(mt.nodes, node{
			left:  b.TxHashes[left],
			right: b.TxHashes[right],
		})
	}

	return mt.GetRootHash()
}
