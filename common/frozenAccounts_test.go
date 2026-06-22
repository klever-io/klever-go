package common

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsAccountFrozen(t *testing.T) {
	t.Run("frozen account is detected", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			addr, err := hex.DecodeString(hexAddr)
			require.NoError(t, err)
			require.True(t, IsAccountFrozen(addr), "expected %s to be frozen", hexAddr)
		}
	})

	t.Run("unrelated account is not frozen", func(t *testing.T) {
		addr, _ := hex.DecodeString("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		require.False(t, IsAccountFrozen(addr))
	})

	t.Run("empty and nil are not frozen", func(t *testing.T) {
		require.False(t, IsAccountFrozen(nil))
		require.False(t, IsAccountFrozen([]byte{}))
	})
}
