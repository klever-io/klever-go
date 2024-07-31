package models

// SendTXRequest -
type SendTXRequest struct {
	Type      uint32        `form:"type" json:"type"`
	Sender    string        `form:"sender" json:"sender"`
	Nonce     uint64        `form:"nonce" json:"nonce"`
	PermID    int32         `form:"permID" json:"permID"`
	Data      [][]byte      `form:"data" json:"data"`
	Contract  interface{}   `form:"contract" json:"contract"`
	Contracts []interface{} `form:"contracts" json:"contracts"`
	KDAFee    string        `form:"kdaFee" json:"kdaFee"`
}

type ContractInfo struct {
	ContractType *uint32 `json:"contractType"`
}

// TransferTXRequest -
type TransferTXRequest struct {
	Receiver     string `form:"receiver" json:"receiver"`
	Amount       int64  `form:"amount" json:"amount"`
	KDA          string `form:"kda" json:"kda"`
	KDARoyalties int64  `form:"kdaRoyalties" json:"kdaRoyalties"`
	KLVRoyalties int64  `form:"klvRoyalties" json:"klvRoyalties"`
}

// CreateAssetTXRequest -
type CreateAssetTXRequest struct {
	Type          uint32            `form:"type" json:"type"`
	Name          string            `form:"name" json:"name"`
	Ticker        string            `form:"ticker" json:"ticker"`
	OwnerAddress  string            `form:"ownerAddress" json:"ownerAddress"`
	AdminAddress  string            `form:"adminAddress" json:"adminAddress"`
	Logo          string            `form:"logo" json:"logo"`
	URIs          map[string]string `form:"uris" json:"uris"`
	Precision     uint32            `form:"precision" json:"precision"`
	InitialSupply int64             `form:"initialSupply" json:"initialSupply"`
	MaxSupply     int64             `form:"maxSupply" json:"maxSupply"`
	Royalties     *RoyaltiesInfo    `form:"royalties" json:"royalties"`
	Properties    *PropertiesInfo   `form:"properties" json:"properties"`
	Attributes    *AttributesInfo   `form:"attributes" json:"attributes"`
	Staking       *StakingInfo      `form:"staking" json:"staking"`
	Roles         []*RolesInfo      `form:"roles" json:"roles"`
}

// RoyaltiesInfo -
type RoyaltiesInfo struct {
	Address            string                       `form:"address" json:"address"`
	TransferPercentage []*RoyaltyData               `form:"transferPercentage" json:"transferPercentage"`
	TransferFixed      int64                        `form:"transferFixed" json:"transferFixed"`
	MarketPercentage   uint32                       `form:"marketPercentage" json:"marketPercentage"`
	MarketFixed        int64                        `form:"marketFixed" json:"marketFixed"`
	ITOPercentage      uint32                       `form:"itoPercentage" json:"itoPercentage"`
	ITOFixed           int64                        `form:"itoFixed" json:"itoFixed"`
	SplitRoyalties     map[string]*RoyaltySplitInfo `form:"splitRoyalties" json:"splitRoyalties"`
}

// RoyaltySplitInfo
type RoyaltySplitInfo struct {
	PercentTransferPercentage uint32 `json:"percentTransferPercentage,omitempty"`
	PercentTransferFixed      uint32 `json:"percentTransferFixed,omitempty"`
	PercentMarketPercentage   uint32 `json:"percentMarketPercentage,omitempty"`
	PercentMarketFixed        uint32 `json:"percentMarketFixed,omitempty"`
	PercentITOPercentage      uint32 `json:"percentITOPercentage,omitempty"`
	PercentITOFixed           uint32 `json:"percentITOFixed,omitempty"`
}

// RoyaltyData -
type RoyaltyData struct {
	Amount     int64  `form:"amount" json:"amount"`
	Percentage uint32 `form:"percentage" json:"percentage"`
}

// RoyaltyDataTX -
type RoyaltyDataTX struct {
	Amount     float64 `form:"amount" json:"amount"`
	Percentage float64 `form:"percentage" json:"percentage"`
}

