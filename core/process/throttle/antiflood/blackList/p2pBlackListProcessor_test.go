package blackList_test

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/core/process/throttle/antiflood/blackList"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

const selfPid = "current pid"

//-------- NewP2PQuotaBlacklistProcessor

func TestNewP2PQuotaBlacklistProcessor_NilCacherShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		nil,
		&mock.PeerBlackListHandlerStub{},
		1,
		1,
		2,
		time.Second,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrNilCacher))
}

func TestNewP2PQuotaBlacklistProcessor_NilBlackListHandlerShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		nil,
		1,
		1,
		2,
		time.Second,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrNilBlackListCacher))
}

func TestNewP2PQuotaBlacklistProcessor_InvalidThresholdNumReceivedFloodShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		&mock.PeerBlackListHandlerStub{},
		0,
		1,
		2,
		time.Second,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrInvalidValue))
}

func TestNewP2PQuotaBlacklistProcessor_InvalidThresholdSizeReceivedFloodShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		&mock.PeerBlackListHandlerStub{},
		1,
		0,
		2,
		time.Second,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrInvalidValue))
}

func TestNewP2PQuotaBlacklistProcessor_InvalidNumFloodingSlotsShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		&mock.PeerBlackListHandlerStub{},
		1,
		1,
		1,
		time.Second,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrInvalidValue))
}

func TestNewP2PQuotaBlacklistProcessor_InvalidBanDurationShouldErr(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		&mock.PeerBlackListHandlerStub{},
		1,
		1,
		2,
		time.Millisecond,
		"",
		selfPid,
	)

	assert.True(t, check.IfNil(pbp))
	assert.True(t, errors.Is(err, common.ErrInvalidValue))
}

func TestNewP2PQuotaBlacklistProcessor_ShouldWork(t *testing.T) {
	t.Parallel()

	pbp, err := blackList.NewP2PBlackListProcessor(
		commonMock.NewCacherStub(),
		&mock.PeerBlackListHandlerStub{},
		1,
		1,
		2,
		time.Second,
		"",
		selfPid,
	)

	assert.False(t, check.IfNil(pbp))
	assert.Nil(t, err)
}

//------- AddQuota

func TestP2PQuotaBlacklistProcessor_AddQuotaUnderThresholdShouldNotCallGetOrPut(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			GetCalled: func(key []byte) (interface{}, bool) {
				assert.Fail(t, "should not have called get")
				return nil, false
			},
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) (evicted bool) {
				assert.Fail(t, "should not have called put")
				return false
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.AddQuota("identifier", thresholdNum-1, thresholdSize-1, 1, 1)
}

func TestP2PQuotaBlacklistProcessor_AddQuotaOverThresholdInexistentDataOnGetShouldPutOne(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	putCalled := false
	identifier := core.PeerID("identifier")
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			GetCalled: func(key []byte) (interface{}, bool) {
				return nil, false
			},
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) (evicted bool) {
				putCalled = true
				assert.Equal(t, uint32(1), value)
				assert.Equal(t, identifier, core.PeerID(key))

				return false
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.AddQuota(identifier, thresholdNum, thresholdSize, 1, 1)

	assert.True(t, putCalled)
}

func TestP2PQuotaBlacklistProcessor_AddQuotaOverThresholdDataNotValidOnGetShouldPutOne(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	putCalled := false
	identifier := core.PeerID("identifier")
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			GetCalled: func(key []byte) (interface{}, bool) {
				return "invalid data", true
			},
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) (evicted bool) {
				putCalled = true
				assert.Equal(t, uint32(1), value)
				assert.Equal(t, identifier, core.PeerID(key))

				return false
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.AddQuota(identifier, thresholdNum, thresholdSize, 1, 1)

	assert.True(t, putCalled)
}

func TestP2PQuotaBlacklistProcessor_AddQuotaShouldIncrement(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	putCalled := false
	identifier := core.PeerID("identifier")
	existingValue := uint32(445)
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			GetCalled: func(key []byte) (interface{}, bool) {
				return existingValue, true
			},
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) (evicted bool) {
				putCalled = true
				assert.Equal(t, existingValue+1, value)
				assert.Equal(t, identifier, core.PeerID(key))

				return false
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.AddQuota(identifier, thresholdNum, thresholdSize, 1, 1)

	assert.True(t, putCalled)
}

