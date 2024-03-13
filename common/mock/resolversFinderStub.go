package mock

import (
	"github.com/klever-io/klever-go/data/retriever"
)

// ResolversFinderStub -
type ResolversFinderStub struct {
	ResolversContainerStub
	ChainResolverCalled func(baseTopic string) (retriever.Resolver, error)
	IterateCalled       func(handler func(key string, resolver retriever.Resolver) bool)
}

// ChainResolver -
func (rfs *ResolversFinderStub) ChainResolver(baseTopic string) (retriever.Resolver, error) {
	return rfs.ChainResolverCalled(baseTopic)
}

// Iterate -
func (rfs *ResolversFinderStub) Iterate(handler func(key string, resolver retriever.Resolver) bool) {
	if rfs.IterateCalled != nil {
		rfs.IterateCalled(handler)
	}
}
