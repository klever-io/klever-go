package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
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

func TestEpochStartBlockProcessor_GetEpochStartBlockShouldNotReturnBelowTargetAfterTimeOut(t *testing.T) {
	t.Parallel()

	peers := []core.PeerID{"peer_0", "peer_1", "peer_2", "peer_3", "peer_4", "peer_5"}
	esmbp, _ := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: func() []core.PeerID {
				return peers
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
	// 3 votes meet the floor but not the target of 6, and the timeout applies the same target
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
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

func createPeerIDs(prefix string, numPeers int) []core.PeerID {
	peers := make([]core.PeerID, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		peers = append(peers, core.PeerID(fmt.Sprintf("%s_%d", prefix, i)))
	}

	return peers
}

func createEpochStartInterceptedBlockForEpoch(t *testing.T, nonce uint64, epoch uint32) *interceptedBlocks.InterceptedBlock {
	mb := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        nonce,
			Epoch:        epoch,
			IsEpochStart: true,
		},
	}
	intData, err := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(mb))
	require.NoError(t, err)

	return intData
}

func createEpochStartInterceptedBlock(t *testing.T, nonce uint64) *interceptedBlocks.InterceptedBlock {
	return createEpochStartInterceptedBlockForEpoch(t, nonce, 0)
}

// changingPeers is a connected peer set a test can change while the processor reads it
type changingPeers struct {
	mut   sync.Mutex
	peers []core.PeerID
}

func (cp *changingPeers) get() []core.PeerID {
	cp.mut.Lock()
	defer cp.mut.Unlock()

	return cp.peers
}

func (cp *changingPeers) set(peers []core.PeerID) {
	cp.mut.Lock()
	cp.peers = peers
	cp.mut.Unlock()
}

func fixedPeers(peers []core.PeerID) func() []core.PeerID {
	return func() []core.PeerID {
		return peers
	}
}

func createProcessor(t *testing.T, connectedPeers func() []core.PeerID, percentage uint8, minValid int) *epochStartBlockProcessor {
	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: connectedPeers,
		},
		&mock.RequestHandlerStub{},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		percentage,
		1,
		minValid,
	)
	require.NoError(t, err)

	return esmbp
}

func saveVotes(esmbp *epochStartBlockProcessor, intData *interceptedBlocks.InterceptedBlock, peers []core.PeerID) {
	for _, pid := range peers {
		_ = esmbp.Save(intData, pid, "")
	}
}

func requireNoConsensus(t *testing.T, esmbp *epochStartBlockProcessor) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*durationBetweenChecks)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_PeerCountTarget(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		numConnected int
		percentage   int
		minValid     int
		expected     int
	}{
		{numConnected: 2, percentage: 67, minValid: 2, expected: 2},
		{numConnected: 2, percentage: 67, minValid: 1, expected: 2},
		{numConnected: 1, percentage: 67, minValid: 1, expected: 1},
		{numConnected: 3, percentage: 67, minValid: 2, expected: 3},
		{numConnected: 10, percentage: 67, minValid: 2, expected: 7},
		{numConnected: 20, percentage: 67, minValid: 2, expected: 14},
		{numConnected: 100, percentage: 67, minValid: 2, expected: 67},
		{numConnected: 6, percentage: 50, minValid: 3, expected: 3},
		{numConnected: 6, percentage: 99, minValid: 3, expected: 6},
		{numConnected: 4, percentage: 10, minValid: 3, expected: 3},
	} {
		esmbp := &epochStartBlockProcessor{consensusPercentage: tc.percentage, minNumOfPeersToConsiderBlockValid: tc.minValid}
		assert.Equal(t, tc.expected, esmbp.computePeerCountTarget(tc.numConnected),
			"connected=%d pct=%d min=%d", tc.numConnected, tc.percentage, tc.minValid)
	}
}

func TestEpochStartBlockProcessor_PeerCountTargetFollowsConnectedPeers(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 10)
	cp := &changingPeers{peers: peers[:2]}
	esmbp := createProcessor(t, cp.get, 67, 2)
	intData := createEpochStartInterceptedBlock(t, 10)
	saveVotes(esmbp, intData, peers[:2])

	// 2 connected peers give a target of 2, which both votes meet
	mb, err := esmbp.getConfirmedMetaBlock()
	require.NoError(t, err)
	assert.Equal(t, intData.HeaderHandler(), mb)

	// 10 connected peers raise the target to 7, which the same votes no longer meet
	cp.set(peers)
	mb, err = esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_OnePeerOfTwoConnectedShouldNotReachConsensus(t *testing.T) {
	t.Parallel()

	esmbp := createProcessor(t, fixedPeers([]core.PeerID{"honest", "attacker"}), 67, 2)
	_ = esmbp.Save(createEpochStartInterceptedBlock(t, 7777), "attacker", "")

	requireNoConsensus(t, esmbp)
}

