package bootstrapStorage_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/pruning"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/stretchr/testify/assert"
)

var (
	testMarshalizer = &mock.MarshalizerMock{}
)

func TestNewBootstrapStorer_NilStorerShouldErr(t *testing.T) {
	t.Parallel()

	bt, err := bootstrapStorage.NewBootstrapStorer(testMarshalizer, nil)

	assert.Nil(t, bt)
	assert.Equal(t, bootstrapStorage.ErrNilBootStorer, err)
}

func TestNewBootstrapStorer_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	storer := &mock.StorerStub{}
	bt, err := bootstrapStorage.NewBootstrapStorer(nil, storer)

	assert.Nil(t, bt)
	assert.Equal(t, bootstrapStorage.ErrNilMarshalizer, err)
}

func TestNewBootstrapStorer_ShouldWork(t *testing.T) {
	t.Parallel()

	storer := mock.NewStorerMock("", 0)
	bt, err := bootstrapStorage.NewBootstrapStorer(testMarshalizer, storer)

	assert.NotNil(t, bt)
	assert.Nil(t, err)
	assert.False(t, bt.IsInterfaceNil())
}

func TestBootstrapStorer_PutAndGet(t *testing.T) {
	t.Parallel()

	numSlots := int64(10)
	slot := int64(0)
	storer := mock.NewStorerMock("", 1)
	bt, _ := bootstrapStorage.NewBootstrapStorer(testMarshalizer, storer)

	headerInfo := &bootstrapStorage.BootstrapHeaderInfo{Nonce: 3, Hash: []byte("Hash")}
	dataBoot := &bootstrapStorage.BootstrapData{
		LastHeader: headerInfo,
	}

	err := bt.Put(slot, dataBoot)
	assert.Nil(t, err)

	for i := int64(0); i < numSlots; i++ {
		slot = i
		err = bt.Put(slot, dataBoot)
		assert.Nil(t, err)
	}

	slot = bt.GetHighestSlot()
	for i := numSlots - 1; i >= 0; i-- {
		dataBoot.LastSlot = i - 1
		if i == 0 {
			dataBoot.LastSlot = 0
		}
		data, err := bt.Get(slot)
		assert.Nil(t, err)
		assert.Equal(t, dataBoot, data)
		slot--
	}
}

func TestBootstrapStorer_SaveLastSlot(t *testing.T) {
	t.Parallel()

	putWasCalled := false
	slotInStorage := int64(5)
	marshalizer := &mock.MarshalizerMock{}
	storer := &mock.StorerStub{
		PutCalled: func(key, data []byte) error {
			putWasCalled = true
			rn := bootstrapStorage.SlotNum{}
			err := marshalizer.Unmarshal(&rn, data)
			slotInStorage = rn.Num
			if err != nil {
				fmt.Println(err.Error())
			}
			return nil
		},
		GetCalled: func(key []byte) ([]byte, error) {
			return marshalizer.Marshal(&bootstrapStorage.SlotNum{Num: slotInStorage})
		},
	}
	bt, _ := bootstrapStorage.NewBootstrapStorer(marshalizer, storer)

	assert.Equal(t, slotInStorage, bt.GetHighestSlot())
	newSlot := int64(37)
	err := bt.SaveLastSlot(newSlot)
	assert.Equal(t, newSlot, bt.GetHighestSlot())
	assert.Nil(t, err)
	assert.True(t, putWasCalled)
}

