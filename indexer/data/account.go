package data

import (
	"time"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

type PermissionKey struct {
	Address string `json:"address"`
	Weight  int64  `json:"weight"`
}

type Permissions struct {
	ID             int32           `json:"id"`
	Type           int32           `json:"type"`
	PermissionName string          `json:"permissionName"`
	Threshold      int64           `json:"Threshold"`
	Operations     string          `json:"operations"`
	Signers        []PermissionKey `json:"signers"`
}

// AccountInfo holds (serializable) data about an account
type AccountInfo struct {
	Address         string        `json:"address,omitempty"`
	Nonce           uint64        `json:"nonce"`
	Name            string        `json:"name,omitempty"`
	RootHash        string        `json:"rootHash,omitempty"`
	Balance         int64         `json:"balance"`
	FrozenBalance   int64         `json:"frozenBalance"`
	UnfrozenBalance int64         `json:"unfrozenBalance"`
	Allowance       int64         `json:"allowance"`
	Permissions     []Permissions `json:"permissions"`
	Timestamp       time.Duration `json:"timestamp"`
	CodeHash        string        `json:"codeHash,omitempty"`
	CodeMetadata    string        `json:"codeMetadata,omitempty"`
	Foundation      bool          `json:"foundation,omitempty"`
}

// AccountBalanceHistory represents an entry in the user accounts balances history
type AccountBalanceHistory struct {
	Address         string        `json:"address"`
	Timestamp       time.Duration `json:"timestamp"`
	Balance         int64         `json:"balance"`
	FrozenBalance   int64         `json:"frozenBalance"`
	TokenIdentifier string        `json:"token,omitempty"`
	IsSender        bool          `json:"isSender,omitempty"`
}

type AssetInfo struct {
	Balance       int64 `json:"balance"`
	FrozenBalance int64 `json:"frozenBalance"`
}

type StakingInfo struct {
	StakeValue    int64  `json:"stakeValue"`
	Staked        bool   `json:"staked"`
	StakedEpoch   uint32 `json:"stakedEpoch"`
	UnstakedEpoch uint32 `json:"unstakedEpoch"`
	Delegation    string `json:"delegation"`
}

// Account is a structure that is needed for regular accounts
type Account struct {
	UserAccount state.UserAccountHandler
	IsSender    bool
}

// AccountKDA is a structure that holds information about a kda account
type AccountKDA struct {
	AccountAddress  string                             `json:"address"`
	AssetID         string                             `json:"assetId"`
	Collection      string                             `json:"collection,omitempty"`
	NFTNonce        uint64                             `json:"nftNonce,omitempty"`
	AssetName       string                             `json:"assetName"`
	AssetType       kapps.KDAData_EnumAssetType        `json:"assetType"`
	Balance         int64                              `json:"balance"`
	Precision       uint32                             `json:"precision"`
	FrozenBalance   int64                              `json:"frozenBalance"`
	UnfrozenBalance int64                              `json:"unfrozenBalance"`
	LastClaim       UserKDALastClaim                   `json:"lastClaim"`
	Buckets         []UserKDABucket                    `json:"buckets"`
	Metadata        string                             `json:"metadata,omitempty"`
	MIME            string                             `json:"mime,omitempty"`
	MarketplaceID   string                             `json:"marketplaceId,omitempty"`
	OrderID         string                             `json:"orderId,omitempty"`
	StakingType     kapps.StakingData_EnumInterestType `json:"stakingType"`
}

// UserKDABucket is a structure that holds information about a kda user buckets
type UserKDABucket struct {
	Id            string `json:"id"`
	StakedAt      int64  `json:"stakeAt"`
	StakedEpoch   uint32 `json:"stakedEpoch"`
	UnstakedEpoch uint32 `json:"unstakedEpoch"`
	Value         int64  `json:"balance"`
	Delegation    string `json:"delegation"`
	ValidatorName string `json:"validatorName"`
}

// UserKDALastClaim is a structure that holds information about a kda user last claim
type UserKDALastClaim struct {
	Timestamp int64  `json:"timestamp"`
	Epoch     uint32 `json:"epoch"`
}
type SignRate struct {
	NumSuccess uint32 `json:"numSuccess"`
	NumFailure uint32 `json:"numFailure"`
}

// ValidatorAccountInfo - TODO: should use this as 1 strcuture with peer to index?
type ValidatorAccountInfo struct {
	OwnerAddress                        string    `json:"ownerAddress"`
	BLSPublicKey                        string    `json:"blsPublicKey"`
	RewardsAddress                      string    `json:"rewardsAddress"`
	RegisterNonce                       uint64    `json:"registerNonce"`
	SelfStaked                          bool      `json:"selfStaked"`
	SelfStake                           int64     `json:"selfStake"`
	TotalStake                          int64     `json:"totalStake"`
	JailedEpoch                         uint32    `json:"jailedEpoch"`
	Jailed                              bool      `json:"jailed"`
	Waiting                             bool      `json:"waiting"`
	NumJailed                           uint32    `json:"numJailed"`
	TotalSlash                          int64     `json:"totalSlash"`
	CanDelegate                         bool      `json:"canDelegate"`
	MaxDelegation                       int64     `json:"maxDelegation"`
	Commission                          uint32    `json:"commission"`
	TotalRewards                        int64     `json:"totalRewards"`
	Name                                string    `json:"name"`
	Logo                                string    `json:"logo"`
	URIs                                []*URI    `json:"uris"`
	List                                string    `json:"list"`
	Index                               uint32    `json:"index"`
	AccumulatedFees                     int64     `json:"accumulatedFees"`
	ValidatorSuccessRate                *SignRate `json:"validatorSuccessRate"`
	LeaderSuccessRate                   *SignRate `json:"leaderSuccessRate"`
	ValidatorIgnoredSignaturesRate      uint32    `json:"validatorIgnoredSignaturesRate"`
	Rating                              uint32    `json:"rating"`
	TempRating                          uint32    `json:"tempRating"`
	NumSelectedInSuccessBlocks          uint32    `json:"numSelectedInSuccessBlocks"`
	ConsecutiveProposerMisses           uint32    `json:"consecutiveProposerMisses"`
	TotalValidatorSuccessRate           *SignRate `json:"totalValidatorSuccessRate"`
	TotalLeaderSuccessRate              *SignRate `json:"totalLeaderSuccessRate"`
	TotalValidatorIgnoredSignaturesRate uint32    `json:"totalValidatorIgnoredSignaturesRate"`
}

type ValidatorInfo struct {
	OwnerAddress   string `json:"ownerAddress"`
	RewardsAddress string `json:"rewardsAddress"`
	PublicKey      string `json:"publicKey"`
	List           string `json:"list"`
	SelfStaked     bool   `json:"selfStaked"`
	SelfStake      int64  `json:"selfStake"`
	TotalStake     int64  `json:"totalStake"`
	TotalSlash     int64  `json:"totalSlash"`
	TotalRewards   int64  `json:"totalRewards"`
	Jailed         bool   `json:"jailed"`
	JailedEpoch    uint32 `json:"jailedEpoch"`
	NumJailed      uint32 `json:"numJailed"`
	Waiting        bool   `json:"waiting"`
	CanDelegate    bool   `json:"canDelegate"`
	MaxDelegation  int64  `json:"maxDelegation"`
	Commission     uint32 `json:"commission"`
	Name           string `json:"name"`
	Logo           string `json:"logo"`
	URIs           []*URI `json:"uris"`
}
