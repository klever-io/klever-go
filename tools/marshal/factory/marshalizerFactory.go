package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/tools/marshal"
)

// JSONMarshalizer is the name reserved for the json marshalizer
const JSONMarshalizer = "json"

// TxJSONMarshalizer is the name reserved for the transaction json marshalizer
const TxJSONMarshalizer = "tx-json"

// ProtoMarshalizer is the name reserved for the protobuf marshalizer
const ProtoMarshalizer = "protobuf"

// NewTXSignMarshalizer -
func NewTXSignMarshalizer() (marshal.Marshalizer, error) {
	return NewMarshalizer(TxJSONMarshalizer)
}

// NewInternalMarshalizer -
func NewInternalMarshalizer() (marshal.Marshalizer, error) {
	return NewMarshalizer(ProtoMarshalizer)
}

// NewMarshalizer creates a new marshalizer instance based on the provided parameters
func NewMarshalizer(name string) (marshal.Marshalizer, error) {
	switch name {
	case JSONMarshalizer:
		return &marshal.JSONMarshalizer{}, nil
	case ProtoMarshalizer:
		return &marshal.ProtoMarshalizer{}, nil
	case TxJSONMarshalizer:
		return &marshal.TxJSONMarshalizer{}, nil
	default:
		return nil, fmt.Errorf("%w '%s'", marshal.ErrUnknownMarshalizer, name)
	}
}
