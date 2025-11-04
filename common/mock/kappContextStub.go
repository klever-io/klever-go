package mock

import (
	"time"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/block"
)

var _ kapp.KappContext = (*KAppContextStub)(nil)

type KAppContextStub struct {
	ReceiptsValue               kapp.ReceiptsContext
	OriginalSenderCalled        func() []byte
	ReceiptsCalled              func() kapp.ReceiptsContext
	ContractIDCalled            func() int
	BlockCalled                 func() *block.Block
	TxHashCalled                func() []byte
	TxNonceCalled               func() uint64
	SetContractIDCalled         func(id int)
	SetReturnDataCalled         func(data [][]byte)
	AddReturnDataCalled         func(data []byte)
	GetAndClearReturnDataCalled func() [][]byte
	SubGasUsedCalled            func(gasUsed uint64) error
	GetGasLimitCalled           func() uint64
	GetExecDataCalled           func() []byte
	IsScSimulationCalled        func() bool
	SetExecutionTimeCalled      func(duration time.Duration)
	GetExecutionTimeCalled      func() time.Duration
}

func (k *KAppContextStub) OriginalSender() []byte {
	if k.OriginalSenderCalled != nil {
		return k.OriginalSenderCalled()
	}
	return nil
}

func (k *KAppContextStub) Receipts() kapp.ReceiptsContext {
	if k.ReceiptsCalled != nil {
		return k.ReceiptsCalled()
	}
	return k.ReceiptsValue
}

func (k *KAppContextStub) ContractID() int {
	if k.ContractIDCalled != nil {
		return k.ContractIDCalled()
	}
	return 0
}

func (k *KAppContextStub) Block() *block.Block {
	if k.BlockCalled != nil {
		return k.BlockCalled()
	}
	return nil
}

func (k *KAppContextStub) TxHash() []byte {
	if k.TxHashCalled != nil {
		return k.TxHashCalled()
	}
	return nil
}

func (k *KAppContextStub) TxNonce() uint64 {
	if k.TxNonceCalled != nil {
		return k.TxNonceCalled()
	}
	return 0
}

func (k *KAppContextStub) SetContractID(id int) {
	if k.SetContractIDCalled != nil {
		k.SetContractIDCalled(id)
	}
}

func (k *KAppContextStub) SetReturnData(data [][]byte) {
	if k.SetReturnDataCalled != nil {
		k.SetReturnDataCalled(data)
	}
}

func (k *KAppContextStub) AddReturnData(data []byte) {
	if k.AddReturnDataCalled != nil {
		k.AddReturnDataCalled(data)
	}
}

func (k *KAppContextStub) GetAndClearReturnData() [][]byte {
	if k.GetAndClearReturnDataCalled != nil {
		return k.GetAndClearReturnDataCalled()
	}
	return nil
}

func (k *KAppContextStub) SubGasUsed(gasUsed uint64) error {
	if k.SubGasUsedCalled != nil {
		return k.SubGasUsedCalled(gasUsed)
	}
	return nil
}

func (k *KAppContextStub) GetGasLimit() uint64 {
	if k.GetGasLimitCalled != nil {
		return k.GetGasLimitCalled()
	}
	return 0
}

func (k *KAppContextStub) GetExecData() []byte {
	if k.GetExecDataCalled != nil {
		return k.GetExecDataCalled()
	}
	return nil
}

func (k *KAppContextStub) IsScSimulation() bool {
	if k.IsScSimulationCalled != nil {
		return k.IsScSimulationCalled()
	}
	return false
}

func (k *KAppContextStub) SetExecutionTime(duration time.Duration) {
	if k.SetExecutionTimeCalled != nil {
		k.SetExecutionTimeCalled(duration)
	}
}

func (k *KAppContextStub) GetExecutionTime() time.Duration {
	if k.GetExecutionTimeCalled != nil {
		return k.GetExecutionTimeCalled()
	}
	return 0
}
