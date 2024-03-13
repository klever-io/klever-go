package interceptedBlocks

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

func checkBlockHeaderArgument(arg *ArgInterceptedBlockHeader) error {
	if arg == nil {
		return process.ErrNilArgumentStruct
	}
	if len(arg.HdrBuff) == 0 {
		return process.ErrNilBuffer
	}
	if check.IfNil(arg.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arg.Hasher) {
		return common.ErrNilHasher
	}

	return nil
}

func checkBlockArgument(arg *ArgInterceptedBlock) error {
	if arg == nil {
		return process.ErrNilArgumentStruct
	}
	if len(arg.BlockBuff) == 0 {
		return process.ErrNilBuffer
	}
	if check.IfNil(arg.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(arg.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arg.KeyGen) {
		return common.ErrNilKeyGen
	}
	if check.IfNil(arg.HeaderSigVerifier) {
		return common.ErrNilHeaderSigVerifier
	}
	if check.IfNil(arg.HeaderIntegrityVerifier) {
		return common.ErrNilHeaderIntegrityVerifier
	}

	return nil
}

func checkHeaderHandler(hdr data.HeaderHandler) error {
	if len(hdr.GetPubKeysBitmap()) == 0 {
		return process.ErrNilPubKeysBitmap
	}
	if len(hdr.GetParentHash()) == 0 {
		return common.ErrNilPreviousBlockHash
	}
	if len(hdr.GetSignature()) == 0 {
		return common.ErrNilSignature
	}
	if len(hdr.GetRandSeed()) == 0 {
		return common.ErrNilRandSeed
	}
	if len(hdr.GetPrevRandSeed()) == 0 {
		return common.ErrNilPrevRandSeed
	}

	return nil
}