func newBootstrapStorerWithConsecutiveSlots(t *testing.T) (process.BootStorer, func() int) {
	t.Helper()

	numPuts := 0
	inner := mock.NewStorerMock("", 1)
	storer := &mock.StorerStub{
		PutCalled: func(key, data []byte) error {
			numPuts++
			return inner.Put(key, data)
		},
		PutInEpochCalled: func(key, data []byte, epoch uint32) error {
			numPuts++
			return inner.PutInEpoch(key, data, epoch)
		},
		GetCalled: inner.Get,
	}
	bt, err := bootstrapStorage.NewBootstrapStorer(testMarshalizer, storer)
	assert.Nil(t, err)

	for nonce := uint64(3); nonce <= 5; nonce++ {
		slot := int64(nonce) + 5
		err = bt.Put(slot, &bootstrapStorage.BootstrapData{
			LastHeader:                 &bootstrapStorage.BootstrapHeaderInfo{Epoch: 1, Nonce: nonce, Hash: []byte(fmt.Sprintf("hash%d", nonce))},
			HighestFinalBlockNonce:     nonce - 1,
			NodesCoordinatorConfigKey:  []byte(fmt.Sprintf("nodesCoordinator%d", nonce)),
			EpochStartTriggerConfigKey: []byte(fmt.Sprintf("epochStart%d", nonce)),
		})
		assert.Nil(t, err)
	}

	return bt, func() int { return numPuts }
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceShouldChangeOnlyTheFinalNonce(t *testing.T) {
	t.Parallel()

	bt, numPuts := newBootstrapStorerWithConsecutiveSlots(t)

	olderBefore, err := bt.Get(9)
	assert.Nil(t, err)
	assert.Equal(t, uint64(3), olderBefore.HighestFinalBlockNonce)
	assert.Equal(t, int64(8), olderBefore.LastSlot)
	newerBefore, err := bt.Get(10)
	assert.Nil(t, err)
	numPutsBefore := numPuts()

	err = bt.RaiseHighestFinalBlockNonce(9, 4, []byte("hash4"))
	assert.Nil(t, err)

	olderAfter, err := bt.Get(9)
	assert.Nil(t, err)
	assert.Equal(t, uint64(4), olderAfter.HighestFinalBlockNonce)
	assert.Equal(t, olderBefore.LastSlot, olderAfter.LastSlot)
	assert.Equal(t, olderBefore.LastHeader, olderAfter.LastHeader)
	assert.Equal(t, olderBefore.NodesCoordinatorConfigKey, olderAfter.NodesCoordinatorConfigKey)
	assert.Equal(t, olderBefore.EpochStartTriggerConfigKey, olderAfter.EpochStartTriggerConfigKey)
	assert.Equal(t, numPutsBefore+1, numPuts())

	newerAfter, err := bt.Get(10)
	assert.Nil(t, err)
	assert.Equal(t, newerBefore, newerAfter)
	assert.Equal(t, int64(10), bt.GetHighestSlot())

	err = bt.Put(11, &bootstrapStorage.BootstrapData{
		LastHeader: &bootstrapStorage.BootstrapHeaderInfo{Epoch: 1, Nonce: 6, Hash: []byte("hash6")},
	})
	assert.Nil(t, err)

	next, err := bt.Get(11)
	assert.Nil(t, err)
	assert.Equal(t, int64(10), next.LastSlot)
	assert.Equal(t, int64(11), bt.GetHighestSlot())
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceShouldNotLowerTheStoredNonce(t *testing.T) {
	t.Parallel()

	bt, numPuts := newBootstrapStorerWithConsecutiveSlots(t)

	before, err := bt.Get(10)
	assert.Nil(t, err)
	assert.Equal(t, uint64(4), before.HighestFinalBlockNonce)
	numPutsBefore := numPuts()

	for _, nonce := range []uint64{4, 3, 0} {
		err = bt.RaiseHighestFinalBlockNonce(10, nonce, []byte("hash5"))
		assert.Nil(t, err)

		after, errGet := bt.Get(10)
		assert.Nil(t, errGet)
		assert.Equal(t, before, after)
		assert.Equal(t, numPutsBefore, numPuts())
	}

	err = bt.RaiseHighestFinalBlockNonce(10, 5, []byte("hash5"))
	assert.Nil(t, err)

	after, err := bt.Get(10)
	assert.Nil(t, err)
	assert.Equal(t, uint64(5), after.HighestFinalBlockNonce)
	assert.Equal(t, numPutsBefore+1, numPuts())
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceMissingRecordShouldErr(t *testing.T) {
	t.Parallel()

	bt, numPuts := newBootstrapStorerWithConsecutiveSlots(t)
	numPutsBefore := numPuts()

	err := bt.RaiseHighestFinalBlockNonce(7, 4, []byte("hash4"))
	assert.NotNil(t, err)

	_, err = bt.Get(7)
	assert.NotNil(t, err)
	assert.Equal(t, numPutsBefore, numPuts())
	assert.Equal(t, int64(10), bt.GetHighestSlot())
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceOtherHeaderShouldErr(t *testing.T) {
	t.Parallel()

	bt, numPuts := newBootstrapStorerWithConsecutiveSlots(t)

	before, err := bt.Get(9)
	assert.Nil(t, err)
	numPutsBefore := numPuts()

	err = bt.RaiseHighestFinalBlockNonce(9, 4, []byte("hash5"))
	assert.Equal(t, bootstrapStorage.ErrBootDataHeaderMismatch, err)

	after, err := bt.Get(9)
	assert.Nil(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, numPutsBefore, numPuts())
	assert.Equal(t, int64(10), bt.GetHighestSlot())
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceAboveTheHeaderNonceShouldErr(t *testing.T) {
	t.Parallel()

	bt, numPuts := newBootstrapStorerWithConsecutiveSlots(t)

	before, err := bt.Get(9)
	assert.Nil(t, err)
	numPutsBefore := numPuts()

	err = bt.RaiseHighestFinalBlockNonce(9, 5, []byte("hash4"))
	assert.Equal(t, bootstrapStorage.ErrFinalNonceAboveHeader, err)

	after, err := bt.Get(9)
	assert.Nil(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, numPutsBefore, numPuts())
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceStorageFailureShouldErrAndKeepTheRecord(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected error")
	recordKey := []byte("9")

	newFailingBootstrapStorer := func(t *testing.T) (process.BootStorer, *mock.StorerMock, *bool, *bool) {
		failMarshal, failPut := false, false
		inner := mock.NewStorerMock("", 1)
		storer := &mock.StorerStub{
			PutCalled: func(key, data []byte) error {
				if failPut {
					return errExpected
				}
				return inner.Put(key, data)
			},
			PutInEpochCalled: func(key, data []byte, epoch uint32) error {
				if failPut {
					return errExpected
				}
				return inner.PutInEpoch(key, data, epoch)
			},
			GetCalled: inner.Get,
		}
		marshalizer := &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) ([]byte, error) {
				if failMarshal {
					return nil, errExpected
				}
				return testMarshalizer.Marshal(obj)
			},
			UnmarshalCalled: testMarshalizer.Unmarshal,
		}
		bt, err := bootstrapStorage.NewBootstrapStorer(marshalizer, storer)
		assert.Nil(t, err)

		for nonce := uint64(3); nonce <= 5; nonce++ {
			err = bt.Put(int64(nonce)+5, &bootstrapStorage.BootstrapData{
				LastHeader:             &bootstrapStorage.BootstrapHeaderInfo{Epoch: 1, Nonce: nonce, Hash: []byte(fmt.Sprintf("hash%d", nonce))},
				HighestFinalBlockNonce: nonce - 1,
			})
			assert.Nil(t, err)
		}

		return bt, inner, &failMarshal, &failPut
	}

	t.Run("invalid stored record", func(t *testing.T) {
		t.Parallel()

		bt, inner, _, _ := newFailingBootstrapStorer(t)
		invalidRecord := []byte("not a boot record")
		assert.Nil(t, inner.Put(recordKey, invalidRecord))

		err := bt.RaiseHighestFinalBlockNonce(9, 4, []byte("hash4"))
		assert.NotNil(t, err)

		stored, err := inner.Get(recordKey)
		assert.Nil(t, err)
		assert.Equal(t, invalidRecord, stored)
		assert.Equal(t, int64(10), bt.GetHighestSlot())
	})

	t.Run("marshal failure", func(t *testing.T) {
		t.Parallel()

		bt, inner, failMarshal, _ := newFailingBootstrapStorer(t)
		storedBefore, err := inner.Get(recordKey)
		assert.Nil(t, err)

		*failMarshal = true
		err = bt.RaiseHighestFinalBlockNonce(9, 4, []byte("hash4"))
		assert.Equal(t, errExpected, err)

		storedAfter, err := inner.Get(recordKey)
		assert.Nil(t, err)
		assert.Equal(t, storedBefore, storedAfter)
		assert.Equal(t, int64(10), bt.GetHighestSlot())
	})

	t.Run("put failure", func(t *testing.T) {
		t.Parallel()

		bt, inner, _, failPut := newFailingBootstrapStorer(t)
		storedBefore, err := inner.Get(recordKey)
		assert.Nil(t, err)

		*failPut = true
		err = bt.RaiseHighestFinalBlockNonce(9, 4, []byte("hash4"))
		assert.Equal(t, errExpected, err)

		storedAfter, err := inner.Get(recordKey)
		assert.Nil(t, err)
		assert.Equal(t, storedBefore, storedAfter)
		assert.Equal(t, int64(10), bt.GetHighestSlot())
	})
}

func TestBootstrapStorer_RaiseHighestFinalBlockNonceAcrossAnEpochChangeShouldBeReadAfterReopeningAtTheRecordEpoch(t *testing.T) {
	t.Parallel()

	newPruningStorer := func(t *testing.T, persistersByPath map[string]storage.Persister, pruningEnabled bool) (*pruning.PruningStorer, eventNotifier.ActionHandler) {
		t.Helper()

		var epochStartHandler eventNotifier.ActionHandler
		ps, err := pruning.NewPruningStorer(&pruning.StorerArgs{
			Identifier:  "BootstrapData",
			CacheConf:   storageUnit.CacheConfig{Capacity: 10, Type: storageUnit.LRUCache, Shards: 3},
			PathManager: &mock.PathManagerStub{},
			PersisterFactory: &mock.PersisterFactoryStub{
				CreateCalled: func(path string) (storage.Persister, error) {
					persister, ok := persistersByPath[path]
					if !ok {
						persister = memorydb.New()
						persistersByPath[path] = persister
					}

					return persister, nil
				},
			},
			Notifier: &mock.EpochStartNotifierStub{
				RegisterHandlerCalled: func(handler eventNotifier.ActionHandler) {
					epochStartHandler = handler
				},
			},
			MaxBatchSize:          10,
			NumOfEpochsToKeep:     3,
			NumOfActivePersisters: 3,
			PruningEnabled:        pruningEnabled,
		})
		assert.Nil(t, err)

		return ps, epochStartHandler
	}

	tests := []struct {
		name           string
		pruningEnabled bool
	}{
		{name: "storage pruning enabled", pruningEnabled: true},
		{name: "storage pruning disabled", pruningEnabled: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			persistersByPath := make(map[string]storage.Persister)
			ps, epochStartHandler := newPruningStorer(t, persistersByPath, tt.pruningEnabled)
			bt, err := bootstrapStorage.NewBootstrapStorer(testMarshalizer, ps)
			assert.Nil(t, err)

			for nonce := uint64(2); nonce <= 3; nonce++ {
				err = bt.Put(int64(nonce)+5, &bootstrapStorage.BootstrapData{
					LastHeader:             &bootstrapStorage.BootstrapHeaderInfo{Epoch: 0, Nonce: nonce, Hash: []byte(fmt.Sprintf("hash%d", nonce))},
					HighestFinalBlockNonce: nonce - 1,
				})
				assert.Nil(t, err)
			}

			ps.SetEpochForPutOperation(1)
			epochStartHandler.EpochStartAction(&block.Block{Header: &block.BlockHeader{Epoch: 1, Nonce: 4}})
			err = bt.Put(9, &bootstrapStorage.BootstrapData{
				LastHeader:             &bootstrapStorage.BootstrapHeaderInfo{Epoch: 1, Nonce: 4, Hash: []byte("hash4")},
				HighestFinalBlockNonce: 3,
			})
			assert.Nil(t, err)

			err = bt.RaiseHighestFinalBlockNonce(8, 3, []byte("hash3"))
			assert.Nil(t, err)
			err = bt.SaveLastSlot(8)
			assert.Nil(t, err)
			assert.Nil(t, ps.Close())

			reopened, _ := newPruningStorer(t, persistersByPath, tt.pruningEnabled)
			reopenedBootStorer, err := bootstrapStorage.NewBootstrapStorer(testMarshalizer, reopened)
			assert.Nil(t, err)

			assert.Equal(t, int64(8), reopenedBootStorer.GetHighestSlot())
			head, err := reopenedBootStorer.Get(8)
			assert.Nil(t, err)
			assert.Equal(t, []byte("hash3"), head.LastHeader.GetHash())
			assert.Equal(t, uint64(3), head.HighestFinalBlockNonce)
		})
	}
}
