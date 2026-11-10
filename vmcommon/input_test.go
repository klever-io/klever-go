package vmcommon

import (
	"math"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/stretchr/testify/assert"
)

func TestCheckKLVName(t *testing.T) {
	// Test when id is nil
	id := checkKLVName(nil)
	assert.Equal(t, kdautils.KLVIdentifier, id)

	// Test when id is empty
	id = checkKLVName([]byte{})
	assert.Equal(t, kdautils.KLVIdentifier, id)

	// Test with KLV
	id = checkKLVName([]byte("KLV"))
	assert.Equal(t, kdautils.KLVIdentifier, id)

	// Test when id is not empty
	id = checkKLVName([]byte("test_id"))
	assert.Equal(t, []byte("test_id"), id)
}

func TestVMInput_GetKDACallValue(t *testing.T) {
	input := VMInput{
		KDATransfers: []*KDATransfer{
			{
				KDAValue:     big.NewInt(10),
				KDATokenName: []byte("KLV"),
			},
			{
				KDAValue:     big.NewInt(20),
				KDATokenName: []byte("KLV"),
			},
		},
	}

	// Test getting KDA call value for a token that exists
	value := input.GetKDACallValue([]byte("KLV"))
	assert.Equal(t, big.NewInt(30), value)

	// Test getting KDA call value for a token that doesn't exist
	value = input.GetKDACallValue([]byte("nonexistent"))
	assert.Equal(t, big.NewInt(0), value)

	// Test getting KDA call value with nil id (should return default KLV)
	value = input.GetKDACallValue(nil)
	assert.Equal(t, big.NewInt(30), value)
}

func TestKDATransfer_SetExecuted(t *testing.T) {
	transfer := &KDATransfer{}
	assert.False(t, transfer.executed)

	transfer.SetExecuted()
	assert.True(t, transfer.executed)
}

func TestKDATransfer_IsExecuted(t *testing.T) {
	transfer := &KDATransfer{}
	assert.False(t, transfer.IsExecuted())

	transfer.SetExecuted()
	assert.True(t, transfer.IsExecuted())
}

func TestCountPendingKDATransfers(t *testing.T) {
	assert.Equal(t, uint64(0), CountPendingKDATransfers(nil))

	transfers := []*KDATransfer{{}, {}, {}}
	assert.Equal(t, uint64(3), CountPendingKDATransfers(transfers))

	transfers[0].SetExecuted()
	transfers[2].SetExecuted()
	assert.Equal(t, uint64(1), CountPendingKDATransfers(transfers))
}

func TestKDATransfer_Clone(t *testing.T) {
	original := &KDATransfer{
		KDAValue:      big.NewInt(100),
		KDATokenName:  []byte("KLV"),
		KDATokenType:  1,
		KDATokenNonce: 12345,
		executed:      true,
		KDARoyalties:  10,
		KLVRoyalties:  20,
	}

	clone := original.Clone()

	// Check that the clone is a deep copy and not the same reference
	assert.NotSame(t, original, &clone)

	// Check that the values were copied correctly
	assert.Equal(t, original.KDAValue, clone.KDAValue)
	assert.Equal(t, original.KDATokenName, clone.KDATokenName)
	assert.Equal(t, original.KDATokenType, clone.KDATokenType)
	assert.Equal(t, original.KDATokenNonce, clone.KDATokenNonce)
	assert.Equal(t, original.executed, clone.executed)
	assert.Equal(t, original.KDARoyalties, clone.KDARoyalties)
	assert.Equal(t, original.KLVRoyalties, clone.KLVRoyalties)
}

func TestContractCallInput_Iterator(t *testing.T) {
	ccInput := &ContractCallInput{
		VMInput: VMInput{
			Arguments: [][]byte{
				[]byte("arg1"),
				[]byte("arg2"),
			},
		},
	}

	// Test HasNextArg
	assert.True(t, ccInput.HasNextArg())
	arg := ccInput.NextArg()
	assert.Equal(t, "arg1", string(arg))

	// Test NextArg
	assert.True(t, ccInput.HasNextArg())
	arg = ccInput.NextArg()
	assert.Equal(t, "arg2", string(arg))

	// No more arguments
	assert.False(t, ccInput.HasNextArg())
}

