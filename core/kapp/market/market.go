package market

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.MarketKapp = (*marketKapp)(nil)

var log = logger.GetOrCreate("kapp/market")

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

// GetMarketEscrowTotal returns the total KLV held in open market orders: each unclaimed order's
// RoyaltiesFixedDeposit (always KLV) plus its CurrentBid when the order is priced in KLV.
func (m *marketKapp) GetMarketEscrowTotal() (int64, error) {
	app, err := m.accountsCacher.LoadKAppUncached(kapps.MarketKAppAddress)
	if err != nil {
		return 0, err
	}

	dataTrie := app.DataTrie()
	if check.IfNil(dataTrie) {
		return 0, nil
	}

	leavesChannels, err := dataTrie.GetAllLeavesOnChannel(app.GetRootHash(), context.Background())
	if err != nil {
		return 0, err
	}

	// Collect order keys first so the scan goroutine finishes before we read values back.
	prefix := []byte(kapps.MarketOrderPrefix + kapps.Sp)
	orderKeys := make([][]byte, 0)

	// A truncated walk undercounts the total, which must not be reported as a success.
	err = leavesChannels.ForEach(func(leaf data.KeyValueHolder) error {
		if !bytes.HasPrefix(leaf.Key(), prefix) {
			return nil
		}
		key := make([]byte, len(leaf.Key()))
		copy(key, leaf.Key())
		orderKeys = append(orderKeys, key)

		return nil
	})
	if err != nil {
		return 0, err
	}

	total := int64(0)
	for _, key := range orderKeys {
		add, err := m.orderEscrowAmount(app, key)
		if err != nil {
			return 0, err
		}
		if total > math.MaxInt64-add {
			log.Warn("GetMarketEscrowTotal: sum would overflow int64, skipping entry")
			continue
		}
		total += add
	}

	return total, nil
}

