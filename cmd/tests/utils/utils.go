package utils

import (
	operatorUtils "github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
)

const txSignPubkeyLen = 32

var (
	walletPubKeyConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(txSignPubkeyLen)
)

func LoadKey(pem string) ([]byte, []byte, string, error) {
	return operatorUtils.LoadKey(pem, 0, walletPubKeyConverter, "", "")
}
