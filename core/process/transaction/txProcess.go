package transaction

import (
	"bytes"
	"errors"
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"google.golang.org/protobuf/proto"
)

var log = logger.GetOrCreate("process/transaction")
var logSC = logger.GetOrCreate("process/transaction.smartcontract")
var _ process.TransactionProcessor = (*txProcessor)(nil)

const (
	OrderNotExecuted = iota
	OrderExecuted
)

type txProcessor struct {
	*baseTxProcessor
	txFeeHandler       process.TransactionFeeHandler
	ratingsData        process.RatingsInfoHandler
	proposalController kapps.ActiveProposalController
}

// ArgsNewTxProcessor defines the arguments needed for new tx processor
type ArgsNewTxProcessor struct {
	Cfg            config.Config
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	KAppController kapp.KAppController
	PubkeyConv     core.PubkeyConverter
	KeyGen         crypto.KeyGenerator
	SingleSigner   crypto.SingleSigner
	TxFeeHandler   process.TransactionFeeHandler
	EconomicsFee   process.EconomicsDataHandler
	EpochNotifier  process.EpochNotifier
	RatingsData    process.RatingsInfoHandler
	AccountsCacher state.AccountsCacher
	ForkController core.ForkController
	ScProcessor    process.SmartContractProcessor
}

