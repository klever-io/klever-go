package validators

import (
	"bytes"
	"encoding/hex"
	"math"
	"unicode/utf8"

	"github.com/bugsnag/bugsnag-go/v2"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.ValidatorsKapp = (*validatorsKApp)(nil)

const (
	VALIDATOR_PREFIX     = "VAL"
	VALIDATOR_S_PREFIX   = "VALS"
	VALIDATOR_BLS_PREFIX = "BLS"
	VALIDATOR_BUCKETS    = "VALB"
)

type validatorActionType uint8

const (
	unknownAction             validatorActionType = 0
	leaderSuccess             validatorActionType = 1
	leaderFail                validatorActionType = 2
	validatorSuccess          validatorActionType = 3
	validatorIgnoredSignature validatorActionType = 4
)

var log = logger.GetOrCreate("kapp/validator")

type validatorsKApp struct {
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	ratingsData    process.RatingsInfoHandler
	rater          sharding.PeerAccountListAndRatingHandler
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewValidatorKApp holds the arguments needed to create a ValidatorsKApp
type ArgsNewValidatorKApp struct {
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
	RatingsData    process.RatingsInfoHandler
}

// NewValidatorKApp creates a validator KApp
func NewValidatorKApp(
	args *ArgsNewValidatorKApp,
) (*validatorsKApp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.RatingsData) {
		return nil, common.ErrNilRater
	}

	v := &validatorsKApp{
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		ratingsData:    args.RatingsData,
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

func (v *validatorsKApp) SetKAppController(controller kapp.KAppController) error {
	v.KAppController = controller

	return nil
}

func (v *validatorsKApp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	v.accountsCacher = cacher

	return nil
}

func (v *validatorsKApp) GetAccountsCacher() state.AccountsCacher {
	return v.accountsCacher
}

func (v *validatorsKApp) SetRater(rater sharding.PeerAccountListAndRatingHandler) {
	v.rater = rater
}

func (v *validatorsKApp) getKApp() (state.KAppAccountHandler, error) {
	kapp, err := v.accountsCacher.LoadKApp(kapps.ValidatorsKAppAddress)
	if err != nil {
		return nil, err
	}

	return kapp, nil
}

func (v *validatorsKApp) saveKApp(app state.KAppAccountHandler) error {
	return v.accountsCacher.UpdateKapp(app)
}

func (v *validatorsKApp) validatorKey(address []byte) []byte {
	return append([]byte(VALIDATOR_PREFIX+kapps.Sp), address...)
}

func (v *validatorsKApp) validatorBucketsKey(address []byte) []byte {
	return append([]byte(VALIDATOR_BUCKETS+kapps.Sp), address...)
}

func (v *validatorsKApp) GetValidatorsInfo(validators [][]byte) ([]kapp.ValidatorAccountInfoHandler, error) {
	vList := make([]kapp.ValidatorAccountInfoHandler, len(validators))

	app, err := v.getKApp()
	if err != nil {
		return nil, err
	}

	for i, owner := range validators {
		// get val
		vd, err := v.getValidator(app, owner)
		if err != nil {
			return nil, err
		}
		// get stats
		peerAcc, err := v.loadPeerAccount(vd.BlsPubKey)
		if err != nil {
			return nil, err
		}

		vList[i] = &ValidatorAccountInfo{vd, peerAcc}
	}

	return vList, nil
}

func (v *validatorsKApp) getValidator(app state.KAppAccountHandler, sender []byte) (*ValidatorData, error) {
	// Check if account already registered as validator
	vKey := v.validatorKey(sender)
	data := app.GetStorage(vKey)
	if len(data) == 0 {
		return nil, common.ErrValidatorNotFound
	}

	val := &ValidatorData{}

	err := v.marshalizer.Unmarshal(val, data)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (v *validatorsKApp) setValidator(app state.KAppAccountHandler, address []byte, val *ValidatorData) error {
	vKey := v.validatorKey(address)
	data, err := v.marshalizer.Marshal(val)
	if err != nil {
		return err
	}

	return app.SetStorage(vKey, data)
}

// Register
func (v *validatorsKApp) Register(tc *transaction.CreateValidatorContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	// Check validator info
	if len(tc.GetOwnerAddress()) != v.addressLen {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if len(tc.GetConfig().GetRewardAddress()) != v.addressLen {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	commission := tc.GetConfig().GetCommission()
	if commission > core.HundredPercent {
		return transaction.Transaction_CommissionTooHigh, common.ErrInvalidValue
	}

	maxDelegationAmount := tc.GetConfig().GetMaxDelegationAmount()
	if maxDelegationAmount < 0 {
		return transaction.Transaction_DelegationAmountInvalid, common.ErrInvalidValue
	}

	if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	for key, uri := range tc.GetConfig().GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}

	if !utf8.ValidString(tc.GetConfig().GetName()) ||
		len(tc.GetConfig().GetName()) > core.MaxNameSize {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	// load Kapp Accoount
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// Check if account already registered as validator
	vKey := v.validatorKey(tc.GetOwnerAddress())
	data := app.GetStorage(vKey)
	if len(data) > 0 {
		return transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	peerAcc, err := v.loadPeerAccount(tc.GetConfig().GetBLSPublicKey())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// verify if BLS key is been used as validator...
	if len(peerAcc.GetOwnerAddress()) > 0 {
		return transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	// check if bls has been revoked
	if v.forkController.FixStakingBuckets() && peerAcc.GetRevoked() {
		return transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	err = peerAcc.SetOwnerAddress(tc.GetOwnerAddress())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	err = peerAcc.SetBLSPublicKey(tc.GetConfig().GetBLSPublicKey())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	err = v.accountsCacher.UpdatePeer(peerAcc)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	val := &ValidatorData{
		OwnerAddress:   tc.GetOwnerAddress(),
		RewardsAddress: tc.Config.GetRewardAddress(),
		RegisterNonce:  ctx.Block().GetNonce(),
		BlsPubKey:      tc.GetConfig().GetBLSPublicKey(),
		JailedEpoch:    math.MaxUint32,
		CanDelegate:    tc.GetConfig().GetCanDelegate(),
		MaxDelegation:  tc.GetConfig().GetMaxDelegationAmount(),
		Commission:     tc.GetConfig().GetCommission(),
		Name:           tc.GetConfig().GetName(),
		Logo:           tc.GetConfig().GetLogo(),
		URIs:           tc.GetConfig().GetURIs(),
	}

	if ctx.Block().GetNonce() == 0 {
		peerAcc.SetListAndIndex(state.List_elected, uint32(ctx.Block().GetNonce()))
	} else {
		peerAcc.SetListAndIndex(state.List_inactive, uint32(ctx.Block().GetNonce()))
	}

	peerAcc.SetRating(v.ratingsData.StartRating())
	peerAcc.SetTempRating(v.ratingsData.StartRating())

	err = v.accountsCacher.UpdatePeer(peerAcc)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	err = v.setValidator(app, tc.GetOwnerAddress(), val)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	err = v.saveKApp(app)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateValidator,
		ctx.ContractID(),
		tc.GetOwnerAddress(),
	))

	return transaction.Transaction_Ok, nil
}

// UpdateValidator
func (v *validatorsKApp) UpdateValidator(sender []byte, tc *transaction.ValidatorConfigContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	if len(tc.GetConfig().GetRewardAddress()) != 0 &&
		len(tc.GetConfig().GetRewardAddress()) != v.addressLen {
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	commission := tc.GetConfig().GetCommission()
	if commission > core.HundredPercent {
		return transaction.Transaction_CommissionTooHigh, common.ErrInvalidValue
	}

	maxDelegationAmount := tc.GetConfig().GetMaxDelegationAmount()
	if maxDelegationAmount < 0 {
		return transaction.Transaction_DelegationAmountInvalid, common.ErrInvalidValue
	}

	// load Kapp Accoount
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// Check if account already registered as validator
	val, err := v.getValidator(app, sender)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// check if BLS Key matchs current
	if len(tc.GetConfig().GetBLSPublicKey()) > 0 &&
		!bytes.Equal(val.BlsPubKey, tc.GetConfig().GetBLSPublicKey()) {

		// Reset old peer
		peerAccOld, err := v.revokPeerAccount(val.BlsPubKey)
		if err != nil {
			_ = bugsnag.Notify(err)
			return transaction.Transaction_InvalidPeerKey, err
		}

		peerAcc, err := v.loadPeerAccount(tc.GetConfig().GetBLSPublicKey())
		if err != nil {
			return transaction.Transaction_InvalidPeerKey, err
		}

		if !bytes.Equal(peerAcc.GetBLSPublicKey(), tc.GetConfig().GetBLSPublicKey()) {
			err = peerAcc.SetBLSPublicKey(tc.GetConfig().GetBLSPublicKey())
			if err != nil {
				return transaction.Transaction_InvalidPeerKey, err
			}

			if v.forkController.FixStakingBuckets() {
				// copy info from old peer
				err = peerAcc.CopyFrom(peerAccOld)
				if err != nil {
					return transaction.Transaction_InvalidPeerKey, err
				}

				if peerAcc.GetList() == state.List_elected {
					peerAcc.SetRating(0)
					peerAcc.SetTempRating(0)
					peerAcc.SetList(state.List_jailed)
				}
			}
		}

		if peerAcc.GetRevoked() {
			return transaction.Transaction_InvalidPeerKey, common.ErrBLSKeyRevoked
		}

		// check inconsistences in peerAcc
		if len(peerAcc.GetOwnerAddress()) > 0 &&
			!bytes.Equal(sender, peerAcc.GetOwnerAddress()) {
			return transaction.Transaction_InvalidPeerKey, common.ErrAccountValidatorNotOwner
		}

		// Set New BLS Key
		err = peerAcc.SetOwnerAddress(sender)
		if err != nil {
			return transaction.Transaction_InvalidPeerKey, err
		}

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return transaction.Transaction_Fail, err
		}

		// Update BLS in validator account
		val.BlsPubKey = tc.Config.GetBLSPublicKey()
	}

	if len(tc.GetConfig().GetRewardAddress()) > 0 {
		val.RewardsAddress = tc.GetConfig().GetRewardAddress()
	}

	if len(tc.GetConfig().GetLogo()) > 0 {
		if !utf8.ValidString(tc.GetConfig().GetLogo()) || len(tc.GetConfig().GetLogo()) > core.MaxLogoURISize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		val.Logo = tc.GetConfig().GetLogo()
	}

	if len(tc.GetConfig().GetURIs()) > 0 {
		if len(tc.GetConfig().GetURIs()) > core.MaxURIMapSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		for key, uri := range tc.GetConfig().GetURIs() {
			if !utf8.ValidString(key) ||
				!utf8.ValidString(uri) ||
				len(key) > core.MaxURIKeySize ||
				len(uri) > core.MaxURIValueSize {
				return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
			}
		}

		val.URIs = tc.GetConfig().GetURIs()
	}

	if len(tc.GetConfig().GetName()) > 0 {
		if !utf8.ValidString(tc.GetConfig().GetName()) ||
			len(tc.GetConfig().GetName()) > core.MaxNameSize {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}

		val.Name = tc.GetConfig().GetName()
	}

	val.CanDelegate = tc.GetConfig().GetCanDelegate()
	val.Commission = tc.GetConfig().GetCommission()
	val.MaxDelegation = tc.GetConfig().GetMaxDelegationAmount()

	err = v.setValidator(app, sender, val)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	err = v.saveKApp(app)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateValidator,
		ctx.ContractID(),
		sender,
	))

	return transaction.Transaction_Ok, nil
}

func (v *validatorsKApp) getExistingAccountFromAddress(adrSrc []byte) (state.UserAccountHandler, error) {
	userAcc, err := v.accountsCacher.GetExistingUser(adrSrc)
	if err != nil {
		return nil, err
	}

	return userAcc, nil
}

func (v *validatorsKApp) undelegate(
	app state.KAppAccountHandler,
	blockEpoch uint32,
	validator []byte,
	sender []byte,
	bucketID []byte,
) error {
	val, err := v.getValidator(app, validator)
	if err != nil {
		return err
	}

	pd, err := v.getValidatorBuckets(app, validator)
	if err != nil {
		return err
	}

	// must use string to marshal proto map due UTF8 issue
	encodedBucketID := hex.EncodeToString(bucketID)

	b, ok := pd.Buckets[encodedBucketID]
	if !ok ||
		// already unstaked
		b.UndelegatedEpoch != core.DefaultUndelegatedEpoch {
		return common.ErrInvalidValue
	}

	b.UndelegatedEpoch = blockEpoch
	// remove bucket from validator if its a recent delegation
	if v.forkController.FixDelegationSameEpoch() {
		if b.DelegatedEpoch == blockEpoch {
			delete(pd.Buckets, encodedBucketID)
		}
	}

	err = v.setValidatorBuckets(app, validator, pd)
	if err != nil {
		return err
	}

	val.TotalStake -= b.Value
	if bytes.Equal(validator, sender) || bytes.Equal(val.OwnerAddress, sender) {
		val.SelfStake -= b.Value
	}

	return v.setValidator(app, validator, val)
}

func (v *validatorsKApp) delegate(
	app state.KAppAccountHandler,
	blockTime int64,
	blockEpoch uint32,
	validator []byte,
	sender []byte,
	bucketID []byte,
	amountDelegation int64,
) error {
	val, err := v.getValidator(app, validator)
	if err != nil {
		return err
	}

	pd, err := v.getValidatorBuckets(app, validator)
	if err != nil {
		return err
	}

	// must use string to marshal proto map due UTF8 issue
	encodedBucketID := hex.EncodeToString(bucketID)

	pd.Buckets[encodedBucketID] = &PeerBucket{
		DelegatedAt:      blockTime,
		DelegatedEpoch:   blockEpoch,
		UndelegatedEpoch: core.DefaultUndelegatedEpoch,
		Value:            amountDelegation,
		Address:          sender,
	}

	err = v.setValidatorBuckets(app, validator, pd)
	if err != nil {
		return err
	}

	val.TotalStake += amountDelegation
	if bytes.Equal(validator, sender) || bytes.Equal(val.OwnerAddress, sender) {
		val.SelfStake += amountDelegation
	}

	return v.setValidator(app, validator, val)
}

// Delegate bucket on validator
func (v *validatorsKApp) Delegate(
	sender []byte,
	blockTime int64,
	blockEpoch uint32,
	tc *transaction.DelegateContract,
) (transaction.Transaction_TXResultCode, [][]byte, error) {
	update := make([][]byte, 0)

	// load Kapp Accoount
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_AccountError, update, err
	}

	// Get Current Delegation
	senderAcc, err := v.getExistingAccountFromAddress(sender)
	if err != nil {
		return transaction.Transaction_AccountError, update, err
	}

	// Check if can Delegate
	val, err := v.getValidator(app, tc.GetToAddress())
	if err != nil {
		return transaction.Transaction_AccountError, update, err
	}

	// Check current deletation mathes or remove
	buckets := senderAcc.GetBuckets(nil, v.forkController.EnableSmartContracts())

	// must use string to marshal proto map due UTF8 issue
	encodedBucketID := hex.EncodeToString(tc.BucketID)
	bucket, ok := buckets[string(encodedBucketID)]
	if !ok || bucket == nil {
		return transaction.Transaction_BucketIDInvalid, update, common.ErrInvalidValue
	}

	// check if bucket is been unfrozen
	if bucket.GetUnstakedEpoch() != core.DefaultUnstakedEpoch {
		return transaction.Transaction_BucketIDInvalid, update, common.ErrInvalidValue
	}

	// return if same delegation
	if bytes.Equal(tc.GetToAddress(), bucket.Delegation) {
		return transaction.Transaction_BucketIDInvalid, update, common.ErrInvalidValue
	}

	currentDelegation := make([]byte, len(bucket.Delegation))
	if len(bucket.Delegation) > 0 {
		copy(currentDelegation, bucket.Delegation)
	}

	amountDelegation := bucket.Value

	// if account is delegating to another peer, must check the for max delegation
	if !bytes.Equal(sender, tc.GetToAddress()) &&
		!bytes.Equal(sender, val.GetOwnerAddress()) {

		// check if allow delegation
		if !val.GetCanDelegate() ||
			// only check if MaxDelegationAmount > 0
			(val.GetMaxDelegation() != 0 && val.GetTotalStake()+amountDelegation > val.GetMaxDelegation()) {
			return transaction.Transaction_MaxDelegationAmount, update, common.ErrInvalidValue
		}
	}

	// REMOVE previous delegation
	if len(currentDelegation) > 0 {
		err = v.undelegate(app, blockEpoch, currentDelegation, sender, tc.BucketID)
		if err != nil {
			return transaction.Transaction_AccountError, update, err
		}
		update = append(update, currentDelegation)
	}

	// DELEGATE
	err = v.delegate(app, blockTime, blockEpoch, tc.GetToAddress(), sender, tc.BucketID, amountDelegation)
	if err != nil {
		return transaction.Transaction_AccountError, update, err
	}

	update = append(update, tc.GetToAddress())

	err = v.saveKApp(app)
	if err != nil {
		return transaction.Transaction_Fail, update, err
	}

	return transaction.Transaction_Ok, update, nil
}

// Undelegate marks a bucket as undelegated to be removed later
func (v *validatorsKApp) Undelegate(
	blockEpoch uint32,
	validator []byte,
	sender []byte,
	tc *transaction.UndelegateContract,
) (transaction.Transaction_TXResultCode, error) {
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	err = v.undelegate(app, blockEpoch, validator, sender, tc.BucketID)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	err = v.saveKApp(app)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	return transaction.Transaction_Ok, nil
}

// Unjail validator
func (v *validatorsKApp) Unjail(sender []byte, tc *transaction.UnjailContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	val, err := v.getValidator(app, sender)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	if peerAcc.GetList() != state.List_jailed {
		return transaction.Transaction_AccountError, common.ErrInvalidPeerList
	}

	peerAcc.SetRating(v.ratingsData.StartRating())
	peerAcc.SetTempRating(v.ratingsData.StartRating())
	peerAcc.SetListAndIndex(state.List_waiting, uint32(ctx.Block().GetNonce()))

	err = v.accountsCacher.UpdatePeer(peerAcc)
	if err != nil {
		return transaction.Transaction_Fail, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.UpdateValidator,
		ctx.ContractID(),
		sender,
	))

	return transaction.Transaction_Ok, nil
}

func (v *validatorsKApp) getValidatorBuckets(app state.KAppAccountHandler, address []byte) (*PeerData, error) {
	peerData := &PeerData{Buckets: make(map[string]*PeerBucket)}

	// Check if account already registered as validator
	vKey := v.validatorBucketsKey(address)
	data := app.GetStorage(vKey)
	if len(data) == 0 {
		return peerData, nil
	}

	err := v.marshalizer.Unmarshal(peerData, data)
	if err != nil {
		return nil, err
	}

	return peerData, nil
}

func (v *validatorsKApp) setValidatorBuckets(app state.KAppAccountHandler, address []byte, pd *PeerData) error {
	// Check if account already registered as validator
	vKey := v.validatorBucketsKey(address)

	data, err := v.marshalizer.Marshal(pd)
	if err != nil {
		return err
	}

	return app.SetStorage(vKey, data)
}

// GetValidatorBuckets returns the unmarshalled peer data for the given key
func (v *validatorsKApp) GetValidatorBuckets(address []byte) (*PeerData, error) {
	// load Kapp Accoount
	app, err := v.getKApp()
	if err != nil {
		return nil, err
	}

	return v.getValidatorBuckets(app, address)
}

func (v *validatorsKApp) computeValidatorActionType(isLeader, validatorSigned bool) validatorActionType {
	if isLeader && validatorSigned {
		return leaderSuccess
	}
	if isLeader && !validatorSigned {
		return leaderFail
	}
	if !isLeader && validatorSigned {
		return validatorSuccess
	}
	if !isLeader && !validatorSigned {
		return validatorIgnoredSignature
	}

	return unknownAction
}

func (v *validatorsKApp) display(blsPub []byte, peerAcc state.PeerAccountHandler) {
	log.Trace("validator statistics",
		"pk", tools.GetTrimmedPk(hex.EncodeToString(blsPub)),
		"leader fail", peerAcc.GetLeaderSuccessRate().NumFailure,
		"leader success", peerAcc.GetLeaderSuccessRate().NumSuccess,
		"val success", peerAcc.GetValidatorSuccessRate().NumSuccess,
		"val ignored sigs", peerAcc.GetValidatorIgnoredSignaturesRate(),
		"val fail", peerAcc.GetValidatorSuccessRate().NumFailure,
		"temp rating", peerAcc.GetTempRating(),
		"rating", peerAcc.GetRating(),
	)
}

func (v *validatorsKApp) DisplayRating(validatorOwners [][]byte) {
	app, err := v.getKApp()
	if err != nil {
		return
	}

	for _, owner := range validatorOwners {
		val, err := v.getValidator(app, owner)
		if err != nil {
			return
		}

		peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
		if err != nil {
			return
		}

		log.Trace("tempRating", "OwnerAddress", owner, "tempRating", peerAcc.GetTempRating())
	}

}

func (v *validatorsKApp) revokPeerAccount(pubkey []byte) (state.PeerAccountHandler, error) {
	peerAccOld, err := v.loadPeerAccount(pubkey)
	if err != nil {
		log.Error("UpdateValidator loading previous peerAccount", "err", err.Error())
		return nil, err
	}

	peerAccOld.SetRevoked()

	err = v.accountsCacher.UpdatePeer(peerAccOld)
	if err != nil {
		return nil, err
	}
	return peerAccOld, nil
}

func (v *validatorsKApp) loadPeerAccount(pubkey []byte) (state.PeerAccountHandler, error) {
	peerAcc, err := v.accountsCacher.LoadPeer(pubkey)
	if err != nil {
		return nil, err
	}

	return peerAcc, nil
}

func (v *validatorsKApp) isValidatorWithLowRating(peerAcc state.PeerAccountHandler) bool {
	if v.rater == nil {
		return false
	}

	minChance := v.rater.GetChance(0)
	return v.rater.GetChance(peerAcc.GetTempRating()) < minChance
}

func (v *validatorsKApp) jailValidatorIfBadRating(peerAcc state.PeerAccountHandler) {
	if !v.isValidatorWithLowRating(peerAcc) {
		return
	}

	log.Trace("jailValidatorIfBadRating", "list", peerAcc.GetList().String(), "pubKey", peerAcc.GetOwnerAddress())
	peerAcc.SetListAndIndex(state.List_jailed, peerAcc.GetIndex())
}

func (v *validatorsKApp) setToJailedIfNeeded(
	peerAcc state.PeerAccountHandler,
	validator *state.ValidatorInfo,
) {
	if validator.List == string(core.ElectedList) || validator.List == string(core.EligibleList) {
		return
	}

	if validator.List == string(core.JailedList) && peerAcc.GetList() != state.List_jailed {
		log.Trace("setToJailedIfNeeded", "pubKey", validator.PublicKey)
		peerAcc.SetListAndIndex(state.List_jailed, validator.Index)
		return
	}

	if v.isValidatorWithLowRating(peerAcc) {
		log.Trace("setToJailedIfNeeded isValidatorWithLowRating", "pubKey", validator.PublicKey)
		peerAcc.SetListAndIndex(state.List_jailed, validator.Index)
	}
}

func (v *validatorsKApp) saveUpdatesForList(
	addrs [][]byte,
	peerType string,
) (bool, error) {
	app, err := v.getKApp()
	if err != nil {
		return false, err
	}

	nodeForcedToStay := false
	for index, addr := range addrs {

		val, err := v.getValidator(app, []byte(addr))
		if err != nil {
			return false, err
		}

		peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
		if err != nil {
			return false, err
		}

		// if node have been elected by NC algo... and unstake (moved to waiting list), it should leave consensus...
		isNodeLeaving := (peerType == state.List_eligible.String() || peerType == state.List_elected.String()) && peerAcc.GetList() == state.List_waiting
		isNodeWithLowRating := v.isValidatorWithLowRating(peerAcc)
		if isNodeWithLowRating {
			// if node reach a minnimum rating, should be sent to jail
			log.Trace("saveUpdatesForList jail validator", "index", index, "addr", addr)
			peerAcc.SetListAndIndex(state.List_jailed, uint32(index))
		} else {
			// update in KApp, new node position
			peerAcc.SetListAndIndex(state.List(state.List_value[peerType]), uint32(index))
		}

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return false, err
		}

		nodeForcedToStay = nodeForcedToStay || isNodeLeaving
	}

	return nodeForcedToStay, nil
}

func (v *validatorsKApp) verifySignaturesBelowSignedThreshold(
	app state.KAppAccountHandler,
	validator *state.ValidatorInfo,
	signedThreshold float32,
) error {
	validatorOccurrences := tools.MaxUint32(1, validator.ValidatorSuccess+validator.ValidatorFailure+validator.ValidatorIgnoredSignatures)
	computedThreshold := float32(validator.ValidatorSuccess) / float32(validatorOccurrences)

	if computedThreshold <= signedThreshold {
		increasedRatingTimes := validator.ValidatorSuccess + validator.ValidatorIgnoredSignatures

		newTempRating := v.rater.RevertIncreaseValidator(validator.TempRating, increasedRatingTimes)
		peerAcc, err := v.loadPeerAccount(validator.PublicKey)
		if err != nil {
			return err
		}

		peerAcc.SetTempRating(newTempRating)
		v.jailValidatorIfBadRating(peerAcc)

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return err
		}

		log.Debug("below signed blocks threshold",
			"pk", validator.PublicKey,
			"signed %", computedThreshold,
			"validatorSuccess", validator.ValidatorSuccess,
			"validatorFailure", validator.ValidatorFailure,
			"validatorIgnored", validator.ValidatorIgnoredSignatures,
			"new tempRating", newTempRating,
			"old tempRating", validator.TempRating,
		)

		// update on feed object
		validator.TempRating = newTempRating
	}
	return nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (v *validatorsKApp) IsInterfaceNil() bool {
	return v == nil
}

func (v *validatorsKApp) PeerAccountToValidatorInfo(pubkey []byte, revoked bool, peerAcc state.PeerAccountHandler) *state.ValidatorInfo {
	return &state.ValidatorInfo{
		OwnerAddress:                    peerAcc.GetOwnerAddress(),
		PublicKey:                       pubkey,
		List:                            peerAcc.GetListString(),
		TempRating:                      peerAcc.GetTempRating(),
		Rating:                          peerAcc.GetRating(),
		RatingModifier:                  0,
		LeaderSuccess:                   peerAcc.GetLeaderSuccessRateSuccess(),
		LeaderFailure:                   peerAcc.GetLeaderSuccessRateFailure(),
		ValidatorSuccess:                peerAcc.GetValidatorSuccessRateSuccess(),
		ValidatorFailure:                peerAcc.GetValidatorSuccessRateFailure(),
		ValidatorIgnoredSignatures:      peerAcc.GetValidatorIgnoredSignaturesRate(),
		TotalLeaderSuccess:              peerAcc.GetTotalLeaderSuccessRateSuccess(),
		TotalLeaderFailure:              peerAcc.GetTotalLeaderSuccessRateFailure(),
		TotalValidatorSuccess:           peerAcc.GetTotalValidatorSuccessRateSuccess(),
		TotalValidatorFailure:           peerAcc.GetTotalValidatorSuccessRateFailure(),
		TotalValidatorIgnoredSignatures: peerAcc.GetTotalValidatorIgnoredSignaturesRate(),
		NumSelectedInSuccessBlocks:      peerAcc.GetNumSelectedInSuccessBlocks(),
		IsPubKeyRevoked:                 revoked,
	}
}
