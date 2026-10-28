package validators

import (
	"bytes"
	"context"
	"encoding/binary"
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
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/crypto/signing/bls"
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
	PENDING_REWARDS      = "PREW"

	// MaxBucketsPerValidator limits the number of delegator buckets per validator
	// to prevent exceeding the MaxLeafSize (786KB) storage limit.
	// Each bucket is ~142 bytes serialized, so 4500 buckets ≈ 639KB (safe margin).
	MaxBucketsPerValidator = 4500
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

// blsPublicKeyValidator validates that a submitted BLS public key is a
// well-formed, prime-order-subgroup G2 point (curve + subgroup + non-zero + length).
type blsPublicKeyValidator interface {
	CheckPublicKeyValid(b []byte) error
}

// blsSignatureVerifier verifies a BLS signature by a public key over a message
type blsSignatureVerifier interface {
	VerifyBLS(key []byte, msg []byte, sig []byte) error
}

type validatorsKApp struct {
	marshalizer     marshal.Marshalizer
	pubkeyConv      core.PubkeyConverter
	accountsCacher  state.AccountsCacher
	forkController  core.ForkController
	ratingsData     process.RatingsInfoHandler
	rater           sharding.PeerAccountListAndRatingHandler
	blsKeyValidator blsPublicKeyValidator
	blsVerifier     blsSignatureVerifier
	addressLen      int
	KAppController  kapp.KAppController
}

// ArgsNewValidatorKApp holds the arguments needed to create a ValidatorsKApp
type ArgsNewValidatorKApp struct {
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
	RatingsData    process.RatingsInfoHandler
	// BLSKeyValidator is optional; when nil a BLS12-381 G2 validator is used.
	BLSKeyValidator blsPublicKeyValidator
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

	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	blsKeyValidator := args.BLSKeyValidator
	if blsKeyValidator == nil {
		blsKeyValidator = signing.NewKeyGenerator(mcl.NewSuiteBLS12())
	}

	v := &validatorsKApp{
		marshalizer:     args.Marshalizer,
		addressLen:      args.PubkeyConv.Len(),
		ratingsData:     args.RatingsData,
		pubkeyConv:      args.PubkeyConv,
		forkController:  args.ForkController,
		blsKeyValidator: blsKeyValidator,
		blsVerifier:     bls.NewBLS(),
	}

	return v, nil
}

// validateBLSPublicKey enforces that the submitted BLS public key is a valid
// BLS12-381 G2 point (on-curve, prime-order subgroup, non-zero, correct length).
// It is gated behind the FixAuditChangesV3 fork so that historical reprocessing
// of blocks produced before the fork epoch stays deterministic.
func (v *validatorsKApp) validateBLSPublicKey(blsPubKey []byte) error {
	if !v.forkController.FixAuditChangesV3() {
		return nil
	}

	if err := v.blsKeyValidator.CheckPublicKeyValid(blsPubKey); err != nil {
		return common.ErrInvalidBLSPublicKey
	}

	return nil
}

// verifyBLSKeyProof checks that sig is a BLS signature by blsPubKey over
// transaction.BLSKeyProofMessage(owner, blsPubKey), which proves the owner holds the
// private key and stops a copied key (or a copied proof) being bound to another
// owner. Callers validate owner and blsPubKey first. Gated behind FixAuditChangesV6;
// skipped at genesis (nonce 0), where validators are registered with public keys only.
func (v *validatorsKApp) verifyBLSKeyProof(ctx kapp.KappContext, owner, blsPubKey, sig []byte) error {
	if !v.forkController.FixAuditChangesV6() {
		return nil
	}

	if ctx.Block().GetNonce() == 0 {
		return nil
	}

	if len(sig) == 0 {
		return common.ErrInvalidBLSKeyProof
	}

	err := v.blsVerifier.VerifyBLS(blsPubKey, transaction.BLSKeyProofMessage(owner, blsPubKey), sig)
	if err != nil {
		return common.ErrInvalidBLSKeyProof
	}

	return nil
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

// Register registers the validator of tc's owner, sent by sender
func (v *validatorsKApp) Register(sender []byte, tc *transaction.CreateValidatorContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	if resultCode, err := v.validateRegisterContract(ctx, sender, tc); err != nil {
		return resultCode, err
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldValidatorAlreadySet, common.ErrAccountValidatorSet.Error())
		return transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	peerAcc, resultCode, err := v.claimPeerAccount(ctx, tc.GetOwnerAddress(), tc.GetConfig().GetBLSPublicKey(), tc.GetConfig().GetBLSSignature())
	if err != nil {
		return resultCode, err
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
		peerAcc.SetListAndIndex(state.List_elected, tools.SafeU64ToU32(ctx.Block().GetNonce()))
	} else {
		peerAcc.SetListAndIndex(state.List_inactive, tools.SafeU64ToU32(ctx.Block().GetNonce()))
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

// validateRegisterContract runs Register's stateless checks, in order: owner address and
// sender, reward address, commission and delegation cap, metadata, BLS key. The proof of the
// key is checked in claimPeerAccount, after the cheaper state lookups
func (v *validatorsKApp) validateRegisterContract(ctx kapp.KappContext, sender []byte, tc *transaction.CreateValidatorContract) (transaction.Transaction_TXResultCode, error) {
	if len(tc.GetOwnerAddress()) != v.addressLen {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOwnerAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	// from FixAuditChangesV6 on only the owner registers its validator, so a proof copied from
	// a pending transaction cannot register the same validator first with another sender's terms
	if v.forkController.FixAuditChangesV6() && ctx.Block().GetNonce() != 0 && !bytes.Equal(sender, tc.GetOwnerAddress()) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidOwnerAddress, common.ErrValidatorOwnerNotSender.Error())
		return transaction.Transaction_AccountError, common.ErrValidatorOwnerNotSender
	}

	if len(tc.GetConfig().GetRewardAddress()) != v.addressLen {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRewardAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if resultCode, err := validateCommissionAndDelegation(ctx, tc.GetConfig()); err != nil {
		return resultCode, err
	}

	if resultCode, err := validateValidatorMetadata(ctx, tc.GetConfig()); err != nil {
		return resultCode, err
	}

	if err := v.validateBLSPublicKey(tc.GetConfig().GetBLSPublicKey()); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBLSKey, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	return transaction.Transaction_Ok, nil
}

// claimPeerAccount binds an unused, unrevoked BLS key to owner on registration, once its
// proof of possession for owner verifies
func (v *validatorsKApp) claimPeerAccount(ctx kapp.KappContext, owner, blsPubKey, proof []byte) (state.PeerAccountHandler, transaction.Transaction_TXResultCode, error) {
	peerAcc, err := v.loadPeerAccount(blsPubKey)
	if err != nil {
		return nil, transaction.Transaction_AccountError, err
	}

	// verify if BLS key is been used as validator...
	if len(peerAcc.GetOwnerAddress()) > 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBLSKeyAlreadyUsed, common.ErrAccountValidatorSet.Error())
		return nil, transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	// check if bls has been revoked
	if v.forkController.FixStakingBuckets() && peerAcc.GetRevoked() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBLSKeyRevoked, common.ErrAccountValidatorSet.Error())
		return nil, transaction.Transaction_AccountError, common.ErrAccountValidatorSet
	}

	// the pairing runs after the lookups that refuse a registration cheaply, and before any write
	if err = v.verifyBLSKeyProof(ctx, owner, blsPubKey, proof); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBLSKeyProof, err.Error())
		return nil, transaction.Transaction_AccountError, err
	}

	err = peerAcc.SetOwnerAddress(owner)
	if err != nil {
		return nil, transaction.Transaction_AccountError, err
	}

	err = peerAcc.SetBLSPublicKey(blsPubKey)
	if err != nil {
		return nil, transaction.Transaction_AccountError, err
	}

	err = v.accountsCacher.UpdatePeer(peerAcc)
	if err != nil {
		return nil, transaction.Transaction_Fail, err
	}

	return peerAcc, transaction.Transaction_Ok, nil
}