// orderEscrowAmount returns the KLV escrowed by one stored order: RoyaltiesFixedDeposit plus
// CurrentBid when priced in KLV; zero for empty, claimed, or negative-value entries.
func (m *marketKapp) orderEscrowAmount(app state.KAppAccountHandler, key []byte) (int64, error) {
	raw := app.GetStorage(key) // trims the trie tail, unlike the raw leaf value
	if len(raw) == 0 {
		return 0, nil
	}
	order := &kapps.MarketOrderData{}
	if err := m.marshalizer.Unmarshal(order, raw); err != nil {
		return 0, err
	}
	if order.IsClaimed {
		return 0, nil
	}
	add := order.RoyaltiesFixedDeposit
	if bytes.Equal(order.CurrencyID, kdautils.KLVIdentifier) {
		add += order.CurrentBid
	}
	if add < 0 {
		log.Warn("GetMarketEscrowTotal: negative escrow amount, skipping", "key", hex.EncodeToString(key))
		return 0, nil
	}
	return add, nil
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderIDMissing, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	orderID := tc.GetID()

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	if tc.GetAmount() <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(orderID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderNotFound, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	// Reject bids on an already-settled (claimed) order, mirroring the IsClaimed
	// guards in Claim and CancelOrder. Early seller-accept in Claim marks the order
	// claimed but leaves EndTime in the future, so a "zombie" order still reads as
	// live and would strand a later bidder (GHSA-26r5-4mm2-px5c). Gated on
	// FixAuditChangesV3: turning a successful Buy into a rejection is
	// consensus-visible and must activate at a future epoch to stay replay-safe.
	if m.forkController.FixAuditChangesV3() && marketOrder.IsClaimed {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOrder, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(marketOrder.CurrencyID, currencyID) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderCurrencyInvalid, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.EndTime < ctx.Block().GetTimestamp() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderExpired, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.CurrentBid >= tc.GetAmount() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBidTooLow, common.ErrInvalidValue.Error())
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	executeBuyMarket := false
	switch marketOrder.MarketType {
	case kapps.MarketOrderData_BuyItNow:
		if tc.GetAmount() < marketOrder.Price {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldPriceBelowMinimum, common.ErrInvalidValue.Error())
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
		executeBuyMarket = true
	case kapps.MarketOrderData_Auction:
		if tc.GetAmount() < marketOrder.ReservePrice {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldPriceBelowMinimum, common.ErrInvalidValue.Error())
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
		if marketOrder.Price > 0 && tc.GetAmount() >= marketOrder.Price {
			executeBuyMarket = true
		}
	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidMarketType, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	// Orders listed before FixAuditChangesV7 were never checked at Sell.
	if status, err := m.checkCurrency(ctx, currencyID); err != nil {
		return status, err
	}

	held, err := m.escrowHoldsNFT(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}
	if !held {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, common.ErrAssetNotFound.Error())
		return transaction.Transaction_BalanceError, common.ErrAssetNotFound
	}

	if status, err := m.checkBidControls(ctx, marketOrder, currencyID, sender, tc.GetAmount()); err != nil {
		return status, err
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, err.Error())
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

// referralBeneficiary resolves who receives the referral share of an order. Orders listed
// at or after FixAuditChangesV4 carry the address snapshotted at Sell time, so a
// ConfigMarketplace issued while the bid sits in escrow can neither redirect the payout to
// another address nor strand the order by clearing it. Orders listed before the fork have
// no snapshot and keep resolving the live marketplace.
func (m *marketKapp) referralBeneficiary(marketOrder *kapps.MarketOrderData) ([]byte, error) {
	if marketOrder.ReferralSnapshotted {
		return marketOrder.ReferralAddress, nil
	}

	_, marketplace, err := m.GetMarketplace(marketOrder.MarketplaceID)
	if err != nil {
		return nil, err
	}

	return marketplace.ReferralAddress, nil
}

// computeReferralAmount credits the referral share to referralAddress, which the caller
// resolved through referralBeneficiary. Taking it as a parameter keeps settlement to a
// single marketplace read for orders that still need one.
func (m *marketKapp) computeReferralAmount(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, referralAddress []byte, referralAmount int64, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	if referralAmount <= 0 {
		return transaction.Transaction_Ok, nil
	}

	referralAcc, err := m.LoadUserAccount(referralAddress)
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

	splitToPay, status, err := m.splitRoyaltyAmount(ctx, value, percentage, *royaltiesToPay)
	if err != nil {
		return status, err
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

// splitRoyaltyAmount returns the share of value a royalty split of percentage receives. Under
// FixMarketBuyOverflow a share above remaining, what the earlier splits left of value, is
// refused. Settlement and checkSettlementControls both take a split's amount from here.
func (m *marketKapp) splitRoyaltyAmount(ctx kapp.KappContext, value int64, percentage int64, remaining int64) (int64, transaction.Transaction_TXResultCode, error) {
	splitToPay, err := tools.ComputePercentageI64(value, percentage, m.forkController.EnableSmartContracts())
	if err != nil {
		return 0, transaction.Transaction_ParameterInvalid, err
	}
	if m.forkController.FixMarketBuyOverflow() && splitToPay > remaining {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return 0, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	return splitToPay, transaction.Transaction_Ok, nil
}

func (m *marketKapp) computeRoyaltiesFixedDeposit(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if marketOrder.RoyaltiesFixedDeposit <= 0 {
		return transaction.Transaction_Ok, nil
	}

	// Each split adds a Transfer receipt that a calling contract reads back as a transfer, so
	// under FixAuditChangesV6 the splits are paid in key order, the same on every node.
	royaltiesMarketFixedToPay := marketOrder.RoyaltiesFixedDeposit
	for _, key := range types.OrderedKeys(asset.Royalties.SplitRoyalties, m.forkController.FixAuditChangesV6()) {
		value := asset.Royalties.SplitRoyalties[key]
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

	// Paid in key order under FixAuditChangesV6, as in computeRoyaltiesFixedDeposit.
	royaltiesMarketPercentageToPay := royaltiesAmount
	for _, key := range types.OrderedKeys(asset.Royalties.SplitRoyalties, m.forkController.FixAuditChangesV6()) {
		value := asset.Royalties.SplitRoyalties[key]
		status, err := m.computeSplitRoyalties(ctx, key, marketOrder, currencyID, royaltiesAmount, int64(value.PercentMarketPercentage), &royaltiesMarketPercentageToPay)
		if err != nil {
			return status, err
		}
	}

	if royaltiesMarketPercentageToPay <= 0 {
		return transaction.Transaction_Ok, nil
	}

	// A receiver without an account used to fail settlement, leaving an ended auction stuck in
	// escrow. Under FixAuditChangesV6 it is created like any other payee (KdaFpr is always
	// active by then); before the fork the legacy loads are kept as they were.
	var kdaOwner state.UserAccountHandler
	var err error
	if m.forkController.FixAuditChangesV6() {
		kdaOwner, err = m.LoadUserAccount(asset.Royalties.Address)
	} else {
		kdaOwner, err = m.GetExistingUserAccount(asset.Royalties.Address)
		if !m.forkController.KdaFpr() {
			kdaOwner, err = m.GetExistingUserAccount(asset.OwnerAddress)
		}
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

// settlementAmounts is how a sale at a given bid splits in the order currency: the referral
// share and who receives it, the percentage royalty (split and remainder) and what is left to
// the order owner. Settlement pays it and checkSettlementControls checks the same legs, so both
// take it from computeSettlementAmounts.
type settlementAmounts struct {
	referralAddress []byte
	referral        int64
	royalties       int64
	marketOwner     int64
}

// computeSettlementAmounts splits bid for marketOrder under the asset's live royalties. It only
// reads state; a failure adds the same receipt and returns the same code settlement always has.
func (m *marketKapp) computeSettlementAmounts(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, royalties *kapps.RoyaltiesData, bid int64) (settlementAmounts, transaction.Transaction_TXResultCode, error) {
	referralAmount, err := tools.ComputePercentageI64(bid, int64(marketOrder.ReferralPercentage), m.forkController.EnableSmartContracts())
	if err != nil {
		return settlementAmounts{}, transaction.Transaction_ParameterInvalid, err
	}

	// Marketplaces whose referral address was cleared before the fork still carry orders that
	// snapshotted a positive percentage, and settlement pays the live address, so LoadAccount
	// rejects the empty address and the order can never settle. CancelOrder refuses expired
	// orders and Claim only recovers when the reserve was not met, so the escrowed asset and
	// the bid would have no exit but the owner voluntarily reconfiguring. Drop the referral
	// share here rather than in computeReferralAmount: marketOwnerAmount is derived below, so
	// zeroing it at the source folds the share into that amount, which settles to the order
	// owner, instead of stranding it inside the KApp.
	//
	// Resolved once here and handed to computeReferralAmount below, so a settlement that
	// still needs the live marketplace reads it a single time. A snapshotted order reads it
	// not at all. The resolution goes through referralBeneficiary so the order is judged on
	// its own address, not the live one: reading the marketplace here would hand the owner
	// the diversion back, since clearing the address after listing would fold the referral
	// share into the amount that settles to the order owner. A snapshot can still be empty
	// when the marketplace was created with a percentage but no address, and that order
	// would strand the same way, so the drop covers both.
	//
	// The read moving ahead of the marketOwnerAmount overflow check is not fork-gated, but
	// it cannot change which error a settlement returns: an order always names a marketplace
	// that exists, since Sell rejects a nil or unknown ID and marketplaces are never deleted.
	var referralAddress []byte
	if referralAmount > 0 {
		referralAddress, err = m.referralBeneficiary(marketOrder)
		if err != nil {
			return settlementAmounts{}, transaction.Transaction_ParameterInvalid, err
		}
	}

	if m.forkController.FixAuditChangesV4() && len(referralAddress) == 0 {
		referralAmount = 0
	}

	// Use the royalty snapshotted into the order at Sell time when present, so the order
	// economics are fixed at list time and a later UpdateRoyalties cannot change the payout
	// (KLC-2457 / GHSA-p7gw-2pcp-5pf8). Orders listed before the fork carry no snapshot and
	// keep using the live asset royalty.
	royaltyPercentage := royalties.MarketPercentage
	if marketOrder.RoyaltySnapshotted {
		royaltyPercentage = marketOrder.RoyaltyPercentage
	}
	royaltiesAmount, err := tools.ComputePercentageI64(bid, int64(royaltyPercentage), m.forkController.EnableSmartContracts())
	if err != nil {
		return settlementAmounts{}, transaction.Transaction_ParameterInvalid, err
	}
	marketOwnerAmount := bid - referralAmount - royaltiesAmount

	if m.forkController.FixMarketBuyOverflow() && marketOwnerAmount < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return settlementAmounts{}, transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	return settlementAmounts{
		referralAddress: referralAddress,
		referral:        referralAmount,
		royalties:       royaltiesAmount,
		marketOwner:     marketOwnerAmount,
	}, transaction.Transaction_Ok, nil
}

// prepareSettlement loads marketOrder's collection and splits its current bid, the sale to
// buyer that settlement pays. It only reads state.
//
// Under FixAuditChangesV6 the sale is a transfer of both assets and follows their controls: a
// paused collection or currency refuses it with AssetPaused, and a leg a direct transfer would
// refuse with KDATransferNotAllowed (see checkSettlementControls). The pause is checked first.
// Either refusal returns without a receipt, so the expired Claim can unwind the order instead;
// a caller that fails on it adds the receipt with addControlRefusedReceipt. Before the fork
// neither asset's controls are read.
func (m *marketKapp) prepareSettlement(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, currencyID []byte, buyer []byte) (*kapps.KDAData, settlementAmounts, transaction.Transaction_TXResultCode, error) {
	_, asset, err := m.KAppController.GetKDAKApp().GetKDA(marketOrder.CollectionID)
	if err != nil {
		return nil, settlementAmounts{}, transaction.Transaction_KAPPError, err
	}

	var currency *kapps.KDAData
	if m.forkController.FixAuditChangesV6() {
		status, errPause := checkNotPaused(asset)
		if errPause != nil {
			return nil, settlementAmounts{}, status, errPause
		}

		currency, status, errPause = m.loadUnpausedKDA(ctx, currencyID)
		if errPause != nil {
			return nil, settlementAmounts{}, status, errPause
		}
	}

	amounts, status, err := m.computeSettlementAmounts(ctx, marketOrder, asset.Royalties, marketOrder.CurrentBid)
	if err != nil {
		return nil, settlementAmounts{}, status, err
	}

	if m.forkController.FixAuditChangesV6() {
		status, err = m.checkSettlementControls(ctx, marketOrder, asset, asset.Royalties, currency, buyer, amounts)
		if err != nil {
			return nil, settlementAmounts{}, status, err
		}
	}

	return asset, amounts, transaction.Transaction_Ok, nil
}

func (m *marketKapp) executeBuyMarket(bidderAcc state.UserAccountHandler, marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	// This is reached from Buy and from both settling branches of Claim, so under
	// FixAuditChangesV6 a pause or a role change after the listing or the bid refuses
	// settlement too. CancelOrder and a Claim below the reserve still return the NFT and the
	// bid to their owners, and so does an expired Claim that a pause or a refused leg stops
	// from settling (see Claim).
	asset, amounts, status, err := m.prepareSettlement(ctx, marketOrder, currencyID, bidderAcc.AddressBytes())
	if err != nil {
		addControlRefusedReceipt(ctx, status)
		return status, err
	}

	status, err = m.computeReferralAmount(ctx, marketOrder, amounts.referralAddress, amounts.referral, currencyID)
	if err != nil {
		return status, err
	}

	status, err = m.computeRoyaltiesFixedDeposit(ctx, marketOrder, asset)
	if err != nil {
		return status, err
	}

	status, err = m.computeRoyaltiesAmount(ctx, marketOrder, asset, currencyID, amounts.royalties)
	if err != nil {
		return status, err
	}

	status, err = m.computeMarketOwnerAmount(ctx, marketOrder, currencyID, amounts.marketOwner)
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

// checkBidControls checks a bid of bid by buyer as if the order settled now. Under
// FixAuditChangesV6 the currency's and the collection's pause and transfer restriction apply to
// every bid, executing or not, before the previous bid is refunded or this one escrowed: an
// ineligible bid refused only at settlement would hold the order until it expires and then only
// unwind it (see Claim), outbidding an eligible buyer for nothing. Before the fork a bid reads
// neither asset.
func (m *marketKapp) checkBidControls(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, currencyID []byte, buyer []byte, bid int64) (transaction.Transaction_TXResultCode, error) {
	if !m.forkController.FixAuditChangesV6() {
		return transaction.Transaction_Ok, nil
	}

	currency, status, err := m.loadUnpausedKDA(ctx, currencyID)
	if err != nil {
		addControlRefusedReceipt(ctx, status)
		return status, err
	}

	collection, status, err := m.loadUnpausedKDA(ctx, marketOrder.CollectionID)
	if err != nil {
		addControlRefusedReceipt(ctx, status)
		return status, err
	}

	royalties := collection.GetRoyalties()
	if royalties == nil {
		royalties = &kapps.RoyaltiesData{}
	}

	amounts, status, err := m.computeSettlementAmounts(ctx, marketOrder, royalties, bid)
	if err != nil {
		return status, err
	}

	status, err = m.checkSettlementControls(ctx, marketOrder, collection, royalties, currency, buyer, amounts)
	addControlRefusedReceipt(ctx, status)
	return status, err
}

// checkSettlementControls refuses a sale a direct transfer from its real origin would refuse:
// the NFT moves from the order owner to buyer (the escrow is only an intermediary), and each
// currency leg paid a positive amount moves from buyer to its receiver: the referral, each
// royalty split, the percentage royalty remainder (royalties.Address, as settled under
// FixAuditChangesV6) and the order owner. The legs come from computeSettlementAmounts and the
// split shares from splitRoyaltyAmount, so the check sees exactly what settlement pays. Splits
// are visited in key order so the check is deterministic; every refusal returns the same code,
// KDATransferNotAllowed, without a receipt (see checkTransferAllowed).
// The RoyaltiesFixedDeposit legs are paid in KLV and are not checked.
func (m *marketKapp) checkSettlementControls(ctx kapp.KappContext, marketOrder *kapps.MarketOrderData, collection *kapps.KDAData, royalties *kapps.RoyaltiesData, currency *kapps.KDAData, buyer []byte, amounts settlementAmounts) (transaction.Transaction_TXResultCode, error) {
	status, err := checkTransferAllowed(collection, marketOrder.OwnerAddress, buyer, 1)
	if err != nil {
		return status, err
	}

	status, err = checkTransferAllowed(currency, buyer, amounts.referralAddress, amounts.referral)
	if err != nil {
		return status, err
	}

	if amounts.royalties > 0 {
		remaining := amounts.royalties
		for _, key := range types.OrderedKeys(royalties.SplitRoyalties, true) {
			splitAddress, errDecode := hex.DecodeString(key)
			if errDecode != nil {
				return transaction.Transaction_LoadAccountError, errDecode
			}

			splitToPay, status, err := m.splitRoyaltyAmount(ctx, amounts.royalties, int64(royalties.SplitRoyalties[key].GetPercentMarketPercentage()), remaining)
			if err != nil {
				return status, err
			}
			remaining -= splitToPay

			status, err = checkTransferAllowed(currency, buyer, splitAddress, splitToPay)
			if err != nil {
				return status, err
			}
		}

		status, err = checkTransferAllowed(currency, buyer, royalties.Address, remaining)
		if err != nil {
			return status, err
		}
	}

	return checkTransferAllowed(currency, buyer, marketOrder.OwnerAddress, amounts.marketOwner)
}

// checkCurrency refuses, from FixAuditChangesV7, an order currency that is not KLV or an existing
// fungible asset, at Sell and, for orders listed before the fork, at Buy. A currency
// named like an NFT's compound ID (COLL/1) shares the NFT's trie key, so pricing in it would move
// the NFT through the fungible balance path (KLC-2816). KLV is the native fungible asset and is
// not loaded. The other paths that take an asset ID (Transfer, Freeze/Unfreeze, Deposit, ITO, fee
// pools) load it with GetKDA, and the KDA trie holds only collection keys, so none of them
// resolves a compound ID; that does not make them require a fungible asset. A read error other
// than ErrAssetNotFound is returned as is.
func (m *marketKapp) checkCurrency(ctx kapp.KappContext, currencyID []byte) (transaction.Transaction_TXResultCode, error) {
	if !m.forkController.FixAuditChangesV7() || bytes.Equal(currencyID, kdautils.KLVIdentifier) {
		return transaction.Transaction_Ok, nil
	}

	if !bytes.Contains(currencyID, []byte(kapps.Sp)) {
		_, currency, err := m.KAppController.GetKDAKApp().GetKDA(currencyID)
		if err != nil && !errors.Is(err, common.ErrAssetNotFound) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
			return transaction.Transaction_KAPPError, err
		}
		if err == nil && currency.AssetType == kapps.KDAData_Fungible {
			return transaction.Transaction_Ok, nil
		}
	}

	ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderCurrencyInvalid, common.ErrInvalidValue.Error())
	return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
}

// escrowHoldsNFT reports whether the escrow's record of marketOrder's NFT holds it (see
// kapps.UserKDA.IsHeldNFT). Sell refuses a spent record from FixAuditChangesV7, but an order listed
// before the fork may escrow one, or none when another order on the same key cleared it (KLC-2835),
// and a sale would charge the buyer for a record that cannot be moved: from the fork such an order
// takes no bid and is not sold, and Claim unwinds it. A missing or undecodable record is not held;
// only a read error fails. Before the fork every record counts as held.
func (m *marketKapp) escrowHoldsNFT(marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData) (bool, error) {
	if !m.forkController.FixAuditChangesV7() {
		return true, nil
	}

	value, err := readEscrow(marketKapp, marketOrder.CollectionID, marketOrder.AssetID)
	if err != nil {
		return false, err
	}

	record := &kapps.UserKDA{}
	return m.marshalizer.Unmarshal(record, value) == nil && record.IsHeldNFT(), nil
}

// loadUnpausedKDA loads id for the FixAuditChangesV6 asset controls, refusing while it is paused
// (see checkNotPaused).
func (m *marketKapp) loadUnpausedKDA(ctx kapp.KappContext, id []byte) (*kapps.KDAData, transaction.Transaction_TXResultCode, error) {
	_, asset, err := m.KAppController.GetKDAKApp().GetKDA(id)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return nil, transaction.Transaction_KAPPError, err
	}

	status, err := checkNotPaused(asset)
	if err != nil {
		return nil, status, err
	}

	return asset, transaction.Transaction_Ok, nil
}

// checkNotPaused refuses moving asset while it is paused, as a direct transfer does. The refusal
// adds no receipt: the expired Claim does not fail on it but unwinds the order, and every other
// caller fails and adds it with addControlRefusedReceipt.
func checkNotPaused(asset *kapps.KDAData) (transaction.Transaction_TXResultCode, error) {
	if !asset.GetAttributes().GetIsPaused() {
		return transaction.Transaction_Ok, nil
	}

	return transaction.Transaction_AssetPaused, process.ErrAssetIsPaused
}

// checkTransferAllowed refuses moving amount of asset from sender to destination when a direct
// transfer would be refused. An amount of zero moves nothing and is never refused. The refusal
// adds no receipt, as in checkNotPaused.
func checkTransferAllowed(asset *kapps.KDAData, sender []byte, destination []byte, amount int64) (transaction.Transaction_TXResultCode, error) {
	if amount <= 0 || asset.IsTransferAllowed(sender, destination) {
		return transaction.Transaction_Ok, nil
	}

	return transaction.Transaction_KDATransferNotAllowed, process.ErrKDATransferNotAllowed
}

// addControlRefusedReceipt adds the receipt of an asset control refusal, a pause or a refused
// transfer, when status is one, for a caller that fails on it. The failure undoes anything
// already moved.
func addControlRefusedReceipt(ctx kapp.KappContext, status transaction.Transaction_TXResultCode) {
	switch status {
	case transaction.Transaction_AssetPaused:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetPaused, process.ErrAssetIsPaused.Error())
	case transaction.Transaction_KDATransferNotAllowed:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldTransferNotAllowed, process.ErrKDATransferNotAllowed.Error())
	}
}

// isControlRefusal reports whether status is an asset control refusal, a pause or a refused
// transfer.
func isControlRefusal(status transaction.Transaction_TXResultCode) bool {
	return status == transaction.Transaction_AssetPaused || status == transaction.Transaction_KDATransferNotAllowed
}

func (m *marketKapp) Claim(sender []byte, tc *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetID() == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderIDMissing, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(tc.GetID())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderNotFound, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	if marketOrder.GetIsClaimed() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldClaimNotAvailable, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(sender, marketOrder.OwnerAddress) && !bytes.Equal(sender, marketOrder.CurrentBidder) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrInvalidValue.Error())
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

	// An escrowed record that does not hold the NFT is never sold (see escrowHoldsNFT): the seller
	// cannot accept its bid, and once the order expires it unwinds as below the reserve.
	held, err := m.escrowHoldsNFT(marketKapp, marketOrder)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if marketOrder.EndTime >= ctx.Block().GetTimestamp() {
		if held && bytes.Equal(sender, marketOrder.OwnerAddress) &&
			marketOrder.ReservePrice > 0 &&
			marketOrder.CurrentBid >= marketOrder.ReservePrice {
			bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
			if err != nil {
				ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
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

		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldClaimNotAvailable, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if held && marketOrder.ReservePrice > 0 && marketOrder.CurrentBid >= marketOrder.ReservePrice {
		bidderAcc, err := m.GetExistingUserAccount(marketOrder.CurrentBidder)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
			return transaction.Transaction_LoadAccountError, err
		}

		// Under FixAuditChangesV6 a sale of a paused asset, or one a direct transfer would
		// refuse, cannot settle, and an expired order can be neither bid on nor cancelled, so
		// failing here would leave the NFT and the bid in escrow until an asset owner, or a
		// marketplace owner, reverses a change: a pause on either asset would hold the other
		// party's asset, a bid escrowed before the fork was never checked, and a later role
		// change or, for an order without a referral snapshot, a repointed marketplace referral
		// address can refuse a bid that passed. Returning both sides to their owners, as below
		// the reserve, is never refused, so the order unwinds instead, without an error
		// receipt. An asset owner or admin who is also the seller or the bidder can use this,
		// by pausing or changing a role or the royalties, to leave an expired auction, and
		// while any issuer's pause lasts, even a short one, either party can claim to unwind
		// instead of settling; the seller could already cancel before the end time, and no
		// value moves to anyone else.
		// The controls run here, before anything moves, as executeBuyMarket runs them. Any
		// other error fails the Claim as settlement would.
		settle := true
		if m.forkController.FixAuditChangesV6() {
			_, _, status, errControls := m.prepareSettlement(ctx, marketOrder, marketOrder.CurrencyID, bidderAcc.AddressBytes())
			switch {
			case isControlRefusal(status):
				settle = false
			case errControls != nil:
				return status, errControls
			}
		}

		if settle {
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
		}
	}

	status, err := m.returnOrderToOwners(ctx, marketKapp, marketOrder)
	if err != nil {
		return status, err
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_SetMarketOrderErr, err
	}

	if err := m.accountsCacher.UpdateKapp(marketKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	return transaction.Transaction_Ok, nil
}

// returnOrderToOwners closes an expired marketOrder without a sale: the NFT and the
// RoyaltiesFixedDeposit return to the order owner and the current bid, if any, to its bidder.
// It moves only what the escrow holds back to whoever put it there, so no asset control can
// refuse it. Claim takes it when the reserve was not met and, under FixAuditChangesV6, when a
// pause or a refused transfer stops the sale; the caller stores the order.
func (m *marketKapp) returnOrderToOwners(ctx kapp.KappContext, marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData) (transaction.Transaction_TXResultCode, error) {
	marketOwnerAcc, err := m.GetExistingUserAccount(marketOrder.OwnerAddress)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	if status, err := m.returnEscrowedNFT(ctx, marketKapp, marketOwnerAcc, marketOrder); err != nil {
		return status, err
	}

	if marketOrder.RoyaltiesFixedDeposit > 0 {
		err = marketOwnerAcc.AddToBalance(marketOrder.RoyaltiesFixedDeposit, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
			return transaction.Transaction_LoadAccountError, err
		}

		err = bidderAcc.AddToBalance(marketOrder.CurrentBid, marketOrder.CurrencyID, m.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) Sell(sender []byte, tc *transaction.SellContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	assetID, currencyID, status, err := m.parseSellContract(ctx, tc)
	if err != nil {
		return status, err
	}

	asset, status, err := m.loadSellableAsset(ctx, assetID[0])
	if err != nil {
		return status, err
	}

	if status, err := m.checkCurrency(ctx, currencyID); err != nil {
		return status, err
	}

	_, marketplace, err := m.GetMarketplace(tc.GetMarketplaceID())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	if asset.Royalties.MarketPercentage+marketplace.ReferralPercentage > core.HundredPercent {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	orderID := m.marketID(ctx, sender)

	marketKapp, _, err := m.GetMarketOrder(orderID)
	if err != nil && !errors.Is(err, common.ErrNotFoundInKApp) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOrder, err.Error())
		return transaction.Transaction_KeyConflict, err
	}

	// From FixAuditChangesV7 an id already in use is refused rather than overwritten, which could
	// strand a live order's escrowed NFT, deposit and bid (KLC-2802).
	if err == nil && m.forkController.FixAuditChangesV7() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOrder, common.ErrInvalidValue.Error())
		return transaction.Transaction_KeyConflict, common.ErrInvalidValue
	}

	reservePrice := int64(0)
	switch kapps.MarketOrderData_EnumMarketType(tc.GetMarketType()) {
	case kapps.MarketOrderData_BuyItNow:
		if tc.GetPrice() == 0 {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPrice, common.ErrInvalidValue.Error())
			return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
		}
	case kapps.MarketOrderData_Auction:
		reservePrice = tc.GetReservePrice()
	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidMarketType, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
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

	// Snapshot the royalty into the order so its economics are fixed at list time and a
	// later UpdateRoyalties cannot change the payout at buy/claim (KLC-2457 /
	// GHSA-p7gw-2pcp-5pf8). Settlement uses this snapshot only when the RoyaltySnapshotted
	// marker is set (see executeBuyMarket); orders without it keep using the live royalty.
	//
	// Gated on FixAuditChangesV3 (the not-yet-live v3 fork): this write changes stored order
	// bytes, so it must activate at a future epoch to stay replay-safe — a re-sync / import-db
	// reindex only writes the snapshot on Sell txs at or after the fork, matching canonical
	// history. FixMarketBuyOverflow is already live and cannot gate a new state write.
	if m.forkController.FixAuditChangesV3() {
		marketOrder.RoyaltyPercentage = asset.Royalties.MarketPercentage
		marketOrder.RoyaltySnapshotted = true
	}

	// Same reasoning for the referral beneficiary: the order already fixes
	// ReferralPercentage at list time, but settlement reloaded the live marketplace to
	// find the address, so an owner could repoint ReferralAddress mid-auction and divert
	// the referral share to a party the bidder never saw. Snapshot the address too, so
	// both halves of the referral payout are decided when the order is listed.
	//
	// Gated on FixAuditChangesV4 rather than V3: V3 already shipped in this audit round
	// with only the royalty fields, so a re-sync must not start writing these bytes at
	// the V3 epoch. Orders listed before V4 carry no marker and keep resolving the live
	// address (see computeReferralAmount).
	if m.forkController.FixAuditChangesV4() {
		marketOrder.ReferralAddress = marketplace.ReferralAddress
		marketOrder.ReferralSnapshotted = true
	}

	if status, err := m.escrowOrder(ctx, marketKapp, marketOrder); err != nil {
		return status, err
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

// parseSellContract checks the fields of tc and returns its asset ID split into collection and
// nonce, and its currency, KLV when unset.
func (m *marketKapp) parseSellContract(ctx kapp.KappContext, tc *transaction.SellContract) ([][]byte, []byte, transaction.Transaction_TXResultCode, error) {
	if tc.GetAssetID() == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrInvalidValue.Error())
		return nil, nil, transaction.Transaction_AssetError, common.ErrInvalidValue
	}

	if tc.GetMarketplaceID() == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, common.ErrInvalidValue.Error())
		return nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	assetID := bytes.Split(tc.GetAssetID(), []byte(kapps.Sp))
	if len(assetID) != 2 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, process.ErrInvalidArgument.Error())
		return nil, nil, transaction.Transaction_AssetTypeInvalid, process.ErrInvalidArgument
	}

	if tc.GetPrice() < 0 || tc.GetReservePrice() < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPrice, common.ErrInvalidValue.Error())
		return nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	oneYearExpiration := time.Unix(ctx.Block().GetTimestamp(), 0).AddDate(1, 0, 0).Unix()
	if tc.GetEndTime() <= ctx.Block().GetTimestamp() || tc.GetEndTime() > oneYearExpiration {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderExpired, common.ErrInvalidValue.Error())
		return nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	if len(kapps.MarketOrderData_EnumMarketType_name[int32(tc.GetMarketType())]) == 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidMarketType, common.ErrInvalidValue.Error())
		return nil, nil, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	return assetID, currencyID, transaction.Transaction_Ok, nil
}

