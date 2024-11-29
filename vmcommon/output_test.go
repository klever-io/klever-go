package vmcommon

import (
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFirstReturnData_VMOutputWithNoReturnDataShouldErr(t *testing.T) {
	vmOutput := VMOutput{
		ReturnData: [][]byte{},
	}

	_, err := vmOutput.GetFirstReturnData(vm.AsBigInt)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no return data")
}

func TestGetFirstReturnData_WithBadReturnDataKindShouldErr(t *testing.T) {
	vmOutput := VMOutput{
		ReturnData: [][]byte{[]byte("100")},
	}

	_, err := vmOutput.GetFirstReturnData(42)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "can't interpret")
}

func TestGetFirstReturnData(t *testing.T) {
	value := big.NewInt(100)

	vmOutput := VMOutput{
		ReturnData: [][]byte{value.Bytes()},
	}

	dataAsBigInt, _ := vmOutput.GetFirstReturnData(vm.AsBigInt)
	dataAsBigIntString, _ := vmOutput.GetFirstReturnData(vm.AsBigIntString)
	dataAsString, _ := vmOutput.GetFirstReturnData(vm.AsString)
	dataAsHex, _ := vmOutput.GetFirstReturnData(vm.AsHex)

	assert.Equal(t, value, dataAsBigInt)
	assert.Equal(t, "100", dataAsBigIntString)
	assert.Equal(t, string(value.Bytes()), dataAsString)
	assert.Equal(t, "64", dataAsHex)
}

func TestOutputContext_MergeCompleteAccounts(t *testing.T) {
	t.Parallel()

	transfer1 := OutputTransfer{
		KDATransfers: KDATransfer{
			KDAValue: big.NewInt(1),
		},
	}
	left := &OutputAccount{
		Address:         []byte("addr1"),
		StorageUpdates:  nil,
		Code:            []byte("code1"),
		OutputTransfers: []OutputTransfer{transfer1},
	}
	right := &OutputAccount{
		Address:         []byte("addr2"),
		StorageUpdates:  map[string]*StorageUpdate{"key": {Data: []byte("data"), Offset: []byte("offset")}},
		Code:            []byte("code2"),
		OutputTransfers: []OutputTransfer{transfer1, transfer1},
	}

	expected := &OutputAccount{
		Address:         []byte("addr2"),
		StorageUpdates:  map[string]*StorageUpdate{"key": {Data: []byte("data"), Offset: []byte("offset")}},
		Code:            []byte("code2"),
		OutputTransfers: []OutputTransfer{transfer1, transfer1},
	}

	left.MergeOutputAccounts(right)
	require.Equal(t, expected, left)
}

func TestOutputTransfer_Clone(t *testing.T) {
	original := &OutputTransfer{
		Index:         1,
		SenderAddress: []byte("sender"),
		RcvAddr:       []byte("receiver"),
		KDATransfers: KDATransfer{
			KDAValue: big.NewInt(100),
		},
	}

	cloned := original.Clone()

	assert.Equal(t, original.Index, cloned.Index)
	assert.Equal(t, original.SenderAddress, cloned.SenderAddress)
	assert.Equal(t, original.RcvAddr, cloned.RcvAddr)
	assert.Equal(t, original.KDATransfers.KDAValue, cloned.KDATransfers.KDAValue)

	// Modify cloned to ensure it's a deep copy
	cloned.Index = 2
	cloned.SenderAddress[0] = 'S'
	cloned.RcvAddr[0] = 'R'
	cloned.KDATransfers.KDAValue.SetInt64(200)

	assert.NotEqual(t, original.Index, cloned.Index)
	assert.NotEqual(t, original.SenderAddress, cloned.SenderAddress)
	assert.NotEqual(t, original.RcvAddr, cloned.RcvAddr)
	assert.NotEqual(t, original.KDATransfers.KDAValue, cloned.KDATransfers.KDAValue)
}

func TestFormatLogDataForCall(t *testing.T) {
	callType := "sc_call"
	functionName := "transfer"
	functionArgs := [][]byte{[]byte("arg1"), []byte("arg2")}

	result := FormatLogDataForCall(callType, functionName, functionArgs)

	assert.Equal(t, 4, len(result))
	assert.Equal(t, []byte(callType), result[0])
	assert.Equal(t, []byte(functionName), result[1])
	assert.Equal(t, functionArgs[0], result[2])
	assert.Equal(t, functionArgs[1], result[3])
}

