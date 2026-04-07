package libp2p_test

import (
	"testing"

	libp2p "github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateP2PPrivKey_LegacySeed_ProducesStablePeerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		seed           string
		legacySeed     bool
		expectedPeerID string
	}{
		{
			name:           "seed=test, legacy=true (Go 1.18 stable)",
			seed:           "test",
			legacySeed:     true,
			expectedPeerID: "16Uiu2HAkyxfGrYUS9XUKxdu5mopdGiNTSqRtKMoZfmPbzFXiN6BH",
		},
		{
			name:           "seed=test, legacy=false (new mode)",
			seed:           "test",
			legacySeed:     false,
			expectedPeerID: "16Uiu2HAmKJJm8aAWVeNk7rKkKys3nH3pbpZjTeqcDeAmcDwpukBY",
		},
		{
			name:           "seed=seed, legacy=true (Go 1.18 stable)",
			seed:           "seed",
			legacySeed:     true,
			expectedPeerID: "16Uiu2HAkw5SNNtSvH1zJiQ6Gc3WoGNSxiyNueRKe6fuAuh57G3Bk",
		},
		{
			name:           "seed=seed, legacy=false (new mode)",
			seed:           "seed",
			legacySeed:     false,
			expectedPeerID: "16Uiu2HAmLnwsbHvKiBrxKF1d4Zr1KCqfQ115C4FmarTuvsVWubAz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			privKey, err := libp2p.CreateP2PPrivKey(tc.seed, tc.legacySeed)
			require.NoError(t, err)
			require.NotNil(t, privKey)

			peerID, err := peer.IDFromPrivateKey(privKey)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedPeerID, peerID.String())
		})
	}
}

func TestCreateP2PPrivKey_LegacySeed_Idempotent(t *testing.T) {
	t.Parallel()

	// Calling twice with the same seed must produce the same key.
	privKey1, err := libp2p.CreateP2PPrivKey("test", true)
	require.NoError(t, err)

	privKey2, err := libp2p.CreateP2PPrivKey("test", true)
	require.NoError(t, err)

	raw1, err := privKey1.Raw()
	require.NoError(t, err)

	raw2, err := privKey2.Raw()
	require.NoError(t, err)

	assert.Equal(t, raw1, raw2, "Same seed must produce identical private keys")
}

func TestCreateP2PPrivKey_DifferentSeeds_DifferentKeys(t *testing.T) {
	t.Parallel()

	privKey1, err := libp2p.CreateP2PPrivKey("seed1", true)
	require.NoError(t, err)

	privKey2, err := libp2p.CreateP2PPrivKey("seed2", true)
	require.NoError(t, err)

	raw1, err := privKey1.Raw()
	require.NoError(t, err)

	raw2, err := privKey2.Raw()
	require.NoError(t, err)

	assert.NotEqual(t, raw1, raw2, "Different seeds must produce different keys")
}

func TestCreateP2PPrivKey_EmptySeed_NoError(t *testing.T) {
	t.Parallel()

	// Empty seed should use crypto/rand (non-deterministic) and not error.
	privKey, err := libp2p.CreateP2PPrivKey("", false)
	require.NoError(t, err)
	require.NotNil(t, privKey)
}

func TestCreateP2PPrivKey_EmptySeed_LegacySeed_NoError(t *testing.T) {
	t.Parallel()

	// Empty seed with legacySeed=true should also use crypto/rand and not error.
	privKey, err := libp2p.CreateP2PPrivKey("", true)
	require.NoError(t, err)
	require.NotNil(t, privKey)
}
