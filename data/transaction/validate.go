package transaction

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"google.golang.org/protobuf/proto"
)

// Constants for URL prefixes to improve maintainability and avoid magic strings
const (
	protoURLPrefix = "type.googleapis.com/proto."
	typeURLPrefix  = "type.googleapis.com/"
	typeSuffix     = "Type"
)

type contractValidate interface {
	Validate(fc core.ForkController) error
}

func (t *Transaction) Validate(fc core.ForkController) error {
	for _, tc := range t.GetRawData().Contract {
		if err := tc.Validate(fc); err != nil {
			return err
		}
	}

	return nil
}

func (t *TXContract) Validate(fc core.ForkController) error {
	var (
		tc  contractValidate
		err error
	)

	if t.Parameter == nil {
		return common.ErrInvalidTransactionType
	}

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
		err = common.ErrInvalidTransactionType
	}

	if err != nil {
		return err
	}

	return tc.Validate(fc)
}

func (tc *TransferContract) Validate(fc core.ForkController) error {
	if len(tc.ToAddress) != core.PubKeyLen ||
		bytes.Equal(tc.ToAddress, core.ZeroAddress) {
		return ErrInvalidReceiverAddress
	}

	return nil
}

func (tc *CreateAssetContract) Validate(fc core.ForkController) error {
	isHumanReadable := kdautils.IsAssetNameHumanReadable
	if !fc.EnableSmartContracts() {
		isHumanReadable = kdautils.IsAssetNameHumanReadableOld
	}

	if !isHumanReadable(tc.GetName()) {
		return ErrInvalidName
	}

	if !kdautils.IsTickerValid(tc.GetTicker()) {
		return ErrInvalidTicker
	}

	if !utf8.ValidString(tc.GetLogo()) || len(tc.GetLogo()) > core.MaxLogoURISize {
		return ErrInvalidLogo
	}

	if tc.GetRoyalties() != nil && len(tc.GetRoyalties().GetTransferPercentage()) > core.MaxTransferRoyalties {
		return ErrInvalidTransferRoyaltiesLen
	}

	if tc.GetRoyalties() != nil && len(tc.GetRoyalties().GetSplitRoyalties()) > core.MaxTransferRoyalties {
		return ErrInvalidSplitRoyaltiesLen
	}

	if len(tc.GetRoles()) > core.MaxRoles {
		return ErrInvalidRolesLen
	}

	if len(tc.GetURIs()) > core.MaxURIMapSize {
		return ErrInvalidURI
	}

	if err := validateURI(tc.GetURIs()); err != nil {
		return err
	}

	if err := tc.validateCreateAssetAfterFork(fc); err != nil {
		return err
	}

	return tc.validateByAssetType()
}

func validateAssetID(fk core.ForkController, assetID []byte, err error) error {
	if !fk.EnableSmartContracts() {
		return nil
	}

	if assetID == nil || bytes.Equal(assetID, kdautils.KLVIdentifier) ||
		bytes.Equal(assetID, kdautils.KFIIdentifier) {
		return nil
	}

	if len(assetID) < core.MinLengthForAssetTicker || len(assetID) > core.MaxLengthForAssetID {
		return err
	}

	return nil
}

func (tc *CreateAssetContract) validateCreateAssetAfterFork(fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	if err := validateAddressOrNil(len(tc.GetOwnerAddress()), ErrInvalidOwnerAddress); err != nil {
		return err
	}

	if err := validateAddressOrNil(len(tc.GetAdminAddress()), ErrInvalidAdminAddress); err != nil {
		return err
	}

	return nil
}

func validateURI(uris map[string]string) error {
	for key, uri := range uris {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return ErrInvalidURI
		}
	}

	return nil
}

