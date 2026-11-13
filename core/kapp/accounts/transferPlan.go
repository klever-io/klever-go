package accounts

import (
	"bytes"
	"encoding/hex"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
)

// Once FixAuditChangesV7 is active a transfer runs in two phases. planTransfer makes every check
// the transfer needs and works out every amount it moves, without mutating any account.
// applyTransferPlan then moves those amounts and checks nothing but the mutations themselves.
//
// planTransfer makes the checks in the order transferLegacy makes them, so the first failing check,
// its result code, error and error receipt are the ones transferLegacy reports. transferLegacy
// interleaves moves between those checks; a transferTally stands in for them, so a check still sees
// the sender balance the earlier moves would have left.
//
// A plan holds addresses and amounts, never loaded accounts: applyTransferPlan loads them again.
// With the accounts cache off every load is a fresh copy, and applying a plan to an account loaded
// before an earlier plan was applied would overwrite what that plan moved.

// transferTally is what the plans already made in a batch take from the sender: the net amount by
// kdautils.ToKDAKey (credits back to the sender count negative) and the non-fungible units sent.
type transferTally struct {
	spent     map[string]int64
	sentUnits map[string]struct{}
}

// newTransferTally returns an empty tally for a new batch.
func newTransferTally() *transferTally {
	return &transferTally{
		spent:     make(map[string]int64),
		sentUnits: make(map[string]struct{}),
	}
}

// available is what is left of balance under key once the plans already recorded take their share.
func (t *transferTally) available(balance int64, key []byte) int64 {
	return balance - t.spent[string(key)]
}

// debit records amount taken from the sender under key.
func (t *transferTally) debit(key []byte, amount int64) {
	t.spent[string(key)] += amount
}

// credit records amount paid back to the sender under key.
func (t *transferTally) credit(key []byte, amount int64) {
	t.spent[string(key)] -= amount
}

// royaltyPayout is one royalty credit, receipted with internalID.
type royaltyPayout struct {
	address    []byte
	amount     int64
	internalID []byte
}

// royaltyPlan is a royalty charged to the sender in assetID and paid out to splits and a receiver.
type royaltyPlan struct {
	assetID   []byte
	assetType kapps.KDAData_EnumAssetType
	amount    int64
	// debitFirst charges the sender before paying the splits; otherwise it is charged just before
	// the receiver is paid, and not at all when the splits take the whole royalty.
	debitFirst bool
	// saveSender hands the sender to the cacher right after it is charged.
	saveSender bool
	splits     []royaltyPayout
	receiver   *royaltyPayout
}

// transferPlan is everything one transfer moves.
type transferPlan struct {
	sender     []byte
	receiver   []byte
	assetID    []byte
	internalID []byte
	assetType  kapps.KDAData_EnumAssetType
	amount     int64
	fixed      *royaltyPlan
	percentage *royaltyPlan
}

// ValidateTransfers checks, without moving anything, that TransferBatch would accept every transfer
// in tcs. It returns the result code and error of the first transfer rejected, and emits the error
// receipt TransferBatch would emit for it.
func (a *accountsKapp) ValidateTransfers(cType transaction.TXContract_ContractType, sender []byte, tcs []*transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
	_, resultCode, err := a.planTransfers(cType, sender, tcs)
	return resultCode, err
}

// TransferBatch moves every transfer in tcs from sender, in order, once all of them have passed
// their checks, so a transfer that fails a check leaves every other one unapplied. The checks see
// the sender balance the earlier transfers of the batch leave, royalties included.
//
// What the checks cannot foresee (a receiver balance overflow, an account the cacher fails to load
// or save) fails while moving value, possibly after earlier transfers of the batch were applied.
// The reported error can then differ from the per-transfer path: that path fails on the move
// concerned, while the batch first fails any check that comes after it and does not pass, in a
// later transfer or later in the same one (a royalty credit that overflows, ahead of an amount the
// sender cannot cover). Either way the caller fails, and the transaction aborts and is reset.
func (a *accountsKapp) TransferBatch(cType transaction.TXContract_ContractType, sender []byte, tcs []*transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
	plans, resultCode, err := a.planTransfers(cType, sender, tcs)
	if err != nil {
		return resultCode, err
	}

	for _, plan := range plans {
		resultCode, err = a.applyTransferPlan(plan)
		if err != nil {
			return resultCode, err
		}
	}

	return transaction.Transaction_Ok, nil
}

