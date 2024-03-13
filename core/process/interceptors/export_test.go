package interceptors

import "github.com/klever-io/klever-go/core/process"

func (mdi *MultiDataInterceptor) Topic() string {
	return mdi.topic
}

func (mdi *MultiDataInterceptor) InterceptedDebugHandler() process.InterceptedDebugger {
	mdi.mutDebugHandler.RLock()
	defer mdi.mutDebugHandler.RUnlock()

	return mdi.debugHandler
}

func (sdi *SingleDataInterceptor) Topic() string {
	return sdi.topic
}

func (sdi *SingleDataInterceptor) InterceptedDebugHandler() process.InterceptedDebugger {
	sdi.mutDebugHandler.RLock()
	defer sdi.mutDebugHandler.RUnlock()

	return sdi.debugHandler
}
