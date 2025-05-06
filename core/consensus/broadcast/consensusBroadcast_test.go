package broadcast_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/stretchr/testify/require"
)

func TestConsensusBroadcast_GetBroadcastMessengerShouldWork(t *testing.T) {
	args := createDefaultMetaChainArgs()

	_, err := broadcast.GetBroadcastMessenger(&broadcast.GetBroadcastMessengerArgs{
		Marshalizer:           args.Marshalizer,
		Hasher:                args.Hasher,
		Messenger:             args.Messenger,
		PrivateKey:            args.PrivateKey,
		PeerSignatureHandler:  args.PeerSignatureHandler,
		HeadersSubscriber:     args.HeadersSubscriber,
		InterceptorsContainer: args.InterceptorsContainer,
		AlarmScheduler:        args.AlarmScheduler,
	})
	require.Nil(t, err)
}
