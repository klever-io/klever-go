package contexts

import (
	"bytes"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

var reservedTestPrefix = [][]byte{[]byte("RESERVED")}

func TestNewStorageContext(t *testing.T) {
	t.Parallel()

	t.Run("empty protected key prefix should error", func(t *testing.T) {
		t.Parallel()

		host := &contextmock.VMHostMock{}
		mockBlockchain := worldmock.NewMockWorld()

		storageCtx, err := NewStorageContext(host, mockBlockchain, make([][]byte, 0))
		require.Equal(t, vmhost.ErrEmptyProtectedKeyPrefix, err)
		require.True(t, check.IfNil(storageCtx))
	})
	t.Run("nil VM host should error", func(t *testing.T) {
		t.Parallel()

		mockBlockchain := worldmock.NewMockWorld()

		storageCtx, err := NewStorageContext(nil, mockBlockchain, reservedTestPrefix)
		require.Equal(t, vmhost.ErrNilVMHost, err)
		require.True(t, check.IfNil(storageCtx))
	})
	t.Run("nil blockchain hook should error", func(t *testing.T) {
		t.Parallel()

		host := &contextmock.VMHostMock{}

		storageCtx, err := NewStorageContext(host, nil, reservedTestPrefix)
		require.Equal(t, vmhost.ErrNilBlockChainHook, err)
		require.True(t, check.IfNil(storageCtx))
	})
	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		host := &contextmock.VMHostMock{}
		mockBlockchain := worldmock.NewMockWorld()

		storageCtx, err := NewStorageContext(host, mockBlockchain, reservedTestPrefix)
		require.Nil(t, err)
		require.False(t, check.IfNil(storageCtx))
	})
}

func TestStorageContext_SetAddress(t *testing.T) {
	t.Parallel()

	addressA := []byte("accountA")
	addressB := []byte("accountB")
	stubOutput := &contextmock.OutputContextStub{}
	accountA := &vmcommon.OutputAccount{
		Address:        addressA,
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
	}
	accountB := &vmcommon.OutputAccount{
		Address:        addressB,
		StorageUpdates: make(map[string]*vmcommon.StorageUpdate),
	}
	stubOutput.GetOutputAccountCalled = func(address []byte) (*vmcommon.OutputAccount, bool) {
		if bytes.Equal(address, addressA) {
			return accountA, false
		}
		if bytes.Equal(address, addressB) {
			return accountB, false
		}
		return nil, false
	}

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(config.MakeGasMapForTests())
	mockMetering.BlockGasLimitMock = uint64(15000)
	mockMetering.GasLeftMock = 20000

	host := &contextmock.VMHostMock{
		OutputContext:         stubOutput,
		MeteringContext:       mockMetering,
		RuntimeContext:        mockRuntime,
		ForkControllerContext: &mock.ForkControllerStub{},
	}
	bcHook := &contextmock.BlockchainHookStub{}

	storageCtx, _ := NewStorageContext(host, bcHook, reservedTestPrefix)

	keyA := []byte("keyA")
	valueA := []byte("valueA")

	storageCtx.SetAddress(addressA)
	storageStatus, err := storageCtx.SetStorage(keyA, valueA)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	require.Equal(t, uint64(len(valueA)), accountA.BytesAddedToStorage)
	require.Equal(t, uint64(0), accountA.BytesDeletedFromStorage)
	foundValueA, _, _, err := storageCtx.GetStorage(keyA)
	require.Nil(t, err)
	require.Equal(t, valueA, foundValueA)
	require.Len(t, storageCtx.GetStorageUpdates(addressA), 1)
	require.Len(t, storageCtx.GetStorageUpdates(addressB), 0)

	keyB := []byte("keyB")
	valueB := []byte("valueB")
	storageCtx.SetAddress(addressB)
	storageStatus, err = storageCtx.SetStorage(keyB, valueB)
	require.Equal(t, uint64(len(valueB)), accountB.BytesAddedToStorage)
	require.Equal(t, uint64(0), accountB.BytesDeletedFromStorage)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	foundValueB, _, _, err := storageCtx.GetStorage(keyB)
	require.Nil(t, err)
	require.Equal(t, valueB, foundValueB)
	require.Len(t, storageCtx.GetStorageUpdates(addressA), 1)
	require.Len(t, storageCtx.GetStorageUpdates(addressB), 1)
	foundValueA, _, _, err = storageCtx.GetStorage(keyA)
	require.Nil(t, err)
	require.Equal(t, []byte(nil), foundValueA)
}

