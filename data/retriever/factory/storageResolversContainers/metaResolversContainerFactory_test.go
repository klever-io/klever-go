package storageResolversContainers_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/factory/storageResolversContainers"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

var errExpected = errors.New("expected error")

func createStubTopicMessageHandlerForMeta(matchStrToErrOnCreate string, matchStrToErrOnRegister string) retriever.TopicMessageHandler {
	tmhs := mock.NewTopicMessageHandlerStub()

	tmhs.CreateTopicCalled = func(name string, createChannelForTopic bool) error {
		if matchStrToErrOnCreate == "" {
			return nil
		}
		if strings.Contains(name, matchStrToErrOnCreate) {
			return errExpected
		}

		return nil
	}

	tmhs.RegisterMessageProcessorCalled = func(topic string, handler p2p.MessageProcessor) error {
		if matchStrToErrOnRegister == "" {
			return nil
		}
		if strings.Contains(topic, matchStrToErrOnRegister) {
			return errExpected
		}

		return nil
	}

	return tmhs
}

func createStoreForMeta() retriever.StorageService {
	return &mock.ChainStorerMock{
		GetStorerCalled: func(unitType retriever.UnitType) storage.Storer {
			return &mock.StorerStub{}
		},
	}
}

//------- NewResolversContainerFactory

func TestNewMetaResolversContainerFactory_NilMessengerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Messenger = nil
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, rcf)
	assert.Equal(t, common.ErrNilMessenger, err)
}

func TestNewMetaResolversContainerFactory_NilStoreShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Store = nil
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, rcf)
	assert.Equal(t, common.ErrNilStore, err)
}

func TestNewMetaResolversContainerFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Marshalizer = nil
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, rcf)
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewMetaResolversContainerFactory_NilUint64SliceConverterShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.Uint64ByteSliceConverter = nil
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, rcf)
	assert.Equal(t, common.ErrNilUint64ByteSliceConverter, err)
}

func TestNewMetaResolversContainerFactory_NilDataPackerShouldErr(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	args.DataPacker = nil
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, rcf)
	assert.Equal(t, common.ErrNilDataPacker, err)
}

func TestNewMetaResolversContainerFactory_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	rcf, err := storageResolversContainers.NewMetaResolversContainerFactory(args)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(rcf))
}

//------- Create

func TestMetaResolversContainerFactory_CreateShouldWork(t *testing.T) {
	t.Parallel()

	args := getArgumentsMeta()
	rcf, _ := storageResolversContainers.NewMetaResolversContainerFactory(args)

	container, err := rcf.Create()

	assert.NotNil(t, container)
	assert.Nil(t, err)
}

func getArgumentsMeta() storageResolversContainers.FactoryArgs {
	return storageResolversContainers.FactoryArgs{
		Messenger:                createStubTopicMessageHandlerForMeta("", ""),
		Store:                    createStoreForMeta(),
		Marshalizer:              &mock.MarshalizerMock{},
		Uint64ByteSliceConverter: &mock.Uint64ByteSliceConverterMock{},
		DataPacker:               &mock.DataPackerStub{},
		ManualEpochStartNotifier: &mock.ManualEpochStartNotifierStub{},
		ChanGracefullyClose:      make(chan endProcess.ArgEndProcess),
	}
}
