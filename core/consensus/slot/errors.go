package slot

import "errors"

// ErrNilConsensusGroup is raised when an operation is attempted with a nil consensus group
var ErrNilConsensusGroup = errors.New("consensusGroup is null")

// ErrEmptyConsensusGroup is raised when an operation is attempted with an empty consensus group
var ErrEmptyConsensusGroup = errors.New("consensusGroup is empty")

// ErrNotFoundInConsensus is raised when self expected in consensus group but not found
var ErrNotFoundInConsensus = errors.New("self not found in consensus group")

// ErrInvalidKey is raised when an invalid key is used with a map
var ErrInvalidKey = errors.New("map key is invalid")

// ErrNilSlotState is raised when a valid slot state is expected but nil used
var ErrNilSlotState = errors.New("slot state is nil")

// ErrNilChannel is raised when a valid channel is expected but nil used
var ErrNilChannel = errors.New("channel is nil")

// ErrNilConsensusState is raised when a valid consensus is expected but nil used
var ErrNilConsensusState = errors.New("consensus state is nil")

// ErrNilExecuteStoredMessages is raised when a valid executeStoredMessages function is expected but nil used
var ErrNilExecuteStoredMessages = errors.New("executeStoredMessages is nil")

// ErrInvalidChainID signals that an invalid chain ID has been provided
var ErrInvalidChainID = errors.New("invalid chain ID in consensus")

// ErrNilConsensusCore is raised when a valid ConsensusCore is expected but nil used
var ErrNilConsensusCore = errors.New("consensus core is nil")

// ErrNilBlockChain is raised when a valid blockchain is expected but nil used
var ErrNilBlockChain = errors.New("blockchain is nil")

// ErrNilHasher is raised when a valid hasher is expected but nil used
var ErrNilHasher = errors.New("hasher is nil")

// ErrNilBlockProcessor is raised when a valid block processor is expected but nil used
var ErrNilBlockProcessor = errors.New("block processor is nil")

// ErrNilBootstrapper is raised when a valid block processor is expected but nil used
var ErrNilBootstrapper = errors.New("bootstrapper is nil")

// ErrNilBroadcastMessenger is raised when a valid broadcast messenger is expected but nil used
var ErrNilBroadcastMessenger = errors.New("broadcast messenger is nil")

// ErrNilChronologyHandler is raised when a valid chronology handler is expected but nil used
var ErrNilChronologyHandler = errors.New("chronology handler is nil")

// ErrNilSlotManager is raised when a valid SlotManager is expected but nil used
var ErrNilSlotManager = errors.New("slotManager is nil")

// ErrNilMarshalizer is raised when a valid marshalizer is expected but nil used
var ErrNilMarshalizer = errors.New("marshalizer is nil")

// ErrNilMultiSigner is raised when a valid multiSigner is expected but nil used
var ErrNilMultiSigner = errors.New("multiSigner is nil")

// ErrNilSyncTimer is raised when a valid sync timer is expected but nil used
var ErrNilSyncTimer = errors.New("sync timer is nil")

// ErrNilNodesCoordinator is raised when a valid validator group selector is expected but nil used
var ErrNilNodesCoordinator = errors.New("validator group selector is nil")

// ErrNilBlsPrivateKey is raised when the bls private key is nil
var ErrNilBlsPrivateKey = errors.New("BLS private key should not be nil")

// ErrNilBlsSingleSigner is raised when a message from itself is received
var ErrNilBlsSingleSigner = errors.New("BLS single signer should not be nil")

// ErrNilAntifloodHandler signals that a nil antiflood handler has been provided
var ErrNilAntifloodHandler = errors.New("nil antiflood handler")

// ErrNilPeerHonestyHandler signals that a nil peer honesty handler has been provided
var ErrNilPeerHonestyHandler = errors.New("nil peer honesty handler")

// ErrNilHeaderSigVerifier signals that a nil header sig verifier has been provided
var ErrNilHeaderSigVerifier = errors.New("nil header sig verifier")

// ErrNilFallbackHeaderValidator signals that a nil fallback header validator has been provided
var ErrNilFallbackHeaderValidator = errors.New("nil fallback header validator")

// ErrNilNodeRedundancyHandler signals that provided node redundancy handler is nil
var ErrNilNodeRedundancyHandler = errors.New("nil node redundancy handler")