// planTransfers plans every transfer of tcs in order against one tally, and returns the result
// code and error of the first one refused.
func (a *accountsKapp) planTransfers(cType transaction.TXContract_ContractType, sender []byte, tcs []*transaction.TransferContract) ([]*transferPlan, transaction.Transaction_TXResultCode, error) {
	tally := newTransferTally()
	plans := make([]*transferPlan, 0, len(tcs))

	for _, tc := range tcs {
		plan, resultCode, err := a.planTransfer(cType, sender, tc, tally)
		if err != nil {
			return nil, resultCode, err
		}
		plans = append(plans, plan)
	}

	return plans, transaction.Transaction_Ok, nil
}

// planTransfer makes the checks of transferLegacy for one transfer, against the sender balance left
// by the plans already recorded in tally, and records this transfer in tally.
func (a *accountsKapp) planTransfer(
	cType transaction.TXContract_ContractType,
	sender []byte,
	tc *transaction.TransferContract,
	tally *transferTally,
) (*transferPlan, transaction.Transaction_TXResultCode, error) {
	target, resultCode, err := a.resolveTransfer(cType, sender, tc)
	if err != nil {
		return nil, resultCode, err
	}

	plan := &transferPlan{
		sender:     target.acntSrc.AddressBytes(),
		receiver:   target.acntDst.AddressBytes(),
		assetID:    target.assetID,
		internalID: target.internalID,
		assetType:  target.kda.AssetType,
		amount:     tc.GetAmount(),
	}

	plan.fixed, resultCode, err = a.planFixedRoyalty(tc, target, tally)
	if err != nil {
		return nil, resultCode, err
	}

	switch target.kda.AssetType {
	case kapps.KDAData_NonFungible:
		resultCode, err = a.planNonFungibleTransfer(tc, target, tally)
	case kapps.KDAData_SemiFungible:
		plan.percentage, resultCode, err = a.planPercentageRoyalty(tc, target, tally)
		if err == nil {
			resultCode, err = a.planSemiFungibleTransfer(tc, target, tally)
		}
	case kapps.KDAData_Fungible:
		plan.percentage, resultCode, err = a.planPercentageRoyalty(tc, target, tally)
		if err == nil {
			resultCode, err = a.planFungibleTransfer(tc, target, tally)
		}
	default:
		resultCode, err = a.rejectTransfer(transaction.Transaction_AssetTypeInvalid, common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid)
	}
	if err != nil {
		return nil, resultCode, err
	}

	return plan, transaction.Transaction_Ok, nil
}

