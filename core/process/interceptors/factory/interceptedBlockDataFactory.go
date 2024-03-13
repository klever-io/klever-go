package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/interceptedBlocks"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.InterceptedDataFactory = (*interceptedBlockDataFactory)(nil)

type interceptedBlockDataFactory struct {
	marshalizer             marshal.Marshalizer
	hasher                  hashing.Hasher
	keyGen                  crypto.KeyGenerator
	headerSigVerifier       process.InterceptedHeaderSigVerifier
	headerIntegrityVerifier process.HeaderIntegrityVerifier
	epochStartTrigger       process.EpochStartTriggerHandler
}

// NewInterceptedBlockDataFactory creates an instance of interceptedBlockDataFactory
func NewInterceptedBlockDataFactory(argument *ArgInterceptedDataFactory) (*interceptedBlockDataFactory, error) {
	if argument == nil {
		return nil, process.ErrNilArgumentStruct
	}
	if check.IfNil(argument.ProtoMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(argument.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(argument.BlockKeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(argument.Signer) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(argument.EpochStartTrigger) {
		return nil, common.ErrNilEpochStartTrigger
	}
	return &interceptedBlockDataFactory{
		marshalizer:             argument.ProtoMarshalizer,
		hasher:                  argument.Hasher,
		keyGen:                  argument.BlockKeyGen,
		headerSigVerifier:       argument.HeaderSigVerifier,
		headerIntegrityVerifier: argument.HeaderIntegrityVerifier,
		epochStartTrigger:       argument.EpochStartTrigger,
	}, nil
}

// Create creates instances of InterceptedData by unmarshalling provided buffer
func (imfd *interceptedBlockDataFactory) Create(buff []byte) (process.InterceptedData, error) {
	arg := &interceptedBlocks.ArgInterceptedBlock{
		BlockBuff:               buff,
		Marshalizer:             imfd.marshalizer,
		Hasher:                  imfd.hasher,
		KeyGen:                  imfd.keyGen,
		HeaderSigVerifier:       imfd.headerSigVerifier,
		HeaderIntegrityVerifier: imfd.headerIntegrityVerifier,
		EpochStartTrigger:       imfd.epochStartTrigger,
	}

	return interceptedBlocks.NewInterceptedBlock(arg)
}

// IsInterfaceNil returns true if there is no value under the interface
func (imfd *interceptedBlockDataFactory) IsInterfaceNil() bool {
	return imfd == nil
}
