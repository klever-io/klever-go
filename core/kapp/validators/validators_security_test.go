package validators

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realBLSValidator returns the production BLS12-381 G2 key validator.
func realBLSValidator() blsPublicKeyValidator {
	return signing.NewKeyGenerator(mcl.NewSuiteBLS12())
}

// validBLSPublicKey generates a well-formed BLS12-381 G2 public key.
func validBLSPublicKey(t *testing.T) []byte {
	t.Helper()

	keyGen := signing.NewKeyGenerator(mcl.NewSuiteBLS12())
	_, pub := keyGen.GeneratePair()
	pkBytes, err := pub.ToByteArray()
	require.NoError(t, err)
	require.NotEmpty(t, pkBytes)

	return pkBytes
}

func setFixAuditChangesV3(v *validatorsKApp, enabled bool) {
	v.forkController.(*mock.ForkControllerStub).FixAuditChangesV3Value = enabled
}

func newRegisterContract(owner, blsPubKey []byte) *transaction.CreateValidatorContract {
	return &transaction.CreateValidatorContract{
		OwnerAddress: owner,
		Config: &transaction.ValidatorConfig{
			RewardAddress:       owner,
			Commission:          1000,
			MaxDelegationAmount: 1000000,
			BLSPublicKey:        blsPubKey,
		},
	}
}

// TestNewValidatorKApp_DefaultsBLSKeyValidator proves that production (which does
// not inject a validator) gets the real BLS12-381 G2 validator by default, so the
// runtime registration path is actually protected.
func TestNewValidatorKApp_DefaultsBLSKeyValidator(t *testing.T) {
	args := createMockArgs()
	args.BLSKeyValidator = nil

	v, err := NewValidatorKApp(args)
	require.NoError(t, err)
	require.NotNil(t, v.blsKeyValidator)

	// the default validator must reject an arbitrary (non-G2) key
	require.Error(t, v.blsKeyValidator.CheckPublicKeyValid([]byte("not-a-bls-key")))
	// and accept a well-formed one
	require.NoError(t, v.blsKeyValidator.CheckPublicKeyValid(validBLSPublicKey(t)))
}

// TestRegister_BLSKeyValidation_GHSA_9wh6_9hq7_9688 covers the consensus-liveness
// DoS fix: runtime validator registration must reject BLS keys that are not valid
// G2 points once the FixAuditChangesV3 fork is active.
func TestRegister_BLSKeyValidation_GHSA_9wh6_9hq7_9688(t *testing.T) {
	t.Parallel()

	owner := makeAddress("owner")

	t.Run("rejects arbitrary (non-96-byte) key post-fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, true)

		resultCode, err := v.Register(newRegisterContract(owner, []byte("blspubkey")))

		assert.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		assert.Equal(t, common.ErrInvalidBLSPublicKey, err)
	})

	t.Run("rejects correct-length but malformed key post-fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, true)

		// same length as a real key, but not a valid curve point
		malformed := bytes.Repeat([]byte{0xFF}, len(validBLSPublicKey(t)))

		resultCode, err := v.Register(newRegisterContract(owner, malformed))

		assert.Equal(t, transaction.Transaction_ParameterInvalid, resultCode)
		assert.Equal(t, common.ErrInvalidBLSPublicKey, err)
	})

	t.Run("accepts valid key post-fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, true)

		resultCode, err := v.Register(newRegisterContract(owner, validBLSPublicKey(t)))

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.NoError(t, err)
	})

	t.Run("accepts arbitrary key pre-fork (reprocessing determinism)", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, false)

		resultCode, err := v.Register(newRegisterContract(owner, []byte("blspubkey")))

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.NoError(t, err)
	})
}

// TestUpdateValidator_BLSKeyValidation covers the config-update path, which also
// writes an unchecked BLS key before the fix.
func TestUpdateValidator_BLSKeyValidation(t *testing.T) {
	t.Parallel()

	owner := makeAddress("owner")

	t.Run("rejects switching to an invalid key post-fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, true)

		// register with a valid key first
		resultCode, err := v.Register(newRegisterContract(owner, validBLSPublicKey(t)))
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		// try to update to an invalid key
		update := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: []byte("newblspubkey"),
			},
		}

		resultCode, err = v.UpdateValidator(owner, update)

		assert.Equal(t, transaction.Transaction_InvalidPeerKey, resultCode)
		assert.Equal(t, common.ErrInvalidBLSPublicKey, err)
	})

	t.Run("accepts switching to a valid key post-fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.blsKeyValidator = realBLSValidator()
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)
		setFixAuditChangesV3(v, true)

		resultCode, err := v.Register(newRegisterContract(owner, validBLSPublicKey(t)))
		require.Equal(t, transaction.Transaction_Ok, resultCode)
		require.NoError(t, err)

		update := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				BLSPublicKey: validBLSPublicKey(t),
			},
		}

		resultCode, err = v.UpdateValidator(owner, update)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.NoError(t, err)
	})
}
