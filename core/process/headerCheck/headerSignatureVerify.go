package headerCheck

import (
	"math/bits"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.InterceptedHeaderSigVerifier = (*HeaderSigVerifier)(nil)

var log = logger.GetOrCreate("process/headerCheck")

// ArgsHeaderSigVerifier is used to store all components that are needed to create a new HeaderSigVerifier
type ArgsHeaderSigVerifier struct {
	Marshalizer             marshal.Marshalizer
	Hasher                  hashing.Hasher
	NodesCoordinator        sharding.NodesCoordinator
	MultiSigVerifier        crypto.MultiSigVerifier
	SingleSigVerifier       crypto.SingleSigner
	KeyGen                  crypto.KeyGenerator
	FallbackHeaderValidator process.FallbackHeaderValidator
}

// HeaderSigVerifier is component used to check if a header is valid
type HeaderSigVerifier struct {
	marshalizer             marshal.Marshalizer
	hasher                  hashing.Hasher
	nodesCoordinator        sharding.NodesCoordinator
	multiSigVerifier        crypto.MultiSigVerifier
	singleSigVerifier       crypto.SingleSigner
	keyGen                  crypto.KeyGenerator
	fallbackHeaderValidator process.FallbackHeaderValidator
}

// NewHeaderSigVerifier will create a new instance of HeaderSigVerifier
func NewHeaderSigVerifier(arguments *ArgsHeaderSigVerifier) (*HeaderSigVerifier, error) {
	err := checkArgsHeaderSigVerifier(arguments)
	if err != nil {
		return nil, err
	}

	return &HeaderSigVerifier{
		marshalizer:             arguments.Marshalizer,
		hasher:                  arguments.Hasher,
		nodesCoordinator:        arguments.NodesCoordinator,
		multiSigVerifier:        arguments.MultiSigVerifier,
		singleSigVerifier:       arguments.SingleSigVerifier,
		keyGen:                  arguments.KeyGen,
		fallbackHeaderValidator: arguments.FallbackHeaderValidator,
	}, nil
}

func checkArgsHeaderSigVerifier(arguments *ArgsHeaderSigVerifier) error {
	if arguments == nil {
		return process.ErrNilArgumentStruct
	}
	if check.IfNil(arguments.Hasher) {
		return process.ErrNilHasher
	}
	if check.IfNil(arguments.KeyGen) {
		return common.ErrNilKeyGen
	}
	if check.IfNil(arguments.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arguments.MultiSigVerifier) {
		return common.ErrNilMultiSigVerifier
	}
	if check.IfNil(arguments.NodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(arguments.SingleSigVerifier) {
		return common.ErrNilSingleSigner
	}
	if check.IfNil(arguments.FallbackHeaderValidator) {
		return process.ErrNilFallbackHeaderValidator
	}

	return nil
}

// VerifySignature will check if signature is correct
func (hsv *HeaderSigVerifier) VerifySignature(header data.HeaderHandler) error {
	randSeed := header.GetPrevRandSeed()
	bitmap := header.GetPubKeysBitmap()
	if len(bitmap) == 0 {
		return process.ErrNilPubKeysBitmap
	}
	if bitmap[0]&1 == 0 {
		return process.ErrBlockProposerSignatureMissing
	}

	// TODO: remove if start of epoch block needs to be validated by the new epoch nodes
	epoch := header.GetEpoch()
	if header.GetIsEpochStart() && epoch > 0 {
		epoch = epoch - 1
	}

	consensusPubKeys, err := hsv.nodesCoordinator.GetConsensusValidatorsPublicKeys(
		randSeed,
		header.GetSlot(),
		epoch,
	)
	if err != nil {
		return err
	}

	err = hsv.verifyConsensusSize(consensusPubKeys, header)
	if err != nil {
		return err
	}

	verifier, err := hsv.multiSigVerifier.Create(consensusPubKeys, 0)
	if err != nil {
		return err
	}

	err = verifier.SetAggregatedSig(header.GetSignature())
	if err != nil {
		return err
	}

	// get marshalled block header without signature and bitmap
	// as this is the message that was signed
	headerCopy := header.GetBlockHeader()

	hash, err := tools.CalculateHash(hsv.marshalizer, hsv.hasher, headerCopy)
	if err != nil {
		return err
	}

	return verifier.Verify(hash, bitmap)
}

func (hsv *HeaderSigVerifier) verifyConsensusSize(consensusPubKeys []string, header data.HeaderHandler) error {
	consensusSize := len(consensusPubKeys)
	bitmap := header.GetPubKeysBitmap()

	expectedBitmapSize := consensusSize / 8
	if consensusSize%8 != 0 {
		expectedBitmapSize++
	}
	if len(bitmap) != expectedBitmapSize {
		log.Debug("wrong size bitmap",
			"expected number of bytes", expectedBitmapSize,
			"actual", len(bitmap))
		return ErrWrongSizeBitmap
	}

	numOfOnesInBitmap := 0
	for index := range bitmap {
		numOfOnesInBitmap += bits.OnesCount8(bitmap[index])
	}

	minNumRequiredSignatures := consensus.GetPBFTThreshold(consensusSize)
	if hsv.fallbackHeaderValidator.ShouldApplyFallbackValidation(header) {
		minNumRequiredSignatures = consensus.GetPBFTFallbackThreshold(consensusSize)
		log.Warn("HeaderSigVerifier.verifyConsensusSize: fallback validation has been applied",
			"minimum number of signatures required", minNumRequiredSignatures,
			"actual number of signatures in bitmap", numOfOnesInBitmap,
		)
	}

	if numOfOnesInBitmap >= minNumRequiredSignatures {
		return nil
	}

	log.Debug("not enough signatures",
		"minimum expected", minNumRequiredSignatures,
		"actual", numOfOnesInBitmap)

	return ErrNotEnoughSignatures
}

// VerifyRandSeed will check if rand seed is correct
func (hsv *HeaderSigVerifier) VerifyRandSeed(header data.HeaderHandler) error {
	leaderPubKey, err := hsv.getLeader(header)
	if err != nil {
		return err
	}

	err = hsv.verifyRandSeed(leaderPubKey, header)
	if err != nil {
		leaderPubKeyBytes, _ := leaderPubKey.ToByteArray()
		log.Trace("block rand seed",
			"slot", header.GetSlot(),
			"nonce", header.GetNonce(),
			"prevRandSeed", header.GetPrevRandSeed(),
			"leader", leaderPubKeyBytes,
			"error", err.Error())
		return err
	}

	return nil
}

// VerifyLeaderSignature will check if leader signature is correct
func (hsv *HeaderSigVerifier) VerifyLeaderSignature(header data.HeaderHandler) error {
	leaderPubKey, err := hsv.getLeader(header)
	if err != nil {
		return err
	}

	err = hsv.verifyLeaderSignature(leaderPubKey, header)
	if err != nil {
		log.Trace("block leader's signature",
			"error", err.Error())
		return err
	}

	return nil
}

// VerifyRandSeedAndLeaderSignature will check if rand seed and leader signature is correct
func (hsv *HeaderSigVerifier) VerifyRandSeedAndLeaderSignature(header data.HeaderHandler) error {
	leaderPubKey, err := hsv.getLeader(header)
	if err != nil {
		return err
	}

	err = hsv.verifyRandSeed(leaderPubKey, header)
	if err != nil {
		leaderPubKeyBytes, _ := leaderPubKey.ToByteArray()
		log.Trace("block rand seed",
			"slot", header.GetSlot(),
			"nonce", header.GetNonce(),
			"prevRandSeed", header.GetPrevRandSeed(),
			"leader", leaderPubKeyBytes,
			"error", err.Error())
		return err
	}

	err = hsv.verifyLeaderSignature(leaderPubKey, header)
	if err != nil {
		log.Trace("block leader's signature",
			"error", err.Error())
		return err
	}

	return nil
}

// IsInterfaceNil will check if interface is nil
func (hsv *HeaderSigVerifier) IsInterfaceNil() bool {
	return hsv == nil
}

func (hsv *HeaderSigVerifier) verifyRandSeed(leaderPubKey crypto.PublicKey, header data.HeaderHandler) error {
	prevRandSeed := header.GetPrevRandSeed()
	randSeed := header.GetRandSeed()

	return hsv.singleSigVerifier.Verify(leaderPubKey, prevRandSeed, randSeed)
}

func (hsv *HeaderSigVerifier) verifyLeaderSignature(leaderPubKey crypto.PublicKey, header data.HeaderHandler) error {
	headerCopy := header.GetBlockHeader()
	headerBytes, err := hsv.marshalizer.Marshal(headerCopy)
	if err != nil {
		return err
	}

	return hsv.singleSigVerifier.Verify(leaderPubKey, headerBytes, header.GetProducerSignature())
}

func (hsv *HeaderSigVerifier) getLeader(header data.HeaderHandler) (crypto.PublicKey, error) {
	prevRandSeed := header.GetPrevRandSeed()

	// TODO: remove if start of epoch block needs to be validated by the new epoch nodes
	epoch := header.GetEpoch()
	if header.GetIsEpochStart() && epoch > 0 {
		epoch = epoch - 1
	}

	headerConsensusGroup, err := hsv.nodesCoordinator.ComputeConsensusGroup(prevRandSeed, header.GetSlot(), epoch)
	if err != nil {
		return nil, err
	}

	leaderPubKeyValidator := headerConsensusGroup[0]
	return hsv.keyGen.PublicKeyFromByteArray(leaderPubKeyValidator.PubKey())
}
