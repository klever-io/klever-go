package vmhooks_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	twos "github.com/klever-io/klever-go/kvm/math/twos-complement"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	gasschedules "github.com/klever-io/klever-go/kvm/scenarioexec/gasSchedules"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// getBytesPtr is where the getters write the value in contract memory
	getBytesPtr = executor.MemPtr(64)
	// getBytesExtraPages grows the mock memory (2 pages of 64 KB) past getBytesPtr plus the
	// oversized value and its sign byte
	getBytesExtraPages = 17
)

// bigIntGetBytesGasMap is flatCloneGasMap with distinct fixed costs for the two getters, so a
// getter charging the other's cost fails
func bigIntGetBytesGasMap() config.GasScheduleMap {
	gasMap := flatCloneGasMap()
	gasMap["BigIntAPICost"]["BigIntGetUnsignedBytes"] = 83
	gasMap["BigIntAPICost"]["BigIntGetSignedBytes"] = 89
	return gasMap
}

type bigIntGetBytesHook struct {
	name      string
	fixedCost func(gasMap config.GasScheduleMap) uint64
	encode    func(*big.Int) []byte
	call      func(hooks *vmhooks.VMHooksImpl, handle int32, ptr executor.MemPtr) int32
}

func bigIntGetBytesHooks() []bigIntGetBytesHook {
	return []bigIntGetBytesHook{
		{
			name:      "BigIntGetUnsignedBytes",
			fixedCost: func(gasMap config.GasScheduleMap) uint64 { return gasMap["BigIntAPICost"]["BigIntGetUnsignedBytes"] },
			encode:    func(v *big.Int) []byte { return v.Bytes() },
			call: func(hooks *vmhooks.VMHooksImpl, handle int32, ptr executor.MemPtr) int32 {
				return hooks.BigIntGetUnsignedBytes(handle, ptr)
			},
		},
		{
			name:      "BigIntGetSignedBytes",
			fixedCost: func(gasMap config.GasScheduleMap) uint64 { return gasMap["BigIntAPICost"]["BigIntGetSignedBytes"] },
			encode:    twos.ToBytes,
			call: func(hooks *vmhooks.VMHooksImpl, handle int32, ptr executor.MemPtr) int32 {
				return hooks.BigIntGetSignedBytes(handle, ptr)
			},
		},
	}
}

// newBigIntGetBytesHost is a mock host on the real metering and managed types, priced by gasMap,
// whose contract memory holds the oversized value
func newBigIntGetBytesHost(t *testing.T, fixV6 bool, gasMap config.GasScheduleMap) (*contextmock.VMHostMock, *contextmock.RuntimeContextMock) {
	t.Helper()
	host, _ := newMockVMHostWithRealMetering(t, fixV6, gasMap, flatCloneGasProvided)
	runtime := host.RuntimeContext.(*contextmock.RuntimeContextMock)
	require.NoError(t, runtime.GetInstance().MemGrow(getBytesExtraPages))
	return host, runtime
}

