package rand_test

import (
	"testing"

	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p/rand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLegacySeedRandReader_NilSeedShouldErr(t *testing.T) {
	t.Parallel()

	srr, err := rand.NewLegacySeedRandReader(nil)

	assert.Nil(t, srr)
	assert.Equal(t, p2p.ErrEmptySeed, err)
}

func TestNewLegacySeedRandReader_EmptySeedShouldErr(t *testing.T) {
	t.Parallel()

	srr, err := rand.NewLegacySeedRandReader([]byte{})

	assert.Nil(t, srr)
	assert.Equal(t, p2p.ErrEmptySeed, err)
}

func TestNewLegacySeedRandReader_ShouldWork(t *testing.T) {
	t.Parallel()

	seed := []byte("seed")
	srr, err := rand.NewLegacySeedRandReader(seed)

	assert.NotNil(t, srr)
	assert.Nil(t, err)
}

func TestLegacySeedRandReader_ReadEmptyBufferShouldReturnZero(t *testing.T) {
	t.Parallel()

	seed := []byte("seed")
	srr, _ := rand.NewLegacySeedRandReader(seed)

	n, err := srr.Read(nil)

	assert.Equal(t, 0, n)
	assert.Nil(t, err)
}

// TestLegacySeedRandReader_ReadShouldMatchGo118 verifies that the frozen PRNG
// produces output identical to Go 1.18's math/rand.New(rand.NewSource(seed)).Read().
// These reference vectors were generated using Go 1.18's implementation with
// seed "seed" (SHA-256 hashed, first 8 bytes as big-endian int64).
func TestLegacySeedRandReader_ReadShouldMatchGo118(t *testing.T) {
	t.Parallel()

	seed := []byte("seed")
	srr, _ := rand.NewLegacySeedRandReader(seed)

	testTbl := []struct {
		pSize int
		p     []byte
		name  string
	}{
		{pSize: 1, p: []byte{15}, name: "1 byte"},
		{pSize: 2, p: []byte{15, 210}, name: "2 bytes"},
		{pSize: 4, p: []byte{15, 210, 236, 97}, name: "4 bytes"},
		{pSize: 5, p: []byte{15, 210, 236, 97, 112}, name: "5 bytes"},
		{pSize: 7, p: []byte{15, 210, 236, 97, 112, 165, 91}, name: "7 bytes (full int63)"},
		{pSize: 8, p: []byte{15, 210, 236, 97, 112, 165, 91, 186}, name: "8 bytes (crosses int63 boundary)"},
		{pSize: 40, p: []byte{
			15, 210, 236, 97, 112, 165, 91, 186,
			90, 248, 217, 41, 162, 62, 20, 141,
			2, 28, 64, 34, 226, 55, 172, 177,
			140, 185, 51, 59, 176, 63, 203, 134,
			242, 200, 225, 59, 139, 84, 188, 74,
		}, name: "40 bytes (ecdsa key derivation size)"},
	}

	for _, tc := range testTbl {
		t.Run(tc.name, func(t *testing.T) {
			p := make([]byte, tc.pSize)

			n, err := srr.Read(p)

			assert.Equal(t, tc.p, p)
			assert.Equal(t, tc.pSize, n)
			assert.Nil(t, err)
		})
	}
}

func TestLegacySeedRandReader_ReadIsIdempotent(t *testing.T) {
	t.Parallel()

	seed := []byte("seed")
	srr, _ := rand.NewLegacySeedRandReader(seed)

	buf1 := make([]byte, 40)
	buf2 := make([]byte, 40)

	n1, err1 := srr.Read(buf1)
	n2, err2 := srr.Read(buf2)

	require.Nil(t, err1)
	require.Nil(t, err2)
	assert.Equal(t, 40, n1)
	assert.Equal(t, 40, n2)
	assert.Equal(t, buf1, buf2, "Consecutive reads from the same reader must produce identical output (stateless)")
}

func TestLegacySeedRandReader_DifferentSeedsShouldDiffer(t *testing.T) {
	t.Parallel()

	srr1, _ := rand.NewLegacySeedRandReader([]byte("seed"))
	srr2, _ := rand.NewLegacySeedRandReader([]byte("test"))

	buf1 := make([]byte, 40)
	buf2 := make([]byte, 40)

	srr1.Read(buf1)
	srr2.Read(buf2)

	assert.NotEqual(t, buf1, buf2, "Different seeds must produce different output")
}
