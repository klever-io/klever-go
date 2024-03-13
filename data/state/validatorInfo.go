//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. validatorInfo.proto

package state

// IsInterfaceNil returns true if there is no value under the interface
func (vi *ValidatorInfo) IsInterfaceNil() bool {
	return vi == nil
}
