package mock

import (
	"github.com/klever-io/klever-go/data"
)

type interceptedBlockMock struct {
	HeaderHandlerToUse data.HeaderHandler
	HashToUse          []byte
}

// NewInterceptedMetaBlockMock -
func NewInterceptedMetaBlockMock(hdr data.HeaderHandler, hash []byte) *interceptedBlockMock {
	return &interceptedBlockMock{
		HeaderHandlerToUse: hdr,
		HashToUse:          hash,
	}
}

// HeaderHandler -
func (i *interceptedBlockMock) HeaderHandler() data.HeaderHandler {
	return i.HeaderHandlerToUse
}

// CheckValidity -
func (i *interceptedBlockMock) CheckValidity() error {
	return nil
}

// IsForCurrentShard -
func (i *interceptedBlockMock) IsForCurrentShard() bool {
	return true
}

// Hash -
func (i *interceptedBlockMock) Hash() []byte {
	return i.HashToUse
}

// Type -
func (i *interceptedBlockMock) Type() string {
	return "type"
}

// String -
func (i *interceptedBlockMock) String() string {
	return "metaBlock"
}

// Identifiers -
func (i *interceptedBlockMock) Identifiers() [][]byte {
	return [][]byte{i.HashToUse}
}

// CheckTXSignature -
func (i *interceptedBlockMock) CheckTXSignature() error {
	return nil
}

// IsInterfaceNil -
func (i *interceptedBlockMock) IsInterfaceNil() bool {
	return i == nil
}
