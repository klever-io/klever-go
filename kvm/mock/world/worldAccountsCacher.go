package worldmock

import (
	"bytes"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state"

	"github.com/klever-io/klever-go/tools/check"
)

// WorldAccountsCacher is a copy of the state.AccountCacher
// we need to implement our own custom version of this interface to access private fields
type WorldAccountsCacher struct {
	cacheEnable     bool
	accounts        state.AccountsAdapter
	peers           state.AccountsAdapter
	kapps           state.AccountsAdapter
	addressList     [][]byte
	usedStorageKeys map[string][][]byte
	userMap         map[string]state.UserAccountHandler
	userList        []string
	kappMap         map[string]state.KAppAccountHandler
	kappList        []string
	peerMap         map[string]state.PeerAccountHandler
	peerList        []string
	mut             sync.RWMutex
}

func NewMockWorldAccountsCacher(args state.ArgsAcccountCacher) (state.AccountsCacher, error) {
	if check.IfNil(args.Accounts) {
		return nil, common.ErrNilAccountsAdapter
	}
	if check.IfNil(args.Peers) {
		return nil, common.ErrNilPeerAccountsAdapter
	}
	if check.IfNil(args.Kapps) {
		return nil, common.ErrNilKAppAccountsAdapter
	}

	return &WorldAccountsCacher{
		accounts: args.Accounts,
		kapps:    args.Kapps,
		peers:    args.Peers,
		userMap:  make(map[string]state.UserAccountHandler),
		userList: make([]string, 0),
		kappMap:  make(map[string]state.KAppAccountHandler),
		kappList: make([]string, 0),
		peerMap:  make(map[string]state.PeerAccountHandler),
		peerList: make([]string, 0),
	}, nil

}

func (acc *WorldAccountsCacher) GetExistingUser(address []byte) (state.UserAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	userAcc, ok := acc.userMap[string(address)]
	if !ok {
		acnt, err := acc.accounts.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		userAcc, ok = acnt.(state.UserAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.userMap[string(address)] = userAcc
			acc.userList = append(acc.userList, string(address))
		}
	}
	// load contract code
	if len(userAcc.GetCodeHash()) > 0 {
		userAcc.SetCode(acc.GetCode(userAcc.GetCodeHash()))
	}
	acc.RegisterAddress(address)
	return userAcc, nil
}

