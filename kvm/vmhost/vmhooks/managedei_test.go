package vmhooks_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVMHooksImpl_ManagedAccHasPerm(t *testing.T) {
	host := newMockVMHost()
	hooks := vmhooks.NewVMHooksImpl(host)

	// invalid perm, coverage only
	result := hooks.ManagedAccHasPerm(-1, 1, 2)
	assert.Equal(t, int32(0), result)
}

// Test function
func TestManagedAccHasPermWithHost(t *testing.T) {
	t.Run("Invalid ops", func(t *testing.T) {
		host := newMockVMHost()

		result := vmhooks.ManagedAccHasPermWithHost(host, -1, 1, 2)
		assert.Equal(t, int32(0), result)
	})

	t.Run("Invalid source address", func(t *testing.T) {
		host := newMockVMHost()

		host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock).GetBytesCalled = func(index int32) ([]byte, error) {
			if index == 1 {
				return nil, errors.New("invalid address")
			}
			return []byte("target"), nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)
		assert.Equal(t, int32(0), result)
	})

	t.Run("Invalid target address", func(t *testing.T) {
		host := newMockVMHost()
		host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock).GetBytesCalled = func(index int32) ([]byte, error) {
			if index == 2 {
				return nil, errors.New("invalid address")
			}
			return []byte("source"), nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(0), result)
	})

	t.Run("GetUserAccount error", func(t *testing.T) {
		host := newMockVMHost()
		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(0), result)
	})

	t.Run("ValidatePerm called", func(t *testing.T) {
		host := newMockVMHost()
		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			return state.NewEmptyUserAccount(), nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(0), result)
	})

	t.Run("No permissions", func(t *testing.T) {
		host := newMockVMHost()
		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			return state.NewEmptyUserAccount(), nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(0), result)
	})

	t.Run("Permission granted owner", func(t *testing.T) {
		host := newMockVMHost()

		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			acc := state.NewEmptyUserAccount()
			acc.UserAccountData.Permissions = []*state.Permission{
				{
					Type:      state.Permission_Owner,
					Threshold: 1,
					Signers:   []*state.Key{{Address: []byte("target"), Weight: 1}},
				},
			}
			return acc, nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(1), result)
	})

	t.Run("Permission granted user", func(t *testing.T) {
		host := newMockVMHost()
		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			acc := state.NewEmptyUserAccount()
			acc.UserAccountData.Permissions = []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{1},
					Signers:    []*state.Key{{Address: []byte("target"), Weight: 1}},
				},
			}
			return acc, nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(1), result)
	})

	t.Run("Permission not granted", func(t *testing.T) {
		host := newMockVMHost()
		host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
			acc := state.NewEmptyUserAccount()
			acc.UserAccountData.Permissions = []*state.Permission{
				{
					Type:       state.Permission_User,
					Threshold:  1,
					Operations: []byte{1},
					Signers:    []*state.Key{{Address: []byte("source"), Weight: 1}},
				},
			}
			return acc, nil
		}

		result := vmhooks.ManagedAccHasPermWithHost(host, 1, 1, 2)

		assert.Equal(t, int32(0), result)
	})
}

func generateAssetIdentifier() []byte {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lettersAndDigits = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Generates token ticker
	ticker := make([]byte, 3)
	for i := range ticker {
		ticker[i] = letters[rand.Intn(len(letters))]
	}

	// Adds hyphen
	ticker = append(ticker, '-')

	// Generates the token random id
	id := make([]byte, 4)
	hasDigit := false
	for i := 0; i < len(id); i++ {
		char := lettersAndDigits[rand.Intn(len(lettersAndDigits))]
		if char >= '0' && char <= '9' {
			hasDigit = true
		}
		id[i] = char //nolint:gosec // G602: i is bounded by len(id) in loop condition
	}

	// Ensure at least one digit on token id
	if !hasDigit {
		// Replace a random position with a digit between 0 and 9
		id[rand.Intn(len(id))] = '0' + byte(rand.Intn(10))
	}

	// Append the id to the token ticker
	ticker = append(ticker, id...)

	return ticker
}

func generateTransfersSlice(length int64) []*vmcommon.KDATransfer {
	transfers := []*vmcommon.KDATransfer{}
	for i := int64(0); i < length; i++ {
		transfers = append(transfers, &vmcommon.KDATransfer{
			KDAValue:     big.NewInt((i + 1) * 100),
			KDATokenName: generateAssetIdentifier(),
		})
	}
	return transfers
}

func shuffleTransferSlice(slice []*vmcommon.KDATransfer) []*vmcommon.KDATransfer {
	rand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
	return slice
}

