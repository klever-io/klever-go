package integrationTest

import (
	"fmt"
	"testing"
	"time"

	mclsinglesig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/klever-io/klever-go/tools/check"
)

var PushToProposerPool = make(map[string]*transaction.Transaction)

// ProposeAndSyncOneBlock proposes a block, syncs the block and then increments the slot
func ProposeAndSyncOneBlock(
	t *testing.T,
	nodes []*processorNode.ProcessorNode,
	xidProposer int,
	slot uint64,
	nonce uint64,
) (uint64, uint64, []*processorNode.ProcessorNode, error) {

	currHdr := nodes[xidProposer].Blkc.GetCurrentBlockHeader()
	if check.IfNil(currHdr) {
		currHdr = nodes[xidProposer].Blkc.GetGenesisHeader()
	}

	prevRandomness := currHdr.GetRandSeed()
	epoch := currHdr.GetEpoch()

	pubKeys, err := nodes[xidProposer].NodesCoordinator.GetConsensusValidatorsPublicKeys(prevRandomness, slot, epoch)
	if err != nil {
		return 0, 0, nil, err
	}

	consensusNodes := processorNode.SelectTestNodesForPubKeys(nodes, pubKeys)

	// push TX to pool if any
	if len(PushToProposerPool) > 0 {
		log.Info("***** PUSHING TX TO PROPOSER POOL *****")
		for txHash, tx := range PushToProposerPool {
			consensusNodes[xidProposer].DataPool.Transactions().AddData([]byte(txHash), tx, 100, "0")
			log.Info("Pushed TX to proposer pool", "txHash", txHash)
			delete(PushToProposerPool, txHash)
		}
	}

	UpdateSlot(consensusNodes, slot)
	err = ProposeBlock(consensusNodes, pubKeys, xidProposer, slot, nonce)
	if err != nil {
		return 0, 0, nil, err
	}

	log.Info("Sync proposed block...", "slot", slot, "nonce", nonce)

	err = SyncBlock(consensusNodes, xidProposer, nonce)
	if err != nil {
		return 0, 0, nil, err
	}

	slot = IncrementAndPrintSlot(slot)
	nonce++

	return slot, nonce, consensusNodes, nil
}

// ProposeBlock proposes a block for every shard
func ProposeBlock(nodes []*processorNode.ProcessorNode, pubKeys []string, xidProposer int, slot uint64, nonce uint64) error {
	log.Info("All nodes propose blocks...")
	singleSigner := &mclsinglesig.BlsSingleSigner{}
	stepDelayAdjustment := StepDelay * time.Duration(1+len(nodes)/3)

	for i, n := range nodes {
		if i != xidProposer {
			continue
		}

		header, err := processorNode.ProposeBlockWithConsensusSigs(nodes, pubKeys, xidProposer, slot, nonce)
		if err != nil {
			return err
		}

		header, err = n.FillHeaderFields(nodes[xidProposer], header, singleSigner)
		if err != nil {
			return err
		}

		err = n.BroadcastBlock(header)
		if err != nil {
			return err
		}

		err = n.CommitBlock(header)
		if err != nil {
			return err
		}
	}

	log.Info("Delaying for disseminating headers...")
	time.Sleep(stepDelayAdjustment)
	log.Info("------ block  proposed ------", "slot", slot, "nonce", nonce, "hash", nodes[xidProposer].Blkc.GetCurrentBlockHeaderHash())
	return nil
}

// SyncBlock synchronizes the proposed block in all the other shard nodes
func SyncBlock(
	nodes []*processorNode.ProcessorNode,
	xidProposer int,
	nonce uint64,
) error {

	log.Info("All nodes sync the proposed block...")
	for i, n := range nodes {
		if i == xidProposer {
			continue
		}

		err := n.SyncNode(nonce)
		if err != nil {
			log.Warn(fmt.Sprintf("SyncNode (%d) on nonce %d could not be synced. Error: %s", i, nonce, err.Error()))

			continue
		}
	}

	time.Sleep(StepDelay)
	log.Info("---------- block synchronized ----------", "nonce", nonce)
	return nil
}

func RevertOneBlock(nodes []*processorNode.ProcessorNode, nonce uint64) error {
	for i, n := range nodes {
		log.Warn("RevertOneBlock", "node", i, "nonce", nonce)
		err := n.RevertOneBlock(nonce)
		if err != nil {
			log.Error("RevertOneBlock", "node", i, "nonce", nonce, "error", err.Error())
			return err
		}
	}
	return nil
}

// CreateNodes creates multiple nodes in different shards
func CreateNodes(
	numNodes int,
) []*processorNode.ProcessorNode {

	nodes := make([]*processorNode.ProcessorNode, numNodes)
	connectableNodes := make([]processorNode.Connectable, len(nodes))

	mainConfig, err := LoadConfig("../integrationTest/config.yaml")
	if err != nil {
		return nil
	}

	enableEpochs, err := LoadEnableEpochsConfig("../integrationTest/enableEpochs.yaml")
	if err != nil {
		return nil
	}

	mainConfig.EnableEpochs = enableEpochs.EnableEpochs
	mainConfig.GasScheduleConfig = enableEpochs.GasSchedule

	for i := 0; i < numNodes; i++ {
		metaNode, err := processorNode.NewBaseProcessorNode(mainConfig)
		if err != nil {
			return nil
		}

		connectableNodes[i] = metaNode
	}

	processorNode.ConnectNodes(connectableNodes)

	return nodes
}
