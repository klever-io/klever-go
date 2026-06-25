package node

import (
	"context"
	"errors"

	"github.com/klever-io/klever-go/common"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools/check"
)

// GetEconomics returns live KLV supply figures plus node-state held aggregates, memoized per block.
// The `/network/economics` route is opt-in (disabled by default in api.yaml). See KLC-2506.
func (n *Node) GetEconomics() (*models.EconomicsResponse, error) {
	return n.economics.get(n.blkc, n.computeEconomics)
}

// computeEconomics reads live supply plus held aggregates from node state. Use GetEconomics (cached).
func (n *Node) computeEconomics() (*models.EconomicsResponse, error) {
	kdaData, err := n.GetAsset(string(kdautils.KLVIdentifier))
	if err != nil {
		return nil, err
	}

	pendingRewardsTotal := int64(0)
	marketEscrowTotal := int64(0)
	if !check.IfNil(n.kappController) {
		pendingRewardsTotal, err = n.kappController.GetValidatorsKApp().GetPendingRewardsTotal()
		if err != nil {
			return nil, err
		}
		marketEscrowTotal, err = n.kappController.GetMarketKApp().GetMarketEscrowTotal()
		if err != nil {
			return nil, err
		}
	}

	feesPoolKLVTotal := int64(0)
	systemAccountKLV := int64(0)
	fprPoolKLVTotal := int64(0)
	if !check.IfNil(n.kapps) {
		feesPoolKLVTotal, err = n.loadFeesPoolKLVTotal()
		if err != nil {
			return nil, err
		}
		systemAccountKLV, err = n.loadSystemAccountKLV()
		if err != nil {
			return nil, err
		}
		fprPoolKLVTotal, err = n.loadFPRPoolKLVTotal()
		if err != nil {
			return nil, err
		}
	}

	accumulatedFeesTotal := int64(0)
	if !check.IfNil(n.validatorsProvider) {
		accumulatedFeesTotal = n.loadAccumulatedFeesTotal()
	}

	// A missing KLV StakingData record (fresh/test networks) means no stake, not an error.
	staking, err := n.loadStakingData(string(kdautils.KLVIdentifier))
	if err != nil && !errors.Is(err, common.ErrStakingNotFound) {
		return nil, err
	}

	totalStaked := int64(0)
	if staking != nil {
		totalStaked = staking.GetTotalStaked()
	}

	return &models.EconomicsResponse{
		InitialSupply:           kdaData.GetInitialSupply(),
		MaxSupply:               kdaData.GetMaxSupply(),
		MintedValue:             kdaData.GetMintedValue(),
		BurnedValue:             kdaData.GetBurnedValue(),
		CirculatingSupply:       kdaData.GetCirculatingSupply(),
		TotalStaked:             totalStaked,
		PendingRewardsTotal:     pendingRewardsTotal,
		MarketEscrowTotal:       marketEscrowTotal,
		FeesPoolKLVTotal:        feesPoolKLVTotal,
		FPRPoolTotal:            fprPoolKLVTotal,
		AccumulatedFeesTotal:    accumulatedFeesTotal,
		SystemAccountKLVBalance: systemAccountKLV,
	}, nil
}

// loadAccumulatedFeesTotal sums KLV fees accrued per validator (reset at epoch end into PREW/Allowance).
// GetLatestPeers returns nil during early sync or on a peer-read error, so this reads 0 there, not an error.
func (n *Node) loadAccumulatedFeesTotal() int64 {
	total := int64(0)
	for _, peer := range n.validatorsProvider.GetLatestPeers() {
		if check.IfNil(peer) {
			continue
		}
		total += peer.GetAccumulatedFees()
	}
	return total
}

// scanKAppDataTrie invokes accumulate with each stored value (GetStorage trims the trie tail). Keys are
// collected before reading values so the scan goroutine finishes first.
func (n *Node) scanKAppDataTrie(address []byte, accumulate func(value []byte) error) error {
	app, err := n.loadKAppAccount(address)
	if err != nil {
		return err
	}

	dataTrie := app.DataTrie()
	if check.IfNil(dataTrie) {
		return nil
	}
	leavesChannel, err := dataTrie.GetAllLeavesOnChannel(app.GetRootHash(), context.Background())
	if err != nil {
		return err
	}

	keys := make([][]byte, 0)
	for leaf := range leavesChannel {
		key := make([]byte, len(leaf.Key()))
		copy(key, leaf.Key())
		keys = append(keys, key)
	}

	for _, key := range keys {
		raw := app.GetStorage(key)
		if len(raw) == 0 {
			continue
		}
		if err := accumulate(raw); err != nil {
			return err
		}
	}
	return nil
}

// loadFeesPoolKLVTotal sums KLVBalance across every KDA fees-pool record (KLV sits in that field).
func (n *Node) loadFeesPoolKLVTotal() (int64, error) {
	total := int64(0)
	err := n.scanKAppDataTrie(kapps.KDAFeesPoolKAppAddress, func(raw []byte) error {
		pool := &kdafeespool.KDAFeesPoolData{}
		if err := n.internalMarshalizer.Unmarshal(pool, raw); err != nil {
			return err
		}
		total += pool.KLVBalance
		return nil
	})
	return total, err
}

// loadSystemAccountKLV returns the KLV held directly by the system-account KApp.
func (n *Node) loadSystemAccountKLV() (int64, error) {
	app, err := n.loadKAppAccount(kapps.SystemAccountKAppAddress)
	if err != nil {
		return 0, err
	}
	// GetUserKDA returns zero (not error) when KLV isn't held, so a real error must propagate.
	userKDA, err := app.GetUserKDA(kdautils.KLVIdentifier, nil, false)
	if err != nil {
		return 0, err
	}
	return userKDA.GetBalance(), nil
}

// loadFPRPoolKLVTotal sums unclaimed KLV staking rewards across every StakingData: CurrentFPRAmount +
// Σ FPR(TotalAmount − TotalClaimed). KLV-denominated only; non-KLV rewards (the KDAS map) are excluded.
func (n *Node) loadFPRPoolKLVTotal() (int64, error) {
	total := int64(0)
	err := n.scanKAppDataTrie(kapps.StakingKAppAddress, func(raw []byte) error {
		staking := &kapps.StakingData{}
		if err := n.internalMarshalizer.Unmarshal(staking, raw); err != nil {
			return err
		}
		total += staking.GetCurrentFPRAmount()
		for _, fpr := range staking.GetFPR() {
			if unclaimed := fpr.GetTotalAmount() - fpr.GetTotalClaimed(); unclaimed > 0 {
				total += unclaimed
			}
		}
		return nil
	})
	return total, err
}

func (n *Node) loadKAppAccount(address []byte) (state.KAppAccountHandler, error) {
	acnt, err := n.kapps.LoadAccount(address)
	if err != nil {
		return nil, err
	}
	app, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return nil, common.ErrWrongTypeAssertion
	}
	return app, nil
}