func TestManagedGetKDACallValue(t *testing.T) {
	cases := []struct {
		title        string
		kdaTransfers []*vmcommon.KDATransfer
	}{
		{
			title:        "With 10 transfers",
			kdaTransfers: generateTransfersSlice(9),
		},
		{
			title:        "With 20 transfers",
			kdaTransfers: generateTransfersSlice(19),
		},
		{
			title:        "With 30 transfers",
			kdaTransfers: generateTransfersSlice(29),
		},
		{
			title:        "With 40 transfers",
			kdaTransfers: generateTransfersSlice(39),
		},
		{
			title:        "With 50 transfers",
			kdaTransfers: generateTransfersSlice(49),
		},
	}

	t.Run("Sucessful", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(contextmock.NewInstanceMock(nil))

				hooksRuntime := hooks.GetRuntimeContext()
				hooksMetering := hooks.GetMeteringContext()
				hooksMgdType := hooks.GetManagedTypesContext()

				lastTransferHandle := int32(len(tt.kdaTransfers) + 1)
				kdaHandle, kdaCallValueHandle := lastTransferHandle, lastTransferHandle
				testKDAId := []byte("TEST-H7K7")
				// Sets KDA ID into bytes on memory using kdaHandle
				hooksMgdType.SetBytes(kdaHandle, testKDAId)

				// As it does not exist, it will be created
				initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, initialKDAValue, big.NewInt(0))

				expectedKDAValue := big.NewInt(1000)
				tt.kdaTransfers = append(tt.kdaTransfers, &vmcommon.KDATransfer{
					KDAValue:     expectedKDAValue,
					KDATokenName: testKDAId,
				})
				shuffleTransferSlice(tt.kdaTransfers)
				hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
						GasProvided:  10_000,
					},
				})
				hooksMetering.InitStateFromContractCallInput(&hooksRuntime.GetVMInput().VMInput)

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// As it now exists the value retrieved must be non-zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, expectedKDAValue)

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				dataCopyPerByte := hooksMetering.GasSchedule().BaseOperationCost.DataCopyPerByte
				// from FixAuditChangesV6 the token id is paid for before it is copied out of the buffer
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas +
					dataCopyPerByte*uint64(len(testKDAId))

				assert.Equal(t, hooksRuntime.GetPointsUsed(), totalGas)
			})
		}
	})

	t.Run("Sucessful with KLV among assets", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(contextmock.NewInstanceMock(nil))

				hooksRuntime := hooks.GetRuntimeContext()
				hooksMetering := hooks.GetMeteringContext()
				hooksMgdType := hooks.GetManagedTypesContext()

				// Adding KLV to the transfers
				tt.kdaTransfers = append(tt.kdaTransfers, &vmcommon.KDATransfer{
					KDAValue:     big.NewInt(1000),
					KDATokenName: kdautils.KLVIdentifier,
				})
				shuffleTransferSlice(tt.kdaTransfers)

				// Getting random transfer to use as target
				transferHandle := int32(len(tt.kdaTransfers) / 2)
				targetTransfer := tt.kdaTransfers[transferHandle]

				kdaHandle, kdaCallValueHandle := transferHandle, transferHandle
				// Sets KDA into bytes on memory using kdaHandle
				hooksMgdType.SetBytes(kdaHandle, targetTransfer.KDATokenName)

				// As it does not exist, it will be created
				initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, initialKDAValue, big.NewInt(0))

				hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
						GasProvided:  10_000,
					},
				})
				hooksMetering.InitStateFromContractCallInput(&hooksRuntime.GetVMInput().VMInput)

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// As it now exists the value retrieved must be non-zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, targetTransfer.KDAValue)

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				dataCopyPerByte := hooksMetering.GasSchedule().BaseOperationCost.DataCopyPerByte
				// from FixAuditChangesV6 the token name is paid for before it is copied out of the buffer
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas +
					dataCopyPerByte*uint64(len(targetTransfer.KDATokenName))

				assert.Equal(t, hooksRuntime.GetPointsUsed(), totalGas)
			})
		}
	})

	t.Run("Error getting token ID", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(contextmock.NewInstanceMock(nil))

				hooksRuntime := hooks.GetRuntimeContext()
				hooksMetering := hooks.GetMeteringContext()
				hooksMgdType := hooks.GetManagedTypesContext()

				lastTransferHandle := int32(len(tt.kdaTransfers) + 1)
				kdaHandle, kdaCallValueHandle := lastTransferHandle, lastTransferHandle

				// As it does not exist, it will be created
				initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, initialKDAValue, big.NewInt(0))

				tt.kdaTransfers = append(tt.kdaTransfers, &vmcommon.KDATransfer{
					KDAValue:     big.NewInt(1000),
					KDATokenName: []byte("TEST-H7K7"),
				})
				shuffleTransferSlice(tt.kdaTransfers)
				hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
						GasProvided:  10_000,
					},
				})
				hooksMetering.InitStateFromContractCallInput(&hooksRuntime.GetVMInput().VMInput)

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				// It will get an error because the ID token is not registered in the VM memoryll set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// Must continue as zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, big.NewInt(0))

				// Calculate gas used
				// the handle is missing, so nothing is copied: the hook charges its fixed cost and
				// then fails, and failing consumes everything that was left
				assert.Equal(t, hooksRuntime.GetVMInput().GasProvided, hooksRuntime.GetPointsUsed())

				vmOutput := vmHost.Output().GetVMOutput()
				assert.Equal(
					t,
					vmOutput.ReturnCode,
					vmcommon.ReturnCode(transaction.Transaction_VMExecutionFailed),
				)
				assert.Equal(t, vmOutput.ReturnMessage, vmhost.ErrArgOutOfRange.Error())
			})
		}
	})

	t.Run("Error getting token ID with KLV among transfers", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(contextmock.NewInstanceMock(nil))

				hooksRuntime := hooks.GetRuntimeContext()
				hooksMetering := hooks.GetMeteringContext()
				hooksMgdType := hooks.GetManagedTypesContext()

				// Adding KLV to the transfers
				tt.kdaTransfers = append(tt.kdaTransfers, &vmcommon.KDATransfer{
					KDAValue:     big.NewInt(1000),
					KDATokenName: kdautils.KLVIdentifier,
				})
				shuffleTransferSlice(tt.kdaTransfers)

				// Getting random transfer to use as target
				transferHandle := int32(len(tt.kdaTransfers) / 2)

				kdaHandle, kdaCallValueHandle := transferHandle, transferHandle

				// As it does not exist, it will be created
				initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, initialKDAValue, big.NewInt(0))

				hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
						GasProvided:  10_000,
					},
				})
				hooksMetering.InitStateFromContractCallInput(&hooksRuntime.GetVMInput().VMInput)

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				// It will get an error because the ID token is not registered in the VM memoryll set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// Must continue as zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, big.NewInt(0))

				// Calculate gas used
				// the handle is missing, so nothing is copied: the hook charges its fixed cost and
				// then fails, and failing consumes everything that was left
				assert.Equal(t, hooksRuntime.GetVMInput().GasProvided, hooksRuntime.GetPointsUsed())

				vmOutput := vmHost.Output().GetVMOutput()
				assert.Equal(
					t,
					vmOutput.ReturnCode,
					vmcommon.ReturnCode(transaction.Transaction_VMExecutionFailed),
				)
				assert.Equal(t, vmOutput.ReturnMessage, vmhost.ErrArgOutOfRange.Error())
			})
		}
	})

	t.Run("With empty transfers slice", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		// Set mock instance
		it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
		require.True(t, ok)
		it.ReplaceInstance(contextmock.NewInstanceMock(nil))

		hooksRuntime := hooks.GetRuntimeContext()
		hooksMetering := hooks.GetMeteringContext()
		hooksMgdType := hooks.GetManagedTypesContext()

		kdaHandle, kdaCallValueHandle := int32(1), int32(1)
		testKDAId := []byte("TEST-H7K7")

		// Sets KDA ID into bytes on memory using kdaHandle
		hooksMgdType.SetBytes(kdaHandle, testKDAId)

		// As it does not exist, it will be created
		initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
		assert.Equal(t, initialKDAValue, big.NewInt(0))

		hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				KDATransfers: []*vmcommon.KDATransfer{},
				GasProvided:  10_000,
			},
		})
		hooksMetering.InitStateFromContractCallInput(&hooksRuntime.GetVMInput().VMInput)

		// Invoking test target hook with empty transfers slice, so no value will be set
		hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

		// As its does not was set, the value retrieved must be zero
		result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
		assert.Equal(t, result, big.NewInt(0))

		// Calculate gas used
		loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
		// from FixAuditChangesV6 the token id is paid for before it is copied out of the buffer
		dataCopyPerByte := hooksMetering.GasSchedule().BaseOperationCost.DataCopyPerByte
		callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
		totalGas := (uint64(loopGas))*uint64(
			len(hooksRuntime.GetVMInput().KDATransfers),
		) + callValueGas + dataCopyPerByte*uint64(len(testKDAId))

		assert.Equal(t, hooksRuntime.GetPointsUsed(), totalGas)

		vmOutput := vmHost.Output().GetVMOutput()
		assert.Equal(
			t,
			vmOutput.ReturnCode,
			vmcommon.ReturnCode(transaction.Transaction_Ok),
		)
	})
}

// Helper function to create a new mock VMHost
func newMockVMHost() *contextmock.VMHostMock {
	gasSchedule := config.MakeGasMapForTests()

	initialGas := uint64(1000)
	mockMetering := &contextmock.MeteringContextMock{
		GasProvidedMock: initialGas,
		GasLeftMock:     initialGas,
	}
	mockMetering.SetGasSchedule(gasSchedule)
	mockRuntime := &contextmock.RuntimeContextMock{}
	mockRuntime.InitState()

	mockBuffer := make([][]byte, 0)
	mockBigInt := make([]*big.Int, 0)
	mType := &hostmock.ManagedTypesContextMock{
		GetBytesCalled: func(index int32) ([]byte, error) {
			switch index {
			case 1:
				return []byte("source"), nil
			case 2:
				return []byte("target"), nil
			default:
				return nil, errors.New("invalid index")
			}
		},
		NewManagedBufferCalled: func() int32 {
			mockBuffer = append(mockBuffer, []byte(""))
			return int32(len(mockBuffer) - 1)
		},
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			mockBuffer = append(mockBuffer, bytes)
			return int32(len(mockBuffer) - 1)
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			mockBigInt = append(mockBigInt, value)
			return int32(len(mockBigInt) - 1)
		},
		NewBigIntFromInt64Called: func(value int64) int32 {
			mockBigInt = append(mockBigInt, big.NewInt(value))
			return int32(len(mockBigInt) - 1)
		},
		GetManagedBufferCalled: func() [][]byte {
			return mockBuffer
		},
		GetManagedBigIntCalled: func() []*big.Int {
			return mockBigInt
		},
	}

	blockchain := &hostmock.BlockchainContextStub{}

	return &contextmock.VMHostMock{
		MeteringContext:       mockMetering,
		RuntimeContext:        mockRuntime,
		ManagedTypesContext:   mType,
		BlockchainContext:     blockchain,
		ForkControllerContext: &commonMock.ForkControllerStub{},
	}
}

const (
	upgradeDestHandle          = int32(1)
	upgradeGasLimit            = int64(0)
	upgradeValueHandle         = int32(2)
	upgradeCodeHandle          = int32(3)
	upgradeSourceAddressHandle = int32(3)
	upgradeCodeMetadataHandle  = int32(4)
	upgradeArgumentsHandle     = int32(5)
	upgradeResultHandle        = int32(6)
)

func newMockVMHostWithUpgradeValue(value *big.Int, fixAuditChangesV4 bool) *contextmock.VMHostMock {
	host := newMockVMHost()
	host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: fixAuditChangesV4}
	host.OutputContext = &contextmock.OutputContextMock{}
	managedTypes := host.ManagedTypesContext.(*hostmock.ManagedTypesContextMock)
	managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
		return []byte("contract"), nil
	}
	managedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
		return value, nil
	}
	managedTypes.ReadManagedVecOfManagedBuffersCalled = func(handle int32) ([][]byte, uint64, error) {
		return [][]byte{[]byte("argument")}, 1, nil
	}
	host.RuntimeContext.(*contextmock.RuntimeContextMock).FailBaseOpsAPI = true
	return host
}