// loadSellableAsset loads collection for Sell, refusing one that is not non-fungible.
func (m *marketKapp) loadSellableAsset(ctx kapp.KappContext, collection []byte) (*kapps.KDAData, transaction.Transaction_TXResultCode, error) {
	_, asset, err := m.KAppController.GetKDAKApp().GetKDA(collection)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return nil, transaction.Transaction_KAPPError, err
	}

	// only allowed to sell non-fungible assets
	if asset.AssetType != kapps.KDAData_NonFungible {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrInvalidValue.Error())
		return nil, transaction.Transaction_AssetTypeInvalid, common.ErrInvalidValue
	}

	// Under FixAuditChangesV6 a paused collection cannot be listed, as it cannot be
	// transferred. Its transfer restriction is not checked here: the buyer is unknown until a
	// bid, and Buy and settlement check the move from the seller to the buyer then.
	if m.forkController.FixAuditChangesV6() {
		status, err := checkNotPaused(asset)
		if err != nil {
			addControlRefusedReceipt(ctx, status)
			return nil, status, err
		}
	}

	return asset, transaction.Transaction_Ok, nil
}

// escrowOrder opens marketOrder: the NFT and the RoyaltiesFixedDeposit move from the order owner
// into the market escrow, and the order is stored. It is the reverse of returnOrderToOwners.
func (m *marketKapp) escrowOrder(ctx kapp.KappContext, marketKapp state.KAppAccountHandler, marketOrder *kapps.MarketOrderData) (transaction.Transaction_TXResultCode, error) {
	ownerAcc, err := m.GetExistingUserAccount(marketOrder.OwnerAddress)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	if status, err := m.checkNotEscrowed(ctx, marketKapp, marketOrder.CollectionID, marketOrder.AssetID); err != nil {
		return status, err
	}

	data, err := ownerAcc.SubInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, m.forkController)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	err = marketKapp.AddInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, data)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		ownerAcc.AddressBytes(),
		kapps.MarketKAppAddress,
		[]byte("1"),
		marketOrder.CollectionID,
		marketOrder.AssetID,
		[]byte{byte(kapps.KDAData_NonFungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	if marketOrder.RoyaltiesFixedDeposit > 0 {
		err = ownerAcc.SubFromBalance(marketOrder.RoyaltiesFixedDeposit, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, err.Error())
			return transaction.Transaction_BalanceError, err
		}

		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.Transfer,
			ctx.ContractID(),
			ownerAcc.AddressBytes(),
			kapps.MarketKAppAddress,
			[]byte(strconv.FormatInt(marketOrder.RoyaltiesFixedDeposit, 10)),
			kdautils.KLVIdentifier,
			nil,
			[]byte{byte(kapps.KDAData_Fungible)},
			marketOrder.MarketplaceID,
			marketOrder.ID,
		))
	}

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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

