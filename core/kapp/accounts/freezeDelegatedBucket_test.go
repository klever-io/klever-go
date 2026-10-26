package accounts

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/validators"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	"github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
)

const (
	delegatedBucketAmount = int64(1_000_000_000)
	delegatedBucketEpoch  = uint32(10)
)

type delegatedBucketValidators interface {
	kapp.ValidatorsKapp
	GetValidatorBuckets(address []byte) (*validators.PeerData, error)
}

// delegatedBucketHarness wires the real accounts kapp, validators kapp and user account. ctx stands
// for one smart-contract transaction: every builtin call made through it shares the tx nonce and
// contract ID 0, with the contract as caller.
type delegatedBucketHarness struct {
	ak             *accountsKapp
	vk             delegatedBucketValidators
	fc             interface{ EpochConfirmed(epoch uint32) }
	ctx            kapp.KappContext
	scAcc          state.UserAccountHandler
	contract       []byte
	validatorOwner []byte
	bucketID       []byte
}

func newDelegatedBucketHarness(t *testing.T, cfg config.EnableEpochs) *delegatedBucketHarness {
	t.Helper()
	require.NoError(t, cfg.Validate())

	h := &delegatedBucketHarness{
		contract:       makeAddress("delegatedBucket-contract"),
		validatorOwner: makeAddress("delegatedBucket-validator"),
	}

	forkController, err := fork.NewForkController(cfg, &commonMock.EpochNotifierStub{})
	require.NoError(t, err)
	h.fc = forkController

	h.useTx(7, delegatedBucketEpoch)
	h.bucketID = kdautils.ToBucketID(&blake2b.Blake2b{}, h.ctx.Block().GetRandSeed(), h.contract, kdautils.KLVIdentifier, 7, 0, delegatedBucketAmount)

	h.scAcc, err = state.NewUserAccount(h.contract)
	require.NoError(t, err)
	require.NoError(t, h.scAcc.AddToBalance(10*delegatedBucketAmount, kdautils.KLVIdentifier, true))

	storage := make(map[string][]byte)
	validatorsKappAcc := &commonMock.KAppAccountHandlerStub{
		GetStorageCalled: func(key []byte) []byte { return storage[string(key)] },
		SetStorageCalled: func(key []byte, value []byte) error {
			storage[string(key)] = value
			return nil
		},
	}
	peers := make(map[string]state.PeerAccountHandler)
	cacher := &commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			require.Equal(t, h.contract, address)
			return h.scAcc, nil
		},
		LoadKAppCalled: func(_ []byte) (state.KAppAccountHandler, error) { return validatorsKappAcc, nil },
		LoadPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
			if p, ok := peers[string(peer)]; ok {
				return p, nil
			}
			p, errP := state.NewPeerAccount(peer)
			if errP != nil {
				return nil, errP
			}
			peers[string(peer)] = p
			return p, nil
		},
	}

	vk, err := validators.NewValidatorKApp(&validators.ArgsNewValidatorKApp{
		Marshalizer:     &commonMock.MarshalizerMock{},
		PubkeyConv:      commonMock.NewPubkeyConverterMock(32),
		ForkController:  forkController,
		RatingsData:     &commonMock.RatingsInfoMock{},
		BLSKeyValidator: &commonMock.KeyGeneratorStub{},
	})
	require.NoError(t, err)
	require.NoError(t, vk.SetAccountsCacher(cacher))
	h.vk = vk

	staking := &kapps.StakingData{InterestType: kapps.StakingData_FPRI, MinEpochsToUnstake: 1, MinEpochsToWithdraw: 1}
	klvKDA := &kapps.KDAData{ID: kdautils.KLVIdentifier, AssetType: kapps.KDAData_Fungible, Precision: 6}

	controller := &kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return h.ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					switch p {
					case kapps.EnumParameter_MinKLVBucketAmount:
						return delegatedBucketAmount
					case kapps.EnumParameter_MaxBucketSize:
						return 100
					}
					return 0
				},
			}
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, klvKDA, nil
				},
				GetStakingCalled: func(_ []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					return nil, staking, nil
				},
			}
		},
		GetValidatorsKAppCalled: func() kapp.ValidatorsKapp { return vk },
	}
	require.NoError(t, vk.SetKAppController(controller))

	h.registerValidator(t, h.validatorOwner)

	h.ak, err = NewAccountKApp(&ArgsNewAccountKApp{
		Hasher:         &blake2b.Blake2b{},
		Marshalizer:    &commonMock.ProtoMarshalizerMock{},
		PubkeyConv:     commonMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	})
	require.NoError(t, err)
	require.NoError(t, h.ak.SetKAppController(controller))
	require.NoError(t, h.ak.SetAccountsCacher(cacher))

	return h
}

