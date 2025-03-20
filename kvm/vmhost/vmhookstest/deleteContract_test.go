package vmhookstest

import (
	"crypto/rand"

	"math/big"
	"testing"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	common "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

var (
	newAddress     = common.MakeTestSCAddressWithDefaultVM("baseOpsTest")
	accountAddress = make([]byte, 32)
	_, _           = rand.Read(accountAddress)
	gasLimit       = uint64(100000)
	gasRemaining   = uint64(82776)
	gasConsumedDiv = uint64(2)
)

func getGasMap() config.GasScheduleMap {
	gasMap := config.MakeGasMapForTests()
	gasMap["BaseOpsAPICost"]["CreateContract"] = DeleteCost
	return gasMap
}

const contractName = "delete-contract"

type DeleteTest struct {
	Name          string
	ParentAddress []byte
	Code          []byte
	CodeHash      []byte
	CodeMeta      *vmcommon.CodeMetadata
	Function      string
	GasLimit      uint64
	Assert        func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *common.VMOutputVerifier)
}

var contractSize = uint64(2604)
var DeleteCost = uint64(10000)
var GetOwnerReturn = uint64(117)
var GetOwner = uint64(51)
var DeleteExec = uint64(13)

// contract is loaded 1 initial call + 2 ExecDestCall
var contractLoadCost = 3 * contractSize

// (Initial Call + 2 ExecDestCall) * contractSize + 2*GetOwner+ DeleteCost
var baseCost = contractLoadCost + 2*GetOwnerReturn + 2*GetOwner + DeleteExec + DeleteCost

// kvm/test/contracts/delete-contract/output/delete-contract.wasm
func TestBaseOps_DeleteContractValidateGas(t *testing.T) {
	baseOpsCode := common.GetTestSCCodeModule(contractName, contractName, "../../")
	codeHash := worldmock.DefaultHasher.Compute(string(baseOpsCode))
	codeMeta := &vmcommon.CodeMetadata{
		Payable:     true,
		PayableBySC: true,
		Upgradeable: true,
		Readable:    true,
	}

	tests := []DeleteTest{
		{
			Name:          "DeleteContractFullfGas",
			ParentAddress: common.ParentAddress,
			Code:          baseOpsCode,
			CodeHash:      codeHash,
			CodeMeta:      codeMeta,
			Function:      "delete_contract_full_gas",
			GasLimit:      gasLimit,
			Assert: func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *common.VMOutputVerifier) {
				verify.Ok()
				verify.GasRemaining(gasLimit - baseCost)
			},
		},
		{
			Name:          "DeleteContractHalfGas",
			ParentAddress: common.ParentAddress,
			Code:          baseOpsCode,
			CodeHash:      codeHash,
			CodeMeta:      codeMeta,
			Function:      "delete_contract_half_gas",
			GasLimit:      gasLimit,
			Assert: func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *common.VMOutputVerifier) {
				verify.Ok()
				verify.GasRemaining(gasLimit - baseCost - gasConsumedDiv)
			},
		},
		{
			Name:          "DeleteContractLessGas",
			ParentAddress: common.ParentAddress,
			Code:          baseOpsCode,
			CodeHash:      codeHash,
			CodeMeta:      codeMeta,
			Function:      "delete_contract_less_gas",
			GasLimit:      gasLimit,
			Assert: func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub, verify *common.VMOutputVerifier) {
				// should fail as contract calls Delete with reduced gasLimt necessary to cover delete costs
				verify.OutOfGas()
				verify.GasRemaining(0)
			},
		},
	}

	for _, test := range tests {
		runDeleteContractTest(t, test)
	}
}

func runDeleteContractTest(
	t *testing.T,
	test DeleteTest,
) {
	contractsMap := make(map[string]*worldmock.Account)
	contractsMap[string(test.ParentAddress)] = &worldmock.Account{
		Address:      test.ParentAddress,
		Balance:      big.NewInt(0),
		CodeHash:     test.CodeHash,
		Code:         test.Code,
		CodeMetadata: test.CodeMeta.ToBytes(),
		OwnerAddress: accountAddress,
		Storage:      make(map[string][]byte),
	}

	contractsMap[string(newAddress)] = &worldmock.Account{
		Address:      newAddress,
		Balance:      big.NewInt(0),
		CodeHash:     test.CodeHash,
		Code:         test.Code,
		CodeMetadata: test.CodeMeta.ToBytes(),
		OwnerAddress: test.ParentAddress,
		Storage:      make(map[string][]byte),
	}

	common.BuildInstanceCallTest(t).
		WithSetup(func(host vmhost.VMHost, stubBlockchainHook *contextmock.BlockchainHookStub) {
			stubBlockchainHook.GetUserAccountCalled = func(address []byte) (state.UserAccountHandler, error) {
				account, ok := contractsMap[string(address)]
				if ok {
					return account, nil
				} else {
					return nil, vmhost.ErrAccountNotFound
				}
			}
		}).
		WithContracts(
			common.CreateInstanceContract(test.ParentAddress).
				WithCode(test.Code),
			common.CreateInstanceContract(newAddress).
				WithCode(test.Code)).
		WithGasSchedule(getGasMap()).
		WithInput(common.CreateTestContractCallInputBuilder().
			WithGasProvided(test.GasLimit).
			WithFunction(test.Function).
			WithArguments(newAddress).
			Build()).
		AndAssertResults(func(v vmhost.VMHost, bhs *contextmock.BlockchainHookStub, vv *common.VMOutputVerifier) {
			t.Logf("Result for %s", test.Name)
			t.Logf("Return Data: %v", vv.VmOutput.ReturnData)
			t.Logf("Return Message: %s", vv.VmOutput.ReturnMessage)
			t.Logf("Return Code: %d", vv.VmOutput.ReturnCode)
			t.Logf("Gas Remaining: %d", vv.VmOutput.GasRemaining)

			test.Assert(v, bhs, vv)
		})
}
