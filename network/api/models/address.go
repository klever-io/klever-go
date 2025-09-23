package models

import "github.com/klever-io/klever-go/kapps"

// AccountResponse -
type AccountResponse struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}

// AccountNonceResponse -
type AccountNonceResponse struct {
	Nonce             uint64 `json:"nonce"`
	FirstPendingNonce uint64 `json:"firstPendingNonce"`
	TxPending         uint64 `json:"txPending"`
}

// BalanceResponse -
type BalanceResponse struct {
	Balance int64 `json:"balance"`
}

// KDAResponse -
type KDAResponse struct {
	UserKDA *kapps.UserKDA `json:"userKDA"`
	Address string         `json:"address"`
	Asset   string         `json:"asset"`
}

// AvailableClaimResponse -
type AvailableClaimResponse struct {
	StakingRewards    int64            `json:"stakingRewards"`
	AllStakingRewards map[string]int64 `json:"allStakingRewards"`
	Allowance         int64            `json:"allowance"`
}

// AvailableClaimListResponse -
type AvailableClaimListResponse struct {
	Assets map[string]*AvailableClaimResponse `json:"assets"`
}