// planFixedRoyalty makes the checks of processFixedRoyaltiesTransfer.
func (a *accountsKapp) planFixedRoyalty(tc *transaction.TransferContract, target *resolvedTransfer, tally *transferTally) (*royaltyPlan, transaction.Transaction_TXResultCode, error) {
	acntSrc, kda := target.acntSrc, target.kda
	if core.IsSmartContractAddress(acntSrc.AddressBytes()) ||
		kda.Royalties == nil ||
		kda.Royalties.TransferFixed <= 0 {
		return nil, transaction.Transaction_Ok, nil
	}

	cdd := a.forkController.EnableSmartContracts()
	fixed := kda.Royalties.TransferFixed

	if cdd && tc.KLVRoyalties != fixed {
		resultCode, err := a.rejectTransfer(transaction.Transaction_ParameterInvalid, common.ErrFieldInvalidRoyalties, common.ErrInvalidValue)
		return nil, resultCode, err
	}

	klvKey := kdautils.ToKDAKey(kdautils.KLVIdentifier, nil)
	if tally.available(acntSrc.GetBalance(kdautils.KLVIdentifier, cdd), klvKey) < fixed {
		resultCode, err := a.rejectTransfer(transaction.Transaction_OutOfFunds, common.ErrFieldInsufficientFunds, process.ErrInsufficientFunds)
		return nil, resultCode, err
	}
	tally.debit(klvKey, fixed)

	plan := &royaltyPlan{
		assetID:    kdautils.KLVIdentifier,
		assetType:  kapps.KDAData_Fungible,
		amount:     fixed,
		debitFirst: true,
		saveSender: true,
	}

	toPay := fixed
	splits, resultCode, err := a.planSplitRoyalties(
		kda.Royalties.SplitRoyalties,
		fixed,
		(*kapps.RoyaltySplitData).GetPercentTransferFixed,
		&toPay,
		func(split royaltyPayout) { creditSender(tally, acntSrc, klvKey, split) },
	)
	if err != nil {
		return nil, resultCode, err
	}
	plan.splits = splits

	if toPay <= 0 {
		return plan, transaction.Transaction_Ok, nil
	}

	royaltyOwner, err := a.LoadUserAccount(kda.Royalties.Address)
	if !a.forkController.KdaFpr() {
		royaltyOwner, err = a.GetExistingUserAccount(kda.OwnerAddress)
	}
	if err != nil {
		resultCode, err = a.rejectTransfer(transaction.Transaction_LoadAccountError, common.ErrFieldLoadRoyaltyAccount, err)
		return nil, resultCode, err
	}

	plan.receiver = &royaltyPayout{address: royaltyOwner.AddressBytes(), amount: toPay}
	creditSender(tally, acntSrc, klvKey, *plan.receiver)

	return plan, transaction.Transaction_Ok, nil
}

// planPercentageRoyalty makes the checks of processPercentageRoyaltiesTransfer. The royalty is
// charged to and checked against the nonce-less balance of the asset.
func (a *accountsKapp) planPercentageRoyalty(tc *transaction.TransferContract, target *resolvedTransfer, tally *transferTally) (*royaltyPlan, transaction.Transaction_TXResultCode, error) {
	acntSrc, assetID, kda := target.acntSrc, target.assetID, target.kda
	royaltyKey := kdautils.ToKDAKey(assetID, nil)

	royaltyAmount, resultCode, err := a.validatePercentageRoyaltiesTransfer(tc, kda, acntSrc, assetID, tally.spent[string(royaltyKey)])
	if err != nil {
		return nil, resultCode, err
	}

	if royaltyAmount <= 0 {
		return nil, transaction.Transaction_Ok, nil
	}

	if royaltyAmount != tc.GetKDARoyalties() {
		resultCode, err = a.rejectTransfer(transaction.Transaction_ParameterInvalid, common.ErrFieldInvalidRoyalties, common.ErrInvalidValue)
		return nil, resultCode, err
	}

	plan := &royaltyPlan{
		assetID:    assetID,
		assetType:  kda.AssetType,
		amount:     royaltyAmount,
		debitFirst: a.forkController.FixMarketBuyOverflow(),
	}
	if plan.debitFirst {
		if resultCode, err = a.chargeRoyaltyToTally(acntSrc, assetID, royaltyKey, royaltyAmount, tally); err != nil {
			return nil, resultCode, err
		}
	}

	toPay := royaltyAmount
	plan.splits, resultCode, err = a.planSplitRoyalties(
		kda.Royalties.SplitRoyalties,
		royaltyAmount,
		(*kapps.RoyaltySplitData).GetPercentTransferPercentage,
		&toPay,
		func(split royaltyPayout) { creditSender(tally, acntSrc, royaltyKey, split) },
	)
	if err != nil {
		return nil, resultCode, err
	}

	if toPay <= 0 {
		return plan, transaction.Transaction_Ok, nil
	}

	royaltyReceiver, err := a.LoadUserAccount(kda.Royalties.Address)
	if err != nil {
		resultCode, err = a.rejectTransfer(transaction.Transaction_LoadAccountError, common.ErrFieldLoadRoyaltyAccount, err)
		return nil, resultCode, err
	}

	if !plan.debitFirst {
		if resultCode, err = a.chargeRoyaltyToTally(acntSrc, assetID, royaltyKey, royaltyAmount, tally); err != nil {
			return nil, resultCode, err
		}
	}

	plan.receiver = &royaltyPayout{address: royaltyReceiver.AddressBytes(), amount: toPay, internalID: target.internalID}
	creditSender(tally, acntSrc, royaltyKey, *plan.receiver)

	return plan, transaction.Transaction_Ok, nil
}

