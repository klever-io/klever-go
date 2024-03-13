package marshal

import (
	"google.golang.org/protobuf/proto"
)

// ProtoObj groups the necessary of a protobuf marshalizeble object
type ProtoObj interface {
	proto.Message
}

// Marshalizer defines the 2 basic operations: serialize (marshal) and deserialize (unmarshal)
type Marshalizer interface {
	Marshal(obj interface{}) ([]byte, error)
	Unmarshal(obj interface{}, buff []byte) error
	IsInterfaceNil() bool
}

// NewProtoMarshalizer return Marshalizer for proto
func NewProtoMarshalizer() Marshalizer {
	return &ProtoMarshalizer{}
}

// Sizer contains method Size that is needed to get the size of a proto obj
type Sizer interface {
	Size() (n int)
}
