package managedbigint

import "math/big"

// Cmp compares x and y and returns:
//
//	-1 if x <  y
//	 0 if x == y
//	+1 if x >  y
func (c *BigIntContainer) Cmp(x, y BigIntHandle) int {
	c.loadBigInt(x, c.register1)
	c.loadBigInt(y, c.register2)
	return c.register1.Cmp(c.register2)
}

// CmpAbs compares the absolute values of x and y and returns:
//
//	-1 if |x| <  |y|
//	 0 if |x| == |y|
//	+1 if |x| >  |y|
func (c *BigIntContainer) CmpAbs(x, y BigIntHandle) int {
	c.loadBigInt(x, c.register1)
	c.loadBigInt(y, c.register2)
	return c.register1.CmpAbs(c.register2)
}

// Abs sets dest to |x| (the absolute value of x).
func (c *BigIntContainer) Abs(dest, x BigIntHandle) BigIntHandle {
	return c.performUnaryOperation((*big.Int).Abs, dest, x)
}

// Neg sets dest to -x.
func (c *BigIntContainer) Neg(dest, x BigIntHandle) BigIntHandle {
	return c.performUnaryOperation((*big.Int).Neg, dest, x)
}

// Add sets dest to the sum x+y.
func (c *BigIntContainer) Add(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Add, dest, x, y)
}

// Sub sets dest to the difference x-y.
func (c *BigIntContainer) Sub(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Sub, dest, x, y)
}

// Mul sets dest to the product x*y.
func (c *BigIntContainer) Mul(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Mul, dest, x, y)
}

// Quo sets dest to the quotient x/y for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
// Quo implements truncated division (like Go); see QuoRem for more details.
func (c *BigIntContainer) Quo(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Quo, dest, x, y)
}

// Rem sets dest to the remainder x%y for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
// Rem implements truncated modulus (like Go); see QuoRem for more details.
func (c *BigIntContainer) Rem(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Rem, dest, x, y)
}

// Div sets dest to the quotient x/y for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
// Div implements Euclidean division (unlike Go); see DivMod for more details.
func (c *BigIntContainer) Div(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Div, dest, x, y)
}

// Mod sets dest to the modulus x%y for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
// Mod implements Euclidean modulus (unlike Go); see DivMod for more details.
func (c *BigIntContainer) Mod(dest, x, y BigIntHandle) BigIntHandle {
	return c.performBinaryOperation((*big.Int).Mod, dest, x, y)
}

// Sqrt sets dest to ⌊√x⌋, the largest integer such that z² ≤ x.
// It panics if x is negative.
func (c *BigIntContainer) Sqrt(dest, x BigIntHandle) BigIntHandle {
	return c.performUnaryOperation((*big.Int).Sqrt, dest, x)
}
