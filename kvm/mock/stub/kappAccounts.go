package stub

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
)

type KAppAccountsStub struct {
	SetKAppControllerCalled func(controller kapp.KAppController) error
	SetAccountsCacherCalled func(cacher state.AccountsCacher) error
	GetAccountsCacherCalled func() state.AccountsCacher
	TransferCalled          func(cType transaction.TXContract_ContractType, sender []byte, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error)
	FreezeCalled            func(sender []byte, tc *transaction.FreezeContract) (transaction.Transaction_TXResultCode, error)
	UnfreezeCalled          func(sender []byte, tc *transaction.UnfreezeContract) (transaction.Transaction_TXResultCode, error)
	DelegateCalled          func(sender []byte, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, error)
	UndelegateCalled        func(sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error)
	WithdrawCalled          func(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error)
	ClaimStakingCalled      func(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error)
	ClaimAllowanceCalled    func(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error)
	SetAccountNameCalled    func(sender []byte, tc *transaction.SetAccountNameContract) (transaction.Transaction_TXResultCode, error)
	UpdatePermissionCalled  func(sender []byte, tc *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNilCalled    func() bool
}

func (stub *KAppAccountsStub) SetKAppController(controller kapp.KAppController) error {
	if stub.SetKAppControllerCalled != nil {
		return stub.SetKAppControllerCalled(controller)
	}

	return nil
}

func (stub *KAppAccountsStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	if stub.SetAccountsCacherCalled != nil {
		return stub.SetAccountsCacherCalled(cacher)
	}

	return nil
}

func (stub *KAppAccountsStub) GetAccountsCacher() state.AccountsCacher {
	if stub.GetAccountsCacherCalled != nil {
		return stub.GetAccountsCacherCalled()
	}

	return nil
}

func (stub *KAppAccountsStub) Transfer(cType transaction.TXContract_ContractType, sender []byte, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
	if stub.TransferCalled != nil {
		return stub.TransferCalled(cType, sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) Freeze(sender []byte, tc *transaction.FreezeContract) (transaction.Transaction_TXResultCode, error) {
	if stub.FreezeCalled != nil {
		return stub.FreezeCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) Unfreeze(sender []byte, tc *transaction.UnfreezeContract) (transaction.Transaction_TXResultCode, error) {
	if stub.UnfreezeCalled != nil {
		return stub.UnfreezeCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) Delegate(sender []byte, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, error) {
	if stub.DelegateCalled != nil {
		return stub.DelegateCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) Undelegate(sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error) {
	if stub.UndelegateCalled != nil {
		return stub.UndelegateCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error) {
	if stub.WithdrawCalled != nil {
		return stub.WithdrawCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) ClaimStaking(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	if stub.ClaimStakingCalled != nil {
		return stub.ClaimStakingCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) ClaimAllowance(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	if stub.ClaimAllowanceCalled != nil {
		return stub.ClaimAllowanceCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) SetAccountName(sender []byte, tc *transaction.SetAccountNameContract) (transaction.Transaction_TXResultCode, error) {
	if stub.SetAccountNameCalled != nil {
		return stub.SetAccountNameCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) UpdatePermission(sender []byte, tc *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error) {
	if stub.UpdatePermissionCalled != nil {
		return stub.UpdatePermissionCalled(sender, tc)
	}

	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KAppAccountsStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}

	return stub == nil
}
