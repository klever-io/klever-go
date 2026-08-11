package common

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// changeNowAccount is the third-party swap-service account released at the
// FixAuditChangesV4 fork; every other entry stays frozen indefinitely.
const changeNowAccount = "bb687dbba23e1844fec674a32cb8809f0d3207506c53fc3d637e40dc56708d63"

// forkStatusStub is declared here rather than reusing common/mock, which imports
// common and so cannot be imported back from an in-package test.
type forkStatusStub struct {
	fixMarketBuyOverflow bool
	fixAuditChangesV4    bool
}

func (s forkStatusStub) FixMarketBuyOverflow() bool { return s.fixMarketBuyOverflow }
func (s forkStatusStub) FixAuditChangesV4() bool    { return s.fixAuditChangesV4 }

var (
	beforeFreeze = forkStatusStub{}
	afterFreeze  = forkStatusStub{fixMarketBuyOverflow: true}
	afterAuditV4 = forkStatusStub{fixMarketBuyOverflow: true, fixAuditChangesV4: true}
)

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()

	raw, err := hex.DecodeString(s)
	require.NoError(t, err)

	return raw
}

func Test_IsAccountFrozen(t *testing.T) {
	t.Run("frozen account is detected", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			addr := mustDecodeHex(t, hexAddr)
			require.True(t, IsAccountFrozen(addr, afterFreeze), "expected %s to be frozen", hexAddr)
		}
	})

	t.Run("no account is frozen before the freeze fork activates", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			addr := mustDecodeHex(t, hexAddr)
			require.False(t, IsAccountFrozen(addr, beforeFreeze), "expected %s to be unfrozen", hexAddr)
		}
	})

	t.Run("unrelated account is not frozen", func(t *testing.T) {
		addr := mustDecodeHex(t, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		require.False(t, IsAccountFrozen(addr, afterFreeze))
		require.False(t, IsAccountFrozen(addr, afterAuditV4))
	})

	t.Run("empty and nil are not frozen", func(t *testing.T) {
		require.False(t, IsAccountFrozen(nil, afterFreeze))
		require.False(t, IsAccountFrozen([]byte{}, afterFreeze))
	})
}

func Test_IsAccountFrozenAtAPI(t *testing.T) {
	t.Run("listed accounts are refused before the freeze fork activates", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			addr := mustDecodeHex(t, hexAddr)
			require.True(t, IsAccountFrozenAtAPI(addr, beforeFreeze), "expected %s to be refused", hexAddr)
		}
	})

	t.Run("thawed account is accepted from the audit v4 fork", func(t *testing.T) {
		addr := mustDecodeHex(t, changeNowAccount)

		require.True(t, IsAccountFrozenAtAPI(addr, afterFreeze))
		require.False(t, IsAccountFrozenAtAPI(addr, afterAuditV4))
	})

	t.Run("every other account stays refused across the audit v4 fork", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			if hexAddr == changeNowAccount {
				continue
			}

			addr := mustDecodeHex(t, hexAddr)
			require.True(t, IsAccountFrozenAtAPI(addr, afterAuditV4), "expected %s to stay refused", hexAddr)
		}
	})

	t.Run("unrelated, empty and nil are not refused", func(t *testing.T) {
		addr := mustDecodeHex(t, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		require.False(t, IsAccountFrozenAtAPI(addr, beforeFreeze))
		require.False(t, IsAccountFrozenAtAPI(nil, beforeFreeze))
		require.False(t, IsAccountFrozenAtAPI([]byte{}, beforeFreeze))
	})
}

func Test_IsAccountFrozen_AuditV4Thaw(t *testing.T) {
	t.Run("swap service account is unfrozen from the audit v4 fork", func(t *testing.T) {
		addr := mustDecodeHex(t, changeNowAccount)

		require.True(t, IsAccountFrozen(addr, afterFreeze), "expected freeze before audit v4")
		require.False(t, IsAccountFrozen(addr, afterAuditV4), "expected thaw at audit v4")
	})

	t.Run("every other account stays frozen across the audit v4 fork", func(t *testing.T) {
		for hexAddr := range frozenAccounts {
			if hexAddr == changeNowAccount {
				continue
			}

			addr := mustDecodeHex(t, hexAddr)
			require.True(t, IsAccountFrozen(addr, afterAuditV4), "expected %s to stay frozen", hexAddr)
		}
	})

	t.Run("exactly one account carries a thaw window", func(t *testing.T) {
		thawed := make([]string, 0, 1)
		for hexAddr, window := range frozenAccounts {
			if window.thawedFrom != nil {
				thawed = append(thawed, hexAddr)
			}
		}

		require.Equal(t, []string{changeNowAccount}, thawed)
	})
}
