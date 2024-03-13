package marshal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestTxStruct struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Data     string `json:"data"`
}

func TestTxJSONMarshalizer_MarshalUnmarshalWithCharactersThatCouldBeEncoded(t *testing.T) {
	t.Parallel()

	tx := &TestTxStruct{
		Sender:   "sndr",
		Receiver: "rcvr",
		Data:     "data@~`!@#$^&*()_=[]{};'<>?,./|<>><!!!!!",
	}

	tjm := TxJSONMarshalizer{}

	// custom json marshalizer
	marshaledTx1, err := tjm.Marshal(tx)
	assert.NotNil(t, marshaledTx1)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(marshaledTx1), tx.Data))

	var resTx1 *TestTxStruct
	err = tjm.Unmarshal(&resTx1, marshaledTx1)
	assert.Equal(t, tx, resTx1)
	assert.NoError(t, err)
}
