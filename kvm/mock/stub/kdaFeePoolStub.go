package stub

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
)

type KDAFeesPoolKappStub struct {
	SetKAppControllerCalled func(controller kapp.KAppController) error
	SetAccountsCacherCalled func(cacher state.AccountsCacher) error
	ComputeCalled           func(klvAmount int64, info data.KDAFeeHandler) (int64, error)
	SwapCalled              func(sender state.UserAccountHandler, klvAmount int64, info data.KDAFeeHandler) error
	ValidateCalled          func(klvFee int64, info data.KDAFeeHandler) error
	ChangePoolOwnerCalled   func(poolID []byte, sender []byte, newOwner []byte) (transaction.Transaction_TXResultCode, error)
	GetPoolOwnerCalled      func(assetID []byte) ([]byte, error)
	UpdatePoolCalled        func(poolID []byte, assetOwner []byte, sender []byte, info *transaction.KDAPoolInfo) (transaction.Transaction_TXResultCode, error)
	DepositCalled           func(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error)
	WithdrawCalled          func(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNilCalled    func() bool
}

func (s *KDAFeesPoolKappStub) SetKAppController(controller kapp.KAppController) error {
	if s.SetKAppControllerCalled != nil {
		return s.SetKAppControllerCalled(controller)
	}
	return nil
}

func (s *KDAFeesPoolKappStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	if s.SetAccountsCacherCalled != nil {
		return s.SetAccountsCacherCalled(cacher)
	}
	return nil
}

func (s *KDAFeesPoolKappStub) Compute(klvAmount int64, info data.KDAFeeHandler) (int64, error) {
	if s.ComputeCalled != nil {
		return s.ComputeCalled(klvAmount, info)
	}
	return 0, nil
}

func (s *KDAFeesPoolKappStub) Swap(sender state.UserAccountHandler, klvAmount int64, info data.KDAFeeHandler) error {
	if s.SwapCalled != nil {
		return s.SwapCalled(sender, klvAmount, info)
	}
	return nil
}

func (s *KDAFeesPoolKappStub) Validate(klvFee int64, info data.KDAFeeHandler) error {
	if s.ValidateCalled != nil {
		return s.ValidateCalled(klvFee, info)
	}
	return nil
}

func (s *KDAFeesPoolKappStub) ChangePoolOwner(poolID []byte, sender []byte, newOwner []byte) (transaction.Transaction_TXResultCode, error) {
	if s.ChangePoolOwnerCalled != nil {
		return s.ChangePoolOwnerCalled(poolID, sender, newOwner)
	}
	return transaction.Transaction_Ok, nil
}

func (s *KDAFeesPoolKappStub) GetPoolOwner(assetID []byte) ([]byte, error) {
	if s.GetPoolOwnerCalled != nil {
		return s.GetPoolOwnerCalled(assetID)
	}
	return nil, nil
}

func (s *KDAFeesPoolKappStub) UpdatePool(poolID []byte, assetOwner []byte, sender []byte, info *transaction.KDAPoolInfo) (transaction.Transaction_TXResultCode, error) {
	if s.UpdatePoolCalled != nil {
		return s.UpdatePoolCalled(poolID, assetOwner, sender, info)
	}
	return transaction.Transaction_Ok, nil
}

func (s *KDAFeesPoolKappStub) Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error) {
	if s.DepositCalled != nil {
		return s.DepositCalled(sender, tc)
	}
	return transaction.Transaction_Ok, nil
}

func (s *KDAFeesPoolKappStub) Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error) {
	if s.WithdrawCalled != nil {
		return s.WithdrawCalled(sender, tc)
	}
	return transaction.Transaction_Ok, nil
}

func (s *KDAFeesPoolKappStub) IsInterfaceNil() bool {
	if s.IsInterfaceNilCalled != nil {
		return s.IsInterfaceNilCalled()
	}
	return s == nil
}
