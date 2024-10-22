package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgInterceptedDataFactory holds all dependencies required by the shard and meta intercepted data factory in order to create
// new instances
type ArgInterceptedDataFactory struct {
	ProtoMarshalizer          marshal.Marshalizer
	TxSignMarshalizer         marshal.Marshalizer
	Hasher                    hashing.Hasher
	MultiSigVerifier          crypto.MultiSigVerifier
	NodesCoordinator          sharding.NodesCoordinator
	AccountKeyGen             crypto.KeyGenerator
	BlockKeyGen               crypto.KeyGenerator
	Signer                    crypto.SingleSigner
	BlockSigner               crypto.SingleSigner
	HeaderSigVerifier         process.InterceptedHeaderSigVerifier
	HeaderIntegrityVerifier   process.HeaderIntegrityVerifier
	AddressPubkeyConv         core.PubkeyConverter
	WhiteListerVerifiedTxs    process.WhiteListHandler
	EpochStartTrigger         process.EpochStartTriggerHandler
	ChainID                   []byte
	MinTransactionVersion     uint32
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	FeeHandler                process.EconomicsDataHandler
	ForkController            core.ForkController
}
