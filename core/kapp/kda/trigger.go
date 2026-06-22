package kda

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	pkda "github.com/klever-io/klever-go/core/process/kda"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// Trigger is a function that triggers an action on a KDA asset
func (k *kdaKapp) Trigger(sender []byte, tc *transaction.AssetTriggerContract, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	assetID, errCode, err := k.processAssetID(ctx, tc.GetAssetID())
	if err != nil {
		return errCode, err
	}

	kdaKApp, asset, err := k.GetKDA(assetID[0])
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	return k.processTriggerType(sender, tc, kdaKApp, assetID, asset, txData)
}

func (k *kdaKapp) processAssetID(ctx kapp.KappContext, assetID []byte) ([][]byte, transaction.Transaction_TXResultCode, error) {
	if assetID == nil {
		return nil, transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	resultID := bytes.Split(assetID, []byte(kapps.Sp))

	if len(resultID) > 2 || len(resultID) == 0 {
		return nil, transaction.Transaction_AssetIDInvalid, common.ErrInvalidValue
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDA,
		ctx.ContractID(),
		resultID[0],
	))

	return resultID, transaction.Transaction_Ok, nil
}

func (k *kdaKapp) processTriggerType(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
	txData [][]byte,
) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	switch tc.GetTriggerType() {
	case transaction.AssetTriggerContract_Mint:
		return k.mintAsset(sender, tc)
	case transaction.AssetTriggerContract_Burn,
		transaction.AssetTriggerContract_Wipe:
		return k.burnAsset(sender, tc)
	case transaction.AssetTriggerContract_Pause, transaction.AssetTriggerContract_Resume:
		return k.handlePauseOrResume(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_ChangeOwner:
		return k.changeOwner(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_AddRole:
		return k.handleAddRole(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_RemoveRole:
		return k.handleRemoveRole(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateMetadata:
		return k.updateMetadata(sender, tc, kdaKApp, assetID, asset, txData)
	case transaction.AssetTriggerContract_StopNFTMint:
		return k.handleStopNFTMint(sender, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_StopRoyaltiesChange:
		return k.handleStopRoyaltiesChange(sender, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateLogo:
		return k.handleUpdateLogo(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateURIs:
		return k.handleUpdateURIs(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_ChangeRoyaltiesReceiver:
		return k.updateRoyaltiesReceiver(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateStaking:
		return k.updateStaking(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateRoyalties:
		return k.updateRoyalties(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_UpdateKDAFeePool:
		return k.updateKDAFeePool(sender, tc, assetID, asset)
	case transaction.AssetTriggerContract_StopNFTMetadataChange:
		return k.handleStopNFTMetadataChange(sender, tc, kdaKApp, assetID, asset)
	case transaction.AssetTriggerContract_ChangeAdmin:
		return k.handleChangeAdmin(sender, tc, kdaKApp, assetID, asset)
	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidTriggerType, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_AssetError, common.ErrAssetTriggerInvalid
	}

}

func (k *kdaKapp) mintAsset(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	return k.KAppController.GetKDAKApp().Mint(sender, &transaction.AssetTriggerContract{
		AssetID:   tc.GetAssetID(),
		Amount:    tc.GetAmount(),
		ToAddress: tc.GetToAddress(),
		Value:     tc.GetValue(),
	})
}

func (k *kdaKapp) burnAsset(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
	toAddress := sender
	if tc.GetTriggerType() == transaction.AssetTriggerContract_Wipe {
		toAddress = tc.GetToAddress()
	}

	return k.KAppController.GetKDAKApp().Burn(sender, &transaction.AssetTriggerContract{
		TriggerType: tc.GetTriggerType(),
		AssetID:     tc.GetAssetID(),
		Amount:      tc.GetAmount(),
		ToAddress:   toAddress,
	})
}

func (k *kdaKapp) handlePauseOrResume(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if tc.GetTriggerType() == transaction.AssetTriggerContract_Pause && !asset.Properties.CanPause {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCannotPause, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_AssetCantBePaused, common.ErrAssetTriggerInvalid
	}

	asset.Attributes.IsPaused = (tc.GetTriggerType() == transaction.AssetTriggerContract_Pause)

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) changeOwner(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !asset.Properties.CanChangeOwner {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetOwnerCantBeChanged, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_AssetOwnerCantBeChanged, common.ErrAssetTriggerInvalid
	}

	// admin cannot change owner
	if !bytes.Equal(asset.OwnerAddress, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	asset.OwnerAddress = tc.GetToAddress()

	if errCode, err := k.KAppController.GetKDAFeesPoolKApp().ChangePoolOwner(asset.ID, sender, tc.GetToAddress()); err != nil {
		return errCode, err
	}

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) checkCanAddRoles(ctx kapp.KappContext, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if asset.GetProperties().GetCanAddRoles() {
		return transaction.Transaction_Ok, nil
	}
	if k.forkController.EnableSmartContracts() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCantAddRoles, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_AssetCantAddRoles, common.ErrAssetTriggerInvalid
	}
	ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCantAddRoles, common.ErrAssetTriggerInvalid.Error())
	return transaction.Transaction_AssetCantBeBurned, common.ErrAssetTriggerInvalid
}

func (k *kdaKapp) handleAddRole(
	sender []byte,
	tc *transaction.AssetTriggerContract,
	kdaKApp state.KAppAccountHandler,
	assetID [][]byte,
	asset *kapps.KDAData,
) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if resCode, err := k.checkCanAddRoles(ctx, asset); err != nil {
		return resCode, err
	}

	if len(tc.GetRole().GetAddress()) != k.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoleAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}
	if k.forkController.EnableSmartContracts() && len(asset.Roles) > core.MaxAssetRoles {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldRoleLimitReached, common.ErrRoleLimitReached.Error())
		return transaction.Transaction_IteratorLimitReached, common.ErrRoleLimitReached
	}

	updated := false
	for i, role := range asset.Roles {
		if bytes.Equal(role.Address, tc.GetRole().GetAddress()) {
			roleInfo := tc.GetRole()
			asset.Roles[i] = &kapps.RolesData{
				Address:             roleInfo.Address,
				HasRoleMint:         roleInfo.HasRoleMint,
				HasRoleSetITOPrices: roleInfo.HasRoleSetITOPrices,
				HasRoleDeposit:      roleInfo.HasRoleDeposit,
				HasRoleTransfer:     roleInfo.HasRoleTransfer,
			}
			updated = true
		}
	}
	if !updated {
		roleInfo := tc.GetRole()
		asset.Roles = append(asset.Roles, &kapps.RolesData{
			Address:             roleInfo.Address,
			HasRoleMint:         roleInfo.HasRoleMint,
			HasRoleSetITOPrices: roleInfo.HasRoleSetITOPrices,
			HasRoleDeposit:      roleInfo.HasRoleDeposit,
			HasRoleTransfer:     roleInfo.HasRoleTransfer,
		})
	}

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleRemoveRole(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	newRoles := make([]*kapps.RolesData, 0)

	for _, role := range asset.Roles {
		if !bytes.Equal(role.Address, tc.GetToAddress()) {
			newRoles = append(newRoles, role)
		}
	}

	asset.Roles = newRoles

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) updateMetadata(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	if !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if !k.TokeTypeHasNonce(asset.AssetType) {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	switch asset.AssetType {
	case kapps.KDAData_NonFungible:
		return k.updateMetadataV1(tc, kdaKApp, assetID, asset, txData)
	case kapps.KDAData_SemiFungible:
		return k.updateMetadataV2(assetID, txData)
	default:
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}
}

func (k *kdaKapp) updateMetadataV1(tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	toAcc, err := k.GetExistingUserAccount(tc.GetToAddress())
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	// check if asset has nonce Metadata only supported with NFT/SFTs
	if len(assetID) != 2 {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetIDInvalid
	}

	// check if asset exists
	userKDA, err := toAcc.GetUserKDA(assetID[0], assetID[1], k.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AssetTypeInvalid, err
	}

	if userKDA.Balance != 1 {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	ctx := k.KAppController.GetCurrentKAppContext()

	if ctx.ContractID() >= len(txData) {
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidContractOrRawDataSize
	}

	if asset.Attributes.IsNFTMetadataChangeStopped {
		if len(userKDA.Metadata) > 0 {
			return transaction.Transaction_NFTMetadataChangeStopped, process.ErrNFTMetadataChangeStopped
		}
	}

	userKDA.MIME = tc.GetMIME()
	userKDA.Metadata = txData[ctx.ContractID()]

	data, err := k.marshalizer.Marshal(userKDA)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	key := kdautils.ToKDAKey(assetID[0], assetID[1])

	err = toAcc.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := k.accountsCacher.UpdateUser(toAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateMetadata,
		ctx.ContractID(),
		toAcc.AddressBytes(),
		assetID[0],
		assetID[1],
	))

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) updateMetadataV2(assetID [][]byte, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	// check if asset has nonce Metadata only supported with NFT/SFTs
	if len(assetID) != 2 {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetIDInvalid
	}

	ctx := k.KAppController.GetCurrentKAppContext()

	if ctx.ContractID() >= len(txData) {
		return transaction.Transaction_ParameterInvalid, process.ErrInvalidContractOrRawDataSize
	}

	args := make([][]byte, 0)
	tokens := strings.Split(string(txData[ctx.ContractID()]), "@")
	for i := 0; i < len(tokens); i++ {
		decoded, err := hex.DecodeString(tokens[i])
		if err != nil {
			return transaction.Transaction_ParameterInvalid, process.ErrInvalidArgument
		}
		args = append(args, decoded)
	}

	if err := k.KAppController.GetSystemAccountKApp().SFTSetMetadata(assetID[0], assetID[1], args); err != nil {
		return transaction.Transaction_AssetError, err
	}

	// add receipt
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDA,
		k.KAppController.GetCurrentKAppContext().ContractID(),
		kdautils.ToKDAKeyWithouPrefix(assetID[0], assetID[1]),
	))

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) handleStopNFTMint(sender []byte, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if k.forkController.EnableSmartContracts() && !k.TokeTypeHasNonce(asset.AssetType) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	if asset.Attributes.IsNFTMintStopped {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldNFTMintStopped, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_NFTMintStopped, common.ErrAssetTriggerInvalid
	}

	asset.Attributes.IsNFTMintStopped = true

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleStopRoyaltiesChange(sender []byte, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if asset.Attributes.IsRoyaltiesChangeStopped {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldRoyaltiesChangeStopped, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_RoyaltiesChangeStopped, common.ErrAssetTriggerInvalid
	}

	asset.Attributes.IsRoyaltiesChangeStopped = true

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleUpdateLogo(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	asset.Logo = tc.GetLogo()

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleUpdateURIs(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(tc.GetURIs()) > core.MaxURIMapSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	for key, uri := range tc.GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}

	asset.URIs = tc.GetURIs()

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) updateRoyaltiesReceiver(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if asset.Royalties == nil {
		asset.Royalties = &kapps.RoyaltiesData{}
	}

	asset.Royalties.Address = tc.GetToAddress()

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) updateStaking(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if tc.GetStaking() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if k.TokeTypeHasNonce(asset.AssetType) {
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	stakingKapp, staking, err := k.GetStaking(asset.ID)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if staking == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if staking.InterestType != kapps.StakingData_EnumInterestType(tc.GetStaking().GetType()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	staking.MinEpochsToClaim = tc.GetStaking().GetMinEpochsToClaim()
	staking.MinEpochsToUnstake = tc.GetStaking().GetMinEpochsToUnstake()
	staking.MinEpochsToWithdraw = tc.GetStaking().GetMinEpochsToWithdraw()

	ctx := k.KAppController.GetCurrentKAppContext()

	switch tc.GetStaking().GetType() {
	case transaction.StakingInfo_APRI:
		staking.InterestType = kapps.StakingData_APRI
		staking.APR = append(staking.APR, &kapps.APRData{
			Timestamp: ctx.Block().GetTimestamp(),
			Value:     tc.GetStaking().GetAPR(),
		})

		if k.forkController.EnableSmartContracts() {
			aprLen := len(staking.APR)

			staking.APR[aprLen-1].Epoch = ctx.Block().GetEpoch()

			if aprLen > core.MaxAPRPercentageUpdates {
				staking.APR = staking.APR[(aprLen - core.MaxAPRPercentageUpdates):]
			}
		}
	case transaction.StakingInfo_FPRI:
		staking.InterestType = kapps.StakingData_FPRI
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrAssetTypeInvalid
	}

	err = k.SetStaking(stakingKapp, asset.ID, staking)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if err := k.accountsCacher.UpdateKapp(stakingKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) updateRoyalties(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if asset.Attributes.IsRoyaltiesChangeStopped {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldRoyaltiesChangeStopped, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_RoyaltiesChangeStopped, common.ErrAssetTriggerInvalid
	}

	if tc.GetRoyalties() == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if asset.Royalties == nil {
		asset.Royalties = &kapps.RoyaltiesData{}
	}

	if len(tc.GetRoyalties().GetAddress()) > 0 {
		if len(tc.GetRoyalties().GetAddress()) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidOwnerAddr
		}

		asset.Royalties.Address = tc.GetRoyalties().GetAddress()
	} else {
		asset.Royalties.Address = asset.OwnerAddress
	}

	// validate max transfer percentage
	if err := validateRoyaltiesTransferLimit(tc.GetRoyalties()); err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if err := validateSplitRoyaltyPercentages(tc.GetRoyalties(), k.forkController.FixMarketBuyOverflow()); err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	switch asset.AssetType {
	case kapps.KDAData_NonFungible,
		kapps.KDAData_SemiFungible:
		errCode, err := k.handleUpdateRoyaltiesNFTandSFT(tc, asset)
		if err != nil {
			return errCode, err
		}

	case kapps.KDAData_Fungible:
		errCode, err := k.handleUpdateRoyaltiesFT(tc, asset)
		if err != nil {
			return errCode, err
		}

	default:
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleUpdateRoyaltiesNFTandSFT(tc *transaction.AssetTriggerContract, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	sumSplitMarketPercent := uint32(0)
	sumSplitMarketFixed := uint32(0)
	sumSplitITOPercent := uint32(0)
	sumSplitITOFixed := uint32(0)
	sumSplitTransferFixed := uint32(0)
	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)

	for key, value := range tc.GetRoyalties().GetSplitRoyalties() {
		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			return transaction.Transaction_AccountError, process.ErrInvalidSplitRoyaltiesAddr
		}

		if len(decodedAddress) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidSplitRoyaltiesAddr
		}

		sumSplitMarketPercent += value.GetPercentMarketPercentage()
		sumSplitMarketFixed += value.GetPercentMarketFixed()
		sumSplitITOPercent += value.GetPercentITOPercentage()
		sumSplitITOFixed += value.GetPercentITOFixed()
		sumSplitTransferFixed += value.GetPercentTransferFixed()

		splitRoyalties[key] = &kapps.RoyaltySplitData{
			PercentMarketPercentage: value.GetPercentMarketPercentage(),
			PercentMarketFixed:      value.GetPercentMarketFixed(),
			PercentTransferFixed:    value.GetPercentTransferFixed(),
			PercentITOPercentage:    value.GetPercentITOPercentage(),
			PercentITOFixed:         value.GetPercentITOFixed(),
		}
	}

	if !pkda.CheckValid100Params(sumSplitMarketPercent,
		sumSplitMarketFixed, sumSplitTransferFixed, sumSplitITOPercent, sumSplitITOFixed) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !pkda.CheckValid100Params(tc.GetRoyalties().GetMarketPercentage(),
		tc.GetRoyalties().GetITOPercentage()) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetMarketFixed() < 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetITOFixed() < 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetTransferFixed() < 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	asset.Royalties.TransferFixed = tc.GetRoyalties().GetTransferFixed()
	asset.Royalties.MarketFixed = tc.GetRoyalties().GetMarketFixed()
	asset.Royalties.MarketPercentage = tc.GetRoyalties().GetMarketPercentage()
	asset.Royalties.ITOFixed = tc.GetRoyalties().GetITOFixed()
	asset.Royalties.ITOPercentage = tc.GetRoyalties().GetITOPercentage()
	asset.Royalties.SplitRoyalties = splitRoyalties

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) handleUpdateRoyaltiesFT(tc *transaction.AssetTriggerContract, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {

	splitRoyalties, err := k.processSplitRoyalties(tc)
	if err != nil {
		return processErrorCode(err)
	}

	if err := validateRoyalties(tc, k.forkController.EnableSmartContracts()); err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	tp, err := processRoyaltiesTransferPercentage(tc.GetRoyalties().GetTransferPercentage())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	// only overwrite empty post fork
	if k.forkController.EnableSmartContracts() || len(tp) > 0 {
		asset.Royalties.TransferPercentage = tp
	}

	if k.forkController.EnableSmartContracts() {
		asset.Royalties.MarketFixed = tc.GetRoyalties().GetMarketFixed()
		asset.Royalties.MarketPercentage = tc.GetRoyalties().GetMarketPercentage()
	}

	asset.Royalties.TransferFixed = tc.GetRoyalties().GetTransferFixed()
	asset.Royalties.ITOFixed = tc.GetRoyalties().GetITOFixed()
	asset.Royalties.ITOPercentage = tc.GetRoyalties().GetITOPercentage()
	asset.Royalties.SplitRoyalties = splitRoyalties

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) processSplitRoyalties(tc *transaction.AssetTriggerContract) (map[string]*kapps.RoyaltySplitData, error) {
	sumSplitTransferPercentage := uint32(0)
	sumSplitTransferFixed := uint32(0)
	sumSplitITOPercentage := uint32(0)
	sumSplitITOFixed := uint32(0)
	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)

	for key, value := range tc.GetRoyalties().GetSplitRoyalties() {
		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			return nil, process.ErrInvalidSplitRoyaltiesAddr
		}

		if len(decodedAddress) != k.pubkeyConv.Len() {
			return nil, process.ErrInvalidSplitRoyaltiesAddr
		}

		sumSplitTransferPercentage += value.GetPercentTransferPercentage()
		sumSplitTransferFixed += value.GetPercentTransferFixed()
		sumSplitITOPercentage += value.GetPercentITOPercentage()
		sumSplitITOFixed += value.GetPercentITOFixed()

		splitRoyalties[key] = &kapps.RoyaltySplitData{
			PercentTransferPercentage: value.GetPercentTransferPercentage(),
			PercentTransferFixed:      value.GetPercentTransferFixed(),
			PercentITOPercentage:      value.GetPercentITOPercentage(),
			PercentITOFixed:           value.GetPercentITOFixed(),
		}
	}

	if !pkda.CheckValid100Params(sumSplitTransferPercentage,
		sumSplitTransferFixed, sumSplitITOPercentage, sumSplitITOFixed) {
		return nil, common.ErrInvalidValue
	}

	return splitRoyalties, nil
}

func processErrorCode(err error) (transaction.Transaction_TXResultCode, error) {
	if errors.Is(err, common.ErrInvalidValue) {
		return transaction.Transaction_ParameterInvalid, err
	}

	return transaction.Transaction_AccountError, err
}

func validateRoyalties(tc *transaction.AssetTriggerContract, checkMarketPercentage bool) error {
	if !pkda.CheckValid100Params(tc.GetRoyalties().GetITOPercentage()) {
		return common.ErrInvalidValue
	}

	if checkMarketPercentage &&
		!pkda.CheckValid100Params(tc.GetRoyalties().GetMarketPercentage()) {
		return common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetITOFixed() < 0 || tc.GetRoyalties().GetTransferFixed() < 0 {
		return common.ErrInvalidValue
	}

	return nil
}

func processRoyaltiesTransferPercentage(tp []*transaction.RoyaltyInfo) ([]*kapps.RoyaltyData, error) {
	var royaltiesTransfer []*kapps.RoyaltyData
	if len(tp) > 0 {
		royaltiesTransfer = make([]*kapps.RoyaltyData, len(tp))
		for i, royalty := range tp {
			if royalty.Amount < 0 || royalty.Percentage > core.HundredPercent {
				return nil, common.ErrInvalidValue
			}

			royaltiesTransfer[i] = &kapps.RoyaltyData{
				Amount:     royalty.Amount,
				Percentage: royalty.Percentage,
			}
		}

		sort.SliceStable(royaltiesTransfer, func(i, j int) bool {
			return royaltiesTransfer[i].Amount < royaltiesTransfer[j].Amount
		})
	}

	return royaltiesTransfer, nil
}

func (k *kdaKapp) updateKDAFeePool(sender []byte, tc *transaction.AssetTriggerContract, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	// after fork, validation happens inside the KDAFeesPoolKapp
	if !k.forkController.FPRComputeAndKdaFeeFlow() && !k.isOwnerOrAdmin(sender, asset) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if tc.GetKDAPool() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if k.TokeTypeHasNonce(asset.AssetType) || len(assetID) > 2 {
		return transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	return k.KAppController.GetKDAFeesPoolKApp().UpdatePool(asset.ID, asset.OwnerAddress, sender, tc.KDAPool)
}

func (k *kdaKapp) handleStopNFTMetadataChange(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if k.forkController.EnableSmartContracts() && !k.TokeTypeHasNonce(asset.AssetType) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	if asset.Attributes.IsNFTMetadataChangeStopped {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldNFTMetadataChangeStopped, common.ErrAssetTriggerInvalid.Error())
		return transaction.Transaction_NFTMetadataChangeStopped, common.ErrAssetTriggerInvalid
	}

	asset.Attributes.IsNFTMetadataChangeStopped = true

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) handleChangeAdmin(sender []byte, tc *transaction.AssetTriggerContract, kdaKApp state.KAppAccountHandler, assetID [][]byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if !k.isOwnerOrAdmin(sender, asset) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(tc.GetToAddress()) > 0 && len(tc.GetToAddress()) != k.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	asset.AdminAddress = tc.GetToAddress()

	return k.updateKApp(kdaKApp, assetID[0], asset)
}

func (k *kdaKapp) isOwnerOrAdmin(sender []byte, asset *kapps.KDAData) bool {
	return bytes.Equal(asset.OwnerAddress, sender) || bytes.Equal(asset.AdminAddress, sender)
}

func (k *kdaKapp) updateKApp(kdaKApp state.KAppAccountHandler, assetID []byte, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if err := k.SetKDA(kdaKApp, assetID, asset); err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKApp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}
