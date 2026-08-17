package factory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/stretchr/testify/require"
)

const corruptPem = "this is not a pem file at all\n"

func newLoaderFor(t *testing.T, pemPath string) *cryptoSigningParamsLoader {
	t.Helper()
	cspf, err := NewCryptoSigningParamsLoader(
		&mock.PubkeyConverterStub{},
		0,
		pemPath,
		&mock.SuiteStub{CreateKeyPairStub: createKeyPair},
		false,
	)
	require.NoError(t, err)
	return cspf
}

// SAFETY: an existing-but-unloadable pem (corrupt file, wrong KEY_PASSWORD,
// bad permissions) must NEVER be replaced by a freshly generated key. Doing so
// would destroy a validator's key material irrecoverably.
func TestKeyFileSafety_CorruptPemIsNeverOverwritten(t *testing.T) {
	dir := t.TempDir()
	pem := filepath.Join(dir, "validatorKey.pem")
	require.NoError(t, os.WriteFile(pem, []byte(corruptPem), 0o600))

	_, _, err := newLoaderFor(t, pem).getSkPk()
	require.Error(t, err, "an unloadable pem must surface an error, not be swallowed")
	require.ErrorContains(t, err, "loading validator key",
		"the error must name what failed, not surface as a bare deserialize error")

	after, rerr := os.ReadFile(pem)
	require.NoError(t, rerr)
	require.Equal(t, corruptPem, string(after),
		"CATASTROPHIC: an unloadable pem was overwritten with a generated key")
}

// Regression: the not-found check must match by error TYPE, not by searching the
// message for "no such file or directory". That text appears in every PEM error
// whose path happens to contain it, which sent a corrupt key file down the
// generate-and-replace branch and destroyed it.
func TestKeyFileSafety_NotFoundIsMatchedByTypeNotMessage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no such file or directory")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	pem := filepath.Join(dir, "validatorKey.pem")
	require.NoError(t, os.WriteFile(pem, []byte(corruptPem), 0o600))

	_, _, err := newLoaderFor(t, pem).getSkPk()
	require.Error(t, err, "a corrupt pem must fail even when its path contains the not-found text")

	after, rerr := os.ReadFile(pem)
	require.NoError(t, rerr)
	require.Equal(t, corruptPem, string(after),
		"CATASTROPHIC: a corrupt pem on a path containing the not-found text was replaced")
}

// The intentional observer path: a genuinely absent pem DOES create a key file.
func TestKeyFileSafety_MissingPemDoesCreateKey(t *testing.T) {
	dir := t.TempDir()
	pem := filepath.Join(dir, "validatorKey.pem")

	_, _, err := newLoaderFor(t, pem).getSkPk()
	require.NoError(t, err, "observer auto-generation must keep working")

	info, serr := os.Stat(pem)
	require.NoError(t, serr, "a key file should have been created")
	require.Greater(t, info.Size(), int64(0))
	t.Logf("missing pem -> created %s (%d bytes)", pem, info.Size())
}
