package txcache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_estimateTxFeeScore(t *testing.T) {
	A := createTxWithParams([]byte("a"), "a", 1, 200, 10_000_000_000)
	B := createTxWithParams([]byte("b"), "b", 1, 200, 100_000_000)
	C := createTxWithParams([]byte("C"), "c", 1, 200, 10_000_000)

	scoreA := estimateTxGas(A)
	scoreB := estimateTxGas(B)
	scoreC := estimateTxGas(C)
	require.Equal(t, int64(10_001_000_000), scoreA)
	require.Equal(t, int64(101_000_000), scoreB)
	require.Equal(t, int64(11_000_000), scoreC)
}
