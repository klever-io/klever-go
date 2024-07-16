package transaction

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"google.golang.org/protobuf/proto"
)

type contractValidate interface {
	Validate() error
}

func (t *Transaction) Validate() error {
	for _, tc := range t.GetRawData().Contract {
		if err := tc.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (t *TXContract) Validate() error {
	var (
		tc  contractValidate
		err error
	)

	switch t.Type {
	case TXContract_TransferContractType:
		tc, err = t.GetTransferContract()
	case TXContract_CreateAssetContractType:
		tc, err = t.GetCreateAssetContract()
	case TXContract_CreateValidatorContractType:
		tc, err = t.GetCreateValidatorContract()
	case TXContract_ValidatorConfigContractType:
		tc, err = t.GetValidatorConfigContract()
	case TXContract_FreezeContractType:
		tc, err = t.GetFreezeContract()
	case TXContract_UnfreezeContractType:
		tc, err = t.GetUnfreezeContract()
	case TXContract_DelegateContractType:
		tc, err = t.GetDelegateContract()
	case TXContract_UndelegateContractType:
		tc, err = t.GetUndelegateContract()
	case TXContract_WithdrawContractType:
		tc, err = t.GetWithdrawContract()
	case TXContract_ClaimContractType:
		tc, err = t.GetClaimContract()
	case TXContract_UnjailContractType:
		tc, err = t.GetUnjailContract()
	case TXContract_AssetTriggerContractType:
		tc, err = t.GetAssetTriggerContract()
	case TXContract_SetAccountNameContractType:
		tc, err = t.GetSetAccountNameContract()
	case TXContract_ProposalContractType:
		tc, err = t.GetProposalContract()
	case TXContract_VoteContractType:
		tc, err = t.GetVoteContract()
	case TXContract_ConfigITOContractType:
		tc, err = t.GetConfigITOContract()
	case TXContract_SetITOPricesContractType:
		tc, err = t.GetSetITOPricesContract()
	case TXContract_BuyContractType:
		tc, err = t.GetBuyContract()
	case TXContract_SellContractType:
		tc, err = t.GetSellContract()
	case TXContract_CancelMarketOrderContractType:
		tc, err = t.GetCancelMarketOrderContract()
	case TXContract_CreateMarketplaceContractType:
		tc, err = t.GetCreateMarketplaceContract()
	case TXContract_ConfigMarketplaceContractType:
		tc, err = t.GetConfigMarketplaceContract()
	case TXContract_UpdateAccountPermissionContractType:
		tc, err = t.GetUpdateAccountPermissionContract()
	case TXContract_DepositContractType:
		tc, err = t.GetDepositContract()
	case TXContract_ITOTriggerContractType:
		tc, err = t.GetITOTriggerContract()
	case TXContract_SmartContractType:
		tc, err = t.GetSmartContract()
	default:
		err = errors.New("invalid transaction type")
	}
	if err != nil {
		return err
	}
	return tc.Validate()
}

func (tc *TransferContract) Validate() error {
	if bytes.Equal(tc.ToAddress, core.ZeroAddress) {
		return errors.New("invalid receiver address")
	}

	return nil
}

func (tc *CreateAssetContract) Validate() error {
	// During the initial transaction validation receipt, we do not have access to the forkController to determine
	// whether the fork is active. Therefore, we continue to use the deprecated
	// IsAssetNameHumanReadableOld function for backward compatibility.
	// In the transaction processing step, the white space character restriction will be applied if the fork is active.
	if !kdautils.IsAssetNameHumanReadableOld(tc.GetName()) {
		return errors.New("invalid name")
	}

	if !kdautils.IsTickerValid(tc.GetTicker()) {
		return errors.New("invalid ticker")
	}

	if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
		return errors.New("invalid logo")
	}

	if tc.GetRoyalties() != nil && len(tc.GetRoyalties().GetTransferPercentage()) > core.MaxTransferRoyalties {
		return errors.New("invalid transfer royalties")
	}

	if len(tc.GetRoles()) > core.MaxRoles {
		return errors.New("invalid transfer royalties")
	}

	if len(tc.GetURIs()) > core.MaxURIMapSize {
		return errors.New("invalid uri")
	}

	for key, uri := range tc.GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return errors.New("invalid uri")
		}
	}

	switch tc.GetType() {
	case CreateAssetContract_Fungible:
		if tc.GetPrecision() < core.MinNumberOfDecimals ||
			tc.GetPrecision() > core.MaxNumberOfDecimals {
			return errors.New("invalid precision")
		}
	case CreateAssetContract_NonFungible:
		if !kdautils.IsNFTContractValid(tc.GetInitialSupply(), tc.GetPrecision()) {
			return errors.New("invalid parameter")
		}
	case CreateAssetContract_SemiFungible:
		if tc.GetPrecision() < core.MinNumberOfDecimals ||
			tc.GetPrecision() > core.MaxNumberOfDecimals {
			return errors.New("invalid precision")
		}
	default:
		return errors.New("invalid asset type")
	}

	return nil
}

