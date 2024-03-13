package blockchain

import (
	"encoding/hex"
	"fmt"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools"
)

func LoadKey(pemFile string, skIndex int, converter core.PubkeyConverter, pwd string) ([]byte, []byte, string, error) {
	encodedSk, pkString, err := tools.LoadSkPkFromPemFile(pemFile, skIndex, pwd)
	if err != nil {
		return nil, nil, "", nil
	}

	skBytes, err := hex.DecodeString(string(encodedSk))
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w for encoded secret key", err)
	}

	pkBytes, err := converter.Decode(pkString)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w for encoded public key %s", err, pkString)
	}

	return skBytes, pkBytes, pkString, nil
}
