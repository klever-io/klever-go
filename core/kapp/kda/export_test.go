package kda

import (
	"bytes"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/block"
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

type KDAKappForTests struct {
	*kdaKapp
}

func NewKDAKappForTests() *KDAKappForTests {
	mockMarshalizer := &mock.MarshalizerMock{}

	kappController := &stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: nil,
				ContractID:     0,
				ContractType:   -1,
				Block:          &block.Block{Header: &block.BlockHeader{}},
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
		GetSystemAccountsCalled: func() kapp.SystemAccountKapp {
			return &mock.SystemAccountKappStub{
				SFTSetMetadataCalled: func(asset, nonce []byte, args [][]byte) error {
					return nil
				},
			}
		},
	}

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)

	return &KDAKappForTests{
		&kdaKapp{
			accountsCacher: &mock.AccountsCacherStub{},
			marshalizer:    mockMarshalizer,
			KAppController: kappController,
			forkController: forkController,
			pubkeyConv:     mock.NewPubkeyConverterMock(32),
		},
	}
}

func (k *KDAKappForTests) GetAccountsCacher() *mock.AccountsCacherStub {
	return k.accountsCacher.(*mock.AccountsCacherStub)
}

func (k *KDAKappForTests) ProcessTriggerType(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
	txData [][]byte,
) (transaction.Transaction_TXResultCode, error) {
	return k.processTriggerType(sender, tc, kdaKApp, assetID, asset, txData)
}

func (k *KDAKappForTests) HandleRemoveRole(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleRemoveRole(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) HandleAddRole(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleAddRole(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) HandlePauseOrResume(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handlePauseOrResume(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) UpdateMetadata(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
	txdata [][]byte,
) (transaction.Transaction_TXResultCode, error) {
	return k.updateMetadata(sender, tc, kdaKApp, assetID, asset, txdata)
}

func (k *KDAKappForTests) HandleStopNFTMint(
	sender []byte,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleStopNFTMint(sender, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) HandleUpdateLogo(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleUpdateLogo(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) HandleStopRoyaltiesChange(
	sender []byte,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleStopRoyaltiesChange(sender, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) UpdateRoyaltiesReceiver(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.updateRoyaltiesReceiver(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) UpdateStaking(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.updateStaking(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) UpdateRoyalties(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.updateRoyalties(sender, tc, kdaKApp, assetID, asset)
}

func (k *KDAKappForTests) HandleUpdateURIs(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	return k.handleUpdateURIs(sender, tc, kdaKApp, assetID, asset)
}
