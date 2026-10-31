package process

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data/state"
)

// The checks below are shared by transaction admission (dataValidators.txValidator) and block
// processing (transaction.baseTxProcessor), so the two can't drift apart (KLR-74). They return
// bare errors; block processing adds its own context on top.

// CheckPermissionOperation returns an error if the permission does not allow the transaction's
// contracts. An owner permission allows every operation.
func CheckPermissionOperation(tx PermissionOperationValidator, permission *state.Permission) error {
	if permission.Type == state.Permission_Owner {
		return nil
	}

	return tx.ValidatePermissionOperation(permission.Operations)
}

// LoadPermissionSignerKeys returns the public key of every signer of a permission, by address
func LoadPermissionSignerKeys(keyGen crypto.KeyGenerator, signers []*state.Key) (map[string]crypto.PublicKey, error) {
	signerKeys := make(map[string]crypto.PublicKey, len(signers))
	for _, signer := range signers {
		publicKey, err := keyGen.PublicKeyFromByteArray(signer.Address)
		if err != nil {
			return nil, err
		}
		signerKeys[string(signer.Address)] = publicKey
	}

	return signerKeys, nil
}

// VerifyPermissionSignatures matches every signature to a signer of the permission, each signer
// counted once, and returns the signers that signed, each followed by its weight, and their total
// weight. It consumes signerKeys. A signature no remaining signer verifies is
// common.ErrInvalidSignature. The threshold is left to CheckPermissionThreshold.
func VerifyPermissionSignatures(
	singleSigner crypto.SingleSigner,
	permission *state.Permission,
	signerKeys map[string]crypto.PublicKey,
	signatures [][]byte,
	txHash []byte,
) ([][]byte, int64, error) {
	var signedBy [][]byte
	var signWeight int64

	for _, signature := range signatures {
		match := false
		for _, signer := range permission.Signers {
			publicKey, ok := signerKeys[string(signer.Address)]
			if !ok {
				continue
			}
			if singleSigner.Verify(publicKey, txHash, signature) == nil {
				signedBy = append(signedBy, signer.Address)
				signedBy = append(signedBy, []byte(fmt.Sprintf("%d", signer.Weight)))
				signWeight += signer.Weight
				match = true
				// a signer counts once
				delete(signerKeys, string(signer.Address))
				break
			}
		}
		if !match {
			return nil, 0, common.ErrInvalidSignature
		}
	}

	return signedBy, signWeight, nil
}

// CheckPermissionThreshold returns an error if the signed weight is below the permission's threshold
func CheckPermissionThreshold(permission *state.Permission, signWeight int64) error {
	if signWeight < permission.Threshold {
		return fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, signWeight, permission.Threshold)
	}

	return nil
}
