package contracts

import (
	"math/big"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/stretchr/testify/require"
)

// DeployContractFromSourceMock -
func DeployContractFromSourceMock(instanceMock *mock.InstanceMock, _ interface{}) {
	instanceMock.AddMockMethod("deployContractFromSource", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		t := instance.T

		arguments := host.Runtime().Arguments()

		if len(arguments) < 3 {
			host.Runtime().SignalUserError("wrong num of arguments")
			return instance
		}

		sourceContractAddress := arguments[0]
		codeMetadata := arguments[1]
		gasForInit := big.NewInt(0).SetBytes(arguments[2])

		newAddress, err :=
			vmhooks.DeployFromSourceContractWithTypedArgs(
				host,
				sourceContractAddress,
				codeMetadata,
				big.NewInt(0),
				[][]byte{},
				gasForInit.Int64(),
			)

		if err != nil {
			host.Runtime().FailExecution(err)
			return instance
		}

		require.NotNil(t, newAddress)

		host.Output().Finish(newAddress)

		return instance
	})
}

// InitMockMethod -
func InitMockMethod(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*testcommon.TestConfig)
	instanceMock.AddMockMethod("init", testcommon.SimpleWasteGasMockMethod(instanceMock, testConfig.GasUsedByInit))
}

func UpgradeMockMethod(instanceMock *mock.InstanceMock, config interface{}) {
	testConfig := config.(*testcommon.TestConfig)
	instanceMock.AddMockMethod("upgrade", testcommon.SimpleWasteGasMockMethod(instanceMock, testConfig.GasUsedByInit))
}

// UpdateContractFromSourceMock -
func UpdateContractFromSourceMock(instanceMock *mock.InstanceMock, _ interface{}) {
	instanceMock.AddMockMethod("updateContractFromSource", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)

		arguments := host.Runtime().Arguments()

		if len(arguments) < 4 {
			host.Runtime().SignalUserError("wrong num of arguments")
			return instance
		}

		sourceContractAddress := arguments[0]
		destinationContractAddress := arguments[1]
		codeMetadata := arguments[2]
		gasForInit := big.NewInt(0).SetBytes(arguments[3])

		vmhooks.UpgradeFromSourceContractWithTypedArgs(
			host,
			sourceContractAddress,
			destinationContractAddress,
			big.NewInt(0).Bytes(),
			[][]byte{},
			gasForInit.Int64(),
			codeMetadata,
		)

		return instance
	})
}
