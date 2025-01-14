package vmhooks_test

import (
	"errors"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
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
		id[i] = char
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
					},
				})

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// As it now exists the value retrieved must be non-zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, expectedKDAValue)

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

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
					},
				})

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// As it now exists the value retrieved must be non-zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, targetTransfer.KDAValue)

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

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
					},
				})

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				// It will get an error because the ID token is not registered in the VM memoryll set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// Must continue as zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, big.NewInt(0))

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

				assert.Equal(t, hooksRuntime.GetPointsUsed(), totalGas)

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
					},
				})

				// Invoking test target hook who will set kdaCallValueHandle on kdaHandle
				// It will get an error because the ID token is not registered in the VM memoryll set kdaCallValueHandle on kdaHandle
				hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

				// Must continue as zero
				result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
				assert.Equal(t, result, big.NewInt(0))

				// Calculate gas used
				loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

				assert.Equal(t, hooksRuntime.GetPointsUsed(), totalGas)

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

		// Sets KDA ID into bytes on memory using kdaHandle
		hooksMgdType.SetBytes(kdaHandle, []byte("TEST-H7K7"))

		// As it does not exist, it will be created
		initialKDAValue := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
		assert.Equal(t, initialKDAValue, big.NewInt(0))

		hooksRuntime.SetVMInput(&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				KDATransfers: []*vmcommon.KDATransfer{},
			},
		})

		// Invoking test target hook with empty transfers slice, so no value will be set
		hooks.ManagedGetKDACallValue(kdaCallValueHandle, kdaHandle)

		// As its does not was set, the value retrieved must be zero
		result := hooksMgdType.GetBigIntOrCreate(kdaCallValueHandle)
		assert.Equal(t, result, big.NewInt(0))

		// Calculate gas used
		loopGas := hooksMetering.GasSchedule().WASMOpcodeCost.Loop
		callValueGas := hooksMetering.GasSchedule().BaseOpsAPICost.GetCallValue
		totalGas := (uint64(loopGas))*uint64(
			len(hooksRuntime.GetVMInput().KDATransfers),
		) + callValueGas

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
		MeteringContext:     mockMetering,
		RuntimeContext:      mockRuntime,
		ManagedTypesContext: mType,
		BlockchainContext:   blockchain,
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
