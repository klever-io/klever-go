//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. peerAccountData.proto
package state

import (
	"github.com/klever-io/klever-go/common"
)

var _ PeerAccountHandler = (*peerAccount)(nil)

// PeerAccount is the struct used in serialization/deserialization
type peerAccount struct {
	*baseAccount
	PeerAccountData
}

// NewEmptyPeerAccount returns an empty peerAccount
func NewEmptyPeerAccount() *peerAccount {
	return &peerAccount{
		baseAccount: &baseAccount{},
		PeerAccountData: PeerAccountData{
			ValidatorSuccessRate:      &SignRate{},
			LeaderSuccessRate:         &SignRate{},
			TotalValidatorSuccessRate: &SignRate{},
			TotalLeaderSuccessRate:    &SignRate{},
		},
	}
}

// NewPeerAccount creates new simple account wrapper for an PeerAccountContainer (that has just been initialized)
func NewPeerAccount(address []byte) (*peerAccount, error) {
	if len(address) == 0 {
		return nil, common.ErrNilAddress
	}

	return &peerAccount{
		baseAccount: &baseAccount{
			address:         address,
			dataTrieTracker: NewTrackableDataTrie(address, nil),
		},
		PeerAccountData: PeerAccountData{
			ValidatorSuccessRate:      &SignRate{},
			LeaderSuccessRate:         &SignRate{},
			TotalValidatorSuccessRate: &SignRate{},
			TotalLeaderSuccessRate:    &SignRate{},
		},
	}, nil
}

// SetBLSPublicKey sets the account's bls public key, saving the old key before changing
func (pa *peerAccount) GetNonce() uint64 {
	return 0
}

// SetBLSPublicKey sets the account's bls public key, saving the old key before changing
func (pa *peerAccount) SetBLSPublicKey(pubKey []byte) error {
	if len(pubKey) < 1 {
		return common.ErrNilBLSPublicKey
	}

	pa.BLSPublicKey = pubKey
	return nil
}

// SetRootHash sets the root hash associated with the account
func (pa *peerAccount) SetRootHash(roothash []byte) {
	pa.RootHash = roothash
}

// SetOwnerAddress sets the account's owner address, saving the old address before changing
func (pa *peerAccount) SetOwnerAddress(address []byte) error {
	if len(address) < 1 {
		return common.ErrEmptyAddress
	}

	pa.OwnerAddress = address
	return nil
}

// SetRevoked revoke a claimed peer account
func (pa *peerAccount) SetRevoked() {
	pa.Revoked = true
}

// IncreaseNonce adds the given value to the current nonce
func (pa *peerAccount) IncreaseNonce(value uint64) {
	log.Error("increase peer account nonce", "blsPublicKey", pa.BLSPublicKey)
}

func (pa *peerAccount) GetLeaderSuccessRateSuccess() uint32 {
	return pa.LeaderSuccessRate.NumSuccess
}

func (pa *peerAccount) GetTotalLeaderSuccessRateSuccess() uint32 {
	return pa.TotalLeaderSuccessRate.NumSuccess
}

func (pa *peerAccount) GetValidatorSuccessRateSuccess() uint32 {
	return pa.ValidatorSuccessRate.NumSuccess
}

func (pa *peerAccount) GetTotalValidatorSuccessRateSuccess() uint32 {
	return pa.TotalValidatorSuccessRate.NumSuccess
}

func (pa *peerAccount) GetLeaderSuccessRateFailure() uint32 {
	return pa.LeaderSuccessRate.NumFailure
}

func (pa *peerAccount) GetTotalLeaderSuccessRateFailure() uint32 {
	return pa.TotalLeaderSuccessRate.NumFailure
}

func (pa *peerAccount) GetValidatorSuccessRateFailure() uint32 {
	return pa.ValidatorSuccessRate.NumFailure
}

func (pa *peerAccount) GetTotalValidatorSuccessRateFailure() uint32 {
	return pa.TotalValidatorSuccessRate.NumFailure
}

func (pa *peerAccount) GetListString() string {
	return pa.List.String()
}

