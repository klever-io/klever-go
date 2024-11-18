package executorwrapper

import (
	"context"
	"time"

	"github.com/klever-io/klever-go/kvm/vmhost"
)

func FailAfterTimeout[K any](f func() K, timeout time.Duration) K {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	done := make(chan bool)
	var result K
	var panicToRaise any
	go func() {
		// go handles panics per go routine
		// if we are encapsulating it into a new go routine, we must handle panics
		defer func() {
			if r := recover(); r != nil {
				panicToRaise = r
				done <- true
			}
		}()
		result = f()
		done <- true
	}()
	select {
	case <-done:
		if panicToRaise != nil {
			panic(panicToRaise)
		}
		return result
	case <-ctx.Done():
		panic(vmhost.ErrExecutionFailedWithTimeout)
	}
}
