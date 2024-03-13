package kapps

import (
	reflect "reflect"
	strconv "strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
)

type activeProposalController struct {
	*ProposalController
}

type ActiveProposalController interface {
	IsInterfaceNil() bool
	GetParameters() (ProposalParameters, error)
	GetActiveParameters() map[int32]*Parameter
	GetParameter(parameter EnumParameter) (reflect.Value, error)
	GetParameterInt(parameter EnumParameter) int64
	GetParameterUint(parameter EnumParameter) uint64
	UpdateParameters(map[int32]*Parameter)
}

func NewProposalController(forks core.ForkController) (*activeProposalController, error) {
	return &activeProposalController{
		ProposalController: &ProposalController{
			ProposalCount:    0,
			ActiveParameters: InitialProposalParameters(forks),
			ActiveProposals:  make(map[uint32]*ActiveProposals),
		}}, nil
}

func (a *activeProposalController) UpdateParameters(params map[int32]*Parameter) {
	//After All processing, updates block activeParams instance
	for key, p := range params {
		param := a.ProposalController.ActiveParameters[key]
		if param == nil {
			a.ProposalController.ActiveParameters[key] = p
			continue
		}

		param.Value = make([]byte, len(p.Value))
		copy(a.ProposalController.ActiveParameters[key].Value, p.Value)
	}
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
			Type:  EnumType_Uint64,
			Value: []byte("10"), //10
		}
	}

	return activeParameters
}

type ProposalParameters map[EnumParameter]reflect.Value

func (p ProposalParameters) GetInt64(key EnumParameter) int64 {
	return p[key].Int()
}

func (p ProposalParameters) GetUint32(key EnumParameter) uint32 {
	return uint32(p[key].Uint())
}

func (p *ProposalController) Validate(parameter EnumParameter, value []byte) (reflect.Value, error) {
	if p.ActiveParameters[int32(parameter)] == nil {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	var result reflect.Value
	switch EnumType(p.ActiveParameters[int32(parameter)].Type) {
	case EnumType_Bool:
		v, err := strconv.ParseBool(string(value))
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Int8:
		v, err := strconv.ParseInt(string(value), 10, 8)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Int16:
		v, err := strconv.ParseInt(string(value), 10, 16)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Int32:
		v, err := strconv.ParseInt(string(value), 10, 32)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Int64:
		v, err := strconv.ParseInt(string(value), 10, 64)

		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Uint8:
		v, err := strconv.ParseUint(string(value), 10, 8)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Uint16:
		v, err := strconv.ParseUint(string(value), 10, 16)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Uint32:
		v, err := strconv.ParseUint(string(value), 10, 32)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Uint64:
		v, err := strconv.ParseUint(string(value), 10, 64)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Float32:
		v, err := strconv.ParseFloat(string(value), 32)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Float64:
		v, err := strconv.ParseFloat(string(value), 64)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Complex64:
		v, err := strconv.ParseComplex(string(value), 64)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_Complex128:
		v, err := strconv.ParseComplex(string(value), 128)
		if err != nil {
			return result, err
		}
		result = reflect.ValueOf(v)
	case EnumType_String:
		result = reflect.ValueOf(string(value))
	case EnumType_Bytes:
		result = reflect.ValueOf(value)
	default:
		return reflect.Value{}, common.ErrInvalidParameter
	}

	return result, p.validateConstraints(parameter, result)
}

func (p *ProposalController) validateConstraints(parameter EnumParameter, value reflect.Value) error {
	switch parameter {
	case EnumParameter_LeaderValidatorRewardsPercentage:
		v := uint32(value.Uint())
		if v > core.HundredPercent {
			return common.ErrInvalidParameter
		}
	}

	return nil
}

func (p *ProposalController) GetParameter(parameter EnumParameter) (reflect.Value, error) {
	parameterValue, ok := p.ActiveParameters[int32(parameter)]
	if !ok {
		return reflect.Value{}, common.ErrInvalidParameter
	}

	return p.Validate(parameter, parameterValue.GetValue())
}

func (p *ProposalController) GetParameterInt(parameter EnumParameter) int64 {
	parameterValue, ok := p.ActiveParameters[int32(parameter)]
	if !ok {
		return 0
	}

	value, err := p.Validate(parameter, parameterValue.GetValue())
	if err != nil {
		return 0
	}

	return value.Int()
}

func (p *ProposalController) GetParameterUint(parameter EnumParameter) uint64 {
	parameterValue, ok := p.ActiveParameters[int32(parameter)]
	if !ok {
		return 0
	}

	value, err := p.Validate(parameter, parameterValue.GetValue())
	if err != nil {
		return 0
	}

	if value.Kind() == reflect.Int64 {
		return uint64(value.Int())
	}

	return value.Uint()
}

func (p *ProposalController) GetParameters() (ProposalParameters, error) {
	parameters := make(ProposalParameters, len(p.ActiveParameters))
	for i := range p.ActiveParameters {
		value, err := p.Validate(EnumParameter(i), p.ActiveParameters[i].Value)
		if err != nil {
			return nil, err
		}

		parameters[EnumParameter(i)] = value
	}

	return parameters, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (p *ProposalController) IsInterfaceNil() bool {
	return p == nil
}
