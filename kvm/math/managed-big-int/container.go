package managedbigint

import "math/big"

// BigIntContainer is a structure holding the data for big.Int numbers without pointers.
type BigIntContainer struct {
	data []big.Word

	register1   *big.Int
	register2   *big.Int
	destination *big.Int
}

// NewBigIntContainer constructs a new BigIntContainer.
func NewBigIntContainer() *BigIntContainer {
	return &BigIntContainer{
		data:        nil,
		register1:   big.NewInt(0),
		register2:   big.NewInt(0),
		destination: big.NewInt(0),
	}
}

// BigIntHandle acts like a pointer to a big.Int value in a BigIntContainer.
type BigIntHandle struct {
	start    int
	length   int
	capacity int
	negative bool
}

// Zero is a reference to Zero value. Zero doesn't need storage.
var Zero = BigIntHandle{
	start:    0,
	length:   0,
	capacity: 0,
	negative: false,
}
