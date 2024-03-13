package contracts

import (
	"math/big"

	mock "github.com/klever-io/klever-go/kvm/mock/context"
)

// GasMismatchCallBackParentMock is an exposed mock contract method
func GasMismatchCallBackParentMock(instanceMock *mock.InstanceMock, _ interface{}) {
	instanceMock.AddMockMethod("callBack", func() *mock.InstanceMock {
		host := instanceMock.Host
		instance := mock.GetMockInstance(host)
		output := host.Output()
		arguments := host.Runtime().Arguments()

		output.Finish(big.NewInt(0xCA11BAC3).Bytes())

		for _, arg := range arguments {
			output.Finish(arg)
		}

		output.Finish(big.NewInt(0xCA11BAC3).Bytes())
		return instance
	})
}
