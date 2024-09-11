package contracts

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	test "github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
)

// WasteGasChildMock is an exposed mock contract method
func WasteGasChildMock(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	instanceMock.AddMockMethod("wasteGas", test.SimpleWasteGasMockMethod(instanceMock, testConfig.GasUsedByChild))
}

// ReportOriginalCaller is an exposed mock contract method
func ReportOriginalCaller(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	instanceMock.AddMockMethod("reportOriginalCaller", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		err := host.Metering().UseGasBounded(testConfig.GasUsedByChild)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
		}

		originalCaller := host.Runtime().GetOriginalCallerAddress()
		host.Output().Finish(originalCaller)
		return instance
	})
}

// FailChildMock is an exposed mock contract method
func FailChildMock(instanceMock *mock.InstanceMock, _ interface{}) {
	instanceMock.AddMockMethod("fail", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		host.Runtime().FailExecution(errors.New("forced fail"))
		return instance
	})
}

// ExecOnSameCtxParentMock is an exposed mock contract method
func ExecOnSameCtxParentMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("execOnSameCtx", func() *mock.InstanceMock {
		testConfig := config.(*test.TestConfig)
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		err := host.Metering().UseGasBounded(testConfig.GasUsedByParent)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
			return instance
		}

		argsPerCall := 3
		arguments := host.Runtime().Arguments()
		if len(arguments)%argsPerCall != 0 {
			host.Runtime().SignalUserError("need 3 arguments per individual call")
			return instance
		}

		input := test.DefaultTestContractCallInput()
		input.GasProvided = testConfig.GasProvidedToChild
		input.CallerAddr = instance.Address

		for callIndex := 0; callIndex < len(arguments); callIndex += argsPerCall {
			input.RecipientAddr = arguments[callIndex+0]
			input.Function = string(arguments[callIndex+1])
			numCalls := big.NewInt(0).SetBytes(arguments[callIndex+2]).Uint64()

			for i := uint64(0); i < numCalls; i++ {
				returnValue := ExecuteOnSameContextInMockContracts(host, input, big.NewInt(0))
				if returnValue != 0 {
					host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
				}
			}
		}

		return instance
	})
}

// ExecOnDestCtxParentMock is an exposed mock contract method
func ExecOnDestCtxParentMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("execOnDestCtx", func() *mock.InstanceMock {
		testConfig := config.(*test.TestConfig)
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		err := host.Metering().UseGasBounded(testConfig.GasUsedByParent)
		if err != nil {
			host.Runtime().SetRuntimeBreakpointValue(vmhost.BreakpointOutOfGas)
			return instance
		}

		argsPerCall := 3
		arguments := host.Runtime().Arguments()
		if len(arguments)%argsPerCall != 0 {
			host.Runtime().SignalUserError("need 3 arguments per individual call")
			return instance
		}

		input := test.DefaultTestContractCallInput()
		input.GasProvided = testConfig.GasProvidedToChild
		input.CallerAddr = instance.Address

		for callIndex := 0; callIndex < len(arguments); callIndex += argsPerCall {
			input.RecipientAddr = arguments[callIndex+0]
			input.Function = string(arguments[callIndex+1])
			numCalls := big.NewInt(0).SetBytes(arguments[callIndex+2]).Uint64()

			for i := uint64(0); i < numCalls; i++ {
				returnValue := ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
				if returnValue != 0 {
					host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
				}
			}
		}

		return instance
	})
}

