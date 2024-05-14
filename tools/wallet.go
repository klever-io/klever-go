package tools

import (
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/tools/check"
)

type key struct {
	skBytes []byte
	pkBytes []byte
}

func CreateWallet(walletPemFile string, pwd string, keyGen crypto.KeyGenerator, pubkeyConverter core.PubkeyConverter) ([]byte, string, error) {
	walletKey, err := generateKey(keyGen)
	if err != nil {
		return nil, "", err
	}

	file, err := generateFile(walletPemFile)
	if err != nil {
		return nil, "", err
	}

	addr, err := writeKeyToStream(file, walletKey, pubkeyConverter, pwd)
	if err != nil {
		return nil, "", err
	}

	// flush system buffers to disk
	err = file.Sync()
	if err != nil {
		return nil, "", err
	}

	log.Info("wallet created", "address", addr)

	return []byte(hex.EncodeToString(walletKey.skBytes)), addr, file.Close()
}

func generateKey(keyGen crypto.KeyGenerator) (key, error) {
	sk, pk := keyGen.GeneratePair()
	skBytes, err := sk.ToByteArray()
	if err != nil {
		return key{}, err
	}

	pkBytes, err := pk.ToByteArray()
	if err != nil {
		return key{}, err
	}

	return key{
		skBytes: skBytes,
		pkBytes: pkBytes,
	}, nil
}

func writeKeyToStream(writer io.Writer, key key, pubkeyConverter core.PubkeyConverter, pwd string) (string, error) {
	if check.IfNilReflect(writer) {
		return "", fmt.Errorf("nil writer")
	}

	pkString := pubkeyConverter.Encode(key.pkBytes)

	blk := &pem.Block{
		Type:  "PRIVATE KEY for " + pkString,
		Bytes: []byte(hex.EncodeToString(key.skBytes)),
	}

	if len(pwd) > 0 {
		var err error
		blk, err = EncryptPEMBlock(blk.Type, []byte(hex.EncodeToString(key.skBytes)), pwd)
		if err != nil {
			return "", err
		}
	}

	return pkString, pem.Encode(writer, blk)
}

func generateFile(walletPemFile string) (*os.File, error) {

	dir, file := filepath.Split(walletPemFile)

	if len(dir) == 0 {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	err := generateFolder(dir)
	if err != nil {
		return nil, err
	}

	filename := filepath.Join(dir, file)
	filename = filepath.Clean(filename)
	backupFileIfExists(filename)
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, common.FileModeUserReadWrite)
}

func generateFolder(absPath string) error {
	log.Info("generating files in", "folder", absPath)

	return os.MkdirAll(absPath, common.DefaultDirPermission)
}

func backupFileIfExists(filename string) {
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return
		}
	}
	//if we reached here the file probably exists, make a timestamped backup
	_ = os.Rename(filename, fmt.Sprintf("%s_%d", filename, time.Now().Unix()))
}

func PemFromSK(skBytes []byte, walletPemFile string, pwd string, keyGen crypto.KeyGenerator, pubkeyConverter core.PubkeyConverter) (string, error) {
	sk, err := keyGen.PrivateKeyFromByteArray(skBytes)
	if err != nil {
		return "", err
	}
	pk := sk.GeneratePublic()
	pkBytes, err := pk.ToByteArray()
	if err != nil {
		return "", err
	}

	walletKey := key{
		skBytes: skBytes,
		pkBytes: pkBytes,
	}

	file, err := generateFile(walletPemFile)
	if err != nil {
		return "", err
	}

	addr, err := writeKeyToStream(file, walletKey, pubkeyConverter, pwd)
	if err != nil {
		return "", err
	}

	log.Info("wallet created", "address", addr)

	return addr, file.Close()
}
