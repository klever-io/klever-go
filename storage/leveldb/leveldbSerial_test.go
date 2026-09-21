package leveldb

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/storage"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
)

func createSerialLevelDb(t *testing.T, path string, batchDelaySeconds int, maxBatchSize int, maxOpenFiles int) (p *SerialDB) {
	if path == "" {
		path = t.TempDir()
	}
	lvdb, err := NewSerialDB(path, batchDelaySeconds, maxBatchSize, maxOpenFiles)

	assert.Nil(t, err, "Failed creating leveldb database file")
	t.Cleanup(func() { _ = lvdb.Close() })
	return lvdb
}

func TestSerialDB_InitNoError(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Init()

	assert.Nil(t, err, "error initializing DB")
}

func TestSerialDB_PutNoError(t *testing.T) {
	key, val := []byte("key"), []byte("value")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Put(key, val)

	assert.Nil(t, err, "error saving in DB")
}

func TestSerialDB_GetErrorAfterPutBeforeTimeout(t *testing.T) {
	key, val := []byte("key"), []byte("value")
	ldb := createSerialLevelDb(t, "", 1, 100, 10)

	_ = ldb.Put(key, val)
	v, err := ldb.Get(key)

	assert.Equal(t, val, v)
	assert.Nil(t, err)
}

func TestSerialDB_GetErrorOnFail(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)
	_ = ldb.Destroy()

	v, err := ldb.Get([]byte("key"))
	assert.Nil(t, v)
	assert.NotNil(t, err)
}

func TestSerialDB_CallsNotBlockingAfterCloseOrDestroy(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)
	_ = ldb.Destroy()

	_, err := ldb.Get([]byte("key"))
	assert.Equal(t, storage.ErrSerialDBIsClosed, err)

	err = ldb.Has([]byte("key"))
	assert.Equal(t, storage.ErrSerialDBIsClosed, err)

	err = ldb.Remove([]byte("key"))
	assert.Equal(t, storage.ErrSerialDBIsClosed, err)

	err = ldb.Put([]byte("key"), []byte("val"))
	assert.Equal(t, storage.ErrSerialDBIsClosed, err)
}

func TestSerialDB_GetOKAfterPutWithTimeout(t *testing.T) {
	key, val := []byte("key"), []byte("value")
	ldb := createSerialLevelDb(t, "", 1, 100, 10)

	_ = ldb.Put(key, val)
	time.Sleep(time.Second * 3)
	v, err := ldb.Get(key)

	assert.Nil(t, err)
	assert.Equal(t, val, v)
}