// validateCommissionAndDelegation checks the commission and the delegation cap
func validateCommissionAndDelegation(ctx kapp.KappContext, cfg *transaction.ValidatorConfig) (transaction.Transaction_TXResultCode, error) {
	if cfg.GetCommission() > core.HundredPercent {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidCommission, common.ErrInvalidValue.Error())
		return transaction.Transaction_CommissionTooHigh, common.ErrInvalidValue
	}

	if cfg.GetMaxDelegationAmount() < 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidDelegation, common.ErrInvalidValue.Error())
		return transaction.Transaction_DelegationAmountInvalid, common.ErrInvalidValue
	}

	return transaction.Transaction_Ok, nil
}

// validateValidatorMetadata checks the logo, the URIs and the name, in that order;
// empty values pass
func validateValidatorMetadata(ctx kapp.KappContext, cfg *transaction.ValidatorConfig) (transaction.Transaction_TXResultCode, error) {
	if !utf8.ValidString(cfg.GetLogo()) || len(cfg.GetLogo()) > core.MaxLogoURISize {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidLogo, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if len(cfg.GetURIs()) > core.MaxURIMapSize {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldURICountExceeded, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	for key, uri := range cfg.GetURIs() {
		if !utf8.ValidString(key) ||
			!utf8.ValidString(uri) ||
			len(key) > core.MaxURIKeySize ||
			len(uri) > core.MaxURIValueSize {
			ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidURI, common.ErrInvalidValue.Error())
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
		}
	}

	if !utf8.ValidString(cfg.GetName()) || len(cfg.GetName()) > core.MaxNameSize {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidName, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	return transaction.Transaction_Ok, nil
}

// UpdateValidator
func (v *validatorsKApp) UpdateValidator(sender []byte, tc *transaction.ValidatorConfigContract) (transaction.Transaction_TXResultCode, error) {
	ctx := v.KAppController.GetCurrentKAppContext()

	if len(tc.GetConfig().GetRewardAddress()) != 0 &&
		len(tc.GetConfig().GetRewardAddress()) != v.addressLen {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidRewardAddress, process.ErrInvalidRcvAddr.Error())
		return transaction.Transaction_AccountError, process.ErrInvalidRcvAddr
	}

	if resultCode, err := validateCommissionAndDelegation(ctx, tc.GetConfig()); err != nil {
		return resultCode, err
	}

	// load Kapp Accoount
	app, err := v.getKApp()
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// Check if account already registered as validator
	val, err := v.getValidator(app, sender)
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldValidatorNotFound, err.Error())
		return transaction.Transaction_AccountError, err
	}

	if resultCode, err := v.rotateKeyAndCheckMetadata(ctx, sender, val, tc.GetConfig()); err != nil {
		return resultCode, err
	}

	applyValidatorConfig(val, tc.GetConfig())

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

