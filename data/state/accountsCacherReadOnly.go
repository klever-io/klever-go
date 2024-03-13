package state

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools/check"
)

type readOnlyAccountsCacher struct {
	originalAccCacher AccountsCacher
}

func NewReadOnlyAccountsCacher(args ArgsAcccountCacher) (*readOnlyAccountsCacher, error) {
	if check.IfNil(args.Accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(args.Peers) {
		return nil, common.ErrNilPeerAccountsAdapter
	}
	if check.IfNil(args.Kapps) {
		return nil, common.ErrNilKAppAccountsAdapter
	}

	accCacher, err := NewAccountsCacher(
		ArgsAcccountCacher{
			Accounts: args.Accounts,
			Kapps:    args.Kapps,
			Peers:    args.Peers,
		},
	)
	if err != nil {
		return nil, err
	}

	return &readOnlyAccountsCacher{
		originalAccCacher: accCacher,
	}, nil

}

func (roAcc *readOnlyAccountsCacher) GetExistingUser(address []byte) (UserAccountHandler, error) {
	return roAcc.originalAccCacher.GetExistingUser(address)
}

func (roAcc *readOnlyAccountsCacher) GetExistingKapp(address []byte) (KAppAccountHandler, error) {
	return roAcc.originalAccCacher.GetExistingKapp(address)
}

func (roAcc *readOnlyAccountsCacher) GetExistingPeer(address []byte) (PeerAccountHandler, error) {
	return roAcc.originalAccCacher.GetExistingPeer(address)
}

func (roAcc *readOnlyAccountsCacher) LoadUser(address []byte) (UserAccountHandler, error) {
	return roAcc.originalAccCacher.LoadUser(address)
}

func (roAcc *readOnlyAccountsCacher) LoadKApp(address []byte) (KAppAccountHandler, error) {
	return roAcc.originalAccCacher.LoadKApp(address)
}

func (roAcc *readOnlyAccountsCacher) LoadPeer(address []byte) (PeerAccountHandler, error) {
	return roAcc.originalAccCacher.LoadPeer(address)
}

func (roAcc *readOnlyAccountsCacher) ResetAll(enableCache bool) {
	roAcc.originalAccCacher.ResetAll(enableCache)
}

func (roAcc *readOnlyAccountsCacher) SaveAll() error {
	return nil
}

func (roAcc *readOnlyAccountsCacher) UpdateUser(account AccountHandler) error {
	return nil
}

func (roAcc *readOnlyAccountsCacher) UpdateKapp(account AccountHandler) error {
	return nil
}

func (roAcc *readOnlyAccountsCacher) UpdatePeer(account AccountHandler) error {
	return nil
}

func (roAcc *readOnlyAccountsCacher) SaveUser(account AccountHandler) error {
	return nil
}

// GetCode returns the code for the given account
func (roAcc *readOnlyAccountsCacher) GetCode(codeHash []byte) []byte {
	return roAcc.originalAccCacher.GetCode(codeHash)
}

func (roAcc *readOnlyAccountsCacher) RemoveAccount(address []byte) error {
	return nil
}

func (acc *readOnlyAccountsCacher) RemoveCode(address []byte) error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (roAcc *readOnlyAccountsCacher) IsInterfaceNil() bool {
	return roAcc == nil
}