func TestEpochStartBlockProcessor_TieShouldNotSelectAnyBlock(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 6)
	esmbp := createProcessor(t, fixedPeers(peers), 50, 3)
	honest := createEpochStartInterceptedBlock(t, 10)
	forged := createEpochStartInterceptedBlock(t, 7777)
	require.NotEqual(t, honest.Hash(), forged.Hash())
	saveVotes(esmbp, honest, peers[:3])
	saveVotes(esmbp, forged, peers[3:])

	// both hashes reach the target of 3, while checking and at the timeout
	requireNoConsensus(t, esmbp)
}

func TestEpochStartBlockProcessor_TimeoutShouldApplyTheSameTarget(t *testing.T) {
	t.Parallel()

	// 10 connected peers give a target of 7
	connected := createPeerIDs("peer", 10)

	t.Run("votes below the target should not select", func(t *testing.T) {
		t.Parallel()

		esmbp := createProcessor(t, fixedPeers(connected), 67, 2)
		saveVotes(esmbp, createEpochStartInterceptedBlock(t, 1), connected[:6])

		mb, err := esmbp.getConfirmedMetaBlock()
		assert.Nil(t, mb)
		assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
	})
	t.Run("votes meeting the target should select", func(t *testing.T) {
		t.Parallel()

		esmbp := createProcessor(t, fixedPeers(connected), 67, 2)
		expected := createEpochStartInterceptedBlock(t, 1)
		saveVotes(esmbp, expected, connected[:7])

		mb, err := esmbp.getConfirmedMetaBlock()
		require.NoError(t, err)
		assert.Equal(t, expected.HeaderHandler(), mb)
	})
}

func TestEpochStartBlockProcessor_GetPrevEpochStartBlockReRequestsTheSameEpoch(t *testing.T) {
	t.Parallel()

	var mut sync.Mutex
	requestedEpochs := make([]uint32, 0)
	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{},
		&mock.RequestHandlerStub{
			RequestStartOfEpochBlockCalled: func(epoch uint32) {
				mut.Lock()
				requestedEpochs = append(requestedEpochs, epoch)
				mut.Unlock()
			},
		},
		&mock.MarshalizerMock{},
		&mock.HasherMock{},
		67,
		2,
		2,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), durationBetweenReRequests+durationBetweenChecks)
	_, _ = esmbp.GetPrevEpochStartBlock(ctx, 5)
	cancel()

	mut.Lock()
	defer mut.Unlock()
	require.GreaterOrEqual(t, len(requestedEpochs), 2)
	for _, epoch := range requestedEpochs {
		assert.Equal(t, uint32(5), epoch)
	}
}

func TestEpochStartBlockProcessor_GetPrevEpochStartBlockResetShouldNotRaceWithSave(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 6)
	esmbp := createProcessor(t, fixedPeers(peers), 50, 3)
	intData := createEpochStartInterceptedBlock(t, 10)

	stop := make(chan struct{})
	saverDone := make(chan struct{})
	go func() {
		defer close(saverDone)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				_ = esmbp.Save(intData, peers[i%len(peers)], "")
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), durationBetweenChecks/2)
	_, _ = esmbp.GetPrevEpochStartBlock(ctx, 4)
	cancel()
	close(stop)
	<-saverDone

	// every concurrent save carries epoch 0, not the requested epoch 4: none may survive the
	// reset of the previous epoch fetch
	assert.Empty(t, esmbp.GetMapMetaBlock())
}

func TestEpochStartBlockProcessor_VotesFromDisconnectedPeersShouldNotCount(t *testing.T) {
	t.Parallel()

	// 5 honest peers, and an attacker holding 3 connection slots that it rotated through 9
	// identities: only the last 3 are still connected, so 8 are connected and the target is 6
	honest := createPeerIDs("honest", 5)
	rotated := createPeerIDs("attacker", 9)
	esmbp := createProcessor(t, fixedPeers(slices.Concat(honest, rotated[6:])), 67, 2)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), honest)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 7777), rotated)

	requireNoConsensus(t, esmbp)
}

