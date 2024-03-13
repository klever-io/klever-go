package marshal_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
)

var protoMarsh = marshal.ProtoMarshalizer{}

func recovedMarshal(obj interface{}) (buf []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			if panicError, ok := p.(error); ok {
				err = panicError
			} else {
				err = fmt.Errorf("%#v", p)
			}
			buf = nil
		}
	}()
	buf, err = protoMarsh.Marshal(obj)
	return
}

func TestProtoMarshalizer_MarshalWrongObj(t *testing.T) {

	obj := "klever"
	encNode, err := recovedMarshal(obj)
	assert.Nil(t, encNode)
	assert.NotNil(t, err)
}
