package kdafeespool

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
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
	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, poolID)
	if err != nil && err != common.ErrAssetPoolNotFound {
		return transaction.Transaction_KAPPError, err
	}

	if pool == nil {
		if !bytes.Equal(assetOwner, sender) {
			return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
		}

		pool = &KDAFeesPoolData{
			OwnerAddress: sender,
			KDA:          poolID,
			AdminAddress: sender,
		}
	}

	if !bytes.Equal(sender, pool.AdminAddress) && !bytes.Equal(sender, assetOwner) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(info.AdminAddress) != 0 {
		pool.AdminAddress = info.AdminAddress
	}

	if len(pool.AdminAddress) != v.pubkeyConv.Len() {
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAddress
	}

	if info.FRatioKLV == 0 || info.FRatioKDA == 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAmount
	}

	pool.Active = info.Active
	pool.FRatioKLV = info.FRatioKLV
	pool.FRatioKDA = info.FRatioKDA

	err = v.setPool(app, poolID, pool)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	ctx := v.KAppController.GetCurrentKAppContext()

	// add receipt
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateKDAPool,
		ctx.ContractID(),
		poolID,
	))

	return transaction.Transaction_Ok, nil
}

// ChangePoolOwner -
func (v *kdaFeesPoolKApp) ChangePoolOwner(assetID []byte, sender []byte, newOwner []byte) (transaction.Transaction_TXResultCode, error) {
	if !v.forkController.EnableSmartContracts() {
		return transaction.Transaction_Ok, nil
	}

	ctx := v.KAppController.GetCurrentKAppContext()

	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		if err == common.ErrAssetPoolNotFound {
			return transaction.Transaction_Ok, nil
		}
		return transaction.Transaction_KAPPError, err
	}

	if !bytes.Equal(sender, pool.OwnerAddress) {
		return transaction.Transaction_AccountNotOwner, common.ErrAccNotOwner
	}

	if len(newOwner) != v.pubkeyConv.Len() {
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolInvalidAddress
	}

	pool.OwnerAddress = newOwner

	err = v.setPool(app, assetID, pool)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
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
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidAsset
	}

	amount := tc.GetAmount()
	if amount <= 0 {
		return transaction.Transaction_AmountInvalid, common.ErrInvalidValue
	}

	currencyID := tc.GetCurrencyID()
	if currencyID == nil {
		currencyID = kdautils.KLVIdentifier
	}

	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if amount <= 0 {
		return transaction.Transaction_AmountInvalid, common.ErrAssetPoolAmountError
	}

	isPoolOwnerOrAdmin := bytes.Equal(sender, pool.AdminAddress) || bytes.Equal(sender, pool.OwnerAddress)

	if !isPoolOwnerOrAdmin {
		if !v.forkController.EnableSmartContracts() {
			// Before fork, only owner or admin could deposit
			return transaction.Transaction_KAPPError, common.ErrAssetPoolInvalidAddress
		}

		// After fork, owner, admin and kda deposit role can deposit
		_, kda, err := v.KAppController.GetKDAKApp().GetKDA(assetID)
		if err != nil {
			return transaction.Transaction_KAPPError, err
		}

		role, err := kda.GetRoleByAddress(sender)
		if err != nil {
			return transaction.Transaction_AssetError, err
		}

		if !role.HasRoleDeposit {
			return transaction.Transaction_AccountError, common.ErrInvalidValue
		}
	}

	// get sender acc
	acc, err := v.accountsCacher.LoadUser(sender)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// check if amount/kda is valid
	balance := acc.GetBalance(currencyID, v.forkController.EnableSmartContracts())
	if balance < amount {
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	// add to pool balance
	switch string(currencyID) {
	case string(kdautils.KLVIdentifier):
		pool.KLVBalance += amount

	case string(pool.KDA):
		pool.KDABalance += amount

	default:
		return transaction.Transaction_AssetError, common.ErrAssetPoolInvalidAsset
	}

	// sub from account
	err = acc.SubFromBalance(amount, currencyID, v.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	err = v.accountsCacher.SaveUser(acc)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	err = v.setPool(app, assetID, pool)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
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
		return transaction.Transaction_KAPPError, err
	}

	pool, err := v.getPool(app, assetID)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	if tc.GetAmount() <= 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrAssetPoolAmountError
	}

	// check if addressTo is allowed to withdraw
	if !bytes.Equal(sender, pool.AdminAddress) && !bytes.Equal(sender, pool.OwnerAddress) {
		return transaction.Transaction_KAPPError, common.ErrAssetPoolInvalidAddress
	}

	// get receiver
	receiver, err := v.accountsCacher.LoadUser(sender)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// check if amount/kda is valid
	switch string(tc.GetCurrencyID()) {
	case string(kdautils.KLVIdentifier):
		if pool.KLVBalance < tc.GetAmount() {
			return transaction.Transaction_OutOfFunds, common.ErrAssetPoolOutOfFunds
		}
		// remove from pool balance
		pool.KLVBalance -= tc.GetAmount()
		// add to addressTo balance
		err = receiver.AddToBalance(tc.GetAmount(), nil, v.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_AccountError, err
		}

	case string(pool.KDA):
		if pool.KDABalance < tc.GetAmount() {
			return transaction.Transaction_OutOfFunds, common.ErrAssetPoolOutOfFunds
		}
		// remove from pool balance
		pool.KDABalance -= tc.GetAmount()
		// add to addressTo balance
		err = receiver.AddToBalance(tc.GetAmount(), tc.GetCurrencyID(), v.forkController.EnableSmartContracts())
		if err != nil {
			return transaction.Transaction_AccountError, err
		}

	default:
		return transaction.Transaction_AssetError, common.ErrAssetPoolInvalidAsset
	}

	err = v.accountsCacher.SaveUser(receiver)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	err = v.setPool(app, assetID, pool)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	err = v.saveKApp(app)
	if err != nil {
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

// Swap - exchange KLV to KDA from pool to sender
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

	// SUB KDA from USER
	err = sender.SubFromBalance(info.GetAmount(), info.GetKDA(), v.forkController.EnableSmartContracts())
	if err != nil {
		return err
	}

	// ADD KDA to Pool
	pool.KDABalance += info.GetAmount()

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

	err = v.accountsCacher.SaveUser(sender)
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

	// TODO: implement automated exchange based on balance with spread
	// compute amount fixed ratio
	// FRatioKDA:FRatioKLV
	// for every FRatioKDA will need FRatioKLV
	// example: 1_000_000 : 4_200
	// 10_000_000 KLV needed requires 42_000 KDA
	fv := big.NewInt(klvFee)
	fv = fv.Mul(fv, big.NewInt(pool.FRatioKDA))
	fv = fv.Quo(fv, big.NewInt(pool.FRatioKLV))

	return pool, fv.Int64(), nil
}

// Validate -
func (v *kdaFeesPoolKApp) Validate(klvFee int64, info data.KDAFeeHandler) error {
	// check if pool exists
	app, err := v.getKApp()
	if err != nil {
		return err
	}

	_, err = v.validate(app, klvFee, info)
	return err
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
