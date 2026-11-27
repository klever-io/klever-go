package vmhooks

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// bigFloatIntPartTestPrec is the managed big float precision
const bigFloatIntPartTestPrec = 53

// bigFloatIntPartByteLen must be the byte length of the integer part op.Int gives, which is what
// BigFloatFloor, BigFloatCeil and BigFloatTruncate charged after converting before
// FixAuditChangesV6, for every float the managed types can hold: both signs, ±0, |op| < 1, exact
// integers, powers of two and their neighbours one ulp away, every exponent up to the bound
func TestBigFloatIntPartByteLen_MatchesConvertedLength(t *testing.T) {
	t.Parallel()

	ulp := new(big.Float).SetPrec(bigFloatIntPartTestPrec).SetMantExp(big.NewFloat(1), -bigFloatIntPartTestPrec)
	half := big.NewFloat(0.5)
	mantissas := []*big.Float{
		half,                                     // a power of two
		new(big.Float).Add(half, ulp),            // one ulp above it
		new(big.Float).Sub(big.NewFloat(1), ulp), // one ulp below the next power of two
		big.NewFloat(0.75),                       // fraction bits set for small exponents
		big.NewFloat(0.999),                      // mantissa with all low bits used
		new(big.Float).Add(big.NewFloat(0.625), ulp), // an odd mantissa
	}
	exponents := []int{
		bigFloatMinExponentForTest, -1000, -53, -2, -1, 0, 1, 2, 3, 7, 8, 9, 15, 16, 17, 52, 53, 54, 63, 64, 65,
		255, 256, 257, 1000, 60000, bigFloatMaxExponentForTest - 1, bigFloatMaxExponentForTest,
	}

	check := func(t *testing.T, op *big.Float) {
		t.Helper()
		intPart, _ := op.Int(nil)
		expected := int64((intPart.BitLen() + 7) / 8)
		require.Equal(t, expected, bigFloatIntPartByteLen(op), "op %.20g", op)
	}

	t.Run("zeros", func(t *testing.T) {
		check(t, big.NewFloat(0))
		check(t, new(big.Float).Neg(big.NewFloat(0)))
	})

	for _, mant := range mantissas {
		for _, exp := range exponents {
			for _, negative := range []bool{false, true} {
				m := new(big.Float).SetPrec(bigFloatIntPartTestPrec).Set(mant)
				if negative {
					m.Neg(m)
				}
				op := new(big.Float).SetMantExp(m, exp)
				t.Run(fmt.Sprintf("%s/exp=%d", m.Text('g', 17), exp), func(t *testing.T) {
					check(t, op)
				})
			}
		}
	}

	t.Run("exactIntegers", func(t *testing.T) {
		for _, v := range []int64{1, -1, 2, 7, 8, 255, 256, -256, 1 << 52, -(1 << 52), 1<<53 - 1} {
			check(t, new(big.Float).SetPrec(bigFloatIntPartTestPrec).SetInt64(v))
		}
	})

	t.Run("random", func(t *testing.T) {
		rng := rand.New(rand.NewSource(2755)) // #nosec G404
		for i := 0; i < 20_000; i++ {
			mant := big.NewFloat(0.5 + rng.Float64()/2)
			if rng.Intn(2) == 0 {
				mant.Neg(mant)
			}
			exp := rng.Intn(bigFloatMaxExponentForTest-bigFloatMinExponentForTest+1) + bigFloatMinExponentForTest
			if i%2 == 0 {
				exp = rng.Intn(200) - 100
			}
			check(t, new(big.Float).SetMantExp(mant, exp))
		}
	})
}

// the managed types' exponent bounds (contexts.bigFloatMaxExponent / bigFloatMinExponent)
const (
	bigFloatMaxExponentForTest = 65025
	bigFloatMinExponentForTest = -65025
)
