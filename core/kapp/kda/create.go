package kda

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

func (k *kdaKapp) Create(sender []byte, tc *transaction.CreateAssetContract) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	err := kda.CheckBasicCreateArguments(tc)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetParams, err.Error())
		return transaction.Transaction_ContractInvalid, err
	}

	assetIdentifier, roles, status, err := k.ValidateCreateAsset(ctx, sender, tc)
	if err != nil {
		return status, err
	}

	asset := kapps.KDAData{
		ID:                assetIdentifier,
		AssetType:         kapps.KDAData_EnumAssetType(tc.GetType()),
		Name:              tc.GetName(),
		Ticker:            tc.GetTicker(),
		OwnerAddress:      tc.GetOwnerAddress(),
		AdminAddress:      tc.GetAdminAddress(),
		ControllerAddress: tc.GetControllerAddress(),
		Logo:              tc.GetLogo(),
		URIs:              tc.GetURIs(),
		Precision:         0,
		InitialSupply:     0,
		CirculatingSupply: 0,
		MaxSupply:         tc.GetMaxSupply(),
		IssueDate:         int64(ctx.Block().GetTimestamp()),
		Royalties: &kapps.RoyaltiesData{
			Address: tc.GetOwnerAddress(),
		},
		Roles: roles,
		Attributes: &kapps.AttributesData{
			IsPaused:                   tc.GetAttributes().GetIsPaused(),
			IsNFTMintStopped:           false,
			IsRoyaltiesChangeStopped:   tc.GetAttributes().GetIsRoyaltiesChangeStopped(),
			IsNFTMetadataChangeStopped: tc.GetAttributes().GetIsNFTMetadataChangeStopped(),
		},
		Properties: &kapps.PropertiesData{
			CanFreeze:      tc.GetProperties().GetCanFreeze(),
			CanWipe:        tc.GetProperties().GetCanWipe(),
			CanPause:       tc.GetProperties().GetCanPause(),
			CanMint:        tc.GetProperties().GetCanMint(),
			CanBurn:        tc.GetProperties().GetCanBurn(),
			CanChangeOwner: tc.GetProperties().GetCanChangeOwner(),
			CanAddRoles:    tc.GetProperties().GetCanAddRoles(),
			LimitTransfer:  tc.GetProperties().GetLimitTransfer(),
		},
	}

	if tc.GetRoyalties() != nil && len(tc.GetRoyalties().GetAddress()) > 0 {
		if len(tc.GetRoyalties().GetAddress()) != k.pubkeyConv.Len() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, process.ErrInvalidOwnerAddr.Error())
			return transaction.Transaction_AccountError, process.ErrInvalidOwnerAddr
		}

		asset.Royalties.Address = tc.GetRoyalties().GetAddress()
	}

	// validate max transfer percentage
	if err := validateRoyaltiesTransferLimit(tc.GetRoyalties()); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	switch asset.AssetType {
	case kapps.KDAData_NonFungible,
		kapps.KDAData_SemiFungible:

		result, err := k.createNFT(ctx, tc, &asset)
		if err != nil {
			return result, err
		}

	case kapps.KDAData_Fungible:
		result, err := k.createFungible(ctx, tc, &asset, assetIdentifier)
		if err != nil {
			return result, err
		}
	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.CreateKDA,
		ctx.ContractID(),
		asset.ID,
	))

	resultCode, err := k.updateAsset(assetIdentifier, &asset)
	if err != nil {
		return resultCode, err
	}

	ctx.SetReturnData([][]byte{asset.ID})

	return transaction.Transaction_Ok, nil
}

