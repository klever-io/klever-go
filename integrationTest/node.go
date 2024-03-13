package integrationTest

import (
	"fmt"
	"testing"
	"time"

	mclsinglesig "github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	"github.com/klever-io/klever-go/tools/check"
)

// ProposeAndSyncOneBlock proposes a block, syncs the block and then increments the slot
func ProposeAndSyncOneBlock(
	t *testing.T,
	nodes []*processorNode.ProcessorNode,
	xidProposer int,
	slot uint64,
	nonce uint64,
) (uint64, uint64, []*processorNode.ProcessorNode, error) {

	currHdr := nodes[0].Blkc.GetCurrentBlockHeader()
	if check.IfNil(currHdr) {
		currHdr = nodes[0].Blkc.GetGenesisHeader()
	}

	prevRandomness := currHdr.GetRandSeed()
	epoch := currHdr.GetEpoch()

	pubKeys, err := nodes[0].NodesCoordinator.GetConsensusValidatorsPublicKeys(prevRandomness, slot, epoch)
	if err != nil {
		return 0, 0, nil, err
	}

	consensusNodes := processorNode.SelectTestNodesForPubKeys(nodes, pubKeys)

	UpdateSlot(consensusNodes, slot)
	err = ProposeBlock(consensusNodes, pubKeys, xidProposer, slot, nonce)
	if err != nil {
		return 0, 0, nil, err
	}
	err = SyncBlock(consensusNodes, xidProposer, slot)
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
	log.Info("------ block  proposed ------ \n")
	return nil
}

// SyncBlock synchronizes the proposed block in all the other shard nodes
func SyncBlock(
	nodes []*processorNode.ProcessorNode,
	xidProposer int,
	slot uint64,
) error {

	log.Info("All nodes sync the proposed block...")
	for i, n := range nodes {
		if i == xidProposer {
			continue
		}

		err := n.SyncNode(slot)
		if err != nil {
			log.Warn(fmt.Sprintf("SyncNode on slot %v could not be synced. Error: %s", slot, err.Error()))
			if err != nil {
				return err
			}
			continue
		}
	}

	time.Sleep(StepDelay)
	log.Info("---------- block synchronized ----------\n")
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