func (acc *WorldAccountsCacher) GetExistingKapp(address []byte) (state.KAppAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	kappAcc, ok := acc.kappMap[string(address)]
	if !ok {
		acnt, err := acc.kapps.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		kappAcc, ok = acnt.(state.KAppAccountHandler)
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

func (acc *WorldAccountsCacher) GetExistingPeer(address []byte) (state.PeerAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	peerAcc, ok := acc.peerMap[string(address)]
	if !ok {
		acnt, err := acc.peers.GetExistingAccount(address)
		if err != nil {
			return nil, err
		}

		peerAcc, ok = acnt.(state.PeerAccountHandler)
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

func (acc *WorldAccountsCacher) LoadUser(address []byte) (state.UserAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	userAcc, ok := acc.userMap[string(address)]
	if !ok {
		acnt, err := acc.accounts.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		userAcc, ok = acnt.(state.UserAccountHandler)
		if !ok {
			return nil, common.ErrWrongTypeAssertion
		}

		// only use cacher if fork have passed
		if acc.cacheEnable {
			acc.userMap[string(address)] = userAcc
			acc.userList = append(acc.userList, string(address))
		}

	}
	// load contract code
	if len(userAcc.GetCodeHash()) > 0 {
		userAcc.SetCode(acc.GetCode(userAcc.GetCodeHash()))
	}
	acc.RegisterAddress(address)
	return userAcc, nil
}

func (acc *WorldAccountsCacher) LoadKApp(address []byte) (state.KAppAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	kappAcc, ok := acc.kappMap[string(address)]
	if !ok {
		acnt, err := acc.kapps.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		kappAcc, ok = acnt.(state.KAppAccountHandler)
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

func (acc *WorldAccountsCacher) LoadPeer(address []byte) (state.PeerAccountHandler, error) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	peerAcc, ok := acc.peerMap[string(address)]
	if !ok {
		acnt, err := acc.peers.LoadAccount(address)
		if err != nil {
			return nil, err
		}

		peerAcc, ok = acnt.(state.PeerAccountHandler)
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

func (acc *WorldAccountsCacher) ResetAll(enableCache bool) {
	acc.mut.Lock()
	defer acc.mut.Unlock()

	acc.userMap = make(map[string]state.UserAccountHandler)
	acc.userList = make([]string, 0)
	acc.kappMap = make(map[string]state.KAppAccountHandler)
	acc.kappList = make([]string, 0)
	acc.peerMap = make(map[string]state.PeerAccountHandler)
	acc.peerList = make([]string, 0)

	acc.cacheEnable = enableCache
}

// SaveAll save all cached accounts (User/KApps/Peers)
// if cacher not enable, list will be empty and none will be saved
func (acc *WorldAccountsCacher) SaveAll() error {
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

func (acc *WorldAccountsCacher) UpdateUser(account state.AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}
	acc.RegisterAddress(account.AddressBytes())
	return acc.accounts.SaveAccount(account)
}

func (acc *WorldAccountsCacher) UpdateKapp(account state.AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}

	return acc.kapps.SaveAccount(account)
}

func (acc *WorldAccountsCacher) UpdatePeer(account state.AccountHandler) error {
	if acc.cacheEnable {
		return nil
	}

	return acc.peers.SaveAccount(account)
}

func (acc *WorldAccountsCacher) SaveUser(account state.AccountHandler) error {
	err := acc.accounts.SaveAccount(account)
	if err != nil {
		// just save to cache in case of error
		// for test purpose only
		acc.mut.Lock()
		defer acc.mut.Unlock()

		_, ok := acc.userMap[string(account.AddressBytes())]
		if !ok {
			// add to list
			acc.userList = append(acc.userList, string(account.AddressBytes()))
		}
		// update
		acc.userMap[string(account.AddressBytes())] = account.(state.UserAccountHandler)
		return nil
	}

	acc.mut.Lock()
	defer acc.mut.Unlock()

	acc.RegisterAddress(account.AddressBytes())

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
func (acc *WorldAccountsCacher) GetCode(codeHash []byte) []byte {
	return acc.accounts.GetCode(codeHash)
}

func (acc *WorldAccountsCacher) RemoveCode(address []byte) error {
	accHandler, err := acc.GetExistingUser(address)
	if err != nil {
		return err
	}

	accHandler.SetCode(make([]byte, 0))

	return nil
}

func (acc *WorldAccountsCacher) RegisterAddress(address []byte) {
	exists := false
	for _, value := range acc.addressList {
		if bytes.Equal(value, address) {
			exists = true
			break
		}
	}

	if !exists {
		acc.addressList = append(acc.addressList, address)
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (acc *WorldAccountsCacher) IsInterfaceNil() bool {
	return acc == nil
}

func (acc *WorldAccountsCacher) GetAddressList() [][]byte {
	return acc.addressList
}

func (acc *WorldAccountsCacher) RegisterStorageKeyUse(address []byte, key []byte) {
	if acc.usedStorageKeys == nil {
		acc.usedStorageKeys = make(map[string][][]byte)
	}

	addressKeys, exists := acc.usedStorageKeys[string(address)]
	if !exists {
		acc.usedStorageKeys[string(address)] = make([][]byte, 0)
	}

	exists = false
	for _, value := range addressKeys {
		if bytes.Equal(value, address) {
			exists = true
			break
		}
	}

	if !exists {
		acc.usedStorageKeys[string(address)] = append(acc.usedStorageKeys[string(address)], key)
	}
}

func (acc *WorldAccountsCacher) GetUsedStorageKeys(address []byte) [][]byte {
	return acc.usedStorageKeys[string(address)]
}
