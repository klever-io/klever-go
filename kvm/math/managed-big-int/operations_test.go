package managedbigint

import (
	"math/big"
	"math/bits"
	"testing"
)

const digitMask = uint64((1 << bits.UintSize) - 1)

var testCases = [][]uint64{
	{0, 0},
	{0, 1},
	{1, 0},
	{5, 5},
	{0, digitMask},
	{digitMask, 0},
	{1, digitMask},
	{digitMask, 1},
	{digitMask, digitMask},
}

func TestInsertExtract(t *testing.T) {
	c := NewBigIntContainer()

	for _, value := range []int64{0, 1, -1, -10, 200} {
		bi := big.NewInt(value)
		x := c.Insert(bi)
		check := c.Get(x)
		if bi.Cmp(check) != 0 {
			t.Errorf("TestInsertExtract failed. Want: %d, got %d", bi, check)
		}
	}
}

func TestAddSub(t *testing.T) {
	c := NewBigIntContainer()

	z := c.Insert(big.NewInt(0))

	for _, testCase := range testCases {
		x := c.Insert(big.NewInt(0).SetUint64(testCase[0]))
		y := c.Insert(big.NewInt(0).SetUint64(testCase[1]))
		z = c.Add(z, x, y)
		z = c.Sub(z, z, y)

		sum := c.Get(z)
		if big.NewInt(0).SetUint64(testCase[0]).Cmp(sum) != 0 {
			t.Errorf("bad result. Want: %d, got %d", testCase[0], sum)
		}
	}
}

func TestSubAdd(t *testing.T) {
	c := NewBigIntContainer()

	z := c.Insert(big.NewInt(0))

	for _, testCase := range testCases {
		x := c.Insert(big.NewInt(0).SetUint64(testCase[0]))
		y := c.Insert(big.NewInt(0).SetUint64(testCase[1]))
		z = c.Sub(z, x, y)
		z = c.Add(z, z, y)

		sum := c.Get(z)
		if big.NewInt(0).SetUint64(testCase[0]).Cmp(sum) != 0 {
			t.Errorf("bad result. Want: %d, got %d", testCase[0], sum)
		}
	}
}
