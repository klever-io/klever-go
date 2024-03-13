package requestHandlers

import "github.com/klever-io/klever-go/core"

// HashSliceResolver can request multiple hashes at once
type HashSliceResolver interface {
	RequestDataFromHashArray(hashes [][]byte, epoch uint32) error
	RequestDataFromHashArrayTo(hashes [][]byte, epoch uint32, peer core.PeerID) error
	IsInterfaceNil() bool
}
