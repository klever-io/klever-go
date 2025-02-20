package processorNode

import (
	"errors"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

func ProposeBlockWithConsensusSigs(nodes []*ProcessorNode, pubKeys []string, indexProposer int, slot uint64, nonce uint64) (data.HeaderHandler, error) {
	startTime := time.Now()
	maxTime := time.Second * 4

	haveTime := func() bool {
		elapsedTime := time.Since(startTime)
		remainingTime := maxTime - elapsedTime
		return remainingTime > 0
	}

	sizeConsensus := len(nodes)

	bitmapSize := sizeConsensus / 8
	if sizeConsensus%8 != 0 {
		bitmapSize++
	}
	bitmap := make([]byte, bitmapSize)

	for i := 0; i < sizeConsensus; i++ {
		bitmap[i/8] |= 1 << (uint16(i) % 8) // #nosec G115
	}

	var blk = &block.Block{
		Header: &block.BlockHeader{
			Slot:  slot,
			Nonce: nonce,
		},
		PubKeysBitmap: bitmap,
	}

	currHdr := nodes[indexProposer].Blkc.GetCurrentBlockHeader()
	var parentHash []byte
	if check.IfNil(currHdr) {
		currHdr = nodes[indexProposer].Blkc.GetGenesisHeader()

		buff, err := TestMarshalizer.Marshal(currHdr.GetBlockHeader())
		if err != nil {
			return nil, err
		}

		parentHash = TestHasher.Compute(string(buff))
	} else {
		parentHash = nodes[indexProposer].Blkc.GetCurrentBlockHeaderHash()
	}

	blk.SetParentHash(parentHash)
	prevRandomness := currHdr.GetRandSeed()

	randSeed, err := nodes[indexProposer].NodeAccount.BlockSingleSigner.Sign(nodes[indexProposer].NodeBlockSignKeyPair.Sk, prevRandomness)
	if err != nil {
		return nil, err
	}

	genesisSlot := uint64(0) //Todo: add it in other place
	// #nosec G115
	finalTimestamp := nodes[indexProposer].SlotManager.Timestamp().Unix() + int64((slot-genesisSlot)*uint64(nodes[indexProposer].SlotManager.TimeDuration().Seconds()))
	blk.SetTimestamp(finalTimestamp)
	blk.SetPrevRandSeed(prevRandomness)
	blk.SetRandSeed(randSeed)
	blk.SetChainID(ChainID)

	blockCreated, err := nodes[indexProposer].BlockProcessor.CreateBlock(blk, haveTime)
	if err != nil {
		return nil, err
	}

	// clear signature, as we need to compute it below
	blockCreated.SetSignature(nil)

	blockCreated.SetPubKeysBitmap(nil)

	blockHeaderHash, err := tools.CalculateHash(TestMarshalizer, TestHasher, blk.Header)
	if err != nil {
		return nil, err
	}

	var msig crypto.MultiSigner
	msigProposer, err := nodes[indexProposer].MultiSigner.Create(pubKeys, 0)
	if err != nil {
		return nil, err
	}
	_, err = msigProposer.CreateSignatureShare(blockHeaderHash, bitmap)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(nodes); i++ {
		msig, _ = nodes[i].MultiSigner.Create(pubKeys, uint16(i)) // #nosec G115
		sigShare, _ := msig.CreateSignatureShare(blockHeaderHash, bitmap)
		_ = msigProposer.StoreSignatureShare(uint16(i), sigShare) // #nosec G115
	}

	sig, err := msigProposer.AggregateSigs(bitmap)
	if err != nil {
		return nil, err
	}

	blockCreated.SetSignature(sig)

	blockCreated.SetPubKeysBitmap(bitmap)

	return blockCreated, nil
}

// DoConsensusSigningOnBlock simulates a consensus aggregated signature on the provided block
func DoConsensusSigningOnBlock(
	blockHeader data.HeaderHandler,
	consensusNodes []*ProcessorNode,
	pubKeys []string,
) (data.HeaderHandler, error) {
	// set bitmap for all consensus nodes signing
	bitmap := make([]byte, len(consensusNodes)/8+1)
	for i := range bitmap {
		bitmap[i] = 0xFF
	}

	bitmap[len(consensusNodes)/8] >>= uint8(8 - (len(consensusNodes) % 8)) // #nosec G115
	blockHeader.SetPubKeysBitmap(bitmap)

	// clear signature, as we need to compute it below
	blockHeader.SetSignature(nil)
	blockHeader.SetPubKeysBitmap(nil)

	blockHeaderHash, err := tools.CalculateHash(TestMarshalizer, TestHasher, blockHeader)
	if err != nil {
		return nil, err
	}

	var msig crypto.MultiSigner
	msigProposer, err := consensusNodes[0].MultiSigner.Create(pubKeys, 0)
	if err != nil {
		return nil, err
	}

	_, err = msigProposer.CreateSignatureShare(blockHeaderHash, bitmap)
	if err != nil {
		return nil, err
	}

	for i := 1; i < len(consensusNodes); i++ {
		msig, _ = consensusNodes[i].MultiSigner.Create(pubKeys, uint16(i)) // #nosec G115
		sigShare, _ := msig.CreateSignatureShare(blockHeaderHash, bitmap)
		_ = msigProposer.StoreSignatureShare(uint16(i), sigShare) // #nosec G115
	}

	sig, err := msigProposer.AggregateSigs(bitmap)
	if err != nil {
		return nil, err
	}

	blockHeader.SetSignature(sig)

	blockHeader.SetPubKeysBitmap(bitmap)

	blockHeader.SetProducerSignature([]byte("leader sign"))

	blockHeader.SetChainID(ChainID)

	return blockHeader, nil
}

func (n *ProcessorNode) GetBlockByNonce(nonce uint64) (*api.Block, error) {
	return n.Node.GetBlockByNonce(nonce, false)
}

// GetBlock returns the first data.HeaderHandler stored in datapools having the nonce provided as parameter
func (n *ProcessorNode) GetBlock(nonce uint64) (data.HeaderHandler, error) {
	invalidCachers := n.DataPool == nil || n.DataPool.Headers() == nil
	if invalidCachers {
		return nil, errors.New("invalid data pool")
	}

	headerObjects, _, err := n.DataPool.Headers().GetHeadersByNonce(nonce)
	if err != nil {
		// try from storer
		header, _, err := process.GetHeaderFromStorageWithNonce(nonce, n.Store, n.Uint64ByteSliceConverter, n.InternalMarshalizer)
		if err != nil {
			return nil, fmt.Errorf("no headers found for nonce %d %s", nonce, err.Error())
		}
		return header, nil
	}

	headerObject := headerObjects[len(headerObjects)-1]

	return headerObject, nil
}

// BroadcastBlock broadcasts the block and body to the connected peers
func (n *ProcessorNode) BroadcastBlock(header data.HeaderHandler) error {
	err := n.BroadcastMessenger.BroadcastBlock(header)
	if err != nil {
		return err
	}

	time.Sleep(WaitTime)

	_, transactions, err := n.BlockProcessor.MarshalizedDataToBroadcast(header)
	if err != nil {
		return err
	}
	err = n.BroadcastMessenger.BroadcastTransactions(transactions)
	if err != nil {
		return err
	}

	return nil
}

// CommitBlock commits the block
func (n *ProcessorNode) CommitBlock(header data.HeaderHandler) error {
	err := n.BlockProcessor.CommitBlock(header)
	if err != nil {
		return err
	}

	return err
}

// ProposeBlockWithConsensusSignature proposes
func ProposeBlockWithConsensusSignature(
	nodes []*ProcessorNode,
	slot uint64,
	nonce uint64,
	randomness []byte,
	epoch uint32,
) (data.HeaderHandler, []*ProcessorNode, error) {
	nodesCoordinatorInstance := nodes[0].NodesCoordinator

	pubKeys, err := nodesCoordinatorInstance.GetConsensusValidatorsPublicKeys(randomness, slot, epoch)
	if err != nil {
		return nil, nil, err
	}

	// select nodes from map based on their pub keys
	consensusNodes := SelectTestNodesForPubKeys(nodes, pubKeys)
	// first node is block proposer
	header, err := consensusNodes[0].ProposeBlock(slot, nonce)
	if err != nil {
		return nil, nil, err
	}

	header.SetPrevRandSeed(randomness)

	header, err = DoConsensusSigningOnBlock(header, consensusNodes, pubKeys)
	if err != nil {
		return nil, nil, err
	}

	return header, consensusNodes, nil
}

func (n *ProcessorNode) ProposeBlock(slot uint64, nonce uint64) (data.HeaderHandler, error) {
	startTime := time.Now()
	maxTime := time.Second * 2

	haveTime := func() bool {
		elapsedTime := time.Since(startTime)
		remainingTime := maxTime - elapsedTime
		return remainingTime > 0
	}

	var blk = &block.Block{
		Header: &block.BlockHeader{
			Slot:  slot,
			Nonce: nonce,
		},
		PubKeysBitmap: []byte{1},
	}

	currHdr := n.Blkc.GetCurrentBlockHeader()
	if check.IfNil(currHdr) {
		currHdr = n.Blkc.GetGenesisHeader()
	}

	buff, err := n.InternalMarshalizer.Marshal(currHdr)
	if err != nil {
		return nil, err
	}

	blk.SetParentHash(TestHasher.Compute(string(buff)))

	genesisSlot := uint64(0) //Todo: add it in other place
	// #nosec G115
	finalTimestamp := n.SlotManager.Timestamp().Unix() + int64((slot-genesisSlot)*uint64(n.SlotManager.TimeDuration().Seconds()))
	blk.SetTimestamp(finalTimestamp)

	blk.SetPrevRandSeed(currHdr.GetRandSeed())

	blk.SetSignature([]byte("aggregate signature"))

	blk.SetRandSeed([]byte("aggregate signature"))

	blk.SetProducerSignature([]byte("leader sign"))

	blk.SetChainID(ChainID)

	blk.TxHashes = make([][]byte, 0)

	blockCreated, err := n.BlockProcessor.CreateBlock(blk, haveTime)
	if err != nil {
		return nil, err
	}

	return blockCreated, nil
}
