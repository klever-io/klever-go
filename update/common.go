package update

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("update")

// MbInfo defines the structure which hold the miniBlock info
type MbInfo struct {
	MbHash  []byte
	Type    block.Type
	TxsInfo []*TxInfo
}

// TxInfo defines the structure which hold the transaction info
type TxInfo struct {
	TxHash []byte
	Tx     data.TransactionHandler
}

// ArgsHardForkProcessor defines the arguments structure needed by hardfork processor methods
type ArgsHardForkProcessor struct {
	Hasher                 hashing.Hasher
	Marshalizer            marshal.Marshalizer
	Body                   *block.Block
	HardForkBlockProcessor HardForkBlockProcessor
}
