package factory

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/stretchr/testify/assert"
)

func TestNewPubkeyConverter_HexShouldWork(t *testing.T) {
	t.Parallel()

	pc, err := NewPubkeyConverter(HexFormat, 32)

	assert.Nil(t, err)
	expected, _ := pubkeyConverter.NewHexPubkeyConverter(32)
	assert.IsType(t, expected, pc)
}

func TestNewPubkeyConverter_Bech32ShouldWork(t *testing.T) {
	t.Parallel()

	pc, err := NewPubkeyConverter(Bech32Format, 32)

	assert.Nil(t, err)
	expected, _ := pubkeyConverter.NewBech32PubkeyConverter(32)
	assert.IsType(t, expected, pc)
}

func TestNewPubkeyConverter_UnknownTypeShouldErr(t *testing.T) {
	t.Parallel()

	pc, err := NewPubkeyConverter("unknown", 32)

	assert.Nil(t, pc)
	assert.True(t, errors.Is(err, common.ErrInvalidPubkeyConverterType))
}
