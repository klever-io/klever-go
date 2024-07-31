package factory

import (
	"fmt"
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

func TestNewDataPoolFromConfig_BadConfigShouldErr(t *testing.T) {
	// We test one (arbitrary and trivial) erroneous config for each component that needs to be created
	args := getGoodArgs()
	args.Config.TxDataPool.Capacity = 0
	holder, err := NewDataPoolFromConfig(args)
	require.Nil(t, holder)
	require.NotNil(t, err)

	args = getGoodArgs()
	args.Config.HeadersPoolConfig.MaxHeadersPerShard = 0
	holder, err = NewDataPoolFromConfig(args)
	require.Nil(t, holder)
	fmt.Println(err)
	require.NotNil(t, err)

	args = getGoodArgs()
	args.Config.TrieNodesDataPool.Capacity = 0
	holder, err = NewDataPoolFromConfig(args)
	require.Nil(t, holder)
	require.NotNil(t, err)
}

func getGoodArgs() ArgsDataPool {
	config := mock.GetGeneralConfig()

	return ArgsDataPool{
		Config: &config,
	}
}
