package multisig

import (
	"github.com/klever-io/klever-go/crypto"
)

func convertStringsToPubKeys(pubKeys []string, kg crypto.KeyGenerator) ([]crypto.PublicKey, error) {
	pk := make([]crypto.PublicKey, 0, len(pubKeys))

	//convert pubKeys
	for _, pubKeyStr := range pubKeys {
		if pubKeyStr == "" {
			return nil, crypto.ErrEmptyPubKeyString
		}

		pubKey, err := kg.PublicKeyFromByteArray([]byte(pubKeyStr))
		if err != nil {
			return nil, crypto.ErrInvalidPublicKeyString
		}

		pk = append(pk, pubKey)
	}
	return pk, nil
}
