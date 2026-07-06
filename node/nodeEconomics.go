package node

import (
	"bytes"
	"context"
	"errors"
	"math"

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

	pendingRewardsTotal, marketEscrowTotal, err := n.loadKAppHeldTotals()
	if err != nil {
		return nil, err
	}

	feesPoolKLVTotal, systemAccountKLV, fprPoolKLVTotal, err := n.loadKAppKLVPools()
	if err != nil {
		return nil, err
	}

	accumulatedFeesTotal, err := n.loadValidatorAccumulatedFees()
	if err != nil {
		return nil, err
	}

	totalStaked, err := n.loadTotalStakedKLV()
	if err != nil {
		return nil, err
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

// loadKAppHeldTotals returns the pending-rewards and market-escrow totals (zero without a controller).
func (n *Node) loadKAppHeldTotals() (int64, int64, error) {
	if check.IfNil(n.kappController) {
		return 0, 0, nil
	}
	pendingRewards, err := n.kappController.GetValidatorsKApp().GetPendingRewardsTotal()
	if err != nil {
		return 0, 0, err
	}
	marketEscrow, err := n.kappController.GetMarketKApp().GetMarketEscrowTotal()
	if err != nil {
		return 0, 0, err
	}
	return pendingRewards, marketEscrow, nil
}

// loadKAppKLVPools returns the fees-pool, system-account, and FPR-pool KLV totals (zero without a
// KApp accounts adapter).
func (n *Node) loadKAppKLVPools() (int64, int64, int64, error) {
	if check.IfNil(n.kapps) {
		return 0, 0, 0, nil
	}
	feesPool, err := n.loadFeesPoolKLVTotal()
	if err != nil {
		return 0, 0, 0, err
	}
	systemAccount, err := n.loadSystemAccountKLV()
	if err != nil {
		return 0, 0, 0, err
	}
	fprPool, err := n.loadFPRPoolKLVTotal()
	if err != nil {
		return 0, 0, 0, err
	}
	return feesPool, systemAccount, fprPool, nil
}

// loadValidatorAccumulatedFees returns the accrued validator fees (zero without peer statistics).
func (n *Node) loadValidatorAccumulatedFees() (int64, error) {
	if check.IfNil(n.validatorStatistics) {
		return 0, nil
	}
	return n.loadAccumulatedFeesTotal()
}

// loadTotalStakedKLV returns KLV TotalStaked; a missing StakingData record (fresh/test networks)
// means no stake, not an error.
func (n *Node) loadTotalStakedKLV() (int64, error) {
	staking, err := n.loadStakingData(string(kdautils.KLVIdentifier))
	if errors.Is(err, common.ErrStakingNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return staking.GetTotalStaked(), nil
}

// loadAccumulatedFeesTotal sums KLV fees accrued per validator (reset at epoch end into PREW/Allowance).
// Returns 0 before the first finalized block (no peers yet); a peer-read error is propagated, not hidden.
func (n *Node) loadAccumulatedFeesTotal() (int64, error) {
	rootHash := n.validatorStatistics.LastFinalizedRootHash()
	if len(rootHash) == 0 {
		return 0, nil
	}
	peers, err := n.validatorStatistics.ListPeerAccounts(rootHash)
	if err != nil {
		return 0, err
	}
	total := int64(0)
	for _, peer := range peers {
		if check.IfNil(peer) {
			continue
		}
		addGuarded(&total, peer.GetAccumulatedFees(), "loadAccumulatedFeesTotal")
	}
	return total, nil
}

// addGuarded adds amount into total, skipping negative or overflowing values so a corrupt stored
// entry cannot poison an authoritative supply figure.
func addGuarded(total *int64, amount int64, context string) {
	if amount < 0 {
		log.Warn(context + ": negative amount, skipping entry")
		return
	}
	if *total > math.MaxInt64-amount {
		log.Warn(context + ": sum would overflow int64, skipping entry")
		return
	}
	*total += amount
}

// scanKAppDataTrie invokes accumulate with each stored value whose key matches prefix (nil scans all;
// GetStorage trims the trie tail). Keys are collected before reading values so the scan goroutine
// finishes first. Mid-walk trie errors are swallowed upstream, so a truncated walk yields a silently
// undercounted total (KLC-2509).
func (n *Node) scanKAppDataTrie(address []byte, prefix []byte, accumulate func(value []byte) error) error {
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
		if len(prefix) > 0 && !bytes.HasPrefix(leaf.Key(), prefix) {
			continue
		}
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
// Fees-pool records are keyed by raw asset ID with no shared prefix, so the scan takes every leaf.
func (n *Node) loadFeesPoolKLVTotal() (int64, error) {
	total := int64(0)
	err := n.scanKAppDataTrie(kapps.KDAFeesPoolKAppAddress, nil, func(raw []byte) error {
		pool := &kdafeespool.KDAFeesPoolData{}
		if err := n.internalMarshalizer.Unmarshal(pool, raw); err != nil {
			return err
		}
		addGuarded(&total, pool.KLVBalance, "loadFeesPoolKLVTotal")
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
	// StakingData records are keyed via ToKDAKey, so only KDA-prefixed leaves are decoded.
	prefix := []byte(kapps.KDAPrefix + kapps.Sp)
	err := n.scanKAppDataTrie(kapps.StakingKAppAddress, prefix, func(raw []byte) error {
		staking := &kapps.StakingData{}
		if err := n.internalMarshalizer.Unmarshal(staking, raw); err != nil {
			return err
		}
		addGuarded(&total, staking.GetCurrentFPRAmount(), "loadFPRPoolKLVTotal")
		for _, fpr := range staking.GetFPR() {
			if unclaimed := fpr.GetTotalAmount() - fpr.GetTotalClaimed(); unclaimed > 0 {
				addGuarded(&total, unclaimed, "loadFPRPoolKLVTotal")
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
