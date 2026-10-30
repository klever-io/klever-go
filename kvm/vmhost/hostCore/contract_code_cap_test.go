package hostCore_test

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/state"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

const (
	codeCapLimit       = int(common.MaxLeafSize)
	codeCapGas         = uint64(100_000_000)
	codeCapHookGas     = int64(50_000_000)
	codeCapDeployFunc  = "deployFromSource"
	codeCapUpdateFunc  = "upgradeFromSource"
	codeCapLibraryFunc = "invalid-library-function"
)

var (
	codeCapContract     = test.MakeTestSCAddress("codeCapContract")
	codeCapSource       = test.MakeTestSCAddress("codeCapSource")
	codeCapParent       = test.MakeTestSCAddress("codeCapParent")
	codeCapInstance     = test.MakeTestSCAddress("codeCapInstance")
	codeCapMetadata     = []byte{vmcommon.MetadataUpgradeable, 0}
	codeCapNewMetadata  = []byte{vmcommon.MetadataUpgradeable, vmcommon.MetadataPayable}
	codeCapPostForkConf = blockchainConfig.EnableEpochs{FixAuditChangesV7: 0}
	codeCapPreForkConf  = blockchainConfig.EnableEpochs{FixAuditChangesV7: 1_000_000}
	errCodeCapLookup    = errors.New("account lookup failed")
)

// codeCapWorld counts stored code reads, so the tests can pin when the cap touches state.
type codeCapWorld struct {
	*worldmock.MockWorld
	codeReads         int
	accountLookups    int
	failAccountLookup int
}

func (world *codeCapWorld) GetUserAccount(address []byte) (state.UserAccountHandler, error) {
	world.accountLookups++
	if world.accountLookups == world.failAccountLookup {
		return nil, errCodeCapLookup
	}
	return world.MockWorld.GetUserAccount(address)
}

func (world *codeCapWorld) GetCode(account state.UserAccountHandler) []byte {
	world.codeReads++
	return world.MockWorld.GetCode(account)
}

// codeCapHarness maps oversized modules to mock instances, so reaching the lifecycle function
// proves the code passed the cap without real wasmer having to compile it.
type codeCapHarness struct {
	t              *testing.T
	host           vmhost.VMHost
	world          *codeCapWorld
	executor       *mock.ExecutorMock
	lifecycleCalls int
	// whether the hook left a code update staged, seen before the failure discards the output
	hookLeftCode bool
}

func newCodeCapHarness(t *testing.T, epochs blockchainConfig.EnableEpochs) *codeCapHarness {
	world := &codeCapWorld{MockWorld: worldmock.NewMockWorld()}
	factory := mock.NewExecutorMockFactory(world.MockWorld)
	host := test.NewTestHostBuilder(t).
		WithExecutorFactory(factory).
		WithBlockchainHook(world).
		WithEnableEpochs(epochs).
		Build()
	t.Cleanup(host.Reset)
	setZeroCodeCosts(host)

	user, err := world.AccountsCacher.LoadUser(test.UserAddress)
	require.NoError(t, err)
	require.NoError(t, world.AccountsCacher.SaveUser(user))

	return &codeCapHarness{t: t, host: host, world: world, executor: factory.LastCreatedExecutor}
}

func setZeroCodeCosts(host vmhost.VMHost) {
	host.Metering().GasSchedule().BaseOperationCost.CompilePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.AoTPreparePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.GetCode = 0
	host.Metering().GasSchedule().BaseOperationCost.StorePerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.DataCopyPerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.PersistPerByte = 0
	host.Metering().GasSchedule().BaseOperationCost.ReleasePerByte = 0
	host.Metering().GasSchedule().BaseOpsAPICost.SignalError = 0
	host.Metering().GasSchedule().BaseOpsAPICost.ExecuteOnSameContext = 0
	host.Metering().GasSchedule().BaseOpsAPICost.ExecuteOnDestContext = 0
	host.Metering().GasSchedule().BaseOpsAPICost.StorageLoad = 0
	host.Metering().GasSchedule().BaseOpsAPICost.StorageStore = 0
	host.Metering().GasSchedule().BaseOpsAPICost.TransferValue = 0
	host.Metering().GasSchedule().BaseOpsAPICost.CreateContract = 0
}

