package kdafeespool

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

var _ kapp.KDAFeesPoolKapp = (*kdaFeesPoolKApp)(nil)

func (v *kdaFeesPoolKApp) UpdatePool(poolID []byte, assetOwner []byte, sender []byte, info *transaction.KDAPoolInfo) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, poolID)
	if err != nil && err != common.ErrAssetPoolNotFound {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if pool == nil {
		if !kapps.MatchesAddress(assetOwner, sender) {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		pool = &KDAFeesPoolData{
			OwnerAddress: sender,
			KDA:          poolID,
			AdminAddress: sender,
		}
	}

	if !kapps.MatchesAddress(pool.AdminAddress, sender) && !kapps.MatchesAddress(assetOwner, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(info.AdminAddress) != 0 {
		pool.AdminAddress = info.AdminAddress
	}

	if len(pool.AdminAddress) != v.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, common.ErrAssetPoolInvalidAddress.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAddress
	}

	if v.invalidFeeRatio(info) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrAssetPoolInvalidAmount.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAmount
	}

	pool.Active = info.Active
	pool.FRatioKLV = info.FRatioKLV
	pool.FRatioKDA = info.FRatioKDA

	err = v.setPool(app, poolID, pool)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	// add receipt
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDAPool,
		ctx.ContractID(),
		poolID,
	))

	return transaction.Transaction_Ok, nil
}

func (v *kdaFeesPoolKApp) invalidFeeRatio(info *transaction.KDAPoolInfo) bool {
	if v.forkController.FixAuditChangesV3() {
		return info.FRatioKLV <= 0 || info.FRatioKDA <= 0
	}

	return info.FRatioKLV == 0 || info.FRatioKDA == 0
}

// ChangePoolOwner -
func (v *kdaFeesPoolKApp) ChangePoolOwner(assetID []byte, sender []byte, newOwner []byte) (transaction.Transaction_TXResultCode, error) {
	if !v.forkController.EnableSmartContracts() {
		return transaction.Transaction_Ok, nil
	}

	ctx := v.KAppController.GetCurrentKAppContext()

	app, err := v.getKApp()
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		if errors.Is(err, common.ErrAssetPoolNotFound) {
			return transaction.Transaction_Ok, nil
		}
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if !kapps.MatchesAddress(pool.OwnerAddress, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAccNotOwner.Error())
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(newOwner) != v.pubkeyConv.Len() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, common.ErrAssetPoolInvalidAddress.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAddress
	}

	// a pool admin left at the former owner (the default on pool creation) would keep
	// withdraw and update rights after the transfer, so it follows the owner. An admin
	// set to any other address was chosen deliberately and stays.
	if v.forkController.FixAuditChangesV7() && kapps.MatchesAddress(pool.AdminAddress, pool.OwnerAddress) {
		pool.AdminAddress = newOwner
	}

	pool.OwnerAddress = newOwner

	err = v.setPool(app, assetID, pool)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	// add receipts
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDAPool,
		ctx.ContractID(),
		assetID,
	))

	return transaction.Transaction_Ok, nil
}

// GetPoolOwner -
func (v *kdaFeesPoolKApp) GetPoolOwner(assetID []byte) ([]byte, error) {
	app, err := v.getKApp()
	if err != nil {
		return nil, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		return nil, err
	}

	return pool.OwnerAddress, nil
}

