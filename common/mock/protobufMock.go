package mock

import (
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/tools/marshal"
	"google.golang.org/protobuf/proto"
)

// ProtobufMarshalizerMock implements marshaling with protobuf
type ProtobufMarshalizerMock struct {
}

// Marshal does the actual serialization of an object through protobuf
func (x *ProtobufMarshalizerMock) Marshal(obj interface{}) ([]byte, error) {
	if msg, ok := obj.(proto.Message); ok {
		// Treat nil message interface as an empty message; nothing to output.
		if msg == nil {
			return nil, nil
		}

		marshaler := proto.MarshalOptions{Deterministic: true}

		enc, err := marshaler.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return enc, nil
	}
	return nil, errors.New("can not serialize the object")
}

// Unmarshal does the actual deserialization of an object through protobuf
func (x *ProtobufMarshalizerMock) Unmarshal(obj interface{}, buff []byte) error {
	if msg, ok := obj.(proto.Message); ok {
		proto.Reset(msg)

		unmarshaler := proto.UnmarshalOptions{}

		return unmarshaler.Unmarshal(buff, msg)
	}
	return fmt.Errorf("%T, %w", obj, marshal.ErrUnmarshallingProto)

}

// IsInterfaceNil returns true if there is no value under the interface
func (x *ProtobufMarshalizerMock) IsInterfaceNil() bool {
	return x == nil
}
