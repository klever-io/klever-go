package bootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func TestNewEpochStartMetaSyncer_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getEpochStartSyncerArgs()
	ess, err := NewEpochStartMetaSyncer(args)
	require.NoError(t, err)
	require.False(t, check.IfNil(ess))
}

func TestEpochStartMetaSyncer_SyncEpochStartMetaRegisterMessengerProcessorFailsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")

	args := getEpochStartSyncerArgs()
	messenger := &mock.MessengerStub{
		RegisterMessageProcessorCalled: func(_ string, _ p2p.MessageProcessor) error {
			return expectedErr
		},
	}
	args.Messenger = messenger
	ess, _ := NewEpochStartMetaSyncer(args)

	mb, err := ess.SyncEpochStartMeta(time.Second)
	require.Equal(t, expectedErr, err)
	require.Nil(t, mb)
}

func TestEpochStartMetaSyncer_SyncEpochStartMetaProcessorFailsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")

	args := getEpochStartSyncerArgs()
	messenger := &mock.MessengerStub{
		ConnectedPeersCalled: func() []core.PeerID {
			return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
		},
	}
	args.Messenger = messenger
	ess, _ := NewEpochStartMetaSyncer(args)

	mbIntercProc := &mock.BlockInterceptorProcessorStub{
		GetEpochStartBlockCalled: func() (*block.Block, error) {
			return nil, expectedErr
		},
	}
	ess.SetEpochStartBlockInterceptorProcessor(mbIntercProc)

	mb, err := ess.SyncEpochStartMeta(time.Second)
	require.Equal(t, expectedErr, err)
	require.Nil(t, mb)
}

func TestEpochStartMetaSyncer_SyncEpochStartMetaShouldWork(t *testing.T) {
	t.Parallel()

	expectedMb := &block.Block{Header: &block.BlockHeader{Nonce: 37}}

	args := getEpochStartSyncerArgs()
	messenger := &mock.MessengerStub{
		ConnectedPeersCalled: func() []core.PeerID {
			return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
		},
	}
	args.Messenger = messenger
	ess, err := NewEpochStartMetaSyncer(args)
	require.Nil(t, err)

	mbIntercProc := &mock.BlockInterceptorProcessorStub{
		GetEpochStartBlockCalled: func() (*block.Block, error) {
			return expectedMb, nil
		},
	}
	ess.SetEpochStartBlockInterceptorProcessor(mbIntercProc)

	mb, err := ess.SyncEpochStartMeta(time.Second)
	require.NoError(t, err)
	require.Equal(t, expectedMb, mb)
}

func getEpochStartSyncerArgs() ArgsNewEpochStartMetaSyncer {
	return ArgsNewEpochStartMetaSyncer{
		RequestHandler:          &mock.RequestHandlerStub{},
		Messenger:               &mock.MessengerStub{},
		Marshalizer:             &mock.MarshalizerMock{},
		TxSignMarshalizer:       &mock.MarshalizerMock{},
		KeyGen:                  &cryptoMock.KeyGenMock{},
		BlockKeyGen:             &cryptoMock.KeyGenMock{},
		Hasher:                  &mock.HasherMock{},
		Signer:                  &cryptoMock.SingleSignerStub{},
		BlockSigner:             &cryptoMock.SingleSignerStub{},
		ChainID:                 []byte("chain-ID"),
		WhitelistHandler:        &mock.WhiteListHandlerStub{},
		AddressPubkeyConv:       cryptoMock.NewPubkeyConverterMock(32),
		NonceConverter:          &mock.Uint64ByteSliceConverterMock{},
		HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
		BlockProcessor:          &mock.BlockInterceptorProcessorStub{},
		ForkController:          &mock.ForkControllerStub{},
	}
}