// marketID derives an order or marketplace id for sender. From FixAuditChangesV7 it also hashes the
// tx hash, so two users calling one contract in a block at the same nonce get distinct ids, and the
// index of the id in the transaction, so two ids derived in one transaction do too (KLC-2802).
func (m *marketKapp) marketID(ctx kapp.KappContext, sender []byte) []byte {
	if !m.forkController.FixAuditChangesV7() {
		return kdautils.ToMarketID(m.hasher, ctx.Block().GetRandSeed(), sender, ctx.TxNonce(), ctx.ContractID(), kdautils.MarketKeyLength)
	}

	index := binary.BigEndian.AppendUint64(nil, ctx.NextIDIndex())
	return kdautils.ToMarketID(m.hasher, ctx.Block().GetRandSeed(), sender, ctx.TxNonce(), ctx.ContractID(), kdautils.MarketKeyLength, ctx.TxHash(), index)
}

// checkNotEscrowed refuses, from FixAuditChangesV7, listing the NFT nonce of collection while the
// escrow already holds it: a second record at the same key, left by a spend through the fungible
// balance path, would otherwise overwrite the first order's escrow (KLC-2835).
func (m *marketKapp) checkNotEscrowed(ctx kapp.KappContext, marketKapp state.KAppAccountHandler, collection []byte, nonce []byte) (transaction.Transaction_TXResultCode, error) {
	if !m.forkController.FixAuditChangesV7() {
		return transaction.Transaction_Ok, nil
	}

	value, err := readEscrow(marketKapp, collection, nonce)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}
	if len(value) > 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOrder, common.ErrInvalidValue.Error())
		return transaction.Transaction_KeyConflict, common.ErrInvalidValue
	}

	return transaction.Transaction_Ok, nil
}

