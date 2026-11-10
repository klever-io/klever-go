package kapps

import (
	reflect "reflect"
	"sync"
	"sync/atomic"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
)

var log = logger.GetOrCreate("kapps/proposal")

// activeProposalController is the live, node-wide parameter set. The block
// processor writes it during epoch start while API handlers and the fee path
// read it from other goroutines, so every access goes through mu and callers
// only ever get copies. pc is a named, unexported field rather than an embedded
// one so that neither the map nor the proto's lock-free methods are reachable
// from outside.
type activeProposalController struct {
	mu sync.RWMutex
	pc *ProposalController
}

type ActiveProposalController interface {
	IsInterfaceNil() bool
	GetActiveParameters() map[int32]*Parameter
	GetParameterInt(parameter EnumParameter) int64
	GetParameterUint(parameter EnumParameter) uint64
	ParseParamAndValidate(parameter EnumParameter, value []byte, fc core.ForkController) (reflect.Value, error)
	UpdateParameters(map[int32]*Parameter)
	RestoreParameters(map[int32]*Parameter)
}

// ProposalControllerHolder publishes an ActiveProposalController reference safely.
// The reference is wired in while the node starts consensus, by which time API
// goroutines are already reading it. The zero value means "not wired in yet".
type ProposalControllerHolder struct {
	ref atomic.Pointer[ActiveProposalController]
}

// Set publishes the controller for concurrent readers
func (h *ProposalControllerHolder) Set(controller ActiveProposalController) {
	h.ref.Store(&controller)
}

// Get returns the published controller, or nil while none has been wired in
func (h *ProposalControllerHolder) Get() ActiveProposalController {
	controller := h.ref.Load()
	if controller == nil {
		return nil
	}

	return *controller
}

// NewProposalController returns a live controller holding the initial parameters for forks.
func NewProposalController(forks core.ForkController) (*activeProposalController, error) {
	return NewActiveProposalController(NewInitialProposalController(forks)), nil
}

// NewInitialProposalController returns the proposal state a fresh chain starts from.
func NewInitialProposalController(forks core.ForkController) *ProposalController {
	return &ProposalController{
		ProposalCount:    0,
		ActiveParameters: InitialProposalParameters(forks),
		ActiveProposals:  make(map[uint32]*ActiveProposals),
	}
}

// NewActiveProposalController wraps pc as the live controller and takes ownership
// of it: the caller must not read or write pc afterwards.
func NewActiveProposalController(pc *ProposalController) *activeProposalController {
	if pc == nil {
		pc = &ProposalController{}
	}
	if pc.ActiveParameters == nil {
		pc.ActiveParameters = make(map[int32]*Parameter)
	}

	return &activeProposalController{pc: pc}
}

// GetActiveParameters returns a deep copy: mutating it never touches the live set.
func (a *activeProposalController) GetActiveParameters() map[int32]*Parameter {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return cloneParameters(a.pc.ActiveParameters)
}

func (a *activeProposalController) GetParameterInt(parameter EnumParameter) int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.pc.GetParameterInt(parameter)
}

func (a *activeProposalController) GetParameterUint(parameter EnumParameter) uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.pc.GetParameterUint(parameter)
}

func (a *activeProposalController) ParseParamAndValidate(parameter EnumParameter, value []byte, fc core.ForkController) (reflect.Value, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.pc.ParseParamAndValidate(parameter, value, fc)
}

// UpdateParameters merges params into the live set. Each entry replaces the
// whole live Parameter (Type and Value), and nil entries are skipped. Values are
// copied in, so the caller's objects (typically the trie-loaded controller) are
// never aliased.
func (a *activeProposalController) UpdateParameters(params map[int32]*Parameter) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, param := range params {
		if param == nil {
			continue
		}

		a.pc.ActiveParameters[key] = cloneParameter(param)
	}
}