type allowedAssetTriggerFields struct {
	ToAddress bool
	Amount    bool
	Value     bool
	MIME      bool
	Logo      bool
	URIs      bool
	Royalties bool
	Role      bool
	Staking   bool
	KDAPool   bool
}

func validateAssetTriggerFields(tc *AssetTriggerContract, allow allowedAssetTriggerFields) bool {
	if !allow.ToAddress && len(tc.GetToAddress()) > 0 {
		return false
	}

	if !allow.Amount && tc.Amount > 0 {
		return false
	}

	if !allow.MIME && len(tc.GetMIME()) > 0 {
		return false
	}

	if !allow.Logo && len(tc.GetLogo()) > 0 {
		return false
	}

	if !allow.URIs && len(tc.GetURIs()) > 0 {
		return false
	}

	if allow.Royalties {
		return validateRoyalties(tc.GetRoyalties())
	} else if tc.GetRoyalties() != nil && !proto.Equal(tc.GetRoyalties(), &RoyaltiesInfo{}) {
		return false
	}

	if !allow.Role && tc.GetRole() != nil && !proto.Equal(tc.GetRole(), &RolesInfo{}) {
		return false
	}

	if !allow.Staking && tc.GetStaking() != nil && !proto.Equal(tc.GetStaking(), &StakingInfo{}) {
		return false
	}

	if allow.KDAPool {
		return validateKDAPool(tc.GetKDAPool())
	} else if tc.GetKDAPool() != nil && !proto.Equal(tc.GetKDAPool(), &KDAPoolInfo{}) {
		return false
	}

	return true
}

func validateKDAPool(poolInfo *KDAPoolInfo) bool {
	if poolInfo == nil {
		return false
	}

	if poolInfo.FRatioKLV == 0 || poolInfo.FRatioKDA == 0 {
		return false
	}

	return true
}

func validateRoyalties(royalties *RoyaltiesInfo) bool {
	return royalties != nil
}

func (tc *AssetTriggerContract) Validate() error {
	var allowed allowedAssetTriggerFields

	switch tc.TriggerType {
	case AssetTriggerContract_Pause,
		AssetTriggerContract_Resume,
		AssetTriggerContract_StopNFTMint,
		AssetTriggerContract_StopNFTMetadataChange,
		AssetTriggerContract_StopRoyaltiesChange:
		allowed = allowedAssetTriggerFields{}
	case AssetTriggerContract_Mint:
		allowed = allowedAssetTriggerFields{ToAddress: true, Amount: true, Value: true}
	case AssetTriggerContract_Wipe:
		allowed = allowedAssetTriggerFields{ToAddress: true, Amount: true}
	case AssetTriggerContract_Burn:
		allowed = allowedAssetTriggerFields{Amount: true}
	case AssetTriggerContract_ChangeOwner,
		AssetTriggerContract_ChangeAdmin,
		AssetTriggerContract_RemoveRole,
		AssetTriggerContract_ChangeRoyaltiesReceiver:
		allowed = allowedAssetTriggerFields{ToAddress: true}
	case AssetTriggerContract_UpdateMetadata:
		allowed = allowedAssetTriggerFields{ToAddress: true, MIME: true}
	case AssetTriggerContract_UpdateLogo:
		if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
			return errors.New("invalid logo")
		}

		allowed = allowedAssetTriggerFields{Logo: true}
	case AssetTriggerContract_UpdateURIs:
		if len(tc.GetURIs()) > core.MaxURIMapSize {
			return errors.New("invalid uri")
		}

		for key, uri := range tc.GetURIs() {
			if !utf8.ValidString(key) ||
				!utf8.ValidString(uri) ||
				len(key) > core.MaxURIKeySize ||
				len(uri) > core.MaxURIValueSize {
				return errors.New("invalid uri")
			}
		}

		allowed = allowedAssetTriggerFields{URIs: true}
	case AssetTriggerContract_UpdateStaking:
		allowed = allowedAssetTriggerFields{Staking: true}
	case AssetTriggerContract_AddRole:
		allowed = allowedAssetTriggerFields{Role: true}
	case AssetTriggerContract_UpdateRoyalties:
		allowed = allowedAssetTriggerFields{Royalties: true}
	case AssetTriggerContract_UpdateKDAFeePool:
		allowed = allowedAssetTriggerFields{KDAPool: true}
	default:
		return errors.New("invalid trigger type")
	}

	if !validateAssetTriggerFields(tc, allowed) {
		return fmt.Errorf("invalid contract fields, allowed fields: assetID:true + %+v", allowed)
	}

	return nil
}

