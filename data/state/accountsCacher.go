package state

import (
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools/check"
)

type accountsCache struct {
	cacheEnable bool
	accounts    AccountsAdapter
	peers       AccountsAdapter
	kapps       AccountsAdapter
	userMap     map[string]UserAccountHandler
	userList    []string
	kappMap     map[string]KAppAccountHandler
	kappList    []string
	peerMap     map[string]PeerAccountHandler
	peerList    []string
	mut         sync.RWMutex
}

type ArgsAcccountCacher struct {
	Accounts AccountsAdapter
	Kapps    AccountsAdapter
	Peers    AccountsAdapter
}

func NewAccountsCacher(args ArgsAcccountCacher) (*accountsCache, error) {
	if check.IfNil(args.Accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(args.Peers) {
		return nil, common.ErrNilPeerAccountsAdapter
	}
	if check.IfNil(args.Kapps) {
		return nil, common.ErrNilKAppAccountsAdapter
	}

	return &accountsCache{
		accounts: args.Accounts,
		kapps:    args.Kapps,
		peers:    args.Peers,
		userMap:  make(map[string]UserAccountHandler),
		userList: make([]string, 0),
		kappMap:  make(map[string]KAppAccountHandler),
		kappList: make([]string, 0),
		peerMap:  make(map[string]PeerAccountHandler),
		peerList: make([]string, 0),
	}, nil

}

func (acc *accountsCache) GetExistingUser(address []byte) (UserAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	userAcc, ok := acc.userMap[string(address)]
	if !ok {
		acnt, err := acc.accounts.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		userAcc, ok = acnt.(UserAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.userMap[string(address)] = userAcc
			acc.userList = append(acc.userList, string(address))
		}
	}

	return userAcc, nil
}

func (acc *accountsCache) GetExistingKapp(address []byte) (KAppAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	kappAcc, ok := acc.kappMap[string(address)]
	if !ok {
		acnt, err := acc.kapps.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		kappAcc, ok = acnt.(KAppAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.kappMap[string(address)] = kappAcc
			acc.kappList = append(acc.kappList, string(address))
		}
	}

	return kappAcc, nil
}

func (acc *accountsCache) GetExistingPeer(address []byte) (PeerAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	peerAcc, ok := acc.peerMap[string(address)]
	if !ok {
		acnt, err := acc.peers.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		peerAcc, ok = acnt.(PeerAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.peerMap[string(address)] = peerAcc
			acc.peerList = append(acc.peerList, string(address))
		}
	}

	return peerAcc, nil
}

func (acc *accountsCache) LoadUser(address []byte) (UserAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	userAcc, ok := acc.userMap[string(address)]
	if !ok {
		acnt, err := acc.accounts.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		userAcc, ok = acnt.(UserAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.userMap[string(address)] = userAcc
			acc.userList = append(acc.userList, string(address))
		}

	}

	return userAcc, nil
}

func (acc *accountsCache) LoadKApp(address []byte) (KAppAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	kappAcc, ok := acc.kappMap[string(address)]
	if !ok {
		acnt, err := acc.kapps.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		kappAcc, ok = acnt.(KAppAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.kappMap[string(address)] = kappAcc
			acc.kappList = append(acc.kappList, string(address))
		}
	}

	return kappAcc, nil
}

func (acc *accountsCache) LoadPeer(address []byte) (PeerAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	peerAcc, ok := acc.peerMap[string(address)]
	if !ok {
		acnt, err := acc.peers.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		peerAcc, ok = acnt.(PeerAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.peerMap[string(address)] = peerAcc
			acc.peerList = append(acc.peerList, string(address))
		}
	}

	return peerAcc, nil
}

func (acc *accountsCache) ResetAll(enableCache bool) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	acc.userMap = make(map[string]UserAccountHandler)
	acc.userList = make([]string, 0)
	acc.kappMap = make(map[string]KAppAccountHandler)
	acc.kappList = make([]string, 0)
	acc.peerMap = make(map[string]PeerAccountHandler)
	acc.peerList = make([]string, 0)

	acc.cacheEnable = enableCache
}

// SaveAll save all cached accounts (User/KApps/Peers)
// if cacher not enable, list will be empty and none will be saved
func (acc *accountsCache) SaveAll() error {
	acc.mut.Lock()

	for _, value := range acc.userList {
		err := acc.accounts.SaveAccount(acc.userMap[value])
		if err != nil {
			return err
		}
	}

	for _, value := range acc.kappList {
		err := acc.kapps.SaveAccount(acc.kappMap[value])
		if err != nil {
			return err
		}
	}

	for _, value := range acc.peerList {
		err := acc.peers.SaveAccount(acc.peerMap[value])
		if err != nil {
			return err
		}
	}

	acc.mut.Unlock()

	acc.ResetAll(acc.cacheEnable)

	return nil
}

func (acc *accountsCache) UpdateUser(account AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}

	return acc.accounts.SaveAccount(account)
}

func (acc *accountsCache) UpdateKapp(account AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}

	return acc.kapps.SaveAccount(account)
}

func (acc *accountsCache) UpdatePeer(account AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}

	return acc.peers.SaveAccount(account)
}

func (acc *accountsCache) SaveUser(account AccountHandler) error {
	err := acc.accounts.SaveAccount(account)
	if err != nil {
		return err
	}

	acc.mut.Lock()
	defer acc.mut.Unlock()

	delete(acc.userMap, string(account.AddressBytes()))
	for i := range acc.userList {
		if acc.userList[i] == string(account.AddressBytes()) {
			// remove from slice
			acc.userList = append(acc.userList[:i], acc.userList[i+1:]...)
			break
		}
	}

	return nil
}

// GetCode returns the code for the given account
func (acc *accountsCache) GetCode(codeHash []byte) []byte {
	return acc.accounts.GetCode(codeHash)
}

func (acc *accountsCache) RemoveCode(address []byte) error {
	accHandler, err := acc.GetExistingUser(address)
	if err != nil {
		return err
	}

	accHandler.SetCode(make([]byte, 0))

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (acc *accountsCache) IsInterfaceNil() bool {
	return acc == nil
}
