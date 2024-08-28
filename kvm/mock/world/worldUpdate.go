package worldmock

import (
	"errors"
	"math/big"

	"github.com/klever-io/klever-go/data/state"

	"github.com/klever-io/klever-go/kapps"

	"github.com/klever-io/klever-go/vmcommon"
)

// UpdateBalance sets a new balance to an account
func (b *MockWorld) UpdateBalance(address []byte, newBalance *big.Int) error {
	user, err := b.AccountsCacher.LoadUser(address)
	if err != nil {
		return err
	}
	userKDA := kapps.UserKDA{Balance: newBalance.Int64()}
	err = user.SetUserKDA(nil, nil, &userKDA)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBalanceWithDelta changes balance of an account by a given amount
func (b *MockWorld) UpdateBalanceWithDelta(address []byte, balanceDelta *big.Int) error {
	return nil
}

// UpdateWorldStateBefore performs gas payment, before transaction
func (b *MockWorld) UpdateWorldStateBefore(
	fromAddr []byte,
	gasLimit uint64,
	gasPrice uint64) error {

	acct, err := b.AccountsCacher.LoadUser(fromAddr)
	if err != nil {
		return err
	}
	if acct == nil {
		return errors.New("method UpdateWorldStateBefore expects an existing address")
	}
	acct.IncreaseNonce(1)
	gasPayment := big.NewInt(0).Mul(
		big.NewInt(0).SetUint64(gasLimit),
		big.NewInt(0).SetUint64(gasPrice))
	if acct.GetBalance(nil, true) < gasPayment.Int64() {
		return errors.New("not enough balance to pay gas upfront")
	}
	err = acct.SubFromBalance(gasPayment.Int64(), nil, true)
	if err != nil {
		return err
	}
	// nonce and gas usage must be updated no matter the result of the transaction
	return b.AccountsCacher.SaveUser(acct)
}

// UpdateAccounts should be called after the VM test has run, to update world state
func (b *MockWorld) UpdateAccounts(
	outputAccounts map[string]*vmcommon.OutputAccount,
	accountsToDelete [][]byte) error {

	for _, modAcct := range outputAccounts {
		b.UpdateAccountFromOutputAccount(modAcct)
	}

	return b.CommitChangesToDB()
}

func (b *MockWorld) CommitChangesToDB() error {
	if err := b.AccountsCacher.SaveAll(); err != nil {
		return err
	}

	_, err := b.AccountsAdapter.Commit()
	if err != nil {
		return err
	}
	_, err = b.KAppsAdapter.Commit()
	if err != nil {
		return err
	}
	_, err = b.PeersAdapter.Commit()
	if err != nil {
		return err
	}

	return nil
}

// UpdateAccountFromOutputAccount updates a single account from a transaction output.
// kdaTransfers are not handled here, they are handled in the transaction using kapps
func (b *MockWorld) UpdateAccountFromOutputAccount(modAcct *vmcommon.OutputAccount) {
	cacher, ok := b.AccountsCacher.(*WorldAccountsCacher)
	if !ok {
		return
	}

	acct, err := b.AccountsCacher.GetExistingUser(modAcct.Address)
	if err != nil {
		acct, err = state.NewUserAccount(modAcct.Address)
		if err != nil {
			return
		}
		acct.SetOwnerAddress(modAcct.CodeDeployerAddress)
	}

	if len(modAcct.Code) > 0 {
		codeMeta := &vmcommon.CodeMetadata{
			Payable:     true,
			Upgradeable: true,
			Readable:    true,
		}
		acct.SetCodeHash(DefaultHasher.Compute(string(modAcct.Code)))
		acct.SetCode(modAcct.Code)
		acct.SetCodeMetadata(codeMeta.ToBytes())
		// update owner if none is set
		if len(acct.GetOwnerAddress()) == 0 {
			acct.SetOwnerAddress(modAcct.CodeDeployerAddress)
		}
	}

	for _, stu := range modAcct.StorageUpdates {
		err := acct.SaveKeyValue(stu.Offset, stu.Data)
		cacher.RegisterStorageKeyUse(acct.AddressBytes(), stu.Offset)

		if err != nil {
			return
		}
	}

	err = b.AccountsCacher.SaveUser(acct)
	if err != nil {
		return
	}
}

// CreateStateBackup -
func (b *MockWorld) CreateStateBackup() {
}

// CommitChanges -
func (b *MockWorld) CommitChanges() error {
	return b.CommitChangesToDB()
}

// RollbackChanges should be called after the VM test has run, if the tx has failed
func (b *MockWorld) RollbackChanges() error {
	b.AccountsCacher.ResetAll(true)
	return nil
}