func (tc *CreateAssetContract) validateByAssetType() error {
	switch tc.GetType() {
	case CreateAssetContract_Fungible, CreateAssetContract_SemiFungible:
		if tc.GetPrecision() < core.MinNumberOfDecimals ||
			tc.GetPrecision() > core.MaxNumberOfDecimals {
			return ErrInvalidPrecision
		}
	case CreateAssetContract_NonFungible:
		if !kdautils.IsNFTContractValid(tc.GetInitialSupply(), tc.GetPrecision()) {
			return ErrInvalidParameter
		}
	default:
		return ErrInvalidAssetType
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
	if royalties == nil {
		return false
	}

	if len(royalties.GetTransferPercentage()) > core.MaxTransferRoyalties ||
		len(royalties.GetSplitRoyalties()) > core.MaxTransferRoyalties {
		return false
	}

	return true
}

func (tc *AssetTriggerContract) Validate(fc core.ForkController) error {
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
			return ErrInvalidLogo
		}

		allowed = allowedAssetTriggerFields{Logo: true}
	case AssetTriggerContract_UpdateURIs:
		if len(tc.GetURIs()) > core.MaxURIMapSize {
			return ErrInvalidURI
		}

		for key, uri := range tc.GetURIs() {
			if !utf8.ValidString(key) ||
				!utf8.ValidString(uri) ||
				len(key) > core.MaxURIKeySize ||
				len(uri) > core.MaxURIValueSize {
				return ErrInvalidURI
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
		return ErrInvalidTriggerType
	}

	if !validateAssetTriggerFields(tc, allowed) {
		return fmt.Errorf("invalid contract fields, allowed fields: assetID:true + %+v", allowed)
	}

	return tc.validateAssetTriggerAfterFork(allowed, fc)
}

func (tc *AssetTriggerContract) validateAssetTriggerAfterFork(allowed allowedAssetTriggerFields, fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	if allowed.ToAddress {
		if err := validateAddressOrNil(len(tc.GetToAddress()), ErrInvalidAddress); err != nil {
			return err
		}
	}

	if allowed.Amount && tc.Amount < 0 {
		return ErrInvalidAmount
	}

	return nil
}

func (tc *CreateValidatorContract) Validate(fc core.ForkController) error {
	if tc.GetConfig() == nil {
		return ErrInvalidConfig
	}

	if !utf8.ValidString(tc.GetConfig().GetName()) ||
		len(tc.GetConfig().GetName()) > core.MaxNameSize {
		return ErrInvalidName
	}

	if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
		return ErrInvalidLogo
	}

	if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
		return ErrInvalidURI
	}

	for key, uri := range tc.GetConfig().GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return ErrInvalidURI
		}
	}

	return nil
}

func (tc *ValidatorConfigContract) Validate(fc core.ForkController) error {
	if tc.GetConfig() == nil {
		return ErrInvalidConfig
	}

	if !utf8.ValidString(tc.GetConfig().GetName()) ||
		len(tc.GetConfig().GetName()) > core.MaxNameSize {
		return ErrInvalidName
	}

	if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
		return ErrInvalidLogo

	}

	if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
		return ErrInvalidURI
	}

	for key, uri := range tc.GetConfig().GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return ErrInvalidURI
		}
	}

	return nil
}

func (tc *FreezeContract) Validate(fc core.ForkController) error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < core.MinLengthForAssetTicker {
		return ErrInvalidAssetID
	}

	if tc.Amount <= 0 {
		return ErrInvalidAmount
	}

	if !fc.EnableSmartContracts() {
		return nil
	}

	if err := validateAssetID(fc, tc.AssetID, ErrInvalidAssetID); err != nil {
		return err
	}

	return nil
}

func (tc *WithdrawContract) validateWithdrawAfterFork(fc core.ForkController) error {
	return validateAssetID(fc, tc.AssetID, ErrInvalidAssetID)
}

func (tc *WithdrawContract) Validate(fc core.ForkController) error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < core.MinLengthForAssetTicker {
		return ErrInvalidAssetID
	}

	// check types
	switch tc.WithdrawType {
	case WithdrawContract_Staking:
		if len(tc.CurrencyID) != 0 ||
			tc.Amount != 0 {
			return ErrInvalidValues
		}
	case WithdrawContract_KDAPool:
		if len(tc.CurrencyID) < core.MinLengthForAssetTicker {
			return ErrInvalidCurrencyID
		}

		if tc.Amount <= 0 {
			return ErrInvalidAmount
		}

		if err := validateAssetID(fc, tc.CurrencyID, ErrInvalidCurrencyID); err != nil {
			return err
		}

	default:
		return ErrInvalidWithdrawType
	}

	return tc.validateWithdrawAfterFork(fc)
}

