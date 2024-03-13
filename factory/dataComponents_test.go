package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/factory"
	"github.com/stretchr/testify/require"
)

func TestNewDataComponentsFactory_NilCoreComponentsShouldErr(t *testing.T) {
	t.Parallel()

	args := getDataArgs()
	args.Core = nil

	dcf, err := factory.NewDataComponentsFactory(args)
	require.Nil(t, dcf)
	require.Equal(t, factory.ErrNilCoreComponents, err)
}

func TestNewDataComponentsFactory_NilPathManagerShouldErr(t *testing.T) {
	t.Parallel()

	args := getDataArgs()
	args.PathManager = nil

	dcf, err := factory.NewDataComponentsFactory(args)
	require.Nil(t, dcf)
	require.Equal(t, factory.ErrNilPathManager, err)
}

func TestNewDataComponentsFactory_NilEpochStartNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := getDataArgs()
	args.EpochStartNotifier = nil

	dcf, err := factory.NewDataComponentsFactory(args)
	require.Nil(t, dcf)
	require.Equal(t, factory.ErrNilEpochStartNotifier, err)
}

func TestNewDataComponentsFactory_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	args := getDataArgs()

	dcf, err := factory.NewDataComponentsFactory(args)
	require.NoError(t, err)
	require.NotNil(t, dcf)
}

func TestDataComponentsFactory_CreateForMetaShouldWork(t *testing.T) {
	t.Parallel()

	args := getDataArgs()
	dcf, err := factory.NewDataComponentsFactory(args)
	require.NoError(t, err)
	dc, err := dcf.Create()
	require.NoError(t, err)
	require.NotNil(t, dc)
}

func getDataArgs() factory.DataComponentsFactoryArgs {
	return factory.DataComponentsFactoryArgs{
		Config:             mock.GetGeneralConfig(),
		Core:               getCoreComponents(),
		PathManager:        &mock.PathManagerStub{},
		EpochStartNotifier: &mock.EpochStartNotifierStub{},
		CurrentEpoch:       0,
	}
}

func getCoreComponents() *factory.CoreComponents {
	ccf := factory.NewCoreComponentsFactory(getCoreArgs())
	cc, _ := ccf.Create()
	return cc
}