// NewTxProcessor creates a new txProcessor engine
func NewTxProcessor(args ArgsNewTxProcessor) (*txProcessor, error) {
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.TxFeeHandler) {
		return nil, process.ErrNilTxFeeHandler
	}
	if check.IfNil(args.EconomicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, common.ErrNilEpochNotifier
	}
	if check.IfNil(args.RatingsData) {
		return nil, common.ErrNilRater
	}
	if check.IfNil(args.KAppController) {
		return nil, common.ErrKAppController
	}
	if check.IfNil(args.SingleSigner) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(args.KeyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(args.AccountsCacher) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(args.ScProcessor) {
		return nil, process.ErrNilSmartContractProcessor
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	baseTxProcess := &baseTxProcessor{
		cfg:            args.Cfg,
		kApps:          args.KAppController,
		pubkeyConv:     args.PubkeyConv,
		economicsFee:   args.EconomicsFee,
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		scProcessor:    args.ScProcessor,
		keyGen:         args.KeyGen,
		singleSigner:   args.SingleSigner,
		accountsCacher: args.AccountsCacher,
		forkController: args.ForkController,
	}

	txProc := &txProcessor{
		baseTxProcessor: baseTxProcess,
		txFeeHandler:    args.TxFeeHandler,
		ratingsData:     args.RatingsData,
	}

	args.EpochNotifier.RegisterNotifyHandler(txProc)

	return txProc, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (txProc *txProcessor) EpochConfirmed(epoch uint32) {
}

// IsInterfaceNil returns true if there is no value under the interface
func (txProc *txProcessor) IsInterfaceNil() bool {
	return txProc == nil
}

// LoadProposalController will load the proposal controller into txProcess instance
func (txProc *txProcessor) SetProposalController(controller kapps.ActiveProposalController) error {
	if check.IfNil(controller) {
		return common.ErrNilProposalController
	}

	txProc.proposalController = controller

	return nil
}

// ProcessBandwidthFee processed bandwidth fees of the transactions
func (txProc *txProcessor) ProcessBandwidthFee(txHash []byte, tx *transaction.Transaction, ownerAcc state.UserAccountHandler) (int64, error) {
	if check.IfNil(ownerAcc) {
		tx.ResultCode = transaction.Transaction_Fail
		return 0, common.ErrNilAddress
	}

	calculatedBdFee := tx.GetBandwidthFee()
	// if fork is inactive use the old order
	if !txProc.forkController.FPRComputeAndKdaFeeFlow() {
		result, err := txProc.consumeFee(calculatedBdFee, tx, ownerAcc)
		if err != nil {
			return result, err
		}
	}

	if tx.GetRawData().GetKDAFee() != nil {
		// Swap KDA to KLV if requested and pool exists
		err := txProc.kApps.GetKDAFeesPoolKApp().Swap(ownerAcc, tx.GetTotalFees(), tx.GetRawData().GetKDAFee())
		if err != nil {
			return 0, err
		}

		tx.Receipts = append(tx.Receipts, NewReceipt(
			UpdateKDAPool,
			defaultTXContractID,
			tx.GetRawData().GetKDAFee().GetKDA(),
		))
	}

	// if fork is active use the new order
	if txProc.forkController.FPRComputeAndKdaFeeFlow() {
		result, err := txProc.consumeFee(calculatedBdFee, tx, ownerAcc)
		if err != nil {
			return result, err
		}
	}

	ownerAcc.IncreaseNonce(1)

	if err := txProc.accountsCacher.SaveUser(ownerAcc); err != nil {
		tx.ResultCode = transaction.Transaction_SaveAccountError
		return 0, err
	}

	txProc.txFeeHandler.ProcessTransactionFee(calculatedBdFee, 0, txHash)

	return calculatedBdFee, txProc.accountsCacher.SaveAll()
}

func (txProc *txProcessor) consumeFee(fee int64, tx *transaction.Transaction, ownerAcc state.UserAccountHandler) (int64, error) {
	if fee == 0 {
		return 0, nil
	}

	balance := ownerAcc.GetBalance(nil, txProc.forkController.EnableSmartContracts())
	if balance == 0 || balance < fee {
		tx.ResultCode = transaction.Transaction_OutOfFunds
		return 0, process.ErrInsufficientFee
	}

	if err := ownerAcc.SubFromBalance(fee, nil, txProc.forkController.EnableSmartContracts()); err != nil {
		tx.ResultCode = transaction.Transaction_BalanceError
		return 0, err
	}

	return 0, nil
}

// ProcessKAppFee processed fees of the transactions
func (txProc *txProcessor) ProcessKAppFee(txHash []byte, tx *transaction.Transaction, ownerAcc state.UserAccountHandler) (int64, error) {
	if check.IfNil(ownerAcc) {
		tx.ResultCode = transaction.Transaction_Fail
		return 0, common.ErrNilAddress
	}

	kappFee := tx.GetKAppFee()

	result, err := txProc.consumeFee(kappFee, tx, ownerAcc)
	if err != nil {
		return result, err
	}

	if err := txProc.accountsCacher.UpdateUser(ownerAcc); err != nil {
		tx.ResultCode = transaction.Transaction_SaveAccountError
		return 0, err
	}

	txProc.txFeeHandler.ProcessTransactionFee(0, kappFee, txHash)

	return kappFee, nil
}

// PreProcessTransaction -
func (txProc *txProcessor) PreProcessTransaction(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
	ownerAcc, err := txProc.accountsCacher.GetExistingUser(tx.GetSender())
	if err != nil {
		tx.ResultCode = transaction.Transaction_LoadAccountError
		return nil, nil, err
	}

	computedHash, err := tools.CalculateHash(txProc.marshalizer, txProc.hasher, tx.RawData)
	if err != nil {
		tx.ResultCode = transaction.Transaction_Fail
		log.Error("invalid tx hash", "computedHash", computedHash)
		return nil, nil, err
	}

	err = txProc.checkTxValues(tx, ownerAcc, computedHash)
	if err != nil {
		tx.ResultCode = transaction.Transaction_ValueInvalid
		log.Error("invalid tx fees/values", "error", err.Error())
		return nil, nil, err
	}

	return ownerAcc, computedHash, nil
}

func cloneReceipts(tx *transaction.Transaction) []*transaction.Transaction_Receipt {
	bkpReceipts := make([]*transaction.Transaction_Receipt, len(tx.Receipts))

	for i, p := range tx.Receipts {
		if p == nil {
			// Skip to next for nil source pointer
			continue
		}

		// Create shallow copy of source element
		v := proto.Clone(p).(*transaction.Transaction_Receipt)

		// Assign address of copy to destination.
		bkpReceipts[i] = v
	}

	return bkpReceipts
}

// findTxIndexInBlock finds the index of a transaction hash in the block's TxHashes
func (txProc *txProcessor) findTxIndexInBlock(block *block.Block, txHash []byte) int {
	for i, hash := range block.TxHashes {
		if bytes.Equal(hash, txHash) {
			return i
		}
	}
	return -1
}

// ProcessTransaction modifies the account states in respect with the transaction data
func (txProc *txProcessor) ProcessTransaction(block *block.Block, txHash []byte, tx *transaction.Transaction) error {
	bkpReceipts, err := txProc.validateAndPrepareTransaction(tx)
	if err != nil {
		return err
	}

	// Consensus account freeze: reject a tx from a frozen sender before any state is
	// touched. On the apply/verify path, so a block carrying such a tx is rejected
	// fleet-wide. Fork gating lives inside IsAccountFrozen, per-account.
	if common.IsAccountFrozen(tx.GetSender(), txProc.forkController) {
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		return common.ErrAccountFrozen
	}

	ownerAcc, err := txProc.accountsCacher.GetExistingUser(tx.GetSender())
	if err != nil {
		tx.ResultCode = transaction.Transaction_AccountError
		return err
	}

	process.DisplayProcessTxDetails(
		"ProcessTransaction: ...",
		ownerAcc,
		tx,
		txHash,
		txProc.pubkeyConv,
	)

	kAppFee, kAppFeeErr := txProc.ProcessKAppFee(txHash, tx, ownerAcc)
	if kAppFeeErr != nil {
		log.Error("error processing kApp fee")
		tx.ResultCode = transaction.Transaction_FeeInvalid
	}

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     -1,
		ContractType:   -1,
		Block:          block,
		TxHash:         txHash,
		TX:             tx,
	})

	if kAppFeeErr == nil {
		// execute contracts
		kAppFeeErr = txProc.processContracts(ctx, ownerAcc, tx)

		// Verify leader vs validator SC execution results
		// If block has TxResults, validate our execution against consensus
		if len(block.TxResults) > 0 {
			// Get validator's execution time from context
			validatorExecTime := ctx.GetExecutionTime()
			if validatorExecTime > 0 {
				log.Debug("Validating transaction result against consensus", "txHash", txHash)

				// Only validate if we have execution time data
				kAppFeeErr = txProc.validateTransactionResult(block, txHash, tx, kAppFeeErr, validatorExecTime.Nanoseconds())
			}
		}
	}

	txProc.accountsCacher.ResetAll(
		txProc.forkController.ProcessorFlowITOPrice(),
	)

	tx.Block = block.GetNonce()

	if kAppFeeErr != nil {
		// reset receipt if TX fail, but preserve system receipts (Debug, Warning, Error) for diagnostics
		tx.Receipts = append(bkpReceipts, ctx.Receipts().GetPreserved()...)

		if kAppFee > 0 {
			txProc.txFeeHandler.RevertTransactionFee(txHash, 0, kAppFee)
		}

		log.Warn(
			"ProcessTransaction: ...",
			"txHash", txHash,
			"error", kAppFeeErr.Error(),
		)

		return kAppFeeErr
	}

	tx.Receipts = append(tx.Receipts, ctx.Receipts().Get()...)

	txProc.kApps.SetCurrentKAppContext(disabled.NewDisabledKappContext())

	return nil
}