// ExecOnDestCtxSingleCallParentMock is an exposed mock contract method
func ExecOnDestCtxSingleCallParentMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("execOnDestCtxSingleCall", func() *mock.InstanceMock {
		testConfig := config.(*test.TestConfig)
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		host.Metering().UseGas(testConfig.GasUsedByParent)

		arguments := host.Runtime().Arguments()
		if len(arguments) < 2 {
			host.Runtime().SignalUserError("needs at least 2 arguments")
			return instance
		}

		input := test.DefaultTestContractCallInput()
		input.GasProvided = testConfig.GasProvidedToChild
		input.CallerAddr = instance.Address

		input.RecipientAddr = arguments[0]
		input.Function = string(arguments[1])

		if len(arguments) > 2 {
			input.Arguments = arguments[2:]
		}

		returnValue := ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
		if returnValue != 0 {
			host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
		}

		originalCaller := host.Runtime().GetOriginalCallerAddress()
		host.Output().Finish(originalCaller)

		return instance
	})
}

// LocalCallAnotherContract is an exposed mock contract method
func LocalCallAnotherContract(exposedFunctionName string, recipientAddress []byte, functionToCall string) func(instanceMock *mock.InstanceMock, config interface{}) {
	return func(instanceMock *mock.InstanceMock, config interface{}) {
		instanceMock.AddMockMethod(exposedFunctionName, func() *mock.InstanceMock {
			testConfig := config.(*test.TestConfig)
			host := instanceMock.Host
			instance := mock.GetMockInstance(host)

			input := test.DefaultTestContractCallInput()
			input.GasProvided = testConfig.GasProvidedToChild
			input.CallerAddr = instance.Address

			input.RecipientAddr = recipientAddress
			input.Function = functionToCall

			returnValue := ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
			if returnValue != 0 {
				host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
			}

			originalCaller := host.Runtime().GetOriginalCallerAddress()
			host.Output().Finish(originalCaller)

			return instance
		})
	}
}

// WasteGasParentMock is an exposed mock contract method
func WasteGasParentMock(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	instanceMock.AddMockMethod("wasteGas", test.SimpleWasteGasMockMethod(instanceMock, testConfig.GasUsedByParent))
}

// InitFunctionMock is the exposed init function
func InitFunctionMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod(vmhost.InitFunctionName, func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		host.Output().Finish([]byte(vmhost.InitFunctionName))
		return instance
	})
}

// InitFunctionMock is the exposed upgrade function
func UpgradeFunctionMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod(vmhost.ContractsUpgradeFunctionName, func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		host.Output().Finish([]byte(vmhost.UpgradeFunctionName))
		return instance
	})
}

const (
	kdaOnCallbackSuccess int = iota
	kdaOnCallbackWrongNumOfArgs
	kdaOnCallbackFail
	kdaOnCallbackNewAsync
	kdaOnCallbackNoReturnData
)

// KDATransferToParentMock is an exposed mock contract method
func KDATransferToParentMock(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	kdaTransferToParentMock(instanceMock, testConfig, kdaOnCallbackSuccess)
}

func KDATransferToParentMockNoReturnData(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	kdaTransferToParentMock(instanceMock, testConfig, kdaOnCallbackNoReturnData)
}

// KDATransferToParentWrongKDAArgsNumberMock is an exposed mock contract method
func KDATransferToParentWrongKDAArgsNumberMock(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	kdaTransferToParentMock(instanceMock, testConfig, kdaOnCallbackWrongNumOfArgs)
}

// KDATransferToParentCallbackWillFail is an exposed mock contract method
func KDATransferToParentCallbackWillFail(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	kdaTransferToParentMock(instanceMock, testConfig, kdaOnCallbackFail)
}

// KDATransferToParentAndNewAsyncFromCallbackMock is an exposed mock contract method
func KDATransferToParentAndNewAsyncFromCallbackMock(instanceMock *mock.InstanceMock, config interface{}) {
	kdaTransferToParentMock(instanceMock, config, kdaOnCallbackNewAsync)
}

