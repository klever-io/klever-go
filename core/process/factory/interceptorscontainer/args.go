package interceptorscontainer

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ShardInterceptorsContainerFactoryArgs holds the arguments needed for ShardInterceptorsContainerFactory
type ShardInterceptorsContainerFactoryArgs struct {
	Accounts                  state.AccountsAdapter
	NodesCoordinator          sharding.NodesCoordinator
	Messenger                 process.TopicHandler
	Store                     retriever.StorageService
	ProtoMarshalizer          marshal.Marshalizer
	TxSignMarshalizer         marshal.Marshalizer
	Hasher                    hashing.Hasher
	KeyGen                    crypto.KeyGenerator
	BlockSignKeyGen           crypto.KeyGenerator
	SingleSigner              crypto.SingleSigner
	BlockSingleSigner         crypto.SingleSigner
	MultiSigner               crypto.MultiSigner
	DataPool                  retriever.PoolsHolder
	AddressPubkeyConverter    core.PubkeyConverter
	MaxTxNonceDeltaAllowed    int
	BlockBlackList            process.TimeCacher
	WhiteListHandler          process.WhiteListHandler
	WhiteListerVerifiedTxs    process.WhiteListHandler
	AntifloodHandler          process.P2PAntifloodHandler
	ChainID                   []byte
	MinTransactionVersion     uint32
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	TxFeeHandler              process.EconomicsDataHandler
}

// MetaInterceptorsContainerFactoryArgs holds the arguments needed for MetaInterceptorsContainerFactory
type MetaInterceptorsContainerFactoryArgs struct {
	NodesCoordinator          sharding.NodesCoordinator
	Messenger                 process.TopicHandler
	Store                     retriever.StorageService
	ProtoMarshalizer          marshal.Marshalizer
	TxSignMarshalizer         marshal.Marshalizer
	Hasher                    hashing.Hasher
	MultiSigner               crypto.MultiSigner
	DataPool                  retriever.PoolsHolder
	Accounts                  state.AccountsAdapter
	MaxTxNonceDeltaAllowed    int
	AddressPubkeyConverter    core.PubkeyConverter
	SingleSigner              crypto.SingleSigner
	BlockSingleSigner         crypto.SingleSigner
	KeyGen                    crypto.KeyGenerator
	BlockKeyGen               crypto.KeyGenerator
	BlackList                 process.TimeCacher
	HeaderSigVerifier         process.InterceptedHeaderSigVerifier
	HeaderIntegrityVerifier   process.HeaderIntegrityVerifier
	WhiteListHandler          process.WhiteListHandler
	WhiteListerVerifiedTxs    process.WhiteListHandler
	AntifloodHandler          process.P2PAntifloodHandler
	EpochStartTrigger         process.EpochStartTriggerHandler
	ChainID                   []byte
	MinTransactionVersion     uint32
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
	TxFeeHandler              process.EconomicsDataHandler
	KAppController            kapp.KAppController
	ForkController            core.ForkController
}
