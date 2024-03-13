package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObject_ToBuffer(t *testing.T) {
	t.Parallel()

	myObj := Object{
		"my": Array{
			Object{
				"key1": "value1",
			},
			Object{
				"key2": "value2",
			},
		},
	}

	myObjBuff := myObj.ToBuffer()
	expected := bytes.Buffer{}
	expected.Write([]byte("{\"my\":[{\"key1\":\"value1\"},{\"key2\":\"value2\"}]}"))
	require.Equal(t, expected.String(), myObjBuff.String())
}