// PropertiesInfo -
type PropertiesInfo struct {
	CanFreeze      bool `form:"canFreeze" json:"canFreeze"`
	CanWipe        bool `form:"canWipe" json:"canWipe"`
	CanPause       bool `form:"canPause" json:"canPause"`
	CanMint        bool `form:"canMint" json:"canMint"`
	CanBurn        bool `form:"canBurn" json:"canBurn"`
	CanChangeOwner bool `form:"canChangeOwner" json:"canChangeOwner"`
	CanAddRoles    bool `form:"canAddRoles" json:"canAddRoles"`
	LimitTransfer  bool `form:"limitTransfer" json:"limitTransfer"`
}

// AttributesInfo -
type AttributesInfo struct {
	IsPaused                   bool `form:"isPaused" json:"isPaused"`
	IsNFTMintStopped           bool `form:"isNFTMintStopped" json:"isNFTMintStopped"`
	IsRoyaltiesChangeStopped   bool `form:"isRoyaltiesChangeStopped" json:"isRoyaltiesChangeStopped"`
	IsNFTMetadataChangeStopped bool `form:"isNFTMetadataChangeStopped" json:"isNFTMetadataChangeStopped"`
}

// StakingInfo -
type StakingInfo struct {
	InterestType        uint32 `form:"interestType" json:"interestType"`
	APR                 uint32 `form:"apr" json:"apr"`
	MinEpochsToClaim    uint32 `form:"minEpochsToClaim" json:"minEpochsToClaim"`
	MinEpochsToUnstake  uint32 `form:"minEpochsToUnstake" json:"minEpochsToUnstake"`
	MinEpochsToWithdraw uint32 `form:"minEpochsToWithdraw" json:"minEpochsToWithdraw"`
}

// RolesInfo -
type RolesInfo struct {
	Address             string `form:"address" json:"address"`
	HasRoleMint         bool   `form:"hasRoleMint" json:"hasRoleMint"`
	HasRoleSetITOPrices bool   `form:"hasRoleSetITOPrices" json:"hasRoleSetITOPrices"`
	HasRoleDeposit      bool   `form:"hasRoleDeposit" json:"hasRoleDeposit"`
	HasRoleTransfer     bool   `form:"hasRoleTransfer" json:"hasRoleTransfer"`
}

// KDAPoolInfo -
type KDAPoolInfo struct {
	Active       bool   `form:"active" json:"active"`
	AdminAddress string `form:"adminAddress" json:"adminAddress"`
	FRatioKLV    int64  `form:"fRatioKLV" json:"fRatioKLV"`
	FRatioKDA    int64  `form:"fRatioKDA" json:"fRatioKDA"`
}

// AssetTriggerTXRequest -
type AssetTriggerTXRequest struct {
	TriggerType uint32            `form:"triggerType" json:"triggerType"`
	AssetID     string            `form:"assetId" json:"assetId"`
	Receiver    string            `form:"receiver" json:"receiver"`
	Amount      int64             `form:"amount" json:"amount"`
	MIME        string            `form:"mime" json:"mime"`
	Logo        string            `form:"logo" json:"logo"`
	Value       int64             `form:"value" json:"value"`
	URIs        map[string]string `form:"uris" json:"uris"`
	Role        *RolesInfo        `form:"role" json:"role"`
	Staking     *StakingInfo      `form:"staking" json:"staking"`
	Royalties   *RoyaltiesInfo    `form:"royalties" json:"royalties"`
	KDAPool     *KDAPoolInfo      `form:"kdaPool" json:"kdaPool"`
}

// CreateValidatorTXRequest -
type CreateValidatorTXRequest struct {
	BLSPublicKey        string            `form:"blsPublicKey" json:"blsPublicKey"`
	OwnerAddress        string            `form:"ownerAddress" json:"ownerAddress"`
	RewardAddress       string            `form:"rewardAddress" json:"rewardAddress"`
	CanDelegate         bool              `form:"canDelegate" json:"canDelegate"`
	Commission          uint32            `form:"commission" json:"commission"`
	MaxDelegationAmount int64             `form:"maxDelegationAmount" json:"maxDelegationAmount"`
	Logo                string            `form:"logo" json:"logo"`
	URIs                map[string]string `form:"uris" json:"uris"`
	Name                string            `form:"name" json:"name"`
}

