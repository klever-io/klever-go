package process_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type permissionSigner struct {
	key     crypto.PrivateKey
	address []byte
}

func newPermissionSigner(t *testing.T, seed byte) permissionSigner {
	keyGen := signing.NewKeyGenerator(ed25519.NewEd25519())
	key, err := keyGen.PrivateKeyFromByteArray(bytes.Repeat([]byte{seed}, 32))
	require.Nil(t, err)
	address, err := key.GeneratePublic().ToByteArray()
	require.Nil(t, err)

	return permissionSigner{key: key, address: address}
}

func signAll(t *testing.T, txHash []byte, signers ...permissionSigner) [][]byte {
	signatures := make([][]byte, 0, len(signers))
	for _, s := range signers {
		signature, err := (&singlesig.Ed25519Signer{}).Sign(s.key, txHash)
		require.Nil(t, err)
		signatures = append(signatures, signature)
	}

	return signatures
}

func TestLoadPermissionSignerKeys(t *testing.T) {
	t.Parallel()

	keyGen := signing.NewKeyGenerator(ed25519.NewEd25519())
	first := newPermissionSigner(t, 1)
	second := newPermissionSigner(t, 2)

	t.Run("one key per signer", func(t *testing.T) {
		t.Parallel()

		keys, err := process.LoadPermissionSignerKeys(keyGen, []*state.Key{
			{Address: first.address, Weight: 1},
			{Address: second.address, Weight: 1},
		})
		require.Nil(t, err)
		assert.Len(t, keys, 2)
		for _, s := range []permissionSigner{first, second} {
			require.Contains(t, keys, string(s.address))
			keyBytes, err := keys[string(s.address)].ToByteArray()
			require.Nil(t, err)
			assert.Equal(t, s.address, keyBytes)
		}
	})

	t.Run("invalid signer address returns the key generator error", func(t *testing.T) {
		t.Parallel()

		_, err := process.LoadPermissionSignerKeys(keyGen, []*state.Key{{Address: make([]byte, 30), Weight: 1}})
		assert.Equal(t, crypto.ErrInvalidParam, err)
	})
}

func TestVerifyPermissionSignatures(t *testing.T) {
	t.Parallel()

	keyGen := signing.NewKeyGenerator(ed25519.NewEd25519())
	singleSigner := &singlesig.Ed25519Signer{}
	txHash := []byte("tx hash")
	first := newPermissionSigner(t, 1)
	second := newPermissionSigner(t, 2)
	outsider := newPermissionSigner(t, 3)

	permission := &state.Permission{
		Type:      state.Permission_Owner,
		Threshold: 3,
		Signers: []*state.Key{
			{Address: first.address, Weight: 1},
			{Address: second.address, Weight: 2},
		},
	}

	verify := func(t *testing.T, signatures [][]byte) ([][]byte, int64, error) {
		keys, err := process.LoadPermissionSignerKeys(keyGen, permission.Signers)
		require.Nil(t, err)

		return process.VerifyPermissionSignatures(singleSigner, permission, keys, signatures, txHash)
	}

	t.Run("returns each signer followed by its weight, and the total weight", func(t *testing.T) {
		t.Parallel()

		signedBy, weight, err := verify(t, signAll(t, txHash, second, first))
		require.Nil(t, err)
		assert.Equal(t, int64(3), weight)
		assert.Equal(t, [][]byte{second.address, []byte("2"), first.address, []byte("1")}, signedBy)
	})

	t.Run("a signer counts once", func(t *testing.T) {
		t.Parallel()

		_, _, err := verify(t, signAll(t, txHash, second, second))
		assert.Equal(t, common.ErrInvalidSignature, err)
	})

	t.Run("a signature from outside the permission", func(t *testing.T) {
		t.Parallel()

		_, _, err := verify(t, signAll(t, txHash, first, outsider))
		assert.Equal(t, common.ErrInvalidSignature, err)
	})

	t.Run("a signature over another hash", func(t *testing.T) {
		t.Parallel()

		_, _, err := verify(t, signAll(t, []byte("other hash"), first))
		assert.Equal(t, common.ErrInvalidSignature, err)
	})

	t.Run("consumes the keys of the signers that signed", func(t *testing.T) {
		t.Parallel()

		keys, err := process.LoadPermissionSignerKeys(keyGen, permission.Signers)
		require.Nil(t, err)

		_, _, err = process.VerifyPermissionSignatures(singleSigner, permission, keys, signAll(t, txHash, first), txHash)
		require.Nil(t, err)
		assert.NotContains(t, keys, string(first.address))
		assert.Contains(t, keys, string(second.address))

		// the same map can't count first again
		_, _, err = process.VerifyPermissionSignatures(singleSigner, permission, keys, signAll(t, txHash, first), txHash)
		assert.Equal(t, common.ErrInvalidSignature, err)
	})

	t.Run("weight below the threshold is not an error here", func(t *testing.T) {
		t.Parallel()

		signedBy, weight, err := verify(t, signAll(t, txHash, first))
		require.Nil(t, err)
		assert.Equal(t, int64(1), weight)
		assert.Equal(t, [][]byte{first.address, []byte("1")}, signedBy)
	})

	t.Run("no signatures weigh nothing and are not an error here", func(t *testing.T) {
		t.Parallel()

		signedBy, weight, err := verify(t, nil)
		require.Nil(t, err)
		assert.Equal(t, int64(0), weight)
		assert.Empty(t, signedBy)
	})
}

func TestCheckPermissionThreshold(t *testing.T) {
	t.Parallel()

	permission := &state.Permission{Threshold: 2}

	assert.Nil(t, process.CheckPermissionThreshold(permission, 2))
	assert.Nil(t, process.CheckPermissionThreshold(permission, 3))

	err := process.CheckPermissionThreshold(permission, 1)
	assert.True(t, errors.Is(err, common.ErrSignatureThreshold))
	assert.Equal(t, fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, 1, 2), err)

	// a threshold of 0 was accepted before FixAuditChangesV3, so such permissions still exist
	assert.Nil(t, process.CheckPermissionThreshold(&state.Permission{Threshold: 0}, 0))
	assert.Equal(t,
		fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, 0, 1),
		process.CheckPermissionThreshold(&state.Permission{Threshold: 1}, 0),
	)
}

type operationValidatorStub struct {
	called bool
	err    error
}

func (s *operationValidatorStub) ValidatePermissionOperation(_ []byte) error {
	s.called = true
	return s.err
}

func TestCheckPermissionOperation(t *testing.T) {
	t.Parallel()

	errNotAllowed := errors.New("not allowed")

	t.Run("an owner permission allows every operation", func(t *testing.T) {
		t.Parallel()

		tx := &operationValidatorStub{err: errNotAllowed}
		err := process.CheckPermissionOperation(tx, &state.Permission{Type: state.Permission_Owner})
		assert.Nil(t, err)
		assert.False(t, tx.called)
	})

	t.Run("a user permission returns the transaction's verdict", func(t *testing.T) {
		t.Parallel()

		tx := &operationValidatorStub{err: errNotAllowed}
		err := process.CheckPermissionOperation(tx, &state.Permission{Type: state.Permission_User})
		assert.Equal(t, errNotAllowed, err)
		assert.True(t, tx.called)
	})
}
