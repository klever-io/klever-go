package factory

import (
	"errors"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func TestNewBootstrapDataProvider_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	bdp, err := NewBootstrapDataProvider(nil)
	require.True(t, check.IfNil(bdp))
	require.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewBootstrapDataProvider_OkValuesShouldWork(t *testing.T) {
	t.Parallel()

	bdp, err := NewBootstrapDataProvider(&mock.MarshalizerMock{})
	require.False(t, check.IfNil(bdp))
	require.NoError(t, err)
}

func TestBootstrapDataProvider_LoadForPath_PersisterCreateErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected err")
	bdp, _ := NewBootstrapDataProvider(&mock.MarshalizerMock{})
	persisterFactory := &mock.PersisterFactoryStub{
		CreateCalled: func(_ string) (persister storage.Persister, e error) {
			persister, e = nil, expectedErr
			return
		},
	}

	bootstrapData, storer, err := bdp.LoadForPath(persisterFactory, "")
	require.Equal(t, expectedErr, err)
	require.Nil(t, storer)
	require.Nil(t, bootstrapData)
}

func TestBootstrapDataProvider_LoadForPath_KeyNotFound(t *testing.T) {
	t.Parallel()

	bdp, _ := NewBootstrapDataProvider(&mock.MarshalizerMock{})
	persisterFactory := &mock.PersisterFactoryStub{
		CreateCalled: func(_ string) (persister storage.Persister, e error) {
			persister, e = memorydb.NewlruDB(20)
			return
		},
	}

	bootstrapData, storer, err := bdp.LoadForPath(persisterFactory, "")
	require.NotNil(t, err)
	require.Nil(t, storer)
	require.Nil(t, bootstrapData)
}

func TestBootstrapDataProvider_LoadForPath_ShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	bdp, _ := NewBootstrapDataProvider(marshalizer)
	persisterToUse := memorydb.New()

	expectedSlot := int64(37)
	slotNum := bootstrapStorage.SlotNum{Num: expectedSlot}
	slotNumBytes, _ := marshalizer.Marshal(&slotNum)
	expectedBD := &bootstrapStorage.BootstrapData{LastSlot: 37}
	expectedBDBytes, _ := marshalizer.Marshal(expectedBD)

	_ = persisterToUse.Put([]byte(core.HighestSlotFromBootStorage), slotNumBytes)

	key := []byte(strconv.FormatInt(expectedSlot, 10))
	_ = persisterToUse.Put(key, expectedBDBytes)
	persisterFactory := &mock.PersisterFactoryStub{
		CreateCalled: func(_ string) (storage.Persister, error) {
			return persisterToUse, nil
		},
	}

	bootstrapData, storer, err := bdp.LoadForPath(persisterFactory, "")
	require.NoError(t, err)
	require.NotNil(t, storer)
	require.Equal(t, expectedBD, bootstrapData)
}
