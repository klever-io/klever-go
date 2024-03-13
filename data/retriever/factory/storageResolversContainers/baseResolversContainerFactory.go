package storageResolversContainers

import (
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/storageResolvers"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

const defaultBeforeGracefulClose = time.Minute

type baseResolversContainerFactory struct {
	container                retriever.ResolversContainer
	messenger                retriever.TopicMessageHandler
	store                    retriever.StorageService
	marshalizer              marshal.Marshalizer
	uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	dataPacker               retriever.DataPacker
	manualEpochStartNotifier retriever.ManualEpochStartNotifier
	chanGracefullyClose      chan endProcess.ArgEndProcess
}

func (brcf *baseResolversContainerFactory) checkParams() error {
	if check.IfNil(brcf.messenger) {
		return common.ErrNilMessenger
	}
	if check.IfNil(brcf.store) {
		return common.ErrNilStore
	}
	if check.IfNil(brcf.marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(brcf.uint64ByteSliceConverter) {
		return common.ErrNilUint64ByteSliceConverter
	}
	if check.IfNil(brcf.dataPacker) {
		return common.ErrNilDataPacker
	}
	if check.IfNil(brcf.manualEpochStartNotifier) {
		return common.ErrNilManualEpochStartNotifier
	}
	if brcf.chanGracefullyClose == nil {
		return common.ErrNilGracefullyCloseChannel
	}

	return nil
}

func (brcf *baseResolversContainerFactory) generateTxResolvers(
	topic string,
	unit retriever.UnitType,
) error {

	identifierTx := topic
	resolver, err := brcf.createTxResolver(identifierTx, unit)
	if err != nil {
		return err
	}

	return brcf.container.Add(identifierTx, resolver)
}

func (brcf *baseResolversContainerFactory) createTxResolver(
	responseTopic string,
	unit retriever.UnitType,
) (retriever.Resolver, error) {

	txStorer := brcf.store.GetStorer(unit)

	arg := storageResolvers.ArgSliceResolver{
		Messenger:                brcf.messenger,
		ResponseTopicName:        responseTopic,
		Storage:                  txStorer,
		DataPacker:               brcf.dataPacker,
		Marshalizer:              brcf.marshalizer,
		ManualEpochStartNotifier: brcf.manualEpochStartNotifier,
		ChanGracefullyClose:      brcf.chanGracefullyClose,
		DelayBeforeGracefulClose: defaultBeforeGracefulClose,
	}
	resolver, err := storageResolvers.NewSliceResolver(arg)
	if err != nil {
		return nil, err
	}

	return resolver, nil
}
