package hostCoretest

import (
	"encoding/binary"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kvm/mock/contracts"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

func decodeSignersDoSPayload(signersLen uint32) []byte {
	payload := make([]byte, 0, 25)
	payload = binary.BigEndian.AppendUint32(payload, 1)
	payload = append(payload, 0)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint32(payload, signersLen)
	return payload
}

func decodeSignersDoSAddress(label string) []byte {
	address := make([]byte, 32)
	copy(address, label)
	return address
}

func TestUpdateAccountPermissionDecodeSignersRejectsOversizedSignerCountWithoutLargeAlloc(t *testing.T) {
	const declaredSigners = uint32(5_000_000)

	oldGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGCPercent)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithConfig(&test.TestConfig{
					GasProvidedToChild: 1_000_000,
				}).
				WithMethods(contracts.ExecOnDestCtxSingleCallParentMock),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithFunction("execOnDestCtxSingleCall").
			WithGasProvided(2_000_000).
			WithArguments(
				decodeSignersDoSAddress("recipient"),
				[]byte(core.BuiltInFunctionUpdateAccountPermission),
				decodeSignersDoSAddress("target-account"),
				decodeSignersDoSPayload(declaredSigners),
			).
			Build()).
		WithSetup(func(host vmhost.VMHost, world *worldmock.MockWorld) {
			require.NoError(t, world.InitBuiltinFunctions(host.GetGasScheduleMap(), host.ForkController()))
			host.SetBuiltInFunctionsContainer(world.BuiltinFuncs.Container)
		}).
		AndAssertResults(func(_ *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			require.NotEqual(t, vmcommon.Ok, verify.VmOutput.ReturnCode)
		})
	require.NoError(t, err)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf(
		"payload_len=%d declared_signers=%d total_alloc_delta=%d",
		len(decodeSignersDoSPayload(declaredSigners)),
		declaredSigners,
		after.TotalAlloc-before.TotalAlloc,
	)
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(declaredSigners)*8)
}