// chargeRoyaltyToTally makes the check of the SubFromBalance that charges a percentage royalty to
// the sender, then records the charge in tally. validatePercentageRoyaltiesTransfer has already
// checked the balance covers the amount and the royalty, but that sum can overflow int64, so
// SubFromBalance is the check that refuses it: with a BalanceError, state.ErrInsufficientFunds and
// the BalanceError receipt the legacy path adds for it.
func (a *accountsKapp) chargeRoyaltyToTally(
	acntSrc state.UserAccountHandler,
	assetID, royaltyKey []byte,
	royaltyAmount int64,
	tally *transferTally,
) (transaction.Transaction_TXResultCode, error) {
	balance := acntSrc.GetBalance(assetID, a.forkController.EnableSmartContracts())
	if tally.available(balance, royaltyKey) < royaltyAmount {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, state.ErrInsufficientFunds)
	}
	tally.debit(royaltyKey, royaltyAmount)

	return transaction.Transaction_Ok, nil
}

// planSplitRoyalties makes the checks of paySplitRoyalties: each split in sorted key order, as
// paySplitRoyalties pays them, so the first split refused and the receipt order are the same.
// recorded is called with each split once it is planned.
func (a *accountsKapp) planSplitRoyalties(
	splits map[string]*kapps.RoyaltySplitData,
	value int64,
	percentOf func(*kapps.RoyaltySplitData) uint32,
	toPay *int64,
	recorded func(royaltyPayout),
) ([]royaltyPayout, transaction.Transaction_TXResultCode, error) {
	payouts := make([]royaltyPayout, 0, len(splits))
	for _, key := range types.OrderedKeys(splits, true) {
		split, resultCode, err := a.planSplitRoyalty(key, value, int64(percentOf(splits[key])), toPay)
		if err != nil {
			return nil, resultCode, err
		}
		payouts = append(payouts, split)
		recorded(split)
	}

	return payouts, transaction.Transaction_Ok, nil
}

// planSplitRoyalty makes the checks of computeSplitRoyalties and takes the split out of toPay.
func (a *accountsKapp) planSplitRoyalty(address string, value int64, percentage int64, toPay *int64) (royaltyPayout, transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	decodedAddress, err := hex.DecodeString(address)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidSplitRoyalties, err.Error())
		return royaltyPayout{}, transaction.Transaction_LoadAccountError, err
	}

	splitRoyalty, err := a.LoadUserAccount(decodedAddress)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadRoyaltyAccount, err.Error())
		return royaltyPayout{}, transaction.Transaction_LoadAccountError, err
	}

	splitToPay, err := tools.ComputePercentageI64(value, percentage, a.forkController.EnableSmartContracts())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, err.Error())
		return royaltyPayout{}, transaction.Transaction_ParameterInvalid, err
	}
	if a.forkController.FixMarketBuyOverflow() && splitToPay > *toPay {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoyalties, common.ErrInvalidValue.Error())
		return royaltyPayout{}, transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}
	*toPay -= splitToPay

	return royaltyPayout{address: splitRoyalty.AddressBytes(), amount: splitToPay}, transaction.Transaction_Ok, nil
}

// creditSender records a royalty paid back to the sender, which a later check may spend.
func creditSender(tally *transferTally, acntSrc state.UserAccountHandler, key []byte, payout royaltyPayout) {
	if bytes.Equal(payout.address, acntSrc.AddressBytes()) {
		tally.credit(key, payout.amount)
	}
}