// rotateKeyAndCheckMetadata rotates the BLS key when cfg changes it and checks cfg's metadata.
// From FixAuditChangesV6 on the metadata is checked first, so a refused update writes nothing.
// Before it the metadata is checked after the rotation, as it always was, so the result of a
// transaction that fails both stays the same
func (v *validatorsKApp) rotateKeyAndCheckMetadata(ctx kapp.KappContext, sender []byte, val *ValidatorData, cfg *transaction.ValidatorConfig) (transaction.Transaction_TXResultCode, error) {
	metadataFirst := v.forkController.FixAuditChangesV6()
	if metadataFirst {
		if resultCode, err := validateValidatorMetadata(ctx, cfg); err != nil {
			return resultCode, err
		}
	}

	// check if BLS Key matchs current
	if len(cfg.GetBLSPublicKey()) > 0 && !bytes.Equal(val.BlsPubKey, cfg.GetBLSPublicKey()) {
		if resultCode, err := v.rotateBLSKey(ctx, sender, val, cfg); err != nil {
			return resultCode, err
		}
	}

	if metadataFirst {
		return transaction.Transaction_Ok, nil
	}

	return validateValidatorMetadata(ctx, cfg)
}

// rotateBLSKey moves sender's validator from val.BlsPubKey to cfg's BLS key: it revokes
// the current key, binds the new one to sender and updates val.BlsPubKey
func (v *validatorsKApp) rotateBLSKey(ctx kapp.KappContext, sender []byte, val *ValidatorData, cfg *transaction.ValidatorConfig) (transaction.Transaction_TXResultCode, error) {
	newKey := cfg.GetBLSPublicKey()

	if err := v.validateBLSPublicKey(newKey); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBLSKey, err.Error())
		return transaction.Transaction_InvalidPeerKey, err
	}

	// from FixAuditChangesV6 on the new key is checked before the current one is revoked, so a
	// refused rotation leaves the current key untouched even where its writes are kept
	checkTargetFirst := v.forkController.FixAuditChangesV6()
	if checkTargetFirst {
		if resultCode, err := v.loadAndCheckRotationTarget(ctx, sender, newKey); err != nil {
			return resultCode, err
		}
	}

	// the proof is only checked from FixAuditChangesV6 on, after the cheaper target check
	if err := v.verifyBLSKeyProof(ctx, sender, newKey, cfg.GetBLSSignature()); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidBLSKeyProof, err.Error())
		return transaction.Transaction_InvalidPeerKey, err
	}

	// Reset old peer
	peerAccOld, err := v.revokePeerAccount(val.BlsPubKey)
	if err != nil {
		_ = bugsnag.Notify(err)
		return transaction.Transaction_InvalidPeerKey, err
	}

	peerAcc, err := v.loadPeerAccount(newKey)
	if err != nil {
		return transaction.Transaction_InvalidPeerKey, err
	}

	err = v.initRotatedPeerAccount(peerAcc, peerAccOld, newKey)
	if err != nil {
		return transaction.Transaction_InvalidPeerKey, err
	}

	if !checkTargetFirst {
		if resultCode, err := checkRotationTarget(ctx, sender, peerAcc); err != nil {
			return resultCode, err
		}
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
	val.BlsPubKey = newKey

	return transaction.Transaction_Ok, nil
}

