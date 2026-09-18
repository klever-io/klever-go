//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. proto/topicMessage.proto
package data