func TestP2PQuotaBlacklistProcessor_AddQuotaForSelfShouldNotIncrement(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	putCalled := false
	existingValue := uint32(445)
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			GetCalled: func(key []byte) (interface{}, bool) {
				return existingValue, true
			},
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) (evicted bool) {
				putCalled = true
				return false
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.AddQuota(selfPid, thresholdNum, thresholdSize, 1, 1)

	assert.False(t, putCalled)
}

//------- ResetStatistics

func TestP2PQuotaBlacklistProcessor_ResetStatisticsRemoveNilValueKey(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	nilValKey := "nil val key"
	removedCalled := false
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			KeysCalled: func() [][]byte {
				return [][]byte{[]byte(nilValKey)}
			},
			PeekCalled: func(key []byte) (value interface{}, ok bool) {
				return nil, false
			},
			RemoveCalled: func(key []byte) {
				if string(key) == nilValKey {
					removedCalled = true
				}
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.ResetStatistics()

	assert.True(t, removedCalled)
}

func TestP2PQuotaBlacklistProcessor_ResetStatisticsShouldRemoveInvalidValueKey(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)

	invalidValKey := "invalid val key"
	removedCalled := false
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			KeysCalled: func() [][]byte {
				return [][]byte{[]byte(invalidValKey)}
			},
			PeekCalled: func(key []byte) (value interface{}, ok bool) {
				return "invalid value", true
			},
			RemoveCalled: func(key []byte) {
				if string(key) == invalidValKey {
					removedCalled = true
				}
			},
		},
		&mock.PeerBlackListHandlerStub{},
		thresholdNum,
		thresholdSize,
		2,
		time.Second,
		"",
		selfPid,
	)

	pbp.ResetStatistics()

	assert.True(t, removedCalled)
}

func TestP2PQuotaBlacklistProcessor_ResetStatisticsUnderNumFloodingSlotsShouldNotBlackList(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)
	numFloodingSlots := uint32(30)

	key := "key"
	removedCalled := false
	upsertCalled := false
	duration := time.Second * 3892
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			KeysCalled: func() [][]byte {
				return [][]byte{[]byte(key)}
			},
			PeekCalled: func(key []byte) (value interface{}, ok bool) {
				return numFloodingSlots - 2, true
			},
			RemoveCalled: func(key []byte) {
				removedCalled = true
			},
		},
		&mock.PeerBlackListHandlerStub{
			UpsertCalled: func(pid core.PeerID, span time.Duration) error {
				upsertCalled = true
				assert.Equal(t, duration, span)

				return nil
			},
		},
		thresholdNum,
		thresholdSize,
		numFloodingSlots,
		duration,
		"",
		selfPid,
	)

	pbp.ResetStatistics()

	assert.False(t, removedCalled)
	assert.False(t, upsertCalled)
}

func TestP2PQuotaBlacklistProcessor_ResetStatisticsOverNumFloodingSlotsShouldBlackList(t *testing.T) {
	t.Parallel()

	thresholdNum := uint32(10)
	thresholdSize := uint64(20)
	numFloodingSlots := uint32(30)

	key := "key"
	removedCalled := false
	upsertCalled := false
	duration := time.Second * 3892
	pbp, _ := blackList.NewP2PBlackListProcessor(
		&commonMock.CacherStub{
			KeysCalled: func() [][]byte {
				return [][]byte{[]byte(key)}
			},
			PeekCalled: func(key []byte) (value interface{}, ok bool) {
				return numFloodingSlots, true
			},
			RemoveCalled: func(key []byte) {
				removedCalled = true
			},
		},
		&mock.PeerBlackListHandlerStub{
			UpsertCalled: func(pid core.PeerID, span time.Duration) error {
				upsertCalled = true
				assert.Equal(t, duration, span)

				return nil
			},
		},
		thresholdNum,
		thresholdSize,
		numFloodingSlots,
		duration,
		"",
		selfPid,
	)

	pbp.ResetStatistics()

	assert.True(t, removedCalled)
	assert.True(t, upsertCalled)
}
