//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. requestData.proto
package retriever

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// UnmarshalWith sets the fields according to p2p.MessageP2P.Data() contents
// Errors if something went wrong
func (rd *RequestData) UnmarshalWith(marshalizer marshal.Marshalizer, message p2p.MessageP2P) error {
	if marshalizer == nil || marshalizer.IsInterfaceNil() {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(message) {
		return common.ErrNilMessage
	}
	if message.Data() == nil {
		return common.ErrNilDataToProcess
	}

	err := marshalizer.Unmarshal(rd, message.Data())
	if err != nil {
		return err
	}

	return nil
}
