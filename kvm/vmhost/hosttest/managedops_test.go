package hostCoretest

import (
	"testing"

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

// overflowStartingPosition + a small positive length overflows int32, which
// used to slip past the bounds check and panic on the slice expression itself.
const overflowStartingPosition = int32(2147483642) // math.MaxInt32 - 5

func TestManaged_GetByteSlice_OverflowRejectedGracefully(t *testing.T) {
	data := "ABCDEFGHIJ"
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						hooks := vmhooks.NewVMHooksImpl(host)
						mBuffer := managedType.NewManagedBufferFromBytes([]byte(data))

						result := hooks.MBufferGetByteSlice(mBuffer, overflowStartingPosition, 10, 0)
						require.Equal(t, int32(1), result)

						host.Output().Finish([]byte("done"))
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
			verify.Ok().ReturnData([]byte("done"))
		})
	assert.Nil(t, err)
}

func TestManaged_CopyByteSlice_OverflowRejectedGracefully(t *testing.T) {
	sourceData := "ABCDEFGHIJ"
	destinationData := "0123456789"
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						sourceMBuffer := managedType.NewManagedBufferFromBytes([]byte(sourceData))
						destMBuffer := managedType.NewManagedBufferFromBytes([]byte(destinationData))

						result := vmhooks.ManagedBufferCopyByteSliceWithHost(
							host, sourceMBuffer, overflowStartingPosition, 10, destMBuffer)
						require.Equal(t, int32(1), result)

						host.Output().Finish([]byte("done"))
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
			verify.Ok().ReturnData([]byte("done"))
		})
	assert.Nil(t, err)
}

func TestManaged_SetByteSlice_OverflowRejectedGracefully(t *testing.T) {
	bufferData := "ABCDEFGHIJ"
	patchData := []byte("XY")
	_, err := test.BuildMockInstanceCallTest(t).
		WithContracts(
			test.CreateMockContract(test.ParentAddress).
				WithBalance(1000).
				WithMethods(func(parentInstance *mock.InstanceMock, config interface{}) {
					parentInstance.AddMockMethod("testFunction", func() *mock.InstanceMock {
						host := parentInstance.Host
						managedType := host.ManagedTypes()
						mBuffer := managedType.NewManagedBufferFromBytes([]byte(bufferData))

						result := vmhooks.ManagedBufferSetByteSliceWithTypedArgs(
							host, mBuffer, overflowStartingPosition, int32(len(patchData)), patchData)
						require.Equal(t, int32(1), result)

						host.Output().Finish([]byte("done"))
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
			verify.Ok().ReturnData([]byte("done"))
		})
	assert.Nil(t, err)
}