type mockNextOutputTransferIndexProvider struct {
	crtIndex uint32
}

func (m *mockNextOutputTransferIndexProvider) GetCrtTransferIndex() uint32 {
	return m.crtIndex
}

func (m *mockNextOutputTransferIndexProvider) SetCrtTransferIndex(index uint32) {
	m.crtIndex = index
}

func (m *mockNextOutputTransferIndexProvider) NextOutputTransferIndex() uint32 {
	index := m.crtIndex
	m.crtIndex++
	return index
}

func (m *mockNextOutputTransferIndexProvider) IsInterfaceNil() bool {
	return m == nil
}

func TestVMOutput_ReindexTransfers(t *testing.T) {
	vmOutput := &VMOutput{
		OutputAccounts: map[string]*OutputAccount{
			"acc1": {
				OutputTransfers: []OutputTransfer{
					{Index: 1},
					{Index: 2},
				},
			},
			"acc2": {
				OutputTransfers: []OutputTransfer{
					{Index: 3},
				},
			},
		},
	}
	mockIndexer := &mockNextOutputTransferIndexProvider{crtIndex: 10}

	err := vmOutput.ReindexTransfers(mockIndexer)
	require.NoError(t, err)

	assert.Equal(t, uint32(10), vmOutput.OutputAccounts["acc1"].OutputTransfers[0].Index)
	assert.Equal(t, uint32(11), vmOutput.OutputAccounts["acc1"].OutputTransfers[1].Index)
	assert.Equal(t, uint32(12), vmOutput.OutputAccounts["acc2"].OutputTransfers[0].Index)
	assert.Equal(t, uint32(13), mockIndexer.GetCrtTransferIndex())
}

func TestVMOutput_GetNextAvailableOutputTransferIndex(t *testing.T) {
	vmOutput := &VMOutput{
		OutputAccounts: map[string]*OutputAccount{
			"acc1": {
				OutputTransfers: []OutputTransfer{
					{Index: 1},
					{Index: 3},
				},
			},
			"acc2": {
				OutputTransfers: []OutputTransfer{
					{Index: 2},
				},
			},
		},
	}

	nextIndex := vmOutput.GetNextAvailableOutputTransferIndex()
	assert.Equal(t, uint32(4), nextIndex)
}

func TestVMOutput_ComputeTotalGasConsumed(t *testing.T) {
	t.Run("only counts system logs", func(t *testing.T) {
		vmOutput := &VMOutput{
			Logs: []*LogEntry{
				{
					Identifier: []byte(core.TotalConsumedGasString),
					Topics:     [][]byte{big.NewInt(100).Bytes()},
					IsSystemLog:   true,
				},
				{
					// Contract trying to spoof system log
					Identifier: []byte(core.TotalConsumedGasString),
					Topics:     [][]byte{big.NewInt(500).Bytes()},
					IsSystemLog:   false,
				},
				{
					Identifier: []byte(core.TotalConsumedGasString),
					Topics:     [][]byte{big.NewInt(50).Bytes()},
					IsSystemLog:   true,
				},
			},
		}

		totalGas := vmOutput.ComputeTotalGasConsumed()
		assert.Equal(t, big.NewInt(150), totalGas, "should only sum gas from system logs")
	})

	t.Run("handles various log types", func(t *testing.T) {
		vmOutput := &VMOutput{
			Logs: []*LogEntry{
				{
					Identifier: []byte(core.TotalConsumedGasString),
					Topics:     [][]byte{big.NewInt(100).Bytes()},
					IsSystemLog:   true,
				},
				{
					Identifier: []byte("other"),
					Topics:     [][]byte{big.NewInt(1000).Bytes()},
					IsSystemLog:   true,
				},
				{
					Identifier: []byte(core.TotalConsumedGasString),
					Topics:     [][]byte{big.NewInt(50).Bytes()},
					IsSystemLog:   true,
				},
			},
		}

		totalGas := vmOutput.ComputeTotalGasConsumed()
		assert.Equal(t, big.NewInt(150), totalGas)
	})
}

func TestVMOutput_IsInterfaceNil(t *testing.T) {
	var vmOutput *VMOutput = nil
	assert.True(t, vmOutput.IsInterfaceNil())

	vmOutput = &VMOutput{}
	assert.False(t, vmOutput.IsInterfaceNil())
}
