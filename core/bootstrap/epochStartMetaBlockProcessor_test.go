package bootstrap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	cMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process/block/interceptedBlocks"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func createDefaultBlockArgument(blck *block.Block) *interceptedBlocks.ArgInterceptedBlock {
	arg := &interceptedBlocks.ArgInterceptedBlock{
		Hasher:                  &mock.HasherMock{},
		Marshalizer:             &mock.MarshalizerMock{},
		KeyGen:                  createMockKeyGen(),
		HeaderSigVerifier:       &cMock.HeaderSigVerifierStub{},
		HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
		EpochStartTrigger:       &mock.EpochStartTriggerStub{},
		ForkController:          &mock.ForkControllerStub{},
	}

	arg.BlockBuff, _ = arg.Marshalizer.Marshal(blck)

	return arg
}

func createMockKeyGen() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

func TestNewEpochStartBlockProcessor_NilMessengerShouldErr(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		nil,
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.Equal(t, common.ErrNilMessenger, err)
	assert.True(t, check.IfNil(esmbp))
}

func TestNewEpochStartBlockProcessor_NilRequestHandlerShouldErr(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		nil,
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.Equal(t, common.ErrNilRequestHandler, err)
	assert.True(t, check.IfNil(esmbp))
}

func TestNewEpochStartBlockProcessor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		nil,
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.Equal(t, common.ErrNilMarshalizer, err)
	assert.True(t, check.IfNil(esmbp))
}

func TestNewEpochStartBlockProcessor_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		nil,
		50,
		3,
		3,
	)

	assert.Equal(t, common.ErrNilHasher, err)
	assert.True(t, check.IfNil(esmbp))
}

func TestNewEpochStartBlockProcessor_InvalidConsensusPercentageShouldErr(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		101,
		3,
		3,
	)

	assert.Equal(t, common.ErrInvalidConsensusThreshold, err)
	assert.True(t, check.IfNil(esmbp))
}

func TestNewEpochStartBlockProcessorOkValsShouldWork(t *testing.T) {
	t.Parallel()

	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.NoError(t, err)
	assert.False(t, check.IfNil(esmbp))
}

func getConnectedPeers(counter int) []core.PeerID {
	switch counter {
	case 0:
		return []core.PeerID{"peer0", "peer1"}
	case 1:
		return []core.PeerID{"peer0", "peer1", "peer2", "peer3"}
	case 2:
		return []core.PeerID{"peer0", "peer1", "peer2", "peer3", "peer4", "peer5"}
	}
	return nil
}

func TestNewEpochStartBlockProcessorOkValsShouldWorkAfterMoreTriesWaitingForConnectedPeers(t *testing.T) {
	t.Parallel()

	counter := 0
	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				peers := getConnectedPeers(counter)
				counter++
				return peers
			},
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.NoError(t, err)
	assert.False(t, check.IfNil(esmbp))
}

func TestEpochStartBlockProcessor_Validate(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.Nil(t, esmbp.Validate(nil, ""))
}

func TestEpochStartBlockProcessor_SaveNilInterceptedDataShouldNotReturnError(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	err := esmbp.Save(nil, "peer0", "")
	assert.NoError(t, err)
}

func TestEpochStartBlockProcessor_SaveOkInterceptedDataShouldWork(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	assert.Zero(t, len(esmbp.GetMapMetaBlock()))
	mb := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        10,
			IsEpochStart: true,
		},
	}
	intData, _ := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(mb))
	err := esmbp.Save(intData, "peer0", "")
	assert.NoError(t, err)

	assert.Equal(t, 1, len(esmbp.GetMapMetaBlock()))
}

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldTimeOut(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
			},
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldReturnMostReceivedAfterTimeOut(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
			},
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		99,
		3,
		3,
	)

	expectedMetaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        10,
			IsEpochStart: true,
		},
	}
	intData, _ := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(expectedMetaBlock))

	for i := 0; i < esmbp.minNumOfPeersToConsiderBlockValid; i++ {
		_ = esmbp.Save(intData, core.PeerID(fmt.Sprintf("peer_%d", i)), "")
	}

	// we need a slightly more time than 1 second in order to also properly test the select branches
	timeout := time.Second + time.Millisecond*500
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	assert.NoError(t, err)
	assert.Equal(t, expectedMetaBlock, mb)
}

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldWorkFromFirstTry(t *testing.T) {
	t.Parallel()

	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
			},
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		50,
		3,
		3,
	)

	expectedMetaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        10,
			IsEpochStart: true,
		},
	}
	intData, _ := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(expectedMetaBlock))

	for i := 0; i < 6; i++ {
		_ = esmbp.Save(intData, core.PeerID(fmt.Sprintf("peer_%d", i)), "")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	assert.NoError(t, err)
	assert.Equal(t, expectedMetaBlock, mb)
}

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldWorkAfterMultipleTries(t *testing.T) {
	t.Parallel()

	testEpochStartMbIsReceivedWithSleepBetweenReceivedMessages(t, durationBetweenChecks-10*time.Millisecond)
}

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldWorkAfterMultipleRequests(t *testing.T) {
	t.Parallel()

	testEpochStartMbIsReceivedWithSleepBetweenReceivedMessages(t, durationBetweenChecks-10*time.Millisecond)
}

func testEpochStartMbIsReceivedWithSleepBetweenReceivedMessages(t *testing.T, tts time.Duration) {
	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				return []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
			},
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		64,
		3,
		3,
	)
	expectedMetaBlock := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        10,
			IsEpochStart: true,
		},
	}
	intData, _ := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(expectedMetaBlock))
	go func() {
		index := 0
		for {
			time.Sleep(tts)
			_ = esmbp.Save(intData, core.PeerID(fmt.Sprintf("peer_%d", index)), "")
			_ = esmbp.Save(intData, core.PeerID(fmt.Sprintf("peer_%d", index+1)), "")
			index += 2
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	assert.NoError(t, err)
	assert.Equal(t, expectedMetaBlock, mb)
}
