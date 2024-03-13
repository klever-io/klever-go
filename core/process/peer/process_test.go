package peer_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/peer"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

const (
	validatorIncreaseRatingStep = int32(1)
	validatorDecreaseRatingStep = int32(-2)
	proposerIncreaseRatingStep  = int32(2)
	proposerDecreaseRatingStep  = int32(-4)
	minRating                   = uint32(1)
	maxRating                   = uint32(100)
	startRating                 = uint32(50)
	defaultChancesSelection     = uint32(1)
	consensusGroupFormat        = "%s_%v_%v"
)

func createMockPubkeyConverter() *cryptoMock.PubkeyConverterMock {
	return cryptoMock.NewPubkeyConverterMock(32)
}

func createMockArguments() peer.ArgValidatorStatisticsProcessor {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	argsNewEconomicsData := economics.ArgsNewEconomicsData{
		EpochNotifier: &mock.EpochNotifierStub{},
	}
	economicsData, _ := economics.NewEconomicsData(argsNewEconomicsData)

	_ = economicsData.SetProposalController(proposalController)

	arguments := peer.ArgValidatorStatisticsProcessor{
		Marshalizer: &mock.ProtoMarshalizerMock{},
		DataPool: &mock.PoolsHolderStub{
			HeadersCalled: func() retriever.HeadersPool {
				return nil
			},
		},
		StorageService:     &mock.ChainStorerMock{},
		NodesCoordinator:   &mock.NodesCoordinatorMock{},
		PubkeyConv:         createMockPubkeyConverter(),
		PeerAdapter:        getAccountsMock(),
		Rater:              createMockRater(),
		RewardsHandler:     economicsData,
		MaxComputableSlots: 1000,
		NodesSetup:         &mock.NodesSetupStub{},
		EpochNotifier:      &mock.EpochNotifierStub{},
	}
	return arguments
}

func createMockRater() *mock.RaterMock {
	rater := mock.GetNewMockRater()
	rater.MinRating = minRating
	rater.MaxRating = maxRating
	rater.StartRating = startRating
	rater.IncreaseProposer = proposerIncreaseRatingStep
	rater.DecreaseProposer = proposerDecreaseRatingStep
	rater.IncreaseValidator = validatorIncreaseRatingStep
	rater.DecreaseValidator = validatorDecreaseRatingStep
	return rater
}

func createMockCache() map[string]data.HeaderHandler {
	return make(map[string]data.HeaderHandler)
}

func getAccountsMock() *mock.AccountsStub {
	return &mock.AccountsStub{
		CommitCalled: func() (bytes []byte, e error) {
			return make([]byte, 0), nil
		},
		LoadAccountCalled: func(address []byte) (handler state.AccountHandler, e error) {
			return &mock.PeerAccountHandlerMock{}, nil
		},
	}
}

func getHeaderHandler(randSeed []byte) *block.Block {
	return &block.Block{
		Header: &block.BlockHeader{
			Nonce:        2,
			PrevRandSeed: randSeed,
			ParentHash:   randSeed,
		},
		PubKeysBitmap: randSeed,
	}
}

func TestNewValidatorStatisticsProcessor_NilPeerAdaptersShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.PeerAdapter = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilPeerAccountsAdapter, err)
}

func TestNewValidatorStatisticsProcessor_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.PubkeyConv = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewValidatorStatisticsProcessor_NilNodesCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.NodesCoordinator = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewValidatorStatisticsProcessor_NilStorageShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.StorageService = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilStorage, err)
}

func TestNewValidatorStatisticsProcessor_ZeroMaxComputableSlotsShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.MaxComputableSlots = 0
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrZeroMaxComputableSlots, err)
}

func TestNewValidatorStatisticsProcessor_NilRaterShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.Rater = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilRater, err)
}

func TestNewValidatorStatisticsProcessor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.Marshalizer = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewValidatorStatisticsProcessor_NilDataPoolShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.DataPool = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilDataPoolHolder, err)
}
