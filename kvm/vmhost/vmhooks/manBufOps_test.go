package vmhooks_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kvm/executor"
	twos "github.com/klever-io/klever-go/kvm/math/twos-complement"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

func TestMBufferToBigFloat(t *testing.T) {
	t.Run("Canonical encoding succeeds", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		canonical, err := big.NewFloat(1.5).GobEncode()
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(canonical)
		bigFloatHandle := int32(100)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(0), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(1.5).Cmp(result))
	})

	// A canonical prec-53 encoding of 1.5 packs its mantissa into a single
	// 64-bit word, of which only the top 53 bits are significant. Appending
	// an extra mantissa word still has a valid prefix and still GobDecodes
	// successfully, but MinPrec() on the decoded value exceeds 53, which is
	// exactly the non-canonical encoding the check rejects.
	t.Run("Non-canonical encoding faults", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		canonical, err := big.NewFloat(1.5).GobEncode()
		require.NoError(t, err)
		nonCanonical := append(append([]byte{}, canonical...), 0, 0, 0, 0, 0, 0, 0, 1)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(101)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})

	// Stuffing non-zero bits into the low, supposedly-unused bits of that same
	// single mantissa word (rather than appending a whole extra word) changes
	// the represented value without changing byte length or declared
	// precision: GobDecode copies the word verbatim and GobEncode reproduces
	// it unchanged, so a raw byte round-trip can't see it. MinPrec() can: it
	// reports the true bits needed to represent the value exactly (64 here),
	// which exceeds 53 even though Prec() still reports 53.
	t.Run("Excess mantissa bits within a single word fault", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		nonCanonical, err := hex.DecodeString("010a0000003500000001c00000000000ffff")
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(103)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})

	t.Run("Pre-fork: excess mantissa bits within a single word succeeds", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		fc := commonMock.NewForkControllerStub()
		fc.FixAuditChangesV4Value = false
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParametersWithFork(fc))
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		nonCanonical, err := hex.DecodeString("010a0000003500000001c00000000000ffff")
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(nonCanonical)
		bigFloatHandle := int32(104)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(0), ret)
	})

	t.Run("Exponent too big faults", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		outOfRange := new(big.Float).SetPrec(53)
		outOfRange.SetMantExp(big.NewFloat(0.5), 65026)
		encoded, err := outOfRange.GobEncode()
		require.NoError(t, err)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(encoded)
		bigFloatHandle := int32(102)

		ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)

		require.Equal(t, int32(1), ret)
		result, err := hooks.GetManagedTypesContext().GetBigFloat(bigFloatHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewFloat(0).Cmp(result), "destination must be left untouched (still the zero value GetBigFloatOrCreate seeded)")
	})
}

// maxManagedBufferLength mirrors the cap the hooks enforce on a caller-supplied size
const maxManagedBufferLength = 16_000_000

const overMaxManagedBufferLength = maxManagedBufferLength + 1

var manBufTestContractCode = []byte("manBufOpsTestContract...........")

var manBufTestArgument = []byte("abcdefgh")

func newManBufHooks(t *testing.T, fixAuditChangesV5 bool) *vmhooks.VMHooksImpl {
	t.Helper()

	mockWorld := worldmock.NewMockWorld()
	fc := commonMock.NewForkControllerStub()
	fc.FixAuditChangesV5Value = fixAuditChangesV5

	hostParameters := makeHostParametersWithFork(fc)
	executorFactory := contextmock.NewExecutorMockFactory(mockWorld)
	hostParameters.OverrideVMExecutor = executorFactory

	vmHost, err := hostCore.NewVMHost(mockWorld, hostParameters)
	require.NoError(t, err)

	executorFactory.LastCreatedExecutor.CreateAndStoreInstanceMock(
		t, vmHost, manBufTestContractCode, nil, nil, nil, 0, true)

	vmHost.Runtime().InitStateFromContractCallInput(&vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			Arguments:   [][]byte{manBufTestArgument},
			CallerAddr:  make([]byte, len(manBufTestContractCode)),
			GasProvided: 1_000_000,
		},
		RecipientAddr: manBufTestContractCode,
		Function:      "testFunction",
	})
	require.NoError(t, vmHost.Runtime().StartWasmerInstance(
		contextmock.MockContractCode(manBufTestContractCode), 1_000_000, true))
	vmHost.Storage().SetAddress(manBufTestContractCode)

	return vmhooks.NewVMHooksImpl(vmHost)
}

func requireNotEnoughGas(t *testing.T, hooks *vmhooks.VMHooksImpl) {
	t.Helper()
	require.ErrorIs(t, hooks.GetRuntimeContext().GetAllErrors(), vmhost.ErrNotEnoughGas)
}

