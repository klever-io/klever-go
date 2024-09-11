package kdaconvert

import (
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

// kdaTokenKeyPrefix is the prefix of storage keys belonging to KDA tokens.
var kdaTokenKeyPrefix = []byte(kapps.ProtectedKleverKeyPrefix + core.KDAKeyIdentifier)

// kdaDataMarshalizer is the global marshalizer to be used for encoding/decoding KDA data
var kdaDataMarshalizer = marshal.NewProtoMarshalizer()

// makeTokenKey creates the storage key corresponding to the given tokenName.
func makeTokenKey(tokenName []byte, nonce uint64) []byte {
	nonceBytes := big.NewInt(0).SetUint64(nonce).Bytes()
	tokenKey := append(kdaTokenKeyPrefix, tokenName...)
	tokenKey = append(tokenKey, nonceBytes...)
	return tokenKey
}
