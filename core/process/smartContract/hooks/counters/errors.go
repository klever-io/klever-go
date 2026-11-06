package counters

import "errors"

// ErrInvalidMaximumValue signals that a per-transaction maximum is missing or zero
var ErrInvalidMaximumValue = errors.New("invalid per-transaction maximum")
