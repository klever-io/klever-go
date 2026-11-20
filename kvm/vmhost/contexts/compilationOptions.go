package contexts

import (
	builtinMath "math"

	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
)

// BuildCompilationOptions builds the options handed to the executor when a contract is
// compiled or restored from compiled code. It is the single source for those options:
// the runtime uses it on both instantiation paths, and the KLC-3001 compiled-code warmer
// uses it to precompile modules that the runtime later restores, so the two can never
// disagree about how the same contract is compiled and metered.
//
// tableCheckActive is FixAuditChangesV4 (declared-table-size cap) and
// tableInitPerElementActive is FixAuditChangesV5 (per-element table.init charge).
func BuildCompilationOptions(
	gasSchedule *config.GasCost,
	tableCheckActive bool,
	tableInitPerElementActive bool,
	gasLimit uint64,
) executor.CompilationOptions {
	return executor.CompilationOptions{
		GasLimit:                gasLimit,
		UnmeteredLocals:         uint64(gasSchedule.WASMOpcodeCost.LocalsUnmetered),
		MaxMemoryGrow:           uint64(gasSchedule.WASMOpcodeCost.MaxMemoryGrow),
		MaxMemoryGrowDelta:      uint64(gasSchedule.WASMOpcodeCost.MaxMemoryGrowDelta),
		MaxDeclaredTableSize:    effectiveMaxDeclaredTableSize(gasSchedule, tableCheckActive),
		TableInitPerElementCost: effectiveTableInitPerElementCost(gasSchedule, tableInitPerElementActive),
		OpcodeTrace:             false,
		Metering:                true,
		RuntimeBreakpoints:      true,
	}
}

// effectiveMaxDeclaredTableSize returns the table-maximum cap to pass to the
// executor for this instantiation. The executor has no fork-epoch awareness
// of its own (see KLC-2526 / KLR-19): it always enforces whatever value it is
// given, on every instantiation, which is what makes it safe to call
// unconditionally without risking a consensus split during rollout - nodes
// on the old binary never see this parameter at all, and nodes on the new
// binary only start passing the real cap once FixAuditChangesV4 is reached
// network-wide. Before that, math.MaxUint64 disables the check entirely.
func effectiveMaxDeclaredTableSize(gasSchedule *config.GasCost, tableCheckActive bool) uint64 {
	if !tableCheckActive {
		return builtinMath.MaxUint64
	}
	return uint64(gasSchedule.WASMOpcodeCost.MaxDeclaredTableSize)
}

// effectiveTableInitPerElementCost returns the per-element table.init price to
// hand the executor for this instantiation. 0 means "charge the flat opcode cost
// only", which is exactly the pre-fork behaviour, so the executor stays
// unconditional and fork-unaware just as it does for the table cap.
func effectiveTableInitPerElementCost(gasSchedule *config.GasCost, tableInitPerElementActive bool) uint64 {
	if !tableInitPerElementActive {
		return 0
	}
	return uint64(gasSchedule.WASMOpcodeCost.TableInitPerElement)
}