func TestEpochStartBlockProcessor_VotesOfPeersThatDisconnectedAfterVotingShouldNotCount(t *testing.T) {
	t.Parallel()

	// the same attacker lets each identity vote while connected and then leave, so the 5
	// honest peers and the 3 current identities remain (target 6)
	honest := createPeerIDs("honest", 5)
	rotated := createPeerIDs("attacker", 9)
	cp := &changingPeers{peers: slices.Concat(honest, rotated)}
	esmbp := createProcessor(t, cp.get, 67, 2)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), honest)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 7777), rotated)
	cp.set(slices.Concat(honest, rotated[6:]))

	mb, err := esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_TimeoutShouldRequireTheConnectedPeerTarget(t *testing.T) {
	t.Parallel()

	// 3 honest and 4 attacker connections: 7 connected, so the target is 5, which neither
	// side reaches; the attacker's simple majority must not win at the timeout
	honest := createPeerIDs("honest", 3)
	attacker := createPeerIDs("attacker", 4)
	esmbp := createProcessor(t, fixedPeers(slices.Concat(honest, attacker)), 67, 2)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), honest)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 7777), attacker)

	requireNoConsensus(t, esmbp)
}

func TestEpochStartBlockProcessor_SilentConnectedPeersShouldCountTowardTheTarget(t *testing.T) {
	t.Parallel()

	// 10 connected peers give a target of 7: a unanimous answer from 4 of them is not enough,
	// whatever the other 6 are doing
	connected := createPeerIDs("peer", 10)
	esmbp := createProcessor(t, fixedPeers(connected), 67, 2)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), connected[:4])

	requireNoConsensus(t, esmbp)
}

func TestEpochStartBlockProcessor_LatestVoteOfAPeerShouldReplaceItsPreviousOne(t *testing.T) {
	t.Parallel()

	// every peer first answers with the old epoch start block and, after the epoch rolls
	// over, with the new one: only the new block holds their votes
	peers := createPeerIDs("peer", 3)
	esmbp := createProcessor(t, fixedPeers(peers), 67, 2)
	newBlock := createEpochStartInterceptedBlock(t, 20)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), peers)
	saveVotes(esmbp, newBlock, peers)

	ctx, cancel := context.WithTimeout(context.Background(), 3*durationBetweenChecks)
	mb, err := esmbp.GetEpochStartBlock(ctx)
	cancel()
	require.NoError(t, err)
	assert.Equal(t, newBlock.HeaderHandler(), mb)
}

func TestEpochStartBlockProcessor_OlderAnswerProcessedLateShouldNotReplaceANewerVote(t *testing.T) {
	t.Parallel()

	// the interceptor handles each message on its own goroutine, so a peer's older answer can
	// reach Save after its newer one: the newer block must keep the vote
	peers := createPeerIDs("peer", 3)
	esmbp := createProcessor(t, fixedPeers(peers), 67, 2)
	newBlock := createEpochStartInterceptedBlock(t, 20)
	saveVotes(esmbp, newBlock, peers)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 10), peers)

	mb, err := esmbp.getConfirmedMetaBlock()
	require.NoError(t, err)
	assert.Equal(t, newBlock.HeaderHandler(), mb)
}

func TestEpochStartBlockProcessor_PrevEpochFetchShouldIgnoreOtherEpochs(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 3)
	esmbp := createProcessor(t, fixedPeers(peers), 67, 2)
	esmbp.beginFetch(true, 6203)

	// late answers to the latest epoch start request carry the next epoch
	saveVotes(esmbp, createEpochStartInterceptedBlockForEpoch(t, 20, 6204), peers)
	assert.Empty(t, esmbp.GetMapMetaBlock())

	previous := createEpochStartInterceptedBlockForEpoch(t, 10, 6203)
	saveVotes(esmbp, previous, peers)

	mb, err := esmbp.getConfirmedMetaBlock()
	require.NoError(t, err)
	assert.Equal(t, previous.HeaderHandler(), mb)
}

func TestEpochStartBlockProcessor_MetaBlocksWithoutVotesShouldBeDropped(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 2)
	esmbp := createProcessor(t, fixedPeers(peers), 67, 2)
	first := createEpochStartInterceptedBlock(t, 1)
	third := createEpochStartInterceptedBlock(t, 3)

	saveVotes(esmbp, first, peers)
	_ = esmbp.Save(createEpochStartInterceptedBlock(t, 2), peers[0], "")
	// the first block still holds a vote
	assert.Len(t, esmbp.GetMapMetaBlock(), 2)

	_ = esmbp.Save(third, peers[1], "")
	// the first block lost its last vote
	assert.Len(t, esmbp.GetMapMetaBlock(), 2)
	assert.NotContains(t, esmbp.GetMapMetaBlock(), string(first.Hash()))

	// sending the same block again changes nothing
	_ = esmbp.Save(third, peers[1], "")
	assert.Len(t, esmbp.GetMapMetaBlock(), 2)
}

