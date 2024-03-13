package managedbigint

import "math/big"

// And sets dest = x & y.
func (c *BigIntContainer) And(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).And, dest, x, y)
}

// AndNot sets dest = x &^ y.
func (c *BigIntContainer) AndNot(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).AndNot, dest, x, y)
}

// Or sets dest = x | y.
func (c *BigIntContainer) Or(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Or, dest, x, y)
}

// Xor sets dest = x ^ y.
func (c *BigIntContainer) Xor(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Xor, dest, x, y)
}

// Not sets dest = ^x.
func (c *BigIntContainer) Not(dest, x BigIntHandle) BigIntHandle {
	return c.performUnaryOperation((*big.Int).Not, dest, x)
}