func requireLengthExceedsMaximum(t *testing.T, hooks *vmhooks.VMHooksImpl) {
	t.Helper()
	require.ErrorIs(t, hooks.GetRuntimeContext().GetAllErrors(), vmhost.ErrManagedBufferLengthExceedsMaximum)
}

func writeToMemory(t *testing.T, hooks *vmhooks.VMHooksImpl, offset executor.MemPtr, data []byte) {
	t.Helper()
	require.NoError(t, hooks.MemStore(offset, data))
}

func readFromMemory(t *testing.T, hooks *vmhooks.VMHooksImpl, offset executor.MemPtr, length int) []byte {
	t.Helper()
	data, err := hooks.MemLoad(offset, executor.MemLength(length))
	require.NoError(t, err)
	return data
}

func resetGas(hooks *vmhooks.VMHooksImpl, gas uint64) {
	hooks.GetRuntimeContext().SetPointsUsed(0)
	provideGas(hooks, gas)
}

func gasConsumedBy(hooks *vmhooks.VMHooksImpl, call func()) uint64 {
	metering := hooks.GetMeteringContext()
	before := metering.GasLeft()
	call()

	return before - metering.GasLeft()
}

// manBufVerifyFunc checks the state a hook leaves behind: done reports whether the hook's
// work is expected to have happened (the buffer written, the value parsed, the memory copied)
// or to have been prevented entirely.
type manBufVerifyFunc func(t *testing.T, done bool)

// manBufGasCase describes one managed-buffer hook whose size-dependent gas must be charged,
// bounded, before the host work it pays for. Every gas amount is in units of the test gas
// schedule, which prices every flat cost and every byte at 1.
type manBufGasCase struct {
	name string
	// preForkGas is the pre-fork cost: it covers everything the legacy path charges with a
	// bounded helper, but not the per-byte charges this fix adds. Post-fork the hook must
	// reject it; pre-fork it proceeds. The pair at identical gas is what proves the gate
	// is not inert.
	preForkGas uint64
	// partialGas budgets cover some but not all of the post-fork per-byte charges, so the
	// hook must still reject them post-fork
	partialGas []uint64
	// postForkGas covers every post-fork charge, so the hook proceeds
	postForkGas uint64
	failRet     int32
	okRet       int32
	// okRetIsHandle marks hooks that return a fresh handle on success rather than a code
	okRetIsHandle bool
	// run sets the hook's inputs up on a fresh host, funds it with gas, invokes the hook and
	// returns its return code plus a check of the state it left behind
	run func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc)
}

func (c manBufGasCase) requireProceeded(t *testing.T, ret int32) {
	t.Helper()
	if c.okRetIsHandle {
		require.GreaterOrEqual(t, ret, int32(0))
		return
	}
	require.Equal(t, c.okRet, ret)
}

func runManBufGasCases(t *testing.T, cases []manBufGasCase) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("post-fork: gas covering only the pre-fork cost is rejected before the work", func(t *testing.T) {
				hooks := newManBufHooks(t, true)
				ret, verify := c.run(t, hooks, c.preForkGas)

				require.Equal(t, c.failRet, ret)
				requireNotEnoughGas(t, hooks)
				verify(t, false)
			})

			for _, gas := range c.partialGas {
				t.Run(fmt.Sprintf("post-fork: gas of %d covering only part of the per-byte charges is rejected", gas), func(t *testing.T) {
					hooks := newManBufHooks(t, true)
					ret, verify := c.run(t, hooks, gas)

					require.Equal(t, c.failRet, ret)
					requireNotEnoughGas(t, hooks)
					verify(t, false)
				})
			}

			t.Run("pre-fork: gas covering only the pre-fork cost still proceeds", func(t *testing.T) {
				hooks := newManBufHooks(t, false)
				ret, verify := c.run(t, hooks, c.preForkGas)

				c.requireProceeded(t, ret)
				verify(t, true)
			})

			t.Run("post-fork: gas covering the per-byte charges proceeds", func(t *testing.T) {
				hooks := newManBufHooks(t, true)
				ret, verify := c.run(t, hooks, c.postForkGas)

				c.requireProceeded(t, ret)
				require.NoError(t, hooks.GetRuntimeContext().GetAllErrors())
				verify(t, true)
			})
		})
	}
}

func requireBufferEquals(t *testing.T, hooks *vmhooks.VMHooksImpl, handle int32, expected []byte) {
	t.Helper()
	result, err := hooks.GetManagedTypesContext().GetBytes(handle)
	require.NoError(t, err)
	if len(expected) == 0 {
		require.Empty(t, result)
		return
	}
	require.Equal(t, expected, result)
}

