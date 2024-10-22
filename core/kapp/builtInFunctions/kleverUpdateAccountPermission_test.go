package builtInFunctions

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAcntCacher() *mock.AccountsCacherStub {
	return &mock.AccountsCacherStub{}
}

func newKAppControllerMock() *stub.KAppControllerStub {
	return &stub.KAppControllerStub{}
}

func TestNewKleverUpdateAccountPermissionFunc(t *testing.T) {
	acntCacher := newAcntCacher()
	forkController := mock.NewForkControllerStub()
	kappController := newKAppControllerMock()
	t.Run("NilMarshalizer", func(t *testing.T) {
		kuap, err := NewKleverUpdateAccountPermissionFunc(0, nil, acntCacher, forkController, kappController)
		assert.Nil(t, kuap)
		assert.Equal(t, ErrNilMarshalizer, err)
	})

	t.Run("NilForkController", func(t *testing.T) {
		kuap, err := NewKleverUpdateAccountPermissionFunc(0, &mock.MarshalizerMock{}, acntCacher, nil, kappController)
		assert.Nil(t, kuap)
		assert.Equal(t, ErrNilEnableEpochsHandler, err)
	})

	t.Run("ValidInput", func(t *testing.T) {
		kuap, err := NewKleverUpdateAccountPermissionFunc(0, &mock.MarshalizerMock{}, acntCacher, forkController, kappController)
		assert.NotNil(t, kuap)
		assert.Nil(t, err)
	})
}

func createMock() *kleverUpdateAccountPermission {
	acntCacher := newAcntCacher()
	forkController := mock.NewForkControllerStub()
	kappController := newKAppControllerMock()

	kuap, _ := NewKleverUpdateAccountPermissionFunc(100, &mock.MarshalizerMock{}, acntCacher, forkController, kappController)
	return kuap

}

func TestSetNewGasConfig(t *testing.T) {
	kuap := createMock()

	t.Run("NilGasCost", func(t *testing.T) {
		initialGasCost := kuap.funcGasCost
		kuap.SetNewGasConfig(nil)
		assert.Equal(t, initialGasCost, kuap.funcGasCost)
	})

	t.Run("ValidGasCost", func(t *testing.T) {
		newGasCost := &vmcommon.GasCost{
			BuiltInCost: vmcommon.BuiltInCost{
				UpdateAccountPermission: 200,
			},
		}
		kuap.SetNewGasConfig(newGasCost)
		assert.Equal(t, uint64(200), kuap.funcGasCost)
	})
}

func makeAddress(addr string) []byte {
	data := make([]byte, 32)
	copy(data, addr)
	return data
}

func TestProcessBuiltinFunction(t *testing.T) {
	defaultAddress := makeAddress("address")
	recipientAddress := makeAddress("recipient")

	testPermission, err := EncodeAccountPermissionData([]*transaction.AccPermission{
		{
			Signers:    []*transaction.AccKey{{Address: recipientAddress, Weight: 1}},
			Threshold:  1,
			Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
		},
	})
	require.NoError(t, err)

	t.Run("NilVMInput", func(t *testing.T) {
		kuap := createMock()

		var vmInput *vmcommon.ContractCallInput = nil
		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.Equal(t, ErrNilVmInput, err)
	})

	t.Run("InvalidArguments", func(t *testing.T) {
		kuap := createMock()

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress},
			},
		}
		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.Equal(t, ErrInvalidArguments, err)
	})

	t.Run("LoadUserError", func(t *testing.T) {
		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return nil, common.ErrWrongTypeAssertion
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, testPermission},
			},
		}
		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("InvalidPermission", func(t *testing.T) {
		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return state.NewUserAccount(defaultAddress)
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, []byte("permissions")},
			},
			RecipientAddr: recipientAddress,
		}

		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.Equal(t, ErrInvalidArguments, err)
	})

	t.Run("NoPermissionToUpdate", func(t *testing.T) {
		acnt, _ := state.NewUserAccount(defaultAddress)
		acnt.Permissions = []*state.Permission{
			{
				Signers:    []*state.Key{{Address: defaultAddress, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}

		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return acnt, nil
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, testPermission}, // Valid encoded empty permission
			},
			RecipientAddr: recipientAddress,
		}

		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.EqualError(t, err, "invalid permission operation")
	})

	t.Run("UpdatePermissionError", func(t *testing.T) {
		acnt, _ := state.NewUserAccount(defaultAddress)
		acnt.Permissions = []*state.Permission{
			{
				Signers:    []*state.Key{{Address: recipientAddress, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}

		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return acnt, nil
		}
		kuap.kappController.(*stub.KAppControllerStub).GetAccountsKAppCalled = func() kapp.AccountsKapp {
			return &stub.KAppAccountsStub{
				UpdatePermissionCalled: func(address []byte, contract *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_AccountError, errors.New("update error")
				},
			}
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, testPermission}, // Valid encoded empty permission
			},
			RecipientAddr: recipientAddress,
		}

		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.EqualError(t, err, "update error")
	})

	t.Run("UpdatePermissionInvalidResultCode", func(t *testing.T) {
		acnt, _ := state.NewUserAccount(defaultAddress)
		acnt.Permissions = []*state.Permission{
			{
				Signers:    []*state.Key{{Address: recipientAddress, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}

		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return acnt, nil
		}
		kuap.kappController.(*stub.KAppControllerStub).GetAccountsKAppCalled = func() kapp.AccountsKapp {
			return &stub.KAppAccountsStub{
				UpdatePermissionCalled: func(address []byte, contract *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_AccountError, nil
				},
			}
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, testPermission}, // Valid encoded empty permission
			},
			RecipientAddr: recipientAddress,
		}

		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.Nil(t, vmOutput)
		assert.EqualError(t, err, "UpdatePermission error: AccountError")
	})

	t.Run("SuccessfulUpdate", func(t *testing.T) {
		acnt, _ := state.NewUserAccount(defaultAddress)
		acnt.Permissions = []*state.Permission{
			{
				Signers:    []*state.Key{{Address: recipientAddress, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}

		kuap := createMock()
		kuap.accountsCacher.(*mock.AccountsCacherStub).LoadUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return acnt, nil
		}
		kuap.kappController.(*stub.KAppControllerStub).GetAccountsKAppCalled = func() kapp.AccountsKapp {
			return &stub.KAppAccountsStub{}
		}

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments:   [][]byte{defaultAddress, testPermission}, // Valid encoded empty permission
				GasProvided: 200,
			},
			RecipientAddr: recipientAddress,
		}

		vmOutput, err := kuap.ProcessBuiltinFunction(vmInput)
		assert.NotNil(t, vmOutput)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0x64), vmOutput.GasRemaining)
		assert.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
	})
}

