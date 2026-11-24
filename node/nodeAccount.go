package node

import (
	"context"
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
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
		if errors.Is(err, common.ErrAccNotFound) {
			return state.NewUserAccount(addr)
		}
		return nil, fmt.Errorf("could not fetch sender address from provided param: %w", err)
	}

	account, ok := n.castAccountToUserAccount(accWrp)
	if !ok {
		return nil, errors.New("this is not an user account address")
	}

	// V2 Epoch Rewards: Add pending rewards to allowance for API response
	// Before fork activation, this returns 0 (no overhead impact on result)
	if !check.IfNil(n.kappController) {
		pendingRewards, err := n.kappController.GetValidatorsKApp().GetPendingRewards(account.AddressBytes())
		if err != nil {
			log.Warn("Failed to get pending rewards for account", "address", address, "error", err)
		} else if pendingRewards > 0 {
			if err := account.AddToAllowance(pendingRewards); err != nil {
				log.Warn("Failed to add pending rewards to allowance", "address", address, "error", err)
			}
		}
	}

	return account, nil
}

// GetNextNonce will return account next nonce
func (n *Node) GetNextNonce(address string) (uint64, uint64, uint64, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return 0, 0, 0, common.ErrNilPubkeyConverter
	}
	// pending nonces build on the live nonce, on purpose: the committed one lags the block in progress
	if check.IfNil(n.processingAccounts) {
		return 0, 0, 0, common.ErrNilAccountsAdapter
	}

	addr, err := n.addressPubkeyConverter.Decode(address)
	if err != nil {
		return 0, 0, 0, err
	}

	accWrp, err := n.processingAccounts.GetExistingAccount(addr)
	if err != nil {
		if errors.Is(err, common.ErrAccNotFound) {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, fmt.Errorf("could not fetch sender address from provided param: %w", err)
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

	return userAccount.GetBalance([]byte(kda), n.forkController.EnableSmartContracts()), nil
}

// GetUserKDA returns user KDA for a specific asset in an account
func (n *Node) GetUserKDA(address string, assetId string) (*kapps.UserKDA, error) {
	account, err := n.getAccountHandler(address)
	if err != nil {
		return nil, err
	}

	userAccount, ok := n.castAccountToUserAccount(account)
	if !ok {
		return nil, nil
	}

	return userAccount.GetUserKDA([]byte(assetId), nil, n.forkController.EnableSmartContracts())
}

// loadStakingData loads and unmarshals staking data for a given asset.
func (n *Node) loadStakingData(assetId string) (*kapps.StakingData, error) {
	stakingKapp, err := n.loadKAppAccount(kapps.StakingKAppAddress)
	if err != nil {
		return nil, err
	}

	return n.readStakingData(stakingKapp, assetId)
}

// readStakingData unmarshals staking data for a given asset from an already loaded staking kapp.
func (n *Node) readStakingData(stakingKapp state.KAppAccountHandler, assetId string) (*kapps.StakingData, error) {
	key := kdautils.ToKDAKey([]byte(assetId), nil)
	stakedBytes, err := stakingKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(stakedBytes) == 0 {
		return nil, common.ErrStakingNotFound
	}

	kdaStaking := &kapps.StakingData{}
	if err = n.internalMarshalizer.Unmarshal(kdaStaking, stakedBytes); err != nil {
		return nil, err
	}

	return kdaStaking, nil
}

// readKDAData unmarshals KDA data for a given asset from an already loaded KDA kapp.
func (n *Node) readKDAData(kdaKapp state.KAppAccountHandler, assetId string) (*kapps.KDAData, error) {
	key := kdautils.ToKDAKey([]byte(assetId), nil)
	kdaBytes, err := kdaKapp.DataTrieTracker().RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(kdaBytes) == 0 {
		return nil, common.ErrAssetNotFound
	}

	kda := &kapps.KDAData{}
	if err = n.internalMarshalizer.Unmarshal(kda, kdaBytes); err != nil {
		return nil, err
	}

	return kda, nil
}

// getKLVAllowanceWithPending returns the KLV allowance including pending rewards from V2 epoch rewards.
func (n *Node) getKLVAllowanceWithPending(userAccount state.UserAccountHandler) int64 {
	allowance := userAccount.GetAllowance()

	if check.IfNil(n.kappController) {
		return allowance
	}

	pendingRewards, err := n.kappController.GetValidatorsKApp().GetPendingRewards(userAccount.AddressBytes())
	if err != nil {
		log.Warn("Failed to get pending rewards for account", "address", userAccount.AddressBytes(), "error", err)
	} else if pendingRewards > 0 {
		allowance += pendingRewards
	}

	return allowance
}

// computeRewardsForAsset extracts the computed rewards for the given asset from the rewards map.
func computeRewardsForAsset(rewards map[string]int64, assetId string) int64 {
	if len(rewards) == 0 {
		return 0
	}
	// if assetID is KFI, computedRewards must be in KLV stack
	if assetId == string(kdautils.KFIIdentifier) {
		assetId = string(kdautils.KLVIdentifier)
	}
	return rewards[assetId]
}

// availableClaimState holds the request-invariant state an available-claim lookup reads.
// Loading it once per request keeps a multi-asset lookup at a constant number of
// accounts-adapter lock acquisitions instead of three per asset.
type availableClaimState struct {
	userAccount state.UserAccountHandler
	stakingKapp state.KAppAccountHandler
	kdaKapp     state.KAppAccountHandler
}

// loadAvailableClaimState loads the user account and the staking and KDA kapps. A nil state
// with a nil error means the address does not hold a user account. Each of the three loads takes
// an accounts-adapter lock, so ctx is checked before each one.
func (n *Node) loadAvailableClaimState(ctx context.Context, address string) (*availableClaimState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	account, err := n.getAccountHandler(address)
	if err != nil {
		return nil, err
	}

	userAccount, ok := n.castAccountToUserAccount(account)
	if !ok {
		return nil, nil
	}

	if err = ctx.Err(); err != nil {
		return nil, err
	}

	stakingKapp, err := n.loadKAppAccount(kapps.StakingKAppAddress)
	if err != nil {
		return nil, err
	}

	if err = ctx.Err(); err != nil {
		return nil, err
	}

	kdaKapp, err := n.loadKAppAccount(kapps.KDAKAppAddress)
	if err != nil {
		return nil, err
	}

	return &availableClaimState{
		userAccount: userAccount,
		stakingKapp: stakingKapp,
		kdaKapp:     kdaKapp,
	}, nil
}

// computeAvailableClaim computes the available claim for one asset from the preloaded state.
// KLV also reads the pending rewards, which loads the Validators kapp from the kapps adapter: a
// fourth adapter lock on top of the three the state load takes, so ctx is checked before it.
func (n *Node) computeAvailableClaim(ctx context.Context, claimState *availableClaimState, assetId string) (*models.AvailableClaimResponse, error) {
	kdaStaking, err := n.readStakingData(claimState.stakingKapp, assetId)
	if err != nil {
		return nil, err
	}

	kda, err := n.readKDAData(claimState.kdaKapp, assetId)
	if err != nil {
		return nil, err
	}

	userAccount := claimState.userAccount
	userKDA, err := userAccount.GetUserKDA([]byte(assetId), nil, n.forkController.EnableSmartContracts())
	if err != nil {
		return nil, err
	}

	// Read the header per asset, after the asset's data, so it is as recent as possible.
	header := n.blkc.GetCurrentBlockHeader()
	if check.IfNil(header) {
		return nil, common.ErrNilHeader
	}

	var rewards map[string]int64
	if !isClaimInFlight(userKDA, header) {
		rewards, err = userAccount.ComputeAvailableClaim(
			[]byte(assetId),
			header.GetEpoch(),
			header.GetTimestamp(),
			userKDA,
			kdaStaking,
			n.forkController,
		)
		if state.APRGainDropReason(kdaStaking, kda, rewards, []byte(assetId), err, n.forkController) != nil {
			rewards, err = nil, nil
		}
		if err != nil && !errors.Is(err, state.ErrClaimNotAvailable) {
			return nil, err
		}
	}

	allowance := int64(0)
	if assetId == "KLV" {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		allowance = n.getKLVAllowanceWithPending(userAccount)
	}

	return &models.AvailableClaimResponse{
		StakingRewards:    computeRewardsForAsset(rewards, assetId),
		AllStakingRewards: rewards,
		Allowance:         allowance,
	}, nil
}

// isClaimInFlight reports whether the user's last claim is newer than the current header. The
// data tries are shared with block processing, which writes a block's state before it publishes
// that block's header (at the end of CommitBlock), so a claim in the block being executed can be
// visible while the header is still the previous block's. Nothing is claimable then, and the claim
// maths would run backwards (negative APR rewards), so the preview reports zero instead.
func isClaimInFlight(userKDA *kapps.UserKDA, header data.HeaderHandler) bool {
	lastClaim := userKDA.GetLastClaim()
	if lastClaim == nil {
		return false
	}

	return lastClaim.GetTimestamp() > header.GetTimestamp() || lastClaim.GetEpoch() > header.GetEpoch()
}

// GetAvailableClaim returns the rewards available for a specific asset in an account
func (n *Node) GetAvailableClaim(address string, assetId string) (int64, map[string]int64, int64, error) {
	claimState, err := n.loadAvailableClaimState(context.Background(), address)
	if err != nil {
		return 0, nil, 0, err
	}
	if claimState == nil {
		return 0, nil, 0, nil
	}

	claim, err := n.computeAvailableClaim(context.Background(), claimState, assetId)
	if err != nil {
		return 0, nil, 0, err
	}

	return claim.StakingRewards, claim.AllStakingRewards, claim.Allowance, nil
}

// GetAvailableClaimList returns the rewards available for each of the given assets in an account.
// The account and kapp state is loaded once and shared by every asset, and the assets are
// computed sequentially so the shared account handlers are never read from more than one goroutine.
// A request-level failure is returned as err; per-asset failures are returned keyed by asset.
// ctx is checked before each state load, before each asset and before the pending-rewards load
// of KLV, so a request that was cancelled or ran past its deadline stops taking accounts-adapter
// locks and fails as a whole with ctx's error. A lookup already blocked on a lock is not
// interrupted; it is the next check that ends the request.
func (n *Node) GetAvailableClaimList(ctx context.Context, address string, assetIds []string) (map[string]*models.AvailableClaimResponse, map[string]error, error) {
	claimState, err := n.loadAvailableClaimState(ctx, address)
	if err != nil {
		return nil, nil, err
	}

	claims := make(map[string]*models.AvailableClaimResponse, len(assetIds))
	claimErrors := make(map[string]error)
	if claimState == nil {
		for _, assetId := range assetIds {
			claims[assetId] = &models.AvailableClaimResponse{}
		}
		return claims, claimErrors, nil
	}

	for _, assetId := range assetIds {
		if err = ctx.Err(); err != nil {
			return nil, nil, err
		}

		claim, errClaim := n.computeAvailableClaim(ctx, claimState, assetId)
		if errClaim != nil {
			// a context that ended inside the asset fails the request, not just this asset
			if err = ctx.Err(); err != nil {
				return nil, nil, err
			}
			claimErrors[assetId] = errClaim
			continue
		}
		claims[assetId] = claim
	}

	return claims, claimErrors, nil
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
