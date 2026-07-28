package leveldb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/klever-io/klever-go/storage"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var _ storage.Persister = (*SerialDB)(nil)

// SerialDB holds a pointer to the leveldb database and the path to where it is stored.
type SerialDB struct {
	*baseLevelDb
	path                    string
	maxBatchSize            int
	batchDelaySeconds       int
	sizeBatch               int
	batch                   storage.Batcher
	mutBatch                sync.RWMutex
	mutAccess               sync.RWMutex
	mutFlush                sync.Mutex
	dbAccess                chan serialQueryer
	cancel                  context.CancelFunc
	mutClosed               sync.Mutex
	closed                  bool
	testHookBeforeBatchSend func()
}

// NewSerialDB is a constructor for the leveldb persister
// It creates the files in the location given as parameter
func NewSerialDB(path string, batchDelaySeconds int, maxBatchSize int, maxOpenFiles int) (s *SerialDB, err error) {
	return newSerialDB(path, batchDelaySeconds, maxBatchSize, maxOpenFiles, nil)
}

// newSerialDB is the internal constructor allowing tests to inject testHookBeforeBatchSend
// before the background goroutines start, avoiding a data race on the field.
func newSerialDB(path string, batchDelaySeconds int, maxBatchSize int, maxOpenFiles int, testHookBeforeBatchSend func()) (s *SerialDB, err error) {
	err = os.MkdirAll(path, rwxOwner)
	if err != nil {
		return nil, err
	}

	if maxOpenFiles < 1 {
		return nil, storage.ErrInvalidNumOpenFiles
	}

	options := &opt.Options{
		// disable internal cache
		BlockCacheCapacity:     -1,
		OpenFilesCacheCapacity: maxOpenFiles,
	}

	db, err := openLevelDB(path, options)
	if err != nil {
		return nil, fmt.Errorf("%w for path %s", err, path)
	}

	bldb := &baseLevelDb{
		db: db,
	}

	ctx, cancel := context.WithCancel(context.Background())
	dbStore := &SerialDB{
		baseLevelDb:             bldb,
		path:                    path,
		maxBatchSize:            maxBatchSize,
		batchDelaySeconds:       batchDelaySeconds,
		sizeBatch:               0,
		dbAccess:                make(chan serialQueryer),
		cancel:                  cancel,
		closed:                  false,
		testHookBeforeBatchSend: testHookBeforeBatchSend,
	}

	dbStore.batch = NewBatch()

	go dbStore.batchTimeoutHandle(ctx)
	go dbStore.processLoop(ctx)

	runtime.SetFinalizer(dbStore, func(db *SerialDB) {
		_ = db.Close()
	})

	return dbStore, nil
}

func (s *SerialDB) batchTimeoutHandle(ctx context.Context) {
	for {
		select {
		case <-time.After(time.Duration(s.batchDelaySeconds) * time.Second):
			err := s.putBatch()
			if err != nil {
				log.Warn("leveldb serial putBatch", "error", err.Error())
				continue
			}
		case <-ctx.Done():
			log.Debug("closing the timed batch handler", "path", s.path)
			return
		}
	}
}

func (s *SerialDB) updateBatchWithIncrementLocked() error {
	s.mutBatch.Lock()
	s.sizeBatch++
	if s.sizeBatch < s.maxBatchSize {
		s.mutBatch.Unlock()
		return nil
	}
	s.mutBatch.Unlock()

	return s.putBatchLocked()
}

// Put adds the value to the (key, val) storage medium
func (s *SerialDB) Put(key, val []byte) error {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return storage.ErrSerialDBIsClosed
	}

	s.mutBatch.RLock()
	err := s.batch.Put(key, val)
	s.mutBatch.RUnlock()
	if err != nil {
		return err
	}

	return s.updateBatchWithIncrementLocked()
}

// Get returns the value associated to the key
func (s *SerialDB) Get(key []byte) ([]byte, error) {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return nil, storage.ErrSerialDBIsClosed
	}

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