func TestArgument_Bytes(t *testing.T) {
	var arg Argument = []byte("test")
	assert.Equal(t, []byte("test"), arg.Bytes())

	var nilArg Argument = nil
	assert.Nil(t, nilArg.Bytes())
}

func TestArgument_String(t *testing.T) {
	var arg Argument = []byte("test")
	assert.Equal(t, "test", arg.String())

	var nilArg Argument = nil
	assert.Equal(t, "", nilArg.String())
}

var (
	maxUint64Argument  Argument = big.NewInt(0).SetUint64(math.MaxUint64).Bytes()
	twoPow64Argument   Argument = big.NewInt(0).Lsh(big.NewInt(1), 64).Bytes()
	oversizedArgument  Argument = big.NewInt(0).Add(big.NewInt(0).Lsh(big.NewInt(1), 64), big.NewInt(5)).Bytes()
	zeroPaddedArgument Argument = []byte{0, 0, 0, 0, 0, 0, 0, 0, 5}
)

func TestArgument_Uint64(t *testing.T) {
	var arg Argument = big.NewInt(100).Bytes()
	assert.Equal(t, uint64(100), arg.Uint64())

	assert.Equal(t, uint64(math.MaxUint64), maxUint64Argument.Uint64())
	assert.Equal(t, uint64(math.MaxUint64), twoPow64Argument.Uint64())
	assert.Equal(t, uint64(math.MaxUint64), oversizedArgument.Uint64())
	assert.Equal(t, uint64(5), zeroPaddedArgument.Uint64())

	var nilArg Argument = nil
	assert.Equal(t, uint64(0), nilArg.Uint64())
}

func TestArgument_Uint32(t *testing.T) {
	var arg Argument = big.NewInt(100).Bytes()
	assert.Equal(t, uint32(100), arg.Uint32())

	assert.Equal(t, uint32(math.MaxUint32), maxUint64Argument.Uint32())
	assert.Equal(t, uint32(math.MaxUint32), twoPow64Argument.Uint32())
	assert.Equal(t, uint32(math.MaxUint32), oversizedArgument.Uint32())
	assert.Equal(t, uint32(5), zeroPaddedArgument.Uint32())

	var nilArg Argument = nil
	assert.Equal(t, uint32(0), nilArg.Uint32())
}

func TestArgument_Int64(t *testing.T) {
	var arg Argument = big.NewInt(100).Bytes()
	assert.Equal(t, int64(100), arg.Int64())

	assert.Equal(t, int64(-1), maxUint64Argument.Int64())
	assert.Equal(t, int64(-1), twoPow64Argument.Int64())
	assert.Equal(t, int64(-1), oversizedArgument.Int64())
	assert.Equal(t, int64(5), zeroPaddedArgument.Int64())

	var nilArg Argument = nil
	assert.Equal(t, int64(0), nilArg.Int64())
}

func TestArgument_Int32(t *testing.T) {
	var arg Argument = big.NewInt(100).Bytes()
	assert.Equal(t, int32(100), arg.Int32())

	var overflowArg Argument = big.NewInt(0).SetUint64(math.MaxUint64).Bytes()
	assert.Equal(t, int32(math.MaxInt32), overflowArg.Int32()) // Overflow

	assert.Equal(t, int32(math.MaxInt32), twoPow64Argument.Int32())
	assert.Equal(t, int32(math.MaxInt32), oversizedArgument.Int32())
	assert.Equal(t, int32(5), zeroPaddedArgument.Int32())

	var nilArg Argument = nil
	assert.Equal(t, int32(0), nilArg.Int32())
}

func TestArgument_Bool(t *testing.T) {
	var arg Argument = []byte{1}
	assert.True(t, arg.Bool())

	var falseArg Argument = []byte{0}
	assert.False(t, falseArg.Bool())

	var nilArg Argument = nil
	assert.False(t, nilArg.Bool())
}
