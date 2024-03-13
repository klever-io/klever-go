package bootstrapStorage_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
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