// useTx starts a new transaction context.
func (h *delegatedBucketHarness) useTx(nonce uint64, epoch uint32) {
	h.ctx = kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: makeAddress("delegatedBucket-eoa"),
		ContractID:     0,
		Block:          &block.Block{Header: &block.BlockHeader{Nonce: 100, Epoch: epoch, Timestamp: 1_700_000_000, RandSeed: []byte("delegatedBucket-rand-seed")}},
		TX:             &transaction.Transaction{RawData: &transaction.Transaction_Raw{Nonce: nonce}},
	})
}

func (h *delegatedBucketHarness) freeze(t *testing.T) (transaction.Transaction_TXResultCode, error) {
	t.Helper()
	return h.ak.Freeze(h.contract, &transaction.FreezeContract{AssetID: kdautils.KLVIdentifier, Amount: delegatedBucketAmount})
}

func (h *delegatedBucketHarness) registerValidator(t *testing.T, owner []byte) {
	t.Helper()
	h.registerValidatorWithCap(t, owner, 0)
}

func (h *delegatedBucketHarness) registerValidatorWithCap(t *testing.T, owner []byte, maxDelegation int64) {
	t.Helper()

	// from FixAuditChangesV6 on a registration needs a real BLS key and its proof for the owner
	sk, pk := signing.NewKeyGenerator(mcl.NewSuiteBLS12()).GeneratePair()
	blsKey, err := pk.ToByteArray()
	require.NoError(t, err)
	proof, err := singlesig.NewBlsSigner().Sign(sk, transaction.BLSKeyProofMessage(owner, blsKey))
	require.NoError(t, err)

	code, err := h.vk.Register(owner, &transaction.CreateValidatorContract{
		OwnerAddress: owner,
		Config: &transaction.ValidatorConfig{
			RewardAddress:       owner,
			BLSPublicKey:        blsKey,
			BLSSignature:        proof,
			CanDelegate:         true,
			Commission:          1000,
			MaxDelegationAmount: maxDelegation,
		},
	})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
}

func (h *delegatedBucketHarness) delegate(t *testing.T) {
	t.Helper()
	h.delegateTo(t, h.validatorOwner)
}

func (h *delegatedBucketHarness) delegateTo(t *testing.T, validator []byte) {
	t.Helper()
	code, err := h.ak.Delegate(h.contract, &transaction.DelegateContract{ToAddress: validator, BucketID: h.bucketID})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
}

func (h *delegatedBucketHarness) totalStake(t *testing.T) int64 {
	t.Helper()
	total, _ := h.stakeOf(t, h.validatorOwner)
	return total
}

func (h *delegatedBucketHarness) stakeOf(t *testing.T, validator []byte) (total int64, self int64) {
	t.Helper()
	infos, err := h.vk.GetValidatorsInfo([][]byte{validator})
	require.NoError(t, err)
	return infos[0].GetTotalStake(), infos[0].GetSelfStake()
}

func (h *delegatedBucketHarness) activeBucketsValue(t *testing.T) int64 {
	t.Helper()
	return h.activeBucketsValueOf(t, h.validatorOwner)
}

// activeBucketsValueOf sums the validator's buckets that are still delegated.
func (h *delegatedBucketHarness) activeBucketsValueOf(t *testing.T, validator []byte) int64 {
	t.Helper()
	pd, err := h.vk.GetValidatorBuckets(validator)
	require.NoError(t, err)
	sum := int64(0)
	for _, b := range pd.Buckets {
		if b.UndelegatedEpoch == core.DefaultUndelegatedEpoch {
			sum += b.Value
		}
	}
	return sum
}