func (tc *CreateValidatorContract) Validate() error {
	if tc.GetConfig() == nil {
		return errors.New("invalid config")
	}

	if !utf8.ValidString(tc.GetConfig().GetName()) ||
		len(tc.GetConfig().GetName()) > core.MaxNameSize {
		return errors.New("invalid name")
	}

	if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
		return errors.New("invalid logo")

	}

	if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
		return errors.New("invalid uri")
	}

	for key, uri := range tc.GetConfig().GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return errors.New("invalid uri")
		}
	}

	return nil
}

func (tc *ValidatorConfigContract) Validate() error {
	if tc.GetConfig() == nil {
		return errors.New("invalid config")
	}

	if !utf8.ValidString(tc.GetConfig().GetName()) ||
		len(tc.GetConfig().GetName()) > core.MaxNameSize {
		return errors.New("invalid name")
	}

	if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
		return errors.New("invalid logo")

	}

	if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
		return errors.New("invalid uri")
	}

	for key, uri := range tc.GetConfig().GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return errors.New("invalid uri")
		}
	}

	return nil
}

func (tc *FreezeContract) Validate() error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < 3 {
		return errors.New("invalid asset id")
	}

	if tc.Amount <= 0 {
		return errors.New("invalid amount")
	}

	return nil
}

func (tc *WithdrawContract) Validate() error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < 3 {
		return errors.New("invalid asset id")
	}

	// check types
	switch tc.WithdrawType {
	case WithdrawContract_Staking:
		if len(tc.CurrencyID) != 0 ||
			tc.Amount != 0 {
			return errors.New("invalid values")
		}
	case WithdrawContract_KDAPool:
		if len(tc.CurrencyID) < 3 {
			return errors.New("invalid currency id")
		}

		if tc.Amount <= 0 {
			return errors.New("invalid amount")
		}
	default:
		return errors.New("invalid withdraw type")
	}

	return nil
}

func (tc *DepositContract) Validate() error {
	// check types
	switch tc.DepositType {
	case DepositContract_FPRDeposit,
		DepositContract_KDAPool:
		if len(tc.ID) < 3 {
			return errors.New("invalid id")
		}

		if len(tc.CurrencyID) < 3 {
			return errors.New("invalid currency id")
		}

		if tc.Amount <= 0 {
			return errors.New("invalid amount")
		}

		return nil
	}

	return errors.New("invalid deposit type")
}

func (tc *UnfreezeContract) Validate() error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < 3 {
		return errors.New("invalid asset id")
	}

	if len(tc.BucketID) != 0 {
		return errors.New("invalid bucket id")
	}

	return nil
}

func (tc *DelegateContract) Validate() error {
	if len(tc.ToAddress) == 0 {
		return errors.New("invalid receiver address")
	}

	if len(tc.BucketID) == 0 {
		return errors.New("invalid bucket id")
	}

	return nil
}

