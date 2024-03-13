package resolvers

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ retriever.Resolver = (*TrieNodeResolver)(nil)

// maxBuffToSendTrieNodes represents max buffer size to send in bytes
var maxBuffToSendTrieNodes = uint64(1 << 18) //256KB

// ArgTrieNodeResolver is the argument structure used to create new TrieNodeResolver instance
type ArgTrieNodeResolver struct {
	SenderResolver   retriever.TopicResolverSender
	TrieDataGetter   retriever.TrieDataGetter
	Marshalizer      marshal.Marshalizer
	AntifloodHandler retriever.P2PAntifloodHandler
	Throttler        retriever.ResolverThrottler
}

// TrieNodeResolver is a wrapper over Resolver that is specialized in resolving trie node requests
type TrieNodeResolver struct {
	retriever.TopicResolverSender
	messageProcessor
	trieDataGetter retriever.TrieDataGetter
}

// NewTrieNodeResolver creates a new trie node resolver
func NewTrieNodeResolver(arg ArgTrieNodeResolver) (*TrieNodeResolver, error) {
	if check.IfNil(arg.SenderResolver) {
		return nil, common.ErrNilResolverSender
	}
	if check.IfNil(arg.TrieDataGetter) {
		return nil, common.ErrNilTrieDataGetter
	}
	if check.IfNil(arg.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(arg.AntifloodHandler) {
		return nil, common.ErrNilAntifloodHandler
	}
	if check.IfNil(arg.Throttler) {
		return nil, common.ErrNilThrottler
	}

	return &TrieNodeResolver{
		TopicResolverSender: arg.SenderResolver,
		trieDataGetter:      arg.TrieDataGetter,
		messageProcessor: messageProcessor{
			marshalizer:      arg.Marshalizer,
			antifloodHandler: arg.AntifloodHandler,
			topic:            arg.SenderResolver.RequestTopic(),
			throttler:        arg.Throttler,
		},
	}, nil
}

// ProcessReceivedMessage will be the callback func from the p2p.Messenger and will be called each time a new message was received
// (for the topic this validator was registered to, usually a request topic)
func (tnRes *TrieNodeResolver) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	err := tnRes.canProcessMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	tnRes.throttler.StartProcessing()
	defer tnRes.throttler.EndProcessing()

	rd, err := tnRes.parseReceivedMessage(message, fromConnectedPeer)
	if err != nil {
		return err
	}

	switch rd.Type {
	case retriever.RequestDataType_HashType:
		return tnRes.resolveOneHash(rd.Value, message)
	case retriever.RequestDataType_HashArrayType:
		return tnRes.resolveMultipleHashes(rd.Value, message)
	default:
		return common.ErrRequestTypeNotImplemented
	}
}

func (tnRes *TrieNodeResolver) resolveMultipleHashes(hashesBuff []byte, message p2p.MessageP2P) error {
	b := batch.Batch{}
	err := tnRes.marshalizer.Unmarshal(&b, hashesBuff)
	if err != nil {
		return err
	}
	if b.IsCompressed {
		err = b.Decompress(tnRes.marshalizer)
		if err != nil {
			log.Error("TrieNodeResolver.resolveMultipleHashes", "err", err.Error())
			return err
		}
	}
	hashes := b.Data

	remainingSpace := maxBuffToSendTrieNodes
	nodes := make([][]byte, 0, maxBuffToSendTrieNodes)
	var nextNodes [][]byte
	for _, hash := range hashes {
		nextNodes, remainingSpace, err = tnRes.getSubTrie(hash, remainingSpace)
		if err != nil {
			continue
		}

		nodes = append(nodes, nextNodes...)

		lenNextNodes := uint64(len(nextNodes))
		if lenNextNodes == 0 || remainingSpace == 0 {
			break
		}
	}

	return tnRes.sendResponse(nodes, message)
}

func (tnRes *TrieNodeResolver) resolveOneHash(hash []byte, message p2p.MessageP2P) error {
	nodes, _, err := tnRes.getSubTrie(hash, maxBuffToSendTrieNodes)
	if err != nil {
		return err
	}

	return tnRes.sendResponse(nodes, message)
}

func (tnRes *TrieNodeResolver) getSubTrie(hash []byte, remainingSpace uint64) ([][]byte, uint64, error) {
	serializedNodes, remainingSpace, err := tnRes.trieDataGetter.GetSerializedNodes(hash, remainingSpace)
	if err != nil {
		tnRes.ResolverDebugHandler().LogFailedToResolveData(
			tnRes.topic,
			hash,
			err,
		)

		return nil, remainingSpace, err
	}

	tnRes.ResolverDebugHandler().LogSucceededToResolveData(tnRes.topic, hash)

	return serializedNodes, remainingSpace, nil
}

func (tnRes *TrieNodeResolver) sendResponse(serializedNodes [][]byte, message p2p.MessageP2P) error {
	buff, err := tnRes.marshalizer.Marshal(&batch.Batch{Data: serializedNodes})
	if err != nil {
		return err
	}

	return tnRes.Send(buff, message.Peer())
}

// RequestDataFromHash requests trie nodes from other peers having input a trie node hash
func (tnRes *TrieNodeResolver) RequestDataFromHash(hash []byte, _ uint32) error {
	return tnRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashType,
			Value: hash,
		},
		[][]byte{hash},
	)
}

// RequestDataFromHashArray requests trie nodes from other peers having input multiple trie node hashes
func (tnRes *TrieNodeResolver) RequestDataFromHashArray(hashes [][]byte, _ uint32) error {
	b := &batch.Batch{
		Data: hashes,
	}
	buffHashes, err := tnRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return tnRes.SendOnRequestTopic(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashArrayType,
			Value: buffHashes,
		},
		hashes,
	)
}

// RequestDataFromHashArray requests a list of tx hashes from other peers
func (tnRes *TrieNodeResolver) RequestDataFromHashArrayTo(hashes [][]byte, epoch uint32, peer core.PeerID) error {
	b := &batch.Batch{
		Data: hashes,
	}
	buffHashes, err := tnRes.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	return tnRes.SendOnRequestTopicTo(
		&retriever.RequestData{
			Type:  retriever.RequestDataType_HashArrayType,
			Value: buffHashes,
			Epoch: epoch,
		},
		hashes,
		peer,
	)
}

// SetNumPeersToQuery will set the number of intra shard and cross shard number of peer to query
func (tnRes *TrieNodeResolver) SetNumPeersToQuery(intra int, cross int) {
	tnRes.TopicResolverSender.SetNumPeersToQuery(intra, cross)
}

// NumPeersToQuery will return the number of intra shard and cross shard number of peer to query
func (tnRes *TrieNodeResolver) NumPeersToQuery() (int, int) {
	return tnRes.TopicResolverSender.NumPeersToQuery()
}

// SetResolverDebugHandler will set a resolver debug handler
func (tnRes *TrieNodeResolver) SetResolverDebugHandler(handler retriever.ResolverDebugHandler) error {
	return tnRes.TopicResolverSender.SetResolverDebugHandler(handler)
}

// IsInterfaceNil returns true if there is no value under the interface
func (tnRes *TrieNodeResolver) IsInterfaceNil() bool {
	return tnRes == nil
}
