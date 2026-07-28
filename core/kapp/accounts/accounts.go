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
	"github.com/klever-io/klever-go/tools"
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

// isUninitializedContractAddress checks if an address is a smart contract address
// that has not been initialized (no code or codeMeta present)
func (a *accountsKapp) isUninitializedContractAddress(address []byte) bool {
	if !core.IsSmartContractAddress(address) {
		return false
	}

	// Try to get the existing account - if it doesn't exist, it's uninitialized
	userAcc, err := a.accountsCacher.GetExistingUser(address)
	if err != nil {
		// Account doesn't exist, so it's uninitialized
		return true
	}

	// we need to check code, hash and metadata because deleted contracts
	// still keep some properties, and deleted contracts are considered initialized
	codeHash := userAcc.GetCodeHash()
	codeMeta := userAcc.GetCodeMetadata()
	return len(codeHash) == 0 && len(codeMeta) == 0
}

// checkReadOnly fails closed when a state-mutating accounts operation is attempted
// from a read-only execution context (e.g. a VM query). Defense in depth behind the
// guard in BlockChainHookImpl.ProcessBuiltInFunction: it also covers accounts calls
// that do not come through the built-in dispatch. A nil controller is refused too,
// since an unknown execution context must not be assumed writable.
func (a *accountsKapp) checkReadOnly() error {
	if check.IfNil(a.KAppController) || a.KAppController.IsReadOnly() {
		return process.ErrReadOnlyKAppMutation
	}

	return nil
}

func (a *accountsKapp) Transfer(cType transaction.TXContract_ContractType, sender []byte, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	acntSrc, acntDst, resultCode, err := a.validateAndLoadAccounts(sender, tc)
	if err != nil {
		return resultCode, err
	}

	assetID, internalID, kda, resultCode, err := a.loadKDA(tc.GetAssetID())
	if err != nil {
		return resultCode, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	if kda.Attributes.IsPaused {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetPaused, process.ErrAssetIsPaused.Error())
		return transaction.Transaction_AssetPaused, process.ErrAssetIsPaused
	}

	if a.forkController.EnableSmartContracts() {
		// check for transfer roles if SC fork enabled
		if !kda.IsTransferAllowed(sender, tc.GetToAddress()) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldTransferNotAllowed, process.ErrKDATransferNotAllowed.Error())
			return transaction.Transaction_KDATransferNotAllowed, process.ErrKDATransferNotAllowed
		}
		// block transfer to uninitialized contract addresses
		// but only block on transfer actions, smart contract actions (such deploy or call) are allowed
		if cType != transaction.TXContract_SmartContractType &&
			a.isUninitializedContractAddress(tc.GetToAddress()) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldUninitializedContract, process.ErrContractAccountNotAllowed.Error())
			return transaction.Transaction_AccountError, process.ErrContractAccountNotAllowed
		}
	}

	resultCode, err = a.processFixedRoyaltiesTransfer(tc, acntSrc, acntDst, kda)
	if err != nil {
		return resultCode, err
	}

	switch kda.AssetType {
	case kapps.KDAData_NonFungible:
		if a.forkController.FixAuditChangesV3() && tc.Amount != 1 {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
			return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
		}
		return a.processNonFungibleTransfer(assetID, internalID, acntSrc, acntDst)
	case kapps.KDAData_SemiFungible:
		resultCode, err := a.processPercentageRoyaltiesTransfer(tc, assetID, internalID, acntSrc, acntDst, kda)
		if err != nil {
			return resultCode, err
		}

		return a.processSemiFungibleTransfer(tc, assetID, internalID, acntSrc, acntDst, kda)
	case kapps.KDAData_Fungible:
		resultCode, err := a.processPercentageRoyaltiesTransfer(tc, assetID, internalID, acntSrc, acntDst, kda)
		if err != nil {
			return resultCode, err
		}

		return a.processFungibleTransfer(tc, assetID, acntSrc, acntDst, kda)
	default:
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}
}

