package vmhooks_test

import (
	"fmt"
	"math"
	"testing"

	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

// overflowStartingPosition + overflowLength overflows int32. Pre-fork this wraps
// negative and panics on the raw slice expression; post-fork it must be rejected
// gracefully. See FixAuditChangesV3 gating in manBufOps.go.
const overflowStartingPosition = int32(math.MaxInt32 - 5)
const overflowLength = int32(10)

func TestManagedBufferByteSlice_OverflowIsForkGated(t *testing.T) {
	t.Run("MBufferGetByteSlice", func(t *testing.T) {
		for _, fixAuditV3 := range []bool{false, true} {
			fixAuditV3 := fixAuditV3
			t.Run(fmt.Sprintf("FixAuditChangesV3=%t", fixAuditV3), func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(fixAuditV3)))
				require.NoError(t, err)
				hooks := vmhooks.NewVMHooksImpl(vmHost)
				provideGas(hooks, 1_000_000)

				mBuffer := hooks.GetManagedTypesContext().NewManagedBufferFromBytes([]byte("ABCDEFGHIJ"))

				if fixAuditV3 {
					result := hooks.MBufferGetByteSlice(mBuffer, overflowStartingPosition, overflowLength, 0)
					require.Equal(t, int32(1), result, "post-fork must reject the overflow gracefully")
				} else {
					require.Panics(t, func() {
						hooks.MBufferGetByteSlice(mBuffer, overflowStartingPosition, overflowLength, 0)
					}, "pre-fork must preserve the legacy panic-on-overflow behavior")
				}
			})
		}
	})

	t.Run("ManagedBufferCopyByteSliceWithHost", func(t *testing.T) {
		for _, fixAuditV3 := range []bool{false, true} {
			fixAuditV3 := fixAuditV3
			t.Run(fmt.Sprintf("FixAuditChangesV3=%t", fixAuditV3), func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(fixAuditV3)))
				require.NoError(t, err)
				hooks := vmhooks.NewVMHooksImpl(vmHost)
				provideGas(hooks, 1_000_000)

				managedType := hooks.GetManagedTypesContext()
				sourceMBuffer := managedType.NewManagedBufferFromBytes([]byte("ABCDEFGHIJ"))
				destMBuffer := managedType.NewManagedBufferFromBytes([]byte("0123456789"))

				if fixAuditV3 {
					result := vmhooks.ManagedBufferCopyByteSliceWithHost(
						vmHost, sourceMBuffer, overflowStartingPosition, overflowLength, destMBuffer)
					require.Equal(t, int32(1), result, "post-fork must reject the overflow gracefully")
				} else {
					require.Panics(t, func() {
						vmhooks.ManagedBufferCopyByteSliceWithHost(
							vmHost, sourceMBuffer, overflowStartingPosition, overflowLength, destMBuffer)
					}, "pre-fork must preserve the legacy panic-on-overflow behavior")
				}
			})
		}
	})

	t.Run("ManagedBufferSetByteSliceWithTypedArgs", func(t *testing.T) {
		for _, fixAuditV3 := range []bool{false, true} {
			fixAuditV3 := fixAuditV3
			t.Run(fmt.Sprintf("FixAuditChangesV3=%t", fixAuditV3), func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(newForkStub(fixAuditV3)))
				require.NoError(t, err)
				hooks := vmhooks.NewVMHooksImpl(vmHost)
				provideGas(hooks, 1_000_000)

				mBuffer := hooks.GetManagedTypesContext().NewManagedBufferFromBytes([]byte("ABCDEFGHIJ"))
				patchData := []byte("XY")

				if fixAuditV3 {
					result := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
						vmHost, mBuffer, overflowStartingPosition, overflowLength, patchData)
					require.Equal(t, int32(1), result, "post-fork must reject the overflow gracefully")
				} else {
					require.Panics(t, func() {
						vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
							vmHost, mBuffer, overflowStartingPosition, overflowLength, patchData)
					}, "pre-fork must preserve the legacy panic-on-overflow behavior")
				}
			})
		}
	})
}
