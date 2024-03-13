package mock

import "github.com/klever-io/klever-go/core"

type topicMessageHandlerStub struct {
	*TopicHandlerStub
	*MessageHandlerStub
}

// NewTopicMessageHandlerStub -
func NewTopicMessageHandlerStub() *topicMessageHandlerStub {
	return &topicMessageHandlerStub{
		TopicHandlerStub:   &TopicHandlerStub{},
		MessageHandlerStub: &MessageHandlerStub{},
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *topicMessageHandlerStub) ID() core.PeerID {
	return s.MessageHandlerStub.ID()
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *topicMessageHandlerStub) IsInterfaceNil() bool {
	return s == nil
}
