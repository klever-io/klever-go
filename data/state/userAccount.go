//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. userAccountData.proto
package state

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ UserAccountHandler = (*userAccount)(nil)

// Account is the struct used in serialization/deserialization
type userAccount struct {
	*baseAccount
	UserAccountData
}

// NewEmptyUserAccount creates new simple account wrapper for an AccountContainer (that has just been initialized)
func NewEmptyUserAccount() *userAccount {
	return &userAccount{
		baseAccount: &baseAccount{},
		UserAccountData: UserAccountData{
			Balance: 0,
		},
	}
}

// NewUserAccount creates new simple account wrapper for an AccountContainer (that has just been initialized)
func NewUserAccount(address []byte) (*userAccount, error) {
	if len(address) == 0 {
		return nil, common.ErrNilAddress
	}

	return &userAccount{
		baseAccount: &baseAccount{
			address:         address,
			dataTrieTracker: NewTrackableDataTrie(address, nil),
		},
		UserAccountData: UserAccountData{
			Address: address,
		},
	}, nil
}

// NewUserAccountNoCheck creates new simple account wrapper for an AccountContainer (that has just been initialized) wihtou check address
func NewUserAccountNoCheck(address []byte) *userAccount {
	return &userAccount{
		baseAccount: &baseAccount{
			address:         address,
			dataTrieTracker: NewTrackableDataTrie(address, nil),
		},
		UserAccountData: UserAccountData{
			Address: address,
		},
	}
}

// IncreaseNonce adds the given value to the current nonce
func (a *userAccount) IncreaseNonce(value uint64) {
	a.Nonce = a.Nonce + value
}

// SetName sets the users name
func (a *userAccount) SetName(name []byte) {
	a.Name = make([]byte, 0, len(name))
	a.Name = append(a.Name, name...)
}

// SetRootHash sets the root hash associated with the account
func (a *userAccount) SetRootHash(roothash []byte) {
	a.RootHash = roothash
}

func (a *userAccount) HasNewCode() bool {
	return a.hasNewCode
}

// GetUserKDA returns the unmarshalled kda data for the given key
func (a *userAccount) GetUserKDA(assetID []byte, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
	kdaData := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	if check.IfNil(a.dataTrieTracker.DataTrie()) &&
		(!checkDirtData || len(a.dataTrieTracker.DirtyData()) == 0) {
		return kdaData, nil
	}

	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}

	// check assetID for NFT
	key := kdautils.ToKDAKey(assetID, nonce)

	value, err := a.dataTrieTracker.RetrieveValue(key)
	if err != nil && err != common.ErrNilTrie {
		return nil, err
	}
	if len(value) == 0 {
		return kdaData, nil
	}

	marshalizer := marshal.NewProtoMarshalizer()

	err = marshalizer.Unmarshal(kdaData, value)
	if err != nil {
		return nil, err
	}

	return kdaData, nil
}

// SetUserKDA returns the unmarshalled kda data for the given key
func (a *userAccount) SetUserKDA(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
	if assetID == nil {
		assetID = kdautils.KLVIdentifier
	}
	marshalizer := marshal.NewProtoMarshalizer()

	data, err := marshalizer.Marshal(userKDA)
	if err != nil {
		return err
	}

	key := kdautils.ToKDAKey(assetID, nonce)

	// delete trie key if data is empty
	if len(data) == 0 {
		return a.dataTrieTracker.SaveKeyValue(key, nil)
	}

	return a.dataTrieTracker.SaveKeyValue(key, data)
}

// AddToAllowance adds new value to allowance
func (a *userAccount) AddToAllowance(value int64) error {
	if value < 0 {
		return ErrInvalidValue
	}

	a.Allowance += value

	return nil
}

// isCompoundAssetID reports whether assetID holds the key separator. No KDA ID does (tickers are
// A-Z0-9), so such an ID can only name an NFT or SFT unit (COLL/1): the fungible balance of it would
// read and write that unit's key (KLC-2816, KLC-2876). Ungated: on mainnet no order or market
// transaction ever used one, and every other path loads its asset ID with GetKDA.
func isCompoundAssetID(assetID []byte) bool {
	return bytes.Contains(assetID, []byte(kapps.Sp))
}

