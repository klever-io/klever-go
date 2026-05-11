package interceptors

import (
	"bytes"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
)

type baseDataInterceptor struct {
	throttler        process.InterceptorThrottler
	antifloodHandler process.P2PAntifloodHandler
	topic            string
	currentPeerID    core.PeerID
	processor        process.InterceptorProcessor
	mutDebugHandler  sync.RWMutex
	// debugHandler is rotatable at runtime via SetInterceptedDebugHandler.
	// Implementers of process.InterceptedDebugger MUST remain safe to call
	// after being swapped out: a worker goroutine spawned by
	// ProcessReceivedMessage may have snapshotted the previous pointer under
	// mutDebugHandler.RLock() and then released the lock before invoking
	// LogProcessedHashes / LogReceivedHashes. The contract is "callable until
	// GC", not "callable only while installed".
	debugHandler process.InterceptedDebugger
}

func (bdi *baseDataInterceptor) preProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if message == nil {
		return common.ErrNilMessage
	}
	if message.Data() == nil {
		return common.ErrNilDataToProcess
	}

	// Per-peer rate limits don't make sense for messages we delivered to ourselves,
	// so the antiflood layer is skipped when the sentinel matches. The local throttler,
	// however, is the node's protection against running out of goroutines / memory and
	// MUST run unconditionally — otherwise a remote peer that manages to spoof
	// (or a future code path that bypasses) the libp2p signature check could blow past
	// the local capacity ceiling. See GHSA-74m6-4hjp-7226 / KLC-2356.
	if !bdi.isMessageFromSelfToSelf(fromConnectedPeer, message) {
		err := bdi.antifloodHandler.CanProcessMessage(message, fromConnectedPeer)
		if err != nil {
			return err
		}
		err = bdi.antifloodHandler.CanProcessMessagesOnTopic(fromConnectedPeer, bdi.topic, 1, uint64(len(message.Data())), message.SeqNo())
		if err != nil {
			return err
		}
	}

	if !bdi.throttler.CanProcess() {
		return common.ErrSystemBusy
	}

	bdi.throttler.StartProcessing()
	return nil
}

// isMessageFromSelfToSelf returns true when the inbound message looks like one this
// node broadcast to itself. The byte-equality check is a sentinel, not a cryptographic
// verification — it relies on libp2p's pubsub signature policy
// (pubsub.WithMessageSignaturePolicy default StrictSign, set in
// network/p2p/libp2p/netMessenger.go via withMessageSigning=true) to ensure that any
// message reaching this point has already had Signature() cryptographically verified
// against From() and the sender's peer ID. If withMessageSigning is ever flipped off,
// or a non-pubsub path delivers messages here, this sentinel becomes a spoofable
// authentication-bypass vector — only the unconditional throttler.CanProcess() call
// in preProcessMessage prevents it from being a full anti-flood bypass. See
// GHSA-74m6-4hjp-7226 / KLC-2356 (CWE-290 / CWE-693).
func (bdi *baseDataInterceptor) isMessageFromSelfToSelf(fromConnectedPeer core.PeerID, message p2p.MessageP2P) bool {
	return bytes.Equal(message.Signature(), message.From()) &&
		bytes.Equal(message.From(), bdi.currentPeerID.Bytes()) &&
		fromConnectedPeer == bdi.currentPeerID
}

func (bdi *baseDataInterceptor) processInterceptedData(data process.InterceptedData, msg p2p.MessageP2P) {
	err := bdi.processor.Validate(data, msg.Peer())
	if err != nil {
		log.Trace("intercepted data is not valid",
			"hash", data.Hash(),
			"type", data.Type(),
			"pid", p2p.MessageOriginatorPid(msg),
			"seq no", p2p.MessageOriginatorSeq(msg),
			"data", data.String(),
			"error", err.Error(),
		)
		bdi.processDebugInterceptedData(data, err)

		return
	}

	err = bdi.processor.Save(data, msg.Peer(), bdi.topic)
	if err != nil {
		log.Trace("intercepted data can not be processed",
			"hash", data.Hash(),
			"type", data.Type(),
			"pid", p2p.MessageOriginatorPid(msg),
			"seq no", p2p.MessageOriginatorSeq(msg),
			"data", data.String(),
			"error", err.Error(),
		)
		bdi.processDebugInterceptedData(data, err)

		return
	}

	log.Trace("intercepted data is processed",
		"hash", data.Hash(),
		"type", data.Type(),
		"pid", p2p.MessageOriginatorPid(msg),
		"seq no", p2p.MessageOriginatorSeq(msg),
		"data", data.String(),
	)
	// Pass nil explicitly: the success branch is reached only after Save
	// returned no error, and reusing the loop-variable err here previously
	// risked passing a stale non-nil if the surrounding code is ever
	// refactored (CWE-252).
	bdi.processDebugInterceptedData(data, nil)
}

func (bdi *baseDataInterceptor) processDebugInterceptedData(interceptedData process.InterceptedData, err error) {
	identifiers := interceptedData.Identifiers()
	// Read the debugHandler under the same lock that SetInterceptedDebugHandler
	// uses to write it. Without this guard, the worker goroutine spawned by
	// ProcessReceivedMessage would race with concurrent runtime swaps of the
	// debug handler (CWE-362).
	bdi.mutDebugHandler.RLock()
	debugHandler := bdi.debugHandler
	bdi.mutDebugHandler.RUnlock()
	debugHandler.LogProcessedHashes(bdi.topic, identifiers, err)
}

func (bdi *baseDataInterceptor) receivedDebugInterceptedData(interceptedData process.InterceptedData) {
	identifiers := interceptedData.Identifiers()
	// Same locking discipline as processDebugInterceptedData (CWE-362).
	bdi.mutDebugHandler.RLock()
	debugHandler := bdi.debugHandler
	bdi.mutDebugHandler.RUnlock()
	debugHandler.LogReceivedHashes(bdi.topic, identifiers)
}

// SetInterceptedDebugHandler will set a new intercepted debug handler
func (bdi *baseDataInterceptor) SetInterceptedDebugHandler(handler process.InterceptedDebugger) error {
	if check.IfNil(handler) {
		return process.ErrNilDebugger
	}

	bdi.mutDebugHandler.Lock()
	bdi.debugHandler = handler
	bdi.mutDebugHandler.Unlock()

	return nil
}
