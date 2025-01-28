package interceptedBlocks

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgInterceptedBlock is the argument for the intercepted miniblock
type ArgInterceptedBlock struct {
	BlockBuff               []byte
	Marshalizer             marshal.Marshalizer
	Hasher                  hashing.Hasher
	KeyGen                  crypto.KeyGenerator
	HeaderSigVerifier       process.InterceptedHeaderSigVerifier
	HeaderIntegrityVerifier process.HeaderIntegrityVerifier
	EpochStartTrigger       process.EpochStartTriggerHandler
	ForkController          core.ForkController
}