// checkRotationTarget refuses rotating sender's validator to a revoked key or to a key bound to another owner
func checkRotationTarget(ctx kapp.KappContext, sender []byte, peerAcc state.PeerAccountHandler) (transaction.Transaction_TXResultCode, error) {
	if peerAcc.GetRevoked() {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBLSKeyRevoked, common.ErrBLSKeyRevoked.Error())
		return transaction.Transaction_InvalidPeerKey, common.ErrBLSKeyRevoked
	}

	if len(peerAcc.GetOwnerAddress()) > 0 &&
		!bytes.Equal(sender, peerAcc.GetOwnerAddress()) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldBLSKeyNotOwned, common.ErrAccountValidatorNotOwner.Error())
		return transaction.Transaction_InvalidPeerKey, common.ErrAccountValidatorNotOwner
	}

	return transaction.Transaction_Ok, nil
}

// loadAndCheckRotationTarget runs checkRotationTarget on the peer account of newKey
func (v *validatorsKApp) loadAndCheckRotationTarget(ctx kapp.KappContext, sender, newKey []byte) (transaction.Transaction_TXResultCode, error) {
	peerAcc, err := v.loadPeerAccount(newKey)
	if err != nil {
		return transaction.Transaction_InvalidPeerKey, err
	}

	return checkRotationTarget(ctx, sender, peerAcc)
}

// initRotatedPeerAccount sets the BLS key on a peer account seen for the first time and,
// from FixStakingBuckets on, carries the old peer's state over, jailing it if elected.
// From FixAuditChangesV7 the copied epoch fees and counters are cleared on the old peer.
// Epoch end does not pay that key. The clear keeps a sum over every peer from counting
// the snapshot twice, and it drops this epoch's stats from the revoked key.
func (v *validatorsKApp) initRotatedPeerAccount(peerAcc, peerAccOld state.PeerAccountHandler, newKey []byte) error {
	if bytes.Equal(peerAcc.GetBLSPublicKey(), newKey) {
		return nil
	}

	err := peerAcc.SetBLSPublicKey(newKey)
	if err != nil {
		return err
	}

	if !v.forkController.FixStakingBuckets() {
		return nil
	}

	// copy info from old peer
	err = peerAcc.CopyFrom(peerAccOld)
	if err != nil {
		return err
	}

	// the copy leaves the snapshot on both keys; drop it from the revoked key, which epoch end
	// does not pay, and only after the copy has succeeded
	if v.forkController.FixAuditChangesV7() {
		peerAccOld.ClearEpochAccounting()
		err = v.accountsCacher.UpdatePeer(peerAccOld)
		if err != nil {
			return err
		}
	}

	if peerAcc.GetList() == state.List_elected {
		peerAcc.SetRating(0)
		peerAcc.SetTempRating(0)
		peerAcc.SetList(state.List_jailed)
	}

	return nil
}

