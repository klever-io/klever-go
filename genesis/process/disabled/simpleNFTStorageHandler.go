package disabled

import (
	"github.com/klever-io/klever-go/data/dkda"
	"github.com/klever-io/klever-go/data/state"
)

// SimpleNFTStorage implements the SimpleNFTStorage interface but does nothing as it is disabled
type SimpleNFTStorage struct {
}

// GetKDANFTTokenOnDestination is disabled
func (s *SimpleNFTStorage) GetKDANFTTokenOnDestination(_ state.UserAccountHandler, _ []byte, _ uint64) (*dkda.KDigitalToken, bool, error) {
	return &dkda.KDigitalToken{Value: 0}, true, nil
}

// IsInterfaceNil return true if underlying object is nil
func (s *SimpleNFTStorage) IsInterfaceNil() bool {
	return s == nil
}