// planNonFungibleTransfer makes the checks of the non-fungible branch of transferLegacy and of
// SubInternalKDA in processNonFungibleTransfer.
func (a *accountsKapp) planNonFungibleTransfer(tc *transaction.TransferContract, target *resolvedTransfer, tally *transferTally) (transaction.Transaction_TXResultCode, error) {
	if a.forkController.FixAuditChangesV3() && tc.Amount != 1 {
		ctx := a.KAppController.GetCurrentKAppContext()
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	unitKey := kdautils.ToKDAKey(target.assetID, target.internalID)
	if _, sent := tally.sentUnits[string(unitKey)]; sent {
		// SubInternalKDA reads back the nil value the earlier send left in the dirty data.
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, common.ErrNegativeValue)
	}

	data, err := target.acntSrc.RetrieveValue(unitKey)
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, err)
	}
	if len(data) == 0 || a.forkController.FixAuditChangesV7() && !state.IsHeldNFTRecord(data) {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, common.ErrAssetNotFound)
	}
	tally.sentUnits[string(unitKey)] = struct{}{}

	return transaction.Transaction_Ok, nil
}

// planSemiFungibleTransfer makes the checks of processSemiFungibleTransfer, including the nonce
// checks of SubFromBalanceWithNonce.
func (a *accountsKapp) planSemiFungibleTransfer(tc *transaction.TransferContract, target *resolvedTransfer, tally *transferTally) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()
	cdd := a.forkController.EnableSmartContracts()

	if !cdd {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid.Error())
		return transaction.Transaction_AssetTypeInvalid, common.ErrAssetTypeInvalid
	}

	key := kdautils.ToKDAKey(target.assetID, target.internalID)
	balance := target.acntSrc.GetBalanceWithNonce(target.assetID, target.internalID, cdd)
	resultCode, err := a.planAmount(tc, balance, key, tally)
	if err != nil {
		return resultCode, err
	}

	nonce, err := strconv.ParseInt(string(target.internalID), 10, 64)
	if err == nil && nonce <= 0 {
		err = state.ErrInvalidNonce
	}
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBalanceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	tally.debit(key, tc.GetAmount())

	return transaction.Transaction_Ok, nil
}

// planFungibleTransfer makes the checks of processFungibleTransfer.
func (a *accountsKapp) planFungibleTransfer(tc *transaction.TransferContract, target *resolvedTransfer, tally *transferTally) (transaction.Transaction_TXResultCode, error) {
	key := kdautils.ToKDAKey(target.assetID, nil)
	balance := target.acntSrc.GetBalance(target.assetID, a.forkController.EnableSmartContracts())
	resultCode, err := a.planAmount(tc, balance, key, tally)
	if err != nil {
		return resultCode, err
	}

	tally.debit(key, tc.GetAmount())

	return transaction.Transaction_Ok, nil
}

// planAmount makes the amount and balance checks shared by processFungibleTransfer and
// processSemiFungibleTransfer.
func (a *accountsKapp) planAmount(tc *transaction.TransferContract, balance int64, key []byte, tally *transferTally) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	value := tc.GetAmount()
	if value <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ContractInvalid, common.ErrInvalidValue
	}

	if tally.available(balance, key) < value {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, process.ErrInsufficientFunds.Error())
		return transaction.Transaction_OutOfFunds, process.ErrInsufficientFunds
	}

	return transaction.Transaction_Ok, nil
}