// Deposit -
func (v *kdaFeesPoolKApp) Deposit(sender []byte, tc *transaction.DepositContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	assetID := tc.GetID()
	if assetID == nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrInvalidAsset.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidAsset
	}

	amount := tc.GetAmount()
	if amount <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if amount <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrAssetPoolAmountError.Error())
		return transaction.Transaction_AmountInvalid, common.ErrAssetPoolAmountError
	}

	isPoolOwnerOrAdmin := kapps.MatchesAddress(pool.AdminAddress, sender) || kapps.MatchesAddress(pool.OwnerAddress, sender)

	if !isPoolOwnerOrAdmin {
		if !v.forkController.EnableSmartContracts() {
			// Before fork, only owner or admin could deposit
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAssetPoolInvalidAddress.Error())
			return transaction.Transaction_KAPPError, common.ErrAssetPoolInvalidAddress
		}

		// After fork, owner, admin and kda deposit role can deposit
		_, kda, err := v.KAppController.GetKDAKApp().GetKDA(assetID)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
			return transaction.Transaction_KAPPError, err
		}

		role, err := kda.GetRoleByAddress(sender)
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRoleAddress, err.Error())
			return transaction.Transaction_AssetError, err
		}

		if !role.HasRoleDeposit {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMissingDepositRole, common.ErrInvalidValue.Error())
			return transaction.Transaction_AccountError, common.ErrInvalidValue
		}
	}

	// get sender acc
	acc, err := v.accountsCacher.LoadUser(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_AccountError, err
	}

	// check if amount/kda is valid
	balance := acc.GetBalance(currencyID, v.forkController.EnableSmartContracts())
	if balance < amount {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, common.ErrBalance.Error())
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	// add to pool balance
	switch string(currencyID) {
	case string(kdautils.KLVIdentifier):
		pool.KLVBalance += amount

	case string(pool.KDA):
		pool.KDABalance += amount

	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrAssetPoolInvalidAsset.Error())
		return transaction.Transaction_AssetError, common.ErrAssetPoolInvalidAsset
	}

	// sub from account
	err = acc.SubFromBalance(amount, currencyID, v.forkController.EnableSmartContracts())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, err.Error())
		return transaction.Transaction_AccountError, err
	}

	err = v.accountsCacher.SaveUser(acc)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_AccountError, err
	}

	err = v.setPool(app, assetID, pool)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	// add receipts
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		sender,
		kapps.KDAFeesPoolKAppAddress,
		[]byte(strconv.FormatInt(amount, 10)),
		[]byte(currencyID),
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
	))

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDAPool,
		ctx.ContractID(),
		assetID,
	))

	return transaction.Transaction_Ok, nil
}

// func (v *kdaFeesPoolKApp) Withdraw(poolID []byte, kdaID []byte, amount int64, addressTo []byte) (transaction.Transaction_TXResultCode, error) {

// Withdraw -
func (v *kdaFeesPoolKApp) Withdraw(sender []byte, tc *transaction.WithdrawContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	assetID := tc.GetAssetID()
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	if tc.GetAmount() <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, common.ErrAssetPoolAmountError.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolAmountError
	}

	// check if addressTo is allowed to withdraw
	if !kapps.MatchesAddress(pool.AdminAddress, sender) && !kapps.MatchesAddress(pool.OwnerAddress, sender) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidPermission, common.ErrAssetPoolInvalidAddress.Error())
		return transaction.Transaction_KAPPError, common.ErrAssetPoolInvalidAddress
	}

	// get receiver
	receiver, err := v.accountsCacher.LoadUser(sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_AccountError, err
	}

	// check if amount/kda is valid
	switch string(tc.GetCurrencyID()) {
	case string(kdautils.KLVIdentifier):
		if pool.KLVBalance < tc.GetAmount() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, common.ErrAssetPoolOutOfFunds.Error())
			return transaction.Transaction_OutOfFunds, common.ErrAssetPoolOutOfFunds
		}
		// remove from pool balance
		pool.KLVBalance -= tc.GetAmount()
		// add to addressTo balance
		err = receiver.AddToBalance(tc.GetAmount(), nil, v.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, err.Error())
			return transaction.Transaction_AccountError, err
		}

	case string(pool.KDA):
		if pool.KDABalance < tc.GetAmount() {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFunds, common.ErrAssetPoolOutOfFunds.Error())
			return transaction.Transaction_OutOfFunds, common.ErrAssetPoolOutOfFunds
		}
		// remove from pool balance
		pool.KDABalance -= tc.GetAmount()
		// add to addressTo balance
		err = receiver.AddToBalance(tc.GetAmount(), tc.GetCurrencyID(), v.forkController.EnableSmartContracts())
		if err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAmount, err.Error())
			return transaction.Transaction_AccountError, err
		}

	default:
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAssetID, common.ErrAssetPoolInvalidAsset.Error())
		return transaction.Transaction_AssetError, common.ErrAssetPoolInvalidAsset
	}

	err = v.accountsCacher.SaveUser(receiver)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidAddress, err.Error())
		return transaction.Transaction_AccountError, err
	}

	err = v.setPool(app, assetID, pool)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAssetNotFound, err.Error())
		return transaction.Transaction_KAPPError, err
	}

	// add receipts
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Transfer,
		ctx.ContractID(),
		kapps.KDAFeesPoolKAppAddress,
		sender,
		[]byte(strconv.FormatInt(tc.Amount, 10)),
		[]byte(tc.CurrencyID),
		nil,
		[]byte{byte(kapps.KDAData_Fungible)},
	))

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDAPool,
		ctx.ContractID(),
		assetID,
	))

	return transaction.Transaction_Ok, nil
}

