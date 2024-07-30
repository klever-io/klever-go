package factory

import (
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/factory/interceptorscontainer"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage/timecache"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

const timeSpanForBadHeaders = time.Minute

// ArgsEpochStartInterceptorContainer holds the arguments needed for creating a new epoch start interceptors
// container factory
type ArgsEpochStartInterceptorContainer struct {
	Config                    config.Config
	TxSignMarshalizer         marshal.Marshalizer
	ProtoMarshalizer          marshal.Marshalizer
	Hasher                    hashing.Hasher
	Messenger                 process.TopicHandler
	DataPool                  retriever.PoolsHolder
	SingleSigner              crypto.SingleSigner
	BlockSingleSigner         crypto.SingleSigner
	KeyGen                    crypto.KeyGenerator
	BlockKeyGen               crypto.KeyGenerator
	WhiteListHandler          process.WhiteListHandler
	WhiteListerVerifiedTxs    process.WhiteListHandler
	AddressPubkeyConv         core.PubkeyConverter
	NonceConverter            typeConverters.Uint64ByteSliceConverter
	ChainID                   []byte
	MinTransactionVersion     uint32
	HeaderIntegrityVerifier   process.HeaderIntegrityVerifier
	EnableSignTxWithHashEpoch uint32
	TxSignHasher              hashing.Hasher
	EpochNotifier             process.EpochNotifier
}

// NewEpochStartInterceptorsContainer will return a real interceptors container factory, but with many disabled components
func NewEpochStartInterceptorsContainer(args ArgsEpochStartInterceptorContainer) (process.InterceptorsContainer, error) {
	nodesCoordinator := disabled.NewNodesCoordinator()
	storer := disabled.NewChainStorer()
	antiFloodHandler := disabled.NewAntiFloodHandler()
	multiSigner := disabled.NewMultiSigner()
	accountsAdapter := disabled.NewAccountsAdapter()
	if check.IfNil(args.AddressPubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	blackListHandler := timecache.NewTimeCache(timeSpanForBadHeaders)
	feeHandler := &disabled.FeeHandler{}
	headerSigVerifier := disabled.NewHeaderSigVerifier()
	epochStartTrigger := disabled.NewEpochStartTrigger()
	//validityAttester := disabled.NewValidityAttester()

	kAppController := disabled.NewKAppsController()

	containerFactoryArgs := interceptorscontainer.MetaInterceptorsContainerFactoryArgs{
		NodesCoordinator:          nodesCoordinator,
		Messenger:                 args.Messenger,
		Store:                     storer,
		ProtoMarshalizer:          args.ProtoMarshalizer,
		TxSignMarshalizer:         args.TxSignMarshalizer,
		Hasher:                    args.Hasher,
		MultiSigner:               multiSigner,
		DataPool:                  args.DataPool,
		Accounts:                  accountsAdapter,
		AddressPubkeyConverter:    args.AddressPubkeyConv,
		SingleSigner:              args.SingleSigner,
		BlockSingleSigner:         args.BlockSingleSigner,
		KeyGen:                    args.KeyGen,
		BlockKeyGen:               args.BlockKeyGen,
		TxFeeHandler:              feeHandler,
		BlackList:                 blackListHandler,
		HeaderSigVerifier:         headerSigVerifier,
		HeaderIntegrityVerifier:   args.HeaderIntegrityVerifier,
		EpochStartTrigger:         epochStartTrigger,
		WhiteListHandler:          args.WhiteListHandler,
		WhiteListerVerifiedTxs:    args.WhiteListerVerifiedTxs,
		AntifloodHandler:          antiFloodHandler,
		ChainID:                   args.ChainID,
		MinTransactionVersion:     args.MinTransactionVersion,
		EnableSignTxWithHashEpoch: args.EnableSignTxWithHashEpoch,
		TxSignHasher:              args.TxSignHasher,
		EpochNotifier:             args.EpochNotifier,
		KAppController:            kAppController,
	}

	interceptorsContainerFactory, err := interceptorscontainer.NewMetaInterceptorsContainerFactory(containerFactoryArgs)
	if err != nil {
		return nil, err
	}

	container, err := interceptorsContainerFactory.Create()
	if err != nil {
		return nil, err
	}

	return container, nil
}
