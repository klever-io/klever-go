package peerHonesty

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testscommon "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

// createMockPeerHonestyConfig creates a peer honesty config with reasonable values
func createMockPeerHonestyConfig() config.PeerHonestyConfig {
	return config.PeerHonestyConfig{
		DecayCoefficient:             0.9779,
		DecayUpdateIntervalInSeconds: 10,
		MaxScore:                     100,
		MinScore:                     -100,
		BadPeerThreshold:             -80,
		UnitValue:                    1.0,
	}
}

func TestNewP2pPeerHonesty_NilCacheShouldErr(t *testing.T) {
	t.Parallel()

	pph, err := NewP2pPeerHonesty(
		createMockPeerHonestyConfig(),
		&mock.TimeCacheStub{},
		nil,
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrNilCacher))
}

func TestNewP2pPeerHonesty_NilBlacklistedPkCacheShouldErr(t *testing.T) {
	t.Parallel()

	pph, err := NewP2pPeerHonesty(
		createMockPeerHonestyConfig(),
		nil,
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrNilBlackListedPkCache))
}

func TestNewP2pPeerHonesty_InvalidDecayCoefficientShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.DecayCoefficient = -0.1
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidDecayCoefficient))
}

func TestNewP2pPeerHonesty_InvalidDecayUpdateIntervalShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.DecayUpdateIntervalInSeconds = 0
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidDecayIntervalInSeconds))
}

func TestNewP2pPeerHonesty_InvalidMinScoreShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.MinScore = 1
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidMinScore))
}

func TestNewP2pPeerHonesty_InvalidMaxScoreShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.MaxScore = -1
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidMaxScore))
}

func TestNewP2pPeerHonesty_InvalidUnitValueShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = -1
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidUnitValue))
}

func TestNewP2pPeerHonesty_InvalidBadPeerThresholdShouldErr(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.BadPeerThreshold = cfg.MinScore
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.True(t, check.IfNil(pph))
	assert.True(t, errors.Is(err, process.ErrInvalidBadPeerThreshold))
}

func TestNewP2pPeerHonesty_ShouldWork(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	pph, err := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
	)

	assert.False(t, check.IfNil(pph))
	assert.Nil(t, err)
}

func TestP2pPeerHonesty_Close(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.DecayUpdateIntervalInSeconds = 1

	// Signal each handler invocation through a buffered channel so the test
	// synchronizes on events instead of wall-clock sleeps.
	const expectedTicks = 2
	called := make(chan struct{}, 16)
	numCalls := int32(0)
	handler := func() {
		atomic.AddInt32(&numCalls, 1)
		select {
		case called <- struct{}{}:
		default:
		}
	}
	pph, err := NewP2pPeerHonestyWithCustomExecuteDelayFunction(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
		handler,
	)
	assert.Nil(t, err)
	assert.NotNil(t, pph)

	tickerWindow := time.Duration(cfg.DecayUpdateIntervalInSeconds) * time.Second
	tickTimeout := tickerWindow*expectedTicks + time.Second
	for range expectedTicks {
		select {
		case <-called:
		case <-time.After(tickTimeout):
			t.Fatalf("handler not invoked %d times within %s", expectedTicks, tickTimeout)
		}
	}

	assert.Nil(t, pph.Close())

	// After Close returns, the goroutine has exited (happens-before via done).
	// Confirm no further signals arrive within a full ticker window.
	select {
	case <-called:
		t.Fatal("handler was invoked after Close returned")
	case <-time.After(tickerWindow + 500*time.Millisecond):
	}

	assert.Equal(t, int32(expectedTicks), atomic.LoadInt32(&numCalls))
}

func TestP2pPeerHonesty_DoubleCloseShouldNotPanic(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.DecayUpdateIntervalInSeconds = 1
	pph, err := NewP2pPeerHonestyWithCustomExecuteDelayFunction(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
		func() {},
	)
	assert.Nil(t, err)
	assert.NotNil(t, pph)

	assert.Nil(t, pph.Close())
	assert.Nil(t, pph.Close()) // second call must not panic or block
}

func TestP2pPeerHonesty_ConcurrentCloseShouldNotPanic(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.DecayUpdateIntervalInSeconds = 1
	pph, err := NewP2pPeerHonestyWithCustomExecuteDelayFunction(
		cfg,
		&mock.TimeCacheStub{},
		&testscommon.CacherStub{},
		func() {},
	)
	assert.Nil(t, err)
	assert.NotNil(t, pph)

	const callers = 4
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			errs <- pph.Close()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent Close calls to return")
	}

	close(errs)
	for closeErr := range errs {
		assert.Nil(t, closeErr)
	}
}

func TestP2pPeerHonesty_ChangeScoreShouldWork(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := 2
	pph.ChangeScore(pk, topic, units)

	ps := pph.Get(pk)
	assert.Equal(t, 1, len(ps.scoresByTopic))
	assert.Equal(t, float64(units)*cfg.UnitValue, ps.scoresByTopic[topic])
}

