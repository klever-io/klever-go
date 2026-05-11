package processor

import (
	"runtime/debug"
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
)

var _ process.InterceptorProcessor = (*HdrInterceptorProcessor)(nil)

var log = logger.GetOrCreate("process/interceptors/processor")

// HdrInterceptorProcessor is the processor used when intercepting headers
// (shard headers, meta headers) structs which satisfy HeaderHandler interface.
type HdrInterceptorProcessor struct {
	headers            retriever.HeadersPool
	blackList          process.TimeCacher
	registeredHandlers []func(topic string, hash []byte, data interface{})
	mutHandlers        sync.RWMutex
}

// NewHdrInterceptorProcessor creates a new TxInterceptorProcessor instance
func NewHdrInterceptorProcessor(argument *ArgHdrInterceptorProcessor) (*HdrInterceptorProcessor, error) {
	if argument == nil {
		return nil, process.ErrNilArgumentStruct
	}
	if check.IfNil(argument.Headers) {
		return nil, common.ErrNilCacher
	}
	if check.IfNil(argument.BlockBlackList) {
		return nil, process.ErrNilBlackListCacher
	}

	return &HdrInterceptorProcessor{
		headers:            argument.Headers,
		blackList:          argument.BlockBlackList,
		registeredHandlers: make([]func(topic string, hash []byte, data interface{}), 0),
	}, nil
}

// Validate checks if the intercepted data can be processed
func (hip *HdrInterceptorProcessor) Validate(data process.InterceptedData, _ core.PeerID) error {
	interceptedHdr, ok := data.(process.HdrValidatorHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	hip.blackList.Sweep()
	isBlackListed := hip.blackList.Has(string(interceptedHdr.Hash()))
	if isBlackListed {
		return process.ErrHeaderIsBlackListed
	}

	return nil
}

// Save will save the received data into the headers cacher as hash<->[plain header structure]
// and in headersNonces as nonce<->hash
func (hip *HdrInterceptorProcessor) Save(data process.InterceptedData, _ core.PeerID, topic string) error {
	interceptedHdr, ok := data.(process.HdrValidatorHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	// Defer the InterceptedData accessor calls into the spawned goroutine so they
	// execute INSIDE notify()'s recover boundary. If we evaluated them here as
	// arguments to `go hip.notify(...)`, a panic in HeaderHandler() / Hash()
	// would surface on the caller stack — Save has no recover frame and the
	// process would crash (CWE-755).
	go func() {
		hip.notify(interceptedHdr, topic)
	}()

	hip.headers.AddHeader(interceptedHdr.Hash(), interceptedHdr.HeaderHandler())

	return nil
}

func (hip *HdrInterceptorProcessor) Notify(data process.InterceptedData, fromConnectedPeer core.PeerID, topic string) error {
	//
	return nil
}

// RegisterHandler registers a callback function to be notified of incoming headers
func (hip *HdrInterceptorProcessor) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {
	if handler == nil {
		return
	}

	hip.mutHandlers.Lock()
	hip.registeredHandlers = append(hip.registeredHandlers, handler)
	hip.mutHandlers.Unlock()
}

// IsInterfaceNil returns true if there is no value under the interface
func (hip *HdrInterceptorProcessor) IsInterfaceNil() bool {
	return hip == nil
}

func (hip *HdrInterceptorProcessor) notify(interceptedHdr process.HdrValidatorHandler, topic string) {
	// Resolve identity inside the recover boundary so a panic in either
	// accessor (HeaderHandler / Hash) is caught by the per-call recover
	// below rather than crashing the process (CWE-755).
	defer func() {
		if r := recover(); r != nil {
			log.Error("HdrInterceptorProcessor.notify panicked while resolving header",
				"topic", topic,
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}()
	header := interceptedHdr.HeaderHandler()
	hash := interceptedHdr.Hash()

	// Snapshot the handlers under the read-lock and release the lock BEFORE
	// invoking any user-supplied callback. Holding the read-lock across user
	// callbacks risks reentrant deadlocks (a handler that calls
	// RegisterHandler would block on Lock waiting for its own RLock to
	// release) and lock leaks if the callback panics (CWE-667 / CWE-833).
	hip.mutHandlers.RLock()
	snapshot := make([]func(topic string, hash []byte, data interface{}), len(hip.registeredHandlers))
	copy(snapshot, hip.registeredHandlers)
	hip.mutHandlers.RUnlock()

	// Per-handler recover (failure isolation): a panic in handler[i] must NOT
	// skip handlers[i+1..N-1]. Each invocation runs through invokeHandlerSafely
	// so it has its own recover boundary.
	for i, handler := range snapshot {
		hip.invokeHandlerSafely(handler, topic, hash, header, i)
	}
}

// invokeHandlerSafely calls a single registered handler under a panic-recover
// boundary. A panic in the handler is logged with full forensic context
// (topic, hash, handler index, panic value, stack) and absorbed so the caller
// can continue iterating over the rest of the snapshot.
func (hip *HdrInterceptorProcessor) invokeHandlerSafely(
	handler func(topic string, hash []byte, data interface{}),
	topic string,
	hash []byte,
	header data.HeaderHandler,
	handlerIndex int,
) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("HdrInterceptorProcessor.notify handler panicked",
				"topic", topic,
				"hash", hash,
				"handlerIndex", handlerIndex,
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}()
	handler(topic, hash, header)
}
