package errors_test

import (
	"errors"
	"testing"

	apiErr "github.com/klever-io/klever-go/network/api/errors"
	"github.com/stretchr/testify/assert"
)

func TestAPIErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		msg      []interface{}
		expected string
	}{
		{
			name:     "No additional messages",
			err:      errors.New("base error"),
			msg:      []interface{}{},
			expected: "base error",
		},
		{
			name:     "Single string message",
			err:      errors.New("base error"),
			msg:      []interface{}{"additional info"},
			expected: "base error: additional info",
		},
		{
			name:     "Single error message",
			err:      errors.New("base error"),
			msg:      []interface{}{errors.New("additional error")},
			expected: "base error: additional error",
		},
		{
			name:     "Multiple messages (string and error)",
			err:      errors.New("base error"),
			msg:      []interface{}{"info 1", errors.New("info 2")},
			expected: "base error: info 1: info 2",
		},
		{
			name:     "Multiple non-string/error messages",
			err:      errors.New("base error"),
			msg:      []interface{}{123, true, nil},
			expected: "base error: 123: true: <nil>",
		},
		{
			name:     "Single integer message",
			err:      errors.New("base error"),
			msg:      []interface{}{42},
			expected: "base error: 42",
		},
		{
			name:     "Nil message",
			err:      errors.New("base error"),
			msg:      []interface{}{nil},
			expected: "base error: <nil>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := apiErr.APIErrorString(tc.err, tc.msg...)
			assert.Equal(t, tc.expected, result)
		})
	}
}
