package bootstrap

import (
	"context"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors"
	interceptorsFactory "github.com/klever-io/klever-go/core/process/interceptors/factory"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

type epochStartMetaSyncer struct {
	requestHandler        RequestHandler
	messenger             Messenger
	marshalizer           marshal.Marshalizer
	hasher                hashing.Hasher
	singleDataInterceptor process.Interceptor
	blockProcessor        EpochStartBlockInterceptorProcessor
}

// ArgsNewEpochStartMetaSyncer -
type ArgsNewEpochStartMetaSyncer struct {
	RequestHandler          RequestHandler
	Messenger               Messenger
	Marshalizer             marshal.Marshalizer
	TxSignMarshalizer       marshal.Marshalizer
	KeyGen                  crypto.KeyGenerator
	BlockKeyGen             crypto.KeyGenerator
	Hasher                  hashing.Hasher
	Signer                  crypto.SingleSigner
	BlockSigner             crypto.SingleSigner
	ChainID                 []byte
	EconomicsData           process.EconomicsDataHandler
	WhitelistHandler        process.WhiteListHandler
	AddressPubkeyConv       core.PubkeyConverter
	NonceConverter          typeConverters.Uint64ByteSliceConverter
	HeaderIntegrityVerifier process.HeaderIntegrityVerifier
	BlockProcessor          EpochStartBlockInterceptorProcessor
	ForkController          core.ForkController
}

// thresholdForConsideringMetaBlockCorrect represents the percentage (between 0 and 100) of connected peers to send
// the same meta block in order to consider it correct
const thresholdForConsideringMetaBlockCorrect = 67

// NewEpochStartMetaSyncer will return a new instance of epochStartMetaSyncer
func NewEpochStartMetaSyncer(args ArgsNewEpochStartMetaSyncer) (*epochStartMetaSyncer, error) {
	if check.IfNil(args.AddressPubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return nil, common.ErrNilHeaderIntegrityVerifier
	}
	if check.IfNil(args.BlockProcessor) {
		return nil, common.ErrNilBlockProcessor
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	e := &epochStartMetaSyncer{
		requestHandler: args.RequestHandler,
		messenger:      args.Messenger,
		marshalizer:    args.Marshalizer,
		hasher:         args.Hasher,
		blockProcessor: args.BlockProcessor,
	}

	argsInterceptedDataFactory := interceptorsFactory.ArgInterceptedDataFactory{
		ProtoMarshalizer:        args.Marshalizer,
		TxSignMarshalizer:       args.TxSignMarshalizer,
		Hasher:                  args.Hasher,
		MultiSigVerifier:        disabled.NewMultiSigVerifier(),
		NodesCoordinator:        disabled.NewNodesCoordinator(),
		AccountKeyGen:           args.KeyGen,
		BlockKeyGen:             args.BlockKeyGen,
		Signer:                  args.Signer,
		BlockSigner:             args.BlockSigner,
		AddressPubkeyConv:       args.AddressPubkeyConv,
		FeeHandler:              args.EconomicsData,
		HeaderSigVerifier:       disabled.NewHeaderSigVerifier(),
		HeaderIntegrityVerifier: args.HeaderIntegrityVerifier,
		EpochStartTrigger:       disabled.NewEpochStartTrigger(),
		ForkController:          args.ForkController,
		//ValidityAttester:  disabled.NewValidityAttester(),
	}

	interceptedMetaHdrDataFactory, err := interceptorsFactory.NewInterceptedBlockDataFactory(&argsInterceptedDataFactory)
	if err != nil {
		return nil, err
	}

	e.singleDataInterceptor, err = interceptors.NewSingleDataInterceptor(
		interceptors.ArgSingleDataInterceptor{
			Topic:            common.BlocksTopic,
			DataFactory:      interceptedMetaHdrDataFactory,
			Processor:        args.BlockProcessor,
			Throttler:        disabled.NewThrottler(),
			AntifloodHandler: disabled.NewAntiFloodHandler(),
			WhiteListRequest: args.WhitelistHandler,
			CurrentPeerID:    args.Messenger.ID(),
		},
	)
	if err != nil {
		return nil, err
	}

	return e, nil
}

// SyncEpochStartMeta syncs the latest epoch start metablock
func (e *epochStartMetaSyncer) SyncEpochStartMeta(timeToWait time.Duration) (*block.Block, error) {
	err := e.initTopicForEpochStartMetaBlockInterceptor()
	if err != nil {
		return nil, err
	}
	defer func() {
		e.resetTopicsAndInterceptors()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeToWait)
	mb, errConsensusNotReached := e.blockProcessor.GetEpochStartBlock(ctx)
	cancel()

	if errConsensusNotReached != nil {
		return nil, errConsensusNotReached
	}

	return mb, nil
}

// SyncPrevEpochStartMeta syncs the pre latest epoch start metablock
func (e *epochStartMetaSyncer) SyncPrevEpochStartMeta(timeToWait time.Duration, epoch uint32) (*block.Block, error) {
	err := e.initTopicForEpochStartMetaBlockInterceptor()
	if err != nil {
		return nil, err
	}
	defer func() {
		e.resetTopicsAndInterceptors()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeToWait)
	mb, errConsensusNotReached := e.blockProcessor.GetPrevEpochStartBlock(ctx, epoch)
	cancel()

	if errConsensusNotReached != nil {
		return nil, errConsensusNotReached
	}

	return mb, nil
}

func (e *epochStartMetaSyncer) resetTopicsAndInterceptors() {
	err := e.messenger.UnregisterMessageProcessor(common.BlocksTopic)
	if err != nil {
		log.Trace("error unregistering message processors", "error", err)
	}
}

func (e *epochStartMetaSyncer) initTopicForEpochStartMetaBlockInterceptor() error {
	err := e.messenger.CreateTopic(common.BlocksTopic, true)
	if err != nil {
		log.Warn("error messenger create topic", "error", err)
		return err
	}

	e.resetTopicsAndInterceptors()
	err = e.messenger.RegisterMessageProcessor(common.BlocksTopic, e.singleDataInterceptor)
	if err != nil {
		return err
	}

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (e *epochStartMetaSyncer) IsInterfaceNil() bool {
	return e == nil
}
