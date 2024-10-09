package chain

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common"
	testscommon "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	imock "github.com/klever-io/klever-go/integrationTest/mock"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/sharding/mock"
	"github.com/klever-io/klever-go/vmcommon/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVMConfig() config.VirtualMachineConfig {
	return config.VirtualMachineConfig{
		WasmVMVersions: []config.WasmVMVersionByEpoch{
			{StartEpoch: 0, Version: "v1.5"},
			{StartEpoch: 10, Version: "v1.5"}, // only have one version implemented starting at 1.5
		},
	}
}

func createMockVMAccountsArguments() ArgVMContainerFactory {
	kdaTransferParser, _ := parsers.NewKDATransferParser(&mock.MarshalizerMock{})
	return ArgVMContainerFactory{
		Config:        makeVMConfig(),
		GasSchedule:   notifierMock.NewGasScheduleNotifierMock(kvmConfig.MakeGasMapForTests()),
		EpochNotifier: &testscommon.EpochNotifierStub{},
		// EnableEpochsHandler: &testscommon.EnableEpochsHandlerStub{},
		WasmVMChangeLocker: &sync.RWMutex{},
		KDATransferParser:  kdaTransferParser,
		ForkController:     &imock.ForkControllerStub{},
		BuiltInFunctions:   builtInFunctions.NewBuiltInFunctionContainer(),
		BlockChainHook:     &contextmock.BlockchainHookStub{},
		Hasher:             &mock.HasherMock{},
	}
}

func TestNewVMContainerFactory_NilGasScheduleShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.GasSchedule = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilGasSchedule, err)
}

func TestNewVMContainerFactory_NilKDATransferParserShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.KDATransferParser = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilKDATransferParser, err)
}

func TestNewVMContainerFactory_NilLockerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.WasmVMChangeLocker = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilLocker, err)
}

func TestNewVMContainerFactory_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.EpochNotifier = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilEpochNotifier, err)
}

func TestNewVMContainerFactory_NilEnableEpochsHandlerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.ForkController = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilEnableEpochsHandler, err)
}

func TestNewVMContainerFactory_NilBuiltinFunctionsShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.BuiltInFunctions = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilBuiltInFunction, err)
}

func TestNewVMContainerFactory_NilBlockChainHookShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.BlockChainHook = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilBlockChainHook, err)
}

func TestNewVMContainerFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	args.Hasher = nil
	vmf, err := NewVMContainerFactory(args)

	assert.Nil(t, vmf)
	assert.Equal(t, process.ErrNilHasher, err)
}

func TestNewVMContainerFactory_OkValues(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	vmf, err := NewVMContainerFactory(args)

	assert.NotNil(t, vmf)
	assert.Nil(t, err)
	assert.False(t, vmf.IsInterfaceNil())
}

func TestVmContainerFactory_Create(t *testing.T) {
	t.Parallel()

	args := createMockVMAccountsArguments()
	vmf, _ := NewVMContainerFactory(args)
	require.NotNil(t, vmf)

	container, err := vmf.Create()
	require.Nil(t, err)
	require.NotNil(t, container)
	defer func() {
		_ = container.Close()
	}()

	assert.Nil(t, err)
	assert.NotNil(t, container)

	vm, err := container.Get(common.WasmVirtualMachine)
	assert.Nil(t, err)
	assert.NotNil(t, vm)

	acc := vmf.BlockChainHookImpl()
	assert.NotNil(t, acc)
}

func TestVmContainerFactory_ResolveWasmVMVersion(t *testing.T) {
	epochNotifierInstance := notifier.NewGenericEpochNotifier()

	numCalled := 0
	gasScheduleNotifier := notifierMock.NewGasScheduleNotifierMock(kvmConfig.MakeGasMapForTests())
	gasScheduleNotifier.RegisterNotifyHandlerCalled = func(handler core.GasScheduleSubscribeHandler) {
		numCalled++
		handler.GasScheduleChange(gasScheduleNotifier.GasSchedule)
	}
	args := createMockVMAccountsArguments()
	args.GasSchedule = gasScheduleNotifier
	args.EpochNotifier = epochNotifierInstance
	vmf, _ := NewVMContainerFactory(args)
	require.NotNil(t, vmf)

	container, err := vmf.Create()
	require.Nil(t, err)
	require.NotNil(t, container)
	defer func() {
		_ = container.Close()
	}()
	require.Equal(t, "v1.5", getWasmVMVersion(t, container))

	epochNotifierInstance.CheckEpoch(1)
	require.Equal(t, "v1.5", getWasmVMVersion(t, container))

	epochNotifierInstance.CheckEpoch(999)
	require.Equal(t, "v1.5", getWasmVMVersion(t, container))

	require.Equal(t, numCalled, 1)
}

func getWasmVMVersion(t testing.TB, container process.VirtualMachinesContainer) string {
	vm, err := container.Get(common.WasmVirtualMachine)
	require.Nil(t, err)
	require.NotNil(t, vm)

	return vm.GetVersion()
}

func TestGetMatchingVersion(t *testing.T) {
	tests := []struct {
		name           string
		epoch          uint32
		wasmVMVersions []config.WasmVMVersionByEpoch
		want           config.WasmVMVersionByEpoch
	}{
		{
			name:  "Test epoch within range",
			epoch: 5,
			wasmVMVersions: []config.WasmVMVersionByEpoch{
				{StartEpoch: 0, Version: "v1"},
				{StartEpoch: 10, Version: "v2"},
			},
			want: config.WasmVMVersionByEpoch{StartEpoch: 0, Version: "v1"},
		},
		{
			name:  "Test epoch at boundary",
			epoch: 10,
			wasmVMVersions: []config.WasmVMVersionByEpoch{
				{StartEpoch: 0, Version: "v1"},
				{StartEpoch: 10, Version: "v2"},
				{StartEpoch: 20, Version: "v3"},
			},
			want: config.WasmVMVersionByEpoch{StartEpoch: 10, Version: "v2"},
		},
		{
			name:  "Test epoch beyond range",
			epoch: 30,
			wasmVMVersions: []config.WasmVMVersionByEpoch{
				{StartEpoch: 0, Version: "v1"},
				{StartEpoch: 10, Version: "v2"},
				{StartEpoch: 20, Version: "v3"},
			},
			want: config.WasmVMVersionByEpoch{StartEpoch: 20, Version: "v3"},
		},
		{
			name:           "Test version empty and epoch empty",
			epoch:          30,
			wasmVMVersions: []config.WasmVMVersionByEpoch{},
			want:           config.WasmVMVersionByEpoch{StartEpoch: 0, Version: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmf := &vmContainerFactory{
				wasmVMVersions: tt.wasmVMVersions,
			}
			got := vmf.getMatchingVersion(tt.epoch)
			if got.Version != tt.want.Version {
				t.Errorf("getMatchingVersion() got version = %v, want %v", got.Version, tt.want.Version)
			}
			if got.StartEpoch != tt.want.StartEpoch {
				t.Errorf("getMatchingVersion() got startEpoch = %v, want %v", got.StartEpoch, tt.want.StartEpoch)
			}
		})
	}
}