func (harness *codeCapHarness) registerCode(code []byte) {
	instance := harness.executor.CreateAndStoreInstanceMock(
		harness.t, harness.host, codeCapInstance, nil, nil, nil, 0, false)
	countLifecycleCall := func() *mock.InstanceMock {
		harness.lifecycleCalls++
		return instance
	}
	instance.AddMockMethod(vmhost.InitFunctionName, countLifecycleCall)
	instance.AddMockMethod(vmhost.ContractsUpgradeFunctionName, countLifecycleCall)
	harness.executor.InstanceMap[string(code)] = *instance
}

func (harness *codeCapHarness) addContract(address []byte, code []byte, owner []byte) {
	harness.registerCode(code)
	harness.executor.CreateAndStoreInstanceMock(
		harness.t, harness.host, address, nil, codeCapMetadata, owner, 0, true)

	account, err := harness.world.GetAccount(address)
	require.NoError(harness.t, err)
	account.SetCode(code)
	harness.world.PutAccount(account)
}

func (harness *codeCapHarness) addParent(function string, body func(host vmhost.VMHost)) {
	instance := harness.executor.CreateAndStoreInstanceMock(
		harness.t, harness.host, codeCapParent, nil, nil, nil, 0, true)
	instance.AddMockMethod(function, func() *mock.InstanceMock {
		body(instance.Host)
		return instance
	})
}

func (harness *codeCapHarness) verifier(vmOutput *vmcommon.VMOutput, err error) *test.VMOutputVerifier {
	return test.NewVMOutputVerifierWithAllErrors(harness.t, vmOutput, err, harness.host.Runtime().GetAllErrors())
}

func (harness *codeCapHarness) directDeploy(code []byte, gas uint64) *test.VMOutputVerifier {
	input := test.CreateTestContractCreateInputBuilder().
		WithCallerAddr(test.UserAddress).
		WithContractCode(code).
		WithGasProvided(gas).
		WithArguments().
		Build()

	vmOutput, err := harness.host.RunSmartContractCreate(input)
	return harness.verifier(vmOutput, err)
}

func (harness *codeCapHarness) directUpgrade(code []byte, gas uint64) *test.VMOutputVerifier {
	input := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(test.UserAddress).
		WithRecipientAddr(codeCapContract).
		WithFunction(vmhost.UpgradeFunctionName).
		WithArguments(code, codeCapNewMetadata).
		WithGasProvided(gas).
		Build()

	vmOutput, err := harness.host.RunSmartContractCall(input)
	return harness.verifier(vmOutput, err)
}

func (harness *codeCapHarness) callParent(function string) *test.VMOutputVerifier {
	input := test.CreateTestContractCallInputBuilder().
		WithCallerAddr(test.UserAddress).
		WithRecipientAddr(codeCapParent).
		WithFunction(function).
		WithGasProvided(codeCapGas).
		Build()

	vmOutput, err := harness.host.RunSmartContractCall(input)
	return harness.verifier(vmOutput, err)
}

// hookUpgrade drives executeUpgrade through the upgrade-from-source hook.
func (harness *codeCapHarness) hookUpgrade(code []byte, gasLimit int64) *test.VMOutputVerifier {
	harness.addContract(codeCapSource, code, test.UserAddress)
	harness.addParent(codeCapUpdateFunc, func(host vmhost.VMHost) {
		vmhooks.UpgradeFromSourceContractWithTypedArgs(
			host, codeCapSource, codeCapContract, nil, nil, gasLimit, codeCapNewMetadata)
		harness.hookLeftCode = host.Output().HasPendingCodeUpdate(codeCapContract)
	})

	return harness.callParent(codeCapUpdateFunc)
}