func TestEpochStartBlockProcessor_VotesOfDisconnectedPeersShouldBeDropped(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 3)
	cp := &changingPeers{peers: peers}
	esmbp := createProcessor(t, cp.get, 67, 2)
	for i, pid := range peers {
		_ = esmbp.Save(createEpochStartInterceptedBlock(t, uint64(i+1)), pid, "")
	}
	require.Len(t, esmbp.GetMapMetaBlock(), 3)

	cp.set(peers[:1])
	esmbp.checkMaps()
	assert.Len(t, esmbp.GetMapMetaBlock(), 1)
}

func TestEpochStartBlockProcessor_PeersLeavingShouldNotLetTheRemainingPeersDecide(t *testing.T) {
	t.Parallel()

	// the fetch starts with 21 connected peers, a floor of 11; then every peer but the two
	// attacker connections leaves, which alone would drop the target to 2
	peers := createPeerIDs("peer", 21)
	cp := &changingPeers{peers: peers}
	esmbp := createProcessor(t, cp.get, 67, 2)
	esmbp.beginFetch(false, 0)

	remaining := peers[:2]
	cp.set(remaining)
	saveVotes(esmbp, createEpochStartInterceptedBlock(t, 7777), remaining)

	mb, err := esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_AttackerConnectionsLeavingShouldNotStallTheRemainingPeers(t *testing.T) {
	t.Parallel()

	// the fetch starts with 6 honest and 3 short-lived attacker connections, a floor of 5; once
	// the attacker leaves, the 6 honest peers meet the target of 5 on their own
	honest := createPeerIDs("honest", 6)
	cp := &changingPeers{peers: slices.Concat(honest, createPeerIDs("attacker", 3))}
	esmbp := createProcessor(t, cp.get, 67, 2)
	esmbp.beginFetch(false, 0)

	cp.set(honest)
	expected := createEpochStartInterceptedBlock(t, 10)
	saveVotes(esmbp, expected, honest)

	mb, err := esmbp.getConfirmedMetaBlock()
	require.NoError(t, err)
	assert.Equal(t, expected.HeaderHandler(), mb)
}

func TestEpochStartBlockProcessor_FetchShouldRecordTheStartFloorAndLetTheTargetGrow(t *testing.T) {
	t.Parallel()

	peers := createPeerIDs("peer", 30)
	cp := &changingPeers{peers: peers[:21]}
	esmbp := createProcessor(t, cp.get, 67, 2)

	ctx, cancel := context.WithTimeout(context.Background(), durationBetweenChecks/2)
	_, _ = esmbp.GetPrevEpochStartBlock(ctx, 4)
	cancel()
	assert.Equal(t, 11, esmbp.startPeerCountFloor)

	// 30 connected peers raise the target to 21, above the start floor
	cp.set(peers)
	intData := createEpochStartInterceptedBlockForEpoch(t, 10, 4)
	saveVotes(esmbp, intData, peers[:20])
	mb, err := esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)

	_ = esmbp.Save(intData, peers[20], "")
	mb, err = esmbp.getConfirmedMetaBlock()
	require.NoError(t, err)
	assert.Equal(t, intData.HeaderHandler(), mb)
}

// createEpochStartBlockCopy returns the same epoch start header with the given signature, which
// the header hash does not cover
func createEpochStartBlockCopy(t *testing.T, signature string) *interceptedBlocks.InterceptedBlock {
	mb := &block.Block{
		Header:    &block.BlockHeader{Nonce: 10, IsEpochStart: true},
		Signature: []byte(signature),
	}
	intData, err := interceptedBlocks.NewInterceptedBlock(createDefaultBlockArgument(mb))
	require.NoError(t, err)

	return intData
}

