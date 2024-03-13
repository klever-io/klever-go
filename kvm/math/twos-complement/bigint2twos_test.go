package twoscomplement

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToBytesOf(t *testing.T) {
	assertToBytesOk(t, "0", []byte{})
	assertToBytesOk(t, "1", []byte{0x01})
	assertToBytesOk(t, "-1", []byte{0xFF})
	assertToBytesOk(t, "-2", []byte{0xFE})
	assertToBytesOk(t, "127", []byte{0x7f})
	assertToBytesOk(t, "128", []byte{0x00, 0x80}) // first bit 1 requires extra 0-byte to keep sign
	assertToBytesOk(t, "255", []byte{0x00, 0xFF})
	assertToBytesOk(t, "256", []byte{0x01, 0x00})
	assertToBytesOk(t, "-255", []byte{0xFF, 0x01})
	assertToBytesOk(t, "-256", []byte{0xFF, 0x00})
	assertToBytesOk(t, "-257", []byte{0xFE, 0xFF})
}

func assertToBytesOk(t *testing.T, input string, expected []byte) {
	inputBi := big.NewInt(0)
	_ = inputBi.UnmarshalText([]byte(input))

	result := ToBytes(inputBi)
	assert.True(t, bytes.Equal(result, expected), "ToBytes returned wrong result. Want: %v. Have: %v", expected, result)
}

func TestToBytesOfLength(t *testing.T) {
	assertToBytesOfLengthOk(t, "0", 0, []byte{})
	assertToBytesOfLengthOk(t, "0", 1, []byte{0x00})
	assertToBytesOfLengthOk(t, "1", 1, []byte{0x01})
	assertToBytesOfLengthErr(t, "1", 0)
	assertToBytesOfLengthOk(t, "-1", 1, []byte{0xFF})
	assertToBytesOfLengthErr(t, "-2", 0)
	assertToBytesOfLengthOk(t, "0", 3, []byte{0x00, 0x00, 0x00})
	assertToBytesOfLengthOk(t, "1", 3, []byte{0x00, 0x00, 0x01})
	assertToBytesOfLengthOk(t, "-1", 3, []byte{0xFF, 0xFF, 0xFF})

	assertToBytesOfLengthOk(t, "128", 2, []byte{0x00, 0x80})
	assertToBytesOfLengthOk(t, "255", 2, []byte{0x00, 0xFF})
	assertToBytesOfLengthOk(t, "256", 2, []byte{0x01, 0x00})
	assertToBytesOfLengthOk(t, "-255", 2, []byte{0xFF, 0x01})
	assertToBytesOfLengthOk(t, "-256", 2, []byte{0xFF, 0x00})
	assertToBytesOfLengthOk(t, "-257", 2, []byte{0xFE, 0xFF})

	assertToBytesOfLengthErr(t, "128", 1)
	assertToBytesOfLengthErr(t, "255", 1)
	assertToBytesOfLengthErr(t, "256", 1)
	assertToBytesOfLengthErr(t, "-255", 1)
	assertToBytesOfLengthErr(t, "-256", 1)
	assertToBytesOfLengthErr(t, "-257", 1)
}

func assertToBytesOfLengthOk(t *testing.T, input string, length int, expected []byte) {
	inputBi := big.NewInt(0)
	_ = inputBi.UnmarshalText([]byte(input))

	result, err := ToBytesOfLength(inputBi, length)
	assert.Nil(t, err)
	assert.True(t, bytes.Equal(result, expected), "ToBytesOfLength returned wrong result. Want: %v. Have: %v.", expected, result)
}

func assertToBytesOfLengthErr(t *testing.T, input string, length int) {
	inputBi := big.NewInt(0)
	_ = inputBi.UnmarshalText([]byte(input))

	result, err := ToBytesOfLength(inputBi, length)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf("representation of %d does not fit in %d bytes", inputBi, length))
	assert.Equal(t, len(result), 0)
}
