package broadcast_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/alarm"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createInterceptorContainer() process.InterceptorsContainer {
	return &mock.InterceptorsContainerStub{
		GetCalled: func(topic string) (process.Interceptor, error) {
			return &mock.InterceptorStub{
				ProcessReceivedMessageCalled: func(message p2p.MessageP2P) error {
					return nil
				},
			}, nil
		},
	}
}

func createDefaultMetaChainArgs() broadcast.ChainMessengerArgs {
	marshalizerMock := &mock.MarshalizerMock{}
	messengerMock := &mock.MessengerStub{}
	privateKeyMock := &cryptoMock.PrivateKeyMock{}
	singleSignerMock := &mock.SingleSignerMock{}
	hasher := mock.HasherMock{}
	headersSubscriber := &mock.HeadersCacherStub{}
	interceptorsContainer := createInterceptorContainer()
	peerSigHandler := &mock.PeerSignatureHandler{Signer: singleSignerMock}
	alarmScheduler := alarm.NewAlarmScheduler()

	return broadcast.ChainMessengerArgs{
		Marshalizer:                marshalizerMock,
		Hasher:                     hasher,
		Messenger:                  messengerMock,
		PrivateKey:                 privateKeyMock,
		PeerSignatureHandler:       peerSigHandler,
		HeadersSubscriber:          headersSubscriber,
		InterceptorsContainer:      interceptorsContainer,
		MaxValidatorDelayCacheSize: 2,
		MaxDelayCacheSize:          2,
		AlarmScheduler:             alarmScheduler,
	}
}

func TestChainMessenger_NewChainMessengerNilMarshalizerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.Marshalizer = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func TestChainMessenger_NewChainMessengerNilHasherShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.Hasher = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestChainMessenger_NewChainMessengerNilMessengerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.Messenger = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilMessenger, err)
}

func TestChainMessenger_NewChainMessengerNilPrivateKeyShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.PrivateKey = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, crypto.ErrNilPrivateKey, err)
}

func TestChainMessenger_NewChainMessengerNilPeerSignatureHandlerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.PeerSignatureHandler = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilPeerSignatureHandler, err)
}

func TestChainMessenger_NewChainMessengerNilInterceptorsContainerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.InterceptorsContainer = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilInterceptorsContainer, err)
}

func TestChainMessenger_NewChainMessengerNilHeadersSubscrivberShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.HeadersSubscriber = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilHeadersSubscriber, err)
}

func TestChainMessenger_NewChainMessengerNilAlarmSchedulerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.AlarmScheduler = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilAlarmScheduler, err)
}

func TestChainMessenger_NewChainMessengerZeroCacheSizeShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.MaxDelayCacheSize = 0
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrInvalidCacheSize, err)
}

func TestChainMessenger_NewChainMessengerShouldWork(t *testing.T) {
	args := createDefaultMetaChainArgs()
	mcm, err := broadcast.NewChainMessenger(args)

	assert.NotNil(t, mcm)
	assert.Equal(t, nil, err)
	assert.False(t, mcm.IsInterfaceNil())
}

func TestChainMessenger_BroadcastBlockShouldErrNilMetaHeader(t *testing.T) {
	args := createDefaultMetaChainArgs()
	mcm, _ := broadcast.NewChainMessenger(args)

	err := mcm.BroadcastBlock(nil)
	assert.Equal(t, common.ErrNilHeader, err)
}
func TestChainMessenger_BroadcastBlockShouldWork(t *testing.T) {
	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
		},
	}
	args := createDefaultMetaChainArgs()
	args.Messenger = messenger
	mcm, _ := broadcast.NewChainMessenger(args)

	err := mcm.BroadcastBlock(&block.Block{})
	assert.Nil(t, err)
}

func TestChainMessenger_BroadcastConsensusMessageShoulError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		singleSignerMock *mock.SingleSignerMock
		marshalizer      marshal.Marshalizer
	}{
		{
			name: "should err when peer signature is invalid",
			singleSignerMock: &mock.SingleSignerMock{
				SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
					return nil, fmt.Errorf("mock-error")
				},
			},
		},
		{
			name: "should err when marshalizer is invalid",
			marshalizer: &mock.MarshalizerStub{
				MarshalCalled: func(obj interface{}) ([]byte, error) {
					return nil, fmt.Errorf("mock-error")
				},
			},
		},
	}

	for _, tt := range tests {
		args := createDefaultMetaChainArgs()
		singleSigner := &mock.SingleSignerMock{
			SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
				return nil, nil
			},
		}
		if tt.singleSignerMock != nil {
			singleSigner = tt.singleSignerMock
		}
		args.PeerSignatureHandler = &mock.PeerSignatureHandler{Signer: singleSigner}

		var marshalizer marshal.Marshalizer
		marshalizer = &mock.MarshalizerMock{}
		if tt.marshalizer != nil {
			marshalizer = tt.marshalizer
		}
		args.Marshalizer = marshalizer

		mcm, err := broadcast.NewChainMessenger(args)
		require.Nil(t, err)

		err = mcm.BroadcastConsensusMessage(&consensus.Message{
			OriginatorPid: []byte{1, 2, 3},
			Signature:     []byte{4, 5, 6},
		})
		require.NotNil(t, err)
	}
}

