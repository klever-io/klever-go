package hostCoretest

import (
	"math/big"
	"testing"

	blockchainConfig "github.com/klever-io/klever-go/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

func TestForbiddenOps_BulkAndSIMD(t *testing.T) {
	wasmModules := []string{"data-drop", "memory-init", "memory-fill", "memory-copy", "simd"}

	for _, moduleName := range wasmModules {
		testCase := testcommon.BuildInstanceCallTest(t).
			WithContracts(
				testcommon.CreateInstanceContract(testcommon.ParentAddress).
					WithCode(testcommon.GetTestSCCodeModule("forbidden-opcodes/"+moduleName, moduleName, "../../"))).
			WithInput(testcommon.CreateTestContractCallInputBuilder().
				WithGasProvided(100000).
				WithFunction("main").
				Build())

		assertResults := func(_ vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *testcommon.VMOutputVerifier) {
			verify.ContractInvalid()
		}

		testCase.AndAssertResults(assertResults)
	}
}

// TestForbiddenOps_Tables pins that no table instruction compiles: the
// executor prices none of them, so each fails as an unsupported operator.
// Every fixture declares a 2..10 table, well under MaxDeclaredTableSize, so
// the rejection can only come from the opcode itself. The rejection is not
// fork-gated, so it holds on either side of FixAuditChangesV5, which selects
// the table.init pricing.
func TestForbiddenOps_Tables(t *testing.T) {
	wasmModules := []string{
		"table-get", "table-set", "table-grow", "table-size",
		"table-init", "table-fill", "table-copy", "elem-drop",
	}
	schedules := map[string]blockchainConfig.EnableEpochs{
		"all forks active":         {},
		"before FixAuditChangesV5": {FixAuditChangesV5: 10},
	}

	for scheduleName, enableEpochs := range schedules {
		for _, moduleName := range wasmModules {
			t.Run(scheduleName+"/"+moduleName, func(t *testing.T) {
				testcommon.BuildInstanceCallTest(t).
					WithEnableEpochs(enableEpochs).
					WithContracts(
						testcommon.CreateInstanceContract(testcommon.ParentAddress).
							WithCode(testcommon.GetTestSCCodeModule("forbidden-opcodes/"+moduleName, moduleName, "../../"))).
					WithInput(testcommon.CreateTestContractCallInputBuilder().
						WithGasProvided(100000).
						WithFunction("main").
						Build()).
					AndAssertResults(func(_ vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *testcommon.VMOutputVerifier) {
						verify.ContractInvalid()
					})
			})
		}
	}
}

// TestForbiddenOps_TablesCallIndirectStillAllowed is the control for
// TestForbiddenOps_Tables: the same module shape with only call_indirect
// through an active element segment still runs.
func TestForbiddenOps_TablesCallIndirectStillAllowed(t *testing.T) {
	testcommon.BuildInstanceCallTest(t).
		WithContracts(
			testcommon.CreateInstanceContract(testcommon.ParentAddress).
				WithCode(testcommon.GetTestSCCode("table-call-indirect", "../../"))).
		WithInput(testcommon.CreateTestContractCallInputBuilder().
			WithGasProvided(100000).
			WithFunction("main").
			Build()).
		AndAssertResults(func(_ vmhost.VMHost, _ *contextmock.BlockchainHookStub, verify *testcommon.VMOutputVerifier) {
			verify.Ok()
		})
}

func TestForbiddenOps_FloatingPoints(t *testing.T) {
	testcommon.BuildInstanceCreatorTest(t).
		WithInput(testcommon.CreateTestContractCreateInputBuilder().
			WithGasProvided(1000).
			WithKDATransfers([]*vmcommon.KDATransfer{{KDAValue: big.NewInt(88)}}).
			WithArguments([]byte{2}).
			WithContractCode(testcommon.GetTestSCCode("num-with-fp", "../../")).
			Build()).
		WithAddress(newAddress).
		AndAssertResults(func(_ *contextmock.BlockchainHookStub, verify *testcommon.VMOutputVerifier) {
			verify.ContractInvalid()
		})
}
