package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
)

// HexFormat defines the hex format for the pubkey converter
const HexFormat = "hex"

// Bech32Format defines the bech32 format for the pubkey converter
const Bech32Format = "bech32"

// NewPubkeyConverter will create a new pubkey converter based on the config provided
func NewPubkeyConverter(Type string, Length int) (core.PubkeyConverter, error) {
	switch Type {
	case HexFormat:
		return pubkeyConverter.NewHexPubkeyConverter(Length)
	case Bech32Format:
		return pubkeyConverter.NewBech32PubkeyConverter(Length)
	default:
		return nil, fmt.Errorf("%w unrecognized type %s", common.ErrInvalidPubkeyConverterType, Type)
	}
}