func TestChainMessenger_BroadcastConsensusMessageShoulWork(t *testing.T) {
	t.Parallel()
	singleSignerMock := &mock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			return nil, nil
		},
	}

	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
		},
	}

	args := createDefaultMetaChainArgs()
	args.PeerSignatureHandler = &mock.PeerSignatureHandler{Signer: singleSignerMock}
	args.Messenger = messenger
	mcm, err := broadcast.NewChainMessenger(args)
	require.Nil(t, err)

	err = mcm.BroadcastConsensusMessage(&consensus.Message{
		OriginatorPid: []byte{1, 2, 3},
		Signature:     []byte{4, 5, 6},
	})
	require.Nil(t, err)
}

func TestChainMessenger_BroadcastBlockDataLeaderShouldErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		header      data.HeaderHandler
		blockBuff   []byte
		transaction [][]byte
		marshalizer marshal.Marshalizer
		expectedErr error
	}{
		{
			name:        "should err when header is nil",
			header:      nil,
			expectedErr: common.ErrNilHeader,
		},
		{
			name:   "should err when header hash calculation fails",
			header: &mock.HeaderHandlerStub{},
			marshalizer: &mock.MarshalizerStub{
				MarshalCalled: func(obj interface{}) ([]byte, error) {
					return nil, fmt.Errorf("mock-error")
				},
			},
			expectedErr: fmt.Errorf("mock-error"),
		},
	}

	for _, tt := range tests {
		args := createDefaultMetaChainArgs()
		var marshalizer marshal.Marshalizer
		marshalizer = &mock.MarshalizerMock{}
		if tt.marshalizer != nil {
			marshalizer = tt.marshalizer
		}
		args.Marshalizer = marshalizer

		mcm, err := broadcast.NewChainMessenger(args)
		require.Nil(t, err)
		err = mcm.BroadcastBlockDataLeader(tt.header, nil, nil)
		require.Equal(t, tt.expectedErr, err)
	}
}

func TestChainMessenger_BroadcastBlockDataLeaderShouldOK(t *testing.T) {
	t.Parallel()

	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
		},
	}

	args := createDefaultMetaChainArgs()
	args.Messenger = messenger

	mcm, err := broadcast.NewChainMessenger(args)
	require.Nil(t, err)

	err = mcm.BroadcastBlockDataLeader(&block.Block{
		Header: &block.BlockHeader{
			Nonce: 10,
		},
	}, nil, [][]byte{{1, 2, 3}})

	require.Nil(t, err)
}

func TestChainMessenger_PrepareBroadcastHeaderValidatorShouldOK(t *testing.T) {
	t.Parallel()

	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
		},
	}

	args := createDefaultMetaChainArgs()
	args.Messenger = messenger

	mcm, err := broadcast.NewChainMessenger(args)
	require.Nil(t, err)

	mcm.PrepareBroadcastHeaderValidator(&block.Block{}, nil, 0, nil)
}

func TestChainMessenger_PrepareBroadcastBlockDataValidatorShouldOK(t *testing.T) {
	t.Parallel()

	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
		},
	}

	args := createDefaultMetaChainArgs()
	args.Messenger = messenger

	mcm, err := broadcast.NewChainMessenger(args)
	require.Nil(t, err)

	mcm.PrepareBroadcastBlockDataValidator(&block.Block{}, nil, 0, nil)
}

func TestChainMessenger_CloseShouldOK(t *testing.T) {
	t.Parallel()

	args := createDefaultMetaChainArgs()

	mcm, err := broadcast.NewChainMessenger(args)
	require.Nil(t, err)

	mcm.Close()
}

func TestChainMessenger_BroadcastTransactionsShouldWork(t *testing.T) {
	args := createDefaultMetaChainArgs()
	mcm, _ := broadcast.NewChainMessenger(args)

	err := mcm.BroadcastTransactions(nil)
	assert.Nil(t, err)
}

func TestChainMessenger_BroadcastHeaderNilHeaderShouldErr(t *testing.T) {
	args := createDefaultMetaChainArgs()
	mcm, _ := broadcast.NewChainMessenger(args)

	err := mcm.BroadcastHeader(nil)
	assert.Equal(t, common.ErrNilHeader, err)
}

func TestChainMessenger_BroadcastHeaderOkHeaderShouldWork(t *testing.T) {
	channelCalled := make(chan bool)

	messenger := &mock.MessengerStub{
		BroadcastCalled: func(topic string, buff []byte) {
			channelCalled <- true
		},
	}
	args := createDefaultMetaChainArgs()
	args.Messenger = messenger
	mcm, _ := broadcast.NewChainMessenger(args)

	hdr := block.Block{
		Header: &block.BlockHeader{
			Nonce: 10,
		},
	}

	err := mcm.BroadcastHeader(&hdr)
	assert.Nil(t, err)

	wasCalled := false
	select {
	case <-channelCalled:
		wasCalled = true
	case <-time.After(time.Millisecond * 100):
	}

	assert.Nil(t, err)
	assert.True(t, wasCalled)
}
