package indexer

import (
	"errors"
	"sync/atomic"
	"time"
)

const eventQueueBufferSize = 1000

var EventQueue = make(chan Event, eventQueueBufferSize)
var UseEventQueue bool

// logsSubscriberChecker backs SetLogsSubscriberChecker/GetLogsSubscriberChecker. An
// atomic.Value instead of a plain package-level func var: it is written once (by the
// websocket hub during its construction) and cleared again on hub shutdown, but read on
// every block from the commit goroutine — a bare var would race between that write and
// those reads.
var logsSubscriberChecker atomic.Value // holds a `func() bool`, possibly nil

// SetLogsSubscriberChecker installs the function dispatchLogEvents consults before paying
// the full bech32/hex-encoding conversion cost on the block-commit goroutine, so a block
// with many SC events costs nothing extra when nobody would receive them. Call it with nil
// to unwire a hub that is shutting down — otherwise a later block still consults a stopped
// hub's stale state.
func SetLogsSubscriberChecker(checker func() bool) {
	logsSubscriberChecker.Store(&checker)
}

// GetLogsSubscriberChecker returns the currently installed checker, or nil if none is set
// (no hub wired yet, or this indexer package used outside the websocket feature).
func GetLogsSubscriberChecker() func() bool {
	stored, _ := logsSubscriberChecker.Load().(*func() bool)
	if stored == nil {
		return nil
	}
	return *stored
}

type Event struct {
	EvType  EventType
	Message interface{}
}

type EventType string

const (
	UNKNOWN           EventType = ""
	USER_TRANSACTIONS EventType = "user_transactions"
	ACCOUNTS          EventType = "accounts"
	BLOCKS            EventType = "blocks"
	TRANSACTIONS      EventType = "transactions"
	LOGS              EventType = "logs"
)

const dropLogIntervalSeconds = 10

var (
	droppedEventCount int64
	lastDropLogTime   int64
)

func trySendEvent(event Event) {
	select {
	case EventQueue <- event:
	default:
		atomic.AddInt64(&droppedEventCount, 1)
		now := time.Now().Unix()
		if last := atomic.LoadInt64(&lastDropLogTime); now-last >= dropLogIntervalSeconds {
			if atomic.CompareAndSwapInt64(&lastDropLogTime, last, now) {
				count := atomic.SwapInt64(&droppedEventCount, 0)
				log.Warn("event queue full, dropping events", "type", string(event.EvType), "droppedCount", count)
			}
		}
	}
}

var ErrUnknownEventType = errors.New("unknown event type")

func NewEventTypeStrict(evType string) (EventType, error) {
	switch evType {
	case "transactions":
		return TRANSACTIONS, nil
	case "accounts":
		return ACCOUNTS, nil
	case "blocks":
		return BLOCKS, nil
	case "user_transactions":
		return USER_TRANSACTIONS, nil
	case "logs":
		return LOGS, nil
	default:
		return UNKNOWN, ErrUnknownEventType
	}
}
