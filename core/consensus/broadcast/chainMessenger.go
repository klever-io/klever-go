package broadcast

import (
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("consensus/broadcast")

var _ consensus.BroadcastMessenger = (*chainMessenger)(nil)

type chainMessenger struct {
	marshalizer          marshal.Marshalizer
	hasher               hashing.Hasher
	messenger            consensus.P2PMessenger
	privateKey           crypto.PrivateKey
	peerSignatureHandler crypto.PeerSignatureHandler
}

// ChainMessengerArgs holds the arguments for creating a chainMessenger instance
type ChainMessengerArgs struct {
	Marshalizer                marshal.Marshalizer
	Hasher                     hashing.Hasher
	Messenger                  consensus.P2PMessenger
	PrivateKey                 crypto.PrivateKey
	PeerSignatureHandler       crypto.PeerSignatureHandler
	HeadersSubscriber          consensus.HeadersPoolSubscriber
	InterceptorsContainer      process.InterceptorsContainer
	MaxDelayCacheSize          uint32
	MaxValidatorDelayCacheSize uint32
}

// NewChainMessenger creates a new chainMessenger object
func NewChainMessenger(
	args ChainMessengerArgs,
) (*chainMessenger, error) {

	err := checkMetaChainNilParameters(args)
	if err != nil {
		return nil, err
	}

	cm := &chainMessenger{
		marshalizer:          args.Marshalizer,
		hasher:               args.Hasher,
		messenger:            args.Messenger,
		privateKey:           args.PrivateKey,
		peerSignatureHandler: args.PeerSignatureHandler,
	}

	return cm, nil
}

func checkMetaChainNilParameters(
	args ChainMessengerArgs,
) error {
	if check.IfNil(args.Marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(args.Messenger) {
		return common.ErrNilMessenger
	}
	if check.IfNil(args.PrivateKey) {
		return crypto.ErrNilPrivateKey
	}
	if check.IfNil(args.PeerSignatureHandler) {
		return common.ErrNilPeerSignatureHandler
	}
	if check.IfNil(args.InterceptorsContainer) {
		return common.ErrNilInterceptorsContainer
	}
	if check.IfNil(args.HeadersSubscriber) {
		return common.ErrNilHeadersSubscriber
	}
	if args.MaxDelayCacheSize == 0 || args.MaxValidatorDelayCacheSize == 0 {
		return common.ErrInvalidCacheSize
	}

	return nil
}

// BroadcastBlock will send on metachain blocks topic the header
func (cm *chainMessenger) BroadcastBlock(blck data.HeaderHandler) error {

	if check.IfNil(blck) {
		return common.ErrNilHeader
	}

	b := blck.(*block.Block)
	msgBlockBody, err := cm.marshalizer.Marshal(b)
	if err != nil {
		return err
	}

	go cm.messenger.Broadcast(common.BlocksTopic, msgBlockBody)

	return nil
}

// BroadcastTransactions will send on transaction topic the transactions
func (cm *chainMessenger) BroadcastTransactions(transactions [][]byte) error {
	txs := len(transactions)

	if txs == 0 {
		return nil
	}

	dataPacker, err := partitioning.NewSimpleDataPacker(cm.marshalizer)
	if err != nil {
		return err
	}

	// forward txs to the destination shards in packets
	packets, err := dataPacker.PackDataInChunks(transactions, core.MaxBulkTransactionSize)
	if err != nil {
		return err
	}

	for _, buff := range packets {
		go cm.messenger.Broadcast(common.TransactionTopic, buff)
	}

	log.Debug("commonMessenger.BroadcastTransactions",
		"num txs", txs,
	)

	return nil
}

// BroadcastBlockAndTransactions will send on metachain blocks topic the header and txsBuff
func (cm *chainMessenger) BroadcastBlockAndTransactions(blockBuff []byte, txsBuff [][]byte) error {
	if len(blockBuff) == 0 {
		return common.ErrNilHeader
	}

	go cm.messenger.Broadcast(common.BlocksTopic, blockBuff)

	time.Sleep(core.ExtraDelayBetweenBroadcastMbsAndTxs)

	err := cm.BroadcastTransactions(txsBuff)
	if err != nil {
		log.Warn("chainMessenger.BroadcastBlockAndTransactions", "error", err.Error())
	}

	return nil
}

// BroadcastHeader will send on metachain blocks topic the header
func (cm *chainMessenger) BroadcastHeader(header data.HeaderHandler) error {
	if check.IfNil(header) {
		return common.ErrNilHeader
	}

	msgHeader, err := cm.marshalizer.Marshal(header)
	if err != nil {
		return err
	}

	go cm.messenger.Broadcast(common.BlocksTopic, msgHeader)

	return nil
}

// BroadcastConsensusMessage will send on consensus topic the consensus message
func (cm *chainMessenger) BroadcastConsensusMessage(message *consensus.Message) error {
	signature, err := cm.peerSignatureHandler.GetPeerSignature(cm.privateKey, message.OriginatorPid)
	if err != nil {
		return err
	}

	message.Signature = signature

	buff, err := cm.marshalizer.Marshal(message)
	if err != nil {
		return err
	}

	go cm.messenger.Broadcast(common.ConsensusTopic, buff)

	return nil
}

// BroadcastBlockDataLeader broadcasts the block data as consensus group leader
func (cm *chainMessenger) BroadcastBlockDataLeader(
	header data.HeaderHandler,
	blockBuff []byte,
	transactions [][]byte,
) error {
	if check.IfNil(header) {
		return common.ErrNilHeader
	}
	// delayedBlockBroadcaster

	// go cm.BroadcastBlockAndTransactions(blockBuff, transactions)
	go cm.BroadcastTransactions(transactions)

	return nil
}

// Close closes all the started infinite looping goroutines and subcomponents
func (cm *chainMessenger) Close() {

}

// IsInterfaceNil returns true if there is no value under the interface
func (cm *chainMessenger) IsInterfaceNil() bool {
	return cm == nil
}
