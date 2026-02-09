package indexer

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

func trySendEvent(event Event) {
	select {
	case EventQueue <- event:
	default:
		log.Warn("event queue full, dropping event", "type", string(event.EvType))
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