// AddToBalance adds new value to balance
func (a *userAccount) AddToBalance(value int64, assetID []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrInvalidValue
	}

	if isCompoundAssetID(assetID) {
		return ErrCompoundAssetID
	}

	if assetID == nil || bytes.Equal(assetID, kdautils.KLVIdentifier) {
		newBalance := a.Balance + value
		if newBalance < 0 {
			return ErrAddBalanceOverflow
		}

		a.Balance = newBalance
		return nil
	}

	var userKDA *kapps.UserKDA
	var err error
	if len(userKDAopts) == 1 {
		userKDA = userKDAopts[0]
	}

	if userKDA == nil {
		userKDA, err = a.GetUserKDA(assetID, nil, cdd)
		if err != nil {
			return err
		}
	}

	newBalance := userKDA.Balance + value
	if newBalance < 0 {
		return ErrAddBalanceOverflow
	}
	userKDA.Balance = newBalance

	return a.SetUserKDA(assetID, nil, userKDA)
}

// AddToBalance adds new value to balance
func (a *userAccount) AddToBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrInvalidValue
	}

	// nothing to do here
	if value == 0 {
		return nil
	}

	// parse and check nonce
	internalIDInt, err := strconv.ParseInt(string(nonce), 10, 64)
	if err != nil {
		return err
	}

	if internalIDInt <= 0 {
		return ErrInvalidNonce
	}

	var userKDA *kapps.UserKDA
	if len(userKDAopts) == 1 {
		userKDA = userKDAopts[0]
	}

	if userKDA == nil {
		userKDA, err = a.GetUserKDA(assetID, nonce, cdd)
		if err != nil {
			return err
		}
	}

	newBalance := userKDA.Balance + value
	if newBalance < 0 {
		return ErrAddBalanceOverflow
	}
	userKDA.Balance = newBalance

	return a.SetUserKDA(assetID, nonce, userKDA)
}

// SubFromBalance subtracts new value from balance
func (a *userAccount) SubFromBalance(value int64, assetID []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrInvalidValue
	}

	if isCompoundAssetID(assetID) {
		return ErrCompoundAssetID
	}

	if assetID == nil || bytes.Equal(assetID, kdautils.KLVIdentifier) {
		newBalance := a.Balance - value
		if newBalance < 0 {
			return ErrInsufficientFunds
		}

		a.Balance = newBalance
		return nil
	}

	var userKDA *kapps.UserKDA
	var err error
	if len(userKDAopts) == 1 {
		userKDA = userKDAopts[0]
	}

	if userKDA == nil {
		userKDA, err = a.GetUserKDA(assetID, nil, cdd)
		if err != nil {
			return err
		}
	}

	newBalance := userKDA.Balance - value
	if newBalance < 0 {
		return ErrInsufficientFunds
	}
	userKDA.Balance = newBalance

	return a.SetUserKDA(assetID, nil, userKDA)
}

// AddToBalance adds new value to balance, clear key if balance == 0
func (a *userAccount) SubFromBalanceWithNonce(value int64, assetID []byte, nonce []byte, cdd bool, userKDAopts ...*kapps.UserKDA) error {
	if value < 0 {
		return ErrInvalidValue
	}

	// parse and check nonce
	internalIDInt, err := strconv.ParseInt(string(nonce), 10, 64)
	if err != nil {
		return err
	}

	if internalIDInt <= 0 {
		return ErrInvalidNonce
	}

	var userKDA *kapps.UserKDA
	if len(userKDAopts) == 1 {
		userKDA = userKDAopts[0]
	}

	if userKDA == nil {
		userKDA, err = a.GetUserKDA(assetID, nonce, cdd)
		if err != nil {
			return err
		}
	}

	newBalance := userKDA.Balance - value
	if newBalance < 0 {
		return ErrAddBalanceOverflow
	}
	userKDA.Balance = newBalance

	return a.SetUserKDA(assetID, nonce, userKDA)
}

// AddInternalKDA adds new internal KDA to user
func (a *userAccount) AddInternalKDA(assetID []byte, internalID []byte, data []byte) error {
	if len(data) == 0 {
		return common.ErrAssetNotFound
	}

	key := kdautils.ToKDAKey(assetID, internalID)

	return a.dataTrieTracker.SaveKeyValue(key, data)
}

