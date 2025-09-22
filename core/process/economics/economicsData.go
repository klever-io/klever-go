package economics

import (
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/core/process/txsimulator/data"
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
	if err := ed.validateProposalController(); err != nil {
		return nil, err
	}

	cost := &transaction.CostResponse{
		KAppFee:      0,
		BandwidthFee: tx.GetDataSize(),
	}

	gasMultiplier := ed.proposalController.GetParameterUint(kapps.EnumParameter_GasMultiplier)

	hasSC, err := ed.processContracts(tx, cost)
	if err != nil {
		return nil, err
	}

	if simulateSC && hasSC {
		estimatedGas, err := ed.simulateSmartContract(tx, gasMultiplier)
		if err != nil {
			return nil, err
		}
		cost.GasEstimated = estimatedGas
	}

	ed.applyFeePerDataByte(cost)

	cost.GasMultiplier = gasMultiplier

	// add safety margin of 10%
	cost.SafetyMargin = cost.GasEstimated / 10

	return cost, nil
}

func (ed *EconomicsData) validateProposalController() error {
	if check.IfNil(ed.proposalController) {
		return process.ErrProposalNotInitialized
	}
	return nil
}

func (ed *EconomicsData) processContracts(tx process.TransactionWithFeeHandler, cost *transaction.CostResponse) (bool, error) {
	hasSC := false
	for _, c := range tx.GetContracts() {
		feeType, ok := ContractEnum[c.Type]
		if !ok {
			return hasSC, process.ErrInvalidTransactionType
		}

		value := ed.proposalController.GetParameterInt(feeType)

		cost.KAppFee += value
		cost.BandwidthFee += BaseTxSize

		if c.Type == transaction.TXContract_SmartContractType {
			hasSC = true
		}
	}

	if hasSC && len(tx.GetContracts()) > 1 {
		return hasSC, process.ErrSmartContractFailMaxContracts
	}

	return hasSC, nil
}

func (ed *EconomicsData) simulateSmartContract(tx process.TransactionWithFeeHandler, gasMultiplier uint64) (uint64, error) {
	res, err := ed.txSimulatorProcessor.ProcessTx(tx.GetTransaction())
	if err != nil {
		return 0, err
	}

	if res.FailReason != "" {
		return 0, ed.handleSimulationFailure(res)
	}

	if res.VMOutput == nil {
		return 0, process.ErrNilVMOutput
	}

	totalGasConsumed := res.VMOutput.ComputeTotalGasConsumed()

	// increase 1 BW for minimum gas consumed to prevent `memory limit reached` error
	return totalGasConsumed.Uint64() + gasMultiplier, nil
}

func (ed *EconomicsData) handleSimulationFailure(res *data.SimulationResults) error {
	if res.VMOutput != nil && res.VMOutput.ReturnMessage != "" {
		return fmt.Errorf("%w: %s - (%s)", process.ErrInvalidArgument, res.FailReason, res.VMOutput.ReturnMessage)
	}

	return fmt.Errorf("%w: %s", process.ErrInvalidArgument, res.FailReason)
}

func (ed *EconomicsData) applyFeePerDataByte(cost *transaction.CostResponse) {
	feePerDataByte := ed.proposalController.GetParameterInt(kapps.EnumParameter_FeePerDataByte)
	cost.BandwidthFee *= feePerDataByte
}

// EpochConfirmed is called whenever a new epoch is confirmed
func (ed *EconomicsData) EpochConfirmed(epoch uint32) {

}

// CheckValidityTxValues checks if the provided transaction is economically correct
func (ed *EconomicsData) CheckValidityTxValues(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
	cost, err := ed.ComputeTransactionCost(tx, false)
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

func (ed *EconomicsData) ComputeGas(tx *transaction.Transaction, computedCost *transaction.CostResponse) (uint64, uint64, error) {
	freeBandwidth := tx.GetBandwidthFee() - computedCost.BandwidthFee
	if freeBandwidth < 0 {
		return 0, 0, fmt.Errorf("%w, freeBandwidth: %d",
			process.ErrInvalidTXFees,
			freeBandwidth,
		)
	}

	gasLimit := uint64(freeBandwidth) * uint64(computedCost.GasMultiplier)
	gasMultiplier := uint64(computedCost.GasMultiplier)

	log.Trace("freeBandwidth", "freeBandwidth", freeBandwidth, "gasLimit", tx.GasLimit, "gasMultiplier", tx.GasMultiplier)

	// validate gas limit
	if gasLimit > ed.MaxGasLimitPerTX() {
		return 0, 0, fmt.Errorf("%w, gasLimit: %d, maxGasLimit: %d",
			process.ErrInvalidMaxGasLimitPerTx,
			gasLimit,
			ed.MaxGasLimitPerTX(),
		)
	}

	return gasLimit, gasMultiplier, nil
}

// LeaderPercentage will return leader reward percentage
func (ed *EconomicsData) LeaderPercentage() float64 {
	return ed.leaderPercentage
}

func (ed *EconomicsData) MaxGasLimitPerBlock() uint64 {
	maxGasPerBlock := ed.proposalController.GetParameterUint(kapps.EnumParameter_MaxGasPerBlock)

	return maxGasPerBlock
}

func (ed *EconomicsData) MaxGasLimitPerTX() uint64 {
	maxGasPerTx := ed.proposalController.GetParameterUint(kapps.EnumParameter_MaxGasPerTX)

	return maxGasPerTx
}

// IsInterfaceNil returns true if there is no value under the interface
func (ed *EconomicsData) IsInterfaceNil() bool {
	return ed == nil
}
