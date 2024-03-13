package factory

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/network/p2p"
)

// P2PAntifloodHandler defines the behavior of a component able to signal that the system is too busy (or flooded) processing
// p2p messages
type P2PAntifloodHandler interface {
	CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopic(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error
	ResetForTopic(topic string)
	SetMaxMessagesForTopic(topic string, maxNum uint32)
	SetDebugger(debugger process.AntifloodDebugger) error
	SetPeerValidatorMapper(validatorMapper process.PeerValidatorMapper) error
	SetTopicsForAll(topics ...string)
	ApplyConsensusSize(size int)
	BlacklistPeer(peer core.PeerID, reason string, duration time.Duration)
	IsOriginatorElectedForTopic(pid core.PeerID, topic string) error
	IsInterfaceNil() bool
}

// CryptoComponents struct holds the crypto components
type CryptoComponents struct {
	TxSingleSigner       crypto.SingleSigner
	SingleSigner         crypto.SingleSigner
	MultiSigner          crypto.MultiSigner
	BlockSignKeyGen      crypto.KeyGenerator
	TxSignKeyGen         crypto.KeyGenerator
	InitialPubKeys       []string
	PeerSignatureHandler crypto.PeerSignatureHandler
}

// NodesSetupHandler defines which actions should be done for handling initial nodes setup
type NodesSetupHandler interface {
	InitialNodesPubKeys() []string
	InitialElectedNodesPubKeys() ([]string, error)
	IsInterfaceNil() bool
}

// HeaderSigVerifierHandler is the interface needed to check that a header's signature is correct
type HeaderSigVerifierHandler interface {
	VerifyRandSeed(header data.HeaderHandler) error
	VerifyLeaderSignature(header data.HeaderHandler) error
	VerifyRandSeedAndLeaderSignature(header data.HeaderHandler) error
	VerifySignature(header data.HeaderHandler) error
	IsInterfaceNil() bool
}

// HeaderIntegrityVerifierHandler is the interface needed to check that a header's integrity is correct
type HeaderIntegrityVerifierHandler interface {
	Verify(header data.HeaderHandler) error
	GetVersion(epoch uint32) string
	IsInterfaceNil() bool
}
