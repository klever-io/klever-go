//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. proto/dkda.proto

package dkda

// New returns a new batch from given buffers
func New() *KDigitalToken {
	return &KDigitalToken{
		Value: int64(0),
	}
}
