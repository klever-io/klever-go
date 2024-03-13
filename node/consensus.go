package node

import (
	"github.com/klever-io/klever-go/core/consensus/slot"
)

// createConsensusState method creates a consensusState object
func (n *Node) createConsensusState(epoch uint32) (*slot.ConsensusState, error) {
	selfId, err := n.pubKey.ToByteArray()
	if err != nil {
		return nil, err
	}

	eligibleNodesPubKeys, err := n.nodesCoordinator.GetConsensusWhitelistedNodes(epoch)
	if err != nil {
		return nil, err
	}

	slotConsensus := slot.NewSlotConsensus(
		eligibleNodesPubKeys,
		n.consensusGroupSize,
		string(selfId))

	slotConsensus.ResetSlotState()

	slotThreshold := slot.NewSlotThreshold()

	slotStatus := slot.NewSlotStatus()
	slotStatus.ResetSlotStatus()

	consensusState := slot.NewConsensusState(
		slotConsensus,
		slotThreshold,
		slotStatus)

	return consensusState, nil
}
