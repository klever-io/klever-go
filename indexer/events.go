package indexer

import (
	"errors"
	"sync/atomic"
	"time"
)

const eventQueueBufferSize = 1000

var EventQueue = make(chan Event, eventQueueBufferSize)
var UseEventQueue bool

// LogsSubscriberChecker, when set (by the websocket hub during its construction),
// reports whether dispatching a LOGS event would actually be delivered anywhere — an
// address-scoped LOGS subscriber, or a configured mirror endpoint. dispatchLogEvents
// consults it before paying the full bech32/hex-encoding conversion cost on the
// block-commit goroutine, so a block with many SC events costs nothing extra when nobody
// would receive them. nil (no hub wired yet, or this indexer package used outside the
// websocket feature) is treated as "yes, convert" so nothing is silently dropped absent a
// hub that could report otherwise.
var LogsSubscriberChecker func() bool

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

// NewEventType is the non-strict counterpart of NewEventTypeStrict, returning UNKNOWN
// instead of an error for an unrecognized type. Delegates to it rather than duplicating
// the switch, so the two can't silently drift out of sync (e.g. a new type added to one
// and forgotten in the other).
func NewEventType(evType string) EventType {
	t, _ := NewEventTypeStrict(evType)
	return t
}
