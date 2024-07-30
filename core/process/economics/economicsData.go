package economics

import (
	"fmt"
	"math/big"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

const BaseTxSize = core.BaseTxSize

var _ process.EconomicsDataHandler = (*EconomicsData)(nil)
var log = logger.GetOrCreate("process/economics")

// EconomicsData will store information about economics
type EconomicsData struct {
	leaderPercentage float64

	proposalController   kapps.ActiveProposalController
	txSimulatorProcessor txsimulator.TransactionSimulatorProcessor
}

// ArgsNewEconomicsData defines the arguments needed for new economics data
type ArgsNewEconomicsData struct {
	EpochNotifier process.EpochNotifier
}

var ContractEnum = map[transaction.TXContract_ContractType]kapps.EnumParameter{
	transaction.TXContract_TransferContractType:                kapps.EnumParameter_KAppFeeTransfer,
	transaction.TXContract_CreateAssetContractType:             kapps.EnumParameter_KAppFeeCreateAsset,
	transaction.TXContract_AssetTriggerContractType:            kapps.EnumParameter_KAppFeeAssetTrigger,
	transaction.TXContract_CreateValidatorContractType:         kapps.EnumParameter_KAppFeeCreateValidator,
	transaction.TXContract_ValidatorConfigContractType:         kapps.EnumParameter_KAppFeeValidatorConfig,
	transaction.TXContract_FreezeContractType:                  kapps.EnumParameter_KAppFeeFreeze,
	transaction.TXContract_UnfreezeContractType:                kapps.EnumParameter_KAppFeeUnfreeze,
	transaction.TXContract_DelegateContractType:                kapps.EnumParameter_KAppFeeDelegate,
	transaction.TXContract_UndelegateContractType:              kapps.EnumParameter_KAppFeeUndelegate,
	transaction.TXContract_WithdrawContractType:                kapps.EnumParameter_KAppFeeWithdraw,
	transaction.TXContract_ClaimContractType:                   kapps.EnumParameter_KAppFeeClaim,
	transaction.TXContract_UnjailContractType:                  kapps.EnumParameter_KAppFeeUnjail,
	transaction.TXContract_SetAccountNameContractType:          kapps.EnumParameter_KAppFeeSetAccountName,
	transaction.TXContract_ProposalContractType:                kapps.EnumParameter_KAppFeeProposal,
	transaction.TXContract_VoteContractType:                    kapps.EnumParameter_KAppFeeVote,
	transaction.TXContract_ConfigITOContractType:               kapps.EnumParameter_KAppFeeConfigITO,
	transaction.TXContract_SetITOPricesContractType:            kapps.EnumParameter_KAppFeeSetITOPrices,
	transaction.TXContract_BuyContractType:                     kapps.EnumParameter_KAppFeeBuy,
	transaction.TXContract_SellContractType:                    kapps.EnumParameter_KAppFeeSell,
	transaction.TXContract_CancelMarketOrderContractType:       kapps.EnumParameter_KAppFeeCancelMarketOrder,
	transaction.TXContract_CreateMarketplaceContractType:       kapps.EnumParameter_KAppFeeCreateMarketplace,
	transaction.TXContract_ConfigMarketplaceContractType:       kapps.EnumParameter_KAppFeeConfigMarketplace,
	transaction.TXContract_UpdateAccountPermissionContractType: kapps.EnumParameter_KAppFeeUpdateAccountPermission,
	transaction.TXContract_ITOTriggerContractType:              kapps.EnumParameter_KAppFeeITOTrigger,
	transaction.TXContract_DepositContractType:                 kapps.EnumParameter_KAppFeeDeposit,
	transaction.TXContract_SmartContractType:                   kapps.EnumParameter_KAppFeeSmartContract,
}

// NewEconomicsData will create and object with information about economics parameters
func NewEconomicsData(args ArgsNewEconomicsData) (*EconomicsData, error) {
	ed := &EconomicsData{
		leaderPercentage: 0.5,
	}

	args.EpochNotifier.RegisterNotifyHandler(ed)

	return ed, nil
}

// SetProposalController will load the proposal controller into ed instance
func (ed *EconomicsData) SetProposalController(controller kapps.ActiveProposalController) error {
	if check.IfNil(controller) {
		return common.ErrNilProposalController
	}

	ed.proposalController = controller

	return nil
}

// SetTXSimulatorProcessor will load the tx simulator processor into ed instance
func (ed *EconomicsData) SetTXSimulatorProcessor(txSimulatorProcessor txsimulator.TransactionSimulatorProcessor) error {
	if check.IfNil(txSimulatorProcessor) {
		return common.ErrNilTxSimulatorProcessor
	}

	ed.txSimulatorProcessor = txSimulatorProcessor

	return nil
}

// EstimateTransactionGas will calculate how many gas units a transaction will consume
func (ed *EconomicsData) ComputeTransactionCost(tx process.TransactionWithFeeHandler, simulateSC bool) (*transaction.CostResponse, error) {
	// check if controller is initialized
	if check.IfNil(ed.proposalController) {
		return nil, process.ErrProposalNotInitialized
	}

	cost := &transaction.CostResponse{
		KAppFee:      0,
		BandwidthFee: tx.GetDataSize(),
	}

	gasMultiplier := ed.proposalController.GetParameterUint(kapps.EnumParameter_GasMultiplier)

	hasSC := false
	estimatedGas := uint64(0)
	for _, c := range tx.GetContracts() {
		feeType, ok := ContractEnum[c.Type]
		if !ok {
			return nil, process.ErrInvalidTransactionType
		}

		value := ed.proposalController.GetParameterInt(feeType)

		cost.KAppFee += value
		cost.BandwidthFee += BaseTxSize

		if simulateSC &&
			c.Type == transaction.TXContract_SmartContractType {
			hasSC = true
		}
	}

	if hasSC {
		res, err := ed.txSimulatorProcessor.ProcessTx(tx.GetTransaction())
		if err != nil {
			return nil, err
		}

		if res.FailReason != "" {
			if res.VMOutput != nil && res.VMOutput.ReturnMessage != "" {
				return nil, fmt.Errorf("%w: %s - (%s)", process.ErrInvalidArgument, res.FailReason, res.VMOutput.ReturnMessage)
			}

			return nil, fmt.Errorf("%w: %s", process.ErrInvalidArgument, res.FailReason)
		}

		if res.VMOutput == nil {
			return nil, process.ErrNilVMOutput
		}

		totalGasConsumed := big.NewInt(0)
		for _, log := range res.VMOutput.Logs {
			if string(log.Identifier) == core.TotalConsumedGasString {
				if len(log.Topics) > 0 {
					totalGasConsumed.Add(totalGasConsumed, big.NewInt(0).SetBytes(log.Topics[0]))
				}
			}
		}

		// increase 1 BW for minimum gas consumed to prevent `memory limit reached` error
		estimatedGas += totalGasConsumed.Uint64() + gasMultiplier
	}

	feePerDataByte := ed.proposalController.GetParameterInt(kapps.EnumParameter_FeePerDataByte)

	cost.BandwidthFee *= feePerDataByte

	cost.GasEstimated = estimatedGas

	cost.GasMultiplier = gasMultiplier

	return cost, nil
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (ed *EconomicsData) EpochConfirmed(epoch uint32) {

}

// CheckValidityTxValues checks if the provided transaction is economically correct
func (ed *EconomicsData) CheckValidityTxValues(tx process.TransactionWithFeeHandler, simulateSC bool) (*transaction.CostResponse, error) {
	cost, err := ed.ComputeTransactionCost(tx, simulateSC)
	if err != nil {
		return nil, err
	}

	if tx.GetBandwidthFee() < cost.BandwidthFee ||
		tx.GetKAppFee() < cost.KAppFee {
		return nil, fmt.Errorf("%w: (%d/%d) (%d/%d)", process.ErrInvalidTransactionFees,
			tx.GetBandwidthFee(), cost.BandwidthFee,
			tx.GetKAppFee(), cost.KAppFee,
		)
	}

	return cost, nil
}

// LeaderPercentage will return leader reward percentage
func (ed *EconomicsData) LeaderPercentage() float64 {
	return ed.leaderPercentage
}

// IsInterfaceNil returns true if there is no value under the interface
func (ed *EconomicsData) IsInterfaceNil() bool {
	return ed == nil
}
