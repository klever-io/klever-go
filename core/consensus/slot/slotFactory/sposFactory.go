package slotFactory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// GetSubslotsFactory returns a subslots factory depending of the given parameter
func GetSubslotsFactory(
	consensusDataContainer slot.ConsensusCoreHandler,
	consensusState *slot.ConsensusState,
	worker slot.WorkerHandler,
	consensusType string,
	appStatusHandler core.AppStatusHandler,
	indexer process.Indexer,
	chainID []byte,
	currentPid core.PeerID,
) (slot.SubslotsFactory, error) {
	switch consensusType {
	case blsConsensusType:
		subSlotFactoryBls, err := bls.NewSubslotsFactory(
			consensusDataContainer,
			consensusState,
			worker,
			chainID,
			currentPid,
		)
		if err != nil {
			return nil, err
		}

		err = subSlotFactoryBls.SetAppStatusHandler(appStatusHandler)
		if err != nil {
			return nil, err
		}

		subSlotFactoryBls.SetIndexer(indexer)

		return subSlotFactoryBls, nil
	default:
		return nil, ErrInvalidConsensusType
	}
}

// GetConsensusCoreFactory returns a consensus service depending of the given parameter
func GetConsensusCoreFactory(consensusType string) (slot.ConsensusService, error) {
	switch consensusType {
	case blsConsensusType:
		return bls.NewConsensusService()
	default:
		return nil, ErrInvalidConsensusType
	}
}

// GetBroadcastMessenger returns a consensus service depending of the given parameter
func GetBroadcastMessenger(
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	messenger consensus.P2PMessenger,
	privateKey crypto.PrivateKey,
	peerSignatureHandler crypto.PeerSignatureHandler,
	headersSubscriber consensus.HeadersPoolSubscriber,
	interceptorsContainer process.InterceptorsContainer,
) (consensus.BroadcastMessenger, error) {

	chainMessengerArgs := broadcast.ChainMessengerArgs{
		Marshalizer:                marshalizer,
		Hasher:                     hasher,
		Messenger:                  messenger,
		PrivateKey:                 privateKey,
		PeerSignatureHandler:       peerSignatureHandler,
		HeadersSubscriber:          headersSubscriber,
		MaxDelayCacheSize:          maxDelayCacheSize,
		MaxValidatorDelayCacheSize: maxDelayCacheSize,
		InterceptorsContainer:      interceptorsContainer,
	}

	return broadcast.NewChainMessenger(chainMessengerArgs)
}