func (tc *DepositContract) Validate(fc core.ForkController) error {
	// check types
	switch tc.DepositType {
	case DepositContract_FPRDeposit,
		DepositContract_KDAPool:
		if len(tc.ID) < core.MinLengthForAssetTicker {
			return ErrInvalidID
		}

		if len(tc.CurrencyID) < core.MinLengthForAssetTicker {
			return ErrInvalidCurrencyID
		}

		if tc.Amount <= 0 {
			return ErrInvalidAmount
		}

		if err := validateAssetID(fc, tc.CurrencyID, ErrInvalidCurrencyID); err != nil {
			return err
		}

		return nil
	}

	return ErrInvalidDepositType
}

func (tc *UnfreezeContract) validateUnfreezeAfterFork(fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	if err := validateAssetID(fc, tc.AssetID, ErrInvalidAssetID); err != nil {
		return err
	}

	if len(tc.AssetID) > core.MinLengthForAssetTicker && len(tc.BucketID) != len(tc.AssetID) {
		return ErrInvalidBucketID
	}

	if (bytes.Equal(tc.AssetID, kdautils.KLVIdentifier) || bytes.Equal(tc.AssetID, kdautils.KFIIdentifier)) && len(tc.BucketID) != core.BucketIDSize {
		return ErrInvalidBucketID
	}

	return nil
}

func (tc *UnfreezeContract) Validate(fc core.ForkController) error {
	if len(tc.AssetID) != 0 && len(tc.AssetID) < core.MinLengthForAssetTicker {
		return ErrInvalidAssetID
	}

	if len(tc.BucketID) == 0 {
		return ErrInvalidBucketID
	}

	return tc.validateUnfreezeAfterFork(fc)
}

func (tc *DelegateContract) Validate(fc core.ForkController) error {
	if len(tc.ToAddress) == 0 {
		return ErrInvalidReceiverAddress
	}

	if len(tc.BucketID) == 0 {
		return ErrInvalidBucketID
	}

	if fc.EnableSmartContracts() {
		if err := validateAddress(len(tc.ToAddress), ErrInvalidToAddress); err != nil {
			return err
		}
	}

	return nil
}

func (tc *UndelegateContract) Validate(fc core.ForkController) error {
	if len(tc.BucketID) == 0 {
		return ErrInvalidBucketID
	}

	return nil
}

func (tc *ClaimContract) Validate(fc core.ForkController) error {
	if tc.ClaimType == ClaimContract_AllowanceClaim {
		if tc.ID != nil &&
			!bytes.Equal(tc.ID, kdautils.KLVIdentifier) {
			return common.ErrAssetIDInvalid
		}
	}

	return nil
}

func (tc *UnjailContract) Validate(fc core.ForkController) error {
	// Empty validation because the contract does not have any fields
	return nil
}

func (tc *SetAccountNameContract) Validate(fc core.ForkController) error {
	if !utf8.Valid(tc.GetName()) ||
		len(tc.GetName()) > core.MaxNameSize {
		return ErrInvalidName
	}

	return nil
}

func (tc *ProposalContract) Validate(fc core.ForkController) error {
	if len(tc.GetParameters()) > core.MaxProposalsLength {
		return ErrInvalidParameter
	}

	if len(tc.GetDescription()) > core.MaxDescriptionLength {
		return ErrInvalidDescription
	}

	for _, parameter := range tc.GetParameters() {
		if !utf8.Valid([]byte(parameter)) ||
			len(parameter) > core.MaxProposalParamLength {
			return ErrInvalidParameter
		}
	}

	return nil
}

func (tc *VoteContract) Validate(fc core.ForkController) error {
	if fc.EnableSmartContracts() && tc.GetProposalID() <= 0 {
		return ErrInvalidProposalID
	}

	if tc.GetAmount() <= 0 {
		return ErrInvalidAmount
	}

	return nil
}

func (tc *ConfigITOContract) validateConfigITOAfterFork(fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	if len(tc.WhitelistInfo) > core.MaxWhitelistSize {
		return ErrInvalidWhitelistSize
	}

	if err := validateAddressOrNil(len(tc.ReceiverAddress), ErrInvalidReceiverAddress); err != nil {
		return err
	}

	if err := validateAssetID(fc, tc.AssetID, ErrInvalidAssetID); err != nil {
		return err
	}

	return nil
}