// ResetAtNewEpoch will reset a set of values after changing epoch
func (pa *peerAccount) ResetAtNewEpoch() {
	pa.AccumulatedFees = 0
	pa.SetRating(pa.GetTempRating())
	pa.TotalLeaderSuccessRate.NumFailure += pa.LeaderSuccessRate.NumFailure
	pa.TotalLeaderSuccessRate.NumSuccess += pa.LeaderSuccessRate.NumSuccess
	pa.TotalValidatorSuccessRate.NumSuccess += pa.ValidatorSuccessRate.NumSuccess
	pa.TotalValidatorSuccessRate.NumFailure += pa.ValidatorSuccessRate.NumFailure
	pa.TotalValidatorIgnoredSignaturesRate += pa.ValidatorIgnoredSignaturesRate
	pa.LeaderSuccessRate.NumFailure = 0
	pa.LeaderSuccessRate.NumSuccess = 0
	pa.ValidatorSuccessRate.NumSuccess = 0
	pa.ValidatorSuccessRate.NumFailure = 0
	pa.ValidatorIgnoredSignaturesRate = 0
	pa.NumSelectedInSuccessBlocks = 0
}

func (pa *peerAccount) AddToAccumulatedFees(value int64) {
	pa.AccumulatedFees += value
}

// IncreaseLeaderSuccessRate increases the account's number of successful signing
func (pa *peerAccount) IncreaseLeaderSuccessRate(value uint32) {
	pa.LeaderSuccessRate.NumSuccess += value
}

// DecreaseLeaderSuccessRate increases the account's number of missing signing
func (pa *peerAccount) DecreaseLeaderSuccessRate(value uint32) {
	pa.LeaderSuccessRate.NumFailure += value
}

// IncreaseValidatorSuccessRate increases the account's number of successful signing
func (pa *peerAccount) IncreaseValidatorSuccessRate(value uint32) {
	pa.ValidatorSuccessRate.NumSuccess += value
}

// DecreaseValidatorSuccessRate increases the account's number of missed signing
func (pa *peerAccount) DecreaseValidatorSuccessRate(value uint32) {
	pa.ValidatorSuccessRate.NumFailure += value
}

// IncreaseValidatorIgnoredSignaturesRate increases the account's number of ignored signatures in successful blocks
func (pa *peerAccount) IncreaseValidatorIgnoredSignaturesRate(value uint32) {
	pa.ValidatorIgnoredSignaturesRate += value
}

// IncreaseNumSelectedInSuccessBlocks sets the account's NumSelectedInSuccessBlocks
func (pa *peerAccount) IncreaseNumSelectedInSuccessBlocks() {
	pa.NumSelectedInSuccessBlocks++
}

// SetList -
func (pa *peerAccount) SetList(list List) {
	pa.List = list
}

// SetListAndIndex -
func (pa *peerAccount) SetListAndIndex(list List, index uint32) {
	pa.List = list
	pa.Index = index
}

// SetRating -
func (pa *peerAccount) SetRating(rating uint32) {
	pa.Rating = rating
}

// SetTempRating -
func (pa *peerAccount) SetTempRating(tempRating uint32) {
	pa.TempRating = tempRating
}

// SetConsecutiveProposerMisses -
func (pa *peerAccount) SetConsecutiveProposerMisses(num uint32) {
	pa.ConsecutiveProposerMisses = num
}

func (pa *peerAccount) CopyFrom(oldHandler PeerAccountHandler) error {
	old, ok := oldHandler.(*peerAccount)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	pa.ShardId = old.ShardId
	pa.List = old.List
	pa.Index = old.Index
	pa.AccumulatedFees = old.AccumulatedFees
	pa.ValidatorSuccessRate = &SignRate{
		NumSuccess: old.ValidatorSuccessRate.NumSuccess, NumFailure: old.ValidatorSuccessRate.NumFailure,
	}
	pa.LeaderSuccessRate = &SignRate{
		NumSuccess: old.LeaderSuccessRate.NumSuccess, NumFailure: old.LeaderSuccessRate.NumFailure,
	}
	pa.ValidatorIgnoredSignaturesRate = old.ValidatorIgnoredSignaturesRate
	pa.Rating = old.Rating
	pa.TempRating = old.TempRating
	pa.NumSelectedInSuccessBlocks = old.NumSelectedInSuccessBlocks
	pa.ConsecutiveProposerMisses = old.ConsecutiveProposerMisses
	pa.TotalValidatorSuccessRate = &SignRate{
		NumSuccess: old.TotalValidatorSuccessRate.NumSuccess, NumFailure: old.TotalValidatorSuccessRate.NumFailure,
	}
	pa.TotalLeaderSuccessRate = &SignRate{
		NumSuccess: old.TotalLeaderSuccessRate.NumSuccess, NumFailure: old.TotalLeaderSuccessRate.NumFailure,
	}
	pa.TotalValidatorIgnoredSignaturesRate = old.TotalValidatorIgnoredSignaturesRate

	return nil
}

// IsInterfaceNil return if there is no value under the interface
func (pa *peerAccount) IsInterfaceNil() bool {
	return pa == nil
}