func TestManagedBufferHooksChargePerByteBeforeTheWork(t *testing.T) {
	data := []byte("abcdefgh")
	half := []byte("abcd")
	marker := []byte("........")
	memOffset := executor.MemPtr(0)
	resultOffset := executor.MemPtr(64)
	bigIntValue := big.NewInt(0x01020304)
	bigIntChargedBytes := uint64(bigIntValue.BitLen())/8 + 1
	encodedFloat, err := big.NewFloat(1.5).GobEncode()
	require.NoError(t, err)
	const randomLength = int32(64)

	// pick returns expected when the work happened and otherwise, so the verify closures read
	// as "buffer holds X after the call, Y when the call was prevented"
	pick := func(done bool, expected, otherwise []byte) []byte {
		if done {
			return expected
		}
		return otherwise
	}

	runManBufGasCases(t, []manBufGasCase{
		{
			name:          "MBufferNewFromBytes charges per byte before the memory load",
			preForkGas:    1,
			postForkGas:   1 + uint64(len(data)),
			failRet:       -1,
			okRetIsHandle: true,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, memOffset, data)
				provideGas(hooks, gas)
				ret := hooks.MBufferNewFromBytes(memOffset, executor.MemLength(len(data)))
				return ret, func(t *testing.T, done bool) {
					if done {
						requireBufferEquals(t, hooks, ret, data)
					}
				}
			},
		},
		{
			name:        "MBufferGetBytes charges per byte before the clone",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(data)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, resultOffset, marker)
				sourceHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(data)
				provideGas(hooks, gas)
				ret := hooks.MBufferGetBytes(sourceHandle, resultOffset)
				return ret, func(t *testing.T, done bool) {
					require.Equal(t, pick(done, data, marker), readFromMemory(t, hooks, resultOffset, len(data)))
				}
			},
		},
		{
			name:        "MBufferGetByteSlice charges per byte before the clone",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(data)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, resultOffset, marker[:len(half)])
				sourceHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(data)
				provideGas(hooks, gas)
				ret := hooks.MBufferGetByteSlice(sourceHandle, 0, int32(len(half)), resultOffset)
				return ret, func(t *testing.T, done bool) {
					require.Equal(t, pick(done, half, marker[:len(half)]), readFromMemory(t, hooks, resultOffset, len(half)))
				}
			},
		},
		{
			name:        "MBufferSetBytes charges per byte before the memory load",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(data)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, memOffset, data)
				mBufferHandle := hooks.GetManagedTypesContext().NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferSetBytes(mBufferHandle, memOffset, executor.MemLength(len(data)))
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, mBufferHandle, pick(done, data, nil))
				}
			},
		},
		{
			name:        "MBufferSetByteSlice charges the data and the destination before the memory load",
			preForkGas:  1,
			partialGas:  []uint64{1 + uint64(len(half))},
			postForkGas: 1 + uint64(len(half)) + uint64(len(marker)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, memOffset, half)
				mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(marker)
				provideGas(hooks, gas)
				ret := hooks.MBufferSetByteSlice(mBufferHandle, 2, executor.MemLength(len(half)), memOffset)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, mBufferHandle, pick(done, []byte("..abcd.."), marker))
				}
			},
		},
		{
			name:        "MBufferAppendBytes charges the data and the accumulator before the memory load",
			preForkGas:  1,
			partialGas:  []uint64{1 + uint64(len(half))},
			postForkGas: 1 + 2*uint64(len(half)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				writeToMemory(t, hooks, memOffset, []byte("efgh"))
				accumulatorHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(half)
				provideGas(hooks, gas)
				ret := hooks.MBufferAppendBytes(accumulatorHandle, memOffset, executor.MemLength(len(half)))
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, accumulatorHandle, pick(done, data, half))
				}
			},
		},
		{
			name:        "MBufferCopyByteSlice charges the source and the copy before the write",
			preForkGas:  1,
			partialGas:  []uint64{1 + uint64(len(data))},
			postForkGas: 1 + uint64(len(data)) + uint64(len(half)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				sourceHandle := managedType.NewManagedBufferFromBytes(data)
				destinationHandle := managedType.NewManagedBufferFromBytes(marker)
				provideGas(hooks, gas)
				ret := vmhooks.ManagedBufferCopyByteSliceWithHost(hooks.GetVMHost(), sourceHandle, 0, int32(len(half)), destinationHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, destinationHandle, pick(done, half, marker))
				}
			},
		},
		{
			name:        "MBufferEq charges both buffers before the clones",
			preForkGas:  1,
			partialGas:  []uint64{1 + uint64(len(data))},
			postForkGas: 1 + 2*uint64(len(data)),
			failRet:     -1,
			okRet:       1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				firstHandle := managedType.NewManagedBufferFromBytes(data)
				secondHandle := managedType.NewManagedBufferFromBytes(data)
				provideGas(hooks, gas)
				return hooks.MBufferEq(firstHandle, secondHandle), func(*testing.T, bool) {}
			},
		},
		{
			name:        "MBufferAppend charges the data and the accumulator before the append",
			preForkGas:  1,
			partialGas:  []uint64{1 + uint64(len(half))},
			postForkGas: 1 + 2*uint64(len(half)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				accumulatorHandle := managedType.NewManagedBufferFromBytes(half)
				dataHandle := managedType.NewManagedBufferFromBytes([]byte("efgh"))
				provideGas(hooks, gas)
				ret := hooks.MBufferAppend(accumulatorHandle, dataHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, accumulatorHandle, pick(done, data, half))
				}
			},
		},
		{
			name:        "MBufferToBigIntUnsigned charges per byte before the parse",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(half)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				mBufferHandle := managedType.NewManagedBufferFromBytes(bigIntValue.Bytes())
				bigIntHandle := managedType.NewBigIntFromInt64(0)
				provideGas(hooks, gas)
				ret := hooks.MBufferToBigIntUnsigned(mBufferHandle, bigIntHandle)
				return ret, func(t *testing.T, done bool) {
					value, err := managedType.GetBigInt(bigIntHandle)
					require.NoError(t, err)
					require.Equal(t, int64(pick2(done, int(bigIntValue.Int64()), 0)), value.Int64())
				}
			},
		},
		{
			name:        "MBufferToBigIntSigned charges per byte before the parse",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(half)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				mBufferHandle := managedType.NewManagedBufferFromBytes(twos.ToBytes(bigIntValue))
				bigIntHandle := managedType.NewBigIntFromInt64(0)
				provideGas(hooks, gas)
				ret := hooks.MBufferToBigIntSigned(mBufferHandle, bigIntHandle)
				return ret, func(t *testing.T, done bool) {
					value, err := managedType.GetBigInt(bigIntHandle)
					require.NoError(t, err)
					require.Equal(t, int64(pick2(done, int(bigIntValue.Int64()), 0)), value.Int64())
				}
			},
		},
		{
			name:        "MBufferFromBigIntUnsigned charges per byte before the write",
			preForkGas:  1,
			postForkGas: 1 + bigIntChargedBytes,
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				bigIntHandle := managedType.NewBigInt(bigIntValue)
				mBufferHandle := managedType.NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferFromBigIntUnsigned(mBufferHandle, bigIntHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, mBufferHandle, pick(done, bigIntValue.Bytes(), nil))
				}
			},
		},
		{
			name:        "MBufferFromBigIntSigned charges per byte before the write",
			preForkGas:  1,
			postForkGas: 1 + bigIntChargedBytes,
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				bigIntHandle := managedType.NewBigInt(bigIntValue)
				mBufferHandle := managedType.NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferFromBigIntSigned(mBufferHandle, bigIntHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, mBufferHandle, pick(done, twos.ToBytes(bigIntValue), nil))
				}
			},
		},
		{
			name:        "MBufferToBigFloat charges per byte before the decode",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(encodedFloat)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				mBufferHandle := managedType.NewManagedBufferFromBytes(encodedFloat)
				bigFloatHandle := int32(200)
				provideGas(hooks, gas)
				ret := hooks.MBufferToBigFloat(mBufferHandle, bigFloatHandle)
				return ret, func(t *testing.T, done bool) {
					if !done {
						return
					}
					value, err := managedType.GetBigFloat(bigFloatHandle)
					require.NoError(t, err)
					require.Equal(t, 0, big.NewFloat(1.5).Cmp(value))
				}
			},
		},
		{
			name:        "MBufferFromBigFloat charges per byte before the write",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(encodedFloat)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				managedType := hooks.GetManagedTypesContext()
				bigFloatHandle, err := managedType.PutBigFloat(big.NewFloat(1.5))
				require.NoError(t, err)
				mBufferHandle := managedType.NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferFromBigFloat(mBufferHandle, bigFloatHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, mBufferHandle, pick(done, encodedFloat, nil))
				}
			},
		},
		{
			name:        "MBufferSetRandom charges per byte before the allocation",
			preForkGas:  1,
			postForkGas: 1 + uint64(randomLength),
			failRet:     -1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				destinationHandle := hooks.GetManagedTypesContext().NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferSetRandom(destinationHandle, randomLength)
				return ret, func(t *testing.T, done bool) {
					result, err := hooks.GetManagedTypesContext().GetBytes(destinationHandle)
					require.NoError(t, err)
					require.Len(t, result, pick2(done, int(randomLength), 0))
				}
			},
		},
		{
			name:        "MBufferGetArgument charges per byte before the copy",
			preForkGas:  1,
			postForkGas: 1 + uint64(len(manBufTestArgument)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				destinationHandle := hooks.GetManagedTypesContext().NewManagedBuffer()
				provideGas(hooks, gas)
				ret := hooks.MBufferGetArgument(0, destinationHandle)
				return ret, func(t *testing.T, done bool) {
					requireBufferEquals(t, hooks, destinationHandle, pick(done, manBufTestArgument, nil))
				}
			},
		},
		{
			// pre-fork MBufferFinish already bounds its flat and persist charges, so the
			// pre-fork cost covers both; only the clone is newly charged post-fork
			name:        "MBufferFinish charges the clone before it, on top of the persist charge",
			preForkGas:  1 + uint64(len(data)),
			postForkGas: 1 + 2*uint64(len(data)),
			failRet:     1,
			okRet:       0,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, manBufVerifyFunc) {
				sourceHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(data)
				provideGas(hooks, gas)
				ret := hooks.MBufferFinish(sourceHandle)
				return ret, func(t *testing.T, done bool) {
					if done {
						require.Equal(t, [][]byte{data}, hooks.GetOutputContext().ReturnData())
						return
					}
					require.Empty(t, hooks.GetOutputContext().ReturnData())
				}
			},
		},
	})
}

