package data

// TrieIteratorChannels holds the channels used while iterating over the leaves of a trie.
//
// LeavesChan receives every visited leaf and is closed once the iteration ends, no matter whether
// it ended by exhausting the trie or by failing part-way through. Err reports the error that cut
// the iteration short, and returns nil for a complete walk.
//
// Ranging over LeavesChan alone cannot tell a complete walk from one truncated by a read failure
// or by a cancelled context. Consume the iteration with ForEach, which drains the leaves and then
// reports such a failure.
type TrieIteratorChannels struct {
	LeavesChan chan KeyValueHolder

	done chan struct{}
	err  error
}

// NewTrieIteratorChannels returns the channels used to stream trie leaves to a consumer.
func NewTrieIteratorChannels(leavesBufferSize int) *TrieIteratorChannels {
	return &TrieIteratorChannels{
		LeavesChan: make(chan KeyValueHolder, leavesBufferSize),
		done:       make(chan struct{}),
	}
}

// NewCompletedTrieIteratorChannels returns channels already holding the given leaves and describing
// a complete iteration - with no leaves, an iteration over an empty trie. For callers that have every
// leaf up front and so never need a producer goroutine.
func NewCompletedTrieIteratorChannels(leaves ...KeyValueHolder) *TrieIteratorChannels {
	return newClosedTrieIteratorChannels(nil, leaves...)
}

// NewFailedTrieIteratorChannels returns channels describing an iteration that delivered the given
// leaves and then failed with err.
func NewFailedTrieIteratorChannels(err error, leaves ...KeyValueHolder) *TrieIteratorChannels {
	return newClosedTrieIteratorChannels(err, leaves...)
}

func newClosedTrieIteratorChannels(err error, leaves ...KeyValueHolder) *TrieIteratorChannels {
	channels := NewTrieIteratorChannels(len(leaves))
	for _, leaf := range leaves {
		channels.LeavesChan <- leaf
	}
	channels.Close(err)

	return channels
}

// Close ends the iteration, handing err (if any) to the consumer. It must be called exactly once,
// by the producer only, after the last leaf has been sent.
func (tic *TrieIteratorChannels) Close(err error) {
	tic.err = err
	close(tic.LeavesChan)
	close(tic.done)
}

// ForEach hands every leaf of the iteration to f, and returns the first error raised - by f or by the
// iteration itself. Returning nil from f skips the leaf and keeps going. LeavesChan is always drained
// to completion, so the producer goroutine is released even when f fails on the first leaf.
func (tic *TrieIteratorChannels) ForEach(f func(leaf KeyValueHolder) error) error {
	if tic == nil || tic.LeavesChan == nil {
		return ErrNilTrieIteratorChannels
	}

	var errForEach error
	for leaf := range tic.LeavesChan {
		if errForEach != nil {
			continue
		}

		errForEach = f(leaf)
	}

	if errForEach != nil {
		return errForEach
	}

	return tic.Err()
}

// Err returns the error that interrupted the iteration, or nil if every leaf was delivered. It waits
// for the producer to finish, so it must only be called once LeavesChan has been drained to
// completion - calling it mid-iteration deadlocks against the producer. Repeated calls return the
// same error.
func (tic *TrieIteratorChannels) Err() error {
	if tic == nil || tic.done == nil {
		return ErrNilTrieIteratorChannels
	}

	<-tic.done

	return tic.err
}