func (tc *UndelegateContract) Validate() error {
	if len(tc.BucketID) == 0 {
		return errors.New("invalid bucket id")
	}

	return nil
}

func (tc *ClaimContract) Validate() error {
	if tc.ClaimType == ClaimContract_AllowanceClaim {
		if tc.ID != nil &&
			!bytes.Equal(tc.ID, kdautils.KLVIdentifier) {
			return common.ErrAssetIDInvalid
		}
	}

	return nil
}

func (tc *UnjailContract) Validate() error {
	// Empty validation because the contract does not have any fields
	return nil
}

func (tc *SetAccountNameContract) Validate() error {
	if !utf8.Valid(tc.GetName()) ||
		len(tc.GetName()) > core.MaxNameSize {
		return errors.New("invalid name")
	}

	return nil
}

func (tc *ProposalContract) Validate() error {
	if len(tc.GetParameters()) > core.MaxProposalsLength {
		return errors.New("invalid parameters")
	}

	if len(tc.GetDescription()) > core.MaxDescriptionLength {
		return errors.New("invalid description")
	}

	for _, parameter := range tc.GetParameters() {
		if !utf8.Valid([]byte(parameter)) ||
			len(parameter) > core.MaxProposalParamLength {
			return errors.New("invalid parameter")
		}
	}

	return nil
}

func (tc *VoteContract) Validate() error {
	if tc.GetProposalID() <= 0 {
		return errors.New("invalid proposal id")
	}

	if tc.GetAmount() <= 0 {
		return errors.New("invalid amount")
	}

	return nil
}

func (tc *ConfigITOContract) Validate() error {
	if len(tc.GetPackInfo()) > 0 {
		if len(tc.GetPackInfo()) > core.MaxPacks {
			return errors.New("invalid packs size")
		}

		for _, packInfo := range tc.GetPackInfo() {
			if len(packInfo.GetPacks()) > core.MaxPackItems {
				return errors.New("invalid pack items size")
			}
		}
	}

	return nil
}

func (tc *SetITOPricesContract) Validate() error {
	if len(tc.GetPackInfo()) > 0 {
		if len(tc.GetPackInfo()) > core.MaxPacks {
			return errors.New("invalid packs size")
		}

		for _, packInfo := range tc.GetPackInfo() {
			if len(packInfo.GetPacks()) > core.MaxPackItems {
				return errors.New("invalid pack items size")
			}
		}
	}

	return nil
}

func (tc *BuyContract) Validate() error {
	if len(tc.GetID()) == 0 {
		return errors.New("invalid pack id")
	}

	if tc.GetAmount() <= 0 {
		return errors.New("invalid amount")
	}

	return nil
}

func (tc *SellContract) Validate() error {
	if len(tc.MarketplaceID) == 0 {
		return errors.New("invalid marketplace id")
	}

	if len(tc.AssetID) == 0 {
		return errors.New("invalid asset id")
	}

	if tc.Price < 0 {
		return errors.New("invalid price")
	}

	if tc.ReservePrice < 0 {
		return errors.New("invalid reserve price")
	}

	if tc.EndTime <= 0 {
		return errors.New("invalid end time")
	}

	return nil
}

func (tc *CancelMarketOrderContract) Validate() error {
	if len(tc.GetOrderID()) == 0 {
		return errors.New("invalid order id")
	}

	return nil
}

func (tc *CreateMarketplaceContract) Validate() error {
	if tc.GetName() == nil ||
		!utf8.Valid([]byte(tc.GetName())) ||
		len(tc.GetName()) > core.MaxNameSize {
		return errors.New("invalid name")
	}

	return nil
}

func (tc *ConfigMarketplaceContract) Validate() error {
	if !utf8.Valid([]byte(tc.GetName())) ||
		len(tc.GetName()) > core.MaxNameSize {
		return errors.New("invalid name")
	}
	return nil
}

func (tc *UpdateAccountPermissionContract) Validate() error {
	if len(tc.GetPermissions()) > core.MaxAccountPermission {
		return errors.New("invalid permission size")
	}

	for _, perm := range tc.GetPermissions() {
		if len(perm.GetSigners()) > core.MaxPermissionSigners {
			return errors.New("invalid permission size")
		}

		if !utf8.Valid([]byte(perm.PermissionName)) ||
			len(perm.PermissionName) > core.MaxNameSize {
			return errors.New("invalid permission name")
		}

		if len(perm.Operations) > core.MaxOperationsSize {
			return errors.New("invalid permission operation")
		}
	}

	return nil
}