// RestoreParameters replaces the whole live set in one step, so a concurrent
// reader sees either the previous set or params, never a mix.
func (a *activeProposalController) RestoreParameters(params map[int32]*Parameter) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.pc.ActiveParameters = cloneParameters(params)
}

// IsInterfaceNil returns true if there is no value under the interface
func (a *activeProposalController) IsInterfaceNil() bool {
	return a == nil
}

func cloneParameters(params map[int32]*Parameter) map[int32]*Parameter {
	clone := make(map[int32]*Parameter, len(params))
	for key, param := range params {
		if param == nil {
			continue
		}

		clone[key] = cloneParameter(param)
	}

	return clone
}

func cloneParameter(param *Parameter) *Parameter {
	value := make([]byte, len(param.Value))
	copy(value, param.Value)

	return &Parameter{Type: param.Type, Value: value}
}

func InitialProposalParameters(forks core.ForkController) map[int32]*Parameter {
	activeParameters := make(map[int32]*Parameter)

	activeParameters[int32(EnumParameter_FeePerDataByte)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("4000"), // 4_000 => 250(base tx size) * 4K = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeCreateAsset)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("20000000000"), // 20_000_000_000 = 20K KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeCreateValidator)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("50000000000"), // 50_000_000_000 = 50K KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeTransfer)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("500000"), // 500_000 = 0.5 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeAssetTrigger)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("2000000"), // 2_000_000 = 2 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeValidatorConfig)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000000"), // 1_000_000_000 = 1000 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeFreeze)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeUnfreeze)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeDelegate)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeUndelegate)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeWithdraw)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeClaim)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeUnjail)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("10000000000"), // 10_000_000_000 = 10K KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeSetAccountName)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("100000000"), // 100_000_000 = 100 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeProposal)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("500000000"), // 500_000_000 = 500 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeVote)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeConfigITO)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("20000000000"), // 20_000_000_000 = 20K KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeSetITOPrices)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeBuy)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000"), // 1_000_000 = 1 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeSell)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("10000000"), // 10_000_000 = 10 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeCancelMarketOrder)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("50000000"), // 50_000_000 = 50 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeCreateMarketplace)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("50000000000"), // 50_000_000_000 = 50K KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeConfigMarketplace)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000000"), // 1_000_000_000 = 1000 KLV
	}
	activeParameters[int32(EnumParameter_KAppFeeUpdateAccountPermission)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000000"), // 1_000_000_000 = 1000 KLV
	}
	activeParameters[int32(EnumParameter_MaxEpochsUnclaimed)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("100"), // 100 epochs
	}
	activeParameters[int32(EnumParameter_MinSelfDelegatedAmount)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1500000000000"), // 1_500_000_000_000 = 1.5M KLV
	}
	activeParameters[int32(EnumParameter_MinTotalDelegatedAmount)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("10000000000000"), // 10_000_000_000_000 = 10M KLV
	}
	activeParameters[int32(EnumParameter_BlockRewards)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("0"), // 0 KLV initally
	}
	activeParameters[int32(EnumParameter_StakingRewards)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("0"), // 0 KLV initally
	}
	activeParameters[int32(EnumParameter_MaxNFTMintBatch)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("50"), // 50 NFT's mints per transaction
	}
	activeParameters[int32(EnumParameter_MinKFIStakedToEnableProposals)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000000000"), //1_000_000_000_000 = 1M KFI
	}
	activeParameters[int32(EnumParameter_MinKLVBucketAmount)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("1000000000"), // 1_000_000_000 = 1000 KLV
	}
	activeParameters[int32(EnumParameter_MaxBucketSize)] = &Parameter{
		Type:  EnumType_Int64,
		Value: []byte("100"), // 100 buckets
	}
	activeParameters[int32(EnumParameter_LeaderValidatorRewardsPercentage)] = &Parameter{
		Type:  EnumType_Uint32,
		Value: []byte("6000"), // 6000/10000(core.HundredPercent precision) = 0.6 = 60%
	}
	activeParameters[int32(EnumParameter_ProposalMaxEpochsDuration)] = &Parameter{
		Type:  EnumType_Uint32,
		Value: []byte("40"), // 40 epochs
	}

	if forks.KdaFpr() {
		activeParameters[int32(EnumParameter_KAppFeeITOTrigger)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("2000000"), // 2_000_000 = 2 KLV
		}
		activeParameters[int32(EnumParameter_KAppFeeDeposit)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("10000000"), // 10_000_000 = 10 KLV
		}
	}

	if forks.EnableSmartContracts() {
		activeParameters[int32(EnumParameter_KAppFeeSmartContract)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("2000000"), // 2_000_000 = 2 KLV
		}

		activeParameters[int32(EnumParameter_GasMultiplier)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("10"), //10
		}

		activeParameters[int32(EnumParameter_MaxGasPerBlock)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("1500000000"), // 1_500_000_000
		}

		activeParameters[int32(EnumParameter_MaxGasPerTX)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("100000000"), // 100_000_000
		}
	}
	if forks.FixAuditChanges() {
		activeParameters[int32(EnumParameter_MaxGasPerTX)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("400000000"), // 400_000_000 at multiplier 1 = 400 KLV max cost
		}
		activeParameters[int32(EnumParameter_GasMultiplier)] = &Parameter{
			Type:  EnumType_Int64,
			Value: []byte("1"), // 1 -> 1 gas unit = 1 klv minimum unit
		}
	}

	return activeParameters
}