func (h *delegatedBucketHarness) userBucket() *kapps.UserBucket {
	return h.scAcc.GetBuckets(kdautils.KLVIdentifier, true)[hex.EncodeToString(h.bucketID)]
}

func (h *delegatedBucketHarness) frozen() int64 {
	return h.scAcc.GetFrozenBalance(kdautils.KLVIdentifier, true)
}

// TestFreeze_RefusesMergeIntoDelegatedBucket covers a contract that freezes, delegates and
// freezes the same amount again within one transaction: both freezes derive the same bucket ID.
func TestFreeze_RefusesMergeIntoDelegatedBucket(t *testing.T) {
	t.Run("fix on: second freeze is rejected and validator stake stays backed", func(t *testing.T) {
		h := newDelegatedBucketHarness(t, config.EnableEpochs{})

		code, err := h.freeze(t)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		h.delegate(t)
		require.Equal(t, delegatedBucketAmount, h.totalStake(t))

		code, err = h.freeze(t)
		require.ErrorIs(t, err, common.ErrBucketAlreadyDelegated)
		require.Equal(t, transaction.Transaction_BucketIDInvalid, code)
		receipts := h.ctx.Receipts().Get()
		require.NotEmpty(t, receipts)
		require.Equal(t, [][]byte{
			{byte(kapp.ReceiptTypeError), 0},
			[]byte(common.ErrFieldInvalidBucketID),
			[]byte(common.ErrBucketAlreadyDelegated.Error()),
		}, receipts[len(receipts)-1].Data)

		require.Equal(t, 9*delegatedBucketAmount, h.scAcc.GetBalance(kdautils.KLVIdentifier, true), "rejected freeze must not debit")
		require.Equal(t, delegatedBucketAmount, h.frozen())
		require.Equal(t, delegatedBucketAmount, h.userBucket().Value)
		require.Equal(t, h.validatorOwner, h.userBucket().Delegation)
		require.Equal(t, delegatedBucketAmount, h.totalStake(t))

		// A later transaction unfreezes: the delegation is released, nothing stays on the validator.
		h.useTx(8, delegatedBucketEpoch+2)
		code, err = h.ak.Unfreeze(h.contract, &transaction.UnfreezeContract{AssetID: kdautils.KLVIdentifier, BucketID: h.bucketID})
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		require.Zero(t, h.totalStake(t))
		require.Zero(t, h.activeBucketsValue(t))
	})

	t.Run("fix off: second freeze drops the delegation and re-delegating books it twice", func(t *testing.T) {
		h := newDelegatedBucketHarness(t, config.EnableEpochs{FixAuditChangesV6: 1, FixAuditChangesV7: 2})

		_, err := h.freeze(t)
		require.NoError(t, err)
		h.delegate(t)
		_, err = h.freeze(t)
		require.NoError(t, err)
		require.Nil(t, h.userBucket().Delegation)
		require.Equal(t, 2*delegatedBucketAmount, h.frozen())

		h.delegate(t)
		require.Equal(t, 3*delegatedBucketAmount, h.totalStake(t))
		require.Equal(t, 2*delegatedBucketAmount, h.activeBucketsValue(t))
	})
}

// TestDelegate_ReplacesActiveBookingInTotals covers a bucket orphaned before the fork: once
// the fork is active, delegating it again replaces the stale booking instead of adding to it.
func TestDelegate_ReplacesActiveBookingInTotals(t *testing.T) {
	h := newDelegatedBucketHarness(t, config.EnableEpochs{FixAuditChangesV6: 1, FixAuditChangesV7: 2})

	_, err := h.freeze(t)
	require.NoError(t, err)
	h.delegate(t)
	_, err = h.freeze(t)
	require.NoError(t, err)
	require.Nil(t, h.userBucket().Delegation)
	require.Equal(t, delegatedBucketAmount, h.totalStake(t))

	h.fc.EpochConfirmed(1)

	h.delegate(t)
	require.Equal(t, 2*delegatedBucketAmount, h.totalStake(t))
	require.Equal(t, h.activeBucketsValue(t), h.totalStake(t))

	h.useTx(8, delegatedBucketEpoch+2)
	code, err := h.ak.Undelegate(h.contract, &transaction.UndelegateContract{BucketID: h.bucketID})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
	require.Zero(t, h.totalStake(t))
	require.Zero(t, h.activeBucketsValue(t))
}