func (tc *ConfigITOContract) Validate(fc core.ForkController) error {
	if len(tc.GetPackInfo()) > 0 {
		if len(tc.GetPackInfo()) > core.MaxPacks {
			return ErrInvalidPackSize
		}

		for _, packInfo := range tc.GetPackInfo() {
			if len(packInfo.GetPacks()) > core.MaxPackItems {
				return ErrInvalidPackItemSize
			}
		}
	}

	return tc.validateConfigITOAfterFork(fc)
}

func (tc *SetITOPricesContract) Validate(fc core.ForkController) error {
	if len(tc.GetPackInfo()) > 0 {
		if len(tc.GetPackInfo()) > core.MaxPacks {
			return ErrInvalidPackSize
		}

		for _, packInfo := range tc.GetPackInfo() {
			if len(packInfo.GetPacks()) > core.MaxPackItems {
				return ErrInvalidPackItemSize
			}
		}
	}

	if err := validateAssetID(fc, tc.AssetID, ErrInvalidAssetID); err != nil {
		return err
	}

	return nil
}

func (tc *BuyContract) Validate(fc core.ForkController) error {
	if fc.EnableSmartContracts() {
		if len(tc.GetID()) == 0 {
			return ErrInvalidPackID
		}

		if err := validateAssetID(fc, tc.CurrencyID, ErrInvalidCurrencyID); err != nil {
			return err
		}
	}

	if tc.GetAmount() < 0 {
		return ErrInvalidAmount
	}

	return nil
}

func (tc *SellContract) Validate(fc core.ForkController) error {
	if fc.EnableSmartContracts() {
		if len(tc.MarketplaceID) == 0 {
			return ErrInvalidMarketplaceID
		}

		if err := validateAssetID(fc, tc.AssetID, ErrInvalidAssetID); err != nil {
			return err
		}

		if err := validateAssetID(fc, tc.CurrencyID, ErrInvalidCurrencyID); err != nil {
			return err
		}
	}

	if tc.Price < 0 {
		return ErrInvalidPrice
	}

	if tc.ReservePrice < 0 {
		return ErrInvalidReservePrice
	}

	if tc.EndTime < 0 {
		return ErrInvalidEndTime
	}

	return nil
}

func (tc *CancelMarketOrderContract) Validate(fc core.ForkController) error {
	if fc.EnableSmartContracts() && len(tc.GetOrderID()) == 0 {
		return ErrInvalidOrderID
	}

	return nil
}

func (tc *CreateMarketplaceContract) Validate(fc core.ForkController) error {
	if tc.GetName() == nil ||
		!utf8.Valid([]byte(tc.GetName())) ||
		len(tc.GetName()) > core.MaxNameSize {
		return ErrInvalidName
	}

	if fc.EnableSmartContracts() {
		if err := validateAddressOrNil(len(tc.ReferralAddress), ErrInvalidReferralAddress); err != nil {
			return err
		}
	}

	return nil
}

func (tc *ConfigMarketplaceContract) Validate(fc core.ForkController) error {
	if !utf8.Valid([]byte(tc.GetName())) ||
		len(tc.GetName()) > core.MaxNameSize {
		return ErrInvalidName
	}

	if fc.EnableSmartContracts() {
		if err := validateAddressOrNil(len(tc.ReferralAddress), ErrInvalidReferralAddress); err != nil {
			return err
		}
	}

	return nil
}

func (tc *UpdateAccountPermissionContract) validateUpdateAccountPermissionAfterFork(perm *AccPermission, fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	for _, op := range perm.Signers {
		if err := validateAddress(len(op.Address), ErrInvalidSignerAddress); err != nil {
			return err
		}
	}

	return nil
}

