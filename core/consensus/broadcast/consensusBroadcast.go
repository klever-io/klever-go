package broadcast

import (
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

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

	messengerArgs := ChainMessengerArgs{
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

	return NewChainMessenger(messengerArgs)

}
