//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. proto/authMessage.proto
//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. proto/topicMessage.proto
package data

import "google.golang.org/protobuf/proto"

// AuthMessage represents the authentication message used in the handshake process of 2 peers
type AuthMessage struct {
	*AuthMessagePb
}

func (am *AuthMessage) Clone() *AuthMessage {
	return &AuthMessage{
		proto.Clone(am).(*AuthMessagePb),
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (am *AuthMessage) IsInterfaceNil() bool {
	return am == nil
}
