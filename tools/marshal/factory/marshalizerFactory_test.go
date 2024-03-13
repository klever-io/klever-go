package factory

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
)

func TestNewMarshalizer_UnknownTypeShouldErr(t *testing.T) {
	t.Parallel()

	mrs, err := NewMarshalizer("unknown")

	assert.True(t, check.IfNil(mrs))
	assert.True(t, errors.Is(err, marshal.ErrUnknownMarshalizer))
}

func TestNewMarshalizer_JsonShouldWork(t *testing.T) {
	t.Parallel()

	mrs, err := NewMarshalizer(JSONMarshalizer)

	jsonMrs := (*marshal.JSONMarshalizer)(nil)
	assert.Nil(t, err)
	assert.IsType(t, jsonMrs, mrs)
}

func TestNewMarshalizer_ProtobufShouldWork(t *testing.T) {
	t.Parallel()

	mrs, err := NewMarshalizer(ProtoMarshalizer)

	protoMrs := (*marshal.ProtoMarshalizer)(nil)
	assert.Nil(t, err)
	assert.IsType(t, protoMrs, mrs)
}
