package broadcast

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

const maxDelayCacheSize = 20

type GetBroadcastMessengerArgs struct {
	Marshalizer           marshal.Marshalizer
	Hasher                hashing.Hasher
	Messenger             consensus.P2PMessenger
	PrivateKey            crypto.PrivateKey
	PeerSignatureHandler  crypto.PeerSignatureHandler
	HeadersSubscriber     consensus.HeadersPoolSubscriber
	InterceptorsContainer process.InterceptorsContainer
	AlarmScheduler        core.TimersScheduler
}

// GetBroadcastMessenger returns a consensus service depending of the given parameter
func GetBroadcastMessenger(
	args *GetBroadcastMessengerArgs,
) (consensus.BroadcastMessenger, error) {

	messengerArgs := ChainMessengerArgs{
		Marshalizer:                args.Marshalizer,
		Hasher:                     args.Hasher,
		Messenger:                  args.Messenger,
		PrivateKey:                 args.PrivateKey,
		PeerSignatureHandler:       args.PeerSignatureHandler,
		HeadersSubscriber:          args.HeadersSubscriber,
		MaxDelayCacheSize:          maxDelayCacheSize,
		MaxValidatorDelayCacheSize: maxDelayCacheSize,
		InterceptorsContainer:      args.InterceptorsContainer,
		AlarmScheduler:             args.AlarmScheduler,
	}

	return NewChainMessenger(messengerArgs)
}