func upgradeValueCases() []struct {
	purpose           string
	value             *big.Int
	fixAuditChangesV4 bool
	expectedError     error
} {
	return []struct {
		purpose           string
		value             *big.Int
		fixAuditChangesV4 bool
		expectedError     error
	}{
		{
			purpose:           "Should reject a negative value",
			value:             big.NewInt(-10),
			fixAuditChangesV4: true,
			expectedError:     vmhost.ErrTransferNegativeValue,
		},
		{
			purpose:           "Should accept a zero value",
			value:             big.NewInt(0),
			fixAuditChangesV4: true,
			expectedError:     nil,
		},
		{
			purpose:           "Should accept a positive value",
			value:             big.NewInt(10),
			fixAuditChangesV4: true,
			expectedError:     nil,
		},
		{
			purpose:           "Should accept a negative value before the fork",
			value:             big.NewInt(-10),
			fixAuditChangesV4: false,
			expectedError:     nil,
		},
	}
}

func TestVMHooksImpl_ManagedUpgradeContract_ValueValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range upgradeValueCases() {
		t.Run(tt.purpose, func(t *testing.T) {
			host := newMockVMHostWithUpgradeValue(tt.value, tt.fixAuditChangesV4)
			hooks := vmhooks.NewVMHooksImpl(host)

			hooks.ManagedUpgradeContract(
				upgradeDestHandle,
				upgradeGasLimit,
				upgradeValueHandle,
				upgradeCodeHandle,
				upgradeCodeMetadataHandle,
				upgradeArgumentsHandle,
				upgradeResultHandle,
			)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestVMHooksImpl_ManagedUpgradeFromSourceContract_ValueValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range upgradeValueCases() {
		t.Run(tt.purpose, func(t *testing.T) {
			host := newMockVMHostWithUpgradeValue(tt.value, tt.fixAuditChangesV4)
			hooks := vmhooks.NewVMHooksImpl(host)

			hooks.ManagedUpgradeFromSourceContract(
				upgradeDestHandle,
				upgradeGasLimit,
				upgradeValueHandle,
				upgradeSourceAddressHandle,
				upgradeCodeMetadataHandle,
				upgradeArgumentsHandle,
				upgradeResultHandle,
			)

			assert.Equal(t, tt.expectedError, host.RuntimeContext.(*contextmock.RuntimeContextMock).FailExecutionErr)
		})
	}
}

func TestManagedGetKDARolesWithHost_GasCost(t *testing.T) {
	t.Parallel()

	var totalGas uint64

	managedTypes := &hostmock.ManagedTypesContextMock{
		ConsumeGasForBytesCalled: func(bytes []byte) {
			totalGas += uint64(len(bytes))
		},
	}
	blockchain := &contextmock.BlockchainContextMock{}

	var runtimeErr error
	mockErr := errors.New("mock error")

	host := &contextmock.VMHostMock{
		ManagedTypesContext: managedTypes,
		// pinned pre-FixAuditChangesV6, as in TestManagedGetKDATokenDataWithHost_GasCost
		ForkControllerContext: &commonMock.ForkControllerStub{},
		MeteringContext: &contextmock.MeteringContextMock{
			GasCost: &config.GasCost{
				BaseOpsAPICost: config.BaseOpsAPICost{
					GetKDARoles: 10,
				},
			},
			UseAndTraceGasCalled: func(gas uint64) {
				totalGas += gas
			},
		},
		BlockchainContext: blockchain,
		RuntimeContext: &contextmock.RuntimeContextWrapper{
			BaseOpsErrorShouldFailExecutionFunc: func() bool {
				return true
			},
			FailExecutionFunc: func(err error) {
				runtimeErr = err
			},
		},
	}

	cases := []struct {
		purpose       string
		mock          func()
		expectedError error
		expectedGas   uint64
	}{
		{
			purpose: "Should return the base gas cost consumed",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Roles: []*kapps.RolesData{
							{
								Address:             []byte("1"),
								HasRoleMint:         true,
								HasRoleSetITOPrices: true,
							},
						},
					}, &kapps.UserKDA{}, nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 1 + 8, // base + addressCost + roleBytesCost
		},
		{
			purpose: "Should return the base gas cost + cost for each role bytes",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Roles: []*kapps.RolesData{
							{
								Address:             []byte("role1"),
								HasRoleMint:         true,
								HasRoleSetITOPrices: true,
							},
							{
								Address:             []byte("role2"),
								HasRoleMint:         true,
								HasRoleSetITOPrices: true,
							},
						},
					}, &kapps.UserKDA{}, nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 10 + 16, // base + addressesCost + roleBytesCost
		},
		{
			purpose: "Should return error because GetBytes returns error and consume only the base gas",
			mock: func() {
				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return nil, mockErr
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return the error if GetKdaTokenData returns error",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return nil, nil, mockErr
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return nil, nil
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
	}

	for _, tt := range cases {
		t.Run(tt.purpose, func(t *testing.T) {
			as := assert.New(t)

			totalGas = 0
			runtimeErr = nil

			tt.mock()

			vmhooks.ManagedGetKDARolesWithHost(host, 0, 1)

			as.Equal(tt.expectedError, runtimeErr)
			as.Equal(tt.expectedGas, totalGas)
		})
	}
}

func TestManagedGetSftMetadataWithHost_GasCost(t *testing.T) {
	t.Parallel()

	var totalGas uint64

	managedTypes := &hostmock.ManagedTypesContextMock{
		ConsumeGasForBytesCalled: func(bytes []byte) {
			totalGas += uint64(len(bytes))
		},
	}
	blockchain := &contextmock.BlockchainContextMock{}

	var runtimeErr error
	mockErr := errors.New("mock error")

	host := &contextmock.VMHostMock{
		ManagedTypesContext: managedTypes,
		// pinned pre-FixAuditChangesV6, as in TestManagedGetKDATokenDataWithHost_GasCost
		ForkControllerContext: &commonMock.ForkControllerStub{},
		MeteringContext: &contextmock.MeteringContextMock{
			GasCost: &config.GasCost{
				BaseOpsAPICost: config.BaseOpsAPICost{
					GetSFTMetadata: 10,
				},
			},
			UseAndTraceGasCalled: func(gas uint64) {
				totalGas += gas
			},
		},
		BlockchainContext: blockchain,
		RuntimeContext: &contextmock.RuntimeContextWrapper{
			BaseOpsErrorShouldFailExecutionFunc: func() bool {
				return true
			},
			FailExecutionFunc: func(err error) {
				runtimeErr = err
			},
		},
	}

	cases := []struct {
		purpose       string
		mock          func()
		expectedError error
		expectedGas   uint64
	}{
		{
			purpose: "Should return the base gas cost consumed",
			mock: func() {
				blockchain.GetSFTMetaCalled = func(asset []byte, nonce uint64) (*kapps.MetaV2, error) {
					return &kapps.MetaV2{
						Circulation: 10,
						MaxSupply:   5,
						Metadata: &kapps.MetaV2Data{
							Name:       nil,
							Hash:       nil,
							Attributes: nil,
						},
					}, nil
				}
			},
			expectedError: nil,
			expectedGas:   10,
		},
		{
			purpose: "Should return the base gas cost name, attributes addition",
			mock: func() {
				blockchain.GetSFTMetaCalled = func(asset []byte, nonce uint64) (*kapps.MetaV2, error) {
					return &kapps.MetaV2{
						Circulation: 10,
						MaxSupply:   5,
						Metadata: &kapps.MetaV2Data{
							Name:       []byte(strings.Repeat("1", 20)),
							Hash:       []byte(strings.Repeat("1", 30)),
							Attributes: []byte(strings.Repeat("1", 100)),
						},
					}, nil
				}
			},
			expectedError: nil,
			expectedGas:   160,
		},
		{
			purpose: "Should return error because GetSFTMeta returns error and consume only the base gas",
			mock: func() {
				blockchain.GetSFTMetaCalled = func(asset []byte, nonce uint64) (*kapps.MetaV2, error) {
					return nil, mockErr
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return error because GetBytes returns error and consume only the base gas",
			mock: func() {
				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return nil, mockErr
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
	}

	for _, tt := range cases {
		t.Run(tt.purpose, func(t *testing.T) {
			as := assert.New(t)

			totalGas = 0
			runtimeErr = nil

			tt.mock()

			vmhooks.ManagedGetSftMetadataWithHost(host, 0, 1, 2)

			as.Equal(tt.expectedError, runtimeErr)
			as.Equal(tt.expectedGas, totalGas)
		})
	}
}

func TestManagedGetUserKDAWithHost_GasCost(t *testing.T) {
	t.Parallel()

	var totalGas uint64

	managedTypes := &hostmock.ManagedTypesContextMock{
		ConsumeGasForBigIntCopyCalled: func(v ...*big.Int) {
			for _, v := range v {
				totalGas += v.Uint64()
			}
		},
		ConsumeGasForBytesCalled: func(bytes []byte) {
			totalGas += uint64(len(bytes))
		},
	}
	blockchain := &contextmock.BlockchainContextMock{}

	var runtimeErr error

	host := &contextmock.VMHostMock{
		ManagedTypesContext: managedTypes,
		// pinned pre-FixAuditChangesV6, as in TestManagedGetKDATokenDataWithHost_GasCost
		ForkControllerContext: &commonMock.ForkControllerStub{},
		MeteringContext: &contextmock.MeteringContextMock{
			GasCost: &config.GasCost{
				BaseOpsAPICost: config.BaseOpsAPICost{
					GetUserKDA: 10,
				},
			},
			UseAndTraceGasCalled: func(gas uint64) {
				totalGas += gas
			},
		},
		BlockchainContext: blockchain,
		RuntimeContext: &contextmock.RuntimeContextWrapper{
			BaseOpsErrorShouldFailExecutionFunc: func() bool {
				return true
			},
			FailExecutionFunc: func(err error) {
				runtimeErr = err
			},
		},
	}

	mockErr := errors.New("mock error")

	cases := []struct {
		purpose       string
		mock          func()
		expectedError error
		expectedGas   uint64
	}{
		{
			purpose: "Should return the base gas cost consumed",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10,
		},
		{
			purpose: "Should return the base gas cost consumed increased by gas cost for mime and metadata",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{}, &kapps.UserKDA{
						MIME:     []byte("1234567890"),
						Metadata: []byte("12345678901234567890"),
					}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 30,
		},
		{
			purpose: "Should return the base gas cost consumed increased by gas cost for each bucket",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{}, &kapps.UserKDA{
						Buckets: map[string]*kapps.UserBucket{
							"key1": {
								StakedAt:   1,
								Value:      1,
								Delegation: []byte("1234"),
							},
							"key2": {
								StakedAt:   1,
								Value:      1,
								Delegation: []byte("1234"),
							},
						},
					}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 48 + 20, // base + len(buckets) * default + bucketsCopy
		},
		{
			purpose: "Should return error because GetBytes returns error and consume only the base gas",
			mock: func() {
				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return nil, mockErr
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return error because the second GetBytes returns error and consume only the base gas",
			mock: func() {
				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), mockErr
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return the error if GetKdaTokenData returns error",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(address []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return nil, nil, mockErr
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
	}

	for _, tt := range cases {
		t.Run(tt.purpose, func(t *testing.T) {
			as := assert.New(t)

			addressHandle := int32(1)
			tickerHandle := int32(2)
			nonce := int64(0)

			balanceHandle := int32(3)
			frozenHandle := int32(4)
			lastClaimHandle := int32(5)
			bucketsHandle := int32(6)
			mimeHandle := int32(7)
			metadataHandle := int32(8)

			totalGas = 0
			runtimeErr = nil

			tt.mock()

			vmhooks.ManagedGetUserKDAWithHost(
				host,
				addressHandle,
				tickerHandle,
				nonce,
				balanceHandle,
				frozenHandle,
				lastClaimHandle,
				bucketsHandle,
				mimeHandle,
				metadataHandle,
			)
			as.Equal(tt.expectedError, runtimeErr)
			as.Equal(tt.expectedGas, totalGas)
		})
	}
}

func TestManagedGetKDATokenDataWithHost_GasCost(t *testing.T) {
	t.Parallel()

	var totalGas uint64

	managedTypes := &hostmock.ManagedTypesContextMock{
		ConsumeGasForBigIntCopyCalled: func(v ...*big.Int) {
			for _, v := range v {
				totalGas += v.Uint64()
			}
		},
		ConsumeGasForBytesCalled: func(bytes []byte) {
			totalGas += uint64(len(bytes))
		},
	}
	blockchain := &contextmock.BlockchainContextMock{}

	var runtimeErr error

	host := &contextmock.VMHostMock{
		ManagedTypesContext: managedTypes,
		// pinned pre-FixAuditChangesV6: this case counts gas through UseAndTraceGasCalled, which the
		// post-fork bounded charge does not go through. The fork's copy charge is covered two-sided
		// by TestVMHooksImpl_CopyGas_SameBeforeAndFromV6 instead
		ForkControllerContext: &commonMock.ForkControllerStub{},
		MeteringContext: &contextmock.MeteringContextMock{
			GasCost: &config.GasCost{
				BaseOpsAPICost: config.BaseOpsAPICost{
					GetKDATokenData: 10,
				},
			},
			UseAndTraceGasCalled: func(gas uint64) {
				totalGas += gas
			},
		},
		BlockchainContext: blockchain,
		RuntimeContext: &contextmock.RuntimeContextWrapper{
			BaseOpsErrorShouldFailExecutionFunc: func() bool {
				return true
			},
			FailExecutionFunc: func(err error) {
				runtimeErr = err
			},
		},
	}

	cases := []struct {
		purpose       string
		mock          func()
		expectedError error
		expectedGas   uint64
	}{
		{
			purpose: "Should return the base gas cost consumed",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10,
		},
		{
			purpose: "Should return the base gas cost consumed increased by royalties",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Royalties:  &kapps.RoyaltiesData{},
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 32,
		},
		{
			purpose: "Should return the base gas cost consumed increased by royalties with values",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Royalties: &kapps.RoyaltiesData{
							Address: []byte("1"),
							TransferPercentage: []*kapps.RoyaltyData{
								{
									Amount:     1,
									Percentage: 1,
								},
							},
							TransferFixed: 1,
							MarketFixed:   1,
							ITOFixed:      1,
							SplitRoyalties: map[string]*kapps.RoyaltySplitData{
								"1": {
									PercentTransferPercentage: 1,
									PercentTransferFixed:      1,
									PercentMarketPercentage:   1,
									PercentMarketFixed:        1,
									PercentITOPercentage:      1,
									PercentITOFixed:           1,
								},
							},
						},
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + 32 + 1 + 8 + 1 + 1 + 1 + 1 + 28 + 1,
			// base + royaltiesSize + addressCost + transferPercentageCost +
			// transferPercentageAmount + TransferFixed + MarketFixed + ITOFixed +
			// splitRoyaltiesCost (each) + keyCost
		},
		{
			purpose: "Should return the base gas cost consumed increased by roles",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Roles: []*kapps.RolesData{
							{
								Address:             []byte("1"),
								HasRoleMint:         true,
								HasRoleSetITOPrices: true,
							},
							{
								Address:             []byte("2"),
								HasRoleMint:         true,
								HasRoleSetITOPrices: true,
							},
						},
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + (8 * 2) + (1 * 2),
			// base + rolesCost * len(roles) + addressOfEachRoleCost
		},
		{
			purpose: "Should return the base gas cost consumed increased by URIs",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						URIs: map[string]string{
							"1": "1",
							"2": "1",
							"3": "1",
						},
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: nil,
			expectedGas:   10 + (8 * 3) + (1 * 3) + (1 * 3),
			// base + URIsCost * len(URIs) + keyOfEachURICost + valueOfEachURICost
		},
		{
			purpose: "Should return the error if GetBytes returns error",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), errors.New("invalid address")
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return the error if the second GetBytes returns error",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return &kapps.KDAData{
						Properties: &kapps.PropertiesData{},
						Attributes: &kapps.AttributesData{},
					}, &kapps.UserKDA{}, nil
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), errors.New("invalid address")
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
		{
			purpose: "Should return the error if GetKdaTokenData returns error",
			mock: func() {
				blockchain.GetKdaTokenDataCalled = func(addr []byte, ticker []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error) {
					return nil, nil, errors.New("err get KDA")
				}

				managedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					if handle == 1 {
						return []byte("address"), nil
					}

					return []byte("klv-1234"), nil
				}
			},
			expectedError: vmhost.ErrArgOutOfRange,
			expectedGas:   10,
		},
	}

	for _, tt := range cases {
		t.Run(tt.purpose, func(t *testing.T) {
			as := assert.New(t)

			totalGas = 0
			runtimeErr = nil

			tt.mock()

			addressHandle := int32(1)
			tickerHandle := int32(2)
			nonce := int64(0)

			precisionHandle := int32(1)
			idHandle := int32(2)
			nameHandle := int32(3)
			creatorHandle := int32(4)
			adminHandle := int32(5)
			logoHandle := int32(6)
			urisHandle := int32(7)
			initialSupplyHandle := int32(8)
			circulatingSupplyHandle := int32(9)
			maxSupplyHandle := int32(10)
			mintedHandle := int32(11)
			burnedHandle := int32(12)
			royaltiesHandle := int32(13)
			propertiesHandle := int32(14)
			attributesHandle := int32(15)
			rolesHandle := int32(16)
			issueDateHandle := int32(17)

			vmhooks.ManagedGetKDATokenDataWithHost(
				host,
				addressHandle,
				tickerHandle,
				nonce,
				precisionHandle,
				idHandle,
				nameHandle,
				creatorHandle,
				adminHandle,
				logoHandle,
				urisHandle,
				initialSupplyHandle,
				circulatingSupplyHandle,
				maxSupplyHandle,
				mintedHandle,
				burnedHandle,
				royaltiesHandle,
				propertiesHandle,
				attributesHandle,
				rolesHandle,
				issueDateHandle,
			)
			as.Equal(tt.expectedError, runtimeErr)
			as.Equal(tt.expectedGas, totalGas)
		})
	}
}

func TestManagedGetMultiKDACallValue(t *testing.T) {
	t.Parallel()

	host := newMockVMHost()
	host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			KDATransfers: []*vmcommon.KDATransfer{
				{
					KDAValue:     big.NewInt(100),
					KDATokenName: []byte("KFI"),
				},
				{
					KDAValue:     big.NewInt(200),
					KDATokenName: []byte("KLV"),
				},
				{
					KDAValue:     big.NewInt(300),
					KDATokenName: []byte("OTHER_KDA"),
				},
			},
		},
	}

	callValueHandle := int32(0)
	kdaHandler := int32(1)
	bigIntHandler := int32(2)
	var result []byte
	var resultBigInt *big.Int
	mockManaged := host.ManagedTypes().(*hostmock.ManagedTypesContextMock)
	mockManaged.SetBytesCalled = func(index int32, value []byte) {
		if index == callValueHandle {
			result = value
		}
	}
	mockManaged.GetBytesCalled = func(index int32) ([]byte, error) {
		if index == kdaHandler {
			return []byte("OTHER_KDA"), nil
		}
		return nil, nil
	}
	mockManaged.GetBigIntOrCreateCalled = func(index int32) *big.Int {
		if index == bigIntHandler {
			resultBigInt = big.NewInt(0)
			return resultBigInt
		}

		return nil
	}

	hooks := vmhooks.NewVMHooksImpl(host)

	hooks.ManagedGetKDACallValue(bigIntHandler, kdaHandler)
	assert.Equal(t, int64(300), resultBigInt.Int64())

	expected := []byte{
		0x0, 0x0, 0x0, 0x0, // Token KFI - Buffer 0
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // NONCE 0
		0x0, 0x0, 0x0, 0x0, // VALUE KFI - Buffer 0
		0x0, 0x0, 0x0, 0x1, // Token KLV - Buffer 1
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // NONCE 0
		0x0, 0x0, 0x0, 0x1, // VALUE KLV - Buffer 1
		0x0, 0x0, 0x0, 0x2, // Token OTHER_KDA - Buffer 2
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // NONCE 0
		0x0, 0x0, 0x0, 0x2, // VALUE OTHER_KDA - Buffer 2
	}
	hooks.ManagedGetMultiKDACallValue(callValueHandle)
	assert.Equal(t, expected, result)
	// decoded info
	assert.Equal(t, []byte("KFI"), mockManaged.GetManagedBuffer()[0])
	assert.Equal(t, int64(100), mockManaged.GetManagedBigInt()[0].Int64())
	assert.Equal(t, []byte("KLV"), mockManaged.GetManagedBuffer()[1])
	assert.Equal(t, int64(200), mockManaged.GetManagedBigInt()[1].Int64())
	assert.Equal(t, []byte("OTHER_KDA"), mockManaged.GetManagedBuffer()[2])
	assert.Equal(t, int64(300), mockManaged.GetManagedBigInt()[2].Int64())

	expected = []byte{
		0x0, 0x0, 0x0, 0x3, // Token KFI - Buffer 3
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // NONCE 0
		0x0, 0x0, 0x0, 0x3, // VALUE KFI - Buffer 3
		0x0, 0x0, 0x0, 0x4, // Token OTHER_KDA - Buffer 4
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // NONCE 0
		0x0, 0x0, 0x0, 0x4, // VALUE OTHER_KDA - Buffer 4
	}
	hooks.ManagedGetMultiKDAWithoutKLVCallValue(callValueHandle)
	assert.Equal(t, expected, result)
	// decoded info
	assert.Equal(t, []byte("KFI"), mockManaged.GetManagedBuffer()[3])
	assert.Equal(t, int64(100), mockManaged.GetManagedBigInt()[3].Int64())
	assert.Equal(t, []byte("OTHER_KDA"), mockManaged.GetManagedBuffer()[4])
	assert.Equal(t, int64(300), mockManaged.GetManagedBigInt()[4].Int64())
}

func TestManagedGetMultiKDAWithoutKLVCallValueDoesNotMutateVMInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		fixAuditChangesV4  bool
		transfers          []*vmcommon.KDATransfer
		expectedHookTokens [][]byte
		expectedHookValues []int64
		expectedTokens     [][]byte
		expectedOtherKDA   int64
		expectedKLV        int64
	}{
		{
			name:              "with fix, runtime transfers are left untouched",
			fixAuditChangesV4: true,
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100), KDATokenName: []byte("KFI")},
				{KDAValue: big.NewInt(200), KDATokenName: []byte("KLV")},
				{KDAValue: big.NewInt(300), KDATokenName: []byte("OTHER_KDA")},
			},
			expectedHookTokens: [][]byte{[]byte("KFI"), []byte("OTHER_KDA")},
			expectedHookValues: []int64{100, 300},
			expectedTokens:     [][]byte{[]byte("KFI"), []byte("KLV"), []byte("OTHER_KDA")},
			expectedOtherKDA:   300,
			expectedKLV:        200,
		},
		{
			// legacy behaviour: the in-place delete shifts OTHER_KDA over KLV and leaves
			// a stale duplicate behind, so KLV disappears and OTHER_KDA is counted twice
			name:              "without fix, runtime transfers keep the duplicated entry",
			fixAuditChangesV4: false,
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100), KDATokenName: []byte("KFI")},
				{KDAValue: big.NewInt(200), KDATokenName: []byte("KLV")},
				{KDAValue: big.NewInt(300), KDATokenName: []byte("OTHER_KDA")},
			},
			expectedHookTokens: [][]byte{[]byte("KFI"), []byte("OTHER_KDA")},
			expectedHookValues: []int64{100, 300},
			expectedTokens:     [][]byte{[]byte("KFI"), []byte("OTHER_KDA"), []byte("OTHER_KDA")},
			expectedOtherKDA:   600,
			expectedKLV:        0,
		},
		{
			// an empty token name means KLV too, so it hits the same delete-in-place branch
			name:              "with fix, empty token name is filtered without touching runtime transfers",
			fixAuditChangesV4: true,
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100), KDATokenName: []byte("KFI")},
				{KDAValue: big.NewInt(200), KDATokenName: []byte{}},
				{KDAValue: big.NewInt(300), KDATokenName: []byte("OTHER_KDA")},
			},
			expectedHookTokens: [][]byte{[]byte("KFI"), []byte("OTHER_KDA")},
			expectedHookValues: []int64{100, 300},
			expectedTokens:     [][]byte{[]byte("KFI"), []byte{}, []byte("OTHER_KDA")},
			expectedOtherKDA:   300,
			expectedKLV:        200,
		},
		{
			name:              "without fix, empty token name is NOT filtered but other getters properly count it ",
			fixAuditChangesV4: false,
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100), KDATokenName: []byte("KFI")},
				{KDAValue: big.NewInt(200), KDATokenName: []byte{}},
				{KDAValue: big.NewInt(300), KDATokenName: []byte("OTHER_KDA")},
			},
			expectedHookTokens: [][]byte{[]byte("KFI"), []byte{}, []byte("OTHER_KDA")},
			expectedHookValues: []int64{100, 200, 300},
			expectedTokens:     [][]byte{[]byte("KFI"), []byte{}, []byte("OTHER_KDA")},
			expectedOtherKDA:   300,
			expectedKLV:        200,
		},
		{
			name:              "with fix, all transfers filtered out returns an empty buffer",
			fixAuditChangesV4: true,
			transfers: []*vmcommon.KDATransfer{
				{KDAValue: big.NewInt(100), KDATokenName: []byte("KLV")},
				{KDAValue: big.NewInt(200), KDATokenName: []byte{}},
				{KDAValue: big.NewInt(300), KDATokenName: []byte("KLV")},
			},
			expectedTokens:   [][]byte{[]byte("KLV"), []byte{}, []byte("KLV")},
			expectedOtherKDA: 0,
			expectedKLV:      600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host := newMockVMHost()
			host.ForkControllerContext = &commonMock.ForkControllerStub{FixAuditChangesV4Value: tt.fixAuditChangesV4}
			host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput = &vmcommon.ContractCallInput{
				VMInput: vmcommon.VMInput{KDATransfers: tt.transfers},
			}

			callValueHandle := int32(0)
			mockManaged := host.ManagedTypes().(*hostmock.ManagedTypesContextMock)

			hooks := vmhooks.NewVMHooksImpl(host)

			// call the filtering hook first - the ordering is what hides the corruption
			hooks.ManagedGetMultiKDAWithoutKLVCallValue(callValueHandle)

			require.Len(t, mockManaged.GetManagedBuffer(), len(tt.expectedHookTokens))
			for i, expectedToken := range tt.expectedHookTokens {
				assert.Equal(t, expectedToken, mockManaged.GetManagedBuffer()[i])
				assert.Equal(t, tt.expectedHookValues[i], mockManaged.GetManagedBigInt()[i].Int64())
			}

			// what the rest of the execution sees afterwards
			vmInput := host.RuntimeContext.(*contextmock.RuntimeContextMock).VMInput
			require.Len(t, vmInput.KDATransfers, len(tt.expectedTokens))
			actualTokens := make([][]byte, 0, len(vmInput.KDATransfers))
			for _, transfer := range vmInput.KDATransfers {
				actualTokens = append(actualTokens, transfer.KDATokenName)
			}
			assert.Equal(t, tt.expectedTokens, actualTokens)
			assert.Equal(t, tt.expectedOtherKDA, vmInput.GetKDACallValue([]byte("OTHER_KDA")).Int64())
			assert.Equal(t, tt.expectedKLV, vmInput.GetKDACallValue([]byte("KLV")).Int64())
		})
	}
}