func (k kdaKapp) updateAsset(assetID []byte, assetData *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	kdaKapp, _, err := k.GetKDA(nil)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	err = k.SetKDA(kdaKapp, assetID, assetData)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) ValidateCreateAsset(ctx kapp.KappContext, sender []byte, tc *transaction.CreateAssetContract) ([]byte, []*kapps.RolesData, transaction.Transaction_TXResultCode, error) {
	status, err := kda.ValidateCreateAsset(tc, k.forkController, k.pubkeyConv)
	if err != nil {
		return nil, nil, status, err
	}

	contractID := 0
	if k.forkController.EnableSmartContracts() {
		contractID = ctx.ContractID()
	}
	assetIdentifier := kda.CreateNewAssetIdentifier(k.hasher, ctx.Block().GetRandSeed(), sender, ctx.TxNonce(), tc.GetTicker(), contractID)

	// validate if asset already exists
	_, _, err = k.GetKDA(assetIdentifier)
	if err == nil {
		return nil, nil, transaction.Transaction_AssetError, common.ErrAssetAlreadyExists
	}
	if !errors.Is(err, common.ErrAssetNotFound) {
		return nil, nil, transaction.Transaction_AssetError, err
	}

	roles, status, err := k.parseRoles(tc.GetRoles())
	if err != nil {
		return nil, nil, status, err
	}

	if len(kapps.KDAData_EnumAssetType_name[int32(tc.GetType())]) == 0 {
		return nil, nil, transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	status, err = kda.ValidateLogoAndUri(tc)
	if err != nil {
		return nil, nil, status, err
	}

	return assetIdentifier, roles, transaction.Transaction_Ok, nil
}

func (k *kdaKapp) createNFT(ctx kapp.KappContext, tc *transaction.CreateAssetContract, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !k.forkController.EnableSmartContracts() &&
		asset.AssetType == kapps.KDAData_SemiFungible {
		// check Create SFT only post fork
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	if !asset.Properties.CanMint {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetCannotMint, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	switch asset.AssetType {
	case kapps.KDAData_SemiFungible:
		asset.Precision = tc.GetPrecision()

		if tc.GetInitialSupply() != 0 {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidSupply, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
		if err := kda.CheckPrecision(asset.Precision); err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPrecision, err.Error())
			return transaction.Transaction_ParameterInvalid, err
		}

	case kapps.KDAData_NonFungible:
		if !kdautils.IsNFTContractValid(tc.GetInitialSupply(), tc.GetPrecision()) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetParams, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}

	if tc.GetAttributes().GetIsNFTMintStopped() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetMintStopped, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	result, err := k.nftSetRoyalties(ctx, tc, asset)
	if err != nil {
		return result, err
	}

	return transaction.Transaction_Ok, nil
}

func (k *kdaKapp) nftSetRoyalties(ctx kapp.KappContext, tc *transaction.CreateAssetContract, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if tc.GetRoyalties() == nil {
		return transaction.Transaction_Ok, nil
	}

	sumSplitMarketPercent := uint32(0)
	sumSplitMarketFixed := uint32(0)
	sumSplitITOPercent := uint32(0)
	sumSplitITOFixed := uint32(0)
	sumSplitTransferFixed := uint32(0)
	splitRoyalties := make(map[string]*kapps.RoyaltySplitData)

	for key, value := range tc.GetRoyalties().GetSplitRoyalties() {
		decodedAddress, err := hex.DecodeString(key)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidSplitRoyalties, process.ErrInvalidSplitRoyaltiesAddr.Error())
			return transaction.Transaction_AccountError, process.ErrInvalidSplitRoyaltiesAddr
		}

		if len(decodedAddress) != k.pubkeyConv.Len() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidSplitRoyalties, process.ErrInvalidSplitRoyaltiesAddr.Error())
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

	if !kda.CheckValid100Params(sumSplitMarketPercent, sumSplitMarketFixed,
		sumSplitTransferFixed, sumSplitITOPercent, sumSplitITOFixed) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !kda.CheckValid100Params(tc.GetRoyalties().GetMarketPercentage(),
		tc.GetRoyalties().GetITOPercentage()) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue

	}

	if tc.GetRoyalties().GetMarketFixed() < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetITOFixed() < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetRoyalties().GetTransferFixed() < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
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

func (k *kdaKapp) parseRoles(tcRoles []*transaction.RolesInfo) ([]*kapps.RolesData, transaction.Transaction_TXResultCode, error) {
	roles := make([]*kapps.RolesData, 0)

	for _, newRole := range tcRoles {
		if len(newRole.Address) != k.pubkeyConv.Len() {
			return nil, transaction.Transaction_AccountError, process.ErrInvalidRoleAddr
		}

		roleAlreadyExists := false
		for i := range roles {
			if bytes.Equal(roles[i].Address, newRole.Address) {
				roleAlreadyExists = true
				break
			}
		}

		if !roleAlreadyExists {
			roles = append(roles, &kapps.RolesData{
				Address:             newRole.Address,
				HasRoleMint:         newRole.HasRoleMint,
				HasRoleSetITOPrices: newRole.HasRoleSetITOPrices,
				HasRoleDeposit:      newRole.HasRoleDeposit,
				HasRoleTransfer:     newRole.HasRoleTransfer,
			})
		}
	}

	return roles, transaction.Transaction_Ok, nil
}

