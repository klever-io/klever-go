package bootstrapStorage

import "google.golang.org/protobuf/proto"

func (x *BootstrapData) Clone() *BootstrapData {
	return proto.Clone(x).(*BootstrapData)
}

func (x *BootstrapHeaderInfo) Clone() *BootstrapHeaderInfo {
	return proto.Clone(x).(*BootstrapHeaderInfo)
}