// TestDelegate_BackToValidatorInSameEpochSubtractsOnce moves a bucket A -> B -> A within one
// epoch. A's entry is only marked undelegated, already out of the totals, so delegating back must not
// take it out a second time.
func TestDelegate_BackToValidatorInSameEpochSubtractsOnce(t *testing.T) {
	h := newDelegatedBucketHarness(t, config.EnableEpochs{})
	validatorB := makeAddress("delegatedBucket-validator-B")
	h.registerValidator(t, validatorB)

	_, err := h.freeze(t)
	require.NoError(t, err)
	h.delegate(t)

	h.useTx(8, delegatedBucketEpoch+1)
	h.delegateTo(t, validatorB)
	h.delegate(t)

	totalA, _ := h.stakeOf(t, h.validatorOwner)
	totalB, _ := h.stakeOf(t, validatorB)
	require.Equal(t, delegatedBucketAmount, totalA)
	require.Equal(t, h.activeBucketsValue(t), totalA)
	require.Zero(t, totalB)
	require.Zero(t, h.activeBucketsValueOf(t, validatorB))
}

// TestDelegate_ReplacesActiveBookingInSelfStake is the self-delegation variant of the
// pre-fork orphan: the contract owns the validator, so the stale booking also sits in SelfStake.
func TestDelegate_ReplacesActiveBookingInSelfStake(t *testing.T) {
	h := newDelegatedBucketHarness(t, config.EnableEpochs{FixAuditChangesV6: 1, FixAuditChangesV7: 2})
	h.registerValidator(t, h.contract)

	_, err := h.freeze(t)
	require.NoError(t, err)
	h.delegateTo(t, h.contract)
	_, err = h.freeze(t)
	require.NoError(t, err)
	require.Nil(t, h.userBucket().Delegation)

	h.fc.EpochConfirmed(1)

	h.delegateTo(t, h.contract)
	total, self := h.stakeOf(t, h.contract)
	require.Equal(t, 2*delegatedBucketAmount, total)
	require.Equal(t, 2*delegatedBucketAmount, self)
	require.Equal(t, h.activeBucketsValueOf(t, h.contract), total)
}

// TestDelegate_CapExcludesReplacedBooking re-delegates a pre-fork orphan to a
// validator whose cap only fits the replaced stake: the cap check must not count the stale booking.
func TestDelegate_CapExcludesReplacedBooking(t *testing.T) {
	setup := func(t *testing.T) (*delegatedBucketHarness, []byte) {
		h := newDelegatedBucketHarness(t, config.EnableEpochs{FixAuditChangesV6: 1, FixAuditChangesV7: 2})
		capped := makeAddress("delegatedBucket-validator-capped")
		h.registerValidatorWithCap(t, capped, 2*delegatedBucketAmount)

		_, err := h.freeze(t)
		require.NoError(t, err)
		h.delegateTo(t, capped)
		_, err = h.freeze(t)
		require.NoError(t, err)
		require.Nil(t, h.userBucket().Delegation)
		total, _ := h.stakeOf(t, capped)
		require.Equal(t, delegatedBucketAmount, total)
		return h, capped
	}

	t.Run("fix on: re-delegating within the cap succeeds", func(t *testing.T) {
		h, capped := setup(t)
		h.fc.EpochConfirmed(1)

		h.delegateTo(t, capped)
		total, _ := h.stakeOf(t, capped)
		require.Equal(t, 2*delegatedBucketAmount, total)
		require.Equal(t, h.activeBucketsValueOf(t, capped), total)
	})

	t.Run("fix off: the stale booking counts against the cap", func(t *testing.T) {
		h, capped := setup(t)

		code, err := h.ak.Delegate(h.contract, &transaction.DelegateContract{ToAddress: capped, BucketID: h.bucketID})
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_MaxDelegationAmount, code)
	})
}