// ValidatorConfigTXRequest -
type ValidatorConfigTXRequest struct {
	BLSPublicKey        string            `form:"blsPublicKey" json:"blsPublicKey"`
	RewardAddress       string            `form:"rewardAddress" json:"rewardAddress"`
	CanDelegate         bool              `form:"canDelegate" json:"canDelegate"`
	Commission          uint32            `form:"commission" json:"commission"`
	MaxDelegationAmount int64             `form:"maxDelegationAmount" json:"maxDelegationAmount"`
	Logo                string            `form:"logo" json:"logo"`
	URIs                map[string]string `form:"uris" json:"uris"`
	Name                string            `form:"name" json:"name"`
}

// FreezeTXRequest -
type FreezeTXRequest struct {
	Amount int64  `form:"amount" json:"amount"`
	KDA    string `form:"kda" json:"kda"`
}

// UnfreezeTXRequest -
type UnfreezeTXRequest struct {
	KDA      string `form:"kda" json:"kda"`
	BucketID string `form:"bucketId" json:"bucketId"`
}

// DelegateTXRequest -
type DelegateTXRequest struct {
	Receiver string `form:"receiver" json:"receiver"`
	BucketID string `form:"bucketId" json:"bucketId"`
}

// UndelegateTXRequest -
type UndelegateTXRequest struct {
	BucketID string `form:"bucketId" json:"bucketId"`
}

// WithdrawTXRequest -
type WithdrawTXRequest struct {
	KDA          string `form:"kda" json:"kda"`
	WithdrawType int32  `form:"withdrawType" json:"withdrawType"`
	Amount       int64  `form:"amount" json:"amount"`
	CurrencyID   string `form:"currencyID" json:"currencyID"`
}

// ClaimTXRequest -
type ClaimTXRequest struct {
	ClaimType int32  `form:"claimType" json:"claimType"`
	ID        string `form:"id" json:"id"`
}

// UnjailTXRequest -
type UnjailTXRequest struct{}

// SetAccountNameTXRequest -
type SetAccountNameTXRequest struct {
	Name string `form:"name" json:"name"`
}

// ProposalTXRequest -
type ProposalTXRequest struct {
	Parameters     map[int32]string `form:"parameters" json:"parameters"`
	Description    string           `form:"description" json:"description"`
	EpochsDuration uint32           `form:"epochsDuration" json:"epochsDuration"`
}

// VoteTXRequest -
type VoteTXRequest struct {
	Type       uint32 `form:"type" json:"type"`
	ProposalID uint64 `form:"proposalId" json:"proposalId"`
	Amount     int64  `form:"amount" json:"amount"`
}

// ConfigITOTXRequest -
type ConfigITOTXRequest struct {
	KDA                    string                          `form:"kda" json:"kda"`
	ReceiverAddress        string                          `form:"receiverAddress" json:"receiverAddress"`
	Status                 int32                           `form:"status" json:"status"`
	MaxAmount              int64                           `form:"maxAmount" json:"maxAmount"`
	PackInfo               map[string]PackInfoRequest      `form:"packInfo" json:"packInfo"`
	DefaultLimitPerAddress int64                           `form:"defaultLimitPerAddress" json:"defaultLimitPerAddress"`
	WhitelistStatus        int32                           `form:"whitelistStatus" json:"whitelistStatus"`
	WhitelistInfo          map[string]WhitelistInfoRequest `form:"whitelistInfo" json:"whitelistInfo"`
	WhitelistStartTime     int64                           `form:"whitelistStartTime" json:"whitelistStartTime"`
	WhitelistEndTime       int64                           `form:"whitelistEndTime" json:"whitelistEndTime"`
	StartTime              int64                           `form:"startTime" json:"startTime"`
	EndTime                int64                           `form:"endTime" json:"endTime"`
}

// ITOTriggerTXRequest -
type ITOTriggerTXRequest struct {
	TriggerType            uint32                          `form:"triggerType" json:"triggerType"`
	KDA                    string                          `form:"kda" json:"kda"`
	ReceiverAddress        string                          `form:"receiverAddress" json:"receiverAddress"`
	Status                 int32                           `form:"status" json:"status"`
	MaxAmount              int64                           `form:"maxAmount" json:"maxAmount"`
	PackInfo               map[string]PackInfoRequest      `form:"packInfo" json:"packInfo"`
	DefaultLimitPerAddress int64                           `form:"defaultLimitPerAddress" json:"defaultLimitPerAddress"`
	WhitelistStatus        int32                           `form:"whitelistStatus" json:"whitelistStatus"`
	WhitelistInfo          map[string]WhitelistInfoRequest `form:"whitelistInfo" json:"whitelistInfo"`
	WhitelistStartTime     int64                           `form:"whitelistStartTime" json:"whitelistStartTime"`
	WhitelistEndTime       int64                           `form:"whitelistEndTime" json:"whitelistEndTime"`
	StartTime              int64                           `form:"startTime" json:"startTime"`
	EndTime                int64                           `form:"endTime" json:"endTime"`
}