type allowedITOTriggerFields struct {
	ReceiverAddress        bool
	Status                 bool
	MaxAmount              bool
	PackInfo               bool
	DefaultLimitPerAddress bool
	WhitelistStatus        bool
	WhitelistInfo          bool
	WhitelistStartTime     bool
	WhitelistEndTime       bool
	StartTime              bool
	EndTime                bool
}

func validateITOTriggerFields(tc *ITOTriggerContract, allow allowedITOTriggerFields) bool {
	if !allow.ReceiverAddress && len(tc.GetReceiverAddress()) > 0 {
		return false
	}

	if !allow.Status && tc.GetStatus() > 0 {
		return false
	}

	if !allow.MaxAmount && tc.GetMaxAmount() > 0 {
		return false
	}

	if !allow.PackInfo && len(tc.GetPackInfo()) > 0 {
		return false
	}

	if !allow.DefaultLimitPerAddress && tc.GetDefaultLimitPerAddress() > 0 {
		return false
	}

	if !allow.WhitelistStatus && tc.GetWhitelistStatus() > 0 {
		return false
	}

	if !allow.WhitelistInfo && len(tc.GetWhitelistInfo()) > 0 {
		return false
	}

	if !allow.WhitelistStartTime && tc.GetWhitelistStartTime() > 0 {
		return false
	}

	if !allow.WhitelistEndTime && tc.GetWhitelistEndTime() > 0 {
		return false
	}

	if !allow.StartTime && tc.GetStartTime() > 0 {
		return false
	}

	if !allow.EndTime && tc.GetEndTime() > 0 {
		return false
	}

	return true
}

func (tc *ITOTriggerContract) Validate() error {
	if tc.GetAssetID() == nil {
		return errors.New("invalid assetID")
	}

	var allowed allowedITOTriggerFields

	switch tc.TriggerType {
	case ITOTriggerContract_SetITOPrices:
		allowed = allowedITOTriggerFields{PackInfo: true}
	case ITOTriggerContract_UpdateStatus:
		allowed = allowedITOTriggerFields{Status: true}
	case ITOTriggerContract_UpdateReceiverAddress:
		allowed = allowedITOTriggerFields{ReceiverAddress: true}
	case ITOTriggerContract_UpdateMaxAmount:
		allowed = allowedITOTriggerFields{MaxAmount: true}
	case ITOTriggerContract_UpdateDefaultLimitPerAddress:
		allowed = allowedITOTriggerFields{DefaultLimitPerAddress: true}
	case ITOTriggerContract_UpdateTimes:
		allowed = allowedITOTriggerFields{StartTime: true, EndTime: true}
	case ITOTriggerContract_UpdateWhitelistStatus:
		allowed = allowedITOTriggerFields{WhitelistStatus: true}
	case ITOTriggerContract_AddToWhitelist:
		allowed = allowedITOTriggerFields{WhitelistInfo: true}
	case ITOTriggerContract_RemoveFromWhitelist:
		allowed = allowedITOTriggerFields{WhitelistInfo: true}
	case ITOTriggerContract_UpdateWhitelistTimes:
		allowed = allowedITOTriggerFields{WhitelistStartTime: true, WhitelistEndTime: true}
	default:
		return errors.New("invalid trigger type")
	}

	if !validateITOTriggerFields(tc, allowed) {
		return fmt.Errorf("invalid contract fields, allowed fields: assetID:true + %+v", allowed)
	}

	return nil
}

func (tc *SmartContract) Validate() error {
	switch tc.Type {
	case SmartContract_SCDeploy:
		if len(tc.Address) > 0 {
			return errors.New("invalid contract address")
		}
	default:
		if len(tc.Address) != core.PubKeyLen {
			return errors.New("invalid contract address")
		}
	}

	if len(tc.CallValue) > core.MaxCallValueSize {
		return errors.New("invalid call value")
	}

	return nil
}
