package kda

import (
	"bytes"
	"encoding/hex"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	pkda "github.com/klever-io/klever-go/core/process/kda"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

func (k *kdaKapp) Trigger(sender []byte, tc *transaction.AssetTriggerContract, txData [][]byte) (transaction.Transaction_TXResultCode, error) {
	ctx := k.KAppController.GetCurrentKAppContext()

	if tc.GetAssetID() == nil {
		return transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	assetID := bytes.Split(tc.GetAssetID(), []byte(kapps.Sp))
	if len(assetID) > 2 || len(assetID) == 0 {
		return transaction.Transaction_AssetIDInvalid, common.ErrInvalidValue
	}

	kdaKapp, asset, err := k.GetKDA(assetID[0])
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDA,
		ctx.ContractID(),
		assetID[0],
	))

	switch tc.GetTriggerType() {
	case transaction.AssetTriggerContract_Mint:
		return k.KAppController.GetKDAKApp().Mint(sender, &transaction.AssetTriggerContract{AssetID: tc.GetAssetID(), Amount: tc.GetAmount(), ToAddress: tc.GetToAddress(), Value: tc.GetValue()})

	case transaction.AssetTriggerContract_Burn:
		return k.KAppController.GetKDAKApp().Burn(sender, &transaction.AssetTriggerContract{TriggerType: tc.GetTriggerType(), AssetID: tc.GetAssetID(), Amount: tc.GetAmount(), ToAddress: sender})

	case transaction.AssetTriggerContract_Wipe:
		return k.KAppController.GetKDAKApp().Burn(sender, &transaction.AssetTriggerContract{TriggerType: tc.GetTriggerType(), AssetID: tc.GetAssetID(), Amount: tc.GetAmount(), ToAddress: tc.GetToAddress()})

	case transaction.AssetTriggerContract_Pause:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !asset.Properties.CanPause {
			return transaction.Transaction_AssetCantBePaused, common.ErrAssetTriggerInvalid
		}

		asset.Attributes.IsPaused = true
	case transaction.AssetTriggerContract_Resume:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		asset.Attributes.IsPaused = false
	case transaction.AssetTriggerContract_ChangeOwner:
		if !asset.Properties.CanChangeOwner {
			return transaction.Transaction_AssetOwnerCantBeChanged, common.ErrAssetTriggerInvalid
		}

		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
		}

		asset.OwnerAddress = tc.GetToAddress()
	case transaction.AssetTriggerContract_AddRole:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !asset.GetProperties().GetCanAddRoles() {
			return transaction.Transaction_AssetCantBeBurned, common.ErrAssetTriggerInvalid
		}

		if len(tc.GetRole().GetAddress()) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
		}

		updated := false
		for i, role := range asset.Roles {
			if bytes.Equal(role.Address, tc.GetRole().GetAddress()) {
				roleInfo := tc.GetRole()
				asset.Roles[i] = &kapps.RolesData{
					Address:             roleInfo.Address,
					HasRoleMint:         roleInfo.HasRoleMint,
					HasRoleSetITOPrices: roleInfo.HasRoleSetITOPrices,
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
			})
		}
	case transaction.AssetTriggerContract_RemoveRole:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		newRoles := make([]*kapps.RolesData, 0)

		for _, role := range asset.Roles {
			if !bytes.Equal(role.Address, tc.GetToAddress()) {
				newRoles = append(newRoles, role)
			}
		}

		asset.Roles = newRoles
	case transaction.AssetTriggerContract_UpdateMetadata:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !k.TokeTypeHasNonce(asset.AssetType) {
			return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
		}

		if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
		}

		if asset.AssetType == kapps.KDAData_SemiFungible {
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

			k.KAppController.GetCurrentKAppContext().Receipts().Add(txProcess.NewReceipt(
				txProcess.UpdateKDA,
				k.KAppController.GetCurrentKAppContext().ContractID(),
				kdautils.ToKDAKeyWithouPrefix(assetID[0], assetID[1]),
			))

			return transaction.Transaction_Ok, nil
		}

		toAcc, err := k.GetExistingUserAccount(tc.GetToAddress())
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

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

	case transaction.AssetTriggerContract_StopNFTMint:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if asset.Attributes.IsNFTMintStopped {
			return transaction.Transaction_NFTMintStopped, common.ErrAssetTriggerInvalid
		}

		asset.Attributes.IsNFTMintStopped = true
	case transaction.AssetTriggerContract_StopRoyaltiesChange:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if asset.Attributes.IsRoyaltiesChangeStopped {
			return transaction.Transaction_RoyaltiesChangeStopped, common.ErrAssetTriggerInvalid
		}

		asset.Attributes.IsRoyaltiesChangeStopped = true
	case transaction.AssetTriggerContract_UpdateLogo:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		asset.Logo = tc.GetLogo()
	case transaction.AssetTriggerContract_UpdateURIs:
		if !bytes.Equal(asset.OwnerAddress, sender) {
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
	case transaction.AssetTriggerContract_ChangeRoyaltiesReceiver:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if len(tc.GetToAddress()) != k.pubkeyConv.Len() {
			return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
		}

		asset.Royalties.Address = tc.GetToAddress()
	case transaction.AssetTriggerContract_UpdateStaking:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetStaking() == nil {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if k.TokeTypeHasNonce(asset.AssetType) {
			return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
		}

		stakingKapp, staking, err := k.GetStaking(assetID[0])
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		if staking.InterestType != kapps.StakingData_EnumInterestType(tc.GetStaking().GetType()) {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		staking.MinEpochsToClaim = tc.GetStaking().GetMinEpochsToClaim()
		staking.MinEpochsToUnstake = tc.GetStaking().GetMinEpochsToUnstake()
		staking.MinEpochsToWithdraw = tc.GetStaking().GetMinEpochsToWithdraw()

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

		err = k.SetStaking(stakingKapp, assetID[0], staking)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		if err := k.accountsCacher.UpdateKapp(stakingKapp); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}
	case transaction.AssetTriggerContract_UpdateRoyalties:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if asset.Attributes.IsRoyaltiesChangeStopped {
			return transaction.Transaction_RoyaltiesChangeStopped, common.ErrAssetTriggerInvalid
		}

		if tc.GetRoyalties() == nil {
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

		switch asset.AssetType {
		case kapps.KDAData_NonFungible,
			kapps.KDAData_SemiFungible:
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

		case kapps.KDAData_Fungible:

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

			if !pkda.CheckValid100Params(sumSplitTransferPercentage,
				sumSplitTransferFixed, sumSplitITOPercentage, sumSplitITOFixed) {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}

			if !pkda.CheckValid100Params(tc.GetRoyalties().GetMarketPercentage(),
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

			asset.Royalties.TransferFixed = tc.GetRoyalties().GetTransferFixed()
			asset.Royalties.MarketFixed = tc.GetRoyalties().GetMarketFixed()
			asset.Royalties.MarketPercentage = tc.GetRoyalties().GetMarketPercentage()
			asset.Royalties.ITOFixed = tc.GetRoyalties().GetITOFixed()
			asset.Royalties.ITOPercentage = tc.GetRoyalties().GetITOPercentage()
			asset.Royalties.SplitRoyalties = splitRoyalties
		default:
			return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
		}
	case transaction.AssetTriggerContract_UpdateKDAFeePool:
		if !k.forkController.FPRComputeAndKdaFeeFlow() && !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if tc.GetKDAPool() == nil {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		if k.TokeTypeHasNonce(asset.AssetType) || len(assetID) > 2 {
			return transaction.Transaction_AssetError, common.ErrInvalidValue
		}

		resultCode, err := k.KAppController.GetKDAFeesPoolKApp().UpdatePool(asset.ID, asset.OwnerAddress, sender, tc.KDAPool)
		if err != nil {
			return resultCode, err
		}

		// add receipt
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.UpdateKDAPool,
			ctx.ContractID(),
			assetID[0],
		))
	case transaction.AssetTriggerContract_StopNFTMetadataChange:
		if !bytes.Equal(asset.OwnerAddress, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		if asset.Attributes.IsNFTMetadataChangeStopped {
			return transaction.Transaction_NFTMetadataChangeStopped, common.ErrAssetTriggerInvalid
		}

		asset.Attributes.IsNFTMetadataChangeStopped = true
	default:
		return transaction.Transaction_AssetError, common.ErrAssetTriggerInvalid
	}

	err = k.SetKDA(kdaKapp, assetID[0], asset)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if err := k.accountsCacher.UpdateKapp(kdaKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}