func (a *accountsKapp) validateAndLoadAccounts(sender []byte, tc *transaction.TransferContract) (state.UserAccountHandler, state.UserAccountHandler, transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	if len(tc.GetToAddress()) != a.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, process.ErrInvalidRcvAddr.Error())
		return nil, nil, transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if bytes.Equal(sender, tc.GetToAddress()) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldSameAddress, process.ErrSameSenderAndReceiverAddress.Error())
		return nil, nil, transaction.Transaction_SameAccountError, process.ErrSameSenderAndReceiverAddress
	}

	acntSrc, err := a.LoadUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	acntDst, err := a.LoadUserAccount(tc.GetToAddress())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadReceiverAccount, err.Error())
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	return acntSrc, acntDst, transaction.Transaction_Ok, nil
}

func (a *accountsKapp) loadKDA(kdaID []byte) ([]byte, []byte, *kapps.KDAData, transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()
	parsedKDA := bytes.Split(kdaID, []byte(kapps.Sp))

	assetID := parsedKDA[0]
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	_, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return nil, nil, nil, transaction.Transaction_KAPPError, err
	}

	var internalID []byte
	if len(parsedKDA) > 1 {
		if !a.TokenTypeHasNonce(kda.AssetType) || len(parsedKDA) != 2 {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrInvalidValue.Error())
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
		assetRoyalties, err = kda.GetTransferRoyaltyByAmount(amountToTransfer, a.forkController.KdaFpr(), a.forkController.EnableSmartContracts())
		if err != nil {
			return 0, 0, err
		}
	}

	return klvRoyalties, assetRoyalties, nil
}