// WhitelistInfoRequest -
type WhitelistInfoRequest struct {
	Limit int64 `form:"limit" json:"limit"`
}

type SetITOPricesTXRequest struct {
	KDA      string                     `form:"kda" json:"kda"`
	PackInfo map[string]PackInfoRequest `form:"packInfo" json:"packInfo"`
}

// PackInfoRequest -
type PackInfoRequest struct {
	Packs []PackItemRequest `form:"packs" json:"packs"`
}

// PackItemRequest -
type PackItemRequest struct {
	Amount int64 `form:"amount" json:"amount"`
	Price  int64 `form:"price" json:"price"`
}

// SetITOPricesTXRequest -

// BuyTXRequest -
type BuyTXRequest struct {
	BuyType    int32  `form:"buyType" json:"buyType"`
	ID         string `form:"id" json:"id"`
	CurrencyID string `form:"currencyId" json:"currencyId"`
	Amount     int64  `form:"amount" json:"amount"`
}

// SellTXRequest -
type SellTXRequest struct {
	MarketType    int32  `form:"marketType" json:"marketType"`
	MarketplaceID string `form:"marketplaceId" json:"marketplaceId"`
	AssetID       string `form:"assetId" json:"assetId"`
	CurrencyID    string `form:"currencyId" json:"currencyId"`
	Price         int64  `form:"price" json:"price"`
	ReservePrice  int64  `form:"reservePrice" json:"reservePrice"`
	EndTime       int64  `form:"endTime" json:"endTime"`
}

// CancelMarketOrderTXRequest -
type CancelMarketOrderTXRequest struct {
	OrderID string `form:"orderId" json:"orderId"`
}

// CreateMarketplaceTXRequest -
type CreateMarketplaceTXRequest struct {
	Name               string `form:"name" json:"name"`
	ReferralAddress    string `form:"referralAddress" json:"referralAddress"`
	ReferralPercentage uint32 `form:"referralPercentage" json:"referralPercentage"`
}

// ConfigMarketplaceTXRequest -
type ConfigMarketplaceTXRequest struct {
	MarketplaceID      string `form:"marketplaceId" json:"marketplaceId"`
	Name               string `form:"name" json:"name"`
	ReferralAddress    string `form:"referralAddress" json:"referralAddress"`
	ReferralPercentage uint32 `form:"referralPercentage" json:"referralPercentage"`
}

// SignerTXRequest -
type SignerTXRequest struct {
	Address string `form:"address" json:"address"`
	Weight  int64  `form:"weight" json:"weight"`
}

// PermissionTXRequest -
type PermissionTXRequest struct {
	Type           int32             `form:"type" json:"type"`
	PermissionName string            `form:"permissionName" json:"permissionName"`
	Threshold      int64             `form:"threshold" json:"threshold"`
	Operations     string            `form:"operations" json:"operations"`
	Signers        []SignerTXRequest `form:"signers" json:"signers"`
}

// UpdateAccountPermissionTXRequest -
type UpdateAccountPermissionTXRequest struct {
	Permissions []PermissionTXRequest `form:"permissions" json:"permissions"`
}

// DepositTXRequest -
type DepositTXRequest struct {
	DepositType int32  `form:"depositType" json:"depositType"`
	KDA         string `form:"kda" json:"kda"`
	CurrencyID  string `form:"currencyId" json:"currencyId"`
	Amount      int64  `form:"amount" json:"amount"`
}

// SmartContractRequest -
type SmartContractRequest struct {
	SCType    int32            `form:"scType" json:"scType"`
	Address   string           `form:"address" json:"address"`
	CallValue map[string]int64 `form:"callValue" json:"callValue"`
}