// SubInternalKDA clears the user's NFT holder record internalID of assetID and returns it. From
// FixAuditChangesV7 a record that does not hold the NFT (see kapps.UserKDA.IsHeldNFT) is refused
// before it is cleared, so the refusal leaves it as it was (KLC-2835).
func (a *userAccount) SubInternalKDA(assetID []byte, internalID []byte, forks core.ForkController) ([]byte, error) {
	key := kdautils.ToKDAKey(assetID, internalID)

	data, err := a.dataTrieTracker.RetrieveValue(key)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || forks.FixAuditChangesV7() && !IsHeldNFTRecord(data) {
		return nil, common.ErrAssetNotFound
	}

	err = a.dataTrieTracker.SaveKeyValue(key, nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// IsHeldNFTRecord reports whether data, a user's NFT holder record, holds the NFT (see
// kapps.UserKDA.IsHeldNFT). A record that does not decode does not hold it.
func IsHeldNFTRecord(data []byte) bool {
	userKDA := &kapps.UserKDA{}
	if err := marshal.NewProtoMarshalizer().Unmarshal(userKDA, data); err != nil {
		return false
	}

	return userKDA.IsHeldNFT()
}

// GetBalance returns the actual balance from the account
func (a *userAccount) GetBalance(assetID []byte, cdd bool) int64 {
	if assetID == nil || bytes.Equal(assetID, kdautils.KLVIdentifier) {
		return a.Balance
	}

	return a.GetBalanceWithNonce(assetID, nil, cdd)
}

// GetBalance returns the actual balance from the account
func (a *userAccount) GetBalanceWithNonce(assetID []byte, nonce []byte, cdd bool) int64 {
	userKDA, err := a.GetUserKDA(assetID, nonce, cdd)
	if err != nil {
		return 0
	}

	return userKDA.Balance
}

// GetAllowance returns account allowance to withdraw
func (a *userAccount) GetAllowance() int64 {
	return a.Allowance
}

// GetBalance returns the actual balance from the account
func (a *userAccount) GetFrozenBalance(assetID []byte, cdd bool) int64 {
	userKDA, err := a.GetUserKDA(assetID, nil, cdd)
	if err != nil {
		return 0
	}

	return userKDA.FrozenBalance
}

// GetBucket -
func (a *userAccount) GetBuckets(assetID []byte, cdd bool) map[string]*kapps.UserBucket {
	userKDA, err := a.GetUserKDA(assetID, nil, cdd)
	if err != nil {
		return nil
	}

	return userKDA.Buckets
}

// Freeze -
func (a *userAccount) Freeze(
	assetID []byte,
	bucketID []byte,
	value int64,
	staking *kapps.StakingData,
	userKDA *kapps.UserKDA,
	opts FreezeOptions,
) error {
	if value <= 0 {
		return ErrInvalidValue
	}

	if userKDA.Buckets == nil {
		userKDA.Buckets = make(map[string]*kapps.UserBucket)
	}

	// must use string to marshal proto map due UTF8 issue
	parsedBucketID := hex.EncodeToString(bucketID)
	if !bytes.Equal(assetID, kdautils.KLVIdentifier) && !bytes.Equal(assetID, kdautils.KFIIdentifier) {
		parsedBucketID = string(assetID)
	}

	oldValue := int64(0)
	toAdd := value
	var history []*kapps.StakeSegment
	if userKDA.Buckets[parsedBucketID] != nil {
		bucket := userKDA.Buckets[parsedBucketID]
		oldValue = bucket.Value
		if opts.NewStakingFlow && bucket.UnstakedEpoch != core.DefaultUnstakedEpoch {
			toAdd += oldValue
		}

		if opts.KeepStakeHistory {
			history = stakeHistoryAfterFreeze(bucket, opts.BlockEpoch, userKDA.LastClaim.GetEpoch())
		}
	}

	userKDA.Buckets[parsedBucketID] = &kapps.UserBucket{
		StakedAt:      opts.BlockTime,
		StakedEpoch:   opts.BlockEpoch,
		UnstakedEpoch: core.DefaultUnstakedEpoch,
		Value:         oldValue + value,
		Delegation:    nil,
		History:       history,
	}

	userKDA.FrozenBalance += toAdd
	staking.TotalStaked += toAdd

	return nil
}

// stakeHistoryAfterFreeze records what the bucket had staked before this freeze rewrote it, so an
// ordinary claim can still price the reward epochs that stake earned.
//
// Segments already covered by LastClaim are dropped, so the list is bounded by the claim cursor
// rather than by the number of top-ups.
func stakeHistoryAfterFreeze(bucket *kapps.UserBucket, epoch, lastClaimEpoch uint32) []*kapps.StakeSegment {
	history := make([]*kapps.StakeSegment, 0, len(bucket.History)+1)

	for _, segment := range bucket.History {
		// Each segment carries its own end, so a claim cursor past that end retires it without
		// consulting its neighbours.
		if segment.ThroughEpoch > lastClaimEpoch {
			history = append(history, segment)
		}
	}

	// The stake being replaced was live until this freeze, or until the unfreeze that preceded
	// it -- an unfreeze at U still earns pool U, which is where the interval closes.
	through := epoch
	if bucket.UnstakedEpoch != core.DefaultUnstakedEpoch {
		through = bucket.UnstakedEpoch
	}

	// StakedEpoch == through leaves the empty interval (through, through], which no pool epoch
	// can fall in; through <= lastClaimEpoch is already paid. Both same-epoch cases -- a top-up
	// and an unfreeze/re-freeze inside one epoch -- fall out of the first condition.
	if bucket.StakedEpoch < through && lastClaimEpoch < through {
		history = append(history, &kapps.StakeSegment{
			StakedEpoch:  bucket.StakedEpoch,
			ThroughEpoch: through,
			Value:        bucket.Value,
		})
	}

	if len(history) == 0 {
		return nil
	}

	return history
}

// Unfreeze -
func (a *userAccount) Unfreeze(
	assetID []byte,
	bucketID []byte,
	blockEpoch uint32,
	staking *kapps.StakingData,
	userKDA *kapps.UserKDA,
	newStakingFlow bool,
) ([]byte, int64, error) {
	if userKDA.Buckets == nil {
		return nil, 0, ErrNotStaked
	}

	// must use string to marshal proto map due UTF8 issue
	parsedBucketID := hex.EncodeToString(bucketID)
	if !bytes.Equal(assetID, kdautils.KLVIdentifier) && !bytes.Equal(assetID, kdautils.KFIIdentifier) {
		parsedBucketID = string(assetID)
	}

	if userKDA.Buckets[parsedBucketID] == nil {
		return nil, 0, ErrNotStaked
	}

	if userKDA.Buckets[parsedBucketID].UnstakedEpoch != core.DefaultUnstakedEpoch ||
		blockEpoch-userKDA.Buckets[parsedBucketID].StakedEpoch < staking.MinEpochsToUnstake {
		return nil, 0, ErrUnstakeNotAvailable
	}
	userKDA.Buckets[parsedBucketID].UnstakedEpoch = blockEpoch

	if newStakingFlow && staking.InterestType == kapps.StakingData_APRI {
		staking.TotalStaked -= userKDA.FrozenBalance
		userKDA.FrozenBalance = 0
	} else {
		userKDA.FrozenBalance -= userKDA.Buckets[parsedBucketID].Value
		staking.TotalStaked -= userKDA.Buckets[parsedBucketID].Value
	}

	return userKDA.Buckets[parsedBucketID].Delegation, userKDA.Buckets[parsedBucketID].Value, nil
}

// Delegate -
func (a *userAccount) Delegate(bucketID []byte, delegation []byte, userKDA *kapps.UserKDA) (int64, error) {
	if userKDA.Buckets == nil {
		return 0, ErrNotStaked
	}

	// must use string to marshal proto map due UTF8 issue
	parsedBucketID := hex.EncodeToString(bucketID)
	if userKDA.Buckets[parsedBucketID] == nil {
		return 0, ErrNotStaked
	}

	userKDA.Buckets[parsedBucketID].Delegation = delegation

	return userKDA.Buckets[parsedBucketID].Value, nil
}

// Undelegate -
func (a *userAccount) Undelegate(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
	if userKDA.Buckets == nil {
		return nil, 0, ErrNotStaked
	}

	// must use string to marshal proto map due UTF8 issue
	parsedBucketID := hex.EncodeToString(bucketID)

	if userKDA.Buckets[parsedBucketID] == nil {
		return nil, 0, ErrNotStaked
	}

	if userKDA.Buckets[parsedBucketID].Delegation == nil {
		return nil, 0, ErrUndelegatingNotAvailable
	}

	delegation := userKDA.Buckets[parsedBucketID].Delegation

	userKDA.Buckets[parsedBucketID].Delegation = nil

	return delegation, userKDA.Buckets[parsedBucketID].Value, nil
}

func (a *userAccount) Withdraw(assetID []byte, epoch uint32, minEpochsToWithdraw uint32, userKDA *kapps.UserKDA) (int64, error) {
	withdraw := int64(0)
	for key, bucket := range userKDA.Buckets {
		if uint64(epoch) >= uint64(bucket.UnstakedEpoch)+uint64(minEpochsToWithdraw) {
			withdraw += bucket.Value
			delete(userKDA.Buckets, key)
		}
	}

	if withdraw == 0 {
		return 0, ErrWithdrawNotAvailable
	}

	if bytes.Equal(assetID, kdautils.KLVIdentifier) {
		a.Balance += withdraw
	} else {
		userKDA.Balance += withdraw
	}

	err := a.SetUserKDA(assetID, nil, userKDA)
	if err != nil {
		return 0, err
	}

	return withdraw, nil
}

// ClaimStaking -
func (a *userAccount) Claim(claimType transaction.ClaimContract_EnumClaimType,
	assetID []byte, epoch uint32, blockTime int64,
	staking *kapps.StakingData, kda *kapps.KDAData,
	userKDA *kapps.UserKDA,
	forkController core.ForkController,
) (map[string]int64, error) {
	switch claimType {
	case transaction.ClaimContract_StakingClaim:
		if isAPRStakingWithV6(staking, forkController) {
			return a.claimAPRStaking(assetID, epoch, blockTime, userKDA, staking, kda, forkController)
		}

		amountToAdd, err := a.claimStaking(assetID, epoch, blockTime, userKDA, staking, forkController)
		if err != nil {
			return nil, err
		}

		// APR interest gains should try mint and return error if reach max supply
		if staking != nil && staking.InterestType == kapps.StakingData_APRI {
			if kda.MaxSupply > 0 && (kda.CirculatingSupply+amountToAdd[string(assetID)]) > kda.MaxSupply {
				return nil, common.ErrMaxSupplyExceeded
			}
			kda.CirculatingSupply += amountToAdd[string(assetID)]
			kda.MintedValue += amountToAdd[string(assetID)]
		}

		return amountToAdd, nil
	case transaction.ClaimContract_AllowanceClaim:
		if !bytes.Equal(assetID, kdautils.KLVIdentifier) {
			return nil, common.ErrAssetIDInvalid
		}

		return a.claimAllowance()
	case transaction.ClaimContract_MarketClaim:
		return nil, nil
	}

	return nil, ErrInvalidClaimType
}

// ComputeAvailableClaim -
func (a *userAccount) ComputeAvailableClaim(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	switch staking.InterestType {
	case kapps.StakingData_APRI:
		rewards, err := a.computeClaimAPR(assetID, epoch, blockTime, userKDA, staking, forkController)
		if err != nil {
			return nil, err
		}
		return rewards, nil
	case kapps.StakingData_FPRI:
		rewards, err := a.computeClaimFPR(assetID, epoch, blockTime, userKDA, staking, forkController)
		if err != nil {
			return nil, err
		}
		return rewards, nil
	}

	return nil, ErrInvalidStakeType
}

// ClaimStaking -
func (a *userAccount) claimStaking(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	amountToAdd, err := a.ComputeAvailableClaim(assetID, epoch, blockTime, userKDA, staking, forkController)
	if err != nil {
		return nil, err
	}

	return amountToAdd, nil
}

func (a *userAccount) computeClaimAPR(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	if userKDA.LastClaim == nil || userKDA.LastClaim.Timestamp == 0 {
		return nil, nil
	}

	if userKDA.Buckets == nil || userKDA.Buckets[string(assetID)] == nil {
		return nil, nil
	}

	amount := userKDA.Buckets[string(assetID)].Value
	unstaked := userKDA.Buckets[string(assetID)].UnstakedEpoch != core.DefaultUnstakedEpoch
	if unstaked {
		return nil, nil
	}

	if amount == 0 {
		return nil, nil
	}

	// if APR restrict minEpochToClaim, user will not be able to freeze more than once
	// until the claim period has passed, removing this option for now
	//
	// if epoch-userKDA.LastClaim.Epoch < staking.MinEpochsToClaim {
	// 	return nil, ErrClaimNotAvailable
	// }

	currAPR := int64(0)
	gains := &aprGains{values: make(map[string]int64), key: string(assetID), forkController: forkController}
	lastClaim := userKDA.LastClaim.Timestamp

	for _, apr := range staking.APR {
		if blockTime <= apr.Timestamp {
			break
		}

		if apr.Timestamp > lastClaim {
			gains.add(lastClaim, apr.Timestamp, amount, currAPR)
			lastClaim = apr.Timestamp
		}

		currAPR = int64(apr.Value)
	}

	if currAPR > 0 {
		gains.add(lastClaim, blockTime, amount, currAPR)
	}

	return gains.result()
}

type aprGains struct {
	values         map[string]int64
	key            string
	forkController core.ForkController
	err            error
}

func (g *aprGains) add(initTime, endTime, amount, currAPR int64) {
	if g.err != nil {
		return
	}

	gain, err := computeAPR(initTime, endTime, amount, currAPR, g.forkController)
	if err != nil {
		g.err = err
		return
	}

	if !g.forkController.FixAuditChangesV6() {
		g.values[g.key] += gain
		return
	}

	if gain < 0 {
		g.err = common.ErrNegativeValue
		return
	}

	g.values[g.key], g.err = tools.SafeAddI64(g.values[g.key], gain)
}

func (g *aprGains) result() (map[string]int64, error) {
	if g.err != nil {
		return nil, g.err
	}

	return g.values, nil
}

func isAPRStakingWithV6(staking *kapps.StakingData, forkController core.ForkController) bool {
	return forkController.FixAuditChangesV6() && staking != nil && staking.InterestType == kapps.StakingData_APRI
}

func (a *userAccount) claimAPRStaking(assetID []byte, epoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, kda *kapps.KDAData, forkController core.ForkController) (map[string]int64, error) {
	gains, err := a.computeClaimAPR(assetID, epoch, blockTime, userKDA, staking, forkController)
	reason := APRGainDropReason(staking, kda, gains, assetID, err, forkController)
	if reason != nil {
		return nil, fmt.Errorf("%w: %w", ErrAPRGainDropped, reason)
	}
	if err != nil {
		return nil, err
	}

	addAPRGainToSupply(kda, gains[string(assetID)])

	return gains, nil
}

// APRGainDropReason returns why, once FixAuditChangesV6 is active, an APR claim cannot credit the accrued gain, or nil when it can
func APRGainDropReason(staking *kapps.StakingData, kda *kapps.KDAData, gains map[string]int64, assetID []byte, computeErr error, forkController core.ForkController) error {
	if !isAPRStakingWithV6(staking, forkController) {
		return nil
	}

	if errors.Is(computeErr, common.ErrInt64Overflow) {
		return common.ErrInt64Overflow
	}

	gain := gains[string(assetID)]
	if computeErr != nil || gain <= 0 {
		return nil
	}

	if kda.GetCirculatingSupply() < 0 {
		return ErrNegativeCirculatingSupply
	}

	if kda.GetMaxSupply() > 0 && gain > kda.GetMaxSupply()-kda.GetCirculatingSupply() {
		return common.ErrMaxSupplyExceeded
	}

	_, err := tools.SafeAddI64(kda.GetCirculatingSupply(), gain)

	return err
}

func addAPRGainToSupply(kda *kapps.KDAData, gain int64) {
	kda.CirculatingSupply += gain

	if kda.MintedValue > math.MaxInt64-gain {
		kda.MintedValue = math.MaxInt64
		return
	}

	kda.MintedValue += gain
}

func computeAPR(initTime, endTime, amount, currAPR int64, forkController core.ForkController) (int64, error) {
	if !forkController.BigBucketsCompute() {
		return int64(float64((endTime-initTime)*amount*currAPR)/float64(core.OneYearTimestamp)) / int64(core.HundredPercent), nil
	}

	v := big.NewFloat(float64(endTime - initTime))
	v = v.Mul(v, big.NewFloat(float64(amount)))
	v = v.Mul(v, big.NewFloat(float64(currAPR)))
	v = v.Quo(v, big.NewFloat(float64(core.HundredPercent)))
	v = v.Quo(v, big.NewFloat(float64(core.OneYearTimestamp)))

	if forkController.FixAuditChangesV6() && v.Cmp(big.NewFloat(math.MaxInt64)) >= 0 {
		return 0, common.ErrInt64Overflow
	}

	intV, _ := v.Int64()

	return intV, nil
}

// bucketStakeAt reports the value the bucket had staked when fprEpoch was
// distributed. Before FixAuditChangesV5 only the current stake is visible, so a
// top-up that moves StakedEpoch forward hides every pool at or below it; from
// the fork on, the segments recorded at each top-up keep those pools priced at
// the value that actually earned them.
func bucketStakeAt(bucket *kapps.UserBucket, fprEpoch uint32, forkController core.ForkController) (int64, bool) {
	if bucket.StakedEpoch < fprEpoch {
		return bucket.Value, true
	}

	if !forkController.FixAuditChangesV5() {
		return 0, false
	}

	for _, segment := range bucket.History {
		if segment.StakedEpoch < fprEpoch && fprEpoch <= segment.ThroughEpoch {
			return segment.Value, true
		}
	}

	return 0, false
}

func (a *userAccount) computeClaimFPR(assetID []byte, blockEpoch uint32, blockTime int64, userKDA *kapps.UserKDA, staking *kapps.StakingData, forkController core.ForkController) (map[string]int64, error) {
	if userKDA.Buckets == nil || userKDA.LastClaim == nil {
		return nil, nil
	}

	lastClaimEpoch := userKDA.LastClaim.Epoch

	if blockEpoch-lastClaimEpoch < uint32(staking.MinEpochsToClaim) {
		return nil, ErrClaimNotAvailable
	}

	gains := make(map[string]int64)
	for _, fpr := range staking.FPR {
		if fpr.Epoch <= lastClaimEpoch || fpr.Epoch > blockEpoch {
			continue
		}

		for _, bucket := range userKDA.Buckets {
			if err := a.accrueBucketFPR(gains, fpr, bucket, forkController); err != nil {
				return nil, err
			}
		}
	}

	return gains, nil
}

func (a *userAccount) accrueBucketFPR(gains map[string]int64, fpr *kapps.FPRData, bucket *kapps.UserBucket, forkController core.ForkController) error {
	stakedValue, staked := bucketStakeAt(bucket, fpr.Epoch, forkController)
	if !staked {
		return nil
	}

	if !forkController.BigBucketsCompute() {
		// sanity test...
		if stakedValue > fpr.TotalStaked {
			return ErrInconsistentStakingData
		}
	}

	if !bucketEligibleForFPR(bucket, fpr.Epoch, forkController) {
		return nil
	}

	if fpr.TotalAmount > 0 {
		kdaID := string(kdautils.KLVIdentifier)
		if !forkController.ClaimKFI() {
			kdaID = string(kdautils.KFIIdentifier)
		}

		intAmount := a.calculateFPRAmount(fpr.TotalAmount, fpr.TotalStaked, stakedValue, fpr.TotalClaimed, forkController)
		gains[kdaID] += intAmount
		fpr.TotalClaimed += intAmount
	}

	for key, value := range fpr.KDAS {
		intAmount := a.calculateFPRAmount(value.TotalAmount, fpr.TotalStaked, stakedValue, value.TotalClaimed, forkController)
		gains[key] += intAmount
		fpr.KDAS[key].TotalClaimed += intAmount
	}

	return nil
}

func bucketEligibleForFPR(bucket *kapps.UserBucket, fprEpoch uint32, forkController core.ForkController) bool {
	if bucket.UnstakedEpoch == core.DefaultUnstakedEpoch {
		return true
	}

	if forkController.FixAuditChangesV3() {
		return fprEpoch <= bucket.UnstakedEpoch
	}

	return !forkController.FixStakingBuckets()
}

func (a *userAccount) calculateFPRAmount(totalAmount, totalStaked, bucketValue, totalClaimed int64, forkController core.ForkController) int64 {
	if !forkController.BigBucketsCompute() {
		intAmount := int64(float64(totalAmount) / float64(totalStaked) * float64(bucketValue))
		return intAmount
	}

	if totalAmount <= 0 ||
		totalStaked <= 0 {
		return 0
	}

	v := big.NewFloat(float64(totalAmount))
	v = v.Quo(v, big.NewFloat(float64(totalStaked)))
	v = v.Mul(v, big.NewFloat(float64(bucketValue)))
	intAmount, _ := v.Int64()

	if totalClaimed+intAmount > totalAmount {
		intAmount = totalAmount - totalClaimed
	}

	if forkController.FPRComputeAndKdaFeeFlow() &&
		intAmount < 0 {
		intAmount = 0
	}

	return intAmount
}

func (a *userAccount) claimAllowance() (map[string]int64, error) {
	value := a.Allowance
	a.Allowance = 0

	return map[string]int64{string(kdautils.KLVIdentifier): value}, nil
}

// GetPermission retrive permission by ID
func (a *userAccount) GetPermission(id int32) (*Permission, bool, error) {
	if len(a.Permissions) == 0 {
		if id == 0 {
			return &Permission{
				ID:         0,
				Type:       Permission_Owner,
				Threshold:  1,
				Operations: make([]byte, 0),
				Signers: []*Key{
					{
						Address: a.address,
						Weight:  1,
					},
				},
			}, false, nil
		}
		return nil, false, ErrInvalidPermissionID
	}

	for _, p := range a.Permissions {
		if p.ID == id {
			return p, true, nil
		}
	}

	return nil, false, ErrInvalidPermissionID
}

// SetPermissions update account permissions
func (a *userAccount) SetPermissions(permissions []*Permission) {
	a.Permissions = permissions
}

// SetOwnerAddress sets the owner address of an account,
func (a *userAccount) SetOwnerAddress(address []byte) {
	a.OwnerAddress = address
}

// SetCodeHash sets the code hash associated with the account
func (a *userAccount) SetCodeHash(codeHash []byte) {
	a.CodeHash = codeHash
}

func (a *userAccount) GetCodeHash() []byte {
	return a.CodeHash
}

// SetCodeMetadata sets the code metadata
func (a *userAccount) SetCodeMetadata(codeMetadata []byte) {
	a.CodeMetadata = codeMetadata
}

// Equal checks if two accounts are equal
func (a *userAccount) Equal(toCompare UserAccountHandler) bool {
	toCompareUserAccount, ok := toCompare.(*userAccount)
	if !ok {
		return false
	}

	reflectOk := reflect.DeepEqual(a.baseAccount, toCompareUserAccount.baseAccount)
	if !reflectOk {
		return false
	}

	p1, err := proto.Marshal(&a.UserAccountData)
	if err != nil {
		return false
	}
	p2, err := proto.Marshal(&toCompareUserAccount.UserAccountData)
	if err != nil {
		return false
	}

	return bytes.Equal(p1, p2)
}

func (x *Permission) CheckPermissionGrantedForContracts(contractTypes ...transaction.TXContract_ContractType) bool {
	if x == nil {
		return false
	}

	if x.Type == Permission_Owner {
		return true
	}

	return transaction.CheckPermissionGrantedForContracts(x.Operations, contractTypes...)
}

func (x *Permission) CheckPermissionGrantedForUint64(ops uint64) bool {
	if x == nil {
		return false
	}

	if x.Type == Permission_Owner {
		return true
	}

	permissionUint64 := tools.BytesToUint64LE(x.Operations)

	// Check if all requested operations are permitted
	return (ops & ^permissionUint64) == 0
}
