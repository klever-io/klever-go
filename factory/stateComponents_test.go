package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
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