// applyValidatorConfig copies cfg onto val; an empty reward address, logo, URI map or
// name keeps the current value
func applyValidatorConfig(val *ValidatorData, cfg *transaction.ValidatorConfig) {
	if len(cfg.GetRewardAddress()) > 0 {
		val.RewardsAddress = cfg.GetRewardAddress()
	}

	if len(cfg.GetLogo()) > 0 {
		val.Logo = cfg.GetLogo()
	}

	if len(cfg.GetURIs()) > 0 {
		val.URIs = cfg.GetURIs()
	}

	if len(cfg.GetName()) > 0 {
		val.Name = cfg.GetName()
	}

	val.CanDelegate = cfg.GetCanDelegate()
	val.Commission = cfg.GetCommission()
	val.MaxDelegation = cfg.GetMaxDelegationAmount()
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

	addStake(val, validator, sender, -b.Value)

	return v.setValidator(app, validator, val)
}

// addStake moves the validator's stake totals by delta for a bucket owned by staker.
func addStake(val *ValidatorData, validator, staker []byte, delta int64) {
	val.TotalStake += delta
	if bytes.Equal(validator, staker) || bytes.Equal(val.OwnerAddress, staker) {
		val.SelfStake += delta
	}
}

// replacedBooking loads the target validator's buckets and its active booking of the bucket, which a
// delegation replaces rather than adds to. Before FixAuditChangesV6 it loads nothing.
func (v *validatorsKApp) replacedBooking(
	app state.KAppAccountHandler,
	validator []byte,
	encodedBucketID string,
) (*PeerData, *PeerBucket, error) {
	if !v.forkController.FixAuditChangesV6() {
		return nil, nil, nil
	}

	pd, err := v.getValidatorBuckets(app, validator)
	if err != nil {
		return nil, nil, err
	}

	booking := pd.GetBuckets()[encodedBucketID]
	if booking == nil || booking.GetUndelegatedEpoch() != core.DefaultUndelegatedEpoch {
		return pd, nil, nil
	}

	return pd, booking, nil
}

// acceptsDelegation reports whether the validator takes amount from sender. Its owner always can;
// anyone else needs CanDelegate and room under MaxDelegation once the replaced booking is taken out.
func acceptsDelegation(val *ValidatorData, validator, sender []byte, amount int64, replaced *PeerBucket) bool {
	if bytes.Equal(sender, validator) || bytes.Equal(sender, val.GetOwnerAddress()) {
		return true
	}

	if !val.GetCanDelegate() {
		return false
	}

	// only check if MaxDelegationAmount > 0
	return val.GetMaxDelegation() == 0 || val.GetTotalStake()-replaced.GetValue()+amount <= val.GetMaxDelegation()
}

