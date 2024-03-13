package factory

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

type cryptoSigningParamsLoader struct {
	pubkeyConverter     core.PubkeyConverter
	skIndex             int
	skPemFileName       string
	suite               crypto.Suite
	skPkProviderHandler func() ([]byte, []byte, error)
	isInImportMode      bool
}

// NewCryptoSigningParamsLoader returns a new instance of cryptoSigningParamsLoader
func NewCryptoSigningParamsLoader(
	pubkeyConverter core.PubkeyConverter,
	skIndex int,
	skPemFileName string,
	suite crypto.Suite,
	isInImportMode bool,
) (*cryptoSigningParamsLoader, error) {
	if check.IfNil(pubkeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(suite) {
		return nil, crypto.ErrNilSuite
	}

	cspf := &cryptoSigningParamsLoader{
		pubkeyConverter: pubkeyConverter,
		skIndex:         skIndex,
		skPemFileName:   skPemFileName,
		suite:           suite,
		isInImportMode:  isInImportMode,
	}
	cspf.skPkProviderHandler = cspf.getSkPk

	return cspf, nil
}

// Get returns a key generator, a private key, and a public key
func (cspf *cryptoSigningParamsLoader) Get() (*CryptoParams, error) {
	cryptoParams := &CryptoParams{}
	cryptoParams.KeyGenerator = signing.NewKeyGenerator(cspf.suite)

	if cspf.isInImportMode {
		return cspf.generateCryptoParams(cryptoParams)
	}

	return cspf.readCryptoParams(cryptoParams)
}

func (cspf *cryptoSigningParamsLoader) readCryptoParams(cryptoParams *CryptoParams) (*CryptoParams, error) {
	sk, readPk, err := cspf.skPkProviderHandler()
	if err != nil {
		return nil, err
	}
	cryptoParams.PrivateKey, err = cryptoParams.KeyGenerator.PrivateKeyFromByteArray(sk)
	if err != nil {
		return nil, err
	}

	cryptoParams.PublicKey = cryptoParams.PrivateKey.GeneratePublic()
	if len(readPk) > 0 {
		cryptoParams.PublicKeyBytes, err = cryptoParams.PublicKey.ToByteArray()
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(cryptoParams.PublicKeyBytes, readPk) {
			return nil, ErrPublicKeyMismatch
		}
	}

	cryptoParams.PublicKeyString = cspf.pubkeyConverter.Encode(cryptoParams.PublicKeyBytes)

	return cryptoParams, nil
}

func (cspf *cryptoSigningParamsLoader) generateCryptoParams(cryptoParams *CryptoParams) (*CryptoParams, error) {
	log.Warn("the node is in import mode! Will generate a fresh new BLS key")
	cryptoParams.PrivateKey, cryptoParams.PublicKey = cryptoParams.KeyGenerator.GeneratePair()

	var err error
	cryptoParams.PublicKeyBytes, err = cryptoParams.PublicKey.ToByteArray()
	if err != nil {
		return nil, err
	}

	cryptoParams.PublicKeyString = cspf.pubkeyConverter.Encode(cryptoParams.PublicKeyBytes)

	return cryptoParams, nil
}

func (cspf *cryptoSigningParamsLoader) getSkPk() ([]byte, []byte, error) {
	skIndex := cspf.skIndex
	encodedSk, pkString, err := tools.LoadSkPkFromPemFile(cspf.skPemFileName, skIndex, os.Getenv("KEY_PASSWORD"))
	if err != nil {
		if strings.Contains(err.Error(), ErrFileNotFound.Error()) {
			keyGen := signing.NewKeyGenerator(cspf.suite)
			encodedSk, pkString, err = tools.CreateWallet(cspf.skPemFileName, os.Getenv("KEY_PASSWORD"), keyGen, cspf.pubkeyConverter)
			if err != nil {
				return nil, nil, err
			}
		}

	}

	skBytes, err := hex.DecodeString(string(encodedSk))
	if err != nil {
		return nil, nil, fmt.Errorf("%w for encoded secret key", err)
	}

	pkBytes, err := cspf.pubkeyConverter.Decode(pkString)
	if err != nil {
		return nil, nil, fmt.Errorf("%w for encoded public key %s", err, pkString)
	}

	return skBytes, pkBytes, nil
}
