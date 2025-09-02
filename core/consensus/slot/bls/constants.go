package bls

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core/consensus"
)

var log = logger.GetOrCreate("consensus/spos/bls")

const (
	// SrStartSlot defines ID of Subslot "Start slot"
	SrStartSlot = iota
	// SrBlock defines ID of Subslot "block"
	SrBlock
	// SrSignature defines ID of Subslot "signature"
	SrSignature
	// SrEndSlot defines ID of Subslot "End slot"
	SrEndSlot
)

const (
	// MtUnknown defines ID of a message that has unknown data inside
	MtUnknown consensus.MessageType = iota
	// MtBlockHeader defines ID of a message that has and a block header inside
	MtBlockHeader
	// DEPRECATED: MtBlockBody defines ID of a message that has a block body inside
	MtBlockBody
	// DEPRECATED: MtBlockBodyAndHeader defines ID of a message that has a block body and header inside
	MtBlockBodyAndHeader
	// MtSignature defines ID of a message that has a Signature inside
	MtSignature
	// MtBlockHeaderFinalInfo defines ID of a message that has a block header final info inside
	// (aggregate signature, bitmap and seal leader signature for the proposed and accepted header)
	MtBlockHeaderFinalInfo
)

// waitingAllSigsMaxTimeThreshold specifies the max allocated time for waiting all signatures from the total time of the subslot signature
const waitingAllSigsMaxTimeThreshold = 0.5

// processingThresholdPercent specifies the max allocated time for processing the block as a percentage of the total time of the slot
const processingThresholdPercent = 85

// srStartStartTime specifies the start time, from the total time of the slot, of Subslot Start
const srStartStartTime = 0.0

// srEndStartTime specifies the end time, from the total time of the slot, of Subslot Start
const srStartEndTime = 0.05

// srBlockStartTime specifies the start time, from the total time of the slot, of Subslot Block
const srBlockStartTime = 0.05

// srBlockEndTime specifies the end time, from the total time of the slot, of Subslot Block
const srBlockEndTime = 0.3

// srSignatureStartTime specifies the start time, from the total time of the slot, of Subslot Signature
const srSignatureStartTime = 0.3

// srSignatureEndTime specifies the end time, from the total time of the slot, of Subslot Signature
const srSignatureEndTime = 0.80

// srEndStartTime specifies the start time, from the total time of the slot, of Subslot End
const srEndStartTime = 0.80

// srEndEndTime specifies the end time, from the total time of the slot, of Subslot End
const srEndEndTime = 0.95

const (
	// BlockHeaderStringValue represents the string to be used to identify a block header
	BlockHeaderStringValue = "(BLOCK_HEADER)"

	// BlockBodyStringValue represents the string to be used to identify a block body
	BlockBodyStringValue = "(BLOCK_BODY)"

	// BlockSignatureStringValue represents the string to be used to identify a block's signature
	BlockSignatureStringValue = "(SIGNATURE)"

	// BlockHeaderFinalInfoStringValue represents the string to be used to identify a block's header final info
	BlockHeaderFinalInfoStringValue = "(FINAL_INFO)"

	// BlockUnknownStringValue represents the string to be used to identify an unknown block
	BlockUnknownStringValue = "(UNKNOWN)"

	// BlockDefaultStringValue represents the message to identify a message that is undefined
	BlockDefaultStringValue = "Undefined message type"
)

func getStringValue(msgType consensus.MessageType) string {
	switch msgType {
	case MtBlockHeader:
		return BlockHeaderStringValue
	case MtSignature:
		return BlockSignatureStringValue
	case MtBlockHeaderFinalInfo:
		return BlockHeaderFinalInfoStringValue
	case MtUnknown:
		return BlockUnknownStringValue
	default:
		return BlockDefaultStringValue
	}
}

// getSubslotName returns the name of each Subslot from a given Subslot ID
func getSubslotName(subslotId int) string {
	switch subslotId {
	case SrStartSlot:
		return "(START_SLOT)"
	case SrBlock:
		return "(BLOCK)"
	case SrSignature:
		return "(SIGNATURE)"
	case SrEndSlot:
		return "(END_SLOT)"
	default:
		return "Undefined subslot"
	}
}