// delegate books the bucket on the validator, replacing the active booking pd and replaced come
// from (both nil before FixAuditChangesV6).
func (v *validatorsKApp) delegate(
	app state.KAppAccountHandler,
	validator []byte,
	encodedBucketID string,
	booking *PeerBucket,
	pd *PeerData,
	replaced *PeerBucket,
) error {
	val, err := v.getValidator(app, validator)
	if err != nil {
		return err
	}

	if pd == nil {
		pd, err = v.getValidatorBuckets(app, validator)
		if err != nil {
			return err
		}
	}

	// Check if adding a new bucket would exceed the maximum limit
	// (skip check if bucket already exists - it's a re-delegation)
	// Only enforced after EpochRewardsV2 fork to maintain consensus during upgrade
	if v.forkController.EpochRewardsV2() {
		if _, exists := pd.Buckets[encodedBucketID]; !exists && len(pd.Buckets) >= MaxBucketsPerValidator {
			return common.ErrValidatorMaxDelegatorsReached
		}
	}

	// An active bucket being overwritten is still counted in the stake totals; take it out first so
	// they keep matching the sum of the buckets.
	if replaced != nil {
		addStake(val, validator, replaced.Address, -replaced.Value)
	}

	pd.Buckets[encodedBucketID] = booking

	err = v.setValidatorBuckets(app, validator, pd)
	if err != nil {
		return err
	}

	addStake(val, validator, booking.Address, booking.Value)

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

	currentDelegation := bytes.Clone(bucket.Delegation)

	amountDelegation := bucket.Value

	// An active booking of this bucket on the target is replaced, not added to: the cap check and
	// delegate both take it out of the totals.
	pd, replaced, err := v.replacedBooking(app, tc.GetToAddress(), encodedBucketID)
	if err != nil {
		return transaction.Transaction_AccountError, update, err
	}

	if !acceptsDelegation(val, tc.GetToAddress(), sender, amountDelegation, replaced) {
		return transaction.Transaction_MaxDelegationAmount, update, common.ErrInvalidValue
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
	booking := &PeerBucket{
		DelegatedAt:      blockTime,
		DelegatedEpoch:   blockEpoch,
		UndelegatedEpoch: core.DefaultUndelegatedEpoch,
		Value:            amountDelegation,
		Address:          sender,
	}
	err = v.delegate(app, tc.GetToAddress(), encodedBucketID, booking, pd, replaced)
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
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldValidatorNotFound, err.Error())
		return transaction.Transaction_Fail, err
	}

	peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	if peerAcc.GetList() != state.List_jailed {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldUnjailNotAvailable, common.ErrInvalidPeerList.Error())
		return transaction.Transaction_AccountError, common.ErrInvalidPeerList
	}

	peerAcc.SetRating(v.ratingsData.StartRating())
	peerAcc.SetTempRating(v.ratingsData.StartRating())
	peerAcc.SetListAndIndex(state.List_waiting, tools.SafeU64ToU32(ctx.Block().GetNonce()))

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

func (v *validatorsKApp) revokePeerAccount(pubkey []byte) (state.PeerAccountHandler, error) {
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

	var nodeForcedToStay bool
	for index, addr := range addrs {
		peerAcc, err := v.getPeerAccount(addr, app)
		if err != nil {
			return false, err
		}

		// if node have been elected by NC algo... and unstake (moved to waiting list), it should leave consensus...
		isNodeLeaving := isNodeLeavingConsensus(peerType, peerAcc)
		err = v.processValidator(index, addr, peerType, peerAcc)
		if err != nil {
			return false, err
		}

		nodeForcedToStay = nodeForcedToStay || isNodeLeaving
	}

	return nodeForcedToStay, nil
}

// Refactored method to process each validator
func (v *validatorsKApp) getPeerAccount(
	addr []byte,
	app state.KAppAccountHandler,
) (state.PeerAccountHandler, error) {
	val, err := v.getValidator(app, addr)
	if err != nil {
		return nil, err
	}

	return v.loadPeerAccount(val.BlsPubKey)
}

// Helper function to check if the node is leaving consensus
func isNodeLeavingConsensus(peerType string, peerAcc state.PeerAccountHandler) bool {
	return (peerType == state.List_eligible.String() || peerType == state.List_elected.String()) &&
		peerAcc.GetList() == state.List_waiting
}

func (v *validatorsKApp) processValidator(
	index int,
	addr []byte,
	peerType string,
	peerAcc state.PeerAccountHandler,
) error {
	isNodeWithLowRating := v.isValidatorWithLowRating(peerAcc)
	if isNodeWithLowRating {
		// if node reach a minimum rating, should be sent to jail
		log.Trace("saveUpdatesForList jail validator", "index", index, "addr", addr)
		peerAcc.SetListAndIndex(state.List_jailed, uint32(index)) // #nosec G115
	} else {
		// update in KApp, new node position
		peerAcc.SetListAndIndex(state.List(state.List_value[peerType]), uint32(index)) // #nosec G115
	}

	return v.accountsCacher.UpdatePeer(peerAcc)
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

// pendingRewardsKey returns the key for storing pending rewards for a user address
func (v *validatorsKApp) pendingRewardsKey(address []byte) []byte {
	return append([]byte(PENDING_REWARDS+kapps.Sp), address...)
}

// getPendingRewards retrieves pending rewards for a user from the KApp data trie
func (v *validatorsKApp) getPendingRewards(app state.KAppAccountHandler, address []byte) (int64, error) {
	key := v.pendingRewardsKey(address)
	data := app.GetStorage(key)
	if len(data) == 0 {
		return 0, nil
	}

	if len(data) != 8 {
		return 0, common.ErrInvalidValue
	}

	//nolint:gosec // G115: rewards value stored as uint64 but returned as int64 for API compatibility
	return int64(binary.BigEndian.Uint64(data)), nil
}

// setPendingRewards stores pending rewards for a user in the KApp data trie
func (v *validatorsKApp) setPendingRewards(app state.KAppAccountHandler, address []byte, amount int64) error {
	if amount < 0 {
		return common.ErrInvalidValue
	}

	key := v.pendingRewardsKey(address)

	if amount == 0 {
		// Remove entry if amount is zero
		return app.SetStorage(key, nil)
	}

	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(amount))

	return app.SetStorage(key, data)
}