// pick2 is pick for lengths
func pick2(done bool, expected, otherwise int) int {
	if done {
		return expected
	}
	return otherwise
}

// manBufCapCase describes one hook that takes a caller-supplied size and, post-fork, must
// refuse anything above maxManagedBufferLength while accepting exactly the cap. run builds
// the inputs for the given length on a fresh, amply funded host, invokes the hook and
// reports its return code plus whether the work was carried out.
type manBufCapCase struct {
	name    string
	failRet int32
	run     func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (ret int32, built bool)
}

func runManBufCapCases(t *testing.T, cases []manBufCapCase) {
	const ampleGas = uint64(1_000_000_000)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("post-fork: a length over the cap is rejected", func(t *testing.T) {
				hooks := newManBufHooks(t, true)
				provideGas(hooks, ampleGas)
				ret, built := c.run(t, hooks, overMaxManagedBufferLength)

				require.Equal(t, c.failRet, ret)
				require.False(t, built)
				requireLengthExceedsMaximum(t, hooks)
			})

			// the cap is a > comparison: exactly maxManagedBufferLength must still be served
			t.Run("post-fork: a length exactly at the cap is accepted", func(t *testing.T) {
				hooks := newManBufHooks(t, true)
				provideGas(hooks, ampleGas)
				_, built := c.run(t, hooks, maxManagedBufferLength)

				require.True(t, built)
				require.NoError(t, hooks.GetRuntimeContext().GetAllErrors())
			})

			t.Run("pre-fork: a length over the cap is not rejected by the cap", func(t *testing.T) {
				hooks := newManBufHooks(t, false)
				provideGas(hooks, ampleGas)
				_, built := c.run(t, hooks, overMaxManagedBufferLength)

				require.True(t, built)
				require.NotErrorIs(t, hooks.GetRuntimeContext().GetAllErrors(), vmhost.ErrManagedBufferLengthExceedsMaximum)
			})
		})
	}
}

