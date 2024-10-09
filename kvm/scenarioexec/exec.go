package scenarioexec

import (
	"fmt"
	"os"

	logger "github.com/klever-io/klever-go-logger"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/kapps"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	"github.com/klever-io/klever-go/kvm/executor"
	worldhook "github.com/klever-io/klever-go/kvm/mock/world"
	scencontroller "github.com/klever-io/klever-go/kvm/scenarioexec/controller"
	scenexpressionreconstructor "github.com/klever-io/klever-go/kvm/scenarioexec/expression/reconstructor"
	scenfileresolver "github.com/klever-io/klever-go/kvm/scenarioexec/fileresolver"
	gasSchedules "github.com/klever-io/klever-go/kvm/scenarioexec/gasSchedules"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/tools"
	vmi "github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

var log = logger.GetOrCreate("vm/scenarios")

// TestVMType is the VM type argument we use in tests.
var TestVMType = []byte{0, 0}

// VMTestExecutor parses, interprets and executes both .test.json tests and .scen.json scenarios with VM.
type VMTestExecutor struct {
	World              *worldhook.MockWorld
	vm                 vmi.VMExecutionHandler
	OverrideVMExecutor executor.ExecutorAbstractFactory
	vmHost             vmhost.VMHost
	checkGas           bool
	scenarioTraceGas   []bool
	fileResolver       scenfileresolver.FileResolver
	exprReconstructor  scenexpressionreconstructor.ExprReconstructor
}

var _ scencontroller.TestExecutor = (*VMTestExecutor)(nil)
var _ scencontroller.ScenarioRunner = (*VMTestExecutor)(nil)

// NewVMTestExecutor prepares a new VMTestExecutor instance.
func NewVMTestExecutor() (*VMTestExecutor, error) {
	world := worldhook.NewMockWorld()

	return &VMTestExecutor{
		World:             world,
		vm:                nil,
		checkGas:          true,
		scenarioTraceGas:  make([]bool, 0),
		fileResolver:      nil,
		exprReconstructor: scenexpressionreconstructor.ExprReconstructor{},
	}, nil
}

// InitVM will initialize the VM and the builtin function containscenexpressionreconstructor.
// Does nothing if the VM is already initialized.
func (ae *VMTestExecutor) InitVM(scenGasSchedule scenjsonmodel.GasSchedule) error {
	if ae.vm != nil {
		// clear old vm instances
		ae.vm = nil
		ae.vmHost = nil
	}
	gasSchedule, err := ae.gasScheduleMapFromScenarios(scenGasSchedule)
	if err != nil {
		return err
	}

	epochNotifier := &commonMock.EpochNotifierStub{}

	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:                0,
		ProcessorFlowITOPrice:   0,
		FixStakingBuckets:       0,
		KdaFpr:                  0,
		BigBucketsCompute:       0,
		FPRComputeAndKdaFeeFlow: 0,
		FixDelegationSameEpoch:  0,
		SmartContracts:          0,
	}, epochNotifier)

	err = ae.World.InitBuiltinFunctions(gasSchedule, forkController)
	if err != nil {
		return err
	}

	protectedKeys := [][]byte{
		[]byte(kapps.ProtectedKleverKeyPrefix),
		[]byte(kapps.ProtectedKLVKeyPrefix),
		[]byte(kapps.ProtectedKFIKeyPrefix),
		[]byte(kapps.KDAPrefix),
	}

	kdaTransferParser, _ := parsers.NewKDATransferParser(worldhook.WorldMarshalizer)

	// timeout from environment in ms
	executionTimeout := uint32(2_000)
	if value, ok := os.LookupEnv("KVM_EXECUTION_TIMEOUT"); ok {
		// pase the value to int
		valueUint, err := tools.SafeStringToU32(value)
		if err != nil {
			log.Warn("Failed to parse KVM_EXECUTION_TIMEOUT, using default value")
		} else {
			executionTimeout = valueUint
		}
	}

	vm, err := hostCore.NewVMHost(
		ae.World,
		&vmhost.VMHostParameters{
			VMType:                   TestVMType,
			OverrideVMExecutor:       ae.OverrideVMExecutor,
			GasSchedule:              gasSchedule,
			BuiltInFuncContainer:     ae.World.BuiltinFuncs.Container,
			ProtectedKeyPrefix:       protectedKeys,
			KDATransferParser:        kdaTransferParser,
			WasmerSIGSEGVPassthrough: false,
			Hasher:                   worldhook.DefaultHasher,
			EpochNotifier:            epochNotifier,
			ForkController:           forkController,

			TimeOutForSCExecutionInMilliseconds: executionTimeout,
		})
	if err != nil {
		return err
	}

	ae.vm = vm
	ae.vmHost = vm
	return nil
}

// GetVM yields a reference to the VMExecutionHandler used.
func (ae *VMTestExecutor) GetVM() vmi.VMExecutionHandler {
	return ae.vm
}

func (ae *VMTestExecutor) getVMHost() vmhost.VMHost {
	return ae.vmHost
}

func (ae *VMTestExecutor) gasScheduleMapFromScenarios(scenGasSchedule scenjsonmodel.GasSchedule) (kvmConfig.GasScheduleMap, error) {
	switch scenGasSchedule {
	case scenjsonmodel.GasScheduleDefault:
		return gasSchedules.LoadGasScheduleConfig(gasSchedules.GetV1())
	case scenjsonmodel.GasScheduleDummy:
		return kvmConfig.MakeGasMapForTests(), nil
	case scenjsonmodel.GasScheduleV1:
		return gasSchedules.LoadGasScheduleConfig(gasSchedules.GetV1())
	default:
		return nil, fmt.Errorf("unknown scenario GasSchedule: %d", scenGasSchedule)
	}
}

// PeekTraceGas returns the last position from the scenarioTraceGas, if existing
func (ae *VMTestExecutor) PeekTraceGas() bool {
	length := len(ae.scenarioTraceGas)
	if length != 0 {
		return ae.scenarioTraceGas[length-1]
	}
	return false
}
