package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/network/p2p"
)

// SlotWorkerMock -
type SlotWorkerMock struct {
	AddReceivedMessageCallCalled func(
		messageType consensus.MessageType,
		receivedMessageCall func(cnsDta *consensus.Message) bool,
	)
	AddReceivedHeaderHandlerCalled         func(handler func(data.HeaderHandler))
	RemoveAllReceivedMessagesCallsCalled   func()
	ProcessReceivedMessageCalled           func(message p2p.MessageP2P) error
	SendConsensusMessageCalled             func(cnsDta *consensus.Message) bool
	ExtendCalled                           func(subslotId int)
	GetConsensusStateChangedChannelsCalled func() chan bool
	GetBroadcastBlockCalled                func(data.HeaderHandler) error
	GetBroadcastHeaderCalled               func(data.HeaderHandler) error
	ExecuteStoredMessagesCalled            func()
	DisplayStatisticsCalled                func()
	ReceivedHeaderCalled                   func(headerHandler data.HeaderHandler, headerHash []byte)
	SetAppStatusHandlerCalled              func(ash core.AppStatusHandler) error
	ResetConsensusMessagesCalled           func()
}

// AddReceivedMessageCall -
func (sposWorkerMock *SlotWorkerMock) AddReceivedMessageCall(messageType consensus.MessageType,
	receivedMessageCall func(cnsDta *consensus.Message) bool) {
	sposWorkerMock.AddReceivedMessageCallCalled(messageType, receivedMessageCall)
}

// AddReceivedHeaderHandler -
func (sposWorkerMock *SlotWorkerMock) AddReceivedHeaderHandler(handler func(data.HeaderHandler)) {
	if sposWorkerMock.AddReceivedHeaderHandlerCalled != nil {
		sposWorkerMock.AddReceivedHeaderHandlerCalled(handler)
	}
}

// RemoveAllReceivedMessagesCalls -
func (sposWorkerMock *SlotWorkerMock) RemoveAllReceivedMessagesCalls() {
	sposWorkerMock.RemoveAllReceivedMessagesCallsCalled()
}

// ProcessReceivedMessage -
func (sposWorkerMock *SlotWorkerMock) ProcessReceivedMessage(message p2p.MessageP2P, _ core.PeerID) error {
	return sposWorkerMock.ProcessReceivedMessageCalled(message)
}

// SendConsensusMessage -
func (sposWorkerMock *SlotWorkerMock) SendConsensusMessage(cnsDta *consensus.Message) bool {
	return sposWorkerMock.SendConsensusMessageCalled(cnsDta)
}

// Extend -
func (sposWorkerMock *SlotWorkerMock) Extend(subslotId int) {
	sposWorkerMock.ExtendCalled(subslotId)
}

// GetConsensusStateChangedChannel -
func (sposWorkerMock *SlotWorkerMock) GetConsensusStateChangedChannel() chan bool {
	return sposWorkerMock.GetConsensusStateChangedChannelsCalled()
}

// BroadcastBlock -
func (sposWorkerMock *SlotWorkerMock) BroadcastBlock(header data.HeaderHandler) error {
	return sposWorkerMock.GetBroadcastBlockCalled(header)
}

// ExecuteStoredMessages -
func (sposWorkerMock *SlotWorkerMock) ExecuteStoredMessages() {
	sposWorkerMock.ExecuteStoredMessagesCalled()
}

// DisplayStatistics -
func (sposWorkerMock *SlotWorkerMock) DisplayStatistics() {
	if sposWorkerMock.DisplayStatisticsCalled != nil {
		sposWorkerMock.DisplayStatisticsCalled()
	}
}

// ReceivedHeader -
func (sposWorkerMock *SlotWorkerMock) ReceivedHeader(headerHandler data.HeaderHandler, headerHash []byte) {
	if sposWorkerMock.ReceivedHeaderCalled != nil {
		sposWorkerMock.ReceivedHeaderCalled(headerHandler, headerHash)
	}
}

// SetAppStatusHandler -
func (sposWorkerMock *SlotWorkerMock) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if sposWorkerMock.SetAppStatusHandlerCalled != nil {
		return sposWorkerMock.SetAppStatusHandlerCalled(ash)
	}

	return nil
}

// Close -
func (sposWorkerMock *SlotWorkerMock) Close() error {
	return nil
}

// StartWorking -
func (sposWorkerMock *SlotWorkerMock) StartWorking() {
}

// ResetConsensusMessages -
func (sposWorkerMock *SlotWorkerMock) ResetConsensusMessages() {
	if sposWorkerMock.ResetConsensusMessagesCalled != nil {
		sposWorkerMock.ResetConsensusMessagesCalled()
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (sposWorkerMock *SlotWorkerMock) IsInterfaceNil() bool {
	return sposWorkerMock == nil
}
