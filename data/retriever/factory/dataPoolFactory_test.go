package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDataPoolFromConfig(t *testing.T) {
	args := getGoodArgs()
	holder, err := NewDataPoolFromConfig(args)
	require.Nil(t, err)
	require.NotNil(t, holder)
}

func TestNewDataPoolFromConfig_MissingDependencyShouldErr(t *testing.T) {
	args := getGoodArgs()
	args.Config = nil
	holder, err := NewDataPoolFromConfig(args)
	require.Nil(t, holder)
	require.Equal(t, common.ErrNilConfig, err)
}

func getGoodArgs() ArgsDataPool {
	config := mock.GetGeneralConfig()

	return ArgsDataPool{
		Config: &config,
	}
}