type ProposalParameters map[EnumParameter]reflect.Value

func (p *ProposalController) ParseParam(parameter EnumParameter, value []byte) (reflect.Value, error) {
	param := p.ActiveParameters[int32(parameter)]
	if param == nil {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	parser, ok := parsers[EnumType(param.Type)]
	if !ok {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	return parser(value)
}

// ParseParamAndValidate parses the value and validates it against the constraints
func (p *ProposalController) ParseParamAndValidate(parameter EnumParameter, value []byte, fc core.ForkController) (reflect.Value, error) {
	param := p.ActiveParameters[int32(parameter)]
	if param == nil {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	parser, ok := parsers[EnumType(param.Type)]
	if !ok {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	result, err := parser(value)
	if err != nil {
		return reflect.Value{}, err
	}

	return result, p.validateConstraints(parameter, result, fc)
}

// Ceilings derived from existing limits, so the arithmetic downstream cannot overflow.
const (
	// a block holds at most MaxKappIterator transactions at the per-tx gas ceiling
	maxGasPerBlock = int64(core.MaxGasLimitPerTx) * core.MaxKappIterator
)

// Policy ceilings, orders of magnitude above every live value, so one proposal cannot
// brick the chain.
const (
	// the multiplier scales every tx's gas budget; the live value is 1
	maxGasMultiplier = 10_000

	// proposalKapp.finalizeProposal computes EpochEnd = Epoch + EpochsDuration as uint32,
	// and a wrapped EpochEnd never settles
	maxProposalEpochsDuration = 1_000

	// BlockRewards and StakingRewards are each minted on every block, unchecked against
	// MaxSupply. 1000 KLV is ~66x the live 15 KLV; both at the ceiling take ~4.6e9 blocks
	// to overflow the int64 supply accounting, rather than ~460 at MaxKLVSupply.
	maxEmissionPerBlock = 1_000_000_000

	// Every KApp fee shares one ceiling, 20x the dearest genesis fee (50K KLV). Proposing,
	// voting and freezing included: at 1M KLV they stay payable, by choice, so a captured
	// majority cannot price out the proposal that undoes it.
	maxKAppFee = 1_000_000_000_000 // 1M KLV

	// paid per byte by every tx, the proposal that undoes it included
	maxFeePerDataByte = 4_000_000 // a 250 byte tx costs 1000 KLV

	maxKLVSupply = 10_000_000_000_000_000 // 10B KLV at precision 6
	maxKFISupply = 21_000_000_000_000     // 21M KFI at precision 6
)

type paramLimits struct {
	min, max int64
}

var (
	// Fees floor at 0: zero is a legitimate choice, a negative fee inverts the cost of
	// every transaction that pays it.
	kappFeeLimits = paramLimits{0, maxKAppFee}

	// Per-block emission, summed into the circulating supply.
	emissionLimits = paramLimits{0, maxEmissionPerBlock}

	// Compared against a balance or a stake, so at or below zero everyone meets them.
	klvStakeLimits = paramLimits{1, maxKLVSupply}

	// Both a make() capacity and a loop bound with a trie write per iteration.
	iterationLimits = paramLimits{1, core.MaxKappIterator}
)

// paramBounds is the range a proposal may set each parameter to from FixAuditChangesV6.
// A parameter without a row is rejected, so one cannot be added without bounds.
var paramBounds = map[EnumParameter]paramLimits{
	EnumParameter_FeePerDataByte: {0, maxFeePerDataByte},

	EnumParameter_KAppFeeCreateValidator:         kappFeeLimits,
	EnumParameter_KAppFeeCreateAsset:             kappFeeLimits,
	EnumParameter_KAppFeeTransfer:                kappFeeLimits,
	EnumParameter_KAppFeeAssetTrigger:            kappFeeLimits,
	EnumParameter_KAppFeeValidatorConfig:         kappFeeLimits,
	EnumParameter_KAppFeeFreeze:                  kappFeeLimits,
	EnumParameter_KAppFeeUnfreeze:                kappFeeLimits,
	EnumParameter_KAppFeeDelegate:                kappFeeLimits,
	EnumParameter_KAppFeeUndelegate:              kappFeeLimits,
	EnumParameter_KAppFeeWithdraw:                kappFeeLimits,
	EnumParameter_KAppFeeClaim:                   kappFeeLimits,
	EnumParameter_KAppFeeUnjail:                  kappFeeLimits,
	EnumParameter_KAppFeeSetAccountName:          kappFeeLimits,
	EnumParameter_KAppFeeProposal:                kappFeeLimits,
	EnumParameter_KAppFeeVote:                    kappFeeLimits,
	EnumParameter_KAppFeeConfigITO:               kappFeeLimits,
	EnumParameter_KAppFeeSetITOPrices:            kappFeeLimits,
	EnumParameter_KAppFeeBuy:                     kappFeeLimits,
	EnumParameter_KAppFeeSell:                    kappFeeLimits,
	EnumParameter_KAppFeeCancelMarketOrder:       kappFeeLimits,
	EnumParameter_KAppFeeCreateMarketplace:       kappFeeLimits,
	EnumParameter_KAppFeeConfigMarketplace:       kappFeeLimits,
	EnumParameter_KAppFeeUpdateAccountPermission: kappFeeLimits,
	EnumParameter_KAppFeeITOTrigger:              kappFeeLimits,
	EnumParameter_KAppFeeDeposit:                 kappFeeLimits,
	EnumParameter_KAppFeeSmartContract:           kappFeeLimits,

	EnumParameter_BlockRewards:   emissionLimits,
	EnumParameter_StakingRewards: emissionLimits,

	EnumParameter_MinSelfDelegatedAmount:  klvStakeLimits,
	EnumParameter_MinTotalDelegatedAmount: klvStakeLimits,
	EnumParameter_MinKLVBucketAmount:      klvStakeLimits,

	// Any value in range, even above the KFI staked or circulating: governance then pauses
	// until enough KFI is frozen. An unreachable value needs half the stake to pass, and a
	// fork can reset it.
	EnumParameter_MinKFIStakedToEnableProposals: {1, maxKFISupply},

	// the FPR slices keep one entry per epoch up to this, re-marshalled on every block and
	// walked on every claim
	EnumParameter_MaxEpochsUnclaimed: iterationLimits,

	EnumParameter_MaxNFTMintBatch: iterationLimits,
	EnumParameter_MaxBucketSize:   iterationLimits,

	EnumParameter_ProposalMaxEpochsDuration:        {1, maxProposalEpochsDuration},
	EnumParameter_LeaderValidatorRewardsPercentage: {0, int64(core.HundredPercent)},

	// MaxGasPerBlock is floored at the MaxGasPerTX ceiling, so a block always fits the largest tx
	EnumParameter_GasMultiplier:  {1, maxGasMultiplier},
	EnumParameter_MaxGasPerBlock: {core.MaxGasLimitPerTx, maxGasPerBlock},
	EnumParameter_MaxGasPerTX:    {core.MinGasLimit, core.MaxGasLimitPerTx},
}

func validateBounds(parameter EnumParameter, value reflect.Value) error {
	limits, ok := paramBounds[parameter]
	if !ok {
		log.Error("parameter has no bounds", "parameter", parameter)
		return common.ErrInvalidParameter
	}

	var v int64
	if value.CanInt() {
		v = value.Int()
	} else {
		// only Uint32 parameters are unsigned; a wrapped value goes negative and fails the floor
		v = int64(value.Uint()) // #nosec G115
	}

	if v < limits.min || v > limits.max {
		log.Error("invalid parameter", "parameter", parameter, "value", v, "min", limits.min, "max", limits.max)
		return common.ErrInvalidParameter
	}

	return nil
}

func (p *ProposalController) validateConstraints(parameter EnumParameter, value reflect.Value, fc core.ForkController) error {
	if fc.FixAuditChangesV6() {
		return validateBounds(parameter, value)
	}

	switch parameter {
	case EnumParameter_LeaderValidatorRewardsPercentage:
		if value.Uint() > uint64(core.HundredPercent) {
			log.Error("invalid LeaderValidatorRewardsPercentage", "value", value.Uint(), "max", core.HundredPercent)
			return common.ErrInvalidParameter
		}
	case EnumParameter_GasMultiplier:
		if value.Int() < 1 {
			log.Error("invalid GasMultiplier", "value", value.Int(), "min", 1)
			return common.ErrInvalidParameter
		}
	case EnumParameter_MaxGasPerBlock:
		if value.Int() < core.MinGasLimit {
			log.Error("invalid MaxGasPerBlock", "value", value.Int(), "min", core.MinGasLimit)
			return common.ErrInvalidParameter
		}
	case EnumParameter_MaxGasPerTX:
		maxValue := int64(core.MaxGasLimitPerTx)
		if !fc.FixAuditChanges() {
			maxValue = core.MaxGasBandwidthPerBatchPerSender
		}

		if value.Int() < core.MinGasLimit || value.Int() > maxValue {
			log.Error("invalid MaxGasPerTX", "value", value.Int(), "min", core.MinGasLimit, "max", maxValue)
			return common.ErrInvalidParameter
		}
	}

	return nil
}

func (p *ProposalController) GetParameterInt(parameter EnumParameter) int64 {
	parameterValue, ok := p.ActiveParameters[int32(parameter)]
	if !ok {
		return 0
	}

	value, err := p.ParseParam(parameter, parameterValue.GetValue())
	if err != nil {
		return 0
	}

	return value.Int()
}

// GetParameterUint returns the parameter value as uint64
// if the parameter is not found or does not match constrains it returns 0
func (p *ProposalController) GetParameterUint(parameter EnumParameter) uint64 {
	parameterValue, ok := p.ActiveParameters[int32(parameter)]
	if !ok {
		return 0
	}

	value, err := p.ParseParam(parameter, parameterValue.GetValue())
	if err != nil {
		return 0
	}

	if value.Kind() == reflect.Int64 {
		return uint64(value.Int()) // #nosec G115
	}

	return value.Uint()
}

// IsInterfaceNil returns true if there is no value under the interface
func (p *ProposalController) IsInterfaceNil() bool {
	return p == nil
}
