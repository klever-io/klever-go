package hostCore_test

import (
	"bytes"
	"math/big"
	"testing"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

// TestDirectDeploy_OnlyTheDeployedContractKeepsCode pins what scProcessor.printScDeployed relies on from
// FixAuditChangesV7 to pick the contracts a deploy records an SCTrigger receipt for: in a deploy's output
// only the new contract carries Code. A contract the constructor calls is in the output too, with its
// storage write, but with no Code (KLC-2770).
func TestDirectDeploy_OnlyTheDeployedContractKeepsCode(t *testing.T) {
	deployedCode := mock.MockContractCode(bytes.Repeat([]byte{'d'}, 32))
	touchedCode := mock.MockContractCode(bytes.Repeat([]byte{'t'}, 32))
	touched := test.MakeTestSCAddress("touchedByInit")

	harness := newCodeCapHarness(t, codeCapPostForkConf)
	harness.mockNewAddress(test.UserAddress)

	touchedInstance := harness.executor.CreateAndStoreInstanceMock(
		t, harness.host, touched, nil, codeCapMetadata, test.UserAddress, 0, true)
	touchedInstance.AddMockMethod("touch", func() *mock.InstanceMock {
		_, err := touchedInstance.Host.Storage().SetStorage([]byte("touched"), []byte("by init"))
		require.NoError(t, err)
		return touchedInstance
	})
	harness.executor.InstanceMap[string(touchedCode)] = *touchedInstance
	account, err := harness.world.GetAccount(touched)
	require.NoError(t, err)
	account.SetCode(touchedCode)
	harness.world.PutAccount(account)

	deployedInstance := harness.executor.CreateAndStoreInstanceMock(
		t, harness.host, codeCapInstance, nil, nil, nil, 0, false)
	deployedInstance.AddMockMethod(vmhost.InitFunctionName, func() *mock.InstanceMock {
		vmhooks.ExecuteOnDestContextWithTypedArgs(
			deployedInstance.Host, codeCapHookGas, big.NewInt(0), []byte("touch"), touched, [][]byte{})
		return deployedInstance
	})
	harness.executor.InstanceMap[string(deployedCode)] = *deployedInstance

	verify := harness.directDeploy(deployedCode, codeCapGas)
	verify.Ok()

	requireCodeInOutput(t, verify, codeCapContract, deployedCode)

	touchedOutput := verify.VmOutput.OutputAccounts[string(touched)]
	require.NotNil(t, touchedOutput, "the contract that init calls is in the output")
	require.Contains(t, touchedOutput.StorageUpdates, "touched")
	require.Empty(t, touchedOutput.Code, "a contract the deploy only calls carries no code")
}