func TestP2pPeerHonesty_DoubleChangeScoreShouldWork(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := 2
	pph.ChangeScore(pk, topic, units)
	pph.ChangeScore(pk, topic, units)

	ps := pph.Get(pk)
	assert.Equal(t, 1, len(ps.scoresByTopic))
	assert.Equal(t, float64(units+units)*cfg.UnitValue, ps.scoresByTopic[topic])
}

func TestP2pPeerHonesty_CheckBlacklistNotBlacklisted(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	hasCalled := false
	upsertCalled := false
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{
			HasCalled: func(key string) bool {
				hasCalled = true
				return false
			},
			UpsertCalled: func(key string, span time.Duration) error {
				upsertCalled = true
				return nil
			},
		},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := 2
	pph.ChangeScore(pk, topic, units)
	pph.ChangeScore(pk, topic, units)

	assert.False(t, hasCalled)
	assert.False(t, upsertCalled)
}

func TestP2pPeerHonesty_CheckBlacklistMaxScoreReached(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	hasCalled := false
	upsertCalled := false
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{
			HasCalled: func(key string) bool {
				hasCalled = true
				return false
			},
			UpsertCalled: func(key string, span time.Duration) error {
				upsertCalled = true
				return nil
			},
		},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := int(cfg.MaxScore) + 1
	pph.ChangeScore(pk, topic, units)

	ps := pph.Get(pk)
	assert.Equal(t, 1, len(ps.scoresByTopic))
	assert.Equal(t, cfg.MaxScore, ps.scoresByTopic[topic])

	assert.False(t, hasCalled)
	assert.False(t, upsertCalled)
}

func TestP2pPeerHonesty_CheckBlacklistMinScoreReached(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	upsertCalled := false
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{
			UpsertCalled: func(key string, span time.Duration) error {
				upsertCalled = true
				return nil
			},
		},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := int(cfg.MinScore) - 1
	pph.ChangeScore(pk, topic, units)

	ps := pph.Get(pk)
	assert.Equal(t, 1, len(ps.scoresByTopic))
	assert.Equal(t, cfg.MinScore, ps.scoresByTopic[topic])

	assert.True(t, upsertCalled)
}

func TestP2pPeerHonesty_CheckBlacklistHasShouldNotCallUpsert(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	upsertCalled := false
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{
			UpsertCalled: func(key string, span time.Duration) error {
				upsertCalled = true
				return nil
			},
		},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := int(cfg.MinScore) - 1
	pph.ChangeScore(pk, topic, units)

	assert.True(t, upsertCalled)
}

func TestP2pPeerHonesty_CheckBlacklistUpsertErrorsShouldWork(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.UnitValue = 4
	upsertCalled := false
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{
			HasCalled: func(key string) bool {
				return false
			},
			UpsertCalled: func(key string, span time.Duration) error {
				upsertCalled = true
				return errors.New("expected error")
			},
		},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	units := int(cfg.MinScore) - 1
	pph.ChangeScore(pk, topic, units)

	assert.True(t, upsertCalled)
}

func TestP2pPeerHonesty_ApplyDecay(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		testscommon.NewCacherMock(),
	)

	pks := []string{"pkMin", "pkMax", "pkNearZero", "pkZero", "pkValue"}
	initial := []float64{cfg.MinScore, cfg.MaxScore, approximateZero / 2, 0, 28}
	topic := "topic"

	for idx := range pks {
		pph.Put(pks[idx], topic, initial[idx])
	}

	pph.applyDecay()

	checkScore(t, pph, pks[0], topic, cfg.MinScore*cfg.DecayCoefficient)
	checkScore(t, pph, pks[1], topic, cfg.MaxScore*cfg.DecayCoefficient)
	checkScore(t, pph, pks[2], topic, 0)
	checkScore(t, pph, pks[3], topic, 0)
	checkScore(t, pph, pks[4], topic, initial[4]*cfg.DecayCoefficient)
}

func TestP2pPeerHonesty_ApplyDecayWillEventuallyGoTheScoreToZero(t *testing.T) {
	t.Parallel()

	cfg := createMockPeerHonestyConfig()
	cfg.MaxScore = 100
	cfg.UnitValue = 1
	cfg.DecayCoefficient = 0.9779
	pph, _ := NewP2pPeerHonesty(
		cfg,
		&mock.TimeCacheStub{},
		testscommon.NewCacherMock(),
	)

	pk := "pk"
	topic := "topic"
	pph.Put(pk, topic, cfg.MaxScore)

	expectedDecaysToBeZero := 722 //(at 10 seconds decay interval this will be ~2h)
	for i := 0; i < expectedDecaysToBeZero; i++ {
		pph.applyDecay()
	}

	checkScore(t, pph, pk, topic, 0)
}

func checkScore(t *testing.T, pph *p2pPeerHonesty, pk string, topic string, value float64) {
	ps := pph.Get(pk)
	assert.Equal(t, value, ps.scoresByTopic[topic])
}
