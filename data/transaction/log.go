package transaction

// GetLogEvents returns the interface for the underlying events of the log structure
func (l *Log) GetLogEvents() []EventHandler {
	events := make([]EventHandler, len(l.Events))
	for i, e := range l.Events {
		events[i] = e
	}
	return events
}

// IsInterfaceNil verifies if underlying object is nil
func (l *Log) IsInterfaceNil() bool {
	return l == nil
}

// IsInterfaceNil verifies if underlying object is nil
func (e *Event) IsInterfaceNil() bool {
	return e == nil
}
