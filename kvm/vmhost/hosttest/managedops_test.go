package hostCoretest

import (
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wasm memory ~~~> managed buffer
func TestManaged_SetByteSlice(t *testing.T) {
	prefix := "ABCD"
	slice := "EFGHIJKLMN"
	suffix := "OPR"
	data := prefix + slice + suffix
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						mBuffer := managedType.NewManagedBufferFromBytes(
							make([]byte, len(data)))
						result := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
							host, mBuffer, int32(len(prefix)), int32(len(slice)), []byte(data))
						if result != 0 {
							vmhooks.WithFaultAndHost(host, vmhost.ErrSignalError, true)
						}
						bufferBytes, err := managedType.GetBytes(mBuffer)
						if err != nil {
							vmhooks.WithFaultAndHost(host, err, true)
						}
						host.Output().Finish(bufferBytes)
						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				// |....ABCDEFGHIJ...|
				ReturnData(append(make([]byte, len(prefix)),
					append([]byte(data[0:len(slice)]), make([]byte, len(suffix))...)...))
		})
	assert.Nil(t, err)
}

// managed buffer ~~~> managed buffer
func TestManaged_CopyByteSlice_DifferentBuffer(t *testing.T) {
	prefix := "ABCD"
	slice := "EFGHIJKLMN"
	suffix := "OPR"
	sourceData := prefix + slice + suffix
	destinationData := "01234567890123456789"
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						sourceMBuffer := managedType.NewManagedBufferFromBytes(
							[]byte(sourceData))
						destMBuffer := managedType.NewManagedBufferFromBytes(
							[]byte(destinationData))
						result := vmhooks.ManagedBufferCopyByteSliceWithHost(
							host, sourceMBuffer, int32(len(prefix)), int32(len(slice)), destMBuffer)
						if result != 0 {
							vmhooks.WithFaultAndHost(host, vmhost.ErrSignalError, true)
						}
						destBytes, err := managedType.GetBytes(destMBuffer)
						if err != nil {
							vmhooks.WithFaultAndHost(host, err, true)
						}
						host.Output().Finish(destBytes)
						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok().
				ReturnData([]byte(slice))
		})
	assert.Nil(t, err)
}

func TestManaged_CopyByteSlice_SameBuffer(t *testing.T) {
	prefix := "ABCD"
	slice := "EFGHIJKLMN"
	suffix := "OPR"
	sourceData := prefix + slice + suffix
	deltaForSlice := int32(2)
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						sourceMBuffer := managedType.NewManagedBufferFromBytes(
							[]byte(sourceData))
						result := vmhooks.ManagedBufferCopyByteSliceWithHost(
							host, sourceMBuffer, int32(len(prefix))-deltaForSlice, int32(len(slice)), sourceMBuffer)
						if result != 0 {
							vmhooks.WithFaultAndHost(host, vmhost.ErrSignalError, true)
						}
						destBytes, err := managedType.GetBytes(sourceMBuffer)
						if err != nil {
							vmhooks.WithFaultAndHost(host, err, true)
						}
						host.Output().Finish(destBytes)
						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1000).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			prefixLen := int32(len(prefix))
			sliceLen := int32(len(slice))
			verify.Ok().
				// |CDEFGHIJKL|
				ReturnData(
					append([]byte(prefix)[prefixLen-deltaForSlice:prefixLen],
						[]byte(slice)[:sliceLen-deltaForSlice]...))
		})
	assert.Nil(t, err)
}