func (txProc *txProcessor) validateAndPrepareTransaction(tx *transaction.Transaction) ([]*transaction.Transaction_Receipt, error) {
	if check.IfNil(tx) {
		return nil, process.ErrNilTransaction
	}

	txProc.accountsCacher.ResetAll(
		txProc.forkController.ProcessorFlowITOPrice(),
	)

	return cloneReceipts(tx), nil
}

func (txProc *txProcessor) processContracts(ctx kapp.KappContext, ownerAcc state.UserAccountHandler, tx *transaction.Transaction) error {
	var kAppFeeErr, err error
	for i, contract := range tx.RawData.Contract {
		ctx.SetContractID(i)
		txProc.kApps.SetCurrentKAppContext(ctx)

		switch contract.Type {
		case transaction.TXContract_TransferContractType:
			err = txProc.transferContract(ctx, tx)
		case transaction.TXContract_CreateAssetContractType:
			err = txProc.createAssetContract(ctx, tx)
		case transaction.TXContract_AssetTriggerContractType:
			err = txProc.assetTriggerContract(ctx, tx)
		case transaction.TXContract_CreateValidatorContractType:
			err = txProc.createValidatorContract(ctx, tx)
		case transaction.TXContract_ValidatorConfigContractType:
			err = txProc.validatorConfigContract(ctx, tx)
		case transaction.TXContract_FreezeContractType:
			err = txProc.freezeContract(ctx, tx)
		case transaction.TXContract_UnfreezeContractType:
			err = txProc.unfreezeContract(ctx, tx)
		case transaction.TXContract_DelegateContractType:
			err = txProc.delegateContract(ctx, tx)
		case transaction.TXContract_UndelegateContractType:
			err = txProc.undelegateContract(ctx, tx)
		case transaction.TXContract_WithdrawContractType:
			err = txProc.withdrawContract(ctx, tx)
		case transaction.TXContract_ClaimContractType:
			err = txProc.claimContract(ctx, tx)
		case transaction.TXContract_UnjailContractType:
			err = txProc.unjailContract(ctx, tx)
		case transaction.TXContract_SetAccountNameContractType:
			err = txProc.setAccountNameContract(ctx, tx)
		case transaction.TXContract_ProposalContractType:
			err = txProc.proposalContract(ctx, tx)
		case transaction.TXContract_VoteContractType:
			err = txProc.voteContract(ctx, tx)
		case transaction.TXContract_ConfigITOContractType:
			err = txProc.configITOContract(ctx, tx)
		case transaction.TXContract_SetITOPricesContractType:
			err = txProc.setITOPricesContract(ctx, tx)
		case transaction.TXContract_BuyContractType:
			err = txProc.buyContract(ctx, tx)
		case transaction.TXContract_SellContractType:
			err = txProc.sellContract(ctx, tx)
		case transaction.TXContract_CancelMarketOrderContractType:
			err = txProc.cancelMarketOrderContract(ctx, tx)
		case transaction.TXContract_CreateMarketplaceContractType:
			err = txProc.createMarketplaceContract(ctx, tx)
		case transaction.TXContract_ConfigMarketplaceContractType:
			err = txProc.configMarketplaceContract(ctx, tx)
		case transaction.TXContract_UpdateAccountPermissionContractType:
			err = txProc.updateAccountPermission(ctx, tx)
		case transaction.TXContract_DepositContractType:
			err = txProc.depositContract(ctx, tx)
		case transaction.TXContract_ITOTriggerContractType:
			err = txProc.itoTriggerContract(ctx, tx)
		case transaction.TXContract_SmartContractType:
			err = txProc.smartContract(ctx, ownerAcc, tx)
		default:
			tx.ResultCode = transaction.Transaction_ContractNotFound
			err = process.ErrInvalidTransactionType
		}
		if err != nil {
			kAppFeeErr = err
			break
		}
	}

	if kAppFeeErr == nil {
		kAppFeeErr = txProc.accountsCacher.SaveAll()
		if kAppFeeErr != nil {
			tx.ResultCode = transaction.Transaction_SaveAccountError
		}
	}

	return kAppFeeErr
}

