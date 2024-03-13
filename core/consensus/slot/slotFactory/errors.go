package slotFactory

import (
	"errors"
)

// ErrInvalidConsensusType signals that an invalid consensus type has been provided
var ErrInvalidConsensusType = errors.New("invalid consensus type")
