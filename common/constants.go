package common

import "time"

// DefaultDirPermission represents the default directory permissions
const DefaultDirPermission = 0750

// FileModeUserReadWrite represents the permission for a file which allows the user for reading and writing
const FileModeUserReadWrite = 0600

// BaseOperationCost represents the field name for base operation costs
const BaseOperationCost = "BaseOperationCost"

// BuiltInCost represents the field name for built-in operation costs
const BuiltInCost = "BuiltInCost"

// BaseOpsAPICost represents the field name of the SC API (EEI) gas costs
const BaseOpsAPICost = "BaseOpsAPICost"

// MaxPerTransaction represents the field name of max counts per transaction in block chain hook
const MaxPerTransaction = "MaxPerTransaction"

const EmptyJSonMarshalData = "{}"

// Error receipt field constants for consistent error tracking in transaction receipts.
// These are used as the first parameter in ctx.Receipts().AddError(contractID, field, reason)
// to identify which field/aspect of a transaction caused the error.
const (
	// Account/Transfer errors
	ErrFieldAddressNotPayable     = "AddressNotPayable"
	ErrFieldAssetPaused           = "AssetPaused"
	ErrFieldTransferNotAllowed    = "TransferNotAllowed"
	ErrFieldUninitializedContract = "UninitializedContract"
	ErrFieldInvalidAddress        = "InvalidAddress"
	ErrFieldInvalidToAddress      = "InvalidToAddress"
	ErrFieldSameAddress           = "SameAddress"
	ErrFieldInvalidAmount         = "InvalidAmount"
	ErrFieldInsufficientFunds     = "InsufficientFunds"
	ErrFieldLoadSenderAccount     = "LoadSenderAccount"
	ErrFieldLoadReceiverAccount   = "LoadReceiverAccount"
	ErrFieldKAppError             = "KAppError"
	ErrFieldBalanceError          = "BalanceError"
	ErrFieldFreezeError           = "FreezeError"
	ErrFieldUnfreezeError         = "UnfreezeError"
	ErrFieldDelegateError         = "DelegateError"
	ErrFieldUndelegateError       = "UndelegateError"
	ErrFieldSaveAccountError      = "SaveAccountError"
	ErrFieldStakingError          = "StakingError"
	ErrFieldClaimError            = "ClaimError"
	ErrFieldMaxBucketsExceeded    = "MaxBucketsExceeded"
	ErrFieldInvalidAssetType      = "InvalidAssetType"
	ErrFieldBucketNotFound        = "BucketNotFound"
	ErrFieldInvalidBucketID       = "InvalidBucketID"
	ErrFieldWithdrawNotAvailable  = "WithdrawNotAvailable"
	ErrFieldClaimNotAvailable     = "ClaimNotAvailable"
	ErrFieldNoValidClaims         = "NoValidClaims"
	ErrFieldInvalidName           = "InvalidName"
	ErrFieldInvalidPermission     = "InvalidPermission"
	ErrFieldInvalidPermissionSigs = "InvalidPermissionSigners"

	// Validator errors
	ErrFieldInvalidOwnerAddress  = "InvalidOwnerAddress"
	ErrFieldInvalidRewardAddress = "InvalidRewardAddress"
	ErrFieldInvalidCommission    = "InvalidCommission"
	ErrFieldInvalidDelegation    = "InvalidDelegation"
	ErrFieldInvalidBLSKey        = "InvalidBLSKey"
	ErrFieldBLSKeyAlreadyUsed    = "BLSKeyAlreadyUsed"
	ErrFieldBLSKeyRevoked        = "BLSKeyRevoked"
	ErrFieldBLSKeyNotOwned       = "BLSKeyNotOwned"
	ErrFieldValidatorAlreadySet  = "ValidatorAlreadySet"
	ErrFieldValidatorNotFound    = "ValidatorNotFound"
	ErrFieldValidatorJailed      = "ValidatorJailed"
	ErrFieldUnjailNotAvailable   = "UnjailNotAvailable"
	ErrFieldInvalidURI           = "InvalidURI"
	ErrFieldURICountExceeded     = "URICountExceeded"
	ErrFieldInvalidLogo          = "InvalidLogo"

	// Proposal errors
	ErrFieldInvalidProposal       = "InvalidProposal"
	ErrFieldInvalidProposalParams = "InvalidProposalParams"
	ErrFieldProposalNotActive     = "ProposalNotActive"
	ErrFieldProposalNotFound      = "ProposalNotFound"
	ErrFieldInvalidVote           = "InvalidVote"
	ErrFieldInvalidVoteType       = "InvalidVoteType"
	ErrFieldInvalidVoteAmount     = "InvalidVoteAmount"
	ErrFieldMinKFIStakedUnreached = "MinKFIStakedUnreached"
	ErrFieldInsufficientFrozenKFI = "InsufficientFrozenKFI"
	ErrFieldProposalMaxVoters     = "ProposalMaxVoters"

	ErrFieldScriptAlreadyProposed = "ScriptAlreadyProposed"

	// KDA Asset errors
	ErrFieldInvalidAssetParams       = "InvalidAssetParams"
	ErrFieldInvalidPrecision         = "InvalidPrecision"
	ErrFieldInvalidSupply            = "InvalidSupply"
	ErrFieldInvalidRoleAddress       = "InvalidRoleAddress"
	ErrFieldInvalidStakingType       = "InvalidStakingType"
	ErrFieldInvalidSplitRoyalties    = "InvalidSplitRoyalties"
	ErrFieldAssetCannotMint          = "AssetCannotMint"
	ErrFieldAssetCannotBurn          = "AssetCannotBurn"
	ErrFieldAssetCannotWipe          = "AssetCannotWipe"
	ErrFieldAssetCannotPause         = "AssetCannotPause"
	ErrFieldAssetCannotFreeze        = "AssetCannotFreeze"
	ErrFieldAssetMintStopped         = "AssetMintStopped"
	ErrFieldAssetOwnerCantBeChanged  = "AssetOwnerCantBeChanged"
	ErrFieldAssetCantAddRoles        = "AssetCantAddRoles"
	ErrFieldRoleLimitReached         = "RoleLimitReached"
	ErrFieldRoyaltiesChangeStopped   = "RoyaltiesChangeStopped"
	ErrFieldNFTMintStopped           = "NFTMintStopped"
	ErrFieldNFTMetadataChangeStopped = "NFTMetadataChangeStopped"
	ErrFieldInvalidTriggerType       = "InvalidTriggerType"
	ErrFieldInvalidAssetID           = "InvalidAssetID"
	ErrFieldMissingDepositRole       = "MissingDepositRole"
	ErrFieldMaxDepositKDAsExceeded   = "MaxDepositKDAsExceeded"

	// ITO/Market errors
	ErrFieldInvalidPacks         = "InvalidPacks"
	ErrFieldInvalidPrice         = "InvalidPrice"
	ErrFieldPriceBelowMinimum    = "PriceBelowMinimum"
	ErrFieldInvalidOrder         = "InvalidOrder"
	ErrFieldOrderIDMissing       = "OrderIDMissing"
	ErrFieldOrderCurrencyInvalid = "OrderCurrencyInvalid"
	ErrFieldOrderExpired         = "OrderExpired"
	ErrFieldOrderNotFound        = "OrderNotFound"
	ErrFieldInvalidMarketType    = "InvalidMarketType"
	ErrFieldBidTooLow            = "BidTooLow"
	ErrFieldMarketplaceError     = "MarketplaceError"
	ErrFieldITONotFound          = "ITONotFound"
	ErrFieldITONotActive         = "ITONotActive"
	ErrFieldITOTimeWindow        = "ITOTimeWindow"
	ErrFieldITOConfigError       = "ITOConfigError"
	ErrFieldMaxSupplyExceeded    = "MaxSupplyExceeded"
	ErrFieldInvalidRoyalties     = "InvalidRoyalties"
	ErrFieldInvalidMetadata      = "InvalidMetadata"
	ErrFieldAssetAlreadyExists   = "AssetAlreadyExists"
	ErrFieldAssetNotFound        = "AssetNotFound"
)

// ExtraDelayForRequestBlockInfo represents the number of seconds to wait since a block has been received and the
// moment when its components,like transactions, would be requested too if they are still missing
const ExtraDelayForRequestBlockInfo = ExtraDelayForBroadcastBlockInfo + time.Second

// ExtraDelayForBroadcastBlockInfo represents the number of seconds to wait since a block has been broadcast and the
// moment when its components,transactions, would be broadcast too
const ExtraDelayForBroadcastBlockInfo = 1 * time.Second