// readEscrow reads the escrow's record of the NFT nonce of collection, nil when it holds none. A key
// cleared earlier in the block reads back as ErrNegativeValue (pinned in trackableDataTrie_test.go),
// and one never written on a KApp with no trie yet as ErrNilTrie: neither holds a record.
func readEscrow(marketKapp state.KAppAccountHandler, collection []byte, nonce []byte) ([]byte, error) {
	value, err := marketKapp.DataTrieTracker().RetrieveValue(kdautils.ToKDAKey(collection, nonce))
	if errors.Is(err, common.ErrNilTrie) || errors.Is(err, common.ErrNegativeValue) {
		return nil, nil
	}

	return value, err
}

// returnEscrowedNFT moves marketOrder's escrowed record back to ownerAcc, the order owner. From
// FixAuditChangesV7 an escrow that holds none, cleared by another order on the same key
// (KLC-2835), returns nothing, so the order still unwinds and its bid and deposit go back.
func (m *marketKapp) returnEscrowedNFT(ctx kapp.KappContext, marketKapp state.KAppAccountHandler, ownerAcc state.UserAccountHandler, marketOrder *kapps.MarketOrderData) (transaction.Transaction_TXResultCode, error) {
	if m.forkController.FixAuditChangesV7() {
		value, err := readEscrow(marketKapp, marketOrder.CollectionID, marketOrder.AssetID)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
			return transaction.Transaction_KAPPError, err
		}
		if len(value) == 0 {
			return transaction.Transaction_Ok, nil
		}
	}

	data, err := marketKapp.SubInternalKDA(marketOrder.CollectionID, marketOrder.AssetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	err = ownerAcc.AddInternalKDA(marketOrder.CollectionID, marketOrder.AssetID, data)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_AssetError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		marketKapp.AddressBytes(),
		ownerAcc.AddressBytes(),
		[]byte("1"),
		marketOrder.CollectionID,
		marketOrder.AssetID,
		[]byte{byte(kapps.KDAData_NonFungible)},
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	return transaction.Transaction_Ok, nil
}

