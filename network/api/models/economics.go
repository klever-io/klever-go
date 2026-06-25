package models

// EconomicsResponse holds live KLV supply figures plus node-state held aggregates
// (pending rewards, market escrow, fees-pool KLV, FPR pool KLV, system-account KLV). See KLC-2506.
type EconomicsResponse struct {
	InitialSupply           int64 `json:"initialSupply"`
	MaxSupply               int64 `json:"maxSupply"`
	MintedValue             int64 `json:"mintedValue"`
	BurnedValue             int64 `json:"burnedValue"`
	CirculatingSupply       int64 `json:"circulatingSupply"`
	TotalStaked             int64 `json:"totalStaked"`
	PendingRewardsTotal     int64 `json:"pendingRewardsTotal"`
	MarketEscrowTotal       int64 `json:"marketEscrowTotal"`
	FeesPoolKLVTotal        int64 `json:"feesPoolKlvTotal"`
	FPRPoolTotal            int64 `json:"fprPoolTotal"`
	SystemAccountKLVBalance int64 `json:"systemAccountKlvBalance"`
}
