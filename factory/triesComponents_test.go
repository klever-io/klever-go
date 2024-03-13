package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/factory"
	"github.com/stretchr/testify/require"
)

func TestNewTriesComponentsFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := getTriesArgs()
	args.Marshalizer = nil
	tcf, err := factory.NewTriesComponentsFactory(args)
	require.Nil(t, tcf)
	require.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewTriesComponentsFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := getTriesArgs()
	args.Hasher = nil
	tcf, err := factory.NewTriesComponentsFactory(args)
	require.Nil(t, tcf)
	require.Equal(t, common.ErrNilHasher, err)
}

func TestNewTriesComponentsFactory_NilPathManagerShouldErr(t *testing.T) {
	t.Parallel()

	args := getTriesArgs()
	args.PathManager = nil
	tcf, err := factory.NewTriesComponentsFactory(args)
	require.Nil(t, tcf)
	require.Equal(t, factory.ErrNilPathManager, err)
}

func TestNewTriesComponentsFactory_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	args := getTriesArgs()
	tcf, err := factory.NewTriesComponentsFactory(args)
	require.NoError(t, err)
	require.NotNil(t, tcf)
}

func TestTriesComponentsFactory_Create(t *testing.T) {
	t.Parallel()

	args := getTriesArgs()
	tcf, _ := factory.NewTriesComponentsFactory(args)

	tc, err := tcf.Create()
	require.NoError(t, err)
	require.NotNil(t, tc)
}

func getTriesArgs() factory.TriesComponentsFactoryArgs {
	return factory.TriesComponentsFactoryArgs{
		Marshalizer: &mock.MarshalizerMock{},
		Hasher:      &mock.HasherMock{},
		PathManager: &mock.PathManagerStub{},
		Config:      mock.GetGeneralConfig(),
	}
}
