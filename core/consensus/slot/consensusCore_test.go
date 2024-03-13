package slot_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/stretchr/testify/assert"
)

func createDefaultConsensusCoreArgs() *slot.ConsensusCoreArgs {
	consensusCoreMock := mock.InitConsensusCore()

	args := &slot.ConsensusCoreArgs{
		BlockChain:                    consensusCoreMock.Blockchain(),
		BlockProcessor:                consensusCoreMock.BlockProcessor(),
		Bootstrapper:                  consensusCoreMock.BootStrapper(),
		BroadcastMessenger:            consensusCoreMock.BroadcastMessenger(),
		ChronologyHandler:             consensusCoreMock.Chronology(),
		Hasher:                        consensusCoreMock.Hasher(),
		Marshalizer:                   consensusCoreMock.Marshalizer(),
		BlsPrivateKey:                 consensusCoreMock.PrivateKey(),
		BlsSingleSigner:               consensusCoreMock.SingleSigner(),
		MultiSigner:                   consensusCoreMock.MultiSigner(),
		SlotManager:                   consensusCoreMock.SlotManager(),
		NodesCoordinator:              consensusCoreMock.NodesCoordinator(),
		SyncTimer:                     consensusCoreMock.SyncTimer(),
		EpochStartRegistrationHandler: consensusCoreMock.EpochStartRegistrationHandler(),
		AntifloodHandler:              consensusCoreMock.GetAntiFloodHandler(),
		PeerHonestyHandler:            consensusCoreMock.PeerHonestyHandler(),
		HeaderSigVerifier:             consensusCoreMock.HeaderSigVerifier(),
		FallbackHeaderValidator:       consensusCoreMock.FallbackHeaderValidator(),
		NodeRedundancyHandler:         consensusCoreMock.NodeRedundancyHandler(),
	}
	return args
}

func TestConsensusCore_WithNilBlockchainShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.BlockChain = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestConsensusCore_WithNilBlockProcessorShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.BlockProcessor = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestConsensusCore_WithNilBootstrapperShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.Bootstrapper = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)
	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBootstrapper, err)
}

func TestConsensusCore_WithNilBroadcastMessengerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.BroadcastMessenger = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBroadcastMessenger, err)
}

func TestConsensusCore_WithNilChronologyShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.ChronologyHandler = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)
	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilChronologyHandler, err)
}

func TestConsensusCore_WithNilHasherShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.Hasher = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestConsensusCore_WithNilMarshalizerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.Marshalizer = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilMarshalizer, err)
}

func TestConsensusCore_WithNilBlsPrivateKeyShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.BlsPrivateKey = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBlsPrivateKey, err)
}

func TestConsensusCore_WithNilBlsSingleSignerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.BlsSingleSigner = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilBlsSingleSigner, err)
}

func TestConsensusCore_WithNilMultiSignerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.MultiSigner = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestConsensusCore_WithNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.SlotManager = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestConsensusCore_WithNilNodesCoordinatorShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.NodesCoordinator = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilNodesCoordinator, err)
}

func TestConsensusCore_WithNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.SyncTimer = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestConsensusCore_WithNilAntifloodHandlerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.AntifloodHandler = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilAntifloodHandler, err)
}

func TestConsensusCore_WithNilPeerHonestyHandlerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.PeerHonestyHandler = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilPeerHonestyHandler, err)
}

func TestConsensusCore_WithNilHeaderSigVerifierShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.HeaderSigVerifier = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilHeaderSigVerifier, err)
}

func TestConsensusCore_WithNilFallbackHeaderValidatorShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.FallbackHeaderValidator = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilFallbackHeaderValidator, err)
}

func TestConsensusCore_WithNilNodeRedundancyHandlerShouldFail(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	args.NodeRedundancyHandler = nil

	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.Nil(t, consensusCore)
	assert.Equal(t, slot.ErrNilNodeRedundancyHandler, err)
}

func TestConsensusCore_CreateConsensusCoreShouldWork(t *testing.T) {
	t.Parallel()

	args := createDefaultConsensusCoreArgs()
	consensusCore, err := slot.NewConsensusCore(
		args,
	)

	assert.NotNil(t, consensusCore)
	assert.Nil(t, err)
}
