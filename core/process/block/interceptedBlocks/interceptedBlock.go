package interceptedBlocks

import (
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.HdrValidatorHandler = (*InterceptedBlock)(nil)
var _ process.InterceptedData = (*InterceptedBlock)(nil)

// InterceptedBlock is a wrapper over a block
type InterceptedBlock struct {
	block             *block.Block
	marshalizer       marshal.Marshalizer
	hasher            hashing.Hasher
	hash              []byte
	keyGen            crypto.KeyGenerator
	sigVerifier       process.InterceptedHeaderSigVerifier
	integrityVerifier process.HeaderIntegrityVerifier
	epochStartTrigger process.EpochStartTriggerHandler
	forkController    core.ForkController
}

// NewInterceptedBlock creates a new instance of InterceptedBlock struct
func NewInterceptedBlock(arg *ArgInterceptedBlock) (*InterceptedBlock, error) {
	err := checkBlockArgument(arg)
	if err != nil {
		return nil, err
	}

	blk, err := createBlock(arg.Marshalizer, arg.BlockBuff)
	if err != nil {
		return nil, err
	}

	headerBytes, err := arg.Marshalizer.Marshal(blk.Header)
	if err != nil {
		return nil, err
	}

	inBlock := &InterceptedBlock{
		block:             blk,
		marshalizer:       arg.Marshalizer,
		hasher:            arg.Hasher,
		keyGen:            arg.KeyGen,
		sigVerifier:       arg.HeaderSigVerifier,
		integrityVerifier: arg.HeaderIntegrityVerifier,
		epochStartTrigger: arg.EpochStartTrigger,
		forkController:    arg.ForkController,
	}

	inBlock.processFields(arg.BlockBuff, headerBytes)

	return inBlock, nil
}

func createBlock(marshalizer marshal.Marshalizer, blockBuff []byte) (*block.Block, error) {
	blk := &block.Block{}
	err := marshalizer.Unmarshal(blk, blockBuff)
	if err != nil {
		return nil, err
	}

	return blk, nil
}

func (inMb *InterceptedBlock) processFields(mbBuff []byte, headerBytes []byte) {
	inMb.hash = inMb.hasher.Compute(string(headerBytes))
}

// Hash gets the hash of this transaction block header
func (inMb *InterceptedBlock) Hash() []byte {
	return inMb.hash
}

// Block returns the block held by this wrapper
func (inMb *InterceptedBlock) Block() *block.Block {
	return inMb.block
}

// HeaderHandler returns the block held by this wrapper
func (inMb *InterceptedBlock) HeaderHandler() data.HeaderHandler {
	return inMb.block
}

func (inMb *InterceptedBlock) isEpochCorrect() bool {
	if inMb.epochStartTrigger.EpochStartSlot() >= inMb.epochStartTrigger.EpochFinalityAttestingSlot() {
		return true
	}
	if inMb.block.GetEpoch() >= inMb.epochStartTrigger.Epoch() {
		return true
	}
	if inMb.block.GetSlot() <= inMb.epochStartTrigger.EpochStartSlot() {
		return true
	}
	if inMb.block.GetSlot() <= inMb.epochStartTrigger.EpochFinalityAttestingSlot()+process.EpochChangeGracePeriod {
		return true
	}

	return false
}

func (inMb *InterceptedBlock) verifyEpochStartSlot(epoch uint32, slot uint64) bool {
	if epoch == inMb.epochStartTrigger.Epoch() {
		return slot == inMb.epochStartTrigger.EpochStartSlot()
	}

	return slot > inMb.epochStartTrigger.EpochStartSlot()
}

func (inMb *InterceptedBlock) verifyPrevEpochStartSlot(epoch uint32, prevStartSlot uint64) bool {
	if epoch == inMb.epochStartTrigger.Epoch() {
		return prevStartSlot == inMb.epochStartTrigger.PrevEpochStartSlot()
	}

	return prevStartSlot == inMb.epochStartTrigger.EpochStartSlot()
}

func (inMb *InterceptedBlock) isEpochStartCorrect() bool {
	isEpochStarted := inMb.block.GetIsEpochStart()
	epochStartedSlot := inMb.epochStartTrigger.EpochStartSlot()
	epoch := inMb.block.GetEpoch()
	slot := inMb.block.GetSlot()
	prevStartSlot := inMb.block.Header.PrevEpochStartSlot

	if isEpochStarted && (inMb.verifyEpochStartSlot(epoch, slot) && inMb.verifyPrevEpochStartSlot(epoch, prevStartSlot)) {
		return true
	}

	if !isEpochStarted && (epochStartedSlot < slot && prevStartSlot == uint64(0)) {
		return true
	}

	return false
}

// Type returns the type of this intercepted data
func (inMb *InterceptedBlock) Type() string {
	return "intercepted block"
}

// String returns the transactions body's most important fields as string
func (inMb *InterceptedBlock) String() string {
	return fmt.Sprintf("epoch=%d, slot=%d, nonce=%d, block numTxs=%d",
		inMb.block.Header.Epoch,
		inMb.block.Header.Slot,
		inMb.block.Header.Nonce,
		len(inMb.block.TxHashes),
	)
}

// Identifiers returns the identifiers used in requests
func (inMb *InterceptedBlock) Identifiers() [][]byte {
	keyNonce := []byte(fmt.Sprintf("nonce-%d", inMb.block.Header.Nonce))
	keyEpoch := []byte(core.EpochStartIdentifier(inMb.block.Header.Epoch))

	return [][]byte{inMb.hash, keyNonce, keyEpoch}
}

// IsInterfaceNil returns true if there is no value under the interface
func (inMb *InterceptedBlock) IsInterfaceNil() bool {
	return inMb == nil
}

// CheckValidity checks if the received tx block header is valid (not nil fields)
func (inMb *InterceptedBlock) CheckValidity() error {
	if err := inMb.integrity(); err != nil {
		return err
	}

	hdr := inMb.HeaderHandler()

	// verify basic fields such as ChanID and software version to reduce load
	// on CheckValidity
	if err := inMb.integrityVerifier.Verify(hdr); err != nil {
		return err
	}

	if err := inMb.sigVerifier.VerifyRandSeedAndLeaderSignature(hdr); err != nil {
		return fmt.Errorf("%w : verify rand seed and leader signature for intercepted block failed",
			err)
	}

	return inMb.sigVerifier.VerifySignature(hdr)
}

// integrity checks the integrity of the tx block header
func (inMb *InterceptedBlock) integrity() error {
	err := checkHeaderHandler(inMb.HeaderHandler())
	if err != nil {
		return err
	}

	if !inMb.isEpochCorrect() {
		return fmt.Errorf("header with old epoch and bad slot: %w"+
			"headerHash=%s, "+
			"epoch=%v, "+
			"slot=%v ",
			process.ErrEpochDoesNotMatch,
			logger.DisplayByteSlice(inMb.hash),
			inMb.block.Header.Epoch,
			inMb.block.Header.Slot,
		)
	}

	if inMb.forkController.EnableSmartContracts() && !inMb.isEpochStartCorrect() {
		return fmt.Errorf("header with incorrect epochStart: %w"+
			"headerHash=%s, "+
			"epoch=%v, "+
			"slot=%v "+
			"epochStart=%v "+
			"prevEpochStartSlot=%v ",
			process.ErrEpochDoesNotMatch,
			logger.DisplayByteSlice(inMb.hash),
			inMb.block.Header.Epoch,
			inMb.block.Header.Slot,
			inMb.block.Header.GetIsEpochStart(),
			inMb.block.Header.GetPrevEpochStartSlot(),
		)
	}

	blk := inMb.block

	for _, txHash := range blk.TxHashes {
		if txHash == nil {
			return process.ErrNilTxHash
		}
	}

	return nil
}