// BigIntGetUnsignedBytes and BigIntGetSignedBytes serialized the value, wrote it to contract memory
// and then charged DataCopyPerByte for its bytes, unbounded. From FixAuditChangesV6 the fixed cost is
// bounded and the byte length, computed without serializing, is charged bounded first: an underfunded
// call fails with ErrNotEnoughGas without serializing or writing anything. The totals stay as they
// were. Not parallel: it counts allocations
func TestVMHooksImpl_BigIntGetBytes_ChargedBeforeSerializingFromV6(t *testing.T) {
	gasMap := bigIntGetBytesGasMap()
	perByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	require.NotZero(t, perByte)
	oversized := new(big.Int).SetBytes(bytes.Repeat([]byte{0xAB}, flatCloneOversizedLen))
	max32 := new(big.Int).SetBytes(bytes.Repeat([]byte{0xFF}, 32))
	min33 := new(big.Int).Lsh(big.NewInt(1), 32*8)

	type scenario struct {
		name      string
		value     *big.Int
		missing   bool
		ptr       executor.MemPtr
		badPtr    bool
		budget    uint64
		failErr   error
		cloned    bool
		notCloned bool
	}

	for _, hook := range bigIntGetBytesHooks() {
		fixedCost := hook.fixedCost(gasMap)
		success := func(value *big.Int) uint64 {
			return fixedCost + perByte*uint64(len(hook.encode(value)))
		}
		for _, fixV6 := range []bool{false, true} {
			scenarios := []scenario{
				{name: "zero", value: big.NewInt(0)},
				{name: "small", value: big.NewInt(0x7F12)},
				{name: "signBoundary", value: big.NewInt(0x80)},
				{name: "negative", value: big.NewInt(-0x7FFF)},
				{name: "negativePowerOfTwo", value: big.NewInt(-0x8000)},
				{name: "max32Bytes", value: max32},
				{name: "negativeMax32Bytes", value: new(big.Int).Neg(max32)},
				{name: "min33Bytes", value: min33},
				{name: "negativeMin33Bytes", value: new(big.Int).Neg(min33)},
				{name: "oversized", value: oversized, cloned: true},
				{name: "negativeOversized", value: new(big.Int).Neg(oversized), cloned: true},
				{name: "missing", missing: true, budget: fixedCost, failErr: vmhost.ErrNoBigIntUnderThisHandle},
			}
			if fixV6 {
				scenarios = append(scenarios,
					scenario{name: "fixedShortfall", value: oversized, budget: fixedCost - 1,
						failErr: vmhost.ErrNotEnoughGas, notCloned: true},
					scenario{name: "bytesShortfall", value: oversized, budget: success(oversized) - 1,
						failErr: vmhost.ErrNotEnoughGas, notCloned: true},
					// the bytes are now paid for before the write fails, but the fault burns the gas left on
					// both sides, so the total is the budget either way
					scenario{name: "memStoreFails", value: max32, badPtr: true, failErr: executor.ErrMemoryBadBoundsUpper},
				)
			} else {
				// pre-fork the charges overdraw and the hook carries on
				scenarios = append(scenarios,
					scenario{name: "fixedShortfall", value: oversized, budget: fixedCost - 1, cloned: true},
					scenario{name: "bytesShortfall", value: oversized, budget: success(oversized) - 1, cloned: true},
					scenario{name: "memStoreFails", value: max32, badPtr: true, failErr: executor.ErrMemoryBadBoundsUpper},
				)
			}

			for _, sc := range scenarios {
				t.Run(fmt.Sprintf("%s/FixAuditChangesV6=%v/%s", hook.name, fixV6, sc.name), func(t *testing.T) {
					host, runtime := newBigIntGetBytesHost(t, fixV6, gasMap)
					instance := runtime.GetInstance()
					handle := int32(1234)
					if !sc.missing {
						handle = host.ManagedTypes().NewBigInt(sc.value)
					}
					ptr := getBytesPtr
					if sc.badPtr {
						ptr = executor.MemPtr(instance.MemLength() - 10) // #nosec G115
					}
					budget := sc.budget
					if budget == 0 {
						budget = success(oversized) + 1_000_000
					}

					var result int32
					gasUsed, allocated := runFlatCloneHook(host, budget, func() {
						result = hook.call(vmhooks.NewVMHooksImpl(host), handle, ptr)
					})

					if sc.failErr != nil {
						require.ErrorIs(t, runtime.FailExecutionErr, sc.failErr)
						assert.Equal(t, int32(-1), result)
						// a fault burns the gas left
						assert.Equal(t, budget, gasUsed)
						if !sc.missing && !sc.badPtr {
							encodedLength := len(hook.encode(sc.value))
							written, err := instance.MemLoad(ptr, executor.MemLength(encodedLength)) // #nosec G115
							require.NoError(t, err)
							require.Len(t, written, encodedLength)
							assert.True(t, bytes.Equal(make([]byte, len(written)), written),
								"contract memory must be untouched")
						}
					} else {
						require.Nil(t, runtime.FailExecutionErr)
						expected := hook.encode(sc.value)
						require.Equal(t, int32(len(expected)), result) // #nosec G115
						stored, err := instance.MemLoad(ptr, result)
						require.NoError(t, err)
						assert.True(t, bytes.Equal(expected, stored), "contract memory must hold the encoded value")
						if sc.budget == 0 {
							assert.Equal(t, success(sc.value), gasUsed)
						}
					}
					if sc.cloned {
						assert.GreaterOrEqual(t, allocated, uint64(flatCloneOversizedLen))
					}
					if sc.notCloned {
						assert.Less(t, allocated, uint64(flatCloneNoCloneBelow), "the value must not be serialized")
					}
				})
			}
		}
	}
}

// With the real gas schedule, the getters charge the same on both sides of FixAuditChangesV6: the
// fixed cost plus DataCopyPerByte for each byte written. Not parallel: the other tests count
// allocations
func TestVMHooksImpl_BigIntGetBytes_RealScheduleSameBeforeAndFromV6(t *testing.T) {
	gasMap, err := gasschedules.LoadGasScheduleConfig(gasschedules.GetV1())
	require.NoError(t, err)
	perByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	require.NotZero(t, perByte)

	values := map[string]*big.Int{
		"small":             big.NewInt(0x7F12),
		"negative":          big.NewInt(-0x8001),
		"min33Bytes":        new(big.Int).Lsh(big.NewInt(1), 32*8),
		"oversized":         new(big.Int).SetBytes(bytes.Repeat([]byte{0xAB}, flatCloneOversizedLen)),
		"negativeOversized": new(big.Int).SetBytes(bytes.Repeat([]byte{0xAB}, flatCloneOversizedLen)),
	}
	values["negativeOversized"].Neg(values["negativeOversized"])

	for _, hook := range bigIntGetBytesHooks() {
		fixedCost := hook.fixedCost(gasMap)
		require.NotZero(t, fixedCost)
		for name, value := range values {
			t.Run(hook.name+"/"+name, func(t *testing.T) {
				expected := fixedCost + perByte*uint64(len(hook.encode(value)))
				gasUsed := make(map[bool]uint64)
				for _, fixV6 := range []bool{false, true} {
					host, runtime := newBigIntGetBytesHost(t, fixV6, gasMap)
					handle := host.ManagedTypes().NewBigInt(value)
					used, _ := runFlatCloneHook(host, expected, func() {
						hook.call(vmhooks.NewVMHooksImpl(host), handle, getBytesPtr)
					})
					require.Nil(t, runtime.FailExecutionErr, "FixAuditChangesV6=%v", fixV6)
					gasUsed[fixV6] = used
				}
				assert.Equal(t, expected, gasUsed[false], "pre-fork")
				assert.Equal(t, expected, gasUsed[true], "from FixAuditChangesV6")
				t.Logf("%s %s: %d bytes, gas %d before and %d from FixAuditChangesV6",
					hook.name, name, len(hook.encode(value)), gasUsed[false], gasUsed[true])
			})
		}
	}
}
