package kdaconvert

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

// kdaTokenKeyPrefix is the prefix of storage keys belonging to KDA tokens.
var kdaTokenKeyPrefix = []byte(kapps.ProtectedKleverKeyPrefix + kapps.ProtectedKLVKeyPrefix + kapps.ProtectedKFIKeyPrefix)

// kdaRoleKeyPrefix is the prefix of storage keys belonging to KDA roles.
var kdaRoleKeyPrefix = []byte(kapps.ProtectedKleverKeyPrefix)

// kdaNonceKeyPrefix is the prefix of storage keys belonging to KDA nonces.
var kdaNonceKeyPrefix = []byte(kapps.ProtectedKleverKeyPrefix)

// kdaDataMarshalizer is the global marshalizer to be used for encoding/decoding KDA data
var kdaDataMarshalizer = marshal.NewProtoMarshalizer()

// errNegativeValue signals that a negative value has been detected and it is not allowed
var errNegativeValue = errors.New("negative value")

// makeTokenKey creates the storage key corresponding to the given tokenName.
func makeTokenKey(tokenName []byte, nonce uint64) []byte {
	nonceBytes := big.NewInt(0).SetUint64(nonce).Bytes()
	tokenKey := append(kdaTokenKeyPrefix, tokenName...)
	tokenKey = append(tokenKey, nonceBytes...)
	return tokenKey
}

// makeTokenRolesKey creates the storage key corresponding to the roles for the
// given tokenName.
func makeTokenRolesKey(tokenName []byte) []byte {
	tokenRolesKey := append(kdaRoleKeyPrefix, tokenName...)
	return tokenRolesKey
}

// makeLastNonceKey creates the storage key corresponding to the last nonce of
// the given tokenName.
func makeLastNonceKey(tokenName []byte) []byte {
	tokenNonceKey := append(kdaNonceKeyPrefix, tokenName...)
	return tokenNonceKey
}

// isTokenKey returns true if the given storage key belongs to an KDA token.
func isTokenKey(key []byte) bool {
	return bytes.HasPrefix(key, kdaTokenKeyPrefix)
}

// isRoleKey returns true if the given storage key belongs to an KDA role.
func isRoleKey(key []byte) bool {
	return bytes.HasPrefix(key, kdaRoleKeyPrefix)
}

// isNonceKey returns true if the given storage key belongs to an KDA nonce.
func isNonceKey(key []byte) bool {
	return bytes.HasPrefix(key, kdaNonceKeyPrefix)
}

// getTokenNameFromKey extracts the token name from the given storage key; it
// does not check whether the key is indeed a token key or not.
func getTokenNameFromKey(key []byte) []byte {
	return key[len(kdaTokenKeyPrefix):]
}