func (txProc *txProcessor) transferContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	cType := tx.RawData.Contract[ctx.ContractID()].Type
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetTransferContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	// check if address isPayable
	if txProc.forkController.EnableSmartContracts() {
		if err := txProc.validatePayable(tc, tx); err != nil {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldAddressNotPayable, err.Error())
			return err
		}
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Transfer(cType, tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) validatePayable(tc *transaction.TransferContract, tx *transaction.Transaction) error {
	if !core.IsSmartContractAddress(tc.ToAddress) {
		return nil
	}

	dstAccount, err := txProc.accountsCacher.GetExistingUser(tc.ToAddress)
	if err != nil && !errors.Is(err, common.ErrAccNotFound) {
		tx.ResultCode = transaction.Transaction_AccountError
		return err
	}

	if dstAccount == nil || len(dstAccount.GetCodeHash()) == 0 {
		tx.ResultCode = transaction.Transaction_AccountError
		return process.ErrInvalidRcvAddr
	}

	isPayable, err := txProc.scProcessor.IsPayable(tx.GetSender(), tc.ToAddress)
	if err != nil {
		tx.ResultCode = transaction.Transaction_AccountError
		return err
	}

	if !isPayable {
		tx.ResultCode = transaction.Transaction_AccountError
		return process.ErrAccountNotPayable
	}

	return nil
}