func (a *accountsKapp) computeSplitRoyalties(address string, assetID []byte, assetType kapps.KDAData_EnumAssetType, acntSrc state.UserAccountHandler, value int64, percentage int64, royaltiesToPay *int64) (transaction.Transaction_TXResultCode, error) {
	decodedAddress, err := hex.DecodeString(address)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	splitRoyalty, err := a.LoadUserAccount(decodedAddress)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	splitToPay, err := tools.ComputePercentageI64(value, percentage, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}
	if a.forkController.FixMarketBuyOverflow() && splitToPay > *royaltiesToPay {
		splitCtx := a.KAppController.GetCurrentKAppContext()
		splitCtx.Receipts().AddError(splitCtx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}
	*royaltiesToPay -= splitToPay

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
		[]byte{byte(assetType)},
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processFixedRoyaltiesTransfer(tc *transaction.TransferContract, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if core.IsSmartContractAddress(acntSrc.AddressBytes()) ||
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
		status, err := a.computeSplitRoyalties(key, kdautils.KLVIdentifier, kapps.KDAData_Fungible, acntSrc, kda.Royalties.TransferFixed, int64(value.PercentTransferFixed), &royaltiesFixedToPay)
		if err != nil {
			return status, err
		}
	}

	if royaltiesFixedToPay <= 0 {
		return transaction.Transaction_Ok, nil
	}

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

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) validatePercentageRoyaltiesTransfer(tc *transaction.TransferContract, kda *kapps.KDAData, acntSrc state.UserAccountHandler, assetID []byte) (int64, transaction.Transaction_TXResultCode, error) {
	if core.IsSmartContractAddress(acntSrc.AddressBytes()) ||
		kda.Royalties == nil ||
		len(kda.Royalties.TransferPercentage) <= 0 {
		return 0, transaction.Transaction_Ok, nil
	}

	value := tc.GetAmount()
	if value <= 0 {
		return 0, transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	royaltyAmount := int64(0)
	var err error

	if kda.Royalties != nil && len(kda.Royalties.TransferPercentage) > 0 {
		royaltyAmount, err = kda.GetTransferRoyaltyByAmount(value, a.forkController.KdaFpr(), a.forkController.EnableSmartContracts())
		if err != nil {
			return 0, transaction.Transaction_AccountError, err
		}
	}

	balance := acntSrc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value+royaltyAmount {
		return 0, transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	return royaltyAmount, transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processPercentageRoyaltiesTransfer(tc *transaction.TransferContract, assetID, internalID []byte, acntSrc, acntDst state.UserAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	royaltyAmount, status, err := a.validatePercentageRoyaltiesTransfer(tc, kda, acntSrc, assetID)
	if err != nil {
		return status, err
	}

	if royaltyAmount <= 0 {
		return transaction.Transaction_Ok, nil
	}

	if royaltyAmount != tc.GetKDARoyalties() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	royaltiesToPay := royaltyAmount

	if a.forkController.FixMarketBuyOverflow() {
		err = acntSrc.SubFromBalance(royaltyAmount, assetID, a.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}
	}

	for key, value := range kda.Royalties.SplitRoyalties {
		status, err := a.computeSplitRoyalties(key, assetID, kda.AssetType, acntSrc, royaltyAmount, int64(value.PercentTransferPercentage), &royaltiesToPay)
		if err != nil {
			return status, err
		}
	}

	if royaltiesToPay <= 0 {
		return transaction.Transaction_Ok, nil
	}

	royaltyReceiver, err := a.LoadUserAccount(kda.Royalties.Address)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	if !a.forkController.FixMarketBuyOverflow() {
		err = acntSrc.SubFromBalance(royaltyAmount, assetID, a.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}
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
	ctx := a.KAppController.GetCurrentKAppContext()

	if !a.forkController.EnableSmartContracts() {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	value := tc.GetAmount()
	if value <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	balance := acntSrc.GetBalanceWithNonce(assetID, internalID, a.forkController.EnableSmartContracts())
	if balance < value {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, process.ErrInsufficientFunds.Error())
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
	ctx := a.KAppController.GetCurrentKAppContext()

	value := tc.GetAmount()
	if value <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	balance := acntSrc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, process.ErrInsufficientFunds.Error())
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
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ValueInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	buckets := ownerAcc.GetBuckets(tc.GetAssetID(), a.forkController.EnableSmartContracts())
	if len(buckets) >= int(a.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MaxBucketSize)) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMaxBucketsExceeded, common.ErrMaxBucketsExceeded.Error())
		return transaction.Transaction_BucketsExceeded, common.ErrMaxBucketsExceeded
	}

	kdaKapp, kda, err := a.KAppController.GetKDAKApp().GetKDA(assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if kda.AssetType != kapps.KDAData_Fungible {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldStakingError, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_AssetError, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_StakingClaim, assetID, ctx.Block(), ownerAcc, staking, kda, userKDA)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldClaimError, err.Error())
		return transaction.Transaction_ClaimError, err
	}

	balance := ownerAcc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if balance < value {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, process.ErrInsufficientFunds.Error())
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	err = ownerAcc.SubFromBalance(value, assetID, a.forkController.EnableSmartContracts(), userKDA)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBalanceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	var bucketID = assetID
	if bytes.Equal(assetID, kdautils.KLVIdentifier) || bytes.Equal(assetID, kdautils.KFIIdentifier) {
		bucketID = kdautils.ToBucketID(a.hasher, ctx.Block().GetRandSeed(), sender, assetID, ctx.TxNonce(), ctx.ContractID(), tc.GetAmount())
	}

	err = ownerAcc.Freeze(assetID, bucketID, value, ctx.Block().GetEpoch(), ctx.Block().GetTimestamp(), staking, userKDA, a.forkController.FixStakingBuckets())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldFreezeError, err.Error())
		return transaction.Transaction_FreezeError, err
	}

	err = ownerAcc.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldKAppError, err.Error())
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldSaveAccountError, err.Error())
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.updateFPRTotalStake(ctx.Block(), assetID, staking)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldStakingError, err.Error())
		return transaction.Transaction_SetStakingErr, err
	}

	err = a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldStakingError, err.Error())
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldSaveAccountError, err.Error())
		return transaction.Transaction_SaveAccountError, err
	}

	err = a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldKAppError, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldSaveAccountError, err.Error())
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