// ErrInvalidPublicKeySize signals that an invalid public key size has been received from consensus topic
var ErrInvalidPublicKeySize = errors.New("invalid public key size")

// ErrInvalidSignatureSize signals that an invalid signature size has been received from consensus topic
var ErrInvalidSignatureSize = errors.New("invalid signature size")

// ErrInvalidPublicKeyBitmapSize signals that an invalid public key bitmap size has been received from consensus topic
var ErrInvalidPublicKeyBitmapSize = errors.New("invalid public key bitmap size")

// ErrInvalidMessage signals that an invalid message has been received from consensus topic
var ErrInvalidMessage = errors.New("invalid message")

// ErrInvalidHeaderSize signals that an invalid header size has been received from consensus topic
var ErrInvalidHeaderSize = errors.New("invalid header size")

// ErrInvalidBodySize signals that an invalid body size has been received from consensus topic
var ErrInvalidBodySize = errors.New("invalid body size")

// ErrInvalidMessageType signals that an invalid message type has been received from consensus topic
var ErrInvalidMessageType = errors.New("invalid message type")

// ErrInvalidHeaderHashSize signals that an invalid header hash size has been received from consensus topic
var ErrInvalidHeaderHashSize = errors.New("invalid header hash size")

// ErrHeaderkHashNotMatch signals block header hash sent does not match header hash
var ErrHeaderkHashNotMatch = errors.New("invalid header hash")

// ErrNodeIsNotInEligibleList is raised when a node is not in eligible list
var ErrNodeIsNotInEligibleList = errors.New("node is not in eligible list")

// ErrNodeIsNotInElectedList is raised when a node is not in eligible list
var ErrNodeIsNotInElectedList = errors.New("node is not in elected list")

// ErrMessageForFutureSlot is raised when message is for future slot
var ErrMessageForFutureSlot = errors.New("message is for future slot")

// ErrNilAppStatusHandler defines the error for setting a nil AppStatusHandler
var ErrNilAppStatusHandler = errors.New("nil AppStatusHandler")

// ErrNilWorker is raised when a valid Worker is expected but nil used
var ErrNilWorker = errors.New("worker is nil")

// ErrNilWorkerArgs signals that nil a workerArgs has been provided
var ErrNilWorkerArgs = errors.New("worker args is nil")

// ErrNilForkDetector is raised when a valid fork detector is expected but nil used
var ErrNilForkDetector = errors.New("fork detector is nil")

// ErrNilConsensusService is raised when a valid ConsensusService is expected but nil used
var ErrNilConsensusService = errors.New("consensus service is nil")

// ErrNilPeerSignatureHandler signals that a nil peerSignatureHandler object has been provided
var ErrNilPeerSignatureHandler = errors.New("trying to set nil peerSignatureHandler")

// ErrNilHeaderIntegrityVerifier signals that a nil header integrity verifier has been provided
var ErrNilHeaderIntegrityVerifier = errors.New("nil header integrity verifier")

// ErrNilNetworkShardingCollector defines the error for setting a nil network sharding collector
var ErrNilNetworkShardingCollector = errors.New("nil network sharding collector")

// ErrMessageForPastSlot is raised when message is for past slot
var ErrMessageForPastSlot = errors.New("message is for past slot")

// ErrMessageTypeLimitReached signals that a consensus message type limit has been reached for a public key
var ErrMessageTypeLimitReached = errors.New("consensus message type limit has been reached")

// ErrInvalidHeader is raised when header is invalid
var ErrInvalidHeader = errors.New("header is invalid")

// ErrNilMessage signals that a nil message has been received
var ErrNilMessage = errors.New("nil message")

// ErrNilDataToProcess signals that nil data was provided
var ErrNilDataToProcess = errors.New("nil data to process")

// ErrMessageFromItself is raised when a message from itself is received
var ErrMessageFromItself = errors.New("message is from itself")

// ErrSlotCanceled is raised when slot is canceled
var ErrSlotCanceled = errors.New("slot is canceled")

// ErrOriginatorMismatch signals that an original consensus message has been re-broadcast manually by another peer
var ErrOriginatorMismatch = errors.New("consensus message originator mismatch")

// ErrNilSubslot is raised when a valid subslot is expected but nil used
var ErrNilSubslot = errors.New("subslot is nil")

// ErrInvalidConsensusNonce signals an invalid consensus nonce received in consensus message
var ErrInvalidConsensusNonce = errors.New("invald consensus nonce received")
