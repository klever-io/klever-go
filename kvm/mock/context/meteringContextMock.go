package mock

import (
	"github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ vmhost.MeteringContext = (*MeteringContextMock)(nil)

// MeteringContextMock is used in tests to check the MeteringContext interface method calls
type MeteringContextMock struct {
	GasCost           *config.GasCost
	GasLeftMock       uint64
	GasProvidedMock   uint64
	GasComputedToLock uint64
	BlockGasLimitMock uint64
	Err               error
}

// InitState mocked method
func (m *MeteringContextMock) InitState() {}

// PushState mocked method
func (m *MeteringContextMock) PushState() {}

// PopSetActiveState mocked method
func (m *MeteringContextMock) PopSetActiveState() {}

// PopMergeActiveState mocked method
func (m *MeteringContextMock) PopMergeActiveState() {}

// PopDiscard mocked method
func (m *MeteringContextMock) PopDiscard() {}

// ClearStateStack mocked method
func (m *MeteringContextMock) ClearStateStack() {}

// SetGasSchedule mocked method
func (m *MeteringContextMock) SetGasSchedule(gasSchedule config.GasScheduleMap) {
	gasCostConfig, _ := config.CreateGasConfig(gasSchedule)
	m.GasCost = gasCostConfig
}

// GasSchedule mocked method
func (m *MeteringContextMock) GasSchedule() *config.GasCost {
	return m.GasCost
}

// UseGas mocked method
func (m *MeteringContextMock) UseGas(gas uint64) {
	if gas > m.GasLeftMock {
		m.GasLeftMock = 0
		return
	}

	if m.GasLeftMock > 1<<63 {
		m.GasLeftMock = 1 << 63
	}

	m.GasLeftMock -= gas
}

// UseAndTraceGas mocked method
func (m *MeteringContextMock) UseAndTraceGas(_ uint64) {}

// UseGasAndAddTracedGas mocked method
func (m *MeteringContextMock) UseGasAndAddTracedGas(_ string, gasToUse uint64) {
	m.UseGas(gasToUse)
}

// UseGasBoundedAndAddTracedGas -
func (m *MeteringContextMock) UseGasBoundedAndAddTracedGas(_ string, gasToUse uint64) error {
	m.UseGas(gasToUse)
	return nil
}

// RestoreGas mocked method
func (m *MeteringContextMock) RestoreGas(gas uint64) {
	m.GasLeftMock += gas
}

// GasLeft mocked method
func (m *MeteringContextMock) GasLeft() uint64 {
	return m.GasLeftMock
}

// UpdateGasStateOnSuccess mocked method
func (m *MeteringContextMock) UpdateGasStateOnSuccess(_ *vmcommon.VMOutput) error {
	return nil
}

// UpdateGasStateOnFailure mocked method
func (m *MeteringContextMock) UpdateGasStateOnFailure(_ *vmcommon.VMOutput) {}

// InitStateFromContractCallInput mocked method
func (m *MeteringContextMock) InitStateFromContractCallInput(_ *vmcommon.VMInput) {}

// TrackGasUsedByOutOfVMFunction mocked method
func (m *MeteringContextMock) TrackGasUsedByOutOfVMFunction(_ *vmcommon.ContractCallInput, _ *vmcommon.VMOutput) {
}

// GasUsedByContract mocked method
func (m *MeteringContextMock) GasUsedByContract() (uint64, uint64) {
	return 0, 0
}

// GasUsedForExecution mocked method
func (m *MeteringContextMock) GasUsedForExecution() uint64 {
	return 0
}

// GasSpentByContract mocked method
func (m *MeteringContextMock) GasSpentByContract() uint64 {
	return 0
}

// GetGasForExecution mocked method
func (m *MeteringContextMock) GetGasForExecution() uint64 {
	return 0
}

// GetGasProvided mocked method
func (m *MeteringContextMock) GetGasProvided() uint64 {
	return 0
}

// GetSCPrepareInitialCost mocked method
func (m *MeteringContextMock) GetSCPrepareInitialCost() uint64 {
	return 0
}

// BoundGasLimit mocked method
func (m *MeteringContextMock) BoundGasLimit(value int64) uint64 {
	gasLeft := m.GasLeft()

	if value < 0 {
		return 0
	}
	limit := uint64(value)

	if gasLeft < limit {
		return gasLeft
	}
	return limit
}

// UseGasBounded mocked method
func (m *MeteringContextMock) UseGasBounded(gas uint64) error {
	if m.Err != nil {
		return m.Err
	}
	if m.GasLeft() <= gas {
		return vmhost.ErrNotEnoughGas
	}
	m.UseGas(gas)
	return nil
}

// BlockGasLimit mocked method
func (m *MeteringContextMock) BlockGasLimit() uint64 {
	return m.BlockGasLimitMock
}

// DeductInitialGasForExecution mocked method
func (m *MeteringContextMock) DeductInitialGasForExecution(_ []byte) error {
	return m.Err
}

// DeductInitialGasForDirectDeployment mocked method
func (m *MeteringContextMock) DeductInitialGasForDirectDeployment(_ vmhost.CodeDeployInput) error {
	return m.Err
}

// DeductInitialGasForIndirectDeployment mocked method
func (m *MeteringContextMock) DeductInitialGasForIndirectDeployment(_ vmhost.CodeDeployInput) error {
	return m.Err
}

// EnableRestoreGas mocked method
func (m *MeteringContextMock) EnableRestoreGas() {}

// DisableRestoreGas mocked method
func (m *MeteringContextMock) DisableRestoreGas() {}

// StartGasTracing mocked method
func (m *MeteringContextMock) StartGasTracing(_ string) {}

// SetGasTracing mocked method
func (m *MeteringContextMock) SetGasTracing(_ bool) {}

// GetGasTrace returns nil
func (m *MeteringContextMock) GetGasTrace() map[string]map[string][]uint64 {
	return nil
}
