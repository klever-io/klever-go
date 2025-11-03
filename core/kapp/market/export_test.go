package market

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// This file exports private functions for testing purposes only.
// It's only compiled during tests (note the _test.go suffix).

// ComputeReferralAmount exports the private computeReferralAmount method for testing
func (m *marketKapp) ComputeReferralAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, referralAmount int64, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	return m.computeReferralAmount(ctx, marketOrder, referralAmount, currencyID)
}

// ComputeRoyaltiesFixedDeposit exports the private computeRoyaltiesFixedDeposit method for testing
func (m *marketKapp) ComputeRoyaltiesFixedDeposit(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	return m.computeRoyaltiesFixedDeposit(ctx, marketOrder, asset)
}

// ComputeRoyaltiesAmount exports the private computeRoyaltiesAmount method for testing
func (m *marketKapp) ComputeRoyaltiesAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, asset *kapps.KDAData, currencyID []byte, royaltiesAmount int64) (transaction.Transaction_TXResultCode, error) {
	return m.computeRoyaltiesAmount(ctx, marketOrder, asset, currencyID, royaltiesAmount)
}

// ComputeSplitRoyalties exports the private computeSplitRoyalties method for testing
func (m *marketKapp) ComputeSplitRoyalties(ctx kapp.KappContext, address string, marketOrder *kapps.MarketOrderData, currencyID []byte, value int64, percentage int64, royaltiesToPay *int64) (transaction.Transaction_TXResultCode, error) {
	return m.computeSplitRoyalties(ctx, address, marketOrder, currencyID, value, percentage, royaltiesToPay)
}

// ComputeMarketOwnerAmount exports the private computeMarketOwnerAmount method for testing
func (m *marketKapp) ComputeMarketOwnerAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, currencyID []byte, marketOwnerAmount int64) (transaction.Transaction_TXResultCode, error) {
	return m.computeMarketOwnerAmount(ctx, marketOrder, currencyID, marketOwnerAmount)
}

// ExecuteBuyMarket exports the private executeBuyMarket method for testing
func (m *marketKapp) ExecuteBuyMarket(bidderAcc state.UserAccountHandler, marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	return m.executeBuyMarket(bidderAcc, marketKapp, marketOrder, currencyID)
}

