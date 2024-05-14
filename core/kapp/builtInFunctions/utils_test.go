package builtInFunctions

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
)

func TestDecodeAccountPermissionData_Success(t *testing.T) {
	data, err := hex.DecodeString("00000001010000000a5065726d697373696f6e0000000000000001000000043066666600000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions, err := DecodeAccountPermissionData(data)
	assert.NoError(t, err)

	var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)

	addr, err := addressConverter.Decode("klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap")
	assert.NoError(t, err)

	expected := []*transaction.AccPermission{
		{
			Type:           1,
			PermissionName: "Permission",
			Threshold:      1,
			Operations:     []byte{15, 255},
			Signers:        []*transaction.AccKey{{Address: addr, Weight: 0}},
		},
	}

	for i, p := range permissions {
		assert.Equal(t, expected[i].Type, p.Type)
		assert.Equal(t, expected[i].PermissionName, p.PermissionName)
		assert.Equal(t, expected[i].Threshold, p.Threshold)
		assert.Equal(t, expected[i].Operations, p.Operations)
		assert.Equal(t, expected[i].Signers[0].Address, p.Signers[0].Address)
	}
}

func TestDecodeAccountPermissionData_OperationErr(t *testing.T) {
	data, err := hex.DecodeString("00000001010000000a5065726d697373696f6e00000000000000010000000430667a6600000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions, err := DecodeAccountPermissionData(data)
	assert.Nil(t, permissions)
	assert.Error(t, errors.New("error decoding operations"))

	data2, err := hex.DecodeString("00000001010000000a5065726d697373696f6e00000000000000010000000430667a6600000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions2, err := DecodeAccountPermissionData(data2)
	assert.Nil(t, permissions2)
	assert.Error(t, errors.New("invalid permission operation"))
}
