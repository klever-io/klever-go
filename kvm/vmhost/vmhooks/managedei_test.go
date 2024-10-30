package vmhooks_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/assert"
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

// Helper function to create a new mock VMHost
func newMockVMHost() *contextmock.VMHostMock {
	gasSchedule := config.MakeGasMapForTests()

	mockMetering := &contextmock.MeteringContextMock{
		GasLeftMock: 1000,
	}
	mockMetering.SetGasSchedule(gasSchedule)
	mockRuntime := &contextmock.RuntimeContextMock{}
	mockRuntime.InitState()
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
	}

	blockchain := &hostmock.BlockchainContextStub{}

	return &contextmock.VMHostMock{
		MeteringContext:     mockMetering,
		RuntimeContext:      mockRuntime,
		ManagedTypesContext: mType,
		BlockchainContext:   blockchain,
	}
}