func (a *accountsKapp) Unfreeze(
	sender []byte,
	tc *transaction.UnfreezeContract,
) (transaction.Transaction_TXResultCode, error) {
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	// Retrieve owner account
	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetError, common.ErrAssetTypeInvalid
	}

	stakingKapp, staking, err := a.KAppController.GetKDAKApp().GetStaking(assetID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	// Retrieve user KDA
	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	// Claim any pending rewards
	gains, err := a.ClaimBalance(
		transaction.ClaimContract_StakingClaim,
		assetID,
		ctx.Block(),
		ownerAcc,
		staking,
		kda,
		userKDA,
	)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return transaction.Transaction_ClaimError, err
	}

	// Perform the unfreeze operation
	delegationAddress, unfrozenAmount, txResCode, err := a.processUnfreezeLogic(
		ownerAcc,
		assetID,
		tc,
		staking,
		userKDA,
	)
	if err != nil {
		return txResCode, err
	}

	// Update KDA and Staking info
	if txResCode, err := a.updateAccountState(ownerAcc, assetID, userKDA, stakingKapp, kdaKapp, kda, staking); err != nil {
		return txResCode, err
	}

	// Receipts for unfreezing
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Unfreeze,
		ctx.ContractID(),
		tc.GetBucketID(),
		[]byte(
			strconv.FormatInt(int64(ctx.Block().GetEpoch()+staking.GetMinEpochsToWithdraw()), 10),
		),
		ownerAcc.AddressBytes(),
		assetID,
		[]byte(strconv.FormatInt(unfrozenAmount, 10)),
	))

	// Receipts for gains
	for asset, gain := range gains {
		if gain <= 0 {
			continue
		}

		a.addClaimReceipts(
			ctx,
			ownerAcc.AddressBytes(),
			assetID,
			[]byte(asset),
			gain,
			staking.InterestType,
			kda,
		)
	}

	// Receipts for delegation if applicable
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

	if txResCode, err := a.handleProposalVotesOnUnfreeze(ownerAcc, assetID, sender, unfrozenAmount); err != nil {
		return txResCode, err
	}

	return transaction.Transaction_Ok, nil
}

// Handles the unfreeze logic including delegation
func (a *accountsKapp) processUnfreezeLogic(
	ownerAcc state.UserAccountHandler,
	assetID []byte,
	tc *transaction.UnfreezeContract,
	staking *kapps.StakingData,
	userKDA *kapps.UserKDA,
) ([]byte, int64, transaction.Transaction_TXResultCode, error) {
	epoch := a.KAppController.GetCurrentKAppContext().Block().GetEpoch()
	delegationAddress, unfrozenAmount, err := ownerAcc.Unfreeze(
		assetID,
		tc.GetBucketID(),
		epoch,
		staking,
		userKDA,
		a.forkController.FixStakingBuckets(),
	)
	if err != nil {
		return nil, 0, transaction.Transaction_UnfreezeError, err
	}

	if delegationAddress != nil {
		resultCode, err := a.KAppController.GetValidatorsKApp().Undelegate(
			epoch, delegationAddress, ownerAcc.AddressBytes(), &transaction.UndelegateContract{BucketID: tc.GetBucketID()})
		if err != nil {
			return nil, 0, resultCode, err
		}
		if a.forkController.EnableSmartContracts() {
			_, _, err = ownerAcc.Undelegate(tc.GetBucketID(), userKDA)
			if err != nil {
				return nil, 0, transaction.Transaction_UndelegateError, err
			}
		}
	}

	return delegationAddress, unfrozenAmount, transaction.Transaction_Ok, nil
}

func (a *accountsKapp) addClaimReceipts(
	ctx kapp.KappContext,
	ownerAddrBytes, assetID, assetBytes []byte,
	gain int64,
	interestType kapps.StakingData_EnumInterestType,
	kda *kapps.KDAData,
) {
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		claimAddress(interestType),
		ownerAddrBytes,
		[]byte(strconv.FormatInt(gain, 10)),
		txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, assetBytes),
		nil,
		txProcess.AssetTypeReceipt(a.forkController.ClaimKFI(), assetID, assetBytes, kda),
	))

	claimType := []byte(
		strconv.FormatInt(int64(transaction.ClaimContract_StakingClaim.Enum().Number()), 10),
	)
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Claim,
		ctx.ContractID(),
		[]byte(strconv.FormatInt(gain, 10)),
		nil,
		nil,
		assetID,
		txProcess.AssetGainReceipt(a.forkController.ClaimKFI(), assetID, assetBytes),
		claimType,
	))
}

