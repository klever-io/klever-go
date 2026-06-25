package node

import (
	"context"
	"errors"
	"sync"

	"github.com/klever-io/klever-go/common"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools/check"
)

// economicsCache memoizes GetEconomics per block, capping the KApp trie scans (PREW, market
// orders, fees pools, FPR pool) to at most once per block so a polling client can't trigger one
// per request.
type economicsCache struct {
	mu     sync.Mutex
	nonce  uint64
	valid  bool
	cached *models.EconomicsResponse
}

// GetEconomics returns live KLV supply figures plus node-state held aggregates, memoized per
// block. It runs O(n) KApp trie scans, so the `/network/economics` route is opt-in (disabled by
// default in api.yaml). The returned value is cached and shared across callers within a block,
// so treat it as read-only. See KLC-2506.
func (n *Node) GetEconomics() (*models.EconomicsResponse, error) {
	// Lock spans the nonce read + compute so the cache check/update stays atomic and
	// concurrent callers on a fresh block share one scan.
	n.economics.mu.Lock()
	defer n.economics.mu.Unlock()

	currentNonce := uint64(0)
	if header := n.blkc.GetCurrentBlockHeader(); !check.IfNil(header) {
		currentNonce = header.GetNonce()
	}

	if n.economics.valid && n.economics.nonce == currentNonce && n.economics.cached != nil {
		return n.economics.cached, nil
	}

	economics, err := n.computeEconomics()
	if err != nil {
		return nil, err
	}

	n.economics.cached = economics
	n.economics.nonce = currentNonce
	n.economics.valid = true

	return economics, nil
}

// computeEconomics reads live supply plus held aggregates from node state: PREW and market escrow
// (Validators/Market KApp scans), the KDA fees-pool KLV, the FPR staking pool, and the
// system-account KLV. Use GetEconomics (cached) instead of calling this directly.
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
		SystemAccountKLVBalance: systemAccountKLV,
	}, nil
}

// scanKAppDataTrie loads the KApp at address and invokes accumulate with each stored value (the
// trie tail is trimmed by GetStorage). Keys are collected first so the scan goroutine finishes
// before the values are read back.
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

// loadFeesPoolKLVTotal sums KLVBalance across every KDA fees-pool record (KLV is held as that
// field, not the account balance — see kdaFeesPool deposit).
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
	// GetUserKDA returns a zero-value (balance 0) when the asset isn't held; a non-nil error here
	// is a real trie/unmarshal failure, so propagate it rather than report a silent zero.
	userKDA, err := app.GetUserKDA(kdautils.KLVIdentifier, nil, false)
	if err != nil {
		return 0, err
	}
	return userKDA.GetBalance(), nil
}

// loadFPRPoolKLVTotal sums the unclaimed KLV staking rewards across every asset's StakingData in
// the Staking KApp: CurrentFPRAmount (this epoch) + Σ FPR(TotalAmount − TotalClaimed) (past epochs).
// TotalAmount/CurrentFPRAmount are KLV-denominated; non-KLV rewards live in the FPR KDAS map and are
// excluded. This mints into circulatingSupply, so it must be read at the same block to cancel.
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