// applyTransferPlan moves what plan records, with the mutations, cacher updates and receipts of
// transferLegacy in the same order.
func (a *accountsKapp) applyTransferPlan(plan *transferPlan) (transaction.Transaction_TXResultCode, error) {
	acntSrc, acntDst, resultCode, err := a.loadPlanAccounts(plan)
	if err != nil {
		return resultCode, err
	}

	// The fixed royalty is paid first, then the percentage one; a non-fungible plan has none.
	for _, royalty := range []*royaltyPlan{plan.fixed, plan.percentage} {
		if royalty == nil {
			continue
		}
		resultCode, err = a.applyRoyaltyPlan(royalty, acntSrc)
		if err != nil {
			return resultCode, err
		}
	}

	switch plan.assetType {
	case kapps.KDAData_NonFungible:
		return a.applyNonFungibleTransfer(plan, acntSrc, acntDst)
	case kapps.KDAData_SemiFungible:
		return a.applySemiFungibleTransfer(plan, acntSrc, acntDst)
	case kapps.KDAData_Fungible:
		return a.applyFungibleTransfer(plan, acntSrc, acntDst)
	default:
		return a.rejectTransfer(transaction.Transaction_AssetTypeInvalid, common.ErrFieldInvalidAssetType, common.ErrAssetTypeInvalid)
	}
}

// loadPlanAccounts loads the plan's sender and receiver, with the error receipts
// validateAndLoadAccounts emits when either fails.
func (a *accountsKapp) loadPlanAccounts(plan *transferPlan) (state.UserAccountHandler, state.UserAccountHandler, transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	acntSrc, err := a.LoadUserAccount(plan.sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadSenderAccount, err.Error())
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	acntDst, err := a.LoadUserAccount(plan.receiver)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadReceiverAccount, err.Error())
		return nil, nil, transaction.Transaction_LoadAccountError, err
	}

	return acntSrc, acntDst, transaction.Transaction_Ok, nil
}

// applyRoyaltyPlan charges the sender and pays the splits and the receiver, as
// processFixedRoyaltiesTransfer and processPercentageRoyaltiesTransfer do.
func (a *accountsKapp) applyRoyaltyPlan(plan *royaltyPlan, acntSrc state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	if plan.debitFirst {
		resultCode, err := a.chargeRoyalty(plan, acntSrc)
		if err != nil {
			return resultCode, err
		}
	}

	for _, split := range plan.splits {
		resultCode, err := a.payRoyalty(plan, split, acntSrc)
		if err != nil {
			return resultCode, err
		}
	}

	if plan.receiver == nil {
		return transaction.Transaction_Ok, nil
	}

	if !plan.debitFirst {
		resultCode, err := a.chargeRoyalty(plan, acntSrc)
		if err != nil {
			return resultCode, err
		}
	}

	return a.payRoyalty(plan, *plan.receiver, acntSrc)
}

// chargeRoyalty takes the royalty from the sender, and hands the sender to the cacher when the
// plan says to.
func (a *accountsKapp) chargeRoyalty(plan *royaltyPlan, acntSrc state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	err := acntSrc.SubFromBalance(plan.amount, plan.assetID, a.forkController.EnableSmartContracts())
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, err)
	}

	if plan.saveSender {
		if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
			return a.rejectTransfer(transaction.Transaction_SaveAccountError, common.ErrFieldSaveAccountError, err)
		}
	}

	return transaction.Transaction_Ok, nil
}

// payRoyalty credits payout to its account, hands the account to the cacher and adds the transfer
// receipt from the sender. The legacy split payment uses it too.
func (a *accountsKapp) payRoyalty(plan *royaltyPlan, payout royaltyPayout, acntSrc state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()

	payee, err := a.LoadUserAccount(payout.address)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldLoadRoyaltyAccount, err.Error())
		return transaction.Transaction_LoadAccountError, err
	}

	err = payee.AddToBalance(payout.amount, plan.assetID, a.forkController.EnableSmartContracts())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBalanceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	if err := a.accountsCacher.UpdateUser(payee); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldSaveAccountError, err.Error())
		return transaction.Transaction_SaveAccountError, err
	}

	a.addTransferReceipt(acntSrc, payee, []byte(strconv.FormatInt(payout.amount, 10)), plan.assetID, payout.internalID, plan.assetType)

	return transaction.Transaction_Ok, nil
}