// Updates account state with the new KDA and Staking data
func (a *accountsKapp) updateAccountState(
	ownerAcc state.UserAccountHandler,
	assetID []byte,
	userKDA *kapps.UserKDA,
	stakingKapp, kdaKapp state.KAppAccountHandler,
	kda *kapps.KDAData,
	staking *kapps.StakingData,
) (transaction.Transaction_TXResultCode, error) {
	if err := ownerAcc.SetUserKDA(assetID, nil, userKDA); err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := a.updateFPRTotalStake(a.KAppController.GetCurrentKAppContext().Block(), assetID, staking); err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.KAppController.GetKDAKApp().SetStaking(stakingKapp, assetID, staking); err != nil {
		return transaction.Transaction_SetStakingErr, err
	}

	if err := a.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := a.KAppController.GetKDAKApp().SetKDA(kdaKapp, assetID, kda); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := a.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) handleProposalVotesOnUnfreeze(
	ownerAcc state.UserAccountHandler,
	assetID, sender []byte,
	unfrozenAmount int64,
) (transaction.Transaction_TXResultCode, error) {
	if !bytes.Equal(assetID, kdautils.KFIIdentifier) {
		return transaction.Transaction_Ok, nil
	}

	proposalKapp, _, controller, err := a.KAppController.GetProposalKApp().GetProposal(0)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	encodedAddr := hex.EncodeToString(sender)

	for _, proposal := range controller.GetActiveProposals() {
		if err := a.processActiveProposal(proposal, proposalKapp, controller, encodedAddr, unfrozenAmount, ownerAcc, userKDA); err != nil {
			return transaction.Transaction_AccountError, err
		}
	}

	if err := a.accountsCacher.UpdateKapp(proposalKapp); err != nil {
		return transaction.Transaction_AccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) processActiveProposal(
	proposal *kapps.ActiveProposals,
	proposalKapp state.KAppAccountHandler,
	controller *kapps.ProposalController,
	encodedAddr string,
	unfrozenAmount int64,
	ownerAcc state.UserAccountHandler,
	userKDA *kapps.UserKDA,
) error {
	for _, id := range proposal.ProposalIDs {
		_, proposal, _, err := a.KAppController.GetProposalKApp().GetProposal(id)
		if err != nil {
			return err
		}

		voter, exists := proposal.Voters[encodedAddr]
		if !exists || voter == nil {
			continue
		}

		if voter.Amount <= userKDA.FrozenBalance {
			continue
		}

		a.updateVoterAndProposal(voter, proposal, encodedAddr, unfrozenAmount, ownerAcc, id)

		if err := a.updateProposalTotalStaked(proposal); err != nil {
			return err
		}

		if err := a.KAppController.GetProposalKApp().SetProposal(proposalKapp, id, proposal, controller); err != nil {
			return err
		}

	}

	return nil
}

func (a *accountsKapp) updateVoterAndProposal(
	voter *kapps.ProposalData_VoteDetail,
	proposal *kapps.ProposalData,
	encodedAddr string,
	unfrozenAmount int64,
	ownerAcc state.UserAccountHandler,
	proposalID uint64,
) {
	votesToRemove := unfrozenAmount
	receiptAmount := int64(0)

	if votesToRemove >= voter.Amount {
		votesToRemove = voter.Amount
		delete(proposal.Voters, encodedAddr)
	} else {
		voter.Amount -= votesToRemove
		receiptAmount = voter.Amount
	}

	proposal.Votes[int32(voter.Type)] -= votesToRemove

	receipt := txProcess.NewReceipt(
		txProcess.ProposalVote,
		a.KAppController.GetCurrentKAppContext().ContractID(),
		[]byte(strconv.FormatUint(proposalID, 10)),
		ownerAcc.AddressBytes(),
		[]byte(strconv.FormatInt(int64(voter.Type), 10)),
		[]byte(strconv.FormatInt(receiptAmount, 10)),
	)
	a.KAppController.GetCurrentKAppContext().Receipts().Add(receipt)
}

func (a *accountsKapp) updateProposalTotalStaked(proposal *kapps.ProposalData) error {
	if a.forkController.EnableSmartContracts() {
		_, stakedKFI, err := a.KAppController.GetKDAKApp().GetStaking(kdautils.KFIIdentifier)
		if err != nil {
			return err
		}
		proposal.TotalStaked = stakedKFI.TotalStaked
	}
	return nil
}

func (a *accountsKapp) Delegate(sender []byte, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, error) {
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	if len(tc.GetToAddress()) != a.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if len(tc.GetBucketID()) == 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBucketID, process.ErrInvalidArgument.Error())
		return transaction.Transaction_BucketIDInvalid, process.ErrInvalidArgument
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
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
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	if len(tc.GetBucketID()) == 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBucketID, process.ErrInvalidArgument.Error())
		return transaction.Transaction_BucketIDInvalid, process.ErrInvalidArgument
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
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
		return transaction.Transaction_UndelegateError, err
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
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	assetID := tc.GetAssetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldWithdrawNotAvailable, err.Error())
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
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldClaimNotAvailable, err.Error())
		return claimErrorResultCode(err, transaction.Transaction_ClaimError), err
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldNoValidClaims, state.ErrClaimNotAvailable.Error())
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

