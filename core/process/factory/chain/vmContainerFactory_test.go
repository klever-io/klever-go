package chain

import (
	"sync"
	"testing"
	"time"

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
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/sharding/mock"
	"github.com/klever-io/klever-go/vmcommon"
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

func TestVmContainerFactory_GasScheduleChangeRejectsIncompleteScheduleBeforeClearingCodes(t *testing.T) {
	t.Parallel()

	vmf := &vmContainerFactory{}

	incomplete := kvmConfig.MakeGasMapForTests()
	delete(incomplete["WASMOpcodeCost"], "I32Add")

	// rejected before the compiled codes are cleared: this factory has no blockchain hook to clear
	require.NotPanics(t, func() { vmf.GasScheduleChange(incomplete) })
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

func TestVmContainerFactory_KeepsWasmVMWhenVersionMatchesHost(t *testing.T) {
	for _, version := range []string{vmhost.VMVersion, "*", "v1.0", "unknown"} {
		t.Run(version, func(t *testing.T) {
			epochNotifierInstance := notifier.NewGenericEpochNotifier()

			args := createMockVMAccountsArguments()
			args.Config = config.VirtualMachineConfig{
				WasmVMVersions: []config.WasmVMVersionByEpoch{{StartEpoch: 0, Version: version}},
			}
			args.EpochNotifier = epochNotifierInstance
			vmf, err := NewVMContainerFactory(args)
			require.Nil(t, err)

			container, err := vmf.Create()
			require.Nil(t, err)
			defer func() {
				_ = container.Close()
			}()
			initialVM := getWasmVM(t, container)

			vmf.EpochConfirmed(epochNotifierInstance.CurrentEpoch())
			require.Same(t, initialVM, getWasmVM(t, container))

			epochNotifierInstance.CheckEpoch(1)
			require.Same(t, initialVM, getWasmVM(t, container))

			epochNotifierInstance.CheckEpoch(999)
			require.Same(t, initialVM, getWasmVM(t, container))
		})
	}
}

func TestVmContainerFactory_ReplacesWasmVMWhenRunningVersionDiffersFromResolvedHost(t *testing.T) {
	epochNotifierInstance := notifier.NewGenericEpochNotifier()
	blockChainHook := &compiledCodesClearCounter{BlockchainHookStub: &contextmock.BlockchainHookStub{}}

	args := createMockVMAccountsArguments()
	args.Config = config.VirtualMachineConfig{
		WasmVMVersions: []config.WasmVMVersionByEpoch{{StartEpoch: 0, Version: "v1.0"}},
	}
	args.EpochNotifier = epochNotifierInstance
	args.BlockChainHook = blockChainHook
	vmf, err := NewVMContainerFactory(args)
	require.Nil(t, err)

	container, err := vmf.Create()
	require.Nil(t, err)
	defer func() {
		_ = container.Close()
	}()

	staleVM := &contextmock.VMExecutionHandlerStub{
		GetVersionCalled: func() string {
			return "v0.9"
		},
	}
	require.Nil(t, container.Replace(common.WasmVirtualMachine, staleVM))
	clearsBeforeReplace := blockChainHook.numClears

	epochNotifierInstance.CheckEpoch(1)
	replacedVM := getWasmVM(t, container)
	require.NotSame(t, staleVM, replacedVM)
	require.Equal(t, vmhost.VMVersion, replacedVM.GetVersion())
	require.Equal(t, clearsBeforeReplace+1, blockChainHook.numClears)

	epochNotifierInstance.CheckEpoch(2)
	require.Same(t, replacedVM, getWasmVM(t, container))
}

func TestVmContainerFactory_WasmVMReplacementTakesChangeLock(t *testing.T) {
	epochNotifierInstance := notifier.NewGenericEpochNotifier()
	changeLocker := &sync.RWMutex{}

	args := createMockVMAccountsArguments()
	args.EpochNotifier = epochNotifierInstance
	args.WasmVMChangeLocker = changeLocker
	vmf, err := NewVMContainerFactory(args)
	require.Nil(t, err)

	container, err := vmf.Create()
	require.Nil(t, err)
	defer func() {
		_ = container.Close()
	}()

	staleVM := &contextmock.VMExecutionHandlerStub{
		GetVersionCalled: func() string {
			return "v0.9"
		},
	}
	require.Nil(t, container.Replace(common.WasmVirtualMachine, staleVM))

	changeLocker.RLock()
	done := make(chan struct{})
	go func() {
		epochNotifierInstance.CheckEpoch(1)
		close(done)
	}()

	select {
	case <-done:
		changeLocker.RUnlock()
		t.Fatal("the Wasm VM was replaced without taking the change lock")
	case <-time.After(200 * time.Millisecond):
	}

	changeLocker.RUnlock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the Wasm VM replacement did not finish after the change lock was released")
	}
	require.NotSame(t, staleVM, getWasmVM(t, container))
}

type compiledCodesClearCounter struct {
	*contextmock.BlockchainHookStub
	numClears int
}

func (counter *compiledCodesClearCounter) ClearCompiledCodes() {
	counter.numClears++
}

func TestVmContainerFactory_WasmVMClearsCompiledCodesAtActivationEpoch(t *testing.T) {
	testCases := []struct {
		name            string
		enableEpochs    config.EnableEpochs
		confirmedEpochs []uint32
		expectedClears  []int
	}{
		{
			name:            "activation epoch reached",
			enableEpochs:    config.EnableEpochs{SmartContracts: 5},
			confirmedEpochs: []uint32{4, 5, 6},
			expectedClears:  []int{0, 1, 1},
		},
		{
			name:            "jump over the activation epoch",
			enableEpochs:    config.EnableEpochs{SmartContracts: 5},
			confirmedEpochs: []uint32{9},
			expectedClears:  []int{1},
		},
		{
			name:            "moving back over the activation epoch",
			enableEpochs:    config.EnableEpochs{SmartContracts: 5},
			confirmedEpochs: []uint32{6, 4},
			expectedClears:  []int{1, 2},
		},
		{
			name:            "no activation epoch configured",
			enableEpochs:    config.EnableEpochs{},
			confirmedEpochs: []uint32{4, 5, 9, 3},
			expectedClears:  []int{0, 0, 0, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			epochNotifierInstance := notifier.NewGenericEpochNotifier()
			blockChainHook := &compiledCodesClearCounter{BlockchainHookStub: &contextmock.BlockchainHookStub{}}

			args := createMockVMAccountsArguments()
			args.Config = config.VirtualMachineConfig{
				WasmVMVersions: []config.WasmVMVersionByEpoch{{StartEpoch: 0, Version: vmhost.VMVersion}},
			}
			args.EpochNotifier = epochNotifierInstance
			args.BlockChainHook = blockChainHook
			args.EnableEpochs = tc.enableEpochs
			vmf, err := NewVMContainerFactory(args)
			require.Nil(t, err)

			container, err := vmf.Create()
			require.Nil(t, err)
			defer func() {
				_ = container.Close()
			}()
			initialVM := getWasmVM(t, container)
			clearsAfterCreate := blockChainHook.numClears

			for i, epoch := range tc.confirmedEpochs {
				epochNotifierInstance.CheckEpoch(epoch)
				require.Equal(t, clearsAfterCreate+tc.expectedClears[i], blockChainHook.numClears,
					"after confirming epoch %d", epoch)
			}
			require.Same(t, initialVM, getWasmVM(t, container))
		})
	}
}

func TestResolveWasmVMHostVersion(t *testing.T) {
	testCases := []struct {
		version string
		known   bool
	}{
		{version: vmhost.VMVersion, known: true},
		{version: "*", known: true},
		{version: "v1.0", known: true},
		{version: "", known: false},
		{version: "v9.9", known: false},
	}

	for _, tc := range testCases {
		hostVersion, known := resolveWasmVMHostVersion(tc.version)
		require.Equal(t, vmhost.VMVersion, hostVersion, tc.version)
		require.Equal(t, tc.known, known, tc.version)
	}
}

func getWasmVM(t testing.TB, container process.VirtualMachinesContainer) vmcommon.VMExecutionHandler {
	vm, err := container.Get(common.WasmVirtualMachine)
	require.Nil(t, err)
	require.NotNil(t, vm)

	return vm
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
