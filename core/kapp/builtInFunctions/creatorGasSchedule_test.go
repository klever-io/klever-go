package builtInFunctions_test

import (
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	vmconfig "github.com/klever-io/klever-go/kvm/config"
	"github.com/stretchr/testify/require"
)

func TestCheckGasSchedule(t *testing.T) {
	t.Parallel()

	require.NoError(t, builtInFunctions.CheckGasSchedule(vmconfig.MakeGasMapForTests()))

	missingBuiltIn := vmconfig.MakeGasMapForTests()
	delete(missingBuiltIn[core.BuiltInCostString], "Transfer")
	require.Error(t, builtInFunctions.CheckGasSchedule(missingBuiltIn))

	zeroBaseOperation := vmconfig.MakeGasMapForTests()
	zeroBaseOperation[core.BaseOperationCostString]["StorePerByte"] = 0
	require.Error(t, builtInFunctions.CheckGasSchedule(zeroBaseOperation))

	require.Error(t, builtInFunctions.CheckGasSchedule(map[string]map[string]uint64{}))
}