func TestContractHasValidPermission(t *testing.T) {
	acntCacher := newAcntCacher()
	forkController := mock.NewForkControllerStub()
	kappController := newKAppControllerMock()

	kuap, _ := NewKleverUpdateAccountPermissionFunc(100, &mock.MarshalizerMock{}, acntCacher, forkController, kappController)

	t.Run("NoValidPermission", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("other"), Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.False(t, kuap.contractHasValidPermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionButInsufficientWeight", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  2,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.False(t, kuap.contractHasValidPermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionButWrongOperation", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  1,
				Type:       state.Permission_User,
				Operations: []byte{0},
			},
		}
		assert.False(t, kuap.contractHasValidPermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionOwner", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:   []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold: 1,
			},
		}
		assert.True(t, kuap.contractHasValidPermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionUser", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  1,
				Type:       state.Permission_User,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.True(t, kuap.contractHasValidPermission(permissions, []byte("recipient")))
	})
}

func TestGetUpdateAccountPermissionContract(t *testing.T) {
	defaultAddress := makeAddress("address")

	kuap := createMock()

	t.Run("InsufficientArguments", func(t *testing.T) {
		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{},
			},
		}
		contract, err := kuap.getUpdateAccountPermissionContract(vmInput)
		assert.Nil(t, contract)
		assert.Equal(t, ErrInvalidArguments, err)
	})

	t.Run("EmptyPermissionArguments", func(t *testing.T) {
		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress},
			},
		}
		address := vmInput.NextArg()
		assert.Equal(t, defaultAddress, address.Bytes())
		contract, err := kuap.getUpdateAccountPermissionContract(vmInput)
		assert.Nil(t, contract)
		assert.Equal(t, ErrInvalidArguments, err)
	})

	t.Run("InvalidPermissionsEncoding", func(t *testing.T) {
		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, []byte("invalid")},
			},
		}
		address := vmInput.NextArg()
		assert.Equal(t, defaultAddress, address.Bytes())
		contract, err := kuap.getUpdateAccountPermissionContract(vmInput)
		assert.Nil(t, contract)
		assert.Error(t, err)
	})

	t.Run("ValidInput", func(t *testing.T) {
		testPermission := []*transaction.AccPermission{
			{
				Signers:    []*transaction.AccKey{{Address: defaultAddress, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		validPermissions, err := EncodeAccountPermissionData(testPermission)
		require.NoError(t, err)

		vmInput := &vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				Arguments: [][]byte{defaultAddress, validPermissions},
			},
		}
		address := vmInput.NextArg()
		assert.Equal(t, defaultAddress, address.Bytes())
		contract, err := kuap.getUpdateAccountPermissionContract(vmInput)
		assert.NotNil(t, contract)
		assert.Nil(t, err)
		assert.Equal(t, testPermission, contract.Permissions)
	})
}

func TestIsInterfaceNil(t *testing.T) {
	t.Run("NilInterface", func(t *testing.T) {
		var kuap *kleverUpdateAccountPermission
		assert.True(t, kuap.IsInterfaceNil())
	})

	t.Run("NonNilInterface", func(t *testing.T) {
		acntCacher := newAcntCacher()
		forkController := mock.NewForkControllerStub()
		kappController := newKAppControllerMock()
		kuap, _ := NewKleverUpdateAccountPermissionFunc(100, &mock.MarshalizerMock{}, acntCacher, forkController, kappController)
		assert.False(t, kuap.IsInterfaceNil())
	})
}
