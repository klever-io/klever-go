package kdaconvert

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/klever-io/klever-go/data/dkda"
)

// MockKDAData groups together all instances of a token (same token name, different nonces).
type MockKDAData struct {
	TokenIdentifier []byte
	Instances       []*dkda.KDigitalToken
	LastNonce       uint64
	Roles           [][]byte
}

const (
	kdaIdentifierSeparator  = "-"
	kdaRandomSequenceLength = 6
)

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

func extractTokenIdentifierAndNonceKDAWipe(args []byte) ([]byte, uint64) {
	argsSplit := bytes.Split(args, []byte(kdaIdentifierSeparator))
	if len(argsSplit) < 2 {
		return args, 0
	}

	if len(argsSplit[1]) <= kdaRandomSequenceLength {
		return args, 0
	}

	identifier := []byte(fmt.Sprintf("%s-%s", argsSplit[0], argsSplit[1][:kdaRandomSequenceLength]))
	nonce := big.NewInt(0).SetBytes(argsSplit[1][kdaRandomSequenceLength:])

	return identifier, nonce.Uint64()
}

// loads and prepared the KDA instance
func loadMockKDADataInstance(tokenKey []byte, source map[string][]byte, systemAccStorage map[string][]byte) (string, *dkda.KDigitalToken, error) {
	tokenInstance, err := getTokenDataByKey(tokenKey, source, systemAccStorage)
	if err != nil {
		return "", nil, err
	}

	tokenNameFromKey := getTokenNameFromKey(tokenKey)
	tokenName, nonce := extractTokenIdentifierAndNonceKDAWipe(tokenNameFromKey)

	if tokenInstance.TokenMetaData == nil {
		tokenInstance.TokenMetaData = &dkda.MetaData{
			Name:  tokenName,
			Nonce: nonce,
		}
	}

	return string(tokenName), tokenInstance, nil
}

func getOrCreateMockKDAData(tokenName string, resultMap map[string]*MockKDAData) *MockKDAData {
	resultObj := resultMap[tokenName]
	if resultObj == nil {
		resultObj = &MockKDAData{
			TokenIdentifier: []byte(tokenName),
			Instances:       nil,
			LastNonce:       0,
			Roles:           nil,
		}
		resultMap[tokenName] = resultObj
	}
	return resultObj
}
