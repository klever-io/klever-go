package worldmock

import "github.com/klever-io/klever-go/data/state"

// MockGuardedAccountHandler -
type MockGuardedAccountHandler struct{}

// NewMockGuardedAccountHandler -
func NewMockGuardedAccountHandler() *MockGuardedAccountHandler {
	return &MockGuardedAccountHandler{}
}

// GetActiveGuardian -
func (mah *MockGuardedAccountHandler) GetActiveGuardian(_ state.UserAccountHandler) ([]byte, error) {
	return nil, nil
}

// SetGuardian -
func (mah *MockGuardedAccountHandler) SetGuardian(_ state.UserAccountHandler, _ []byte, _ []byte, _ []byte) error {
	return nil
}

// CleanOtherThanActive -
func (mah *MockGuardedAccountHandler) CleanOtherThanActive(_ state.UserAccountHandler) {
}

// IsInterfaceNil -
func (mah *MockGuardedAccountHandler) IsInterfaceNil() bool {
	return mah == nil
}
