package warmup

import (
	"reflect"
	"runtime"
	"testing"

	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	"github.com/stretchr/testify/require"
)

// gasScheduleWithI64AddCost returns a fresh schedule that differs from the test schedule
// only in the I64Add opcode cost.
func gasScheduleWithI64AddCost(delta uint64) map[string]map[string]uint64 {
	schedule := kvmConfig.MakeGasMapForTests()
	schedule["WASMOpcodeCost"]["I64Add"] += delta
	return schedule
}

// TestCompiledCodeWarmer_FingerprintNotFooledByReusedScheduleAddress reproduces the hazard of
// memoizing the derived opcode costs by the schedule's address alone: once the notifier drops
// a schedule, the garbage collector may hand its address to a later map with different costs.
// If the memo does not keep the map alive, such a map is served the old costs and a module
// compiled under them passes the fingerprint recheck.
func TestCompiledCodeWarmer_FingerprintNotFooledByReusedScheduleAddress(t *testing.T) {
	f := newWarmerFixture(t)
	w := f.newStopped(t, 4)

	scheduleA := gasScheduleWithI64AddCost(0)
	addressA := reflect.ValueOf(scheduleA).Pointer()
	f.gas.GasSchedule = scheduleA
	fingerprintA, err := w.currentFingerprint()
	require.NoError(t, err)

	// the notifier moves on: nothing but the warmer may still reference schedule A
	f.gas.GasSchedule = nil
	scheduleA = nil //nolint:ineffassign,wastedassign // drop the last test reference on purpose
	_ = scheduleA

	alive := make([]map[string]map[string]uint64, 0, 1<<16)
	var reused map[string]map[string]uint64
	for round := 0; round < 50 && reused == nil; round++ {
		runtime.GC()
		for i := 0; i < 1000; i++ {
			candidate := make(map[string]map[string]uint64)
			if reflect.ValueOf(candidate).Pointer() == addressA {
				reused = candidate
				break
			}
			alive = append(alive, candidate)
		}
	}
	runtime.KeepAlive(alive)

	if reused != nil {
		// a later schedule really landed on A's address: it must still get its own costs
		for key, values := range gasScheduleWithI64AddCost(1) {
			reused[key] = values
		}
		f.gas.GasSchedule = reused

		fingerprintReused, errReused := w.currentFingerprint()
		require.NoError(t, errReused)
		require.Equal(t, fingerprintA.opcodes.I64Add+1, fingerprintReused.opcodes.I64Add,
			"a schedule allocated at a dropped schedule's address was served the dropped schedule's costs")
	}
	require.Nil(t, reused, "the memo must keep its schedule alive, so its address cannot be reused")
}

// TestCompiledCodeWarmer_FingerprintFollowsScheduleReplacement checks the ordinary case: a
// replaced schedule with different costs yields a different fingerprint, and returning to a
// schedule already seen yields its costs again.
func TestCompiledCodeWarmer_FingerprintFollowsScheduleReplacement(t *testing.T) {
	t.Parallel()

	f := newWarmerFixture(t)
	w := f.newStopped(t, 4)

	scheduleA := gasScheduleWithI64AddCost(0)
	scheduleB := gasScheduleWithI64AddCost(1)

	f.gas.GasSchedule = scheduleA
	fingerprintA, err := w.currentFingerprint()
	require.NoError(t, err)

	f.gas.GasSchedule = scheduleB
	fingerprintB, err := w.currentFingerprint()
	require.NoError(t, err)
	require.NotEqual(t, fingerprintA.opcodes, fingerprintB.opcodes)
	require.Equal(t, fingerprintA.opcodes.I64Add+1, fingerprintB.opcodes.I64Add)

	f.gas.GasSchedule = scheduleA
	fingerprintAgain, err := w.currentFingerprint()
	require.NoError(t, err)
	require.Equal(t, fingerprintA, fingerprintAgain)
}
