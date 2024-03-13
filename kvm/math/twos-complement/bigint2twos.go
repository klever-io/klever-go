package twoscomplement

import (
	"fmt"
	"math/big"
)

var bigOne = big.NewInt(1)

// ToBytes returns a byte array of variable length representing the input big.Int in two's complement.
// Does not alter input.
func ToBytes(bi *big.Int) []byte {
	var resultBytes []byte
	switch bi.Sign() {
	case -1:
		// compute 2's complement
		plus1 := big.NewInt(0)
		plus1 = plus1.Add(bi, bigOne) // add 1
		resultBytes = plus1.Bytes()
		for i, b := range resultBytes {
			resultBytes[i] = ^b // negate every bit
		}
		if len(resultBytes) == 0 || msbIsZero(resultBytes[0]) {
			// if test bit is not 1,
			// add another byte in front
			// to disambiguate from a positive number
			resultBytes = append([]byte{0xFF}, resultBytes...)
		}
	case 0:
		return []byte{}
	case 1:
		resultBytes = bi.Bytes()
		if msbIsOne(resultBytes[0]) {
			// if test bit is not 0,
			// add another byte in front
			// to disambiguate from a negative number
			resultBytes = append([]byte{0x00}, resultBytes...)
		}
	}

	return resultBytes
}

// ToBytesOfLength returns a byte array representation, 2's complement if number is negative.
// Big endian.
// Will return error if value does not fit in requested number of bytes.
func ToBytesOfLength(i *big.Int, bytesLength int) ([]byte, error) {
	switch i.Sign() {
	case -1:
		// compute 2's complement
		plus1 := big.NewInt(0)
		plus1 = plus1.Add(i, bigOne) // add 1
		plus1Bytes := plus1.Bytes()

		// validation
		minimumBytes := len(plus1Bytes)
		if len(plus1Bytes) == 0 || msbIsOne(plus1Bytes[0]) {
			// if leading bit is not 0, then the resulting test bit will not be 1 (gets XOR-ed),
			// require another byte in front
			// to disambiguate from a positive number
			minimumBytes++
		}
		if bytesLength < minimumBytes {
			return []byte{}, fmt.Errorf("representation of %d does not fit in %d bytes", i, bytesLength)
		}

		// copy bytes
		offset := len(plus1Bytes) - bytesLength
		resultBytes := make([]byte, bytesLength)
		for i := 0; i < bytesLength; i++ {
			j := offset + i
			if j < 0 {
				resultBytes[i] = 255 // pad left with 11111111
			} else {
				resultBytes[i] = ^plus1Bytes[j] // also negate every bit
			}
		}
		return resultBytes, nil
	case 0:
		// just zeroes
		return make([]byte, bytesLength), nil
	case 1:
		originalBytes := i.Bytes()

		// validation
		minimumBytes := len(originalBytes)
		if msbIsOne(originalBytes[0]) {
			// if test bit is not 0,
			// add another byte in front
			// to disambiguate from a negative number
			minimumBytes++
		}
		if bytesLength < minimumBytes {
			return []byte{}, fmt.Errorf("representation of %d does not fit in %d bytes", i, bytesLength)
		}

		// copy bytes
		return CopyAlignRight(originalBytes, bytesLength), nil
	}

	// unreachable
	panic("unreachable")
}