func (tc *UpdateAccountPermissionContract) validateUpdateAccountPermissions(fc core.ForkController) error {
	for _, perm := range tc.GetPermissions() {
		if len(perm.GetSigners()) > core.MaxPermissionSigners {
			return ErrInvalidPermissionSize
		}

		if !utf8.Valid([]byte(perm.PermissionName)) ||
			len(perm.PermissionName) > core.MaxNameSize {
			return ErrInvalidPermissionName
		}

		if len(perm.Operations) > core.MaxOperationsSize {
			return ErrInvalidPermissionOperation
		}

		if err := tc.validateUpdateAccountPermissionAfterFork(perm, fc); err != nil {
			return err
		}
	}

	return nil
}

func (tc *UpdateAccountPermissionContract) Validate(fc core.ForkController) error {
	if len(tc.GetPermissions()) > core.MaxAccountPermission {
		return ErrInvalidPermissionSize
	}

	if err := tc.validateUpdateAccountPermissions(fc); err != nil {
		return err
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

func validateAddressOrNil(addressLen int, err error) error {
	if addressLen == 0 {
		return nil
	}

	return validateAddress(addressLen, err)
}

func validateAddress(addressLen int, err error) error {

	if addressLen != core.PubKeyLen {
		return err
	}

	return nil
}

func (tc *ITOTriggerContract) validateITOTriggerAfterFork(fc core.ForkController) error {
	if !fc.EnableSmartContracts() {
		return nil
	}

	if err := validateAssetID(fc, tc.GetAssetID(), ErrInvalidAssetID); err != nil {
		return err
	}

	if err := validateAddressOrNil(len(tc.ReceiverAddress), ErrInvalidReceiverAddress); err != nil {
		return err
	}

	if len(tc.WhitelistInfo) > core.MaxWhitelistSize {
		return ErrInvalidWhitelistSize
	}

	for address := range tc.GetWhitelistInfo() {
		// convert address to byte array
		addrBytes, err := hex.DecodeString(address)
		if err != nil {
			return ErrInvalidWhitelistAddr
		}

		if err := validateAddress(len(addrBytes), ErrInvalidWhitelistAddr); err != nil {
			return err
		}
	}

	return nil
}

func (tc *ITOTriggerContract) Validate(fc core.ForkController) error {
	if tc.GetAssetID() == nil {
		return ErrInvalidAssetIDPreFork
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
		return ErrInvalidTriggerType
	}

	if !validateITOTriggerFields(tc, allowed) {
		return fmt.Errorf("invalid contract fields, allowed fields: assetID:true + %+v", allowed)
	}

	return tc.validateITOTriggerAfterFork(fc)
}

func (tc *SmartContract) Validate(fc core.ForkController) error {
	switch tc.Type {
	case SmartContract_SCDeploy:
		if len(tc.Address) > 0 {
			return ErrInvalidContractAddress
		}
	default:
		if len(tc.Address) != core.PubKeyLen {
			return ErrInvalidContractAddress
		}
	}

	if len(tc.CallValue) > core.MaxCallValueSize {
		return ErrInvalidCallValue
	}

	return nil
}

func IsContractSizeValid(contract []byte, contractType TXContract_ContractType) bool {
	contractSize, ok := ContractMaxSizes[contractType]
	if !ok {
		return false
	}

	return len(contract) <= contractSize
}

// getCanonicalName returns the canonical name for a contract type,
func getCanonicalName(contractType TXContract_ContractType) string {
	// Compute canonical name
	contractName := TXContract_ContractType_name[int32(contractType)] // #nosec G115
	canonicalName := strings.TrimSuffix(contractName, typeSuffix)

	return canonicalName
}

// cleanTypeURL removes known prefixes from the type URL
func cleanTypeURL(typeURL string) string {
	// Remove prefixes in order of specificity
	typeURL = strings.Replace(typeURL, protoURLPrefix, "", 1)
	typeURL = strings.Replace(typeURL, typeURLPrefix, "", 1)
	return typeURL
}

// IsValidTypeURL checks if the typeURL is valid for the contract type
// It returns true if the typeURL is empty or matches the contract type (prefix excluded)
func IsValidTypeURL(typeURL string, contractType TXContract_ContractType) bool {
	// Empty URLs are considered valid
	if len(typeURL) == 0 {
		return true
	}

	// Get canonical name from cache or compute it
	canonicalName := getCanonicalName(contractType)

	// Clean the provided typeURL by removing prefixes
	cleanURL := cleanTypeURL(typeURL)

	return cleanURL == canonicalName
}
