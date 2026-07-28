package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateComponentsFactory_NilPathManagerShouldErr(t *testing.T) {
	t.Parallel()

	args := getStateArgs()
	args.PathManager = nil

	scf, err := factory.NewStateComponentsFactory(args)
	require.Nil(t, scf)
	require.Equal(t, factory.ErrNilPathManager, err)
}

func TestNewStateComponentsFactory_NilCoreComponents(t *testing.T) {
	t.Parallel()

	args := getStateArgs()
	args.Core = nil

	scf, err := factory.NewStateComponentsFactory(args)
	require.Nil(t, scf)
	require.Equal(t, factory.ErrNilCoreComponents, err)
}

func TestNewStateComponentsFactory_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getStateArgs()

	scf, err := factory.NewStateComponentsFactory(args)
	require.NoError(t, err)
	require.NotNil(t, scf)
}

func TestStateComponentsFactory_Create_ShouldWork(t *testing.T) {
	t.Parallel()

	args := getStateArgs()

	scf, err := factory.NewStateComponentsFactory(args)
	assert.NoError(t, err)

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
	assert.NoError(t, err)

	res, err := scf.Create(forkController)
	require.NoError(t, err)
	require.NotNil(t, res)
}

// The VM query elements (cmd/node/sc.go) build their own read-only KAppController
// from StateComponents.KAppArgs instead of re-threading each dependency, so Create
// must expose the very args the production controllers were built from.
func TestStateComponentsFactory_Create_ExposesKAppArgs(t *testing.T) {
	t.Parallel()

	scf, err := factory.NewStateComponentsFactory(getStateArgs())
	require.NoError(t, err)

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
	require.NoError(t, err)

	res, err := scf.Create(forkController)
	require.NoError(t, err)

	require.NotNil(t, res.KAppArgs.Hasher)
	require.NotNil(t, res.KAppArgs.Marshalizer)
	require.NotNil(t, res.KAppArgs.RatingsData)
	require.Same(t, forkController, res.KAppArgs.ForkController,
		"the exposed args must carry the fork controller Create was given")
	require.Same(t, res.AddressPubkeyConverter, res.KAppArgs.PubkeyConv,
		"the exposed args must carry the same pubkey converter as the components")

	require.False(t, res.KAppArgs.ReadOnly,
		"the shared args must stay writable; only the query path flips ReadOnly on its copy")
	require.False(t, res.KAppController.IsReadOnly())
	require.False(t, res.KAppControllerSimulator.IsReadOnly())
}

// Reproduces the query-element wiring: taking KAppArgs by value, flipping ReadOnly
// and building a controller must yield a read-only one while leaving the production
// controllers writable.
func TestStateComponentsFactory_KAppArgs_ReadOnlyCopyDoesNotAffectProduction(t *testing.T) {
	t.Parallel()

	scf, err := factory.NewStateComponentsFactory(getStateArgs())
	require.NoError(t, err)

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
	require.NoError(t, err)

	res, err := scf.Create(forkController)
	require.NoError(t, err)

	queryArgs := res.KAppArgs
	queryArgs.ReadOnly = true

	queryController, err := kappcontroller.NewKappController(queryArgs)
	require.NoError(t, err)
	require.True(t, queryController.IsReadOnly())

	require.NotSame(t, res.KAppController, queryController,
		"the query path must get its own controller, not the production one")
	require.False(t, res.KAppArgs.ReadOnly,
		"ArgsNewKApp is a value type: the query copy must not flip the shared args")
	require.False(t, res.KAppController.IsReadOnly(),
		"the production controller must stay writable")
	require.False(t, res.KAppControllerSimulator.IsReadOnly(),
		"the tx simulator controller must stay writable")
}

func getStateArgs() factory.StateComponentsFactoryArgs {
	return factory.StateComponentsFactoryArgs{
		PathManager: &mock.PathManagerStub{},
		Core:        getCoreComponents(),
		Tries:       getTriesComponents(),
		RatingsData: &mock.RatingsInfoMock{},
	}
}

func getTriesComponents() *factory.TriesComponents {
	tcf, _ := factory.NewTriesComponentsFactory(getTriesArgs())
	tc, _ := tcf.Create()
	return tc
}