func TestManaged_StorageStoreKeyIsolatedFromBufferMutation(t *testing.T) {
	protectedKey := []byte("KDA/FAKE")
	unprotectedSameLengthKey := []byte("AAA/FAKE")
	forgedValue := []byte("serialized-user-kda")

	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, _ interface{}) {
					parentInstance.AddMockMethod("forgeKDAStorage", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						hooks := vmhooks.NewVMHooksImpl(host)

						valueHandle := managedType.NewManagedBufferFromBytes(forgedValue)
						keyHandle := managedType.NewManagedBufferFromBytes(unprotectedSameLengthKey)

						require.Equal(t, int32(0), hooks.MBufferStorageStore(keyHandle, valueHandle))

						result := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
							host, keyHandle, 0, int32(len(protectedKey)), protectedKey)
						require.Equal(t, int32(0), result)

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1_000_000).
			WithFunction("forgeKDAStorage").
			Build()).
		AndAssertResults(func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
			verify.Ok()

			outAcc := verify.VmOutput.OutputAccounts[string(test.ParentAddress)]
			require.NotNil(t, outAcc)

			update := outAcc.StorageUpdates[string(unprotectedSameLengthKey)]
			require.NotNil(t, update)
			require.True(t, update.Written)
			require.Equal(t, unprotectedSameLengthKey, update.Offset)
			require.Equal(t, forgedValue, update.Data)
			require.NotContains(t, outAcc.StorageUpdates, string(protectedKey))
		})

	require.NoError(t, err)
}

const preForkFixAuditChangesV5 = 1_000_000

func runManagedBufferHookWithGas(
	t *testing.T,
	fixAuditChangesV5 uint32,
	gasLeftBeforeHook uint64,
	hook func(host vmhost.VMHost),
	assertResults test.AssertResultsFunc,
) {
	_, err := test.BuildMockInstanceCallTest(t).
		WithEnableEpochs(blockchainConfig.EnableEpochs{FixAuditChangesV5: fixAuditChangesV5}).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						metering := host.Metering()
						metering.UseGas(metering.GasLeft() - gasLeftBeforeHook)

						hook(host)

						return parentInstance
					})
				}),
		).
		WithInput(test.CreateTestContractCallInputBuilder().
			WithRecipientAddr(test.ParentAddress).
			WithGasProvided(1_000_000).
			WithFunction("testFunction").
			Build()).
		AndAssertResults(assertResults)

	require.NoError(t, err)
}

func TestManaged_SetRandom_RejectsInsufficientGas(t *testing.T) {
	const length = int32(64)

	setRandom := func(hookReturn *int32, produced *[]byte) func(host vmhost.VMHost) {
		return func(host vmhost.VMHost) {
			managedType := host.ManagedTypes()
			destinationHandle := managedType.NewManagedBuffer()
			*hookReturn = vmhooks.NewVMHooksImpl(host).MBufferSetRandom(destinationHandle, length)
			result, err := managedType.GetBytes(destinationHandle)
			require.NoError(t, err)
			*produced = result
		}
	}

	t.Run("post-fork: gas covering only the flat charge allocates nothing", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, 1, setRandom(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas().ReturnMessage(vmhost.ErrNotEnoughGas.Error())
			})

		require.Equal(t, int32(-1), hookReturn)
		require.Empty(t, produced)
	})

	t.Run("pre-fork: gas covering only the flat charge still allocates", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, preForkFixAuditChangesV5, 1, setRandom(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Len(t, produced, int(length))
	})

	t.Run("post-fork: gas covering the length-dependent charge allocates", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, uint64(1+length), setRandom(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Len(t, produced, int(length))
	})
}

func TestManaged_SetRandom_RejectsOversizeLength(t *testing.T) {
	const oversizeLength = int32(16_000_001)

	setRandom := func(hookReturn *int32, produced *[]byte) func(host vmhost.VMHost) {
		return func(host vmhost.VMHost) {
			managedType := host.ManagedTypes()
			destinationHandle := managedType.NewManagedBuffer()
			*hookReturn = vmhooks.NewVMHooksImpl(host).MBufferSetRandom(destinationHandle, oversizeLength)
			result, err := managedType.GetBytes(destinationHandle)
			require.NoError(t, err)
			*produced = result
		}
	}

	t.Run("post-fork: an oversize length allocates nothing", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, 900_000, setRandom(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.ExecutionFailed().ReturnMessage(vmhost.ErrManagedBufferLengthExceedsMaximum.Error())
			})

		require.Equal(t, int32(-1), hookReturn)
		require.Empty(t, produced)
	})

	t.Run("pre-fork: an oversize length is still allocated", func(t *testing.T) {
		if test.RaceDetectorEnabled {
			t.Skip("the race detector slows execution past the smart contract execution timeout")
		}

		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, preForkFixAuditChangesV5, 900_000, setRandom(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Len(t, produced, int(oversizeLength))
	})
}