var (
	flatCloneOwner  = bytes.Repeat([]byte{0x01}, vmhost.AddressLen)
	flatCloneSigner = bytes.Repeat([]byte{0x02}, vmhost.AddressLen)
	flatCloneMeta   = []byte{vmcommon.MetadataUpgradeable, 0}
)

// stubFlatCloneAccounts serves flatCloneOwner, with code metadata flatCloneMeta and flatCloneSigner
// as its owner permission signer, and like the accounts DB finds no account under any other
// address. It returns the addresses looked up
func stubFlatCloneAccounts(host *contextmock.VMHostMock) *[][]byte {
	lookups := make([][]byte, 0)
	host.BlockchainContext.(*hostmock.BlockchainContextStub).GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
		lookups = append(lookups, address)
		if !bytes.Equal(address, flatCloneOwner) {
			return nil, common.ErrAccNotFound
		}
		acc := state.NewEmptyUserAccount()
		acc.SetCodeMetadata(flatCloneMeta)
		acc.UserAccountData.Permissions = []*state.Permission{{
			Type:      state.Permission_Owner,
			Threshold: 1,
			Signers:   []*state.Key{{Address: flatCloneSigner, Weight: 1}},
		}}
		return acc, nil
	}
	return &lookups
}

// ManagedGetCodeMetadata cloned the address, whatever its size, for a flat cost. From
// FixAuditChangesV6 the flat cost is bounded and the address is charged DataCopyPerByte for each of
// its bytes before it is cloned; it is then looked up whatever its length, as it always was, since
// an account can live under an address that is not vmhost.AddressLen long. Not parallel: it counts
// allocations
func TestManagedGetCodeMetadata_ChargedBeforeCloneFromV6(t *testing.T) {
	gasMap := flatCloneGasMap()
	fixedCost := gasMap["BaseOpsAPICost"]["GetCodeMetadata"] + gasMap["ManagedBufferAPICost"]["MBufferSetBytes"]
	dataCopyPerByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	untouched := []byte("untouched")
	oversized := append(append([]byte{}, flatCloneOwner...), make([]byte, flatCloneOversizedLen-vmhost.AddressLen)...)

	type scenario struct {
		name     string
		address  []byte
		missing  bool
		budget   uint64
		failErr  error
		response []byte
		lookedUp bool
		cloned   bool
		noClone  bool
	}

	for _, fixV6 := range []bool{false, true} {
		perByte := func(address []byte) uint64 {
			if !fixV6 {
				return 0
			}
			return dataCopyPerByte * uint64(len(address))
		}
		scenarios := []scenario{
			{name: "owner", address: flatCloneOwner, response: flatCloneMeta, lookedUp: true},
			{name: "unknown", address: flatCloneSigner, failErr: common.ErrAccNotFound, lookedUp: true},
			{name: "short", address: flatCloneOwner[:vmhost.AddressLen-1], failErr: common.ErrAccNotFound, lookedUp: true},
			// one byte past vmhost.AddressLen is charged and looked up like any other length
			{name: "oneTooLong", address: append(append([]byte{}, flatCloneOwner...), 0),
				failErr: common.ErrAccNotFound, lookedUp: true},
			{name: "oversized", address: oversized, failErr: common.ErrAccNotFound, lookedUp: true, cloned: true},
			{name: "missing", missing: true, failErr: vmhost.ErrNoManagedBufferUnderThisHandle},
		}
		if fixV6 {
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", address: oversized, budget: fixedCost - 1,
					failErr: vmhost.ErrNotEnoughGas, noClone: true},
				scenario{name: "perByteShortfall", address: oversized, budget: fixedCost + perByte(oversized) - 1,
					failErr: vmhost.ErrNotEnoughGas, noClone: true},
				scenario{name: "ownerPerByteShortfall", address: flatCloneOwner, budget: fixedCost + perByte(flatCloneOwner) - 1,
					failErr: vmhost.ErrNotEnoughGas},
			)
		} else {
			// pre-fork the flat cost overdraws and the hook carries on
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", address: flatCloneOwner, budget: fixedCost - 1,
					response: flatCloneMeta, lookedUp: true},
			)
		}

		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("FixAuditChangesV6=%v/%s", fixV6, sc.name), func(t *testing.T) {
				// a fault burns the gas left, so a row is run on exactly the gas it should use: a
				// charge past it would fail with ErrNotEnoughGas instead, and from the fork one gas
				// less must, or the charge fell short
				expectedGas := fixedCost + perByte(sc.address)
				run := func(budget uint64) (*contextmock.RuntimeContextMock, uint64, uint64, int, []byte) {
					host, runtime := newFlatCloneHost(t, fixV6)
					lookups := stubFlatCloneAccounts(host)
					managedTypes := host.ManagedTypes()
					addressHandle := int32(1234)
					if !sc.missing {
						addressHandle = managedTypes.NewManagedBufferFromBytes(sc.address)
					}
					responseHandle := managedTypes.NewManagedBufferFromBytes(untouched)

					gasUsed, allocated := runFlatCloneHook(host, budget, func() {
						vmhooks.ManagedGetCodeMetadataWithHost(host, addressHandle, responseHandle)
					})

					response, err := managedTypes.GetBytes(responseHandle)
					require.NoError(t, err)
					return runtime, gasUsed, allocated, len(*lookups), response
				}

				budget := sc.budget
				if budget == 0 {
					budget = expectedGas
				}
				runtime, gasUsed, allocated, lookups, response := run(budget)

				if sc.failErr != nil {
					require.ErrorIs(t, runtime.FailExecutionErr, sc.failErr)
				} else {
					require.Nil(t, runtime.FailExecutionErr)
					assert.Equal(t, expectedGas, gasUsed)
				}
				if sc.response != nil {
					assert.Equal(t, sc.response, response)
				} else {
					assert.Equal(t, untouched, response)
				}
				assert.Equal(t, sc.lookedUp, lookups == 1, "lookups: %d", lookups)
				if sc.cloned {
					assert.GreaterOrEqual(t, allocated, uint64(flatCloneOversizedLen))
				}
				if sc.noClone {
					assert.Less(t, allocated, uint64(flatCloneNoCloneBelow), "the address must not be cloned")
				}

				if fixV6 && sc.budget == 0 {
					runtime, _, _, lookups, response = run(expectedGas - 1)
					require.ErrorIs(t, runtime.FailExecutionErr, vmhost.ErrNotEnoughGas)
					assert.Equal(t, untouched, response)
					assert.Zero(t, lookups)
				}
			})
		}
	}
}