// hookDeploy drives CreateNewContract through the deploy-from-source hook.
func (harness *codeCapHarness) hookDeploy(code []byte) ([]byte, error, *test.VMOutputVerifier) {
	var address []byte
	var deployErr error
	harness.addContract(codeCapSource, code, test.UserAddress)
	harness.mockNewAddress(codeCapParent)
	harness.addParent(codeCapDeployFunc, func(host vmhost.VMHost) {
		address, deployErr = vmhooks.DeployFromSourceContractWithTypedArgs(
			host, codeCapSource, codeCapMetadata, big.NewInt(0), [][]byte{}, codeCapHookGas)
		harness.hookLeftCode = host.Output().HasPendingCodeUpdate(codeCapContract)
	})

	verify := harness.callParent(codeCapDeployFunc)
	return address, deployErr, verify
}

// mockNewAddress makes codeCapContract the address of the next contract the creator deploys.
func (harness *codeCapHarness) mockNewAddress(creator []byte) {
	harness.world.NewAddressMocks = append(harness.world.NewAddressMocks, &worldmock.NewAddressMock{
		CreatorAddress: creator,
		NewAddress:     codeCapContract,
	})
}

func (harness *codeCapHarness) setCompilePerByte(cost uint64) {
	harness.host.Metering().GasSchedule().BaseOperationCost.CompilePerByte = cost
}

func codeCapModule(t *testing.T, size int, fill byte) []byte {
	const wrapperLength = 17 // preamble, section id, 3-byte LEB128 length and section name
	code := mock.MockContractCode(bytes.Repeat([]byte{fill}, size-wrapperLength))
	require.Len(t, code, size)
	return code
}

func requireCodeInOutput(t *testing.T, verify *test.VMOutputVerifier, address []byte, code []byte) {
	account := verify.VmOutput.OutputAccounts[string(address)]
	require.NotNil(t, account)
	require.True(t, bytes.Equal(code, account.Code), "output code is not the deployed code")
}

// requireTopLevelRejection: every top-level rejection reports VMContractInvalid, burns the gas and
// stages no code. The error output is always empty, so the staged code is read from the host.
func requireTopLevelRejection(t *testing.T, harness *codeCapHarness, verify *test.VMOutputVerifier) {
	verify.ContractInvalid().
		ReturnMessage(vmhost.ErrContractCodeTooLarge.Error()).
		GasRemaining(0)
	require.Empty(t, verify.VmOutput.OutputAccounts)
	require.False(t, harness.host.Output().HasPendingCodeUpdate(codeCapContract))
}

func TestCodeCap_DirectDeploy(t *testing.T) {
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')
	atLimit := codeCapModule(t, codeCapLimit, 'b')

	t.Run("over limit is rejected before init", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.registerCode(overLimit)
		harness.mockNewAddress(test.UserAddress)

		requireTopLevelRejection(t, harness, harness.directDeploy(overLimit, codeCapGas))
		require.Zero(t, harness.lifecycleCalls)
	})

	t.Run("at limit is accepted", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.registerCode(atLimit)

		harness.directDeploy(atLimit, codeCapGas).Ok()
		require.Equal(t, 1, harness.lifecycleCalls)
	})

	t.Run("over limit is accepted before the fork", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPreForkConf)
		harness.registerCode(overLimit)

		harness.directDeploy(overLimit, codeCapGas).Ok()
		require.Equal(t, 1, harness.lifecycleCalls)
	})

	t.Run("gas is checked first", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.setCompilePerByte(1)

		harness.directDeploy(overLimit, uint64(codeCapLimit)).
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
	})

	// the same-code exemption is for upgrades only, even when the new address already holds the code
	t.Run("identical code at the target address is not exempt", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.addContract(codeCapContract, overLimit, test.UserAddress)
		harness.mockNewAddress(test.UserAddress)

		requireTopLevelRejection(t, harness, harness.directDeploy(overLimit, codeCapGas))
		require.Zero(t, harness.lifecycleCalls)
	})
}