func TestEpochStartBlockProcessor_VotesShouldCoverTheWholeMetaBlock(t *testing.T) {
	t.Parallel()

	// 3 honest peers and 1 attacker that answers with the honest header but a forged signature:
	// 4 connected give a target of 3, and the honest copy leads among the header's voters
	honestBlock := createEpochStartBlockCopy(t, "honest signature")
	forgedBlock := createEpochStartBlockCopy(t, "forged signature")
	require.Equal(t, honestBlock.Hash(), forgedBlock.Hash(), "both copies share the header")

	honest := createPeerIDs("honest", 3)
	attacker := createPeerIDs("attacker", 1)
	esmbp := createProcessor(t, fixedPeers(slices.Concat(honest, attacker)), 67, 2)
	saveVotes(esmbp, honestBlock, honest)
	saveVotes(esmbp, forgedBlock, attacker)

	// the returned copy must not depend on map iteration order
	for i := 0; i < 50; i++ {
		mb, err := esmbp.getConfirmedMetaBlock()
		require.NoError(t, err)
		require.Equal(t, []byte("honest signature"), mb.Signature)
	}
}

func TestEpochStartBlockProcessor_TiedCopiesOfTheAgreedHeaderShouldNotSelect(t *testing.T) {
	t.Parallel()

	// at 50% the 4 connected peers give a target of 2, which both copies of the agreed header
	// reach with a 2 to 2 split: neither may be picked
	peers := createPeerIDs("peer", 4)
	esmbp := createProcessor(t, fixedPeers(peers), 50, 2)
	saveVotes(esmbp, createEpochStartBlockCopy(t, "copy a"), peers[:2])
	saveVotes(esmbp, createEpochStartBlockCopy(t, "copy b"), peers[2:])

	mb, err := esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_ReturnedCopyShouldMeetTheTarget(t *testing.T) {
	t.Parallel()

	// 100 connected give a target of 67: 33 honest peers and 34 attacker connections all vote
	// the honest header, so the header reaches the target, but the attacker's forged copy only
	// leads the header's voters and does not reach the target on its own
	honest := createPeerIDs("honest", 33)
	attacker := createPeerIDs("attacker", 34)
	silent := createPeerIDs("silent", 33)
	esmbp := createProcessor(t, fixedPeers(slices.Concat(honest, attacker, silent)), 67, 2)
	saveVotes(esmbp, createEpochStartBlockCopy(t, "honest signature"), honest)
	saveVotes(esmbp, createEpochStartBlockCopy(t, "forged signature"), attacker)

	mb, err := esmbp.getConfirmedMetaBlock()
	assert.Nil(t, mb)
	assert.Equal(t, common.ErrTimeoutWaitingForBlock, err)
}

func TestEpochStartBlockProcessor_CopyWithUnknownFieldsOutsideTheHeaderShouldCountAsTheSameBlock(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.ProtoMarshalizerMock{}
	mb := &block.Block{
		Header:    &block.BlockHeader{Nonce: 10, IsEpochStart: true},
		Signature: []byte("signature"),
	}
	originalBuff, err := marshalizer.Marshal(mb)
	require.NoError(t, err)

	newCopy := func(blockBuff []byte) *interceptedBlocks.InterceptedBlock {
		arg := createDefaultBlockArgument(mb)
		arg.Marshalizer = marshalizer
		arg.BlockBuff = blockBuff
		intData, errNew := interceptedBlocks.NewInterceptedBlock(arg)
		require.NoError(t, errNew)
		return intData
	}
	originalCopy := newCopy(originalBuff)
	extendedBuff := protowire.AppendTag(bytes.Clone(originalBuff), 99, protowire.BytesType)
	extendedBuff = protowire.AppendBytes(extendedBuff, bytes.Repeat([]byte{0xab}, 4096))
	extendedCopy := newCopy(extendedBuff)
	require.Equal(t, originalCopy.Hash(), extendedCopy.Hash(), "both copies share the header")

	peers := createPeerIDs("peer", 4)
	esmbp, err := NewEpochStartBlockProcessor(
		&mock.MessengerStub{
			ConnectedPeersCalled: fixedPeers(peers),
		},
		&mock.RequestHandlerStub{},
		marshalizer,
		&mock.HasherMock{},
		67,
		1,
		2,
	)
	require.NoError(t, err)
	saveVotes(esmbp, originalCopy, peers[:2])
	saveVotes(esmbp, extendedCopy, peers[2:])

	ctx, cancel := context.WithTimeout(context.Background(), 3*durationBetweenChecks)
	defer cancel()
	confirmed, err := esmbp.GetEpochStartBlock(ctx)
	require.NoError(t, err)
	confirmedBuff, err := marshalizer.Marshal(confirmed)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(originalBuff, confirmedBuff), "confirmed block must match the original encoding")
}
