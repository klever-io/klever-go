package stub

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

type KDAKappStub struct {
	SetKAppControllerCalled func(controller kapp.KAppController) error
	SetAccountsCacherCalled func(cacher state.AccountsCacher) error
	GetAccountsCacherCalled func() state.AccountsCacher
	GetKDACalled            func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error)
	SetKDACalled            func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error
	GetStakingCalled        func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error)
	SetStakingCalled        func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error
	MintCalled              func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error)
	BurnCalled              func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error)
	CreateCalled            func(sender []byte, tc *transaction.CreateAssetContract) (transaction.Transaction_TXResultCode, error)
	TriggerCalled           func(sender []byte, tc *transaction.AssetTriggerContract, txData [][]byte) (transaction.Transaction_TXResultCode, error)
	DepositCalled           func(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNilCalled    func() bool
}

func (stub *KDAKappStub) SetKAppController(controller kapp.KAppController) error {
	if stub.SetKAppControllerCalled != nil {
		return stub.SetKAppControllerCalled(controller)
	}

	return nil
}

func (stub *KDAKappStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	if stub.SetAccountsCacherCalled != nil {
		return stub.SetAccountsCacher(cacher)
	}

	return nil
}

func (stub *KDAKappStub) GetAccountsCacher() state.AccountsCacher {
	if stub.GetAccountsCacherCalled != nil {
		return stub.GetAccountsCacher()
	}
	return nil
}

func (stub *KDAKappStub) GetKDA(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
	if stub.GetKDACalled != nil {
		return stub.GetKDACalled(assetID)
	}
	return nil, nil, nil
}

func (stub *KDAKappStub) SetKDA(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
	if stub.SetKDACalled != nil {
		return stub.SetKDACalled(kdaKapp, assetID, kda)
	}
	return nil
}

func (stub *KDAKappStub) GetStaking(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
	if stub.GetStakingCalled != nil {
		return stub.GetStakingCalled(assetID)
	}
	return nil, nil, nil
}

func (stub *KDAKappStub) SetStaking(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
	if stub.SetStakingCalled != nil {
		return stub.SetStakingCalled(stakingKapp, assetID, staking)
	}
	return nil
}

func (stub *KDAKappStub) Mint(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	if stub.MintCalled != nil {
		return stub.MintCalled(sender, tc)
	}
	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KDAKappStub) Burn(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	if stub.BurnCalled != nil {
		return stub.BurnCalled(sender, tc)
	}
	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KDAKappStub) Create(sender []byte, tc *transaction.CreateAssetContract) (transaction.Transaction_TXResultCode, error) {
	if stub.CreateCalled != nil {
		return stub.CreateCalled(sender, tc)
	}
	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KDAKappStub) Trigger(sender []byte, tc *transaction.AssetTriggerContract, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	if stub.TriggerCalled != nil {
		return stub.TriggerCalled(sender, tc, txData)
	}
	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KDAKappStub) Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error) {
	if stub.DepositCalled != nil {
		return stub.DepositCalled(sender, tc)
	}
	return transaction.Transaction_TXResultCode(0), nil
}

func (stub *KDAKappStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}
	return stub == nil
}
