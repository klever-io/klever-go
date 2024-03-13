package slotFactory_test

import (
	"testing"

	commonmock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/slotFactory"
	netmock "github.com/klever-io/klever-go/network/p2p/mock"

	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

var currentPid = core.PeerID("pid")

func TestGetConsensusCoreFactory_InvalidTypeShouldErr(t *testing.T) {
	t.Parallel()

	csf, err := slotFactory.GetConsensusCoreFactory("invalid")

	assert.Nil(t, csf)
	assert.Equal(t, slotFactory.ErrInvalidConsensusType, err)
}

func TestGetConsensusCoreFactory_BlsShouldWork(t *testing.T) {
	t.Parallel()

	csf, err := slotFactory.GetConsensusCoreFactory(consensus.BlsConsensusType)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(csf))
}

func TestGetSubslotsFactory_BlsNilConsensusCoreShouldErr(t *testing.T) {
	t.Parallel()

	worker := &mock.SposWorkerMock{}
	consensusType := consensus.BlsConsensusType
	statusHandler := netmock.NewAppStatusHandlerMock()
	chainID := []byte("chain-id")
	indexer := &mock.IndexerMock{}
	sf, err := slotFactory.GetSubslotsFactory(
		nil,
		&slot.ConsensusState{},
		worker,
		consensusType,
		statusHandler,
		indexer,
		chainID,
		currentPid,
	)

	assert.Nil(t, sf)
	assert.Equal(t, slot.ErrNilConsensusCore, err)
}

func TestGetSubslotsFactory_BlsNilStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	consensusCore := mock.InitConsensusCore()
	worker := &mock.SposWorkerMock{}
	consensusType := consensus.BlsConsensusType
	chainID := []byte("chain-id")
	indexer := &mock.IndexerMock{}
	sf, err := slotFactory.GetSubslotsFactory(
		consensusCore,
		&slot.ConsensusState{},
		worker,
		consensusType,
		nil,
		indexer,
		chainID,
		currentPid,
	)

	assert.Nil(t, sf)
	assert.Equal(t, slot.ErrNilAppStatusHandler, err)
}

func TestGetSubslotsFactory_BlsShouldWork(t *testing.T) {
	t.Parallel()

	consensusCore := mock.InitConsensusCore()
	worker := &mock.SposWorkerMock{}
	consensusType := consensus.BlsConsensusType
	statusHandler := netmock.NewAppStatusHandlerMock()
	chainID := []byte("chain-id")
	indexer := &mock.IndexerMock{}
	sf, err := slotFactory.GetSubslotsFactory(
		consensusCore,
		&slot.ConsensusState{},
		worker,
		consensusType,
		statusHandler,
		indexer,
		chainID,
		currentPid,
	)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(sf))
}

func TestGetSubslotsFactory_InvalidConsensusTypeShouldErr(t *testing.T) {
	t.Parallel()

	consensusType := "invalid"
	sf, err := slotFactory.GetSubslotsFactory(
		nil,
		nil,
		nil,
		consensusType,
		nil,
		nil,
		nil,
		currentPid,
	)

	assert.Nil(t, sf)
	assert.Equal(t, slotFactory.ErrInvalidConsensusType, err)
}

func TestGetBroadcastMessenger_ShardShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &commonmock.MarshalizerMock{}
	hasher := &commonmock.HasherMock{}
	messenger := &commonmock.MessengerStub{}
	privateKey := &cryptoMock.PrivateKeyMock{}
	peerSigHandler := &commonmock.PeerSignatureHandler{}
	headersSubscriber := &commonmock.HeadersCacherStub{}
	interceptosContainer := &commonmock.InterceptorsContainerStub{}

	bm, err := slotFactory.GetBroadcastMessenger(
		marshalizer,
		hasher,
		messenger,
		privateKey,
		peerSigHandler,
		headersSubscriber,
		interceptosContainer,
	)

	assert.Nil(t, err)
	assert.NotNil(t, bm)
}

func TestGetBroadcastMessenger_MetachainShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hasher := &commonmock.HasherMock{}
	messenger := &commonmock.MessengerStub{}
	privateKey := &cryptoMock.PrivateKeyMock{}
	peerSigHandler := &commonmock.PeerSignatureHandler{}
	headersSubscriber := &commonmock.HeadersCacherStub{}
	interceptosContainer := &commonmock.InterceptorsContainerStub{}

	bm, err := slotFactory.GetBroadcastMessenger(
		marshalizer,
		hasher,
		messenger,
		privateKey,
		peerSigHandler,
		headersSubscriber,
		interceptosContainer,
	)

	assert.Nil(t, err)
	assert.NotNil(t, bm)
}
