package bootstrap

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.InterceptorProcessor = (*epochStartBlockProcessor)(nil)

type storageEpochStartBlockProcessor struct {
	messenger      Messenger
	requestHandler RequestHandler
	marshalizer    marshal.Marshalizer
	hasher         hashing.Hasher
	chanReceived   chan struct{}
	mutMetablock   sync.Mutex
	metaBlock      *block.Block
}

// NewStorageEpochStartBlockProcessor will return an interceptor processor for epoch start meta block when importing
// data from storage
func NewStorageEpochStartBlockProcessor(
	messenger Messenger,
	handler RequestHandler,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
) (*storageEpochStartBlockProcessor, error) {
	if check.IfNil(messenger) {
		return nil, common.ErrNilMessenger
	}
	if check.IfNil(handler) {
		return nil, common.ErrNilRequestHandler
	}
	if check.IfNil(marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}

	processor := &storageEpochStartBlockProcessor{
		messenger:      messenger,
		requestHandler: handler,
		marshalizer:    marshalizer,
		hasher:         hasher,
		chanReceived:   make(chan struct{}, 1),
	}

	return processor, nil
}

// Validate will return nil as there is no need for validation
func (ses *storageEpochStartBlockProcessor) Validate(_ process.InterceptedData, _ core.PeerID) error {
	return nil
}

// Save will handle the consensus mechanism for the fetched metablocks
// All errors are just logged because if this function returns an error, the processing is finished. This way, we ignore
// wrong received data and wait for relevant intercepted data
func (ses *storageEpochStartBlockProcessor) Save(data process.InterceptedData, _ core.PeerID, _ string) error {
	if check.IfNil(data) {
		log.Debug("epoch bootstrapper: nil intercepted data")
		return nil
	}

	log.Debug("received header", "type", data.Type(), "hash", data.Hash())
	interceptedHdr, ok := data.(process.HdrValidatorHandler)
	if !ok {
		log.Warn("saving epoch start meta block error", "error", common.ErrWrongTypeAssertion)
		return nil
	}

	metaBlock, ok := interceptedHdr.HeaderHandler().(*block.Block)
	if !ok {
		log.Warn("saving epoch start meta block error", "error", common.ErrWrongTypeAssertion,
			"header", interceptedHdr.HeaderHandler())
		return nil
	}

	if !metaBlock.GetIsEpochStart() {
		log.Warn("received metablock is not of type epoch start", "error", common.ErrNotEpochStartBlock)
		return nil
	}

	log.Debug("received epoch start meta", "epoch", metaBlock.GetEpoch(), "from peer", "self")
	ses.mutMetablock.Lock()
	ses.metaBlock = metaBlock
	ses.mutMetablock.Unlock()

	select {
	case ses.chanReceived <- struct{}{}:
	default:
	}

	return nil
}

// GetEpochStartMetaBlock will return the metablock after it is confirmed or an error if the number of tries was exceeded
// This is a blocking method which will end after the consensus for the meta block is obtained or the context is done
func (ses *storageEpochStartBlockProcessor) GetEpochStartBlock(ctx context.Context) (*block.Block, error) {
	ses.requestMetaBlock()

	chanRequests := time.After(durationBetweenReRequests)
	for {
		select {
		case <-ses.chanReceived:
			return ses.getMetablock()
		case <-ctx.Done():
			return ses.getMetablock()
		case <-chanRequests:
			ses.requestMetaBlock()
			chanRequests = time.After(durationBetweenReRequests)
		}
	}
}

// todo: temporary
func (ses *storageEpochStartBlockProcessor) GetPrevEpochStartBlock(ctx context.Context, epoch uint32) (*block.Block, error) {
	err := ses.requestMetaBlockByEpoch(epoch)
	if err != nil {
		return nil, err
	}

	chanRequests := time.After(durationBetweenReRequests)
	for {
		select {
		case <-ses.chanReceived:
			return ses.getMetablock()
		case <-ctx.Done():
			return ses.getMetablock()
		case <-chanRequests:
			ses.requestMetaBlock()
			chanRequests = time.After(durationBetweenReRequests)
		}
	}
}

func (ses *storageEpochStartBlockProcessor) getMetablock() (*block.Block, error) {
	ses.mutMetablock.Lock()
	defer ses.mutMetablock.Unlock()

	if check.IfNil(ses.metaBlock) {
		return nil, process.ErrNilBlockHeader
	}

	return ses.metaBlock, nil
}

func (ses *storageEpochStartBlockProcessor) requestMetaBlock() {
	unknownEpoch := uint32(math.MaxUint32)
	ses.requestHandler.RequestStartOfEpochBlock(unknownEpoch)
}

// RegisterHandler registers a callback function to be notified of incoming epoch start metablocks
func (ses *storageEpochStartBlockProcessor) RegisterHandler(_ func(topic string, hash []byte, data interface{})) {
	log.Error("storageEpochStartBlockProcessor.RegisterHandler not implemented")
}

// IsInterfaceNil returns true if there is no value under the interface
func (ses *storageEpochStartBlockProcessor) IsInterfaceNil() bool {
	return ses == nil
}

func (ses *storageEpochStartBlockProcessor) requestMetaBlockByEpoch(epoch uint32) error {
	numConnectedPeers := len(ses.messenger.ConnectedPeers())
	err := ses.requestHandler.SetNumPeersToQuery(common.BlocksTopic, numConnectedPeers, numConnectedPeers)
	if err != nil {
		return err
	}

	ses.requestHandler.RequestStartOfEpochBlock(epoch)
	return nil
}