func TestManagedBufferHooksCapCallerSuppliedLengths(t *testing.T) {
	memOffset := executor.MemPtr(0)

	runManBufCapCases(t, []manBufCapCase{
		{
			name:    "MBufferNewFromBytes caps the loaded length",
			failRet: -1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				ret := hooks.MBufferNewFromBytes(memOffset, length)
				return ret, ret >= 0
			},
		},
		{
			name:    "MBufferSetBytes caps the loaded length",
			failRet: 1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				managedType := hooks.GetManagedTypesContext()
				mBufferHandle := managedType.NewManagedBuffer()
				ret := hooks.MBufferSetBytes(mBufferHandle, memOffset, length)
				return ret, ret == 0 && managedType.GetLength(mBufferHandle) > 0
			},
		},
		{
			name:    "MBufferSetByteSlice caps the loaded length",
			failRet: 1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(make([]byte, length))
				ret := hooks.MBufferSetByteSlice(mBufferHandle, 0, length, memOffset)
				return ret, ret == 0
			},
		},
		{
			name:    "MBufferAppendBytes caps the resulting length",
			failRet: 1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				managedType := hooks.GetManagedTypesContext()
				accumulatorHandle := managedType.NewManagedBuffer()
				ret := hooks.MBufferAppendBytes(accumulatorHandle, memOffset, length)
				return ret, ret == 0 && managedType.GetLength(accumulatorHandle) > 0
			},
		},
		{
			name:    "MBufferCopyByteSlice caps the copied length",
			failRet: 1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				managedType := hooks.GetManagedTypesContext()
				sourceHandle := managedType.NewManagedBufferFromBytes(make([]byte, length))
				destinationHandle := managedType.NewManagedBuffer()
				ret := vmhooks.ManagedBufferCopyByteSliceWithHost(hooks.GetVMHost(), sourceHandle, 0, length, destinationHandle)
				return ret, ret == 0 && managedType.GetLength(destinationHandle) == length
			},
		},
		{
			name:    "MBufferAppend caps the resulting length",
			failRet: 1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				managedType := hooks.GetManagedTypesContext()
				accumulatorHandle := managedType.NewManagedBufferFromBytes(make([]byte, length-1))
				dataHandle := managedType.NewManagedBufferFromBytes([]byte{0xAA})
				ret := hooks.MBufferAppend(accumulatorHandle, dataHandle)
				return ret, ret == 0 && managedType.GetLength(accumulatorHandle) == length
			},
		},
		{
			name:    "MBufferSetRandom caps the allocated length",
			failRet: -1,
			run: func(t *testing.T, hooks *vmhooks.VMHooksImpl, length int32) (int32, bool) {
				managedType := hooks.GetManagedTypesContext()
				destinationHandle := managedType.NewManagedBuffer()
				ret := hooks.MBufferSetRandom(destinationHandle, length)
				return ret, ret == 0 && managedType.GetLength(destinationHandle) == length
			},
		},
	})
}

