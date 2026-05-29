package resolvers

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// messageProcessor is used for basic message validity and parsing
type messageProcessor struct {
	marshalizer      marshal.Marshalizer
	antifloodHandler retriever.P2PAntifloodHandler
	throttler        retriever.ResolverThrottler
	topic            string
}

func (mp *messageProcessor) canProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if check.IfNil(message) {
		return common.ErrNilMessage
	}
	err := mp.antifloodHandler.CanProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return fmt.Errorf("%w on resolver topic %s", err, mp.topic)
	}
	err = mp.antifloodHandler.CanProcessMessagesOnTopic(fromConnectedPeer, mp.topic, 1, uint64(len(message.Data())), message.SeqNo())
	if err != nil {
		return fmt.Errorf("%w on resolver topic %s", err, mp.topic)
	}
	if !mp.throttler.CanProcess() {
		return fmt.Errorf("%w on resolver topic %s", common.ErrSystemBusy, mp.topic)
	}

	return nil
}

// parseReceivedMessage will transform the received p2p.Message in a RequestData object.
func (mp *messageProcessor) parseReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) (*retriever.RequestData, error) {
	rd := &retriever.RequestData{}
	err := rd.UnmarshalWith(mp.marshalizer, message)
	if err != nil {
		//this situation is so severe that we need to black list the peers
		log.Debug("messageProcessor: unmarshal failed on resolver topic",
			"topic", mp.topic,
			"originator", p2p.PeerIDToShortString(message.Peer()),
			"err", process.SanitizeBlacklistReason(err.Error()))
		mp.antifloodHandler.BlacklistPeer(message.Peer(), process.BlacklistReasonUnmarshalable, core.InvalidMessageBlacklistDuration)
		mp.antifloodHandler.BlacklistPeer(fromConnectedPeer, process.BlacklistReasonUnmarshalable, core.InvalidMessageBlacklistDuration)

		return nil, err
	}
	if rd.Value == nil {
		return nil, common.ErrNilValue
	}

	return rd, nil
}
