package process

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSortedPermissionSigners_KeyOrder pins the order of a genesis permission's signers: both the state keys
// and the contract keys that go into the permission transaction's hash follow the sorted addresses on every
// run, not Go's random map order (KLC-2770).
func TestSortedPermissionSigners_KeyOrder(t *testing.T) {
	t.Parallel()

	const numSigners = 8
	signers := make(map[string]int64, numSigners)
	expected := make([][]byte, 0, numSigners)
	for i := numSigners; i > 0; i-- {
		addr := bytes.Repeat([]byte{byte(i * 0x11)}, 32)
		signers[string(addr)] = int64(i)
		expected = append(expected, addr)
	}
	slices.SortFunc(expected, bytes.Compare)

	for run := 0; run < 20; run++ {
		keys, contractSigners := sortedPermissionSigners(signers)
		require.Len(t, keys, numSigners)
		require.Len(t, contractSigners, numSigners)
		for i, addr := range expected {
			require.Equal(t, addr, keys[i].Address, "run %d key %d", run, i)
			require.Equal(t, signers[string(addr)], keys[i].Weight, "run %d key %d", run, i)
			require.Equal(t, addr, contractSigners[i].Address, "run %d signer %d", run, i)
			require.Equal(t, signers[string(addr)], contractSigners[i].Weight, "run %d signer %d", run, i)
		}
	}
}

func TestSortedPermissionSigners_Empty(t *testing.T) {
	t.Parallel()

	keys, contractSigners := sortedPermissionSigners(nil)
	require.Empty(t, keys)
	require.Empty(t, contractSigners)
}
