package pubkeyConverter_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/klever-io/klever-go-logger/check"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/stretchr/testify/assert"
)

func TestNewBech32PubkeyConverter_InvalidSizeShouldErr(t *testing.T) {
	t.Parallel()

	bpc, err := pubkeyConverter.NewBech32PubkeyConverter(-1)
	assert.True(t, errors.Is(err, common.ErrInvalidAddressLength))
	assert.True(t, check.IfNil(bpc))

	bpc, err = pubkeyConverter.NewBech32PubkeyConverter(0)
	assert.True(t, errors.Is(err, common.ErrInvalidAddressLength))
	assert.True(t, check.IfNil(bpc))

	bpc, err = pubkeyConverter.NewBech32PubkeyConverter(3)
	assert.True(t, errors.Is(err, common.ErrInvalidAddressLength))
	assert.True(t, check.IfNil(bpc))
}

func TestNewBech32PubkeyConverter_ShouldWork(t *testing.T) {
	t.Parallel()

	addressLen := 28
	bpc, err := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(bpc))
	assert.Equal(t, addressLen, bpc.Len())
}

func TestBech32PubkeyConverter_DecodeInvalidStringShouldErr(t *testing.T) {
	t.Parallel()

	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	str, err := bpc.Decode("not a bech32 string")

	assert.Equal(t, 0, len(str))
	assert.NotNil(t, err)
}

func TestBech32PubkeyConverter_DecodePrefixMismatchShouldErr(t *testing.T) {
	t.Parallel()

	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	str, err := bpc.Decode("err1xyerxdp4xcmnswfsxyerxdp4xcmnswfsxyerxdp4xcmnswfsxyeqnyphvl")

	assert.Equal(t, 0, len(str))
	assert.True(t, errors.Is(err, common.ErrInvalidKLVAddress))
}

func TestBech32PubkeyConverter_DecodeWrongSizeShouldErr(t *testing.T) {
	t.Parallel()

	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	str, err := bpc.Decode("klv1fdxy24j92g0p89jw")

	assert.Equal(t, 0, len(str))
	assert.True(t, errors.Is(err, common.ErrWrongSize))
}

func TestBech32PubkeyConverter_EncodeDecodeShouldWork(t *testing.T) {
	t.Parallel()

	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	buff := []byte("12345678901234567890123456789012")
	str := bpc.Encode(buff)

	assert.Equal(t, 0, strings.Index(str, pubkeyConverter.Prefix))

	fmt.Printf("generated address: %s\n", str)

	recoveredBuff, err := bpc.Decode(str)

	assert.Nil(t, err)
	assert.Equal(t, buff, recoveredBuff)
}

func TestBech32PubkeyConverter_EncodeWrongLengthShouldReturnEmpty(t *testing.T) {
	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)

	buff := []byte("12345678901234567890")
	str := bpc.Encode(buff)
	assert.Equal(t, "", str)

	buff = []byte{}
	str = bpc.Encode(buff)
	assert.Equal(t, "", str)

	buff = []byte("1234567890123456789012345678901234567890")
	str = bpc.Encode(buff)
	assert.Equal(t, "", str)
}