func TestStorageContext_GetStorageUpdates(t *testing.T) {
	t.Parallel()

	mockOutput := &contextmock.OutputContextMock{}
	account := mockOutput.NewVMOutputAccount([]byte("account"))
	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = false

	account.StorageUpdates["update"] = &vmcommon.StorageUpdate{
		Offset: []byte("update"),
		Data:   []byte("some data"),
	}

	host := &contextmock.VMHostMock{
		OutputContext: mockOutput,
	}

	mockBlockchainHook := worldmock.NewMockWorld()
	storageCtx, _ := NewStorageContext(host, mockBlockchainHook, reservedTestPrefix)

	storageUpdates := storageCtx.GetStorageUpdates([]byte("account"))
	require.Equal(t, 1, len(storageUpdates))
	require.Equal(t, []byte("update"), storageUpdates["update"].Offset)
	require.Equal(t, []byte("some data"), storageUpdates["update"].Data)
}

func TestStorageContext_SetStorage(t *testing.T) {
	t.Parallel()

	address := []byte("account")
	mockOutput := &contextmock.OutputContextMock{}
	account := mockOutput.NewVMOutputAccount(address)
	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = false

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(config.MakeGasMapForTests())
	mockMetering.BlockGasLimitMock = uint64(15000)
	mockMetering.GasLeftMock = 20000

	host := &contextmock.VMHostMock{
		OutputContext:         mockOutput,
		MeteringContext:       mockMetering,
		RuntimeContext:        mockRuntime,
		ForkControllerContext: &mock.ForkControllerStub{},
	}
	bcHook := &contextmock.BlockchainHookStub{}
	storageCtx, _ := NewStorageContext(host, bcHook, reservedTestPrefix)
	storageCtx.SetAddress(address)

	val1 := []byte("value")
	val2 := []byte("newValue")
	val3 := []byte("v")

	key := []byte("key")
	value := val1
	addedBytes := len(value)

	storageStatus, err := storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(0), account.BytesDeletedFromStorage)
	foundValue, _, _, err := storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	value = val2
	addedBytes += len(value) - len(val1)

	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(0), account.BytesDeletedFromStorage)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	value = val2

	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(0), account.BytesDeletedFromStorage)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	value = val1
	deletedBytes := len(val2) - len(val1)

	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(deletedBytes), account.BytesDeletedFromStorage)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	value = val3
	deletedBytes += len(val1) - len(val3)

	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(deletedBytes), account.BytesDeletedFromStorage)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	value = nil
	deletedBytes += len(val3)

	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageDeleted, storageStatus)
	require.Equal(t, uint64(addedBytes), account.BytesAddedToStorage)
	require.Equal(t, uint64(deletedBytes), account.BytesDeletedFromStorage)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, []byte{}, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	mockRuntime.SetReadOnly(true)
	value = val2
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Equal(t, err, vmhost.ErrCannotWriteOnReadOnly)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, []byte{}, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	mockRuntime.SetReadOnly(false)
	key = []byte("other_key")
	value = []byte("other_value")
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	foundValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, value, foundValue)
	require.Len(t, storageCtx.GetStorageUpdates(address), 2)

	key = []byte("RESERVEDkey")
	value = []byte("doesn't matter")
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.Equal(t, vmhost.ErrStoreReservedKey, err)

	key = []byte("RESERVED")
	value = []byte("doesn't matter")
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.Equal(t, vmhost.ErrStoreReservedKey, err)

	key = []byte("mykey")
	timeLockKeyPrefix := string(storageCtx.GetVmProtectedPrefix(vmhost.TimeLockKeyPrefix))
	timeLockKey := vmhost.CustomStorageKey(timeLockKeyPrefix, key)
	lockTimestamp := big.NewInt(0).SetInt64(time.Now().Unix() + 3600).Bytes()

	storageStatus, err = storageCtx.SetProtectedStorage(timeLockKey, lockTimestamp)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)

	foundValue, _, usedCache, err := storageCtx.GetStorage(timeLockKey)
	require.Nil(t, err)
	require.Equal(t, lockTimestamp, foundValue)
	// To validate when the key is protected and has the VM prefix, the value will be checked in the cache
	require.True(t, usedCache)
}

