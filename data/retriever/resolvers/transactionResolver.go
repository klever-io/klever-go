package resolvers

import (
	"fmt"
	"runtime/debug"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/requestHandlers"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ requestHandlers.HashSliceResolver = (*TxResolver)(nil)
var _ retriever.Resolver = (*TxResolver)(nil)

// maxBuffToSendBulkTransactions represents max buffer size to send in bytes
const maxBuffToSendBulkTransactions = 1 << 18 //256KB

// ArgTxResolver is the argument structure used to create new TxResolver instance
type ArgTxResolver struct {
	SenderResolver   retriever.TopicResolverSender
	TxPool           retriever.ShardedDataCacherNotifier
	TxStorage        storage.Storer
	Marshalizer      marshal.Marshalizer
	DataPacker       retriever.DataPacker
	AntifloodHandler retriever.P2PAntifloodHandler
	Throttler        retriever.ResolverThrottler
}

// TxResolver is a wrapper over Resolver that is specialized in resolving transaction requests
type TxResolver struct {
	retriever.TopicResolverSender
	messageProcessor
	txPool     retriever.ShardedDataCacherNotifier
	txStorage  storage.Storer
	dataPacker retriever.DataPacker
}

// NewTxResolver creates a new transaction resolver
func NewTxResolver(arg ArgTxResolver) (*TxResolver, error) {
	if check.IfNil(arg.SenderResolver) {
		return nil, common.ErrNilResolverSender
	}
	if check.IfNil(arg.TxPool) {
		return nil, common.ErrNilTxDataPool
	}
	if check.IfNil(arg.TxStorage) {
		return nil, common.ErrNilTxStorage
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(arg.DataPacker) {
		return nil, common.ErrNilDataPacker
	}
	if check.IfNil(arg.AntifloodHandler) {
		return nil, common.ErrNilAntifloodHandler
	}
	if check.IfNil(arg.Throttler) {
		return nil, common.ErrNilThrottler
	}

	txResolver := &TxResolver{
		TopicResolverSender: arg.SenderResolver,
		txPool:              arg.TxPool,
		txStorage:           arg.TxStorage,
		dataPacker:          arg.DataPacker,
		messageProcessor: messageProcessor{
			marshalizer:      arg.Marshalizer,
			antifloodHandler: arg.AntifloodHandler,
			topic:            arg.SenderResolver.RequestTopic(),
			throttler:        arg.Throttler,
		},
	}

	return txResolver, nil
}

// ProcessReceivedMessage will be the callback func from the p2p.Messenger and will be called each time a new message was received
// (for the topic this validator was registered to, usually a request topic)
func (txRes *TxResolver) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("TxResolver.ProcessReceivedMessage panicked",
				"panic", r,
				"stack", string(debug.Stack()))
			err = fmt.Errorf("%w: %v", common.ErrProcessReceivedMessagePanicked, r)
		}
	}()

	err = txRes.canProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	txRes.throttler.StartProcessing()
	defer txRes.throttler.EndProcessing()

	rd, err := txRes.parseReceivedMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	switch rd.Type {
	case retriever.RequestDataType_HashType:
		err = txRes.resolveTxRequestByHash(rd.Value, message.Peer())
	case retriever.RequestDataType_HashArrayType:
		err = txRes.resolveTxRequestByHashArray(rd.Value, message.Peer())
	default:
		err = common.ErrRequestTypeNotImplemented
	}

	if err != nil {
		err = fmt.Errorf("%w for hash %s", err, logger.DisplayByteSlice(rd.Value))
	}

	return err
}

func (txRes *TxResolver) cleanTxBeforeSend(tx []byte) ([]byte, error) {
	transaction := &dataTransaction.Transaction{}
	if err := txRes.marshalizer.Unmarshal(transaction, tx); err != nil {
		return nil, err
	}
	transaction.PrepareForProcessing()

	txCleaned, err := txRes.marshalizer.Marshal(transaction)
	if err != nil {
		return nil, err
	}
	return txCleaned, nil
}

func (txRes *TxResolver) resolveTxRequestByHash(hash []byte, pid core.PeerID) error {
	//TODO this can be optimized by searching in corresponding datapool (taken by topic name)
	tx, err := txRes.fetchTxAsByteSlice(hash)
	if err != nil {
		return err
	}

	txCleaned, err := txRes.cleanTxBeforeSend(tx)
	if err != nil {
		return err
	}

	b := &batch.Batch{
		Data: [][]byte{txCleaned},
	}
	buff, err := txRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return txRes.Send(buff, pid)
}

