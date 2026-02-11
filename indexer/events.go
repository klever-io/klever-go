package indexer

import (
	"errors"
	"sync/atomic"
	"time"
)

const eventQueueBufferSize = 1000

var EventQueue = make(chan Event, eventQueueBufferSize)
var UseEventQueue bool

type Event struct {
	EvType  EventType
	Message interface{}
}

type EventType string

const (
	UNKNOWN          EventType = ""
	USER_TRANSACTION EventType = "user_transaction"
	ACCOUNTS         EventType = "accounts"
	BLOCKS           EventType = "blocks"
	TRANSACTION      EventType = "transaction"
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
		return TRANSACTION, nil
	case "accounts":
		return ACCOUNTS, nil
	case "blocks":
		return BLOCKS, nil
	case "user_transaction":
		return USER_TRANSACTION, nil
	default:
		return UNKNOWN, ErrUnknownEventType
	}
}

func NewEventType(evType string) EventType {
	switch evType {
	case "transactions":
		return TRANSACTION
	case "accounts":
		return ACCOUNTS
	case "blocks":
		return BLOCKS
	case "user_transaction":
		return USER_TRANSACTION
	default:
		return UNKNOWN
	}
}