func TestStorageContext_SetStorage_GasUsage(t *testing.T) {
	address := []byte("account")
	mockOutput := &contextmock.OutputContextMock{}
	account := mockOutput.NewVMOutputAccount(address)
	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = false

	storeCost := 11
	persistCost := 7
	releaseCost := 5

	gasMap := config.MakeGasMapForTests()
	gasMap["BaseOperationCost"]["StorePerByte"] = uint64(storeCost)
	gasMap["BaseOperationCost"]["PersistPerByte"] = uint64(persistCost)
	gasMap["BaseOperationCost"]["ReleasePerByte"] = uint64(releaseCost)

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(gasMap)
	mockMetering.BlockGasLimitMock = uint64(15000)

	host := &contextmock.VMHostMock{
		OutputContext:         mockOutput,
		MeteringContext:       mockMetering,
		RuntimeContext:        mockRuntime,
		ForkControllerContext: &mock.ForkControllerStub{},
	}
	bcHook := &contextmock.BlockchainHookStub{}

	storageCtx, _ := NewStorageContext(host, bcHook, reservedTestPrefix)
	storageCtx.SetAddress(address)

	gasProvided := 100
	mockMetering.GasLeftMock = uint64(gasProvided)
	key := []byte("key")

	// Store new value
	value := []byte("value")
	storageStatus, _ := storageCtx.SetStorage(key, value)
	gasLeft := gasProvided - storeCost*len(value)
	storedValue, _, _, err := storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	require.Equal(t, gasLeft, int(mockMetering.GasLeft()))
	require.Equal(t, value, storedValue)

	// Update with longer value
	value2 := []byte("value2")
	mockMetering.GasLeftMock = uint64(gasProvided)
	storageStatus, err = storageCtx.SetStorage(key, value2)
	require.Nil(t, err)
	storedValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	gasLeft = gasProvided - persistCost*len(value) - storeCost*(len(value2)-len(value))
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, gasLeft, int(mockMetering.GasLeft()))
	require.Equal(t, value2, storedValue)

	// Revert to initial value
	mockMetering.GasLeftMock = uint64(gasProvided)
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	gasLeft = gasProvided - persistCost*len(value)
	storedValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, gasLeft, int(mockMetering.GasLeft()))
	require.Equal(t, value, storedValue)

	// write same amout of bytes -- Before AuditChanges fork
	value3 := []byte("eulav")
	mockMetering.GasLeftMock = uint64(gasProvided)
	storageStatus, err = storageCtx.SetStorage(key, value3)
	require.Nil(t, err)
	gasLeft = gasProvided
	storedValue, _, _, err = storageCtx.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, gasLeft, int(mockMetering.GasLeft()))
	require.Equal(t, value3, storedValue)

	// write same amout of bytes -- After AuditChanges fork
	hostFork := &contextmock.VMHostMock{
		OutputContext:   mockOutput,
		MeteringContext: mockMetering,
		RuntimeContext:  mockRuntime,
		ForkControllerContext: &mock.ForkControllerStub{
			FixAuditChangesValue: true,
		},
	}

	storageCtxFork, _ := NewStorageContext(hostFork, bcHook, reservedTestPrefix)
	storageCtxFork.SetAddress(address)

	value4 := []byte("lorem")
	mockMetering.GasLeftMock = uint64(gasProvided)
	storageStatus, err = storageCtxFork.SetStorage(key, value4)
	require.Nil(t, err)
	gasLeft = gasProvided - persistCost*len(value)
	storedValue, _, _, err = storageCtxFork.GetStorage(key)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageModified, storageStatus)
	require.Equal(t, gasLeft, int(mockMetering.GasLeft()))
	require.Equal(t, value4, storedValue)
}

func TestStorageContext_StorageProtection(t *testing.T) {
	address := []byte("account")
	mockOutput := &contextmock.OutputContextMock{}
	account := mockOutput.NewVMOutputAccount(address)
	mockOutput.OutputAccountMock = account
	mockOutput.OutputAccountIsNew = false

	mockRuntime := &contextmock.RuntimeContextMock{}
	mockMetering := &contextmock.MeteringContextMock{}
	mockMetering.SetGasSchedule(config.MakeGasMapForTests())
	mockMetering.BlockGasLimitMock = uint64(15000)
	mockMetering.GasLeftMock = 20000

	host := &contextmock.VMHostMock{
		OutputContext:         mockOutput,
		MeteringContext:       mockMetering,
		RuntimeContext:        mockRuntime,
		ForkControllerContext: &mock.ForkControllerStub{},
	}
	bcHook := &contextmock.BlockchainHookStub{}

	storageCtx, _ := NewStorageContext(host, bcHook, reservedTestPrefix)
	storageCtx.SetAddress(address)

	key := storageCtx.GetVmProtectedPrefix("something")
	value := []byte("data")

	storageStatus, err := storageCtx.SetStorage(key, value)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.True(t, errors.Is(err, vmhost.ErrCannotWriteProtectedKey))
	require.Len(t, storageCtx.GetStorageUpdates(address), 0)

	storageCtx.disableStorageProtection()
	protectedKey := append(reservedTestPrefix[0], []byte("ABC")...)
	storageStatus, err = storageCtx.SetStorage(protectedKey, value)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.True(t, errors.Is(err, vmhost.ErrStoreReservedKey))
	require.Len(t, storageCtx.GetStorageUpdates(address), 0)

	storageCtx.disableStorageProtection()
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Nil(t, err)
	require.Equal(t, vmhost.StorageAdded, storageStatus)
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)

	storageCtx.enableStorageProtection()
	storageStatus, err = storageCtx.SetStorage(key, value)
	require.Equal(t, vmhost.StorageUnchanged, storageStatus)
	require.True(t, errors.Is(err, vmhost.ErrCannotWriteProtectedKey))
	require.Len(t, storageCtx.GetStorageUpdates(address), 1)
}

