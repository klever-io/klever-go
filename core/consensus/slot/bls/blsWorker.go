package bls

import (
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
)

// peerMaxMessagesPerSec defines how many messages can be propagated by a pid in a slot. The value was chosen by
// following the next premises:
//  1. a leader can propagate as maximum as 3 messages per slot: proposed header block + proposed body + final info;
//  2. due to the fact that a delayed signature of the proposer (from previous slot) can be received in the current slot
//     adds an extra 1 to the total value, reaching value 4;
//  3. Because the leader might be selected in the next slot and might have an empty data pool, it can send the newly
//     empty proposed block at the very beginning of the next slot. One extra message here, yielding to a total of 5.
//  4. If we consider the forks that can appear on the system wee need to add one more to the value.
//
// Validators only send one signature message in a slot, treating the edge case of a delayed message, will need at most
// 2 messages per slot (which is ok as it is below the set value of 5)
const peerMaxMessagesPerSec = uint32(6)

// worker defines the data needed by spos to communicate between nodes which are in the validators group
type worker struct {
}

// NewConsensusService creates a new worker object
func NewConsensusService() (*worker, error) {
	wrk := worker{}

	return &wrk, nil
}

// InitReceivedMessages initializes the MessagesType map for all messages for the current ConsensusService
func (wrk *worker) InitReceivedMessages() map[consensus.MessageType][]*consensus.Message {
	receivedMessages := make(map[consensus.MessageType][]*consensus.Message)
	receivedMessages[MtBlockBodyAndHeader] = make([]*consensus.Message, 0)
	receivedMessages[MtBlockBody] = make([]*consensus.Message, 0)
	receivedMessages[MtBlockHeader] = make([]*consensus.Message, 0)
	receivedMessages[MtSignature] = make([]*consensus.Message, 0)
	receivedMessages[MtBlockHeaderFinalInfo] = make([]*consensus.Message, 0)

	return receivedMessages
}

// GetMaxMessagesInASlotPerPeer returns the maximum number of messages a peer can send per slot for BLS
func (wrk *worker) GetMaxMessagesInASlotPerPeer() uint32 {
	return peerMaxMessagesPerSec
}

// GetStringValue gets the name of the messageType
func (wrk *worker) GetStringValue(messageType consensus.MessageType) string {
	return getStringValue(messageType)
}

// GetSubslotName gets the subslot name for the subslot id provided
func (wrk *worker) GetSubslotName(subslotId int) string {
	return getSubslotName(subslotId)
}

// IsMessageWithBlockBodyAndHeader returns if the current messageType is about block body and header
func (wrk *worker) IsMessageWithBlockBodyAndHeader(msgType consensus.MessageType) bool {
	return msgType == MtBlockBodyAndHeader
}

// IsMessageWithBlockBody returns if the current messageType is about block body
func (wrk *worker) IsMessageWithBlockBody(msgType consensus.MessageType) bool {
	return msgType == MtBlockBody
}

// IsMessageWithBlockHeader returns if the current messageType is about block header
func (wrk *worker) IsMessageWithBlockHeader(msgType consensus.MessageType) bool {
	return msgType == MtBlockHeader
}

// IsMessageWithSignature returns if the current messageType is about signature
func (wrk *worker) IsMessageWithSignature(msgType consensus.MessageType) bool {
	return msgType == MtSignature
}

// IsMessageWithFinalInfo returns if the current messageType is about header final info
func (wrk *worker) IsMessageWithFinalInfo(msgType consensus.MessageType) bool {
	return msgType == MtBlockHeaderFinalInfo
}

// IsMessageTypeValid returns if the current messageType is valid
func (wrk *worker) IsMessageTypeValid(msgType consensus.MessageType) bool {
	isMessageTypeValid := msgType == MtBlockBodyAndHeader ||
		msgType == MtBlockBody ||
		msgType == MtBlockHeader ||
		msgType == MtSignature ||
		msgType == MtBlockHeaderFinalInfo

	return isMessageTypeValid
}

// IsSubslotSignature returns if the current subslot is about signature
func (wrk *worker) IsSubslotSignature(subslotId int) bool {
	return subslotId == SrSignature
}

// IsSubslotStartSlot returns if the current subslot is about start slot
func (wrk *worker) IsSubslotStartSlot(subslotId int) bool {
	return subslotId == SrStartSlot
}

// GetMessageRange provides the MessageType range used in checks by the consensus
func (wrk *worker) GetMessageRange() []consensus.MessageType {
	var v []consensus.MessageType

	for i := MtBlockBodyAndHeader; i <= MtBlockHeaderFinalInfo; i++ {
		v = append(v, i)
	}

	return v
}

// CanProceed returns if the current messageType can proceed further if previous subslots finished
func (wrk *worker) CanProceed(consensusState *slot.ConsensusState, msgType consensus.MessageType) bool {
	switch msgType {
	case MtBlockBodyAndHeader:
		return consensusState.Status(SrStartSlot) == slot.SsFinished
	case MtBlockBody:
		return consensusState.Status(SrStartSlot) == slot.SsFinished
	case MtBlockHeader:
		return consensusState.Status(SrStartSlot) == slot.SsFinished
	case MtSignature:
		return consensusState.Status(SrBlock) == slot.SsFinished
	case MtBlockHeaderFinalInfo:
		return consensusState.Status(SrSignature) == slot.SsFinished
	}

	return false
}

// IsInterfaceNil returns true if there is no value under the interface
func (wrk *worker) IsInterfaceNil() bool {
	return wrk == nil
}
