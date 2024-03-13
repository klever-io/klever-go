package worldmock

import "github.com/klever-io/klever-go/data/state"

// var _ vmcommon.GuardedAccountHandler = (*GuardedAccountHandlerStub)(nil)

// GuardedAccountHandlerStub -
type GuardedAccountHandlerStub struct {
	GetActiveGuardianCalled    func(handler state.UserAccountHandler) ([]byte, error)
	SetGuardianCalled          func(uah state.UserAccountHandler, guardianAddress []byte, txGuardianAddress []byte, guardianServiceUID []byte) error
	CleanOtherThanActiveCalled func(uah state.UserAccountHandler)
}

// GetActiveGuardian -
func (gahs *GuardedAccountHandlerStub) GetActiveGuardian(handler state.UserAccountHandler) ([]byte, error) {
	if gahs.GetActiveGuardianCalled != nil {
		return gahs.GetActiveGuardianCalled(handler)
	}
	return nil, nil
}

// SetGuardian -
func (gahs *GuardedAccountHandlerStub) SetGuardian(uah state.UserAccountHandler, guardianAddress []byte, txGuardianAddress []byte, guardianServiceUID []byte) error {
	if gahs.SetGuardianCalled != nil {
		return gahs.SetGuardianCalled(uah, guardianAddress, txGuardianAddress, guardianServiceUID)
	}
	return nil
}

// CleanOtherThanActive -
func (gahs *GuardedAccountHandlerStub) CleanOtherThanActive(uah state.UserAccountHandler) {
	if gahs.CleanOtherThanActiveCalled != nil {
		gahs.CleanOtherThanActiveCalled(uah)
	}
}

// IsInterfaceNil -
func (gahs *GuardedAccountHandlerStub) IsInterfaceNil() bool {
	return gahs == nil
}
