package containers

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/stretchr/testify/assert"
)

func createMockContainer(expectedKey string) *mock.ResolversContainerStub {
	return &mock.ResolversContainerStub{
		GetCalled: func(key string) (resolver retriever.Resolver, e error) {
			if key == expectedKey {
				return &mock.ResolverStub{}, nil
			}

			return nil, nil
		},
	}
}

//------- NewResolversFinder

func TestNewResolversFinder_NilContainerShouldErr(t *testing.T) {
	t.Parallel()

	rf, err := NewResolversFinder(nil)

	assert.Nil(t, rf)
	assert.Equal(t, common.ErrNilResolverContainer, err)
}

func TestNewResolversFinder_ShouldWork(t *testing.T) {
	t.Parallel()

	rf, err := NewResolversFinder(&mock.ResolversContainerStub{})

	assert.NotNil(t, rf)
	assert.Nil(t, err)
	assert.False(t, rf.IsInterfaceNil())
}

func TestResolversFinder_ChainResolver(t *testing.T) {
	baseTopic := "baseTopic"

	rf, _ := NewResolversFinder(
		createMockContainer(baseTopic),
	)

	resolver, _ := rf.ChainResolver(baseTopic)
	assert.NotNil(t, resolver)
}