func (a *accountsKapp) transferPendingRewardsToAllowance(sender []byte, ownerAcc state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	if !a.forkController.EpochRewardsV2() {
		return transaction.Transaction_Ok, nil
	}

	pendingRewards, err := a.KAppController.GetValidatorsKApp().ClaimPendingRewards(sender)
	if err != nil {
		return transaction.Transaction_ClaimError, err
	}

	if pendingRewards > 0 {
		if err = ownerAcc.AddToAllowance(pendingRewards); err != nil {
			return transaction.Transaction_ClaimError, err
		}
	}

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) ClaimAllowance(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	ownerAcc, err := a.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	assetID := tc.GetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	if !bytes.Equal(assetID, kdautils.KLVIdentifier) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetIDInvalid.Error())
		return transaction.Transaction_AssetIDInvalid, common.ErrAssetIDInvalid
	}

	userKDA, err := ownerAcc.GetUserKDA(assetID, nil, a.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// V2 Epoch Rewards: Transfer pending rewards to allowance before claiming
	if resultCode, err := a.transferPendingRewardsToAllowance(sender, ownerAcc); err != nil {
		return resultCode, err
	}

	gains, err := a.ClaimBalance(transaction.ClaimContract_AllowanceClaim, kdautils.KLVIdentifier, ctx.Block(), ownerAcc, nil, nil, userKDA)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldClaimNotAvailable, err.Error())
		return claimErrorResultCode(err, transaction.Transaction_ClaimError), err
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
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	if !utf8.Valid(tc.GetName()) ||
		len(tc.GetName()) > core.MaxNameSize {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidName, common.ErrInvalidValue.Error())
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

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.SetAccountName,
		ctx.ContractID(),
		tc.Name,
		sender,
	))

	return transaction.Transaction_Ok, nil
}

// Helper functions for UpdatePermission
func (a *accountsKapp) validatePermissionParams(tc *transaction.UpdateAccountPermissionContract) error {
	if len(tc.Permissions) > core.MaxAccountPermission {
		return common.ErrInvalidParameter
	}
	return nil
}

func (a *accountsKapp) validateSigners(pType transaction.AccPermission_AccPermissionType, signers []*transaction.AccKey) (int64, []*state.Key, error) {
	if len(signers) == 0 || len(signers) > core.MaxPermissionSigners {
		return 0, nil, common.ErrInvalidParameter
	}

	// Verify if type is valid
	if _, exist := state.Permission_PermissionType_name[int32(pType)]; !exist {
		return 0, nil, common.ErrInvalidParameter
	}

	stateSigners := make([]*state.Key, 0)
	weightSum := int64(0)
	dupCheck := make(map[string]bool)

	var err error
	for _, signer := range signers {
		if len(signer.Address) != a.pubkeyConv.Len() {
			return 0, nil, common.ErrInvalidParameter
		}
		if a.forkController.FixAuditChangesV3() && signer.Weight <= 0 {
			return 0, nil, transaction.ErrInvalidSignerWeight
		}
		// Check for duplicate signers
		if dupCheck[string(signer.Address)] {
			return 0, nil, common.ErrInvalidParameter
		}
		dupCheck[string(signer.Address)] = true

		weightSum, err = a.accumulateSignerWeight(weightSum, signer.Weight)
		if err != nil {
			return 0, nil, err
		}
		stateSigners = append(stateSigners, &state.Key{
			Address: append([]byte{}, signer.Address...),
			Weight:  signer.Weight,
		})
	}

	return weightSum, stateSigners, nil
}

func (a *accountsKapp) accumulateSignerWeight(weightSum, weight int64) (int64, error) {
	if a.forkController.FixAuditChangesV3() {
		return tools.SafeAddI64(weightSum, weight)
	}
	return weightSum + weight, nil
}

