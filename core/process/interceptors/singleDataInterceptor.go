package interceptors

import (
	"errors"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug/resolver"
)

// ArgSingleDataInterceptor is the argument for the single-data interceptor
type ArgSingleDataInterceptor struct {
	Topic            string
	DataFactory      process.InterceptedDataFactory
	Processor        process.InterceptorProcessor
	Throttler        process.InterceptorThrottler
	AntifloodHandler process.P2PAntifloodHandler
	WhiteListRequest process.WhiteListHandler
	CurrentPeerID    core.PeerID
}

// SingleDataInterceptor is used for intercepting packed multi data
type SingleDataInterceptor struct {
	*baseDataInterceptor
	factory          process.InterceptedDataFactory
	whiteListRequest process.WhiteListHandler
}

// NewSingleDataInterceptor hooks a new interceptor for single data
func NewSingleDataInterceptor(arg ArgSingleDataInterceptor) (*SingleDataInterceptor, error) {
	if len(arg.Topic) == 0 {
		return nil, process.ErrEmptyTopic
	}
	if check.IfNil(arg.DataFactory) {
		return nil, process.ErrNilInterceptedDataFactory
	}
	if check.IfNil(arg.Processor) {
		return nil, process.ErrNilInterceptedDataProcessor
	}
	if check.IfNil(arg.Throttler) {
		return nil, process.ErrNilInterceptorThrottler
	}
	if check.IfNil(arg.AntifloodHandler) {
		return nil, process.ErrNilAntifloodHandler
	}
	if check.IfNil(arg.WhiteListRequest) {
		return nil, process.ErrNilWhiteListHandler
	}
	if len(arg.CurrentPeerID) == 0 {
		return nil, process.ErrEmptyPeerID
	}

	singleDataIntercept := &SingleDataInterceptor{
		baseDataInterceptor: &baseDataInterceptor{
			throttler:        arg.Throttler,
			antifloodHandler: arg.AntifloodHandler,
			topic:            arg.Topic,
			currentPeerID:    arg.CurrentPeerID,
			processor:        arg.Processor,
			debugHandler:     resolver.NewDisabledInterceptorResolver(),
		},
		factory:          arg.DataFactory,
		whiteListRequest: arg.WhiteListRequest,
	}

	return singleDataIntercept, nil
}

// ProcessReceivedMessage is the callback func from the p2p.Messenger and will be called each time a new message was received
// (for the topic this validator was registered to)
func (sdi *SingleDataInterceptor) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	// Note: synchronization for bdi.debugHandler is handled inside
	// processDebugInterceptedData / receivedDebugInterceptedData themselves
	// (each takes mutDebugHandler.RLock() locally on the goroutine that
	// actually performs the read).
	//
	// An outer RLock here would NOT have prevented CWE-362: ProcessReceivedMessage
	// returns before the worker goroutine spawned at the bottom reads
	// bdi.debugHandler, so the synchronous-frame defer-RUnlock fires too early
	// to cover the race. Per-callsite RLocks are what the race detector requires.
	//
	// Secondary reason: stacking an outer RLock with the per-callsite RLocks
	// would also be unsafe under writer contention. Go's sync.RWMutex is not
	// recursion-aware — once a writer is queued, a nested RLock() blocks behind
	// it (writer preference) and self-deadlocks while the outer RLock is still
	// held (CWE-667).
	err := sdi.preProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	// Guard the throttler slot reserved by preProcessMessage so every synchronous
	// return below releases it exactly once. Ownership transfers to the async
	// goroutine on the success path. Mirrors the fix from GHSA-74m6-4hjp-7226 /
	// KLC-2348 and hardens this path against the same class of leak.
	ownershipTransferred := false
	defer func() {
		if !ownershipTransferred {
			sdi.throttler.EndProcessing()
		}
	}()

	interceptedData, err := sdi.factory.Create(message.Data())
	if err != nil {
		//this situation is so severe that we need to black list the peers
		reason := "can not create object from received bytes, topic " + sdi.topic + ", error " + err.Error()
		sdi.antifloodHandler.BlacklistPeer(message.Peer(), reason, core.InvalidMessageBlacklistDuration)
		sdi.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)

		return err
	}

	sdi.receivedDebugInterceptedData(interceptedData)

	err = interceptedData.CheckValidity()
	if err != nil {
		sdi.processDebugInterceptedData(interceptedData, err)

		isWrongVersion := errors.Is(err, process.ErrInvalidTransactionVersion) ||
			errors.Is(err, process.ErrInvalidChainID)
		if isWrongVersion {
			//this situation is so severe that we need to black list de peers
			reason := "wrong version of received intercepted data, topic " + sdi.topic + ", error " + err.Error()
			sdi.antifloodHandler.BlacklistPeer(message.Peer(), reason, core.InvalidMessageBlacklistDuration)
			sdi.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)
		}

		return err
	}

	errOriginator := sdi.antifloodHandler.IsOriginatorElectedForTopic(message.Peer(), sdi.topic)
	isWhiteListed := sdi.whiteListRequest.IsWhiteListed(interceptedData)
	if !isWhiteListed && errOriginator != nil {
		log.Trace("got message from peer on topic only for validators",
			"originator", p2p.PeerIDToShortString(message.Peer()), "topic",
			sdi.topic, "err", errOriginator)
		return errOriginator
	}

	ownershipTransferred = true
	go func() {
		defer func() {
			// Release the throttler slot before logging so the slot release
			// is unconditional even if logging itself panics on an
			// attacker-influenced panic value (CWE-400 defense-in-depth).
			r := recover()
			sdi.throttler.EndProcessing()
			if r != nil {
				log.Error("SingleDataInterceptor.ProcessReceivedMessage goroutine panicked", "panic", r)
			}
		}()
		sdi.processInterceptedData(interceptedData, message)
	}()

	return nil
}

// RegisterHandler registers a callback function to be notified on received data
func (sdi *SingleDataInterceptor) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {
	sdi.processor.RegisterHandler(handler)
}

// IsInterfaceNil returns true if there is no value under the interface
func (sdi *SingleDataInterceptor) IsInterfaceNil() bool {
	return sdi == nil
}
