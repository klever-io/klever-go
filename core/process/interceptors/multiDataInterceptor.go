package interceptors

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug/resolver"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgMultiDataInterceptor is the argument for the multi-data interceptor
type ArgMultiDataInterceptor struct {
	Topic            string
	Marshalizer      marshal.Marshalizer
	DataFactory      process.InterceptedDataFactory
	Processor        process.InterceptorProcessor
	Throttler        process.InterceptorThrottler
	AntifloodHandler process.P2PAntifloodHandler
	WhiteListRequest process.WhiteListHandler
	CurrentPeerID    core.PeerID
}

// MultiDataInterceptor is used for intercepting packed multi data
type MultiDataInterceptor struct {
	*baseDataInterceptor
	marshalizer      marshal.Marshalizer
	factory          process.InterceptedDataFactory
	whiteListRequest process.WhiteListHandler
}

// NewMultiDataInterceptor hooks a new interceptor for packed multi data
func NewMultiDataInterceptor(arg ArgMultiDataInterceptor) (*MultiDataInterceptor, error) {
	if len(arg.Topic) == 0 {
		return nil, process.ErrEmptyTopic
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, process.ErrNilMarshalizer
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

	multiDataIntercept := &MultiDataInterceptor{
		baseDataInterceptor: &baseDataInterceptor{
			throttler:        arg.Throttler,
			antifloodHandler: arg.AntifloodHandler,
			topic:            arg.Topic,
			currentPeerID:    arg.CurrentPeerID,
			processor:        arg.Processor,
			debugHandler:     resolver.NewDisabledInterceptorResolver(),
		},
		marshalizer:      arg.Marshalizer,
		factory:          arg.DataFactory,
		whiteListRequest: arg.WhiteListRequest,
	}

	return multiDataIntercept, nil
}

// ProcessReceivedMessage is the callback func from the p2p.Messenger and will be called each time a new message was received
// (for the topic this validator was registered to)
func (mdi *MultiDataInterceptor) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	err := mdi.preProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	// Guard the throttler slot reserved by preProcessMessage so every synchronous
	// return below releases it exactly once. Ownership transfers to the async
	// goroutine on the success path. See GHSA-74m6-4hjp-7226 / KLC-2348.
	ownershipTransferred := false
	defer func() {
		if !ownershipTransferred {
			mdi.throttler.EndProcessing()
		}
	}()

	b := batch.Batch{}
	err = mdi.marshalizer.Unmarshal(&b, message.Data())
	if err != nil {
		//this situation is so severe that we need to black list de peers
		reason := "unmarshalable data got on topic " + mdi.topic
		mdi.antifloodHandler.BlacklistPeer(message.Peer(), reason, core.InvalidMessageBlacklistDuration)
		mdi.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)

		return err
	}
	if b.IsCompressed {
		err = b.Decompress(mdi.marshalizer)
		if err != nil {
			log.Error("MultiDataInterceptor.ProcessReceivedMessage", "err", err.Error())
			return err
		}
	}

	multiDataBuff := b.Data
	lenMultiData := len(multiDataBuff)
	if lenMultiData == 0 {
		return process.ErrNoDataInMessage
	}

	err = mdi.antifloodHandler.CanProcessMessagesOnTopic(
		fromConnectedPeer,
		mdi.topic,
		uint32(lenMultiData), // #nosec G115
		uint64(len(message.Data())),
		message.SeqNo(),
	)
	if err != nil {
		return err
	}

	listInterceptedData := make([]process.InterceptedData, len(multiDataBuff))
	errOriginator := mdi.antifloodHandler.IsOriginatorElectedForTopic(message.Peer(), mdi.topic)

	for index, dataBuff := range multiDataBuff {
		var interceptedData process.InterceptedData
		interceptedData, err = mdi.interceptedData(dataBuff, message.Peer(), fromConnectedPeer)
		listInterceptedData[index] = interceptedData
		if err != nil {
			return err
		}

		isWhiteListed := mdi.whiteListRequest.IsWhiteListed(interceptedData)
		if !isWhiteListed && errOriginator != nil {
			log.Trace("got message from peer on topic only for validators", "originator",
				p2p.PeerIDToShortString(message.Peer()),
				"topic", mdi.topic,
				"err", errOriginator)
			return errOriginator
		}
	}

	ownershipTransferred = true
	go func() {
		defer func() {
			// Release the throttler slot before logging so the slot release
			// is unconditional even if logging itself panics on an
			// attacker-influenced panic value (CWE-400 defense-in-depth).
			r := recover()
			mdi.throttler.EndProcessing()
			if r != nil {
				log.Error("MultiDataInterceptor.ProcessReceivedMessage goroutine panicked", "panic", r)
			}
		}()
		for _, interceptedData := range listInterceptedData {
			mdi.processInterceptedData(interceptedData, message)
		}
	}()

	return nil
}

func (mdi *MultiDataInterceptor) interceptedData(dataBuff []byte, originator core.PeerID, fromConnectedPeer core.PeerID) (process.InterceptedData, error) {
	interceptedData, err := mdi.factory.Create(dataBuff)
	if err != nil {
		//this situation is so severe that we need to black list de peers
		reason := "can not create object from received bytes, topic " + mdi.topic + ", error " + err.Error()
		mdi.antifloodHandler.BlacklistPeer(originator, reason, core.InvalidMessageBlacklistDuration)
		mdi.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)

		return nil, err
	}

	mdi.receivedDebugInterceptedData(interceptedData)

	err = interceptedData.CheckValidity()
	if err != nil {
		mdi.processDebugInterceptedData(interceptedData, err)

		isWrongVersion := err == process.ErrInvalidTransactionVersion || err == process.ErrInvalidChainID
		if isWrongVersion {
			//this situation is so severe that we need to black list de peers
			reason := "wrong version of received intercepted data, topic " + mdi.topic + ", error " + err.Error()
			mdi.antifloodHandler.BlacklistPeer(originator, reason, core.InvalidMessageBlacklistDuration)
			mdi.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)
		}

		return nil, err
	}

	return interceptedData, nil
}

// RegisterHandler registers a callback function to be notified on received data
func (mdi *MultiDataInterceptor) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {
	mdi.processor.RegisterHandler(handler)
}

// IsInterfaceNil returns true if there is no value under the interface
func (mdi *MultiDataInterceptor) IsInterfaceNil() bool {
	return mdi == nil
}