func TestSerialDB_RemoveBeforeTimeoutOK(t *testing.T) {
	key, val := []byte("key"), []byte("value")
	ldb := createSerialLevelDb(t, "", 1, 100, 10)

	_ = ldb.Put(key, val)
	_ = ldb.Remove(key)
	time.Sleep(time.Second * 2)
	v, err := ldb.Get(key)

	assert.Nil(t, v)
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

func TestSerialDB_RemoveAfterTimeoutOK(t *testing.T) {
	key, val := []byte("key"), []byte("value")
	ldb := createSerialLevelDb(t, "", 1, 100, 10)

	_ = ldb.Put(key, val)
	time.Sleep(time.Second * 2)
	_ = ldb.Remove(key)
	v, err := ldb.Get(key)

	assert.Nil(t, v)
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

func TestSerialDB_GetPresent(t *testing.T) {
	key, val := []byte("key1"), []byte("value1")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	_ = ldb.Put(key, val)
	v, err := ldb.Get(key)

	assert.Nil(t, err, "error not expected, but got %s", err)
	assert.Equalf(t, v, val, "read:%s but expected: %s", v, val)
}

func TestSerialDB_GetNotPresent(t *testing.T) {
	key := []byte("key2")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	v, err := ldb.Get(key)

	assert.NotNil(t, err, "error expected but got nil, value %s", v)
}

func TestSerialDB_HasPresent(t *testing.T) {
	key, val := []byte("key3"), []byte("value3")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	_ = ldb.Put(key, val)
	err := ldb.Has(key)

	assert.Nil(t, err)
}

func TestSerialDB_HasNotPresent(t *testing.T) {
	key := []byte("key4")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Has(key)

	assert.NotNil(t, err)
	assert.Equal(t, err, storage.ErrKeyNotFound)
}

func TestSerialDB_RemovePresent(t *testing.T) {
	key, val := []byte("key5"), []byte("value5")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	_ = ldb.Put(key, val)
	_ = ldb.Remove(key)
	err := ldb.Has(key)

	assert.NotNil(t, err)
	assert.Equal(t, err, storage.ErrKeyNotFound)
}

func TestSerialDB_RemoveNotPresent(t *testing.T) {
	key := []byte("key6")
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Remove(key)

	assert.Nil(t, err, "no error expected but got %s", err)
}

func TestSerialDB_Close(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Close()

	assert.Nil(t, err, "no error expected but got %s", err)
}

func TestSerialDB_CloseTwice(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	_ = ldb.Close()
	err := ldb.Close()

	assert.Nil(t, err)
}

func TestSerialDB_Destroy(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 10, 1, 10)

	err := ldb.Destroy()

	assert.Nil(t, err, "no error expected but got %s", err)
}

func TestSerialDB_LatestWriteSurvivesOlderDetachedFlushAfterRestart(t *testing.T) {
	dbPath := t.TempDir()

	hookEntered := make(chan struct{})
	hookRelease := make(chan struct{})
	var hookTriggered atomic.Bool
	hook := func() {
		if hookTriggered.CompareAndSwap(false, true) {
			close(hookEntered)
			<-hookRelease
		}
	}

	db, err := newSerialDB(dbPath, 3600, 1, 10, hook)
	assert.Nil(t, err, "Failed creating leveldb database file")
	t.Cleanup(func() { _ = db.Close() })

	firstWriteDone := make(chan error, 1)
	go func() {
		firstWriteDone <- db.Put([]byte("slotKey"), []byte("oldValue"))
	}()

	select {
	case <-hookEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first detached batch handoff")
	}

	secondWriteDone := make(chan error, 1)
	go func() {
		secondWriteDone <- db.Put([]byte("slotKey"), []byte("newValue"))
	}()

	select {
	case err := <-secondWriteDone:
		t.Fatalf("second write completed before first detached flush release: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(hookRelease)
	assert.Nil(t, <-firstWriteDone)
	assert.Nil(t, <-secondWriteDone)
	assert.Nil(t, db.Close())

	reopened := createSerialLevelDb(t, dbPath, 3600, 1, 10)

	val, err := reopened.Get([]byte("slotKey"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("newValue"), val)
}

type flushPauseHook struct {
	armed       atomic.Bool
	triggered   atomic.Bool
	entered     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func newFlushPauseHook() *flushPauseHook {
	return &flushPauseHook{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (h *flushPauseHook) pause() {
	if !h.armed.Load() {
		return
	}
	if h.triggered.CompareAndSwap(false, true) {
		close(h.entered)
		<-h.release
	}
}

func (h *flushPauseHook) unpause() {
	h.releaseOnce.Do(func() { close(h.release) })
}

func (h *flushPauseHook) waitUntilPaused(t *testing.T) {
	t.Helper()

	select {
	case <-h.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the batch handoff to pause")
	}
}

func createSerialLevelDbWithPausedFlush(t *testing.T, maxBatchSize int) (*SerialDB, *flushPauseHook) {
	t.Helper()

	hook := newFlushPauseHook()
	lvdb, err := newSerialDB(t.TempDir(), 3600, maxBatchSize, 10, hook.pause)

	assert.Nil(t, err, "Failed creating leveldb database file")
	t.Cleanup(func() { _ = lvdb.Close() })
	return lvdb, hook
}

func startPausedFlush(t *testing.T, db *SerialDB, hook *flushPauseHook) chan error {
	t.Helper()

	hook.armed.Store(true)
	flushDone := make(chan error, 1)
	go func() {
		flushDone <- db.putBatch()
	}()
	t.Cleanup(hook.unpause)
	hook.waitUntilPaused(t)

	return flushDone
}

type pausedReadResult struct {
	val    []byte
	getErr error
	hasErr error
}

func readWhilePaused(t *testing.T, db *SerialDB, key []byte) pausedReadResult {
	t.Helper()

	getDone := make(chan pausedReadResult, 1)
	go func() {
		val, err := db.Get(key)
		getDone <- pausedReadResult{val: val, getErr: err}
	}()

	hasDone := make(chan error, 1)
	go func() {
		hasDone <- db.Has(key)
	}()

	var result pausedReadResult
	select {
	case result = <-getDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Get blocked during the batch handoff")
	}

	select {
	case result.hasErr = <-hasDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Has blocked during the batch handoff")
	}

	return result
}

func TestSerialDB_ReadsSeeLatestStateDuringFlushHandoff(t *testing.T) {
	key := []byte("handoffKey")

	cases := []struct {
		name        string
		before      func(t *testing.T, db *SerialDB)
		whilePaused func(t *testing.T, db *SerialDB)
		expectedVal []byte
		expectedErr error
	}{
		{
			name: "inserted key",
			before: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, []byte("insertedValue")))
			},
			expectedVal: []byte("insertedValue"),
		},
		{
			name: "updated value",
			before: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, []byte("oldValue")))
				assert.Nil(t, db.putBatch())
				assert.Nil(t, db.Put(key, []byte("newValue")))
			},
			expectedVal: []byte("newValue"),
		},
		{
			name: "removed key",
			before: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, []byte("value")))
				assert.Nil(t, db.putBatch())
				assert.Nil(t, db.Remove(key))
			},
			expectedErr: storage.ErrKeyNotFound,
		},
		{
			name: "value emptied while flushing",
			before: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, []byte("flushingValue")))
			},
			whilePaused: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, nil))
			},
			expectedVal: []byte{},
		},
		{
			name: "value emptied after durable write",
			before: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, []byte("durableValue")))
				assert.Nil(t, db.putBatch())
			},
			whilePaused: func(t *testing.T, db *SerialDB) {
				assert.Nil(t, db.Put(key, nil))
			},
			expectedVal: []byte{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, hook := createSerialLevelDbWithPausedFlush(t, 1000)
			tc.before(t, db)

			flushDone := startPausedFlush(t, db, hook)
			if tc.whilePaused != nil {
				tc.whilePaused(t, db)
			}
			read := readWhilePaused(t, db, key)

			hook.unpause()
			assert.Nil(t, <-flushDone)

			assert.Equal(t, tc.expectedVal, read.val)
			assert.ErrorIs(t, read.getErr, tc.expectedErr)
			assert.ErrorIs(t, read.hasErr, tc.expectedErr)
		})
	}
}

