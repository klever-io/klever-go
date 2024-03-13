package resolvers

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/resolvers/epochproviders"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

var log = logger.GetOrCreate("data/retriever/resolvers")

var _ retriever.HeaderResolver = (*HeaderResolver)(nil)

// ArgHeaderResolver is the argument structure used to create new HeaderResolver instance
type ArgHeaderResolver struct {
	SenderResolver       retriever.TopicResolverSender
	Headers              retriever.HeadersPool
	HdrStorage           storage.Storer
	HeadersNoncesStorage storage.Storer
	Marshalizer          marshal.Marshalizer
	NonceConverter       typeConverters.Uint64ByteSliceConverter
	AntifloodHandler     retriever.P2PAntifloodHandler
	Throttler            retriever.ResolverThrottler
	DataPacker           retriever.DataPacker
}

// HeaderResolver is a wrapper over Resolver that is specialized in resolving headers requests
type HeaderResolver struct {
	retriever.TopicResolverSender
	messageProcessor
	headers              retriever.HeadersPool
	hdrStorage           storage.Storer
	hdrNoncesStorage     storage.Storer
	nonceConverter       typeConverters.Uint64ByteSliceConverter
	epochHandler         retriever.EpochHandler
	epochProviderByNonce retriever.EpochProviderByNonce
	dataPacker           retriever.DataPacker
}

// NewHeaderResolver creates a new header resolver
func NewHeaderResolver(arg ArgHeaderResolver) (*HeaderResolver, error) {
	if check.IfNil(arg.SenderResolver) {
		return nil, common.ErrNilResolverSender
	}
	if check.IfNil(arg.Headers) {
		return nil, common.ErrNilHeadersDataPool
	}
	if check.IfNil(arg.HdrStorage) {
		return nil, common.ErrNilHeadersStorage
	}
	if check.IfNil(arg.HeadersNoncesStorage) {
		return nil, common.ErrNilHeadersNoncesStorage
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(arg.NonceConverter) {
		return nil, common.ErrNilUint64ByteSliceConverter
	}
	if check.IfNil(arg.AntifloodHandler) {
		return nil, common.ErrNilAntifloodHandler
	}
	if check.IfNil(arg.Throttler) {
		return nil, common.ErrNilThrottler
	}
	if check.IfNil(arg.DataPacker) {
		return nil, common.ErrNilDataPacker
	}

	epochHandler := epochproviders.NewNilEpochHandler()
	hdrResolver := &HeaderResolver{
		TopicResolverSender:  arg.SenderResolver,
		headers:              arg.Headers,
		hdrStorage:           arg.HdrStorage,
		hdrNoncesStorage:     arg.HeadersNoncesStorage,
		nonceConverter:       arg.NonceConverter,
		epochHandler:         epochHandler,
		dataPacker:           arg.DataPacker,
		epochProviderByNonce: epochproviders.NewSimpleEpochProviderByNonce(epochHandler),
		messageProcessor: messageProcessor{
			marshalizer:      arg.Marshalizer,
			antifloodHandler: arg.AntifloodHandler,
			topic:            arg.SenderResolver.RequestTopic(),
			throttler:        arg.Throttler,
		},
	}

	return hdrResolver, nil
}

// SetEpochHandler sets the epoch handler for this component
func (hdrRes *HeaderResolver) SetEpochHandler(epochHandler retriever.EpochHandler) error {
	if check.IfNil(epochHandler) {
		return common.ErrNilEpochHandler
	}

	hdrRes.epochHandler = epochHandler
	return nil
}

// ProcessReceivedMessage will be the callback func from the p2p.Messenger and will be called each time a new message was received
// (for the topic this validator was registered to, usually a request topic)
func (hdrRes *HeaderResolver) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	err := hdrRes.canProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	hdrRes.throttler.StartProcessing()
	defer hdrRes.throttler.EndProcessing()

	rd, err := hdrRes.parseReceivedMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	var buff []byte

	switch rd.Type {
	case retriever.RequestDataType_HashType:
		buff, err = hdrRes.resolveHeaderFromHash(rd)
	case retriever.RequestDataType_NonceType:
		buff, err = hdrRes.resolveHeaderFromNonce(rd)
	case retriever.RequestDataType_EpochType:
		buff, err = hdrRes.resolveHeaderFromEpoch(rd.Value)
	default:
		return common.ErrResolveTypeUnknown
	}
	if err != nil {
		hdrRes.ResolverDebugHandler().LogFailedToResolveData(
			hdrRes.topic,
			rd.Value,
			err,
		)
		return err
	}

	if buff == nil {
		hdrRes.ResolverDebugHandler().LogFailedToResolveData(
			hdrRes.topic,
			rd.Value,
			common.ErrMissingData,
		)

		log.Trace("missing data",
			"data", rd)
		return nil
	}

	hdrRes.ResolverDebugHandler().LogSucceededToResolveData(hdrRes.topic, rd.Value)

	return hdrRes.Send(buff, message.Peer())
}

