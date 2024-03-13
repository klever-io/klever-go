package storageBootstrap

import (
	"github.com/klever-io/klever-go/data"
)

// StorageBootstrapper is the main interface for bootstrap from storage execution engine
type storageBootstrapperHandler interface {
	getHeader(hash []byte) (data.HeaderHandler, error)
	getHeaderWithNonce(nonce uint64) (data.HeaderHandler, []byte, error)
	cleanupNotarizedStorage(hash []byte)
	IsInterfaceNil() bool
}
