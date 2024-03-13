package managedbigint

// SetBytes interprets buf as the bytes of a big-endian unsigned
// integer, sets dest to that value, and returns dest.
func (c *BigIntContainer) SetBytes(dest BigIntHandle, buf []byte) BigIntHandle {
	c.loadBigInt(dest, c.destination)

	destDataBefore := c.destination.Bits()

	c.destination = c.destination.SetBytes(buf)

	destDataAfter := c.destination.Bits()

	if bigIntDataMoved(destDataBefore, destDataAfter) {
		return c.Insert(c.destination)
	}

	dest.negative = false
	return dest
}

// GetBytes returns the absolute value of x as a big-endian byte slice.
func (c *BigIntContainer) GetBytes(x BigIntHandle) []byte {
	c.loadBigInt(x, c.register1)
	return c.register1.Bytes()
}

// BitLen returns the length of the absolute value of x in bits.
// The bit length of 0 is 0.
func (c *BigIntContainer) BitLen(x BigIntHandle) int {
	c.loadBigInt(x, c.register1)
	return c.register1.BitLen()
}

// ByteLen returns the minimum number of bytes required to represent the absolute value of x.
// The byte length of 0 is 0.
func (c *BigIntContainer) ByteLen(x BigIntHandle) int {
	c.loadBigInt(x, c.register1)
	bitLen := c.register1.BitLen()
	if bitLen == 0 {
		return 0
	}
	return (bitLen-1)/8 + 1
}

// IsInt64 reports whether x can be represented as an int64.
func (c *BigIntContainer) IsInt64(x BigIntHandle) bool {
	c.loadBigInt(x, c.register1)
	return c.register1.IsInt64()
}

// ToInt64 returns the int64 representation of x.
// If x cannot be represented in an int64, the result is undefined.
func (c *BigIntContainer) ToInt64(x BigIntHandle) int64 {
	c.loadBigInt(x, c.register1)
	return c.register1.Int64()
}

// SetInt64 sets value to dest.
func (c *BigIntContainer) SetInt64(dest BigIntHandle, value int64) BigIntHandle {
	c.loadBigInt(dest, c.destination)

	destDataBefore := c.destination.Bits()

	c.destination = c.destination.SetInt64(value)

	destDataAfter := c.destination.Bits()

	if bigIntDataMoved(destDataBefore, destDataAfter) {
		return c.Insert(c.destination)
	}

	// maybe dest changed sign
	dest.negative = c.destination.Sign() < 0

	return dest
}
