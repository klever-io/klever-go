package mock

import (
	"fmt"

	"github.com/klever-io/klever-go/tools/marshal"
	"google.golang.org/protobuf/proto"
)

// ProtoMarshalizerMock implements marshaling with protobuf
type ProtoMarshalizerMock struct {
}

// Marshal does the actual serialization of an object
// The object to be serialized must implement the protoObj interface
func (pmm *ProtoMarshalizerMock) Marshal(obj interface{}) ([]byte, error) {
	if msg, ok := obj.(marshal.ProtoObj); ok {
		// Treat nil message interface as an empty message; nothing to output.
		if msg == nil {
			return nil, nil
		}

		marshaler := proto.MarshalOptions{Deterministic: true}

		marshalizedObj, err := marshaler.Marshal(msg)
		if err != nil {
			return nil, err
		}

		return marshalizedObj, nil
	}
	return nil, fmt.Errorf("%T, %w", obj, marshal.ErrMarshallingProto)
}

// Unmarshal does the actual deserialization of an object
// The object to be deserialized must implement the protoObj interface
func (pmm *ProtoMarshalizerMock) Unmarshal(obj interface{}, buff []byte) error {
	if msg, ok := obj.(marshal.ProtoObj); ok {
		proto.Reset(msg)

		unmarshaler := proto.UnmarshalOptions{}

		return unmarshaler.Unmarshal(buff, msg)
	}

	return fmt.Errorf("%T, %w", obj, marshal.ErrUnmarshallingProto)
}

// IsInterfaceNil returns true if there is no value under the interface
func (pmm *ProtoMarshalizerMock) IsInterfaceNil() bool {
	return pmm == nil
}