func TestCodeCap_IndirectDeploy(t *testing.T) {
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')
	atLimit := codeCapModule(t, codeCapLimit, 'b')

	t.Run("over limit is rejected before init", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)

		address, err, verify := harness.hookDeploy(overLimit)

		require.ErrorIs(t, err, vmhost.ErrContractCodeTooLarge)
		require.Nil(t, address)
		require.Zero(t, harness.lifecycleCalls)
		require.NotContains(t, verify.VmOutput.OutputAccounts, string(codeCapContract))
		require.False(t, harness.hookLeftCode)
	})

	t.Run("at limit is accepted", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)

		address, err, verify := harness.hookDeploy(atLimit)

		require.NoError(t, err)
		require.Equal(t, 1, harness.lifecycleCalls)
		requireCodeInOutput(t, verify, address, atLimit)
	})

	t.Run("over limit is accepted before the fork", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPreForkConf)

		address, err, verify := harness.hookDeploy(overLimit)

		require.NoError(t, err)
		require.Equal(t, 1, harness.lifecycleCalls)
		requireCodeInOutput(t, verify, address, overLimit)
	})
}

func TestCodeCap_Upgrade(t *testing.T) {
	stored := mock.MockContractCode([]byte("stored"))
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')
	otherOverLimit := codeCapModule(t, codeCapLimit+1, 'c')
	atLimit := codeCapModule(t, codeCapLimit, 'b')

	paths := []struct {
		name         string
		owner        []byte
		upgrade      func(harness *codeCapHarness, code []byte) *test.VMOutputVerifier
		wantRejected func(t *testing.T, harness *codeCapHarness, verify *test.VMOutputVerifier)
	}{
		{
			name:  "direct",
			owner: test.UserAddress,
			upgrade: func(harness *codeCapHarness, code []byte) *test.VMOutputVerifier {
				return harness.directUpgrade(code, codeCapGas)
			},
			wantRejected: requireTopLevelRejection,
		},
		{
			name:  "indirect",
			owner: codeCapParent,
			upgrade: func(harness *codeCapHarness, code []byte) *test.VMOutputVerifier {
				return harness.hookUpgrade(code, codeCapHookGas)
			},
			wantRejected: func(t *testing.T, harness *codeCapHarness, verify *test.VMOutputVerifier) {
				verify.ExecutionFailed().HasRuntimeErrors(vmhost.ErrContractCodeTooLarge.Error())
				require.Empty(t, verify.VmOutput.OutputAccounts)
				require.False(t, harness.hookLeftCode)
			},
		},
	}

	cases := []struct {
		name     string
		epochs   blockchainConfig.EnableEpochs
		stored   []byte
		upgrade  []byte
		rejected bool
	}{
		{name: "over limit", epochs: codeCapPostForkConf, stored: stored, upgrade: overLimit, rejected: true},
		{name: "same length as oversized stored code", epochs: codeCapPostForkConf, stored: overLimit, upgrade: otherOverLimit, rejected: true},
		{name: "at limit", epochs: codeCapPostForkConf, stored: stored, upgrade: atLimit},
		{name: "same code over limit", epochs: codeCapPostForkConf, stored: overLimit, upgrade: overLimit},
		{name: "over limit before the fork", epochs: codeCapPreForkConf, stored: stored, upgrade: overLimit},
	}

	for _, path := range paths {
		for _, testCase := range cases {
			t.Run(path.name+"/"+testCase.name, func(t *testing.T) {
				harness := newCodeCapHarness(t, testCase.epochs)
				harness.addContract(codeCapContract, testCase.stored, path.owner)
				harness.registerCode(testCase.upgrade)

				verify := path.upgrade(harness, testCase.upgrade)

				if testCase.rejected {
					path.wantRejected(t, harness, verify)
					require.Zero(t, harness.lifecycleCalls)
					return
				}

				verify.Ok()
				require.Equal(t, 1, harness.lifecycleCalls)
				requireCodeInOutput(t, verify, codeCapContract, testCase.upgrade)
				require.Equal(t, codeCapNewMetadata, verify.VmOutput.OutputAccounts[string(codeCapContract)].CodeMetadata)
			})
		}
	}
}