func kdaTransferToParentMock(instanceMock *mock.InstanceMock, config interface{}, behavior int) {
	instanceMock.AddMockMethod("transferKDAToParent", func() *mock.InstanceMock {
		testConfig := config.(*test.TestConfig)
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		host.Metering().UseGas(testConfig.GasUsedByChild)

		switch behavior {
		case kdaOnCallbackSuccess:
			host.Output().Finish([]byte("success"))
		case kdaOnCallbackFail:
			host.Output().Finish([]byte("fail"))
		case kdaOnCallbackNewAsync:
			host.Output().Finish([]byte("new_async"))
			host.Output().Finish(host.Runtime().GetContextAddress())
			host.Output().Finish([]byte("wasteGas"))
		}

		arguments := host.Runtime().Arguments()
		numberOfBackTransfers := big.NewInt(0).SetBytes(arguments[0]).Uint64()

		var err error
		for numCallbacks := uint64(0); numCallbacks < numberOfBackTransfers; numCallbacks++ {
			transfer := &vmcommon.KDATransfer{
				KDAValue:      big.NewInt(0).SetUint64(testConfig.CallbackKDATokensToTransfer),
				KDATokenName:  test.KDATestTokenName,
				KDATokenType:  0,
				KDATokenNonce: 0,
			}

			ret := vmhooks.TransferKDANFTExecuteWithTypedArgs(
				host,
				test.ParentAddress,
				[]*vmcommon.KDATransfer{transfer},
				int64(testConfig.GasProvidedToChild), // #nosec G115 - max int64
				nil,
				nil)
			if ret != 0 {
				host.Runtime().FailExecution(fmt.Errorf("Transfer KDA failed"))
			}

		}

		if err != nil {
			host.Runtime().FailExecution(err)
		}

		return instance
	})
}

// test variables
var (
	TestStorageValue1 = []byte{1, 2, 3, 4}
	TestStorageValue2 = []byte{1, 2, 3}
	TestStorageValue3 = []byte{1, 2}
	TestStorageValue4 = []byte{1}
)

// ParentSetStorageMock is an exposed mock contract method
func ParentSetStorageMock(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*test.TestConfig)
	instanceMock.AddMockMethod("parentSetStorage", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		_, _ = host.Storage().SetStorage(test.ParentKeyA, TestStorageValue1) // add
		_, _ = host.Storage().SetStorage(test.ParentKeyA, TestStorageValue2) // delete
		_, _ = host.Storage().SetStorage(test.ParentKeyB, TestStorageValue2) // add
		_, _ = host.Storage().SetStorage(test.ParentKeyB, TestStorageValue3) // delete

		input := test.DefaultTestContractCallInput()
		input.GasProvided = testConfig.GasProvidedToChild
		input.CallerAddr = instance.Address
		input.RecipientAddr = test.ChildAddress
		input.Function = "childSetStorage"

		arguments := host.Runtime().Arguments()
		var returnValue int32
		if bytes.Equal(arguments[0], []byte{0}) {
			returnValue = ExecuteOnSameContextInMockContracts(host, input, big.NewInt(0))
		} else {
			returnValue = ExecuteOnDestContextInMockContracts(host, input, big.NewInt(0))
		}
		if returnValue != 0 {
			host.Runtime().FailExecution(fmt.Errorf("return value %d", returnValue))
		}

		return instance
	})
}

// ChildSetStorageMock is an exposed mock contract method
func ChildSetStorageMock(instanceMock *mock.InstanceMock, _ interface{}) {
	instanceMock.AddMockMethod("childSetStorage", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		_, _ = host.Storage().SetStorage(test.ChildKey, TestStorageValue2)  // add
		_, _ = host.Storage().SetStorage(test.ChildKey, TestStorageValue1)  // add
		_, _ = host.Storage().SetStorage(test.ChildKeyB, TestStorageValue1) // add
		_, _ = host.Storage().SetStorage(test.ChildKeyB, TestStorageValue4) // delete
		return instance
	})
}

// SimpleChildSetStorageMock is an exposed mock contract method
func SimpleChildSetStorageMock(instanceMock *mock.InstanceMock, config interface{}) {
	instanceMock.AddMockMethod("simpleChildSetStorage", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		_, _ = host.Storage().SetStorage(test.ChildKey, test.ChildData)
		return instance
	})
}
