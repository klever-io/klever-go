package mock

import (
	"github.com/klever-io/klever-go/data/retriever"
)

// ResolversContainerStub -
type ResolversContainerStub struct {
	GetCalled          func(key string) (retriever.Resolver, error)
	AddCalled          func(key string, val retriever.Resolver) error
	ReplaceCalled      func(key string, val retriever.Resolver) error
	RemoveCalled       func(key string)
	LenCalled          func() int
	ResolverKeysCalled func() string
	IterateCalled      func(handler func(key string, resolver retriever.Resolver) bool)
}

// Get -
func (rcs *ResolversContainerStub) Get(key string) (retriever.Resolver, error) {
	return rcs.GetCalled(key)
}

// Add -
func (rcs *ResolversContainerStub) Add(key string, val retriever.Resolver) error {
	return rcs.AddCalled(key, val)
}

// AddMultiple -
func (rcs *ResolversContainerStub) AddMultiple(_ []string, _ []retriever.Resolver) error {
	return nil
}

// Replace -
func (rcs *ResolversContainerStub) Replace(key string, val retriever.Resolver) error {
	return rcs.ReplaceCalled(key, val)
}

// Remove -
func (rcs *ResolversContainerStub) Remove(key string) {
	rcs.RemoveCalled(key)
}

// Len -
func (rcs *ResolversContainerStub) Len() int {
	return rcs.LenCalled()
}

// ResolverKeys -
func (rcs *ResolversContainerStub) ResolverKeys() string {
	if rcs.ResolverKeysCalled != nil {
		return rcs.ResolverKeysCalled()
	}

	return ""
}

// Iterate -
func (rcs *ResolversContainerStub) Iterate(handler func(key string, resolver retriever.Resolver) bool) {
	if rcs.IterateCalled != nil {
		rcs.IterateCalled(handler)
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (rcs *ResolversContainerStub) IsInterfaceNil() bool {
	return rcs == nil
}