// TestMBufferAppendChargesLargeAccumulatorCopy covers KLR-50 review finding P2: appending to an
// accumulator whose backing array is full reallocates and copies the whole accumulator, so
// funding only the flat cost and the incoming byte must be rejected post-fork.
func TestMBufferGetByteSliceRejectsOverflowingBounds(t *testing.T) {
	source := []byte("abcdefgh")
	const overflowingPosition = int32(2147483647)

	t.Run("post-fork: an overflowing starting position is rejected", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		sourceHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(source)
		provideGas(hooks, 1_000)

		ret := hooks.MBufferGetByteSlice(sourceHandle, overflowingPosition, 1, executor.MemPtr(0))

		require.Equal(t, int32(1), ret)
	})

	t.Run("pre-fork: an overflowing starting position is not rejected by the bounds check", func(t *testing.T) {
		hooks := newManBufHooks(t, false)
		sourceHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(source)
		provideGas(hooks, 1_000)

		require.Panics(t, func() {
			hooks.MBufferGetByteSlice(sourceHandle, overflowingPosition, 1, executor.MemPtr(0))
		})
	})
}

func TestMBufferSetByteSliceRejectsOverflowingBounds(t *testing.T) {
	initial := []byte("........")
	data := []byte("a")
	dataOffset := executor.MemPtr(0)
	const overflowingPosition = int32(2147483647)

	t.Run("post-fork: an overflowing starting position leaves the buffer untouched", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		writeToMemory(t, hooks, dataOffset, data)
		managedType := hooks.GetManagedTypesContext()
		mBufferHandle := managedType.NewManagedBufferFromBytes(initial)
		provideGas(hooks, 1_000)

		ret := hooks.MBufferSetByteSlice(mBufferHandle, overflowingPosition, executor.MemLength(len(data)), dataOffset)

		require.Equal(t, int32(1), ret)
		result, err := managedType.GetBytes(mBufferHandle)
		require.NoError(t, err)
		require.Equal(t, initial, result)
	})

	t.Run("pre-fork: an overflowing starting position is not rejected by the bounds check", func(t *testing.T) {
		hooks := newManBufHooks(t, false)
		writeToMemory(t, hooks, dataOffset, data)
		mBufferHandle := hooks.GetManagedTypesContext().NewManagedBufferFromBytes(initial)
		provideGas(hooks, 1_000)

		require.Panics(t, func() {
			hooks.MBufferSetByteSlice(mBufferHandle, overflowingPosition, executor.MemLength(len(data)), dataOffset)
		})
	})
}