// Has returns nil if the given key is present in the persistence medium
func (s *SerialDB) Has(key []byte) error {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return storage.ErrSerialDBIsClosed
	}

	s.mutBatch.RLock()
	data := s.batch.Get(key)
	s.mutBatch.RUnlock()

	if data != nil {
		if bytes.Equal(data, []byte(removed)) {
			return storage.ErrKeyNotFound
		}
		return nil
	}

	ch := make(chan error)
	req := &hasAct{
		key:     key,
		resChan: ch,
	}

	s.dbAccess <- req
	result := <-ch
	close(ch)

	return result
}

// Init initializes the storage medium and prepares it for usage
func (s *SerialDB) Init() error {
	// no special initialization needed
	return nil
}

// putBatch writes the Batch data into the database
func (s *SerialDB) putBatch() error {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return storage.ErrSerialDBIsClosed
	}

	return s.putBatchLocked()
}

// putBatchLocked swaps out the current batch and sends it for writing. mutFlush
// serializes the swap-and-send end-to-end so concurrent flushes (from Put/Remove
// triggering a size-based flush and the timer-based flush) can't interleave and
// reorder or drop batches, even though callers now only hold mutAccess.RLock().
func (s *SerialDB) putBatchLocked() error {
	s.mutFlush.Lock()
	defer s.mutFlush.Unlock()

	s.mutBatch.Lock()
	dbBatch, ok := s.batch.(*batch)
	if !ok {
		s.mutBatch.Unlock()
		return storage.ErrInvalidBatch
	}
	s.sizeBatch = 0
	s.batch = NewBatch()
	s.mutBatch.Unlock()

	if s.testHookBeforeBatchSend != nil {
		s.testHookBeforeBatchSend()
	}

	ch := make(chan error)
	req := &putBatchAct{
		batch:   dbBatch,
		resChan: ch,
	}

	s.dbAccess <- req
	result := <-ch
	close(ch)

	return result
}

func (s *SerialDB) isClosed() bool {
	s.mutClosed.Lock()
	isClosed := s.closed
	s.mutClosed.Unlock()

	return isClosed
}

// Close closes the files/resources associated to the storage medium
func (s *SerialDB) Close() error {
	s.mutAccess.Lock()
	defer s.mutAccess.Unlock()

	s.mutClosed.Lock()

	if s.closed {
		s.mutClosed.Unlock()
		return nil
	}

	s.closed = true
	s.mutClosed.Unlock()

	_ = s.putBatchLocked()

	s.cancel()

	return s.db.Close()
}

// Remove removes the data associated to the given key
func (s *SerialDB) Remove(key []byte) error {
	s.mutAccess.RLock()
	defer s.mutAccess.RUnlock()

	if s.isClosed() {
		return storage.ErrSerialDBIsClosed
	}

	s.mutBatch.Lock()
	_ = s.batch.Delete(key)
	s.mutBatch.Unlock()

	return s.updateBatchWithIncrementLocked()
}

// Destroy removes the storage medium stored data
func (s *SerialDB) Destroy() error {
	s.mutAccess.Lock()
	defer s.mutAccess.Unlock()

	s.mutBatch.Lock()
	s.batch.Reset()
	s.sizeBatch = 0
	s.mutBatch.Unlock()

	s.cancel()

	s.mutClosed.Lock()
	s.closed = true
	s.mutClosed.Unlock()

	err := s.db.Close()

	if err != nil {
		return err
	}

	err = os.RemoveAll(s.path)

	return err
}

// DestroyClosed removes the already closed storage medium stored data
func (s *SerialDB) DestroyClosed() error {
	err := os.RemoveAll(s.path)
	if err != nil {
		log.Error("error destroy closed", "error", err, "path", s.path)
	}
	return err
}

func (s *SerialDB) processLoop(ctx context.Context) {
	for {
		select {
		case queryer := <-s.dbAccess:
			queryer.request(s)
		case <-ctx.Done():
			log.Debug("closing the leveldb process loop")
			return
		}
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *SerialDB) IsInterfaceNil() bool {
	return s == nil
}