// ManagedAccHasPerm cloned both addresses, whatever their size, for a flat cost. From
// FixAuditChangesV6 the flat cost is bounded and each address is charged DataCopyPerByte for each
// of its bytes right before it is cloned, the source first, so a missing source handle is never
// charged the target (an unknown source account is, as it is looked up after both clones); both
// are then answered as they always were, whatever their length. Not
// parallel: it counts allocations
func TestManagedAccHasPerm_ChargedBeforeCloneFromV6(t *testing.T) {
	gasMap := flatCloneGasMap()
	fixedCost := gasMap["BaseOpsAPICost"]["Int64Finish"] + gasMap["BaseOpsAPICost"]["GetOwnerAddress"]
	dataCopyPerByte := gasMap["BaseOperationCost"]["DataCopyPerByte"]
	oversized := func(prefix []byte) []byte {
		return append(append([]byte{}, prefix...), make([]byte, flatCloneOversizedLen-len(prefix))...)
	}

	type scenario struct {
		name        string
		ops         int64
		source      []byte
		target      []byte
		missingSrc  bool
		missingTgt  bool
		budget      uint64
		failErr     error
		result      int32
		lookups     int
		chargedSrc  bool
		chargedTgt  bool
		clonedBytes bool
		noCloneLen  bool
	}

	for _, fixV6 := range []bool{false, true} {
		perByte := func(address []byte) uint64 {
			if !fixV6 {
				return 0
			}
			return dataCopyPerByte * uint64(len(address))
		}
		scenarios := []scenario{
			{name: "granted", source: flatCloneOwner, target: flatCloneSigner, result: 1, lookups: 1,
				chargedSrc: true, chargedTgt: true},
			{name: "notGranted", source: flatCloneOwner, target: flatCloneOwner, result: 0, lookups: 1,
				chargedSrc: true, chargedTgt: true},
			{name: "shortTarget", source: flatCloneOwner, target: flatCloneSigner[:vmhost.AddressLen-1], result: 0, lookups: 1,
				chargedSrc: true, chargedTgt: true},
			{name: "oversizedTarget", source: flatCloneOwner, target: oversized(flatCloneSigner), result: 0, lookups: 1,
				chargedSrc: true, chargedTgt: true, clonedBytes: true},
			{name: "oversizedTargetUnknownSource", source: flatCloneSigner, target: oversized(flatCloneSigner),
				failErr: vmhost.ErrArgOutOfRange, lookups: 1, chargedSrc: true, chargedTgt: true, clonedBytes: true},
			// a missing source faults before the target is charged or cloned
			{name: "oversizedTargetMissingSource", missingSrc: true, target: oversized(flatCloneSigner),
				failErr: vmhost.ErrArgOutOfRange, noCloneLen: true},
			{name: "oversizedSource", source: oversized(flatCloneOwner), target: flatCloneSigner,
				failErr: vmhost.ErrArgOutOfRange, lookups: 1, chargedSrc: true, chargedTgt: true, clonedBytes: true},
			{name: "oversizedSourceMissingTarget", source: oversized(flatCloneOwner), missingTgt: true,
				failErr: vmhost.ErrArgOutOfRange, chargedSrc: true, clonedBytes: true},
			{name: "missingTarget", source: flatCloneOwner, missingTgt: true, failErr: vmhost.ErrArgOutOfRange,
				chargedSrc: true},
			{name: "missingSource", missingSrc: true, target: flatCloneSigner, failErr: vmhost.ErrArgOutOfRange},
			// negative ops faults before either address is charged or cloned
			{name: "negativeOps", ops: -1, source: oversized(flatCloneOwner), target: oversized(flatCloneSigner),
				failErr: vmhost.ErrArgOutOfRange, noCloneLen: true},
			// one byte past vmhost.AddressLen is charged and looked up like any other length
			{name: "oneTooLongSource", source: append(append([]byte{}, flatCloneOwner...), 0), target: flatCloneSigner,
				failErr: vmhost.ErrArgOutOfRange, lookups: 1, chargedSrc: true, chargedTgt: true},
			{name: "oneTooLongTarget", source: flatCloneOwner, target: append(append([]byte{}, flatCloneSigner...), 0),
				result: 0, lookups: 1, chargedSrc: true, chargedTgt: true},
		}
		if fixV6 {
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", source: oversized(flatCloneOwner), target: oversized(flatCloneSigner),
					budget: fixedCost - 1, failErr: vmhost.ErrNotEnoughGas, noCloneLen: true},
				scenario{name: "sourceShortfall", source: oversized(flatCloneOwner), target: flatCloneSigner,
					budget: fixedCost + perByte(oversized(flatCloneOwner)) - 1, failErr: vmhost.ErrNotEnoughGas, noCloneLen: true},
				scenario{name: "targetShortfall", source: flatCloneOwner, target: oversized(flatCloneSigner),
					budget:  fixedCost + perByte(flatCloneOwner) + perByte(oversized(flatCloneSigner)) - 1,
					failErr: vmhost.ErrNotEnoughGas, noCloneLen: true},
				scenario{name: "honestShortfall", source: flatCloneOwner, target: flatCloneSigner,
					budget: fixedCost + perByte(flatCloneOwner) + perByte(flatCloneSigner) - 1, failErr: vmhost.ErrNotEnoughGas},
			)
		} else {
			// pre-fork the flat cost overdraws and the hook carries on
			scenarios = append(scenarios,
				scenario{name: "fixedShortfall", source: flatCloneOwner, target: flatCloneSigner,
					budget: fixedCost - 1, result: 1, lookups: 1},
			)
		}

		for _, sc := range scenarios {
			t.Run(fmt.Sprintf("FixAuditChangesV6=%v/%s", fixV6, sc.name), func(t *testing.T) {
				// a fault burns the gas left, so a row is run on exactly the gas it should use: a
				// charge past it would fail with ErrNotEnoughGas instead, and from the fork one gas
				// less must, or the charge fell short
				expectedGas := fixedCost
				if sc.chargedSrc {
					expectedGas += perByte(sc.source)
				}
				if sc.chargedTgt {
					expectedGas += perByte(sc.target)
				}
				ops := sc.ops
				if ops == 0 {
					ops = 1
				}
				run := func(budget uint64) (*contextmock.RuntimeContextMock, int32, uint64, uint64, int) {
					host, runtime := newFlatCloneHost(t, fixV6)
					lookups := stubFlatCloneAccounts(host)
					managedTypes := host.ManagedTypes()
					sourceHandle, targetHandle := int32(1234), int32(4321)
					if !sc.missingSrc {
						sourceHandle = managedTypes.NewManagedBufferFromBytes(sc.source)
					}
					if !sc.missingTgt {
						targetHandle = managedTypes.NewManagedBufferFromBytes(sc.target)
					}

					var result int32
					gasUsed, allocated := runFlatCloneHook(host, budget, func() {
						result = vmhooks.ManagedAccHasPermWithHost(host, ops, sourceHandle, targetHandle)
					})
					return runtime, result, gasUsed, allocated, len(*lookups)
				}

				budget := sc.budget
				if budget == 0 {
					budget = expectedGas
				}
				runtime, result, gasUsed, allocated, lookups := run(budget)

				assert.Equal(t, sc.result, result)
				if sc.failErr != nil {
					require.ErrorIs(t, runtime.FailExecutionErr, sc.failErr)
				} else {
					require.Nil(t, runtime.FailExecutionErr)
					assert.Equal(t, expectedGas, gasUsed)
				}
				assert.Equal(t, sc.lookups, lookups)
				if sc.clonedBytes {
					assert.GreaterOrEqual(t, allocated, uint64(flatCloneOversizedLen))
				} else if sc.noCloneLen {
					assert.Less(t, allocated, uint64(flatCloneNoCloneBelow), "no oversized address may be cloned")
				}

				if fixV6 && sc.budget == 0 {
					runtime, result, _, _, lookups = run(expectedGas - 1)
					require.ErrorIs(t, runtime.FailExecutionErr, vmhost.ErrNotEnoughGas)
					assert.Zero(t, result)
					assert.Zero(t, lookups)
				}
			})
		}
	}
}