func TestMBufferCopyByteSliceRejectsOverflowingBounds(t *testing.T) {
	source := []byte("abcdefgh")
	const overflowingPosition = int32(2147483647)

	t.Run("post-fork: an overflowing starting position leaves the destination untouched", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		managedType := hooks.GetManagedTypesContext()
		sourceHandle := managedType.NewManagedBufferFromBytes(source)
		destinationHandle := managedType.NewManagedBuffer()
		provideGas(hooks, 1_000)

		ret := vmhooks.ManagedBufferCopyByteSliceWithHost(
			hooks.GetVMHost(), sourceHandle, overflowingPosition, 1, destinationHandle)

		require.Equal(t, int32(1), ret)
		result, err := managedType.GetBytes(destinationHandle)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("pre-fork: an overflowing starting position is not rejected by the bounds check", func(t *testing.T) {
		hooks := newManBufHooks(t, false)
		managedType := hooks.GetManagedTypesContext()
		sourceHandle := managedType.NewManagedBufferFromBytes(source)
		destinationHandle := managedType.NewManagedBuffer()
		provideGas(hooks, 1_000)

		require.Panics(t, func() {
			vmhooks.ManagedBufferCopyByteSliceWithHost(
				hooks.GetVMHost(), sourceHandle, overflowingPosition, 1, destinationHandle)
		})
	})
}

func TestMBufferStorageStoreChargesPerByteBeforeClone(t *testing.T) {
	key := []byte("storageKeyOfSomeLength")
	value := []byte("storageValueOfSomeLength")
	const ampleGas = uint64(1_000_000)

	storeWith := func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, uint64) {
		t.Helper()
		managedType := hooks.GetManagedTypesContext()
		keyHandle := managedType.NewManagedBufferFromBytes(key)
		valueHandle := managedType.NewManagedBufferFromBytes(value)
		resetGas(hooks, gas)

		var ret int32
		consumed := gasConsumedBy(hooks, func() {
			ret = hooks.MBufferStorageStore(keyHandle, valueHandle)
		})

		return ret, consumed
	}

	t.Run("post-fork: gas covering only the flat charge leaves the storage untouched", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		ret, _ := storeWith(t, hooks, 1)

		require.Equal(t, int32(1), ret)
		requireNotEnoughGas(t, hooks)
		stored, _, _, err := hooks.GetStorageContext().GetStorage(key)
		require.NoError(t, err)
		require.Empty(t, stored)
	})

	t.Run("post-fork: a budget covering the pre-fork cost leaves the storage untouched", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		preForkRet, preForkConsumed := storeWith(t, preForkHooks, ampleGas)
		require.Equal(t, int32(0), preForkRet)

		postForkHooks := newManBufHooks(t, true)
		postForkRet, _ := storeWith(t, postForkHooks, preForkConsumed)

		require.Equal(t, int32(1), postForkRet)
		requireNotEnoughGas(t, postForkHooks)
		stored, _, _, err := postForkHooks.GetStorageContext().GetStorage(key)
		require.NoError(t, err)
		require.Empty(t, stored)
	})

	t.Run("post-fork charges the key and the value per byte", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		_, preForkConsumed := storeWith(t, preForkHooks, ampleGas)

		postForkHooks := newManBufHooks(t, true)
		postForkRet, postForkConsumed := storeWith(t, postForkHooks, ampleGas)

		require.Equal(t, int32(0), postForkRet)
		require.Equal(t, preForkConsumed+uint64(len(key)+len(value)), postForkConsumed)
	})
}

// useBoundedGas post-fork goes through UseGasBoundedAndAddTracedGas. A real meteringContext is used
// because MeteringContextMock ignores the function name and its StartGasTracing is a no-op, so
// re-adding StartGasTracing beside the helper would trace [0, gas] without any mock test noticing.
// MBufferToBigIntUnsigned on an empty buffer is used because its only charge is the flat one.
func TestUseBoundedGasTracesOnceUnderTheHook(t *testing.T) {
	hooks := newManBufHooks(t, true)
	provideGas(hooks, 1_000_000)
	metering := hooks.GetMeteringContext()
	metering.SetGasTracing(true)
	// the trace the leaked copy used to land in
	metering.StartGasTracing("previousHook")

	managedType := hooks.GetManagedTypesContext()
	mBufferHandle := managedType.NewManagedBuffer()
	bigIntHandle := managedType.NewBigIntFromInt64(0)
	gas := metering.GasSchedule().ManagedBufferAPICost.MBufferToBigIntUnsigned

	var ret int32
	consumed := gasConsumedBy(hooks, func() {
		ret = hooks.MBufferToBigIntUnsigned(mBufferHandle, bigIntHandle)
	})

	require.Equal(t, int32(0), ret)
	require.Equal(t, gas, consumed)
	require.Equal(t, map[string]map[string][]uint64{string(manBufTestContractCode): {
		"previousHook":            {0},
		"mBufferToBigIntUnsigned": {gas},
	}}, metering.GetGasTrace())
}

