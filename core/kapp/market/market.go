package market

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.MarketKapp = (*marketKapp)(nil)

type marketKapp struct {
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewMarketKApp holds the arguments needed to create a MarketKapp
type ArgsNewMarketKApp struct {
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewMarketKApp creates a market KApp
func NewMarketKApp(
	args *ArgsNewMarketKApp,
) (*marketKapp, error) {
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}

	v := &marketKapp{
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (m *marketKapp) IsInterfaceNil() bool {
	return m == nil
}

func (m *marketKapp) SetKAppController(controller kapp.KAppController) error {
	m.KAppController = controller

	return nil
}

func (m *marketKapp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	m.accountsCacher = cacher

	return nil
}

func (m *marketKapp) GetAccountsCacher() state.AccountsCacher {
	return m.accountsCacher
}

func (m *marketKapp) GetExistingUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := m.accountsCacher.GetExistingUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (m *marketKapp) LoadUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := m.accountsCacher.LoadUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (m *marketKapp) GetMarketplace(marketplaceID []byte) (state.KAppAccountHandler, *kapps.Marketplace, error) {
	marketKapp, err := m.accountsCacher.LoadKApp(kapps.MarketKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if marketplaceID == nil {
		return marketKapp, &kapps.Marketplace{}, nil
	}

	key := kdautils.ToMarketplaceKey(marketplaceID)

	marketBytes, err := marketKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(marketBytes) == 0 {
		return marketKapp, &kapps.Marketplace{}, common.ErrNotFoundInKApp
	}

	marketplace := &kapps.Marketplace{}
	err = m.marshalizer.Unmarshal(marketplace, marketBytes)
	if err != nil {
		return nil, nil, err
	}

	return marketKapp, marketplace, nil
}

func (m *marketKapp) SetMarketplace(marketplaceKapp state.KAppAccountHandler, marketplace *kapps.Marketplace) error {
	data, err := m.marshalizer.Marshal(marketplace)
	if err != nil {
		return err
	}

	key := kdautils.ToMarketplaceKey(marketplace.ID)

	err = marketplaceKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (m *marketKapp) GetMarketOrder(orderID []byte) (state.KAppAccountHandler, *kapps.MarketOrderData, error) {
	marketKapp, err := m.accountsCacher.GetExistingKapp(kapps.MarketKAppAddress)
	if err != nil {
		return nil, nil, err
	}

	if orderID == nil {
		return marketKapp, &kapps.MarketOrderData{}, nil
	}

	key := kdautils.ToMarketOrderKey(orderID)

	marketBytes, err := marketKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil && !errors.Is(err, common.ErrNilTrie) {
		return nil, nil, err
	}
	if len(marketBytes) == 0 {
		return marketKapp, &kapps.MarketOrderData{}, common.ErrNotFoundInKApp
	}

	market := &kapps.MarketOrderData{}
	err = m.marshalizer.Unmarshal(market, marketBytes)
	if err != nil {
		return nil, nil, err
	}

	return marketKapp, market, nil
}

func (m *marketKapp) SetMarketOrder(marketKapp state.KAppAccountHandler, order *kapps.MarketOrderData) error {
	data, err := m.marshalizer.Marshal(order)
	if err != nil {
		return err
	}

	key := kdautils.ToMarketOrderKey(order.ID)

	err = marketKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (m *marketKapp) Buy(sender []byte, tc *transaction.BuyContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	orderID := tc.GetID()

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	if tc.GetAmount() <= 0 {
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(orderID)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if !bytes.Equal(marketOrder.CurrencyID, currencyID) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.EndTime < ctx.Block().GetTimestamp() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.CurrentBid >= tc.GetAmount() {
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	executeBuyMarket := false
	switch marketOrder.MarketType {
	case kapps.MarketOrderData_BuyItNow:
		if tc.GetAmount() < marketOrder.Price {
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
		executeBuyMarket = true
	case kapps.MarketOrderData_Auction:
		if tc.GetAmount() < marketOrder.ReservePrice {
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
		if marketOrder.Price > 0 && tc.GetAmount() >= marketOrder.Price {
			executeBuyMarket = true
		}
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if len(marketOrder.CurrentBidder) > 0 {
		lastBidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		err = lastBidderAcc.AddToBalance(marketOrder.CurrentBid, currencyID, m.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := m.accountsCacher.UpdateUser(lastBidderAcc); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.MarketKAppAddress,
			lastBidderAcc.AddressBytes(),
			[]byte(strconv.FormatInt(marketOrder.CurrentBid, 10)),
			currencyID,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
			marketOrder.MarketplaceID,
			marketOrder.ID,
		))

	}

	bidderAcc, err := m.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = bidderAcc.SubFromBalance(tc.GetAmount(), currencyID, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	marketOrder.CurrentBid = tc.GetAmount()
	marketOrder.CurrentBidder = bidderAcc.AddressBytes()

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		bidderAcc.AddressBytes(),
		kapps.MarketKAppAddress,
		[]byte(strconv.FormatInt(tc.GetAmount(), 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	var executeBuyOrder int64 = txProcess.OrderNotExecuted
	if executeBuyMarket {
		executeBuyOrder = txProcess.OrderExecuted
	}
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Buy,
		ctx.ContractID(),
		marketOrder.ID,
		marketOrder.MarketplaceID,
		[]byte(strconv.FormatInt(executeBuyOrder, 10)),
		bidderAcc.AddressBytes(),
		[]byte(strconv.FormatInt(marketOrder.CurrentBid, 10)),
		marketOrder.CurrencyID,
	))

	if executeBuyMarket {
		marketOrder.EndTime = ctx.Block().GetTimestamp()
		marketOrder.IsClaimed = true

		return m.executeBuyMarket(bidderAcc, marketKapp, marketOrder, currencyID)
	}

	if err := m.accountsCacher.UpdateUser(bidderAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeReferralAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, referralAmount int64, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	if referralAmount <= 0 {
		return transaction.Transaction_Ok, nil
	}

	_, marketplace, err := m.GetMarketplace(marketOrder.MarketplaceID)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	referralAcc, err := m.LoadUserAccount(marketplace.ReferralAddress)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = referralAcc.AddToBalance(referralAmount, currencyID, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		referralAcc.AddressBytes(),
		[]byte(strconv.FormatInt(referralAmount, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	if err := m.accountsCacher.UpdateUser(referralAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeSplitRoyalties(ctx kapp.KappContext, address string, marketOrder *kapps.MarketOrderData, currencyID []byte, value int64, percentage int64, royaltiesToPay *int64) (transaction.Transaction_TXResultCode, error) {
	decodedAddress, err := hex.DecodeString(address)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	splitRoyalty, err := m.LoadUserAccount(decodedAddress)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	splitToPay, err := tools.ComputePercentageI64(value, percentage, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}
	*royaltiesToPay -= splitToPay

	err = splitRoyalty.AddToBalance(splitToPay, currencyID, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := m.accountsCacher.UpdateUser(splitRoyalty); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		splitRoyalty.AddressBytes(),
		[]byte(strconv.FormatInt(splitToPay, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeRoyaltiesFixedDeposit(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if marketOrder.RoyaltiesFixedDeposit <= 0 {
		return transaction.Transaction_Ok, nil
	}

	royaltiesMarketFixedToPay := marketOrder.RoyaltiesFixedDeposit
	for key, value := range asset.Royalties.SplitRoyalties {
		status, err := m.computeSplitRoyalties(ctx, key, marketOrder, kdautils.KLVIdentifier, marketOrder.RoyaltiesFixedDeposit, int64(value.PercentMarketFixed), &royaltiesMarketFixedToPay)
		if err != nil {
			return status, err
		}
	}

	if royaltiesMarketFixedToPay <= 0 {
		return transaction.Transaction_Ok, nil
	}

	kdaOwner, err := m.LoadUserAccount(asset.Royalties.Address)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = kdaOwner.AddToBalance(royaltiesMarketFixedToPay, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := m.accountsCacher.UpdateUser(kdaOwner); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		kdaOwner.AddressBytes(),
		[]byte(strconv.FormatInt(royaltiesMarketFixedToPay, 10)),
		kdautils.KLVIdentifier,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeRoyaltiesAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, asset *kapps.KDAData, currencyID []byte, royaltiesAmount int64) (transaction.Transaction_TXResultCode, error) {
	if royaltiesAmount <= 0 {
		return transaction.Transaction_Ok, nil
	}

	royaltiesMarketPercentageToPay := royaltiesAmount
	for key, value := range asset.Royalties.SplitRoyalties {
		status, err := m.computeSplitRoyalties(ctx, key, marketOrder, currencyID, royaltiesAmount, int64(value.PercentMarketPercentage), &royaltiesMarketPercentageToPay)
		if err != nil {
			return status, err
		}
	}

	if royaltiesMarketPercentageToPay <= 0 {
		return transaction.Transaction_Ok, nil
	}

	kdaOwner, err := m.GetExistingUserAccount(asset.Royalties.Address)
	if !m.forkController.KdaFpr() {
		kdaOwner, err = m.GetExistingUserAccount(asset.OwnerAddress)
	}
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = kdaOwner.AddToBalance(royaltiesMarketPercentageToPay, currencyID, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	if err := m.accountsCacher.UpdateUser(kdaOwner); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		kdaOwner.AddressBytes(),
		[]byte(strconv.FormatInt(royaltiesMarketPercentageToPay, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeMarketOwnerAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, currencyID []byte, marketOwnerAmount int64) (transaction.Transaction_TXResultCode, error) {
	if marketOwnerAmount <= 0 {
		return transaction.Transaction_Ok, nil
	}

	marketOwnerAcc, err := m.LoadUserAccount(marketOrder.OwnerAddress)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	err = marketOwnerAcc.AddToBalance(marketOwnerAmount, currencyID, m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		marketOwnerAcc.AddressBytes(),
		[]byte(strconv.FormatInt(marketOwnerAmount, 10)),
		currencyID,
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	if err := m.accountsCacher.UpdateUser(marketOwnerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) executeBuyMarket(bidderAcc state.UserAccountHandler, marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	_, asset, err := m.KAppController.GetKDAKApp().GetKDA(marketOrder.CollectionID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	referralAmount, err := tools.ComputePercentageI64(marketOrder.CurrentBid, int64(marketOrder.ReferralPercentage), m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}
	royaltiesAmount, err := tools.ComputePercentageI64(marketOrder.CurrentBid, int64(asset.Royalties.MarketPercentage), m.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}
	marketOwnerAmount := marketOrder.CurrentBid - referralAmount - royaltiesAmount

	status, err := m.computeReferralAmount(ctx, marketOrder, referralAmount, currencyID)
	if err != nil {
		return status, err
	}

	status, err = m.computeRoyaltiesFixedDeposit(ctx, marketOrder, asset)
	if err != nil {
		return status, err
	}

	status, err = m.computeRoyaltiesAmount(ctx, marketOrder, asset, currencyID, royaltiesAmount)
	if err != nil {
		return status, err
	}

	status, err = m.computeMarketOwnerAmount(ctx, marketOrder, currencyID, marketOwnerAmount)
	if err != nil {
		return status, err
	}

	data, err := marketKapp.SubInternalKDA(marketOrder.CollectionID, marketOrder.AssetID)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	err = bidderAcc.AddInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, data)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		bidderAcc.AddressBytes(),
		[]byte("1"),
		marketOrder.CollectionID,
		marketOrder.AssetID,
		[]byte{byte(kapps.KDAData_NonFungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	if err := m.accountsCacher.UpdateUser(bidderAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) Claim(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(tc.GetID())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if marketOrder.GetIsClaimed() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(sender, marketOrder.OwnerAddress) && !bytes.Equal(sender, marketOrder.CurrentBidder) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketOrder.IsClaimed = true

	claimType := []byte(strconv.FormatInt(int64(transaction.ClaimContract_MarketClaim.Enum().Number()), 10))
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Claim,
		ctx.ContractID(),
		[]byte(strconv.FormatInt(0, 10)),
		marketOrder.ID,
		marketOrder.MarketplaceID,
		[]byte{},
		[]byte{},
		claimType,
	))

	if marketOrder.EndTime >= ctx.Block().GetTimestamp() {
		if bytes.Equal(sender, marketOrder.OwnerAddress) &&
			marketOrder.ReservePrice > 0 &&
			marketOrder.CurrentBid >= marketOrder.ReservePrice {
			bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			resultCode, err := m.executeBuyMarket(bidderAcc, marketKapp, marketOrder, marketOrder.CurrencyID)
			if err != nil {
				return resultCode, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Buy,
				ctx.ContractID(),
				marketOrder.ID,
				marketOrder.MarketplaceID,
			))

			return transaction.Transaction_Ok, nil
		}

		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.ReservePrice > 0 && marketOrder.CurrentBid >= marketOrder.ReservePrice {
		bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		resultCode, err := m.executeBuyMarket(bidderAcc, marketKapp, marketOrder, marketOrder.CurrencyID)
		if err != nil {
			return resultCode, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Buy,
			ctx.ContractID(),
			marketOrder.ID,
			marketOrder.MarketplaceID,
			[]byte(strconv.FormatInt(txProcess.OrderExecuted, 10)),
			bidderAcc.AddressBytes(),
			[]byte(strconv.FormatInt(marketOrder.CurrentBid, 10)),
			marketOrder.CurrencyID,
		))

		return transaction.Transaction_Ok, nil
	} else {
		marketOwnerAcc, err := m.GetExistingUserAccount(marketOrder.OwnerAddress)
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		data, err := marketKapp.SubInternalKDA(marketOrder.CollectionID, marketOrder.AssetID)
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		err = marketOwnerAcc.AddInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, data)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			marketKapp.AddressBytes(),
			marketOwnerAcc.AddressBytes(),
			[]byte("1"),
			marketOrder.CollectionID,
			marketOrder.AssetID,
			[]byte{byte(kapps.KDAData_NonFungible)},
			marketOrder.MarketplaceID,
			marketOrder.ID,
		))

		if marketOrder.RoyaltiesFixedDeposit > 0 {
			err = marketOwnerAcc.AddToBalance(marketOrder.RoyaltiesFixedDeposit, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				marketKapp.AddressBytes(),
				marketOwnerAcc.AddressBytes(),
				[]byte(strconv.FormatInt(marketOrder.RoyaltiesFixedDeposit, 10)),
				kdautils.KLVIdentifier,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				marketOrder.MarketplaceID,
				marketOrder.ID,
			))
		}

		if err := m.accountsCacher.UpdateUser(marketOwnerAcc); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		if len(marketOrder.CurrentBidder) > 0 {
			bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
			if err != nil {
				return transaction.Transaction_LoadAccountError, err
			}

			err = bidderAcc.AddToBalance(marketOrder.CurrentBid, marketOrder.CurrencyID, m.forkController.EnableSmartContracts())
			if err != nil {
				return transaction.Transaction_BalanceError, err
			}

			ctx.Receipts().Add(txProcess.NewReceipt(
				txProcess.Transfer,
				ctx.ContractID(),
				marketKapp.AddressBytes(),
				bidderAcc.AddressBytes(),
				[]byte(strconv.FormatInt(marketOrder.CurrentBid, 10)),
				marketOrder.CurrencyID,
				nil,
				[]byte{byte(kapps.KDAData_Fungible)},
				marketOrder.MarketplaceID,
				marketOrder.ID,
			))

			if err := m.accountsCacher.UpdateUser(bidderAcc); err != nil {
				return transaction.Transaction_SaveAccountError, err
			}
		}
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) Sell(sender []byte, tc *transaction.SellContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetAssetID() == nil {
		return transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	if tc.GetMarketplaceID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	assetID := bytes.Split(tc.GetAssetID(), []byte(kapps.Sp))
	if len(assetID) != 2 {
		return transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
	}

	if tc.GetPrice() < 0 || tc.GetReservePrice() < 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	oneYearExpiration := time.Unix(ctx.Block().GetTimestamp(), 0).AddDate(1, 0, 0).Unix()
	if tc.GetEndTime() <= ctx.Block().GetTimestamp() || tc.GetEndTime() > oneYearExpiration {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	if len(kapps.MarketOrderData_EnumMarketType_name[int32(tc.GetMarketType())]) == 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	_, asset, err := m.KAppController.GetKDAKApp().GetKDA(assetID[0])
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	// only allowed to sell non-fungible assets
	if asset.AssetType != kapps.KDAData_NonFungible {
		return transaction.Transaction_AssetTypeInvalid, common.ErrInvalidValue
	}

	_, marketplace, err := m.GetMarketplace(tc.GetMarketplaceID())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if asset.Royalties.MarketPercentage+marketplace.ReferralPercentage > core.HundredPercent {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	orderID := kdautils.ToMarketID(m.hasher, ctx.Block().GetRandSeed(), sender, ctx.TxNonce(), ctx.ContractID(), kdautils.MarketKeyLength)

	marketKapp, _, err := m.GetMarketOrder(orderID)
	if err != nil && !errors.Is(err, common.ErrNotFoundInKApp) {
		return transaction.Transaction_KeyConflict, err
	}

	reservePrice := int64(0)
	switch kapps.MarketOrderData_EnumMarketType(tc.GetMarketType()) {
	case kapps.MarketOrderData_BuyItNow:
		if tc.GetPrice() == 0 {
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
	case kapps.MarketOrderData_Auction:
		reservePrice = tc.GetReservePrice()
	default:
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := m.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	data, err := ownerAcc.SubInternalKDA(assetID[0], assetID[1])
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	err = marketKapp.AddInternalKDA(assetID[0], assetID[1], data)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		kapps.MarketKAppAddress,
		[]byte("1"),
		assetID[0],
		assetID[1],
		[]byte{byte(asset.AssetType)},
		marketplace.ID,
		orderID,
	))

	if asset.Royalties.MarketFixed > 0 {
		err = ownerAcc.SubFromBalance(asset.Royalties.MarketFixed, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			ownerAcc.AddressBytes(),
			kapps.MarketKAppAddress,
			[]byte(strconv.FormatInt(asset.Royalties.MarketFixed, 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
			marketplace.ID,
			orderID,
		))
	}

	marketOrder := &kapps.MarketOrderData{
		ID:                    orderID,
		MarketplaceID:         marketplace.ID,
		MarketType:            kapps.MarketOrderData_EnumMarketType(tc.GetMarketType()),
		OwnerAddress:          sender,
		CollectionID:          assetID[0],
		AssetID:               assetID[1],
		CurrencyID:            currencyID,
		Price:                 tc.GetPrice(),
		ReservePrice:          reservePrice,
		ReferralPercentage:    marketplace.ReferralPercentage,
		RoyaltiesFixedDeposit: asset.Royalties.MarketFixed,
		StartTime:             ctx.Block().GetTimestamp(),
		EndTime:               tc.GetEndTime(),
		IsClaimed:             false,
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.SetReturnData([][]byte{orderID})

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Sell,
		ctx.ContractID(),
		marketOrder.ID,
		marketOrder.MarketplaceID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) CancelOrder(sender []byte, tc *transaction.CancelMarketOrderContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetOrderID() == nil {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(tc.GetOrderID())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if marketOrder.IsClaimed {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(marketOrder.OwnerAddress, sender) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.EndTime <= ctx.Block().GetTimestamp() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := m.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	if marketOrder.RoyaltiesFixedDeposit > 0 {
		err = ownerAcc.AddToBalance(marketOrder.RoyaltiesFixedDeposit, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.MarketKAppAddress,
			ownerAcc.AddressBytes(),
			[]byte(strconv.FormatInt(marketOrder.RoyaltiesFixedDeposit, 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
			marketOrder.MarketplaceID,
			marketOrder.ID,
		))

		marketOrder.RoyaltiesFixedDeposit = 0
	}

	if len(marketOrder.CurrentBidder) > 0 && marketOrder.CurrentBid > 0 {
		bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
		if !m.forkController.KdaFpr() {
			bidderAcc, err = m.GetExistingUserAccount(sender)
		}
		if err != nil {
			return transaction.Transaction_LoadAccountError, err
		}

		err = bidderAcc.AddToBalance(marketOrder.CurrentBid, marketOrder.CurrencyID, m.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_BalanceError, err
		}

		if err := m.accountsCacher.UpdateUser(bidderAcc); err != nil {
			return transaction.Transaction_SaveAccountError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			kapps.MarketKAppAddress,
			bidderAcc.AddressBytes(),
			[]byte(strconv.FormatInt(marketOrder.CurrentBid, 10)),
			marketOrder.CurrencyID,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
			marketOrder.MarketplaceID,
			marketOrder.ID,
		))

		marketOrder.CurrentBidder = nil
		marketOrder.CurrentBid = 0
	}

	marketOrder.EndTime = ctx.Block().GetTimestamp()
	marketOrder.IsClaimed = true

	data, err := marketKapp.SubInternalKDA(marketOrder.CollectionID, marketOrder.AssetID)
	if err != nil {
		return transaction.Transaction_BalanceError, err
	}

	err = ownerAcc.AddInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, data)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.MarketKAppAddress,
		ownerAcc.AddressBytes(),
		[]byte("1"),
		marketOrder.CollectionID,
		marketOrder.AssetID,
		[]byte{byte(kapps.KDAData_NonFungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.CancelOrder,
		ctx.ContractID(),
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateUser(ownerAcc); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) CreateMarketplace(sender []byte, tc *transaction.CreateMarketplaceContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetName() == nil ||
		!utf8.Valid(tc.GetName()) ||
		len(tc.GetName()) > core.MaxNameSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	referralAddress := sender
	if len(tc.GetReferralAddress()) > 0 {
		if len(tc.GetReferralAddress()) != m.pubkeyConv.Len() {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		referralAddress = tc.GetReferralAddress()
	}

	if tc.GetReferralPercentage() > core.HundredPercent {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketplaceID := kdautils.ToMarketID(m.hasher, ctx.Block().GetRandSeed(), sender, ctx.TxNonce(), ctx.ContractID(), kdautils.MarketKeyLength)

	marketplaceKapp, _, err := m.GetMarketplace(marketplaceID)
	if err != nil && !errors.Is(err, common.ErrNotFoundInKApp) {
		return transaction.Transaction_KeyConflict, err
	}

	marketplace := &kapps.Marketplace{
		ID:                 marketplaceID,
		OwnerAddress:       sender,
		Name:               tc.GetName(),
		ReferralAddress:    referralAddress,
		ReferralPercentage: tc.GetReferralPercentage(),
	}

	err = m.SetMarketplace(marketplaceKapp, marketplace)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if err := m.accountsCacher.UpdateKapp(marketplaceKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.SetReturnData([][]byte{marketplaceID})

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.CreateMarketplace,
		ctx.ContractID(),
		marketplaceID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) ConfigMarketplace(sender []byte, tc *transaction.ConfigMarketplaceContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetReferralAddress() != nil && len(tc.GetReferralAddress()) != m.pubkeyConv.Len() {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetReferralPercentage() > core.HundredPercent {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketplaceKapp, marketplace, err := m.GetMarketplace(tc.GetMarketplaceID())
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if !bytes.Equal(marketplace.OwnerAddress, sender) {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetName() != nil {
		if !utf8.Valid(tc.GetName()) ||
			len(tc.GetName()) > core.MaxNameSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		marketplace.Name = tc.GetName()
	}

	marketplace.ReferralAddress = tc.GetReferralAddress()
	marketplace.ReferralPercentage = tc.GetReferralPercentage()

	err = m.SetMarketplace(marketplaceKapp, marketplace)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if err := m.accountsCacher.UpdateKapp(marketplaceKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.ConfigMarketplace,
		ctx.ContractID(),
		tc.GetMarketplaceID(),
	))

	return transaction.Transaction_Ok, nil
}