// applyNonFungibleTransfer moves the NFT record from the sender to the receiver, as
// processNonFungibleTransfer does.
func (a *accountsKapp) applyNonFungibleTransfer(plan *transferPlan, acntSrc, acntDst state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	data, err := acntSrc.SubInternalKDA(plan.assetID, plan.internalID, a.forkController)
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, err)
	}

	if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
		return a.rejectTransfer(transaction.Transaction_SaveAccountError, common.ErrFieldSaveAccountError, err)
	}

	err = acntDst.AddInternalKDA(plan.assetID, plan.internalID, data)
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_AssetError, common.ErrFieldAssetError, err)
	}

	if err := a.accountsCacher.UpdateUser(acntDst); err != nil {
		return a.rejectTransfer(transaction.Transaction_SaveAccountError, common.ErrFieldSaveAccountError, err)
	}

	a.addTransferReceipt(acntSrc, acntDst, []byte("1"), plan.assetID, plan.internalID, plan.assetType)

	return transaction.Transaction_Ok, nil
}

// applySemiFungibleTransfer moves the SFT amount at its nonce, as processSemiFungibleTransfer does.
func (a *accountsKapp) applySemiFungibleTransfer(plan *transferPlan, acntSrc, acntDst state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()
	cdd := a.forkController.EnableSmartContracts()

	err := acntSrc.SubFromBalanceWithNonce(plan.amount, plan.assetID, plan.internalID, cdd)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBalanceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	err = acntDst.AddToBalanceWithNonce(plan.amount, plan.assetID, plan.internalID, cdd)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBalanceError, err.Error())
		return transaction.Transaction_BalanceError, err
	}

	a.addTransferReceipt(acntSrc, acntDst, []byte(strconv.FormatInt(plan.amount, 10)), plan.assetID, plan.internalID, plan.assetType)

	return transaction.Transaction_Ok, nil
}

// applyFungibleTransfer moves the fungible amount, as processFungibleTransfer does.
func (a *accountsKapp) applyFungibleTransfer(plan *transferPlan, acntSrc, acntDst state.UserAccountHandler) (transaction.Transaction_TXResultCode, error) {
	cdd := a.forkController.EnableSmartContracts()

	err := acntSrc.SubFromBalance(plan.amount, plan.assetID, cdd)
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, err)
	}

	if err := a.accountsCacher.UpdateUser(acntSrc); err != nil {
		return a.rejectTransfer(transaction.Transaction_SaveAccountError, common.ErrFieldSaveAccountError, err)
	}

	err = acntDst.AddToBalance(plan.amount, plan.assetID, cdd)
	if err != nil {
		return a.rejectTransfer(transaction.Transaction_BalanceError, common.ErrFieldBalanceError, err)
	}

	if err := a.accountsCacher.UpdateUser(acntDst); err != nil {
		return a.rejectTransfer(transaction.Transaction_SaveAccountError, common.ErrFieldSaveAccountError, err)
	}

	a.addTransferReceipt(acntSrc, acntDst, []byte(strconv.FormatInt(plan.amount, 10)), plan.assetID, nil, plan.assetType)

	return transaction.Transaction_Ok, nil
}

// rejectTransfer adds the error receipt naming field and err, as the legacy transfer path does for
// the same failure, and returns resultCode and err.
func (a *accountsKapp) rejectTransfer(resultCode transaction.Transaction_TXResultCode, field string, err error) (transaction.Transaction_TXResultCode, error) {
	ctx := a.KAppController.GetCurrentKAppContext()
	ctx.Receipts().AddError(ctx.ContractID(), field, err.Error())

	return resultCode, err
}

// addTransferReceipt adds the transfer receipt of amount of assetID (internalID for an NFT or SFT
// unit) moved from acntSrc to acntDst.
func (a *accountsKapp) addTransferReceipt(
	acntSrc, acntDst state.UserAccountHandler,
	amount, assetID, internalID []byte,
	assetType kapps.KDAData_EnumAssetType,
) {
	ctx := a.KAppController.GetCurrentKAppContext()
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		acntSrc.AddressBytes(),
		acntDst.AddressBytes(),
		amount,
		assetID,
		internalID,
		//nolint:gosec // G115: KDAData_EnumAssetType values are 0-2
		[]byte{byte(assetType)},
	))
}