// addToPendingRewards adds to existing pending rewards for a user
func (v *validatorsKApp) addToPendingRewards(app state.KAppAccountHandler, address []byte, amount int64) error {
	if amount == 0 {
		return nil
	}

	current, err := v.getPendingRewards(app, address)
	if err != nil {
		return err
	}

	return v.setPendingRewards(app, address, current+amount)
}

// GetPendingRewards retrieves pending rewards for a user address (public method for external access)
// External callers run outside the processing goroutine, so this reads an uncached copy
func (v *validatorsKApp) GetPendingRewards(address []byte) (int64, error) {
	app, err := v.accountsCacher.LoadKAppUncached(kapps.ValidatorsKAppAddress)
	if err != nil {
		return 0, err
	}

	return v.getPendingRewards(app, address)
}

// ClaimPendingRewards claims pending rewards for a user from the KApp data trie
// and returns the amount that was claimed. The caller is responsible for
// adding this amount to the user's allowance.
func (v *validatorsKApp) ClaimPendingRewards(address []byte) (int64, error) {
	app, err := v.getKApp()
	if err != nil {
		return 0, err
	}

	pendingAmount, err := v.getPendingRewards(app, address)
	if err != nil {
		return 0, err
	}

	if pendingAmount == 0 {
		return 0, nil
	}

	// Clear pending rewards from KApp trie
	err = v.setPendingRewards(app, address, 0)
	if err != nil {
		return 0, err
	}

	err = v.saveKApp(app)
	if err != nil {
		return 0, err
	}

	return pendingAmount, nil
}

// GetPendingRewardsTotal sums every PREW entry in the Validators KApp trie (uncached, O(n)).
// For low-frequency callers (economics endpoint / per-epoch indexer); KLC-2507 makes it O(1).
func (v *validatorsKApp) GetPendingRewardsTotal() (int64, error) {
	app, err := v.accountsCacher.LoadKAppUncached(kapps.ValidatorsKAppAddress)
	if err != nil {
		return 0, err
	}

	dataTrie := app.DataTrie()
	if check.IfNil(dataTrie) {
		return 0, nil
	}

	leavesChannels, err := dataTrie.GetAllLeavesOnChannel(app.GetRootHash(), context.Background())
	if err != nil {
		return 0, err
	}

	prefix := []byte(PENDING_REWARDS + kapps.Sp)
	total := int64(0)

	// A truncated walk undercounts the total, which must not be reported as a success.
	err = leavesChannels.ForEach(func(leaf data.KeyValueHolder) error {
		if !bytes.HasPrefix(leaf.Key(), prefix) {
			return nil
		}

		// Value = 8-byte BE amount + trie tail (see TrackableDataTrie.SaveKeyValue).
		value := leaf.Value()
		if len(value) < 8 {
			return nil
		}

		//nolint:gosec // G115: stored as uint64, summed as int64 for API parity (see getPendingRewards)
		amount := int64(binary.BigEndian.Uint64(value[:8]))
		if amount < 0 {
			log.Warn("GetPendingRewardsTotal: pending reward out of int64 range, skipping")
			return nil
		}
		if total > math.MaxInt64-amount {
			log.Warn("GetPendingRewardsTotal: sum would overflow int64, skipping entry")
			return nil
		}
		total += amount

		return nil
	})
	if err != nil {
		return 0, err
	}

	return total, nil
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