func TestVMHooksImpl_ManagedContractAccountReadsCountTowardTrieReadsLimit(t *testing.T) {
	t.Parallel()

	countAccountLookup := func(blockchain *hostmock.BlockchainContextStub, lookups *int) {
		blockchain.GetUserAccountCalled = func(_ []byte) (state.UserAccountHandler, error) {
			*lookups++
			return state.NewEmptyUserAccount(), nil
		}
	}

	runContractAccountReadCases(t, []contractAccountReadCase{
		{
			name: "ManagedOwnerAddress",
			install: func(blockchain *hostmock.BlockchainContextStub, lookups *int) {
				blockchain.GetOwnerAddressCalled = func() ([]byte, error) {
					*lookups++
					return []byte("owner"), nil
				}
			},
			call: func(host *contextmock.VMHostMock) {
				vmhooks.NewVMHooksImpl(host).ManagedOwnerAddress(0)
			},
		},
		{
			name:    "ManagedGetCodeMetadata",
			install: countAccountLookup,
			call: func(host *contextmock.VMHostMock) {
				vmhooks.NewVMHooksImpl(host).ManagedGetCodeMetadata(1, 0)
			},
		},
		{
			name:    "ManagedAccHasPerm",
			install: countAccountLookup,
			call: func(host *contextmock.VMHostMock) {
				_ = vmhooks.NewVMHooksImpl(host).ManagedAccHasPerm(1, 1, 2)
			},
		},
	})
}