func TestStorageContext_GetStorageFromAddress(t *testing.T) {
	t.Parallel()

	errTooManyRequests := errors.New("too many requests")

	var testCases = []struct {
		name string

		internalData        []byte
		readable            bool
		exists              bool
		getStorageDataError error
		getUserAccountError error

		expectedData  []byte
		expectedError error

		forkController core.ForkController
	}{
		{
			name:                "Should return GetStorageData error on readable account",
			exists:              true,
			getStorageDataError: errTooManyRequests,
			readable:            true,
			expectedError:       errTooManyRequests,
			forkController:      &mock.ForkControllerStub{},
		},
		{
			name:           "Should return correct internal data on working an readable account",
			exists:         true,
			readable:       true,
			internalData:   []byte("internal data"),
			expectedData:   []byte("internal data"),
			forkController: &mock.ForkControllerStub{},
		},
		{
			name:          "Should return error when reading from a non readable account with AuditChanges fork",
			exists:        true,
			readable:      false,
			expectedError: vmhost.ErrInvalidCallOnReadOnlyMode,
			forkController: &mock.ForkControllerStub{
				FixAuditChangesValue: true,
			},
		},
		{
			name:           "Should not return error when reading from a non readable account without AuditChanges fork",
			exists:         true,
			readable:       false,
			expectedError:  nil,
			forkController: &mock.ForkControllerStub{},
		},
		{
			name:           "Should return error when reading from non existent account",
			exists:         false,
			expectedError:  vmhost.ErrInvalidAccount,
			forkController: &mock.ForkControllerStub{},
		},
		{
			name:                "Should return error when failed to GetUserAccount",
			getUserAccountError: vmhost.ErrBadBounds,
			expectedError:       vmhost.ErrBadBounds,
			forkController:      &mock.ForkControllerStub{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scAddress := []byte("scAccount")
			address := []byte("account")
			mockOutput := &contextmock.OutputContextMock{}
			account := mockOutput.NewVMOutputAccount(scAddress)
			mockOutput.OutputAccountMock = account
			mockOutput.OutputAccountIsNew = false

			mockRuntime := &contextmock.RuntimeContextMock{}
			mockMetering := &contextmock.MeteringContextMock{}
			mockMetering.SetGasSchedule(config.MakeGasMapForTests())
			mockMetering.BlockGasLimitMock = uint64(15000)

			host := &contextmock.VMHostMock{
				OutputContext:         mockOutput,
				MeteringContext:       mockMetering,
				RuntimeContext:        mockRuntime,
				ForkControllerContext: tc.forkController,
			}
			bcHook := &contextmock.BlockchainHookStub{
				GetUserAccountCalled: func(address []byte) (state.UserAccountHandler, error) {
					if tc.getUserAccountError != nil {
						return nil, tc.getUserAccountError
					}
					if !tc.exists { // no error, but also no account
						return nil, nil
					}
					if tc.readable {
						return &worldmock.Account{CodeMetadata: []byte{4, 0}}, nil // readable meta
					}

					return &worldmock.Account{CodeMetadata: []byte{0, 0}}, nil // not readable meta
				},
				GetStorageDataCalled: func(accountsAddress []byte, index []byte) ([]byte, uint32, error) {
					return tc.internalData, 0, tc.getStorageDataError
				},
			}

			storageCtx, _ := NewStorageContext(host, bcHook, reservedTestPrefix)
			storageCtx.SetAddress(scAddress)

			key := []byte("key")
			data, _, _, err := storageCtx.GetStorageFromAddress(address, key)
			require.Equal(t, tc.expectedData, data)
			require.Equal(t, tc.expectedError, err)
		})
	}
}

func TestStorageContext_LoadGasStoreGasPerKey(t *testing.T) {
	// FIXME: Implement this test
}

func TestStorageContext_StoreGasPerKey(t *testing.T) {
	// FIXME: Implement this test
}

func TestStorageContext_PopSetActiveStateIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}

	storageCtx, _ := NewStorageContext(host, &contextmock.BlockchainHookStub{}, reservedTestPrefix)
	storageCtx.PopSetActiveState()

	require.Equal(t, 0, len(storageCtx.stateStack))
}

func TestStorageContext_PopDiscardIfStackIsEmptyShouldNotPanic(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}

	storageCtx, _ := NewStorageContext(host, &contextmock.BlockchainHookStub{}, reservedTestPrefix)
	storageCtx.PopDiscard()

	require.Equal(t, 0, len(storageCtx.stateStack))
}
