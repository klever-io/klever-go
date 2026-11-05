package mock

import (
	"bytes"

	"google.golang.org/protobuf/encoding/protowire"
)

// AppendUnknownField returns a copy of buff followed by a bytes field numbered fieldNumber that holds size bytes of 0xab
func AppendUnknownField(buff []byte, fieldNumber protowire.Number, size int) []byte {
	extended := protowire.AppendTag(bytes.Clone(buff), fieldNumber, protowire.BytesType)
	return protowire.AppendBytes(extended, bytes.Repeat([]byte{0xab}, size))
}
