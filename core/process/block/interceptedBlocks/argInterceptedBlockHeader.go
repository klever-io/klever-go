package interceptedBlocks

import (
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgInterceptedBlockHeader is the argument for the intercepted header
type ArgInterceptedBlockHeader struct {
	HdrBuff     []byte
	Marshalizer marshal.Marshalizer
	Hasher      hashing.Hasher
}
