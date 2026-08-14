package mock

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type streamMock struct {
	mutData         sync.Mutex
	buffStream      *bytes.Buffer
	pid             protocol.ID
	streamClosed    bool
	streamReset     bool
	readDeadline    time.Time
	writeDeadline   time.Time
	writeBlocked    bool
	canRead         bool
	conn            network.Conn
	id              string
	readDeadlineErr error
}

// NewStreamMock -
func NewStreamMock() *streamMock {
	return &streamMock{
		mutData:      sync.Mutex{},
		buffStream:   new(bytes.Buffer),
		streamClosed: false,
		canRead:      false,
	}
}

// Read - blocking read; like a real stream, pending data is delivered before EOF is reported
// (Close is graceful, Reset discards the buffer so subsequent reads fail immediately)
func (sm *streamMock) Read(p []byte) (int, error) {
	for {
		time.Sleep(time.Millisecond * 10)

		sm.mutData.Lock()
		// Honour the read deadline like a real stream: without this a mock read blocks forever
		// regardless of SetReadDeadline, so no mock-based test could ever observe a stalled reader
		// being reclaimed — the exact behaviour the deadline exists to provide.
		if !sm.readDeadline.IsZero() && time.Now().After(sm.readDeadline) {
			sm.mutData.Unlock()
			return 0, os.ErrDeadlineExceeded
		}

		if sm.canRead {
			n, err := sm.buffStream.Read(p)
			sm.canRead = false
			sm.mutData.Unlock()

			return n, err
		}

		if sm.streamClosed {
			sm.mutData.Unlock()
			return 0, io.EOF
		}
		sm.mutData.Unlock()
	}
}

// Write -
func (sm *streamMock) Write(p []byte) (int, error) {
	for {
		sm.mutData.Lock()

		if !sm.writeDeadline.IsZero() && time.Now().After(sm.writeDeadline) {
			sm.mutData.Unlock()
			return 0, os.ErrDeadlineExceeded
		}

		if !sm.writeBlocked {
			n, err := sm.buffStream.Write(p)
			if err == nil {
				sm.canRead = true
			}
			sm.mutData.Unlock()

			return n, err
		}

		sm.mutData.Unlock()
		time.Sleep(time.Millisecond * 10)
	}
}

// Close -
func (sm *streamMock) Close() error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.streamClosed = true
	return nil
}

// Reset - mirrors real stream semantics: pending and future reads unblock with an error
func (sm *streamMock) Reset() error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.buffStream.Reset()
	sm.canRead = false
	sm.streamClosed = true
	sm.streamReset = true
	return nil
}

// IsClosed -
func (sm *streamMock) IsClosed() bool {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	return sm.streamClosed
}

// IsReset reports whether the stream was reset (RST) as opposed to closed (FIN). Close leaves the
// peer's half usable; Reset does not, so tests asserting fail-closed behaviour must distinguish
// them or a downgrade from Reset to Close would go unnoticed.
func (sm *streamMock) IsReset() bool {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	return sm.streamReset
}

// SetDeadline -
func (sm *streamMock) SetDeadline(time.Time) error {
	return nil
}

// SetReadDeadline -
func (sm *streamMock) SetReadDeadline(t time.Time) error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	if sm.readDeadlineErr != nil {
		return sm.readDeadlineErr
	}

	sm.readDeadline = t
	return nil
}

// SetReadDeadlineError makes subsequent SetReadDeadline calls fail with err
func (sm *streamMock) SetReadDeadlineError(err error) {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.readDeadlineErr = err
}

// SetWriteDeadline -
func (sm *streamMock) SetWriteDeadline(t time.Time) error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.writeDeadline = t
	return nil
}

func (sm *streamMock) SetWriteBlocked(blocked bool) {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.writeBlocked = blocked
}

// Protocol -
func (sm *streamMock) Protocol() protocol.ID {
	return sm.pid
}

// SetProtocol -
func (sm *streamMock) SetProtocol(pid protocol.ID) error {
	sm.pid = pid

	return nil
}

// Stat -
func (sm *streamMock) Stat() network.Stats {
	return network.Stats{
		Direction: network.DirOutbound,
	}
}

// Conn -
func (sm *streamMock) Conn() network.Conn {
	return sm.conn
}

// SetConn -
func (sm *streamMock) SetConn(conn network.Conn) {
	sm.conn = conn
}

// ID -
func (sm *streamMock) ID() string {
	return sm.id
}

// SetID -
func (sm *streamMock) SetID(id string) {
	sm.id = id
}

// CloseWrite -
func (sm *streamMock) CloseWrite() error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.streamClosed = true
	return nil
}

// CloseRead -
func (sm *streamMock) CloseRead() error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.streamClosed = true
	return nil
}

// Scope -
func (sm *streamMock) Scope() network.StreamScope {
	return nil
}

// ResetWithError -
func (sm *streamMock) ResetWithError(errCode network.StreamErrorCode) error {
	sm.mutData.Lock()
	defer sm.mutData.Unlock()

	sm.buffStream.Reset()
	sm.canRead = false
	sm.streamClosed = true
	return nil
}