func TestManaged_Append_ChargesPerByteBeforeAppend(t *testing.T) {
	accumulator := []byte("abcd")
	data := []byte("efgh")

	appendBuffer := func(hookReturn *int32, produced *[]byte) func(host vmhost.VMHost) {
		return func(host vmhost.VMHost) {
			managedType := host.ManagedTypes()
			accumulatorHandle := managedType.NewManagedBufferFromBytes(accumulator)
			dataHandle := managedType.NewManagedBufferFromBytes(data)
			*hookReturn = vmhooks.NewVMHooksImpl(host).MBufferAppend(accumulatorHandle, dataHandle)
			result, err := managedType.GetBytes(accumulatorHandle)
			require.NoError(t, err)
			*produced = result
		}
	}

	t.Run("post-fork: gas covering only the flat charge leaves the accumulator untouched", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, 1, appendBuffer(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas().ReturnMessage(vmhost.ErrNotEnoughGas.Error())
			})

		require.Equal(t, int32(1), hookReturn)
		require.Equal(t, accumulator, produced)
	})

	t.Run("pre-fork: gas covering only the flat charge still appends", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, preForkFixAuditChangesV5, 1, appendBuffer(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Equal(t, []byte("abcdefgh"), produced)
	})

	t.Run("post-fork: gas covering only the data leaves the accumulator untouched", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, uint64(1+len(data)), appendBuffer(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas().ReturnMessage(vmhost.ErrNotEnoughGas.Error())
			})

		require.Equal(t, int32(1), hookReturn)
		require.Equal(t, accumulator, produced)
	})

	t.Run("post-fork: gas covering the data and the accumulator copy appends", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, uint64(1+len(data)+len(accumulator)), appendBuffer(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Equal(t, []byte("abcdefgh"), produced)
	})
}

func TestManaged_ToBigIntUnsigned_ChargesPerByteBeforeParse(t *testing.T) {
	source := []byte{0x01, 0x02, 0x03, 0x04}

	toBigInt := func(hookReturn *int32, produced *[]byte) func(host vmhost.VMHost) {
		return func(host vmhost.VMHost) {
			managedType := host.ManagedTypes()
			mBufferHandle := managedType.NewManagedBufferFromBytes(source)
			bigIntHandle := managedType.NewBigIntFromInt64(0)
			*hookReturn = vmhooks.NewVMHooksImpl(host).MBufferToBigIntUnsigned(mBufferHandle, bigIntHandle)
			value, err := managedType.GetBigInt(bigIntHandle)
			require.NoError(t, err)
			*produced = value.Bytes()
		}
	}

	t.Run("post-fork: gas covering only the flat charge leaves the destination untouched", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, 1, toBigInt(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.OutOfGas().ReturnMessage(vmhost.ErrNotEnoughGas.Error())
			})

		require.Equal(t, int32(1), hookReturn)
		require.Empty(t, produced)
	})

	t.Run("pre-fork: gas covering only the flat charge still parses", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, preForkFixAuditChangesV5, 1, toBigInt(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Equal(t, source, produced)
	})

	t.Run("post-fork: gas covering the per-byte charge parses", func(t *testing.T) {
		var hookReturn int32
		var produced []byte

		runManagedBufferHookWithGas(t, 0, uint64(1+len(source)), toBigInt(&hookReturn, &produced),
			func(world *worldmock.MockWorld, verify *test.VMOutputVerifier) {
				verify.Ok()
			})

		require.Equal(t, int32(0), hookReturn)
		require.Equal(t, source, produced)
	})
}
