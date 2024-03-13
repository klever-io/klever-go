package contracts

import (
	"fmt"
	"math/big"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
)

// ExecKDATransferAndCallChild is an exposed mock contract method
func ExecKDATransferAndCallChild(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("execKDATransferAndCall", func() *mock.InstanceMock {
		testConfig := config.(*test.TestConfig)
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		err := host.Metering().UseGasBounded(testConfig.GasUsedByParent)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
			return instance
		}

		arguments := host.Runtime().Arguments()
		if len(arguments) < 3 {
			host.Runtime().SignalUserError("need 3 arguments")
			return instance
		}

		input := test.DefaultTestContractCallInput()
		input.CallerAddr = host.Runtime().GetContextAddress()
		input.GasProvided = testConfig.GasProvidedToChild
		input.Arguments = [][]byte{
			arguments[0],
			{0x00, 0x00, 0x00, 0x01},
			test.KDATestTokenName,
			{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			big.NewInt(int64(testConfig.KDATokensToTransfer)).Bytes(),
		}
		input.Arguments = append(input.Arguments, arguments[2:]...)
		input.RecipientAddr = arguments[0]
		input.Function = string(arguments[1])

		returnValue := ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
		if returnValue != 0 {
			host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
		}

		return instance
	})
}