func TestCodeCap_UpgradeChecksGasFirst(t *testing.T) {
	stored := mock.MockContractCode([]byte("stored"))
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')

	t.Run("direct", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.addContract(codeCapContract, stored, test.UserAddress)
		harness.setCompilePerByte(1)

		harness.directUpgrade(overLimit, uint64(codeCapLimit)).
			OutOfGas().
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
		require.Zero(t, harness.world.codeReads)
	})

	t.Run("indirect", func(t *testing.T) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.addContract(codeCapContract, stored, codeCapParent)
		harness.setCompilePerByte(1)

		harness.hookUpgrade(overLimit, int64(codeCapLimit)).
			HasRuntimeErrors(vmhost.ErrNotEnoughGas.Error())
		// the parent and source contracts only: the target's code is not read before the charge
		require.Equal(t, 2, harness.world.codeReads)
	})
}

// A failed read of the stored code surfaces as itself, not as code too large.
func TestCodeCap_UpgradeReportsStoredCodeLookupFailure(t *testing.T) {
	stored := mock.MockContractCode([]byte("stored"))
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')

	upgrade := func(failAccountLookup int) (*codeCapHarness, *test.VMOutputVerifier) {
		harness := newCodeCapHarness(t, codeCapPostForkConf)
		harness.addContract(codeCapContract, stored, test.UserAddress)
		harness.registerCode(overLimit)
		harness.world.failAccountLookup = failAccountLookup
		return harness, harness.directUpgrade(overLimit, codeCapGas)
	}

	// the size check is the last lookup of a rejected upgrade
	harness, verify := upgrade(0)
	requireTopLevelRejection(t, harness, verify)

	_, verify = upgrade(harness.world.accountLookups)
	verify.HasRuntimeErrors(errCodeCapLookup.Error())
	require.NotEqual(t, vmhost.ErrContractCodeTooLarge.Error(), verify.VmOutput.ReturnMessage)
}

// An account without stored code has nothing to keep, so the size check still rejects.
func TestCodeCap_UpgradeOfAccountWithoutCode(t *testing.T) {
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')

	harness := newCodeCapHarness(t, codeCapPostForkConf)
	harness.addContract(codeCapContract, nil, test.UserAddress)
	harness.registerCode(overLimit)

	requireTopLevelRejection(t, harness, harness.directUpgrade(overLimit, codeCapGas))
	require.Zero(t, harness.lifecycleCalls)
}

// The stored code is read only for a post-fork upgrade over the limit.
func TestCodeCap_UpgradeReadsStoredCodeOnlyOverLimit(t *testing.T) {
	stored := mock.MockContractCode([]byte("stored"))
	overLimit := codeCapModule(t, codeCapLimit+1, 'a')
	atLimit := codeCapModule(t, codeCapLimit, 'b')

	cases := []struct {
		name      string
		epochs    blockchainConfig.EnableEpochs
		upgrade   []byte
		codeReads int
	}{
		{name: "over limit before the fork", epochs: codeCapPreForkConf, upgrade: overLimit, codeReads: 0},
		{name: "at limit", epochs: codeCapPostForkConf, upgrade: atLimit, codeReads: 0},
		{name: "over limit", epochs: codeCapPostForkConf, upgrade: overLimit, codeReads: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			harness := newCodeCapHarness(t, testCase.epochs)
			harness.addContract(codeCapContract, stored, test.UserAddress)
			harness.registerCode(testCase.upgrade)

			harness.directUpgrade(testCase.upgrade, codeCapGas)

			require.Equal(t, testCase.codeReads, harness.world.codeReads)
		})
	}
}

