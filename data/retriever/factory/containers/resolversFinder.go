package containers

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
)

var _ retriever.ResolversFinder = (*resolversFinder)(nil)

// resolversFinder is an implementation of process.ResolverContainer meant to be used
// wherever a resolver fetch is required
type resolversFinder struct {
	retriever.ResolversContainer
}

// NewResolversFinder creates a new resolversFinder object
func NewResolversFinder(container retriever.ResolversContainer) (*resolversFinder, error) {
	if container == nil || container.IsInterfaceNil() {
		return nil, common.ErrNilResolverContainer
	}

	return &resolversFinder{
		ResolversContainer: container,
	}, nil
}

// ChainResolver fetches the metachain Resolver starting from a baseTopic
// baseTopic will be one of the constants defined in factory.go: metaHeaderTopic, MetaPeerChangeTopic and so on
func (rf *resolversFinder) ChainResolver(baseTopic string) (retriever.Resolver, error) {
	return rf.Get(baseTopic)
}

// IsInterfaceNil returns true if underlying struct is nil
func (rf *resolversFinder) IsInterfaceNil() bool {
	return rf == nil || check.IfNil(rf.ResolversContainer)
}
