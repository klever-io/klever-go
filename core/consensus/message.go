//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf  --go_out=. message.proto
package consensus

import "github.com/klever-io/klever-go/core"

// MessageType specifies what type of message was received
type MessageType int

// NewConsensusMessage creates a new Message object
func NewConsensusMessage(
	blHeaderHash []byte,
	signatureShare []byte,
	header []byte,
	pubKey []byte,
	sig []byte,
	msg int,
	slotIndex int64,
	nonce uint64,
	chainID []byte,
	pubKeysBitmap []byte,
	aggregateSignature []byte,
	leaderSignature []byte,
	currentPid core.PeerID,
) *Message {
	return &Message{
		BlockHeaderHash:    blHeaderHash,
		SignatureShare:     signatureShare,
		Header:             header,
		PubKey:             pubKey,
		Signature:          sig,
		MsgType:            int64(msg),
		SlotIndex:          slotIndex,
		ChainID:            chainID,
		PubKeysBitmap:      pubKeysBitmap,
		AggregateSignature: aggregateSignature,
		LeaderSignature:    leaderSignature,
		OriginatorPid:      currentPid.Bytes(),
		Nonce:              nonce,
	}
}
