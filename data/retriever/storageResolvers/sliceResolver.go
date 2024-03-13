package storageResolvers

import (
	"fmt"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// maxBuffToSend represents max buffer size to send in bytes
const maxBuffToSend = 1 << 18 //256KB

// ArgSliceResolver is the argument structure used to create a new sliceResolver instance
type ArgSliceResolver struct {
	Messenger                retriever.MessageHandler
	ResponseTopicName        string
	Storage                  storage.Storer
	DataPacker               retriever.DataPacker
	Marshalizer              marshal.Marshalizer
	ManualEpochStartNotifier retriever.ManualEpochStartNotifier
	ChanGracefullyClose      chan endProcess.ArgEndProcess
	DelayBeforeGracefulClose time.Duration
}

type sliceResolver struct {
	*storageResolver
	storage     storage.Storer
	dataPacker  retriever.DataPacker
	marshalizer marshal.Marshalizer
}

// NewSliceResolver is a wrapper over Resolver that is specialized in resolving single and multiple requests
func NewSliceResolver(arg ArgSliceResolver) (*sliceResolver, error) {
	if check.IfNil(arg.Messenger) {
		return nil, common.ErrNilMessenger
	}
	if check.IfNil(arg.Storage) {
		return nil, common.ErrNilStore
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(arg.DataPacker) {
		return nil, common.ErrNilDataPacker
	}
	if check.IfNil(arg.ManualEpochStartNotifier) {
		return nil, common.ErrNilManualEpochStartNotifier
	}
	if arg.ChanGracefullyClose == nil {
		return nil, common.ErrNilGracefullyCloseChannel
	}

	return &sliceResolver{
		storageResolver: &storageResolver{
			messenger:                arg.Messenger,
			responseTopicName:        arg.ResponseTopicName,
			manualEpochStartNotifier: arg.ManualEpochStartNotifier,
			chanGracefullyClose:      arg.ChanGracefullyClose,
			delayBeforeGracefulClose: arg.DelayBeforeGracefulClose,
		},
		storage:     arg.Storage,
		dataPacker:  arg.DataPacker,
		marshalizer: arg.Marshalizer,
	}, nil
}

// RequestDataFromHash searches the hash in provided storage and then will send to self the message
func (sliceRes *sliceResolver) RequestDataFromHash(hash []byte, _ uint32) error {
	mb, err := sliceRes.storage.Get(hash)
	if err != nil {
		sliceRes.signalGracefullyClose()

		return err
	}

	b := &batch.Batch{
		Data: [][]byte{mb},
	}
	buffToSend, err := sliceRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return sliceRes.sendToSelf(buffToSend)
}

// RequestDataFromHashArray searches the hashes in provided storage and then will send to self the message(s)
func (sliceRes *sliceResolver) RequestDataFromHashArray(hashes [][]byte, _ uint32) error {
	var errFetch error
	errorsFound := 0
	mbsBuffSlice := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		mb, errTemp := sliceRes.storage.Get(hash)
		if errTemp != nil {
			errFetch = fmt.Errorf("%w for hash %s", errTemp, logger.DisplayByteSlice(hash))
			log.Trace("fetchMbAsByteSlice missing",
				"error", errFetch.Error(),
				"hash", hash)
			errorsFound++

			continue
		}
		mbsBuffSlice = append(mbsBuffSlice, mb)
	}

	buffsToSend, errPack := sliceRes.dataPacker.PackDataInChunks(mbsBuffSlice, maxBuffToSend)
	if errPack != nil {
		return errPack
	}

	for _, buff := range buffsToSend {
		errSend := sliceRes.sendToSelf(buff)
		if errSend != nil {
			return errSend
		}
	}

	if errFetch != nil {
		errFetch = fmt.Errorf("resolveMbRequestByHashArray last error %w from %d encountered errors", errFetch, errorsFound)

		sliceRes.signalGracefullyClose()
	}

	return errFetch
}

// It is not supported when trying to request from storage
func (sliceRes *sliceResolver) RequestDataFromHashArrayTo(hashes [][]byte, epoch uint32, peer core.PeerID) error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (sliceRes *sliceResolver) IsInterfaceNil() bool {
	return sliceRes == nil
}