func TestSerialDB_ConcurrentFlushesPreserveReadVisibility(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 3600, 16, 10)

	const writers = 4
	const keysPerWriter = 250

	var wg sync.WaitGroup
	failures := make(chan string)
	var collected []string
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for failure := range failures {
			collected = append(collected, failure)
		}
	}()

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()

			for i := 0; i < keysPerWriter; i++ {
				key := []byte(fmt.Sprintf("writer-%d-key-%d", writer, i))

				if err := ldb.Put(key, []byte("first")); err != nil {
					failures <- fmt.Sprintf("put %s: %v", key, err)
					continue
				}
				if err := ldb.Has(key); err != nil {
					failures <- fmt.Sprintf("has after insert %s: %v", key, err)
				}
				val, err := ldb.Get(key)
				if err != nil {
					failures <- fmt.Sprintf("get after insert %s: %v", key, err)
				} else if !bytes.Equal(val, []byte("first")) {
					failures <- fmt.Sprintf("get after insert %s: got %q", key, val)
				}

				if err := ldb.Put(key, []byte("second")); err != nil {
					failures <- fmt.Sprintf("update %s: %v", key, err)
					continue
				}
				val, err = ldb.Get(key)
				if err != nil {
					failures <- fmt.Sprintf("get after update %s: %v", key, err)
				} else if !bytes.Equal(val, []byte("second")) {
					failures <- fmt.Sprintf("get after update %s: got %q", key, val)
				}

				if err := ldb.Remove(key); err != nil {
					failures <- fmt.Sprintf("remove %s: %v", key, err)
					continue
				}
				if _, err := ldb.Get(key); !errors.Is(err, storage.ErrKeyNotFound) {
					failures <- fmt.Sprintf("get after remove %s: %v", key, err)
				}
				if err := ldb.Has(key); !errors.Is(err, storage.ErrKeyNotFound) {
					failures <- fmt.Sprintf("has after remove %s: %v", key, err)
				}
			}
		}(w)
	}

	wg.Wait()
	close(failures)
	<-collectorDone

	for _, failure := range collected {
		t.Error(failure)
	}
}

func TestSerialDB_ReadsDoNotServeEntriesFromFailedFlush(t *testing.T) {
	ldb := createSerialLevelDb(t, "", 3600, 1000, 10)

	key := []byte("undurableKey")
	assert.Nil(t, ldb.Put(key, []byte("undurableValue")))

	assert.Nil(t, ldb.db.Close())

	assert.NotNil(t, ldb.putBatch())

	ldb.mutBatch.RLock()
	flushing := ldb.flushingBatch
	ldb.mutBatch.RUnlock()
	assert.Nil(t, flushing)

	val, err := ldb.Get(key)
	assert.Nil(t, val)
	assert.NotNil(t, err)
	assert.NotNil(t, ldb.Has(key))
}

