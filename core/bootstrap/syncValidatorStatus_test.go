package bootstrap

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

const initRating = uint32(50)

func TestNewSyncValidatorStatus_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getSyncValidatorStatusArgs()
	svs, err := NewSyncValidatorStatus(args)
	require.NoError(t, err)
	require.False(t, check.IfNil(svs))
}

func TestSyncValidatorStatus_NodesConfigFromMetaBlock(t *testing.T) {
	t.Parallel()
	// TODO: review...
	args := getSyncValidatorStatusArgs()
	svs, _ := NewSyncValidatorStatus(args)

	currMb := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        37,
			Epoch:        0,
			IsEpochStart: true,
		}}

	prevMb := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        36,
			Epoch:        0,
			IsEpochStart: true,
		}}

	vi := make([]*state.ValidatorInfo, 2)
	vi[0] = &state.ValidatorInfo{
		PublicKey:                  []byte("HASH1"),
		List:                       "",
		Index:                      0,
		Rating:                     0,
		LeaderSuccess:              10,
		LeaderFailure:              0,
		ValidatorSuccess:           10,
		ValidatorFailure:           0,
		NumSelectedInSuccessBlocks: 20,
	}

	vi[1] = &state.ValidatorInfo{
		PublicKey:                  []byte("HASH2"),
		List:                       "",
		Index:                      0,
		Rating:                     0,
		LeaderSuccess:              10,
		LeaderFailure:              0,
		ValidatorSuccess:           10,
		ValidatorFailure:           0,
		NumSelectedInSuccessBlocks: 20,
	}

	prevVi := make([]*state.ValidatorInfo, 2)
	prevVi[0] = &state.ValidatorInfo{
		PublicKey:                  []byte("HASH1"),
		List:                       "",
		Index:                      0,
		Rating:                     0,
		LeaderSuccess:              10,
		LeaderFailure:              0,
		ValidatorSuccess:           10,
		ValidatorFailure:           0,
		NumSelectedInSuccessBlocks: 20,
	}

	prevVi[1] = &state.ValidatorInfo{
		PublicKey:                  []byte("HASH2"),
		List:                       "",
		Index:                      0,
		Rating:                     0,
		LeaderSuccess:              10,
		LeaderFailure:              0,
		ValidatorSuccess:           10,
		ValidatorFailure:           0,
		NumSelectedInSuccessBlocks: 20,
	}

	registry, err := svs.NodesConfigFromMetaBlock(currMb, prevMb, vi, prevVi)
	require.NoError(t, err)
	require.NotNil(t, registry)
}

func getSyncValidatorStatusArgs() ArgsNewSyncValidatorStatus {
	return ArgsNewSyncValidatorStatus{
		DataPool: &mock.PoolsHolderStub{
			BlocksCalled: func() storage.Cacher {
				return mock.NewCacherStub()
			},
		},
		Marshalizer:    &mock.MarshalizerMock{},
		Hasher:         &mock.HasherMock{},
		RequestHandler: &mock.RequestHandlerStub{},
		GenesisNodesConfig: &mock.NodesSetupStub{
			InitialNodesInfoCalled: func() ([]sharding.GenesisNodeInfoHandler, []sharding.GenesisNodeInfoHandler, error) {
				return []sharding.GenesisNodeInfoHandler{
						mock.NewNodeInfo([]byte("addr0"), []byte("pubKey0"), initRating),
						mock.NewNodeInfo([]byte("addr1"), []byte("pubKey1"), initRating),
					}, []sharding.GenesisNodeInfoHandler{
						mock.NewNodeInfo([]byte("addr2"), []byte("pubKey2"), initRating),
						mock.NewNodeInfo([]byte("addr3"), []byte("pubKey3"), initRating),
					},
					nil
			},
			GetConsensusGroupSizeCalled: func() uint32 {
				return 2
			},
		},
		NodeShuffler: &mock.NodeShufflerMock{},
		PubKey:       []byte("public key"),
	}
}
