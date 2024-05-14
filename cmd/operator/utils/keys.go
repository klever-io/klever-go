package utils

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

func LoadKey(pemFile string, skIndex int, converter core.PubkeyConverter, pwd string, pwdFilePath string) ([]byte, []byte, string, error) {
	// if password flag, ask for password
	pwd, err := GetPassphrase(pwd, pwdFilePath)
	if err != nil {
		return nil, nil, "", err
	}

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

// GetPassphrase fetches the correct passphrase depending on if a file is available to
// read from or if the user wants to enter in their own passphrase. Otherwise, just use
// the default passphrase. No confirmation of passphrase
func GetPassphrase(pwd string, pwdFilePath string) (string, error) {
	if pwdFilePath != "" {
		if _, err := os.Stat(pwdFilePath); os.IsNotExist(err) {
			return "", fmt.Errorf("password file not found at `%s`", pwdFilePath)
		}
		dat, err := os.ReadFile(filepath.Clean(pwdFilePath))
		if err != nil {
			return "", err
		}
		pw := strings.TrimSuffix(string(dat), "\n")
		return pw, nil
	}

	if pwd == "*" {
		fmt.Println("Enter password:")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		return string(pass), nil
	}

	return pwd, nil
}

// GetPassphraseWithConfirm fetches the correct passphrase depending on if a file is available to
// read from or if the user wants to enter in their own passphrase. Otherwise, just use
// the default passphrase. Passphrase requires a confirmation
func GetPassphraseWithConfirm(pwd string, pwdFilePath string) (string, error) {
	if pwdFilePath != "" {
		if _, err := os.Stat(pwdFilePath); os.IsNotExist(err) {
			return "", fmt.Errorf("password file not found at `%s`", pwdFilePath)
		}
		dat, err := os.ReadFile(filepath.Clean(pwdFilePath))
		if err != nil {
			return "", err
		}
		pw := strings.TrimSuffix(string(dat), "\n")
		return pw, nil
	}

	if pwd == "*" {
		fmt.Println("Enter password:")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		fmt.Println("Repeat the password:")
		repeatPass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		if string(repeatPass) != string(pass) {
			return "", errors.New("password does not match")
		}
		fmt.Println("") // provide feedback when passphrase is entered.
		return string(repeatPass), nil
	}

	return pwd, nil
}
