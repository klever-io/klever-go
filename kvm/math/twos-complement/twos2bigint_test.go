package twoscomplement

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromBytes(t *testing.T) {
	assertFromBytesOk(t, []byte{}, "0")
	assertFromBytesOk(t, []byte{0x00}, "0")
	assertFromBytesOk(t, []byte{0x01}, "1")
	assertFromBytesOk(t, []byte{0xFF}, "-1")
	assertFromBytesOk(t, []byte{0x00, 0xFF}, "255")
	assertFromBytesOk(t, []byte{0x01, 0x00}, "256")
	assertFromBytesOk(t, []byte{0xFF, 0xFF}, "-1")
	assertFromBytesOk(t, []byte{0xFF, 0xFE}, "-2")
	assertFromBytesOk(t, []byte{0xFF, 0x00}, "-256")
}

func assertFromBytesOk(t *testing.T, input []byte, expected string) {
	conv := FromBytes(input)
	expectedBi := big.NewInt(0)
	_ = expectedBi.UnmarshalText([]byte(expected))
	assert.True(t, conv.Cmp(expectedBi) == 0, "FromBytes returned wrong result")
}
