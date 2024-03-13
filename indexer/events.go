package indexer

var EventQueue = make(chan Event)
var UseEventQueue bool

type Event struct {
	EvType  EventType
	Message interface{}
}

// EventType TODO: we can improve this to have subtyping
type EventType string

const (
	UNKNOWN          EventType = ""
	USER_TRANSACTION EventType = "user_transaction"
	ACCOUNTS         EventType = "accounts"
	BLOCKS           EventType = "blocks"
	TRANSACTION      EventType = "transaction"
)

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
