package twoscomplement

import "math/big"

// FromBytes converts a byte array to a number.
// Interprets input as a 2's complement representation if the first bit (most significant) is 1.
// Big endian.
func FromBytes(twosBytes []byte) *big.Int {
	return SetBytes(new(big.Int), twosBytes)
}

// SetBytes changes z to be the number represented as bytes in 2's complement,
// and returns z.
// Big endian.
func SetBytes(z *big.Int, twosBytes []byte) *big.Int {
	if len(twosBytes) == 0 {
		return z.SetInt64(0)
	}

	mostSignificantBit := twosBytes[0] >> 7
	if mostSignificantBit == 0 {
		// positive number, no further processing required
		z = z.SetBytes(twosBytes)
	} else {
		// convert to negative number
		notBytes := make([]byte, len(twosBytes))
		for i, b := range twosBytes {
			notBytes[i] = ^b // negate every bit
		}
		z = z.SetBytes(notBytes)
		z = z.Neg(z)
		z = z.Sub(z, bigOne) // -1
	}

	return z
}
