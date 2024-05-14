package kda

import (
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

func (k *kdaKapp) Mint(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	acntDst, err := k.LoadUserAccount(tc.GetToAddress())
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	assetID, internalID, kdaAcc, kda, resultCode, err := k.parseAndLoadKDA(tc.GetAssetID())
	if err != nil {
		return resultCode, err
	}

	if !kda.Properties.CanMint {
		return transaction.Transaction_AssetCantBeMinted, common.ErrAssetTriggerInvalid
	}

	if tc.GetAmount() <= 0 {
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidArgument
	}

	role, err := kda.GetRoleByAddress(sender)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if !role.HasRoleMint {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if k.forkController.EnableSmartContracts() {
		// if asset is not SFT, internalID must be empty
		if kda.AssetType != kapps.KDAData_SemiFungible && len(internalID) > 0 {
			return transaction.Transaction_AssetError, process.ErrInvalidArgument
		}

		// if asset is not SFT, cannot have value set
		if kda.AssetType != kapps.KDAData_SemiFungible && tc.GetValue() != 0 {
			return transaction.Transaction_AssetError, process.ErrInvalidArgument
		}
	}

	switch kda.AssetType {
	case kapps.KDAData_NonFungible:
		return k.processNonFungibleMint(tc, assetID, acntDst, kdaAcc, kda)
	case kapps.KDAData_SemiFungible:
		return k.processSemiFungibleMint(tc, assetID, internalID, acntDst, kdaAcc, kda)
	case kapps.KDAData_Fungible:
		return k.processFungibleMint(tc, assetID, acntDst, kdaAcc, kda)
	default:
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidUnitValue
	}
}

func (k *kdaKapp) processNonFungibleMint(
	tc *transaction.AssetTriggerContract,
	assetID []byte,
	acntDst state.UserAccountHandler,
	kdaAcc state.KAppAccountHandler,
	kda *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	if kda.Attributes.IsNFTMintStopped {
		return transaction.Transaction_NFTMintStopped, process.ErrInvalidArgument
	}

	if tc.GetAmount() > k.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MaxNFTMintBatch) {
		return transaction.Transaction_AssetError, process.ErrInvalidArgument
	}

	if kda.MaxSupply > 0 && kda.CirculatingSupply+tc.GetAmount() > kda.MaxSupply {
		return transaction.Transaction_AssetError, common.ErrMaxSupplyExceeded
	}

	data, err := k.marshalizer.Marshal(&kapps.UserKDA{Balance: 1})
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	mintedTokens := make([][]byte, 0, tc.GetAmount())

	for i := int64(0); i < tc.GetAmount(); i++ {
		kda.MintedValue++
		kda.CirculatingSupply++

		if kda.MintedValue <= 0 {
			// prevent overflow
			return transaction.Transaction_AssetError, process.ErrSupplyNotValid
		}

		internalID := []byte(strconv.FormatInt(kda.MintedValue, 10))

		mintedTokens = append(mintedTokens, internalID)

		key := kdautils.ToKDAKey(assetID, internalID)

		err = acntDst.DataTrieTracker().SaveKeyValue(key, data)
		if err != nil {
			return transaction.Transaction_AccountError, err
		}

		k.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			k.KAppController.GetCurrentKAppContext().ContractID(),
			core.ZeroAddress,
			acntDst.AddressBytes(),
			[]byte("1"),
			kda.ID,
			internalID,
			[]byte{byte(kda.AssetType)},
		))
	}

	if err := k.accountsCacher.UpdateUser(acntDst); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = k.SetKDA(kdaAcc, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx := k.KAppController.GetCurrentKAppContext()
	ctx.SetReturnData(mintedTokens)

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) processSemiFungibleMint(
	tc *transaction.AssetTriggerContract,
	assetID []byte,
	internalID []byte,
	acntDst state.UserAccountHandler,
	kdaAcc state.KAppAccountHandler,
	kda *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	// check if fork is enabled
	if !k.forkController.EnableSmartContracts() {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	if kda.Attributes.IsNFTMintStopped {
		return transaction.Transaction_NFTMintStopped, process.ErrInvalidArgument
	}

	// if internalID is not provided, create asset
	// else add quantity to existing asset
	if len(internalID) == 0 {
		return k.processSemiFungibleMintNew(tc, assetID, acntDst, kdaAcc, kda)
	}

	if tc.GetValue() != 0 {
		return transaction.Transaction_AssetError, process.ErrInvalidArgument
	}

	// add quantity to existing asset
	return k.processSemiFungibleAddQuantity(tc, assetID, internalID, acntDst, kda)
}

func (k *kdaKapp) processSemiFungibleMintNew(
	tc *transaction.AssetTriggerContract,
	assetID []byte,
	acntDst state.UserAccountHandler,
	kdaAcc state.KAppAccountHandler,
	kda *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	if tc.GetValue() < 0 || tc.GetValue() > 0 && tc.GetAmount() > tc.GetValue() {
		return transaction.Transaction_AssetError, process.ErrInvalidArgument
	}

	if kda.MaxSupply > 0 && kda.CirculatingSupply+1 > kda.MaxSupply {
		return transaction.Transaction_AssetError, common.ErrMaxSupplyExceeded
	}
	kda.MintedValue++
	kda.CirculatingSupply++

	// prevent overflow
	if kda.MintedValue <= 0 {
		return transaction.Transaction_AssetError, process.ErrSupplyNotValid
	}

	internalID := []byte(strconv.FormatInt(kda.MintedValue, 10))
	mintedTokens := [][]byte{internalID}

	err := k.SetKDA(kdaAcc, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx := k.KAppController.GetCurrentKAppContext()
	ctx.SetReturnData(mintedTokens)

	hash := k.hasher.Compute(
		string(k.KAppController.GetCurrentKAppContext().Block().GetRandSeed()) +
			string(k.KAppController.GetCurrentKAppContext().TxHash()) +
			string(assetID) +
			string(internalID),
	)

	if err := k.KAppController.GetSystemAccountKApp().SFTCreateMeta(assetID, internalID, tc.GetValue(), hash); err != nil {
		return transaction.Transaction_AssetError, err
	}

	// add minted tokens to account
	return k.processSemiFungibleAddQuantity(tc, assetID, internalID, acntDst, kda)
}

func (k *kdaKapp) processSemiFungibleAddQuantity(
	tc *transaction.AssetTriggerContract,
	assetID []byte,
	internalID []byte,
	acntDst state.UserAccountHandler,
	kda *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	// check valid internalID and if it have been "minted"/created"
	internalIDInt, err := strconv.ParseInt(string(internalID), 10, 64)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}
	if kda.MintedValue < internalIDInt {
		return transaction.Transaction_AssetError, common.ErrAssetIDInvalid
	}

	if err = acntDst.AddToBalanceWithNonce(tc.GetAmount(), assetID, internalID, k.forkController.EnableSmartContracts()); err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := k.KAppController.GetSystemAccountKApp().SFTAddCirculation(assetID, internalID, tc.GetAmount()); err != nil {
		return transaction.Transaction_AssetError, err
	}

	k.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		k.KAppController.GetCurrentKAppContext().ContractID(),
		core.ZeroAddress,
		acntDst.AddressBytes(),
		[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
		kda.ID,
		internalID,
		[]byte{byte(kda.AssetType)},
	))

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) processFungibleMint(tc *transaction.AssetTriggerContract, assetID []byte, acntDst state.UserAccountHandler, kdaKapp state.KAppAccountHandler, kda *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	kda.MintedValue += tc.GetAmount()
	kda.CirculatingSupply += tc.GetAmount()

	if kda.MintedValue <= 0 {
		// prevent overflow
		return transaction.Transaction_AssetError, process.ErrSupplyNotValid
	}

	if kda.MaxSupply > 0 && kda.CirculatingSupply > kda.MaxSupply {
		return transaction.Transaction_AssetError, common.ErrMaxSupplyExceeded
	}

	err := acntDst.AddToBalance(tc.GetAmount(), assetID, k.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	k.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		k.KAppController.GetCurrentKAppContext().ContractID(),
		core.ZeroAddress,
		acntDst.AddressBytes(),
		[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
		assetID,
		nil,
		[]byte{byte(kda.AssetType)},
	))

	if err := k.accountsCacher.UpdateUser(acntDst); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = k.SetKDA(kdaKapp, assetID, kda)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}