// Swap charges the full signed KDA amount, including any excess over the required fee.
func (v *kdaFeesPoolKApp) Swap(sender state.UserAccountHandler, klvAmount int64, info data.KDAFeeHandler) error {
	if check.IfNil(info) {
		return nil
	}

	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return err
	}

	pool, err := v.validate(app, klvAmount, info)
	if err != nil {
		return err
	}

	err = v.checkSwapTransferControls(sender, info)
	if err != nil {
		return err
	}

	return v.swap(sender, app, pool, klvAmount, info.GetAmount(), info, v.accountsCacher.SaveUser)
}

// SwapBandwidthFee swaps only the bandwidth share of a fee paid in KDA, for the
// FixAuditChangesV7 flow. It runs ahead of the processing snapshot, so it refuses
// everything Swap would refuse for the whole fee: a payment the pool or the sender
// cannot cover in full is still dropped here rather than failing inside the block.
// The rest of the signed amount is left for SwapKAppFee.
func (v *kdaFeesPoolKApp) SwapBandwidthFee(sender state.UserAccountHandler, bandwidthFee int64, totalFee int64, info data.KDAFeeHandler) error {
	if check.IfNil(info) {
		return nil
	}

	if bandwidthFee < 0 || bandwidthFee > totalFee {
		return common.ErrAssetPoolInvalidAmount
	}

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	pool, err := v.validate(app, totalFee, info)
	if err != nil {
		return err
	}

	err = v.checkSwapTransferControls(sender, info)
	if err != nil {
		return err
	}

	if sender.GetBalance(info.GetKDA(), v.forkController.EnableSmartContracts()) < info.GetAmount() {
		return state.ErrInsufficientFunds
	}

	_, bandwidthKDA, err := v.bandwidthShare(app, bandwidthFee, info)
	if err != nil {
		return err
	}

	return v.swap(sender, app, pool, bandwidthFee, bandwidthKDA, info, v.accountsCacher.SaveUser)
}

// SwapKAppFee swaps the KApp fee share of a fee paid in KDA for whatever the signed
// amount has left after SwapBandwidthFee, excess included. It runs inside the
// processing snapshot, so a transaction that fails gives that KDA back untouched.
// The bandwidth share rounds up, so the rest can fall one unit short of the KApp
// fee's own quote: the signed amount is checked against the whole fee instead, as
// Swap does, and on success the two shares together debit exactly what Swap would.
func (v *kdaFeesPoolKApp) SwapKAppFee(sender state.UserAccountHandler, bandwidthFee int64, kappFee int64, info data.KDAFeeHandler) error {
	if check.IfNil(info) {
		return nil
	}

	if bandwidthFee < 0 || kappFee < 0 || kappFee > math.MaxInt64-bandwidthFee {
		return common.ErrAssetPoolInvalidAmount
	}

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	pool, bandwidthKDA, err := v.bandwidthShare(app, bandwidthFee, info)
	if err != nil {
		return err
	}

	_, totalKDA, err := v.compute(app, bandwidthFee+kappFee, info)
	if err != nil {
		return err
	}

	if info.GetAmount() < totalKDA {
		return fmt.Errorf("%w: amount %d/%d", common.ErrAssetPoolAmountError, info.GetAmount(), totalKDA)
	}

	kdaAmount := info.GetAmount() - bandwidthKDA
	if kappFee == 0 && kdaAmount == 0 {
		return nil
	}

	if pool.KLVBalance < kappFee {
		return common.ErrAssetPoolOutOfFunds
	}

	// UpdateUser, not SaveUser: SaveUser evicts the sender from the cache and the KApp
	// fee consumed on it right after would be lost
	return v.swap(sender, app, pool, kappFee, kdaAmount, info, v.accountsCacher.UpdateUser)
}