// the parent upgrades the contract twice in one tx, from two sources
func runTwoCodeCapUpgrades(t *testing.T, stored, first, second []byte) (*codeCapHarness, *test.VMOutputVerifier) {
	harness := newCodeCapHarness(t, codeCapPostForkConf)
	harness.addContract(codeCapContract, stored, codeCapParent)
	src1 := test.MakeTestSCAddress("codeCapSource1")
	src2 := test.MakeTestSCAddress("codeCapSource2")
	harness.addContract(src1, first, test.UserAddress)
	harness.addContract(src2, second, test.UserAddress)
	harness.addParent(codeCapUpdateFunc, func(host vmhost.VMHost) {
		vmhooks.UpgradeFromSourceContractWithTypedArgs(host, src1, codeCapContract, nil, nil, codeCapHookGas/4, codeCapNewMetadata)
		if host.Runtime().GetRuntimeBreakpointValue() != vmhost.BreakpointNone {
			return
		}
		vmhooks.UpgradeFromSourceContractWithTypedArgs(host, src2, codeCapContract, nil, nil, codeCapHookGas/4, codeCapMetadata)
	})
	return harness, harness.callParent(codeCapUpdateFunc)
}

func TestCodeCap_SeveralUpgradesInOneTx(t *testing.T) {
	over := codeCapModule(t, codeCapLimit+1, 'a')
	other := codeCapModule(t, codeCapLimit+1, 'c')
	small := mock.MockContractCode([]byte("small"))

	t.Run("oversized stored, same code twice is accepted", func(t *testing.T) {
		h, v := runTwoCodeCapUpgrades(t, over, over, over)
		v.Ok()
		require.Equal(t, 2, h.lifecycleCalls)
		requireCodeInOutput(t, v, codeCapContract, over)
	})
	t.Run("oversized stored, down to small then back is rejected by staged read", func(t *testing.T) {
		h, v := runTwoCodeCapUpgrades(t, over, small, over)
		v.ExecutionFailed().HasRuntimeErrors(vmhost.ErrContractCodeTooLarge.Error())
		require.Equal(t, 1, h.lifecycleCalls)
	})
	t.Run("oversized stored, same then other oversized is rejected", func(t *testing.T) {
		h, v := runTwoCodeCapUpgrades(t, over, over, other)
		v.ExecutionFailed().HasRuntimeErrors(vmhost.ErrContractCodeTooLarge.Error())
		require.Equal(t, 1, h.lifecycleCalls)
	})
	t.Run("small stored, small then oversized is rejected", func(t *testing.T) {
		h, v := runTwoCodeCapUpgrades(t, small, small, over)
		v.ExecutionFailed().HasRuntimeErrors(vmhost.ErrContractCodeTooLarge.Error())
		require.Equal(t, 1, h.lifecycleCalls)
	})
}

// a rejected nested upgrade must not leave the verification armed for the next instantiation
func TestCodeCap_RejectedUpgradeDoesNotArmTheNextInstantiation(t *testing.T) {
	over := codeCapModule(t, codeCapLimit+1, 'a')
	stored := mock.MockContractCode([]byte("stored"))
	lib := test.MakeTestSCAddress("codeCapLibrary")

	harness := newCodeCapHarness(t, codeCapPostForkConf)
	harness.addContract(codeCapContract, stored, codeCapParent)
	libCalls := 0
	libInstance := harness.executor.CreateAndStoreInstanceMock(t, harness.host, lib, nil, nil, nil, 0, true)
	libInstance.AddMockMethod("libraryMethod", func() *mock.InstanceMock {
		libCalls++
		return libInstance
	})
	libInstance.AddMockMethod(codeCapLibraryFunc, func() *mock.InstanceMock { return libInstance })

	var upgradeErr error
	harness.addParent("go", func(host vmhost.VMHost) {
		input := test.CreateTestContractCallInputBuilder().
			WithCallerAddr(codeCapParent).
			WithRecipientAddr(codeCapContract).
			WithFunction(vmhost.UpgradeFunctionName).
			WithArguments(over, codeCapNewMetadata).
			WithGasProvided(1000).
			Build()
		_, upgradeErr = host.ExecuteOnDestContext(input)
		vmhooks.ExecuteOnDestContextWithTypedArgs(host, 100000, big.NewInt(0), []byte("libraryMethod"), lib, [][]byte{})
	})
	harness.callParent("go").Ok()
	require.ErrorIs(t, upgradeErr, vmhost.ErrContractCodeTooLarge)
	require.Equal(t, 1, libCalls)
}