func (m *marketKapp) CancelOrder(sender []byte, tc *transaction.CancelMarketOrderContract) (transaction.Transaction_TXResultCode, error) {
	ctx := m.KAppController.GetCurrentKAppContext()

	if tc.GetOrderID() == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderIDMissing, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketKapp, marketOrder, err := m.GetMarketOrder(tc.GetOrderID())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderNotFound, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	if marketOrder.IsClaimed {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOrder, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if !bytes.Equal(marketOrder.OwnerAddress, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if marketOrder.EndTime <= ctx.Block().GetTimestamp() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldOrderExpired, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	ownerAcc, err := m.GetExistingUserAccount(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	if marketOrder.RoyaltiesFixedDeposit > 0 {
		err = ownerAcc.AddToBalance(marketOrder.RoyaltiesFixedDeposit, kdautils.KLVIdentifier, m.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
			return transaction.Transaction_LoadAccountError, err
		}

		err = bidderAcc.AddToBalance(marketOrder.CurrentBid, marketOrder.CurrencyID, m.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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

	if status, err := m.returnEscrowedNFT(ctx, marketKapp, ownerAcc, marketOrder); err != nil {
		return status, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.CancelOrder,
		ctx.ContractID(),
		marketOrder.MarketplaceID,
		marketOrder.ID,
	))

	err = m.SetMarketOrder(marketKapp, marketOrder)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidName, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	referralAddress := sender
	if len(tc.GetReferralAddress()) > 0 {
		if len(tc.GetReferralAddress()) != m.pubkeyConv.Len() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidToAddress, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		referralAddress = tc.GetReferralAddress()
	}

	if tc.GetReferralPercentage() > core.HundredPercent {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketplaceID := m.marketID(ctx, sender)

	marketplaceKapp, _, err := m.GetMarketplace(marketplaceID)
	if err != nil && !errors.Is(err, common.ErrNotFoundInKApp) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_KeyConflict, err
	}

	// From FixAuditChangesV7 an id already in use is refused rather than overwritten (KLC-2802).
	if err == nil && m.forkController.FixAuditChangesV7() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, common.ErrInvalidValue.Error())
		return transaction.Transaction_KeyConflict, common.ErrInvalidValue
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetReferralPercentage() > core.HundredPercent {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	marketplaceKapp, marketplace, err := m.GetMarketplace(tc.GetMarketplaceID())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	if !bytes.Equal(marketplace.OwnerAddress, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetName() != nil {
		if !utf8.Valid(tc.GetName()) ||
			len(tc.GetName()) > core.MaxNameSize {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidName, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		marketplace.Name = tc.GetName()
	}

	// Settlement reloads the live marketplace, so an empty address would otherwise strand
	// every open order that snapshotted a positive referral percentage. executeBuyMarket
	// tolerates the marketplaces already cleared before the fork; this gate keeps new ones
	// from appearing, so the referral share is not silently rerouted to the order owner.
	// Since proto3 cannot tell an explicitly empty bytes field from an omitted one, an
	// address can no longer be cleared.
	if m.forkController.FixAuditChangesV4() {
		if len(tc.GetReferralAddress()) > 0 {
			marketplace.ReferralAddress = tc.GetReferralAddress()
		}

		if tc.GetReferralPercentage() > 0 && len(marketplace.ReferralAddress) == 0 {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	} else {
		marketplace.ReferralAddress = tc.GetReferralAddress()
	}

	// The two referral fields do not behave alike: the address above is sticky post-fork,
	// while the percentage is always overwritten, omitted or not. A config that only renames
	// the marketplace therefore still drops referral payouts to zero.
	marketplace.ReferralPercentage = tc.GetReferralPercentage()

	err = m.SetMarketplace(marketplaceKapp, marketplace)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	if err := m.accountsCacher.UpdateKapp(marketplaceKapp); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMarketplaceError, err.Error())
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.ConfigMarketplace,
		ctx.ContractID(),
		tc.GetMarketplaceID(),
	))

	return transaction.Transaction_Ok, nil
}
