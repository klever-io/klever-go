package vmcommon

import (
	"math/big"
	"testing"

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
