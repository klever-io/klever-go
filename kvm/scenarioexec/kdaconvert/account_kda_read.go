package kdaconvert

import (
	"github.com/klever-io/klever-go/data/dkda"
)

// MockKDAData groups together all instances of a token (same token name, different nonces).
type MockKDAData struct {
	TokenIdentifier []byte
	Instances       []*dkda.KDigitalToken
	LastNonce       uint64
	Roles           [][]byte
}

// GetTokenBalance returns the KDA balance of the account, specified by the
// token key.
func GetTokenBalance(tokenIdentifier []byte, nonce uint64, source map[string][]byte) (int64, error) {
	tokenData, err := GetTokenData(tokenIdentifier, nonce, source, make(map[string][]byte))
	if err != nil {
		return 0, err
	}

	return tokenData.Value, nil
}

// GetTokenData gets the KDA information related to a token from the storage of the account.
func GetTokenData(tokenIdentifier []byte, nonce uint64, source map[string][]byte, systemAccStorage map[string][]byte) (*dkda.KDigitalToken, error) {
	tokenKey := makeTokenKey(tokenIdentifier, nonce)
	return getTokenDataByKey(tokenKey, source, systemAccStorage)
}

func getTokenDataByKey(tokenKey []byte, source map[string][]byte, systemAccStorage map[string][]byte) (*dkda.KDigitalToken, error) {
	// default value copied from the protocol
	kdaData := &dkda.KDigitalToken{
		Value: 0,
	}

	marshaledData := source[string(tokenKey)]
	if len(marshaledData) == 0 {
		return kdaData, nil
	}

	err := kdaDataMarshalizer.Unmarshal(kdaData, marshaledData)
	if err != nil {
		return nil, err
	}

	marshaledData = systemAccStorage[string(tokenKey)]
	if len(marshaledData) == 0 {
		return kdaData, nil
	}
	kdaDataFromSystemAcc := &dkda.KDigitalToken{}
	err = kdaDataMarshalizer.Unmarshal(kdaDataFromSystemAcc, marshaledData)
	if err != nil {
		return nil, err
	}

	kdaData.TokenMetaData = kdaDataFromSystemAcc.TokenMetaData

	return kdaData, nil
}
