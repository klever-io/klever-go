package kda

import (
	"bytes"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
)

func ValidateRoyalties(tc *transaction.AssetTriggerContract, checkMarketPercentage bool) error {
	return validateRoyalties(tc, checkMarketPercentage)
}

func ProcessRoyaltiesTransferPercentage(tp []*transaction.RoyaltyInfo) ([]*kapps.RoyaltyData, error) {
	return processRoyaltiesTransferPercentage(tp)
}

func NewKDAKappForTests() *kdaKapp {
	mockMarshalizer := &mock.MarshalizerMock{}

	kappController := &stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: nil,
				ContractID:     0,
				ContractType:   -1,
				Block:          nil,
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				MintCalled: func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
				},
				BurnCalled: func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
				},
			}
		},
		GetKDAFeesPoolKAppCalled: func() kapp.KDAFeesPoolKapp {
			return &stub.KDAFeesPoolKappStub{
				UpdatePoolCalled: func(poolID []byte, assetOwner []byte, sender []byte, info *transaction.KDAPoolInfo) (transaction.Transaction_TXResultCode, error) {
					if bytes.Equal(sender, assetOwner) {
						return transaction.Transaction_Ok, nil
					}
					return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
				},
			}
		},
	}

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)

	return &kdaKapp{
		accountsCacher: &mock.AccountsCacherStub{},
		marshalizer:    mockMarshalizer,
		KAppController: kappController,
		forkController: forkController,
		pubkeyConv:     mock.NewPubkeyConverterMock(32),
	}
}

func (k *kdaKapp) ProcessTriggerType(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
	txData [][]byte,
) (transaction.Transaction_TXResultCode, error) {
	return k.processTriggerType(sender, tc, kdaKApp, assetID, asset, txData)
}

func (k *kdaKapp) HandleRemoveRole(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleRemoveRole(sender, tc, kdaKApp, assetID, asset)
}

func (k *kdaKapp) HandleAddRole(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleAddRole(sender, tc, kdaKApp, assetID, asset)
}
