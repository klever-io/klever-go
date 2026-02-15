package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventTypeStrict_ValidTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected EventType
	}{
		{"transaction", TRANSACTION},
		{"accounts", ACCOUNTS},
		{"blocks", BLOCKS},
		{"user_transaction", USER_TRANSACTION},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := NewEventTypeStrict(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewEventTypeStrict_InvalidType(t *testing.T) {
	result, err := NewEventTypeStrict("invalid_type")
	assert.ErrorIs(t, err, ErrUnknownEventType)
	assert.Equal(t, UNKNOWN, result)
}

func TestNewEventType_BackwardCompatibility(t *testing.T) {
	result := NewEventType("invalid_type")
	assert.Equal(t, UNKNOWN, result)

	result = NewEventType("transaction")
	assert.Equal(t, TRANSACTION, result)
}