// bandwidthShare quotes the bandwidth share of a fee paid in KDA, rounded up and
// capped at the signed amount. Rounding down would price a share below one KDA unit
// at zero, and a failed transaction, which pays only this share, would go free while
// the pool still pays out the bandwidth fee.
func (v *kdaFeesPoolKApp) bandwidthShare(app state.KAppAccountHandler, bandwidthFee int64, info data.KDAFeeHandler) (*KDAFeesPoolData, int64, error) {
	pool, kdaAmount, err := v.compute(app, bandwidthFee, info)
	if err != nil {
		return nil, 0, err
	}

	rest := new(big.Int).Mul(big.NewInt(bandwidthFee), big.NewInt(pool.FRatioKDA))
	rest.Rem(rest, big.NewInt(pool.FRatioKLV))
	if rest.Sign() != 0 && kdaAmount < info.GetAmount() {
		kdaAmount++
	}

	if kdaAmount > info.GetAmount() {
		kdaAmount = info.GetAmount()
	}

	return pool, kdaAmount, nil
}

func (v *kdaFeesPoolKApp) checkSwapTransferControls(sender state.UserAccountHandler, info data.KDAFeeHandler) error {
	if !v.forkController.FixAuditChangesV5() {
		return nil
	}

	_, kda, err := v.KAppController.GetKDAKApp().GetKDA(feeAssetID(info))
	if err != nil {
		return err
	}

	return checkFeeTransferControls(sender.AddressBytes(), kda)
}

// swap moves kdaAmount of the fee asset from the sender to the pool and klvAmount of
// the pool's KLV to the sender, then stores the sender with saveSender.
func (v *kdaFeesPoolKApp) swap(
	sender state.UserAccountHandler,
	app state.KAppAccountHandler,
	pool *KDAFeesPoolData,
	klvAmount int64,
	kdaAmount int64,
	info data.KDAFeeHandler,
	saveSender func(account state.AccountHandler) error,
) error {
	// SUB KDA from USER
	err := sender.SubFromBalance(kdaAmount, info.GetKDA(), v.forkController.EnableSmartContracts())
	if err != nil {
		return err
	}

	// ADD KDA to Pool
	pool.KDABalance += kdaAmount

	// SUB KLV Pool
	pool.KLVBalance -= klvAmount
	if pool.KLVBalance < 0 {
		return state.ErrInsufficientFunds
	}

	// ADD KLV to User
	err = sender.AddToBalance(klvAmount, nil, v.forkController.EnableSmartContracts())
	if err != nil {
		return err
	}

	err = saveSender(sender)
	if err != nil {
		return err
	}

	err = v.setPool(app, info.GetKDA(), pool)
	if err != nil {
		return err
	}

	err = v.saveKApp(app)
	if err != nil {
		return err
	}

	return nil
}

// Compute estimates fee based on KLV amount and type
func (v *kdaFeesPoolKApp) Compute(klvFee int64, info data.KDAFeeHandler) (int64, error) {
	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return 0, err
	}

	return v.computeValue(app, klvFee, info)
}

// ComputeUncached is Compute for callers outside the processing goroutine
// (e.g. the REST API); see AccountsCacher.LoadKAppUncached
func (v *kdaFeesPoolKApp) ComputeUncached(klvFee int64, info data.KDAFeeHandler) (int64, error) {
	app, err := v.accountsCacher.LoadKAppUncached(kapps.KDAFeesPoolKAppAddress)
	if err != nil {
		return 0, err
	}

	return v.computeValue(app, klvFee, info)
}

func (v *kdaFeesPoolKApp) computeValue(app state.KAppAccountHandler, klvFee int64, info data.KDAFeeHandler) (int64, error) {
	_, value, err := v.compute(app, klvFee, info)

	return value, err
}