// addressHooksSeedAccount stores an account under address, with code metadata
// addressHooksSeededMeta and an owner permission signed by signer, as a zero-amount transfer to a
// junk address could leave one in state before FixAuditChangesV6
func addressHooksSeedAccount(t *testing.T, world *worldmock.MockWorld, address []byte, signer []byte) {
	t.Helper()
	acc, err := world.AccountsCacher.LoadUser(address)
	require.NoError(t, err)
	acc.SetCodeMetadata(addressHooksSeededMeta)
	acc.SetPermissions([]*state.Permission{{
		Type:      state.Permission_Owner,
		Threshold: 1,
		Signers:   []*state.Key{{Address: signer, Weight: 1}},
	}})
	require.NoError(t, world.AccountsCacher.UpdateUser(acc))
}

var addressHooksSeededMeta = []byte{vmcommon.MetadataPayable, 0}

// What the address hooks answer for an address that is not vmhost.AddressLen long, on the real
// accounts DB, must not change at FixAuditChangesV6, down to the return message, whether or not an
// account lives under it: from the fork they only pay for the copy first
func TestAddressHooks_OversizedAddressSameResultAcrossV6(t *testing.T) {
	oversized := append(append([]byte{}, test.ParentAddress...), make([]byte, flatCloneOversizedLen)...)
	// accounts a zero-amount transfer to a junk address could have created before the fork
	longAddress := bytes.Repeat([]byte{0x06}, vmhost.AddressLen+8)
	shortAddress := bytes.Repeat([]byte{0x05}, vmhost.AddressLen-1)
	longSignerOwner := bytes.Repeat([]byte{0x07}, vmhost.AddressLen)
	seed := func(t *testing.T) func(host vmhost.VMHost, world *worldmock.MockWorld) {
		return func(host vmhost.VMHost, world *worldmock.MockWorld) {
			addressHooksSeedAccount(t, world, longAddress, test.ParentAddress)
			addressHooksSeedAccount(t, world, shortAddress, test.ParentAddress)
			addressHooksSeedAccount(t, world, longSignerOwner, longAddress)
		}
	}

	type outcome struct {
		result        int32
		returnCode    vmcommon.ReturnCode
		returnMessage string
		response      []byte
	}
	codeMetadata := func(address []byte) func(host vmhost.VMHost) int32 {
		return func(host vmhost.VMHost) int32 {
			managedTypes := host.ManagedTypes()
			response := managedTypes.NewManagedBufferFromBytes([]byte("untouched"))
			vmhooks.ManagedGetCodeMetadataWithHost(host, managedTypes.NewManagedBufferFromBytes(address), response)
			return response
		}
	}
	accHasPerm := func(source, target []byte) func(host vmhost.VMHost) int32 {
		return func(host vmhost.VMHost) int32 {
			managedTypes := host.ManagedTypes()
			return vmhooks.ManagedAccHasPermWithHost(host, 1,
				managedTypes.NewManagedBufferFromBytes(source), managedTypes.NewManagedBufferFromBytes(target))
		}
	}
	type hookCase struct {
		hook         func(host vmhost.VMHost) int32
		codeMetadata bool
		// when set, what the hook answers pre-fork for an account that exists, so the comparison
		// cannot pass on two lookups that both missed
		wantResult   int32
		wantResponse []byte
	}
	hooks := map[string]hookCase{
		"ManagedGetCodeMetadata":                    {hook: codeMetadata(oversized), codeMetadata: true},
		"ManagedGetCodeMetadata/existingLongAddr":   {hook: codeMetadata(longAddress), codeMetadata: true, wantResponse: addressHooksSeededMeta},
		"ManagedGetCodeMetadata/existingShortAddr":  {hook: codeMetadata(shortAddress), codeMetadata: true, wantResponse: addressHooksSeededMeta},
		"ManagedAccHasPerm/oversizedSource":         {hook: accHasPerm(oversized, test.ParentAddress)},
		"ManagedAccHasPerm/oversizedTarget":         {hook: accHasPerm(test.ParentAddress, oversized)},
		"ManagedAccHasPerm/existingLongSource":      {hook: accHasPerm(longAddress, test.ParentAddress), wantResult: 1},
		"ManagedAccHasPerm/existingShortSource":     {hook: accHasPerm(shortAddress, test.ParentAddress), wantResult: 1},
		"ManagedAccHasPerm/longSignerTarget":        {hook: accHasPerm(longSignerOwner, longAddress), wantResult: 1},
		"ManagedAccHasPerm/existingLongSourceShort": {hook: accHasPerm(longAddress, shortAddress)},
	}

	for name, hc := range hooks {
		t.Run(name, func(t *testing.T) {
			outcomes := make([]outcome, 0, 2)
			for _, epochs := range []blockchainConfig.EnableEpochs{preForkV6EnableEpochs(), postForkV6EnableEpochs()} {
				var out outcome
				// enough gas for the post-fork copy charge of the oversized address
				run := runManagedHookInWorld(t, epochs, 100_000_000, seed(t), func(host vmhost.VMHost) func() {
					return func() {
						out.result = hc.hook(host)
						if hc.codeMetadata {
							out.response, _ = host.ManagedTypes().GetBytes(out.result)
						}
					}
				})
				out.returnCode, out.returnMessage = run.returnCode, run.returnMessage
				outcomes = append(outcomes, out)
			}
			if hc.wantResponse != nil {
				require.Equal(t, vmcommon.Ok, outcomes[0].returnCode, outcomes[0].returnMessage)
				require.Equal(t, hc.wantResponse, outcomes[0].response)
			}
			if hc.wantResult != 0 {
				require.Equal(t, vmcommon.Ok, outcomes[0].returnCode, outcomes[0].returnMessage)
				require.Equal(t, hc.wantResult, outcomes[0].result)
			}
			assert.Equal(t, outcomes[0], outcomes[1])
		})
	}
}
