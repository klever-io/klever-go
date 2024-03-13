package node

import (
	"errors"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
)

// GetAccount will return account details for a given address
func (n *Node) GetAccount(address string) (state.UserAccountHandler, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	addr, err := n.addressPubkeyConverter.Decode(address)
	if err != nil {
		return nil, err
	}

	accWrp, err := n.accounts.GetExistingAccount(addr)
	if err != nil {
		if err == common.ErrAccNotFound {
			return state.NewUserAccount(addr)
		}
		return nil, errors.New("could not fetch sender address from provided param: " + err.Error())
	}

	account, ok := n.castAccountToUserAccount(accWrp)
	if !ok {
		return nil, errors.New("this is not an user account address")
	}

	return account, nil
}

// GetNextNonce will return account next nonce
func (n *Node) GetNextNonce(address string) (uint64, uint64, uint64, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return 0, 0, 0, common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.accounts) {
		return 0, 0, 0, common.ErrNilAccountsAdapter
	}

	addr, err := n.addressPubkeyConverter.Decode(address)
	if err != nil {
		return 0, 0, 0, err
	}

	accWrp, err := n.accounts.GetExistingAccount(addr)
	if err != nil {
		if err == common.ErrAccNotFound {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, errors.New("could not fetch sender address from provided param: " + err.Error())
	}

	accNonce := accWrp.GetNonce()
	var numTxsPending uint64
	var firstPendingNonce uint64
	// check mempool for pending TXs
	if txPool, ok := n.dataPool.Transactions().ShardDataStore("0").(TxCache); ok {
		firstPendingNonce, numTxsPending = txPool.PendingFor(addr)
	}

	return accNonce, firstPendingNonce, numTxsPending, nil
}

// GetBalance gets the balance for a specific address
func (n *Node) GetBalance(address string, kda string) (int64, error) {
	account, err := n.getAccountHandler(address)
	if err != nil {
		return 0, err
	}

	userAccount, ok := n.castAccountToUserAccount(account)
	if !ok {
		return 0, nil
	}

	return userAccount.GetBalance([]byte(kda)), nil
}

// GetAvailableClaim returns the rewards avaible for a specific asset in an account
func (n *Node) GetAvailableClaim(address string, assetId string) (int64, map[string]int64, int64, error) {
	account, err := n.getAccountHandler(address)
	if err != nil {
		return 0, nil, 0, err
	}

	userAccount, ok := n.castAccountToUserAccount(account)
	if !ok {
		return 0, nil, 0, nil
	}

	currentBlockHeader := n.blkc.GetCurrentBlockHeader()

	stakingAcnt, err := n.kapps.LoadAccount(kapps.StakingKAppAddress)
	if err != nil {
		return 0, nil, 0, err
	}

	stakingKapp, ok := stakingAcnt.(state.KAppAccountHandler)
	if !ok {
		return 0, nil, 0, common.ErrWrongTypeAssertion
	}

	key := kdautils.ToKDAKey([]byte(assetId), nil)

	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return 0, nil, 0, err
	}
	if len(stakedBytes) == 0 {
		return 0, nil, 0, common.ErrStakingNotFound
	}

	kdaStaking := &kapps.StakingData{}
	err = n.internalMarshalizer.Unmarshal(kdaStaking, stakedBytes)
	if err != nil {
		return 0, nil, 0, err
	}

	kdaAcnt, err := n.kapps.LoadAccount(kapps.KDAKAppAddress)
	if err != nil {
		return 0, nil, 0, err
	}

	kdaKapp, ok := kdaAcnt.(state.KAppAccountHandler)
	if !ok {
		return 0, nil, 0, common.ErrWrongTypeAssertion
	}

	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return 0, nil, 0, err
	}
	if len(kdaBytes) == 0 {
		return 0, nil, 0, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	err = n.internalMarshalizer.Unmarshal(kda, kdaBytes)
	if err != nil {
		return 0, nil, 0, err
	}

	userKDA, err := userAccount.GetUserKDA([]byte(assetId), nil)
	if err != nil {
		return 0, nil, 0, err
	}

	rewards, err := userAccount.ComputeAvailableClaim(
		[]byte(assetId),
		currentBlockHeader.GetEpoch(),
		currentBlockHeader.GetTimestamp(),
		userKDA,
		kdaStaking,
		n.forkController,
	)
	if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
		return 0, nil, 0, err
	}

	allowance := int64(0)
	if assetId == "KLV" {
		allowance = userAccount.GetAllowance()
	}

	computedRewards := int64(0)
	if len(rewards) > 0 {
		// if assetID is KFI, computedRewards must be in KLV stack
		if assetId == string(kdautils.KFIIdentifier) {
			assetId = string(kdautils.KLVIdentifier)
		}
		computedRewards = rewards[assetId]
	}

	return computedRewards, rewards, allowance, nil
}

func (n *Node) castAccountToUserAccount(ah state.AccountHandler) (state.UserAccountHandler, bool) {
	if check.IfNil(ah) {
		return nil, false
	}

	account, ok := ah.(state.UserAccountHandler)
	return account, ok
}

func (n *Node) getAccountHandler(address string) (state.AccountHandler, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	addr, err := n.addressPubkeyConverter.Decode(address)
	if err != nil {
		return nil, errors.New("invalid address, could not decode from: " + err.Error())
	}
	return n.accounts.GetExistingAccount(addr)
}

// GetAddressPCK will return address pub key converter
func (n *Node) GetAddressPCK() core.PubkeyConverter {
	return n.addressPubkeyConverter
}

// GetValidatorPCK will return validator pub key converter
func (n *Node) GetValidatorPCK() core.PubkeyConverter {
	return n.validatorPubkeyConverter
}

// GetEncodedAddressLength will return validator pub key converter
func (n *Node) GetEncodedAddressLength() int {
	return n.encodedAddressLength
}