func TestMBufferStorageLoadChargesPerByteBeforeLoad(t *testing.T) {
	key := []byte("storageKeyOfSomeLength")
	value := []byte("storageValueOfSomeLength")
	const ampleGas = uint64(1_000_000)

	loadWith := func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, uint64) {
		t.Helper()
		managedType := hooks.GetManagedTypesContext()
		keyHandle := managedType.NewManagedBufferFromBytes(key)
		valueHandle := managedType.NewManagedBufferFromBytes(value)
		resetGas(hooks, ampleGas)
		require.Equal(t, int32(0), hooks.MBufferStorageStore(keyHandle, valueHandle))

		destinationHandle := managedType.NewManagedBuffer()
		resetGas(hooks, gas)

		consumed := gasConsumedBy(hooks, func() {
			hooks.MBufferStorageLoad(keyHandle, destinationHandle)
		})

		return destinationHandle, consumed
	}

	t.Run("post-fork: gas below the key charge leaves the destination untouched", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		destinationHandle, _ := loadWith(t, hooks, 1)

		requireNotEnoughGas(t, hooks)
		result, err := hooks.GetManagedTypesContext().GetBytes(destinationHandle)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("post-fork: a budget covering the pre-fork cost leaves the destination untouched", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		preForkDestination, preForkConsumed := loadWith(t, preForkHooks, ampleGas)
		preForkResult, err := preForkHooks.GetManagedTypesContext().GetBytes(preForkDestination)
		require.NoError(t, err)
		require.Equal(t, value, preForkResult)

		postForkHooks := newManBufHooks(t, true)
		postForkDestination, _ := loadWith(t, postForkHooks, preForkConsumed)

		requireNotEnoughGas(t, postForkHooks)
		postForkResult, err := postForkHooks.GetManagedTypesContext().GetBytes(postForkDestination)
		require.NoError(t, err)
		require.Empty(t, postForkResult)
	})

	t.Run("post-fork charges the key and the loaded value per byte", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		_, preForkConsumed := loadWith(t, preForkHooks, ampleGas)

		postForkHooks := newManBufHooks(t, true)
		postForkDestination, postForkConsumed := loadWith(t, postForkHooks, ampleGas)

		result, err := postForkHooks.GetManagedTypesContext().GetBytes(postForkDestination)
		require.NoError(t, err)
		require.Equal(t, value, result)
		require.Equal(t, preForkConsumed+uint64(len(key)+len(value)), postForkConsumed)
	})
}

func TestMBufferStorageLoadFromAddressChargesPerByteBeforeLoad(t *testing.T) {
	key := []byte("storageKeyOfSomeLength")
	value := []byte("storageValueOfSomeLength")
	const ampleGas = uint64(1_000_000)

	loadWith := func(t *testing.T, hooks *vmhooks.VMHooksImpl, gas uint64) (int32, uint64) {
		t.Helper()
		managedType := hooks.GetManagedTypesContext()
		keyHandle := managedType.NewManagedBufferFromBytes(key)
		valueHandle := managedType.NewManagedBufferFromBytes(value)
		resetGas(hooks, ampleGas)
		require.Equal(t, int32(0), hooks.MBufferStorageStore(keyHandle, valueHandle))

		addressHandle := managedType.NewManagedBufferFromBytes(manBufTestContractCode)
		destinationHandle := managedType.NewManagedBuffer()
		resetGas(hooks, gas)

		consumed := gasConsumedBy(hooks, func() {
			hooks.MBufferStorageLoadFromAddress(addressHandle, keyHandle, destinationHandle)
		})

		return destinationHandle, consumed
	}

	t.Run("post-fork: gas covering only the flat charge leaves the destination untouched", func(t *testing.T) {
		hooks := newManBufHooks(t, true)
		destinationHandle, _ := loadWith(t, hooks, 1)

		requireNotEnoughGas(t, hooks)
		result, err := hooks.GetManagedTypesContext().GetBytes(destinationHandle)
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("post-fork: a budget covering the pre-fork cost leaves the destination untouched", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		preForkDestination, preForkConsumed := loadWith(t, preForkHooks, ampleGas)
		preForkResult, err := preForkHooks.GetManagedTypesContext().GetBytes(preForkDestination)
		require.NoError(t, err)
		require.Equal(t, value, preForkResult)

		postForkHooks := newManBufHooks(t, true)
		postForkDestination, _ := loadWith(t, postForkHooks, preForkConsumed)

		requireNotEnoughGas(t, postForkHooks)
		postForkResult, err := postForkHooks.GetManagedTypesContext().GetBytes(postForkDestination)
		require.NoError(t, err)
		require.Empty(t, postForkResult)
	})

	t.Run("post-fork charges the key, the address and the loaded value per byte", func(t *testing.T) {
		preForkHooks := newManBufHooks(t, false)
		_, preForkConsumed := loadWith(t, preForkHooks, ampleGas)

		postForkHooks := newManBufHooks(t, true)
		postForkDestination, postForkConsumed := loadWith(t, postForkHooks, ampleGas)

		result, err := postForkHooks.GetManagedTypesContext().GetBytes(postForkDestination)
		require.NoError(t, err)
		require.Equal(t, value, result)
		require.Equal(t,
			preForkConsumed+uint64(len(key)+len(manBufTestContractCode)+len(value)),
			postForkConsumed)
	})
}