func (a *accountsKapp) validatePermissionName(name string) (string, error) {
	if !a.forkController.KdaFpr() {
		return "", nil
	}

	if !utf8.ValidString(name) || len(name) > core.MaxNameSize {
		return "", common.ErrInvalidParameter
	}

	return name, nil
}

func (a *accountsKapp) buildPermission(
	p *transaction.AccPermission,
	permissionIndex int,
	weightSum int64,
	stateSigners []*state.Key,
) (*state.Permission, error) {
	// Verify threshold
	if p.Threshold > weightSum {
		return nil, common.ErrInvalidParameter
	}
	if a.forkController.FixAuditChangesV3() && p.Threshold <= 0 {
		return nil, transaction.ErrInvalidPermissionThreshold
	}

	permName, err := a.validatePermissionName(p.PermissionName)
	if err != nil {
		return nil, err
	}

	return &state.Permission{
		ID:             int32(permissionIndex), // #nosec G115 valid permission index
		PermissionName: permName,
		Type:           state.Permission_PermissionType(p.Type),
		Threshold:      p.Threshold,
		Operations:     append([]byte{}, p.Operations...),
		Signers:        stateSigners,
	}, nil
}

func (a *accountsKapp) createDefaultOwnerPermission(ownerAcc state.UserAccountHandler, permissionIndex int) *state.Permission {
	return &state.Permission{
		ID:         int32(permissionIndex), // #nosec G115 valid permission index
		Type:       state.Permission_Owner,
		Threshold:  1,
		Operations: make([]byte, 0),
		Signers: []*state.Key{
			{
				Address: ownerAcc.AddressBytes(),
				Weight:  1,
			},
		},
	}
}

func authorizerCanUpdatePermission(permissions []*state.Permission, authorizer []byte) bool {
	for _, permission := range permissions {
		for _, signer := range permission.Signers {
			if !bytes.Equal(signer.Address, authorizer) {
				continue
			}

			if signer.Weight >= permission.Threshold &&
				permission.CheckPermissionGrantedForContracts(transaction.TXContract_UpdateAccountPermissionContractType) {
				return true
			}
		}
	}

	return false
}

func (a *accountsKapp) UpdatePermission(authorizer []byte, target []byte, tc *transaction.UpdateAccountPermissionContract) (transaction.Transaction_TXResultCode, error) {
	if err := a.checkReadOnly(); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := a.KAppController.GetCurrentKAppContext()

	if err := a.validatePermissionParams(tc); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	ownerAcc, err := a.GetExistingUserAccount(target)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	if !bytes.Equal(authorizer, target) && !authorizerCanUpdatePermission(ownerAcc.GetPermissions(), authorizer) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrNoPermission.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrNoPermission
	}

	permissions := make([]*state.Permission, 0)
	hasOwner := false

	// Process provided permissions
	for i, p := range tc.Permissions {
		weightSum, stateSigners, err := a.validateSigners(p.Type, p.Signers)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermissionSigs, err.Error())
			return transaction.Transaction_ParameterInvalid, err
		}

		permission, err := a.buildPermission(p, i, weightSum, stateSigners)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, err.Error())
			return transaction.Transaction_ParameterInvalid, err
		}

		hasOwner = hasOwner || p.Type == transaction.AccPermission_Owner
		permissions = append(permissions, permission)
	}

	// Add default owner permission if none provided
	// Owner permission is required and added to the end of the list
	if !hasOwner {
		permissions = append(permissions, a.createDefaultOwnerPermission(ownerAcc, len(permissions)))
	}

	// Update account permissions
	ownerAcc.SetPermissions(permissions)

	if err := a.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateAccountPermission,
		ctx.ContractID(),
		target,
	))

	return transaction.Transaction_Ok, nil
}

func (a *accountsKapp) TokenTypeHasNonce(tokenType kapps.KDAData_EnumAssetType) bool {
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

// claimErrorResultCode maps a ClaimBalance failure to a transaction result code.
// ErrMaxSupplyExceeded surfaces a dedicated code so it can be distinguished from
// generic claim errors by downstream receipts and clients.
func claimErrorResultCode(err error, defaultCode transaction.Transaction_TXResultCode) transaction.Transaction_TXResultCode {
	if errors.Is(err, common.ErrMaxSupplyExceeded) {
		return transaction.Transaction_MaxSupplyExceeded
	}
	return defaultCode
}