func (v *kdaFeesPoolKApp) compute(app state.KAppAccountHandler, klvFee int64, info data.KDAFeeHandler) (*KDAFeesPoolData, int64, error) {
	pool, err := v.getPool(app, info.GetKDA())
	if err != nil {
		return nil, 0, err
	}

	if !pool.Active {
		return nil, 0, common.ErrAssetPoolNotActive
	}

	if v.forkController.FixAuditChangesV3() && (pool.FRatioKLV <= 0 || pool.FRatioKDA <= 0) {
		return nil, 0, common.ErrAssetPoolInvalidAmount
	}

	// TODO: implement automated exchange based on balance with spread
	// compute amount fixed ratio
	// FRatioKDA:FRatioKLV
	// for every FRatioKDA will need FRatioKLV
	// example: 1_000_000 : 4_200
	// 10_000_000 KLV needed requires 42_000 KDA
	fv := big.NewInt(klvFee)
	fv = fv.Mul(fv, big.NewInt(pool.FRatioKDA))
	fv = fv.Quo(fv, big.NewInt(pool.FRatioKLV))

	if v.forkController.FixAuditChangesV3() && !fv.IsInt64() {
		return nil, 0, common.ErrAssetPoolInvalidAmount
	}

	return pool, fv.Int64(), nil
}

// Validate -
// Validate is the interception-side gate. It must reject everything Swap would reject: a fee payment
// admitted here but refused at execution is gossiped to every node, dropped without consuming a fee
// or a nonce, and can be resubmitted for free, so any Swap rejection reason without a counterpart
// here is a spam amplifier.
func (v *kdaFeesPoolKApp) Validate(senderAddress []byte, klvFee int64, info data.KDAFeeHandler) error {
	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return err
	}

	_, err = v.validate(app, klvFee, info)
	if err != nil {
		return err
	}

	if !v.forkController.FixAuditChangesV5() {
		return nil
	}

	// Read the asset uncached: Validate runs on the interceptor goroutines, concurrently with
	// block processing, and the cached KDA KApp's TrackableDataTrie is a bare map that processing
	// writes on every asset mutation. See AccountsCacher.LoadKAppUncached.
	kda, err := v.KAppController.GetKDAKApp().GetKDAUncached(feeAssetID(info))
	if err != nil {
		return err
	}

	return checkFeeTransferControls(senderAddress, kda)
}

// feeAssetID is the asset a fee is paid in, without the nonce suffix a collection identifier
// carries, defaulting to KLV when the fee names no asset.
func feeAssetID(info data.KDAFeeHandler) []byte {
	assetID := bytes.Split(info.GetKDA(), []byte(kapps.Sp))[0]
	if assetID == nil {
		return kdautils.KLVIdentifier
	}

	return assetID
}

// checkFeeTransferControls rejects a fee payment the asset itself would not allow, so paying a
// fee in KDA cannot bypass the controls a plain transfer of it is subject to. The asset is
// resolved by the caller because the two call sites must read it differently: Swap runs inside
// block processing and reads the cached KApp, Validate runs on an interceptor goroutine and
// cannot.
//
// Deliberately not kapps.KDAData.IsTransferAllowed, which is the same chain plus one chance:
// it also admits the transfer when the *destination* holds the deposit role. Routed through
// here that would let an issuer open its limit-transfer asset to fee payment by granting the
// deposit role to KDAFeesPoolKAppAddress, widening who may pay fees in it. That may well be the
// semantics we want - it is how a plain transfer behaves - but it is a consensus-visible
// broadening and not what this audit fix was scoped to decide, so the narrower check stands.
func checkFeeTransferControls(senderAddress []byte, kda *kapps.KDAData) error {
	if kda.GetAttributes().GetIsPaused() {
		return process.ErrAssetIsPaused
	}

	if !kda.CanSenderTransfer(senderAddress) {
		return process.ErrKDATransferNotAllowed
	}

	return nil
}

func (v *kdaFeesPoolKApp) validate(app state.KAppAccountHandler, klvFee int64, info data.KDAFeeHandler) (*KDAFeesPoolData, error) {
	if klvFee < 0 {
		return nil, common.ErrAssetPoolInvalidAmount
	}

	pool, value, err := v.compute(app, klvFee, info)
	if err != nil {
		return nil, err
	}

	if pool.KLVBalance < klvFee {
		return nil, common.ErrAssetPoolOutOfFunds
	}

	if value <= 0 || info.GetAmount() < value {
		return nil, fmt.Errorf("%w: amount %d/%d",
			common.ErrAssetPoolAmountError,
			info.GetAmount(),
			value,
		)
	}

	return pool, nil
}