// getHoldingFlushLock is the read path as it would look under the alternative
// remediation of acquiring mutFlush before inspecting the batch and holding it
// across the leveldb fallback. BenchmarkSerialDB_ReadsUnderFlushBudget_FlushLockHeld
// runs it over the same primitives so that design can be measured against the
// batch-visibility one Get actually uses.
func (s *SerialDB) getHoldingFlushLock(key []byte) ([]byte, error) {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return nil, storage.ErrSerialDBIsClosed
	}

	s.mutFlush.Lock()
	defer s.mutFlush.Unlock()

	s.mutBatch.RLock()
	data := s.batch.Get(key)
	s.mutBatch.RUnlock()

	if data != nil {
		if bytes.Equal(data, []byte(removed)) {
			return nil, storage.ErrKeyNotFound
		}
		return data, nil
	}

	ch := make(chan *pairResult)
	req := &getAct{
		key:     key,
		resChan: ch,
	}

	s.dbAccess <- req
	result := <-ch
	close(ch)

	if errors.Is(result.err, leveldb.ErrNotFound) {
		return nil, storage.ErrKeyNotFound
	}
	if result.err != nil {
		return nil, result.err
	}

	return result.value, nil
}

// The read-under-flush benchmarks run a fixed budget of Sync:true flushes with
// concurrent readers and report read-latency percentiles. Pinning the flush
// count is what makes the two read designs comparable: a free-running flusher
// is slowed by whichever variant reads faster, so its work would differ between
// runs. The reported flush-µs must be close between variants for the latency
// comparison to mean anything.
//
// The database must sit on a real block device. On hosts where the temp dir is
// tmpfs the fsync is free and the difference under measurement disappears; point
// TMPDIR at a disk-backed directory in that case.
const (
	benchFlushBudget = 1500
	benchReaders     = 4
	benchSampleEvery = 8
)

func runReadsUnderFlushBudget(b *testing.B, read func(*SerialDB, []byte) ([]byte, error)) (time.Duration, int, []time.Duration) {
	b.Helper()

	ldb, err := NewSerialDB(b.TempDir(), 3600, 1000, 10)
	if err != nil {
		b.Fatalf("failed creating leveldb database file: %v", err)
	}
	defer func() { _ = ldb.Close() }()

	key := []byte("benchmarkKey")
	val := []byte("benchmarkValue")
	if err = ldb.Put(key, val); err != nil {
		b.Fatalf("failed seeding the batch: %v", err)
	}

	var stop atomic.Bool
	var wg sync.WaitGroup
	readCounts := make([]int, benchReaders)
	sampled := make([][]time.Duration, benchReaders)

	for r := 0; r < benchReaders; r++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for !stop.Load() {
				start := time.Now()
				if _, err := read(ldb, key); err != nil {
					b.Errorf("read failed: %v", err)
					return
				}
				elapsed := time.Since(start)
				readCounts[idx]++
				if readCounts[idx]%benchSampleEvery == 0 {
					sampled[idx] = append(sampled[idx], elapsed)
				}
			}
		}(r)
	}

	flushStart := time.Now()
	for i := 0; i < benchFlushBudget; i++ {
		if err := ldb.Put(key, val); err != nil {
			b.Fatalf("put failed: %v", err)
		}
		if err := ldb.putBatch(); err != nil {
			b.Fatalf("flush failed: %v", err)
		}
	}
	flushWall := time.Since(flushStart)

	stop.Store(true)
	wg.Wait()

	totalReads := 0
	var samples []time.Duration
	for idx := range sampled {
		totalReads += readCounts[idx]
		samples = append(samples, sampled[idx]...)
	}

	return flushWall, totalReads, samples
}

func benchmarkReadsUnderFlushBudget(b *testing.B, read func(*SerialDB, []byte) ([]byte, error)) {
	var totalFlushWall time.Duration
	totalReads := 0
	var samples []time.Duration

	for i := 0; i < b.N; i++ {
		flushWall, reads, runSamples := runReadsUnderFlushBudget(b, read)
		totalFlushWall += flushWall
		totalReads += reads
		samples = append(samples, runSamples...)
	}
	b.StopTimer()

	if len(samples) == 0 {
		b.Fatal("no read samples were collected")
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	percentile := func(p float64) float64 {
		return float64(samples[int(float64(len(samples)-1)*p)].Nanoseconds())
	}

	totalFlushes := float64(b.N * benchFlushBudget)
	b.ReportMetric(float64(totalFlushWall.Microseconds())/totalFlushes, "flush-µs")
	b.ReportMetric(float64(totalReads)/totalFlushes, "reads/flush")
	b.ReportMetric(percentile(0.50), "read-p50-ns")
	b.ReportMetric(percentile(0.99), "read-p99-ns")
	b.ReportMetric(percentile(0.999), "read-p999-ns")
}

func BenchmarkSerialDB_ReadsUnderFlushBudget_BatchVisibility(b *testing.B) {
	benchmarkReadsUnderFlushBudget(b, (*SerialDB).Get)
}

func BenchmarkSerialDB_ReadsUnderFlushBudget_FlushLockHeld(b *testing.B) {
	benchmarkReadsUnderFlushBudget(b, (*SerialDB).getHoldingFlushLock)
}
