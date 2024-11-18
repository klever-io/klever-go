package executorwrapper

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/kvm/vmhost"

	"github.com/stretchr/testify/assert"
)

func TestFailAfterTimeout(t *testing.T) {
	tests := []struct {
		// prepare
		name          string
		returns       any
		waits         time.Duration
		timeout       time.Duration
		functionPanic error

		// assert
		expectedReturn any
		shouldReturn   bool
		shouldPanic    bool
	}{
		{
			name:    "Should preserve the return value",
			returns: 1,
			waits:   1 * time.Millisecond,
			timeout: 10 * time.Millisecond,

			expectedReturn: 1,
		},
		{
			name:    "Should throw a panic when the timeout is reached",
			returns: 123,
			waits:   20 * time.Millisecond,
			timeout: 10 * time.Millisecond,

			shouldPanic: true,
		},
		{
			name:          "Should throw a panic if the original function panics",
			returns:       123,
			waits:         1 * time.Millisecond,
			timeout:       10 * time.Millisecond,
			functionPanic: vmhost.ErrArgIndexOutOfRange,

			shouldPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hadPanic := false
			var result any = nil
			done := make(chan bool, 1)
			defer close(done)
			// we need to run this in a go routine to handle the panic per test
			go func() {
				defer func() {
					if r := recover(); r != nil {
						hadPanic = true
					}
					done <- true
				}()
				result = FailAfterTimeout(func() any {
					if tt.functionPanic != nil {
						panic(tt.functionPanic)
					}
					time.Sleep(tt.waits)
					return tt.returns
				}, tt.timeout)
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("test exceeded the time limit")
			}

			assert.Equal(t, tt.shouldPanic, hadPanic)
			assert.Equal(t, tt.expectedReturn, result)
		})
	}
}