func (k *kdaKapp) createFungible(ctx kapp.KappContext, tc *transaction.CreateAssetContract,
	asset *kapps.KDAData, assetIdentifier []byte) (transaction.Transaction_TXResultCode, error) {
	asset.Precision = tc.GetPrecision()
	err := kda.CheckPrecision(asset.Precision)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	asset.InitialSupply = tc.GetInitialSupply()
	asset.CirculatingSupply = tc.GetInitialSupply()
	if !kdautils.IsSupplyValid(
		asset.Properties.CanMint,
		asset.InitialSupply,
		asset.CirculatingSupply,
		asset.MaxSupply,
	) {
		return transaction.Transaction_ParameterInvalid, process.ErrSupplyNotValid
	}

	if tc.GetRoyalties() != nil {
		sumSplitTransferPercentage := uint32(0)
		sumSplitTransferFixed := uint32(0)
		sumSplitITOPercentage := uint32(0)
		sumSplitITOFixed := uint32(0)
		splitRoyalties := make(map[string]*kapps.RoyaltySplitData)

		for key, value := range tc.GetRoyalties().GetSplitRoyalties() {
			decodedAddress, err := hex.DecodeString(key)
			if err != nil {
				return transaction.Transaction_AccountError, process.ErrInvalidSplitRoyaltiesAddr
			}

			if len(decodedAddress) != k.pubkeyConv.Len() {
				return transaction.Transaction_AccountError, process.ErrInvalidSplitRoyaltiesAddr
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

		if !kda.CheckValid100Params(sumSplitTransferPercentage, sumSplitTransferFixed,
			sumSplitITOPercentage, sumSplitITOFixed,
			tc.GetRoyalties().GetITOPercentage()) {

			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if tc.GetRoyalties().GetITOFixed() < 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if tc.GetRoyalties().GetTransferFixed() < 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if len(tc.GetRoyalties().GetTransferPercentage()) > 0 {
			royaltiesTransfer := make([]*kapps.RoyaltyData, len(tc.GetRoyalties().GetTransferPercentage()))
			for i, royalty := range tc.GetRoyalties().GetTransferPercentage() {
				if royalty.Amount < 0 || royalty.Percentage > core.HundredPercent {
					return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
				}

				royaltiesTransfer[i] = &kapps.RoyaltyData{
					Amount:     royalty.Amount,
					Percentage: royalty.Percentage,
				}
			}

			sort.SliceStable(royaltiesTransfer, func(i, j int) bool {
				return royaltiesTransfer[i].Amount < royaltiesTransfer[j].Amount
			})

			asset.Royalties.TransferPercentage = royaltiesTransfer
		}

		if k.forkController.KdaFpr() {
			if tc.GetRoyalties().GetTransferFixed() < 0 {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			asset.Royalties.TransferFixed = tc.GetRoyalties().GetTransferFixed()
		}

		asset.Royalties.ITOFixed = tc.GetRoyalties().GetITOFixed()
		asset.Royalties.ITOPercentage = tc.GetRoyalties().GetITOPercentage()
		asset.Royalties.SplitRoyalties = splitRoyalties
	}

	if asset.Properties.CanFreeze {
		stakingKapp, _, err := k.GetStaking(nil)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		staking := kapps.StakingData{
			MinEpochsToClaim:    tc.GetStaking().GetMinEpochsToClaim(),
			MinEpochsToUnstake:  tc.GetStaking().GetMinEpochsToUnstake(),
			MinEpochsToWithdraw: tc.GetStaking().GetMinEpochsToWithdraw(),
		}

		switch tc.GetStaking().GetType() {
		case transaction.StakingInfo_APRI:
			staking.InterestType = kapps.StakingData_APRI
			staking.APR = append(staking.APR, &kapps.APRData{
				Timestamp: ctx.Block().GetTimestamp(),
				Value:     tc.GetStaking().GetAPR(),
			})

			if k.forkController.EnableSmartContracts() {
				staking.APR[len(staking.APR)-1].Epoch = ctx.Block().GetEpoch()
			}
		case transaction.StakingInfo_FPRI:
			staking.InterestType = kapps.StakingData_FPRI
			staking.FPR = append(staking.FPR, &kapps.FPRData{
				Epoch: ctx.Block().GetEpoch(),
			})
		default:
			return transaction.Transaction_ParameterInvalid, common.ErrAssetTypeInvalid
		}

		err = k.SetStaking(stakingKapp, assetIdentifier, &staking)
		if err != nil {
			return transaction.Transaction_SetStakingErr, err
		}

		if err := k.accountsCacher.UpdateKapp(stakingKapp); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}
	}

	if asset.InitialSupply > 0 {
		ownerAcc, err := k.LoadUserAccount(asset.GetOwnerAddress())
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		if err = ownerAcc.AddToBalance(asset.InitialSupply, asset.ID, k.forkController.EnableSmartContracts()); err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := k.accountsCacher.UpdateUser(ownerAcc); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		asset.MintedValue = asset.InitialSupply

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			core.ZeroAddress,
			asset.GetOwnerAddress(),
			[]byte(strconv.FormatInt(asset.InitialSupply, 10)),
			asset.ID,
			nil,
			[]byte{byte(asset.AssetType)},
		))
	}

	return transaction.Transaction_Ok, nil
}