func (txProc *txProcessor) createAssetContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetCreateAssetContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetKDAKApp().Create(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) assetTriggerContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetAssetTriggerContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetKDAKApp().Trigger(tx.GetSender(), tc, tx.GetRawData().GetData())

	return err
}

func (txProc *txProcessor) createValidatorContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetCreateValidatorContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetValidatorsKApp().Register(tc)

	return err
}

func (txProc *txProcessor) validatorConfigContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetValidatorConfigContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetValidatorsKApp().UpdateValidator(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) freezeContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetFreezeContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Freeze(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) unfreezeContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetUnfreezeContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Unfreeze(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) delegateContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetDelegateContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Delegate(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) undelegateContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetUndelegateContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Undelegate(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) withdrawContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetWithdrawContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	switch tc.WithdrawType {
	case transaction.WithdrawContract_Staking:
		tx.ResultCode, err = txProc.kApps.GetAccountsKApp().Withdraw(tx.GetSender(), tc)
	case transaction.WithdrawContract_KDAPool:
		tx.ResultCode, err = txProc.kApps.GetKDAFeesPoolKApp().Withdraw(tx.GetSender(), tc)
	default:
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrWithdrawTypeInvalid
	}

	return err
}

func (txProc *txProcessor) claimContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetClaimContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	switch tc.GetClaimType() {
	case transaction.ClaimContract_StakingClaim:
		tx.ResultCode, err = txProc.kApps.GetAccountsKApp().ClaimStaking(tx.GetSender(), tc)
	case transaction.ClaimContract_AllowanceClaim:
		tx.ResultCode, err = txProc.kApps.GetAccountsKApp().ClaimAllowance(tx.GetSender(), tc)
	case transaction.ClaimContract_MarketClaim:
		tx.ResultCode, err = txProc.kApps.GetMarketKApp().Claim(tx.GetSender(), tc)
	default:
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrClaimTypeInvalid
	}

	return err
}

func (txProc *txProcessor) unjailContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetUnjailContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	// unjail validator kapp
	tx.ResultCode, err = txProc.kApps.GetValidatorsKApp().Unjail(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) setAccountNameContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSetAccountNameContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().SetAccountName(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) proposalContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetProposalContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetProposalKApp().Create(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) voteContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetVoteContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetProposalKApp().Vote(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) configITOContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetConfigITOContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetITOKApp().Config(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) itoTriggerContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetITOTriggerContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetITOKApp().Trigger(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) setITOPricesContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	if txProc.forkController.KdaFpr() {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return common.ErrDeprecatedContract
	}

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSetITOPricesContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetITOKApp().SetPrices(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) buyContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetBuyContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	switch tc.GetBuyType() {
	case transaction.BuyContract_ITOBuy:
		tx.ResultCode, err = txProc.kApps.GetITOKApp().Buy(tx.GetSender(), tc)
	case transaction.BuyContract_MarketBuy:
		tx.ResultCode, err = txProc.kApps.GetMarketKApp().Buy(tx.GetSender(), tc)
	default:
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrBuyTypeInvalid
	}

	return err
}

func (txProc *txProcessor) sellContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSellContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetMarketKApp().Sell(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) cancelMarketOrderContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetCancelMarketOrderContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetMarketKApp().CancelOrder(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) createMarketplaceContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetCreateMarketplaceContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetMarketKApp().CreateMarketplace(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) configMarketplaceContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetConfigMarketplaceContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetMarketKApp().ConfigMarketplace(tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) updateAccountPermission(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetUpdateAccountPermissionContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	tx.ResultCode, err = txProc.kApps.GetAccountsKApp().UpdatePermission(tx.GetSender(), tx.GetSender(), tc)

	return err
}

