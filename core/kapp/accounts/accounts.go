package accounts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.AccountsKapp = (*accountsKapp)(nil)

type accountsKapp struct {
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewAccountKApp holds the arguments needed to create a AccountKapp
type ArgsNewAccountKApp struct {
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewAccountKApp creates a validator KApp
func NewAccountKApp(
	args *ArgsNewAccountKApp,
) (*accountsKapp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}

	v := &accountsKapp{
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (a *accountsKapp) IsInterfaceNil() bool {
	return a == nil
}

func (a *accountsKapp) SetKAppController(controller kapp.KAppController) error {
	a.KAppController = controller

	return nil
}

func (a *accountsKapp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	a.accountsCacher = cacher

	return nil
}

func (a *accountsKapp) GetAccountsCacher() state.AccountsCacher {
	return a.accountsCacher
}

func (a *accountsKapp) GetExistingUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := a.accountsCacher.GetExistingUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (a *accountsKapp) LoadUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := a.accountsCacher.LoadUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (a *accountsKapp) Transfer(cType transaction.TXContract_ContractType, sender []byte, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
	acntSrc, acntDst, resultCode, err := a.validateAndLoadAccounts(sender, tc)
	if err != nil {
		return resultCode, err
	}

	assetID, internalID, kda, resultCode, err := a.loadKDA(tc.GetAssetID())
	if err != nil {
		return resultCode, err
	}

	if kda.Attributes.IsPaused {
		return transaction.Transaction_AssetPaused, process.ErrAssetIsPaused
	}

	// check for transfer roles if SC fork enabled
	if a.forkController.EnableSmartContracts() &&
		!kda.IsTransferAllowed(sender, tc.GetToAddress()) {
		return transaction.Transaction_KDATransferNotAllowed, process.ErrKDATransferNotAllowed
	}

	resultCode, err = a.processFixedRoyaltiesTransfer(cType, tc, acntSrc, acntDst, kda)
	if err != nil {
		return resultCode, err
	}

	switch kda.AssetType {
	case kapps.KDAData_NonFungible:
		return a.processNonFungibleTransfer(assetID, internalID, acntSrc, acntDst)
	case kapps.KDAData_SemiFungible:
		resultCode, err := a.processPercentageRoyaltiesTransfer(cType, tc, assetID, internalID, acntSrc, acntDst, kda)
		if err != nil {
			return resultCode, err
		}

		return a.processSemiFungibleTransfer(tc, assetID, internalID, acntSrc, acntDst, kda)
	case kapps.KDAData_Fungible:
		resultCode, err := a.processPercentageRoyaltiesTransfer(cType, tc, assetID, internalID, acntSrc, acntDst, kda)
		if err != nil {
			return resultCode, err
		}

		return a.processFungibleTransfer(tc, assetID, acntSrc, acntDst, kda)
	default:
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}
}

func (a *accountsKapp) validateAndLoadAccounts(sender []byte, tc *transaction.TransferContract) (state.UserAccountHandler, state.UserAccountHandler, transaction.Transaction_TXResultCode, error) {
	if len(tc.GetToAddress()) != a.pubkeyConv.Len() {
		return nil, nil, transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if bytes.Equal(sender, tc.GetToAddress()) {
		return nil, nil, transaction.Transaction_SameAccountError, process.ErrSameSenderAndReceiverAddress
	}

	acntSrc, err := a.LoadUserAccount(sender)
	if err != nil {
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	acntDst, err := a.LoadUserAccount(tc.GetToAddress())
	if err != nil {
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	return acntSrc, acntDst, transaction.Transaction_Ok, nil
}

func (a *accountsKapp) loadKDA(kdaID []byte) ([]byte, []byte, *kapps.KDAData, transaction.Transaction_TXResultCode, error) {
	parsedKDA := bytes.Split(kdaID, []byte(kapps.Sp))

	assetID := parsedKDA[0]
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	_, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return nil, nil, nil, transaction.Transaction_KAPPError, err
	}

	var internalID []byte
	if len(parsedKDA) > 1 {
		if !a.TokeTypeHasNonce(kda.AssetType) || len(parsedKDA) != 2 {
			return nil, nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		internalID = parsedKDA[1]
	}

	return assetID, internalID, kda, transaction.Transaction_Ok, nil
}

func (a *accountsKapp) ComputeRoyalties(kdaID []byte, amountToTransfer int64) (klvRoyalties int64, assetRoyalties int64, err error) {
	_, _, kda, _, err := a.loadKDA(kdaID)
	if err != nil {
		return 0, 0, err
	}

	return a.computeRoyalties(kda, amountToTransfer)
}

func (a *accountsKapp) computeRoyalties(kda *kapps.KDAData, amountToTransfer int64) (klvRoyalties int64, assetRoyalties int64, err error) {
	if kda.Royalties == nil {
		return 0, 0, nil
	}

	if kda.Royalties.TransferFixed > 0 {
		klvRoyalties = kda.Royalties.TransferFixed
	}

	if len(kda.Royalties.TransferPercentage) > 0 {
		assetRoyalties, err = kda.GetTransferRoyaltyByAmount(amountToTransfer, a.forkController.KdaFpr())
		if err != nil {
			return 0, 0, err
		}
	}

	return klvRoyalties, assetRoyalties, nil
}

func (a *accountsKapp) processFixedRoyaltiesTransfer(cType transaction.TXContract_ContractType, tc *transaction.TransferContract, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if cType != transaction.TXContract_TransferContractType ||
		kda.Royalties == nil ||
		kda.Royalties.TransferFixed <= 0 {
		return transaction.Transaction_Ok, nil
	}

	if a.forkController.EnableSmartContracts() && tc.KLVRoyalties != kda.Royalties.TransferFixed {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	balance := acntSrc.GetBalance(kdautils.KLVIdentifier, a.forkController.EnableSmartContracts())
	if balance < kda.Royalties.TransferFixed {
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	err := acntSrc.SubFromBalance(kda.Royalties.TransferFixed, kdautils.KLVIdentifier, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	royaltiesFixedToPay := kda.Royalties.TransferFixed
	for key, value := range kda.Royalties.SplitRoyalties {

		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		splitRoyalty, err := a.LoadUserAccount(decodedAddress)
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		splitToPay := int64(float64(kda.Royalties.TransferFixed) * float64(value.PercentTransferFixed) / float64(core.HundredPercent))
		royaltiesFixedToPay -= splitToPay

		err = splitRoyalty.AddToBalance(splitToPay, kdautils.KLVIdentifier, a.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := a.accountsCacher.UpdateUser(splitRoyalty); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			a.KAppController.GetCurrentKAppContext().ContractID(),
			acntSrc.AddressBytes(),
			splitRoyalty.AddressBytes(),
			[]byte(strconv.FormatInt(splitToPay, 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
		))

	}

	if royaltiesFixedToPay > 0 {
		royaltyOwner, err := a.LoadUserAccount(kda.Royalties.Address)
		if !a.forkController.KdaFpr() {
			royaltyOwner, err = a.GetExistingUserAccount(kda.OwnerAddress)
		}
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		err = royaltyOwner.AddToBalance(royaltiesFixedToPay, kdautils.KLVIdentifier, a.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := a.accountsCacher.UpdateUser(royaltyOwner); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			a.KAppController.GetCurrentKAppContext().ContractID(),
			acntSrc.AddressBytes(),
			royaltyOwner.AddressBytes(),
			[]byte(strconv.FormatInt(royaltiesFixedToPay, 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
		))
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processPercentageRoyaltiesTransfer(ctType transaction.TXContract_ContractType, tc *transaction.TransferContract, assetID, internalID []byte, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if ctType != transaction.TXContract_TransferContractType ||
		kda.Royalties == nil ||
		len(kda.Royalties.TransferPercentage) <= 0 {
		return transaction.Transaction_Ok, nil
	}

	value := tc.GetAmount()
	if value <= 0 {
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	royaltyAmount := int64(0)
	var err error
	if kda.Royalties != nil && len(kda.Royalties.TransferPercentage) > 0 {
		royaltyAmount, err = kda.GetTransferRoyaltyByAmount(value, a.forkController.KdaFpr())
		if err != nil {
			return transaction.Transaction_AccountError, err
		}
	}

	balance := acntSrc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value+royaltyAmount {
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	if royaltyAmount > 0 {
		if royaltyAmount != tc.GetKDARoyalties() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		royaltiesToPay := royaltyAmount
		for key, value := range kda.Royalties.SplitRoyalties {

			decodedAddress, err := hex.DecodeString(key)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitRoyalty, err := a.LoadUserAccount(decodedAddress)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			splitToPay := int64(float64(royaltyAmount) * float64(value.PercentTransferPercentage) / float64(core.HundredPercent))
			royaltiesToPay -= splitToPay

			err = splitRoyalty.AddToBalance(splitToPay, assetID, a.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := a.accountsCacher.UpdateUser(splitRoyalty); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				a.KAppController.GetCurrentKAppContext().ContractID(),
				acntSrc.AddressBytes(),
				splitRoyalty.AddressBytes(),
				[]byte(strconv.FormatInt(splitToPay, 10)),
				assetID,
				nil,
				[]byte{byte(kda.AssetType)},
			))
		}

		if royaltiesToPay > 0 {
			royaltyReceiver, err := a.LoadUserAccount(kda.Royalties.Address)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			err = acntSrc.SubFromBalance(royaltyAmount, assetID, a.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			err = royaltyReceiver.AddToBalance(royaltiesToPay, assetID, a.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			if err := a.accountsCacher.UpdateUser(royaltyReceiver); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}

			a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				a.KAppController.GetCurrentKAppContext().ContractID(),
				acntSrc.AddressBytes(),
				royaltyReceiver.AddressBytes(),
				[]byte(strconv.FormatInt(royaltiesToPay, 10)),
				assetID,
				internalID,
				[]byte{byte(kda.AssetType)},
			))
		}
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processNonFungibleTransfer(assetID, internalID []byte, acntSrc, acntDst state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	data, err := acntSrc.SubInternalKDA(assetID, internalID)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = acntDst.AddInternalKDA(assetID, internalID, data)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(acntDst); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		a.KAppController.GetCurrentKAppContext().ContractID(),
		acntSrc.AddressBytes(),
		acntDst.AddressBytes(),
		[]byte("1"),
		assetID,
		internalID,
		[]byte{byte(kapps.KDAData_NonFungible)},
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processSemiFungibleTransfer(tc *transaction.TransferContract, assetID, internalID []byte, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !a.forkController.EnableSmartContracts() {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	value := tc.GetAmount()
	if value <= 0 {
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	balance := acntSrc.GetBalanceWithNonce(assetID, internalID, a.forkController.EnableSmartContracts())
	if balance < value {
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	err := acntSrc.SubFromBalanceWithNonce(value, assetID, internalID, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	err = acntDst.AddToBalanceWithNonce(value, assetID, internalID, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		a.KAppController.GetCurrentKAppContext().ContractID(),
		acntSrc.AddressBytes(),
		acntDst.AddressBytes(),
		[]byte(strconv.FormatInt(value, 10)),
		assetID,
		internalID,
		[]byte{byte(kapps.KDAData_SemiFungible)},
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processFungibleTransfer(tc *transaction.TransferContract, assetID []byte, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	value := tc.GetAmount()
	if value <= 0 {
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	balance := acntSrc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value {
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	err := acntSrc.SubFromBalance(value, assetID, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = acntDst.AddToBalance(value, assetID, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := a.accountsCacher.UpdateUser(acntDst); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	a.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		a.KAppController.GetCurrentKAppContext().ContractID(),
		acntSrc.AddressBytes(),
		acntDst.AddressBytes(),
		[]byte(strconv.FormatInt(value, 10)),
		assetID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) ClaimBalance(
	claimType transaction.ClaimContract_EnumClaimType,
	assetID []byte,
	block *block.Block,
	acc state.UserAccountHandler,
	staking *kapps.StakingData,
	kda *kapps.KDAData,
	userKDA *kapps.UserKDA,
) (map[string]int64, error) {
	// Market claim does not update balance based on staking process, return here
	if claimType == transaction.ClaimContract_MarketClaim {
		return nil, nil
	}

	blockTime := block.GetTimestamp()
	epoch := block.GetEpoch()

	// compute gains
	gains, err := acc.Claim(claimType,
		assetID,
		epoch,
		blockTime,
		staking,
		kda,
		userKDA,
		a.forkController,
	)
	if err != nil {
		return nil, err
	}

	if claimType == transaction.ClaimContract_StakingClaim {
		if a.forkController.ClaimKFI() ||
			gains[string(assetID)] > 0 {
			userKDA.LastClaim.Timestamp = blockTime
			userKDA.LastClaim.Epoch = epoch
		}
	}

	for key, value := range gains {
		if value > 0 {
			var userKDAToUpdate *kapps.UserKDA
			if bytes.Equal([]byte(key), assetID) {
				userKDAToUpdate = userKDA
			}

			err = acc.AddToBalance(value, []byte(key), a.forkController.EnableSmartContracts(), userKDAToUpdate)
			if err != nil {
				return nil, err
			}
		}
	}

	return gains, nil
}

func (a *accountsKapp) updateFPRTotalStake(
	block *block.Block,
	assetID []byte,
	staking *kapps.StakingData,
) error {
	if !a.forkController.BigBucketsCompute() {
		return nil
	}

	if bytes.Equal(assetID, kdautils.KLVIdentifier) ||
		bytes.Equal(assetID, kdautils.KFIIdentifier) ||
		staking.InterestType != kapps.StakingData_FPRI {
		return nil
	}

	for _, fpr := range staking.FPR {
		if fpr.Epoch == block.GetEpoch()+1 {
			fpr.TotalStaked = staking.TotalStaked
			break
		}
	}

	return nil
}

func (a *accountsKapp) Freeze(sender []byte, tc *transaction.FreezeContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	assetID := tc.GetAssetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	minAmount := a.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKLVBucketAmount)
	value := tc.GetAmount()
	if (bytes.Equal(assetID, kdautils.KLVIdentifier) &&
		value < minAmount) ||
		value <= 0 {
		return transaction.Transaction_ValueInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	buckets := ownerAcc.GetBuckets(tc.GetAssetID(), a.forkController.EnableSmartContracts())
	if len(buckets) >= int(a.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MaxBucketSize)) {
		return transaction.Transaction_BucketsExceeded, common.ErrMaxBucketsExceeded
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if kda.AssetType != kapps.KDAData_Fungible {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, assetID, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	balance := ownerAcc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value {
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	err = ownerAcc.SubFromBalance(value, assetID, a.forkController.EnableSmartContracts(), userKDA)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	var bucketID = assetID
	if bytes.Equal(assetID, kdautils.KLVIdentifier) || bytes.Equal(assetID, kdautils.KFIIdentifier) {
		bucketID = kdautils.ToBucketID(a.hasher, ctx.Block().GetRandSeed(), sender, assetID, ctx.TxNonce(), ctx.ContractID(), tc.GetAmount())
	}

	err = ownerAcc.Freeze(assetID, bucketID, value, ctx.Block().GetEpoch(), ctx.Block().GetTimestamp(), staking, userKDA, a.forkController.FixStakingBuckets())
	if err != nil {
		return transaction.Transaction_FreezeError, err
	}

	err = ownerAcc.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.updateFPRTotalStake(ctx.Block(), assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	for key, value := range gains {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			claimAddress(staking.InterestType),
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), assetID, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			assetID,
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			claimType,
		))
	}

	ctx.SetReturnData([][]byte{bucketID})

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Freeze,
		ctx.ContractID(),
		bucketID,
		ownerAcc.AddressBytes(),
		assetID,
		[]byte(strconv.FormatInt(value, 10)),
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) Unfreeze(sender []byte, tc *transaction.UnfreezeContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	assetID := tc.GetAssetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if kda.AssetType != kapps.KDAData_Fungible {
		return transaction.Transaction_AssetError, common.ErrAssetTypeInvalid
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	// claim pending rewards
	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, assetID, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	delegationAddress, unfrozenAmount, err := ownerAcc.Unfreeze(assetID, tc.GetBucketID(), ctx.Block().GetEpoch(), staking, userKDA, a.forkController.FixStakingBuckets())
	if err != nil {
		return transaction.Transaction_UnfreezeError, err
	}

	if delegationAddress != nil {
		// undelegate bucket from peer
		resultCode, err := a.KAppController.GetValidatorsKApp().Undelegate(
			ctx.Block().GetEpoch(), delegationAddress, sender, &transaction.UndelegateContract{BucketID: tc.GetBucketID()})
		if err != nil {
			return resultCode, err
		}
	}

	err = ownerAcc.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.updateFPRTotalStake(ctx.Block(), assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Unfreeze,
		ctx.ContractID(),
		tc.GetBucketID(),
		[]byte(strconv.FormatInt(int64(ctx.Block().GetEpoch()+staking.GetMinEpochsToWithdraw()), 10)),
		ownerAcc.AddressBytes(),
		assetID,
		[]byte(strconv.FormatInt(unfrozenAmount, 10)),
	))

	for key, value := range gains {
		if value <= 0 {
			continue
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			claimAddress(staking.InterestType),
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), assetID, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			assetID,
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			claimType,
		))
	}

	if delegationAddress != nil {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Delegate,
			ctx.ContractID(),
			ownerAcc.AddressBytes(),
			tc.GetBucketID(),
			nil,
			[]byte(strconv.FormatInt(unfrozenAmount, 10)),
		))
	}

	//On KFI unfreeze, remove all user votes
	if bytes.Equal(assetID, kdautils.KFIIdentifier) {
		proposalKapp, _, controller, err := a.KAppController.GetProposalKApp().GetProposal(0)
		if err != nil {
			return transaction.Transaction_AccountError, err
		}

		userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_AccountError, err
		}

		// must use string to marshal proto map due UTF8 issue
		encodedAddr := hex.EncodeToString(sender)

		for _, proposal := range controller.GetActiveProposals() {
			for _, id := range proposal.ProposalIDs {
				_, proposal, _, err := a.KAppController.GetProposalKApp().GetProposal(id)
				if err != nil {
					return transaction.Transaction_AccountError, err
				}

				if v, ok := proposal.Voters[encodedAddr]; ok && v != nil {
					if v.Amount <= userKDA.FrozenBalance {
						continue
					}

					votesToRemove := unfrozenAmount
					receiptAmount := int64(0)
					if votesToRemove >= v.Amount {
						votesToRemove = v.Amount
						delete(proposal.Voters, encodedAddr)
					} else {
						v.Amount -= votesToRemove
						receiptAmount = v.Amount
					}

					proposal.Votes[int32(v.Type)] -= votesToRemove

					ctx.Receipts().Add(txProcess.NewReceipt(
						txProcess.ProposalVote,
						ctx.ContractID(),
						[]byte(strconv.FormatUint(id, 10)),           // proposal ID
						ownerAcc.AddressBytes(),                      // Voter
						[]byte(strconv.FormatInt(int64(v.Type), 10)), // Vote type
						[]byte(strconv.FormatInt(receiptAmount, 10)), // Vote weight
					))

					err = a.KAppController.GetProposalKApp().SetProposal(proposalKapp, id, proposal, controller)
					if err != nil {
						return transaction.Transaction_ParameterInvalid, err
					}
				}
			}
		}

		if err := a.accountsCacher.UpdateKapp(proposalKapp); err != nil {
			return transaction.Transaction_AccountError, err
		}
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) Delegate(sender []byte, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	if len(tc.GetToAddress()) != a.pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if len(tc.GetBucketID()) == 0 {
		return transaction.Transaction_BucketIDInvalid, process.ErrInvalidArgument
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(kdautils.KLVIdentifier)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(kdautils.KLVIdentifier)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(kdautils.KLVIdentifier, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// claim current staking...
	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, kdautils.KLVIdentifier, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	var updateValidator [][]byte
	var resultCode transaction.Transaction_TXResultCode
	resultCode, updateValidator, err = a.KAppController.GetValidatorsKApp().Delegate(sender, ctx.Block().GetTimestamp(), ctx.Block().GetEpoch(), tc)
	if err != nil {
		return resultCode, err
	}

	// set new delegation address on account
	amountDelegated, err := ownerAcc.Delegate(tc.BucketID, tc.GetToAddress(), userKDA)
	if err != nil {
		return transaction.Transaction_DeletegateError, err
	}

	err = ownerAcc.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, kdautils.KLVIdentifier, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, kdautils.KLVIdentifier, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	for key, value := range gains {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.StakingKAppAddress, // only for KLV (FPR)
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), kdautils.KLVIdentifier, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), kdautils.KLVIdentifier, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			kdautils.KLVIdentifier,
			[]byte(key),
			claimType,
		))

	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Delegate,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		tc.GetBucketID(),
		tc.GetToAddress(),
		[]byte(strconv.FormatInt(amountDelegated, 10)),
	))

	for _, validator := range updateValidator {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.UpdateValidator,
			ctx.ContractID(),
			validator,
		))
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) Undelegate(sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	if len(tc.GetBucketID()) == 0 {
		return transaction.Transaction_BucketIDInvalid, process.ErrInvalidArgument
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(kdautils.KLVIdentifier)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(kdautils.KLVIdentifier)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(kdautils.KLVIdentifier, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// claim current rewards
	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, kdautils.KLVIdentifier, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	delegationAddress, bucketValue, err := ownerAcc.Undelegate(tc.BucketID, userKDA)
	if err != nil {
		return transaction.Transaction_UndeletegateError, err
	}

	// Undelegate bucket on validator
	resultCode, err := a.KAppController.GetValidatorsKApp().Undelegate(ctx.Block().GetEpoch(), delegationAddress, sender, tc)
	if err != nil {
		return resultCode, err
	}

	err = ownerAcc.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, kdautils.KLVIdentifier, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, kdautils.KLVIdentifier, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	for key, value := range gains {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.StakingKAppAddress, // only for KLV (FPR)
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), kdautils.KLVIdentifier, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), kdautils.KLVIdentifier, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			kdautils.KLVIdentifier,
			[]byte(key),
			claimType,
		))
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Delegate,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		tc.GetBucketID(),
		nil,
		[]byte(strconv.FormatInt(bucketValue, 10)),
	))

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateValidator,
		ctx.ContractID(),
		delegationAddress,
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	assetID := tc.GetAssetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, assetID, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	amount, err := ownerAcc.Withdraw(assetID, ctx.Block().GetEpoch(), staking.GetMinEpochsToWithdraw(), userKDA)
	if err != nil {
		return transaction.Transaction_WithdrawError, err
	}

	err = ownerAcc.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	for key, value := range gains {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			claimAddress(staking.InterestType),
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), assetID, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			assetID,
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			claimType,
		))

	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Withdraw,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		assetID,
		[]byte(strconv.FormatInt(amount, 10)),
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) ClaimStaking(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	assetID := tc.GetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, assetID, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil {
		var resultCode transaction.Transaction_TXResultCode
		if err == common.ErrMaxSupplyExceeded {
			resultCode = transaction.Transaction_MaxSupplyExceeded
		} else {
			resultCode = transaction.Transaction_ClaimError
		}
		return resultCode, err
	}

	validClaims := 0
	for key, value := range gains {
		if value <= 0 {
			continue
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			claimAddress(staking.InterestType),
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			nil,
			txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), assetID, []byte(key), kda),
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			assetID,
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			claimType,
		))

		validClaims++
	}

	if validClaims <= 0 {
		return transaction.Transaction_ClaimError, state.ErrClaimNotAvailable
	}

	err = ownerAcc.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) ClaimAllowance(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	assetID := tc.GetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	if !bytes.Equal(assetID, kdautils.KLVIdentifier) {
		return transaction.Transaction_AssetIDInvalid, common.ErrAssetIDInvalid
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_AllowanceClaim, kdautils.KLVIdentifier, ctx.Block(), ownerAcc, nil, nil, userKDA)
	if err != nil {
		var resultCode transaction.Transaction_TXResultCode
		if err == common.ErrMaxSupplyExceeded {
			resultCode = transaction.Transaction_MaxSupplyExceeded
		} else {
			resultCode = transaction.Transaction_ClaimError
		}
		return resultCode, err
	}

	for key, value := range gains {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.StakingKAppAddress, // Only for KLV (FPR), mint is done during  block processing
			sender,
			[]byte(strconv.FormatInt(value, 10)),
			txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, []byte(key)),
			nil,
			[]byte{byte(kapps.KDAData_Fungible)}, //only KLV is allowed
		))

		claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_AllowanceClaim.Enum().Number()), 10))
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Claim,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(value, 10)),
			nil,
			nil,
			assetID,
			[]byte(key),
			claimType,
		))
	}

	err = ownerAcc.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) SetAccountName(sender []byte, tc *transaction.SetAccountNameContract) (transaction.Transaction_TXResultCode, error) {
	if !utf8.Valid(tc.GetName()) ||
		len(tc.GetName()) > core.MaxNameSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := a.LoadUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	ownerAcc.SetName(tc.Name)

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.SetAccountName,
		ctx.ContractID(),
		tc.Name,
		sender,
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) UpdatePermission(sender []byte, tc *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	// validate data
	if len(tc.Permissions) > core.MaxAccountPermission {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	permission := make([]*state.Permission, 0)

	hasOwner := false
	if len(tc.Permissions) > 0 {
		// check keys
		for _, p := range tc.Permissions {
			if len(p.Signers) == 0 ||
				len(p.Signers) > core.MaxPermissionSigners {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
			}

			// check type
			_, exist := state.Permission_PermissionType_name[int32(p.Type)]
			if !exist {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
			}

			signers := make([]*state.Key, 0)
			weightSum := int64(0)

			dupCheck := make(map[string]bool)
			// Verify address length
			for _, signer := range p.Signers {
				if len(signer.Address) != a.pubkeyConv.Len() {
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
				}
				// Check DUP
				if dupCheck[string(signer.Address)] {
					// same signature was added more than one time
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
				}
				dupCheck[string(signer.Address)] = true

				weightSum += signer.Weight
				signers = append(signers, &state.Key{
					Address: append([]byte{}, signer.Address...),
					Weight:  signer.Weight,
				})
			}

			// verify threshold/signers
			if p.Threshold > weightSum {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
			}

			hasOwner = hasOwner || p.Type == transaction.AccPermission_Owner

			var permissionName string
			if a.forkController.KdaFpr() {
				if !utf8.ValidString(p.PermissionName) ||
					len(p.PermissionName) > core.MaxNameSize {
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
				}

				permissionName = p.PermissionName
			}

			permission = append(permission, &state.Permission{
				ID:             int32(len(permission)),
				PermissionName: permissionName,
				Type:           state.Permission_PermissionType(p.Type),
				Threshold:      p.Threshold,
				Operations:     append([]byte{}, p.Operations...),
				Signers:        signers,
			})
		}
	}

	if !hasOwner {
		// add owner permission if none has been provided
		permission = append(permission, &state.Permission{
			ID:         int32(len(permission)),
			Type:       state.Permission_Owner,
			Threshold:  1,
			Operations: make([]byte, 0),
			Signers: []*state.Key{
				{
					Address: ownerAcc.AddressBytes(),
					Weight:  1,
				},
			},
		})
	}

	// save permission into account
	ownerAcc.SetPermissions(permission)

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateAccountPermission,
		ctx.ContractID(),
		sender,
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) TokeTypeHasNonce(tokenType kapps.KDAData_EnumAssetType) bool {
	if a.forkController.EnableSmartContracts() {
		return tokenType == kapps.KDAData_NonFungible ||
			tokenType == kapps.KDAData_SemiFungible
	}

	return tokenType == kapps.KDAData_NonFungible
}

func claimAddress(interestType kapps.StakingData_EnumInterestType) []byte {
	// case asset has an APR staking, its a mint process and should use zeroAddress
	if interestType == kapps.StakingData_APRI {
		return core.ZeroAddress
	}
	// return StakingKApp address
	return kapps.StakingKAppAddress
}