func (txRes *TxResolver) fetchTxAsByteSlice(hash []byte) ([]byte, error) {
	value, ok := txRes.txPool.SearchFirstData(hash)
	if ok {
		return txRes.marshalizer.Marshal(value)
	}

	buff, err := txRes.txStorage.SearchFirst(hash)
	if err != nil {
		txRes.ResolverDebugHandler().LogFailedToResolveData(
			txRes.topic,
			hash,
			err,
		)

		return nil, err
	}

	txRes.ResolverDebugHandler().LogSucceededToResolveData(txRes.topic, hash)

	return buff, nil
}

func (txRes *TxResolver) resolveTxRequestByHashArray(hashesBuff []byte, pid core.PeerID) error {
	//TODO this can be optimized by searching in corresponding datapool (taken by topic name)
	// Reject oversized hash-array payloads before Unmarshal — see MaxHashArrayBuffSize.
	// No blacklist (logical bound, antiflood gates abuse).
	if len(hashesBuff) > batch.MaxHashArrayBuffSize {
		return common.ErrBatchWireTooLarge
	}
	b := batch.Batch{}
	err := txRes.marshalizer.Unmarshal(&b, hashesBuff)
	if err != nil {
		return err
	}
	// Cap uncompressed batches; the compressed path is bounded inside b.Decompress.
	if len(b.Data) > batch.MaxItemsPerBatch {
		return common.ErrTooManyItemsInBatch
	}
	if b.IsCompressed {
		err = b.Decompress(txRes.marshalizer)
		if err != nil {
			log.Error("TxResolver.resolveTxRequestByHashArray", "err", err.Error())
			return err
		}
	}
	hashes := b.Data

	var errFetch error
	errorsFound := 0
	txsBuffSlice := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		tx, errTemp := txRes.fetchTxAsByteSlice(hash)
		if errTemp != nil {
			errFetch = fmt.Errorf("%w for hash %s", errTemp, logger.DisplayByteSlice(hash))
			//it might happen to error on a tx (maybe it is missing) but should continue
			// as to send back as many as it can
			log.Trace("fetchTxAsByteSlice missing",
				"error", errFetch.Error(),
				"hash", hash)
			errorsFound++

			continue
		}

		txCleaned, err := txRes.cleanTxBeforeSend(tx)
		if err != nil {
			return err
		}

		txsBuffSlice = append(txsBuffSlice, txCleaned)
	}

	buffsToSend, errPack := txRes.dataPacker.PackDataInChunks(txsBuffSlice, maxBuffToSendBulkTransactions)
	if errPack != nil {
		return errPack
	}

	for _, buff := range buffsToSend {
		errSend := txRes.Send(buff, pid)
		if errSend != nil {
			return errSend
		}
	}

	if errFetch != nil {
		errFetch = fmt.Errorf("resolveTxRequestByHashArray last error %w from %d encountered errors", errFetch, errorsFound)
	}

	return errFetch
}

// RequestDataFromHash requests a transaction from other peers having input the tx hash
func (txRes *TxResolver) RequestDataFromHash(hash []byte, epoch uint32) error {
	return txRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashType,
			Value: hash,
			Epoch: epoch,
		},
		[][]byte{hash},
	)
}

// RequestDataFromHashArray requests a list of tx hashes from other peers
func (txRes *TxResolver) RequestDataFromHashArray(hashes [][]byte, epoch uint32) error {
	b := &batch.Batch{
		Data: hashes,
	}
	buffHashes, err := txRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return txRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashArrayType,
			Value: buffHashes,
			Epoch: epoch,
		},
		hashes,
	)
}

// RequestDataFromHashArrayTo requests a list of tx hashes from other peers
func (txRes *TxResolver) RequestDataFromHashArrayTo(hashes [][]byte, epoch uint32, peer core.PeerID) error {
	b := &batch.Batch{
		Data: hashes,
	}
	buffHashes, err := txRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return txRes.SendOnRequestTopicTo(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashArrayType,
			Value: buffHashes,
			Epoch: epoch,
		},
		hashes,
		peer,
	)
}

// SetNumPeersToQuery will set the number of intra shard and cross shard number of peer to query
func (txRes *TxResolver) SetNumPeersToQuery(intra int, cross int) {
	txRes.TopicResolverSender.SetNumPeersToQuery(intra, cross)
}

// NumPeersToQuery will return the number of intra shard and cross shard number of peer to query
func (txRes *TxResolver) NumPeersToQuery() (int, int) {
	return txRes.TopicResolverSender.NumPeersToQuery()
}

// SetResolverDebugHandler will set a resolver debug handler
func (txRes *TxResolver) SetResolverDebugHandler(handler retriever.ResolverDebugHandler) error {
	return txRes.TopicResolverSender.SetResolverDebugHandler(handler)
}

// IsInterfaceNil returns true if there is no value under the interface
func (txRes *TxResolver) IsInterfaceNil() bool {
	return txRes == nil
}