func (txProc *txProcessor) depositContract(ctx kapp.KappContext, tx *transaction.Transaction) error {
	tc, err := tx.RawData.Contract[ctx.ContractID()].GetDepositContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return err
	}

	switch tc.GetDepositType() {
	case transaction.DepositContract_FPRDeposit:
		tx.ResultCode, err = txProc.kApps.GetKDAKApp().Deposit(tx.GetSender(), tc)
	case transaction.DepositContract_KDAPool:
		tx.ResultCode, err = txProc.kApps.GetKDAFeesPoolKApp().Deposit(tx.GetSender(), tc)
	default:
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrDepositTypeInvalid
	}

	return err
}

func (txProc *txProcessor) smartContract(ctx kapp.KappContext, owner state.UserAccountHandler, tx *transaction.Transaction) error {
	if err := txProc.validateSCTransaction(ctx, tx); err != nil {
		return err
	}

	tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
	if err != nil {
		tx.ResultCode = transaction.Transaction_ContractInvalid
		return err
	}

	switch tc.GetType() {
	case transaction.SmartContract_SCDeploy:
		return txProc.deploySC(ctx, tc, tx)
	case transaction.SmartContract_SCInvoke:
		return txProc.invokeSC(ctx, owner, tx, tc)
	default:
		tx.ResultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrSmartContractTypeInvalid
	}

	return err
}

func (txProc *txProcessor) invokeSC(ctx kapp.KappContext, owner state.UserAccountHandler, tx *transaction.Transaction, tc *transaction.SmartContract) error {
	sw := tools.NewStopWatch()
	sw.Start("execute")

	destAcc, err := txProc.accountsCacher.GetExistingUser(tc.GetAddress())
	if err != nil {
		tx.ResultCode = transaction.Transaction_AccountError
		return err
	}

	returnCode, err := txProc.scProcessor.ExecuteSmartContractTransaction(ctx, tc, owner, destAcc)
	if err != nil || returnCode != vmcommon.Ok {
		tx.ResultCode = returnCode.ResultCode()
		if err == nil {
			err = fmt.Errorf("%w: %s", process.ErrSmartContractInvokeFailed, returnCode.String())
		}
		logSC.Debug("error invoke smart contract", "error", err, "returnCode", returnCode)
		return err
	}

	sw.Stop("execute")
	duration := sw.GetMeasurement("execute")
	logSC.Trace("execute smart contract", "duration", duration)

	tx.ResultCode = transaction.Transaction_Ok
	return nil
}

func (txProc *txProcessor) deploySC(ctx kapp.KappContext, tc *transaction.SmartContract, tx *transaction.Transaction) error {
	sw := tools.NewStopWatch()
	sw.Start("deploy")
	returnCode, err := txProc.scProcessor.DeploySmartContract(ctx, tc)
	if err != nil || returnCode != vmcommon.Ok {
		tx.ResultCode = returnCode.ResultCode()
		if err == nil {
			err = fmt.Errorf("%w: %s", process.ErrSmartContractDeploymentFailed, returnCode.String())
		}
		logSC.Debug("error deploying smart contract", "error", err, "returnCode", returnCode)
		return err
	}

	sw.Stop("deploy")
	duration := sw.GetMeasurement("deploy")
	logSC.Trace("deploy smart contract", "duration", duration)

	tx.ResultCode = transaction.Transaction_Ok
	return nil
}

func (txProc *txProcessor) validateSCTransaction(ctx kapp.KappContext, tx *transaction.Transaction) error {
	// ignore smart contract execution if the fork is inactive
	if !txProc.forkController.EnableSmartContracts() {
		tx.ResultCode = transaction.Transaction_ContractNotFound
		return process.ErrInvalidTransactionType
	}

	// Smart contract execution depends on ExecData. In deployment the `data` is a wasm file, in invoke `data` is a function call
	if ctx.GetExecData() == nil {
		tx.ResultCode = transaction.Transaction_ContractInvalid
		return process.ErrInvalidContractOrRawDataSize
	}
	return nil
}
