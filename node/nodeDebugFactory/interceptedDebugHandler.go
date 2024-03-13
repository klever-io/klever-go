package nodeDebugFactory

import (
	"fmt"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug/factory"
)

// InterceptorResolverDebugger is the contant string for the debugger
const InterceptorResolverDebugger = "interceptor resolver debugger"

// CreateInterceptedDebugHandler creates and applies an interceptor-resolver debug handler
func CreateInterceptedDebugHandler(
	node NodeWrapper,
	interceptors process.InterceptorsContainer,
	resolvers retriever.ResolversFinder,
	config config.InterceptorResolverDebugConfig,
) error {
	if check.IfNil(node) {
		return ErrNilNodeWrapper
	}
	if check.IfNil(interceptors) {
		return ErrNilInterceptorContainer
	}
	if check.IfNil(resolvers) {
		return ErrNilResolverContainer
	}

	debugHandler, err := factory.NewInterceptorResolverDebuggerFactory(config)
	if err != nil {
		return err
	}

	var errFound error
	interceptors.Iterate(func(key string, interceptor process.Interceptor) bool {
		err = interceptor.SetInterceptedDebugHandler(debugHandler)
		if err != nil {
			errFound = err
			return false
		}

		return true
	})
	if errFound != nil {
		return fmt.Errorf("%w while setting up debugger on interceptors", errFound)
	}

	resolvers.Iterate(func(key string, resolver retriever.Resolver) bool {
		err = resolver.SetResolverDebugHandler(debugHandler)
		if err != nil {
			errFound = err
			return false
		}

		return true
	})
	if errFound != nil {
		return fmt.Errorf("%w while setting up debugger on resolvers", errFound)
	}

	return node.AddQueryHandler(InterceptorResolverDebugger, debugHandler)
}
