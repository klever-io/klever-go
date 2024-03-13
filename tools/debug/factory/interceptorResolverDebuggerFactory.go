package factory

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/tools/debug/resolver"
)

// NewInterceptorResolverDebuggerFactory will instantiate an InterceptorResolverDebugHandler based on the provided config
func NewInterceptorResolverDebuggerFactory(config config.InterceptorResolverDebugConfig) (InterceptorResolverDebugHandler, error) {
	if !config.Enabled {
		return resolver.NewDisabledInterceptorResolver(), nil
	}

	return resolver.NewInterceptorResolver(config)
}
