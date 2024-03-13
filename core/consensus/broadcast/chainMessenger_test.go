package broadcast_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/stretchr/testify/assert"
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
	}
}

func TestChainMessenger_NewChainMessengerNilMarshalizerShouldFail(t *testing.T) {
	args := createDefaultMetaChainArgs()
	args.Marshalizer = nil
	mcm, err := broadcast.NewChainMessenger(args)

	assert.Nil(t, mcm)
	assert.Equal(t, common.ErrNilMarshalizer, err)
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