func (hdrRes *HeaderResolver) resolveHeaderFromNonce(rd *retriever.RequestData) ([]byte, error) {
	// key is now an encoded nonce (uint64)
	nonce, err := hdrRes.nonceConverter.ToUint64(rd.Value)
	if err != nil {
		return nil, common.ErrInvalidNonceByteSlice
	}

	epoch := rd.Epoch

	// TODO : uncomment this when epoch provider by nonce is complete
	//epoch, err = hdrRes.epochProviderByNonce.EpochForNonce(nonce)
	//if err != nil {
	//	return nil, err
	//}
	//
	//hash, err := hdrRes.hdrNoncesStorage.GetFromEpoch(rd.Value, epoch)
	hash, err := hdrRes.hdrNoncesStorage.SearchFirst(rd.Value)
	if err != nil {
		log.Trace("hdrNoncesStorage.Get from calculated epoch", "error", err.Error())
		// Search the nonce-key pair in data pool
		var hdrBytes []byte
		hdrBytes, err = hdrRes.searchInCache(nonce)
		if err != nil {
			return nil, err
		}

		return hdrBytes, nil
	}

	newRd := &retriever.RequestData{
		Type:  rd.Type,
		Value: hash,
		Epoch: epoch,
	}

	return hdrRes.resolveHeaderFromHash(newRd)
}

func (hdrRes *HeaderResolver) searchInCache(nonce uint64) ([]byte, error) {
	headers, _, err := hdrRes.headers.GetHeadersByNonce(nonce)
	if err != nil {
		return nil, err
	}

	hdr := headers[len(headers)-1]
	buff, err := hdrRes.marshalizer.Marshal(hdr)
	if err != nil {
		return nil, err
	}

	return buff, nil
}

// resolveHeaderFromHash resolves a header using its key (header hash)
func (hdrRes *HeaderResolver) resolveHeaderFromHash(rd *retriever.RequestData) ([]byte, error) {
	value, err := hdrRes.headers.GetHeaderByHash(rd.Value)
	if err != nil {
		return hdrRes.hdrStorage.SearchFirst(rd.Value)

		// TODO : uncomment this when epoch provider by nonce is complete

		//  return hdrRes.hdrStorage.GetFromEpoch(rd.Value, rd.Epoch)
	}

	return hdrRes.marshalizer.Marshal(value)
}

// resolveHeaderFromEpoch resolves a header using its key based on epoch
func (hdrRes *HeaderResolver) resolveHeaderFromEpoch(key []byte) ([]byte, error) {
	actualKey := key

	isUnknownEpoch, err := core.IsUnknownEpochIdentifier(key)
	if err != nil {
		return nil, err
	}
	if isUnknownEpoch {
		actualKey = []byte(core.EpochStartIdentifier(hdrRes.epochHandler.Epoch()))
	}

	return hdrRes.hdrStorage.SearchFirst(actualKey)
}

// RequestDataFromHash requests a header from other peers having input the hdr hash
func (hdrRes *HeaderResolver) RequestDataFromHash(hash []byte, epoch uint32) error {
	return hdrRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashType,
			Value: hash,
			Epoch: epoch,
		},
		[][]byte{hash},
	)
}

// RequestDataFromNonce requests a header from other peers having input the hdr nonce
func (hdrRes *HeaderResolver) RequestDataFromNonce(nonce uint64, epoch uint32) error {
	return hdrRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_NonceType,
			Value: hdrRes.nonceConverter.ToByteSlice(nonce),
			Epoch: epoch,
		},
		[][]byte{hdrRes.nonceConverter.ToByteSlice(nonce)},
	)
}

// RequestDataFromEpoch requests a header from other peers having input the epoch
func (hdrRes *HeaderResolver) RequestDataFromEpoch(identifier []byte) error {
	return hdrRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_EpochType,
			Value: identifier,
		},
		[][]byte{identifier},
	)
}

// SetNumPeersToQuery will set the number of intra shard and cross shard number of peer to query
func (hdrRes *HeaderResolver) SetNumPeersToQuery(intra int, cross int) {
	hdrRes.TopicResolverSender.SetNumPeersToQuery(intra, cross)
}

// NumPeersToQuery will return the number of intra shard and cross shard number of peer to query
func (hdrRes *HeaderResolver) NumPeersToQuery() (int, int) {
	return hdrRes.TopicResolverSender.NumPeersToQuery()
}

// SetResolverDebugHandler will set a resolver debug handler
func (hdrRes *HeaderResolver) SetResolverDebugHandler(handler retriever.ResolverDebugHandler) error {
	return hdrRes.TopicResolverSender.SetResolverDebugHandler(handler)
}

// IsInterfaceNil returns true if there is no value under the interface
func (hdrRes *HeaderResolver) IsInterfaceNil() bool {
	return hdrRes == nil
}
