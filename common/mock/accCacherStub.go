package mock

import "github.com/klever-io/klever-go/data/state"

type AccountsCacherStub struct {
	GetExistingUserCalled  func(address []byte) (state.UserAccountHandler, error)
	GetExistingKappCalled  func(address []byte) (state.KAppAccountHandler, error)
	GetExistingPeerCalled  func(address []byte) (state.PeerAccountHandler, error)
	LoadUserCalled         func(address []byte) (state.UserAccountHandler, error)
	LoadKAppCalled         func(address []byte) (state.KAppAccountHandler, error)
	LoadKAppUncachedCalled func(address []byte) (state.KAppAccountHandler, error)
	LoadPeerCalled         func(address []byte) (state.PeerAccountHandler, error)
	ResetAllCalled         func(enableCache bool)
	SaveAllCalled          func() error
	SaveUserCalled         func(account state.AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdateUserCalled func(account state.AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdateKappCalled func(account state.AccountHandler) error
	/// Deprecated Update account has no effect now, only save account data prior AccountsCacher fork for backward compatibility
	UpdatePeerCalled     func(account state.AccountHandler) error
	GetCodeCalled        func(codeHash []byte) []byte
	RemoveCodeCalled     func(address []byte) error
	IsInterfaceNilCalled func() bool
}

func (stub *AccountsCacherStub) GetExistingUser(address []byte) (state.UserAccountHandler, error) {
	if stub.GetExistingUserCalled != nil {
		return stub.GetExistingUserCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) GetExistingKapp(address []byte) (state.KAppAccountHandler, error) {
	if stub.GetExistingKappCalled != nil {
		return stub.GetExistingKappCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) GetExistingPeer(address []byte) (state.PeerAccountHandler, error) {
	if stub.GetExistingPeerCalled != nil {
		return stub.GetExistingPeerCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) LoadUser(address []byte) (state.UserAccountHandler, error) {
	if stub.LoadUserCalled != nil {
		return stub.LoadUserCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) LoadKApp(address []byte) (state.KAppAccountHandler, error) {
	if stub.LoadKAppCalled != nil {
		return stub.LoadKAppCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) LoadKAppUncached(address []byte) (state.KAppAccountHandler, error) {
	if stub.LoadKAppUncachedCalled != nil {
		return stub.LoadKAppUncachedCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) LoadPeer(address []byte) (state.PeerAccountHandler, error) {
	if stub.LoadPeerCalled != nil {
		return stub.LoadPeerCalled(address)
	}
	return nil, nil
}

func (stub *AccountsCacherStub) ResetAll(enableCache bool) {
	if stub.ResetAllCalled != nil {
		stub.ResetAllCalled(enableCache)
	}
}

func (stub *AccountsCacherStub) SaveAll() error {
	if stub.SaveAllCalled != nil {
		return stub.SaveAllCalled()
	}
	return nil
}

func (stub *AccountsCacherStub) SaveUser(account state.AccountHandler) error {
	if stub.SaveUserCalled != nil {
		return stub.SaveUserCalled(account)
	}
	return nil
}

func (stub *AccountsCacherStub) UpdateUser(account state.AccountHandler) error {
	if stub.UpdateUserCalled != nil {
		return stub.UpdateUserCalled(account)
	}
	return nil
}

func (stub *AccountsCacherStub) UpdateKapp(account state.AccountHandler) error {
	if stub.UpdateKappCalled != nil {
		return stub.UpdateKappCalled(account)
	}
	return nil
}

func (stub *AccountsCacherStub) UpdatePeer(account state.AccountHandler) error {
	if stub.UpdatePeerCalled != nil {
		return stub.UpdatePeerCalled(account)
	}
	return nil
}

func (stub *AccountsCacherStub) GetCode(codeHash []byte) []byte {
	if stub.GetCodeCalled != nil {
		return stub.GetCodeCalled(codeHash)
	}
	return nil
}

func (stub *AccountsCacherStub) RemoveCode(address []byte) error {
	if stub.RemoveCodeCalled != nil {
		return stub.RemoveCodeCalled(address)
	}
	return nil
}

func (stub *AccountsCacherStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}
	return false
}
