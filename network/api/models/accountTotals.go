package models

// AccountTotalsResponse holds aggregates over all user accounts (count + inline KLV Balance and
// Allowance totals). Frozen/unfrozen are excluded (sub-trie; totalStaked covers frozen). See KLC-2506.
type AccountTotalsResponse struct {
	AccountCount   int64 `json:"accountCount"`
	BalanceTotal   int64 `json:"balanceTotal"`
	AllowanceTotal int64 `json:"allowanceTotal"`
}
