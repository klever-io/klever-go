package contracts

import (
	"bytes"
	"fmt"
	"math/big"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
)

// BackTransfer_ParentCallsChild is an exposed mock contract method
func BackTransfer_ParentCallsChild(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("callChild", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		storedResult := []byte("ok")

		testConfig := config.(*test.TestConfig)
		input := test.DefaultTestContractCallInput()
		input.GasProvided = testConfig.GasProvidedToChild
		input.CallerAddr = nil
		input.RecipientAddr = testConfig.ChildAddress
		input.Function = "childFunction"
		returnValue := ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
		if returnValue != 0 {
			host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
		}
		managedTypes := host.ManagedTypes()

		arguments := host.Runtime().Arguments()
		if len(arguments) > 0 {
			checkBackTransfers := arguments[0]
			if checkBackTransfers[0] == 1 {
				kdaTransfers, klv := managedTypes.GetBackTransfers()
				if len(kdaTransfers) != 1 {
					host.Runtime().FailExecution(fmt.Errorf("found kda transfers %d", len(kdaTransfers)))
					storedResult = []byte("err")
				}
				if !bytes.Equal(test.KDATestTokenName, kdaTransfers[0].KDATokenName) {
					host.Runtime().FailExecution(fmt.Errorf("invalid token name %s", string(kdaTransfers[0].KDATokenName)))
					storedResult = []byte("err")
				}
				if big.NewInt(0).SetInt64(testConfig.KDATokensToTransfer).Cmp(kdaTransfers[0].KDAValue) != 0 {
					host.Runtime().FailExecution(fmt.Errorf("invalid token value %d", kdaTransfers[0].KDAValue.Uint64()))
					storedResult = []byte("err")
				}
				if klv.Cmp(big.NewInt(testConfig.TransferFromChildToParent)) != 0 {
					host.Runtime().FailExecution(fmt.Errorf("invalid klv value %d", klv))
					storedResult = []byte("err")
				}
			}
		}

		_, err := host.Storage().SetStorage(test.ParentKeyA, storedResult)
		if err != nil {
			host.Runtime().FailExecution(err)
		}

		return instance
	})
}
