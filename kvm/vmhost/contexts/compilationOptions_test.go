package contexts

import (
	"math"
	"testing"

	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/require"
)

// KLC-3001: the runtime and the compiled-code warmer must build the same options from
// the same inputs, so the builder is shared and its output pinned here.
func TestBuildCompilationOptions(t *testing.T) {
	t.Parallel()

	gasCost, err := config.CreateGasConfig(config.MakeGasMapForTests())
	require.NoError(t, err)
	ops := gasCost.WASMOpcodeCost

	base := executor.CompilationOptions{
		GasLimit:           12345,
		UnmeteredLocals:    uint64(ops.LocalsUnmetered),
		MaxMemoryGrow:      uint64(ops.MaxMemoryGrow),
		MaxMemoryGrowDelta: uint64(ops.MaxMemoryGrowDelta),
		OpcodeTrace:        false,
		Metering:           true,
		RuntimeBreakpoints: true,
	}

	tests := []struct {
		name          string
		tableCheck    bool
		wantTableSize uint64
	}{
		{"before V4", false, math.MaxUint64},
		{"V4", true, uint64(ops.MaxDeclaredTableSize)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := base
			want.MaxDeclaredTableSize = tt.wantTableSize

			got := BuildCompilationOptions(gasCost, tt.tableCheck, 12345)
			require.Equal(t, want, got)
		})
	}
}
