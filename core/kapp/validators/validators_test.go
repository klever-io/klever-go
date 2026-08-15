package validators

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permissiveBLSKeyValidator accepts any BLS public key. Used by the shared test
// harness so tests unrelated to key validation can keep using placeholder keys.
type permissiveBLSKeyValidator struct{}

func (permissiveBLSKeyValidator) CheckPublicKeyValid(_ []byte) error { return nil }

func createMockArgs() *ArgsNewValidatorKApp {
	return &ArgsNewValidatorKApp{
		Marshalizer:     &mock.MarshalizerMock{},
		PubkeyConv:      mock.NewPubkeyConverterMock(32),
		ForkController:  mock.NewForkControllerStub(),
		RatingsData:     &mock.RatingsInfoMock{},
		BLSKeyValidator: permissiveBLSKeyValidator{},
	}
}

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func TestNewValidatorKApp(t *testing.T) {
	t.Run("Successful Initialization", func(t *testing.T) {
		args := createMockArgs()

		v, err := NewValidatorKApp(args)
		assert.NoError(t, err)
		assert.NotNil(t, v)
	})

	t.Run("Nil Marshalizer", func(t *testing.T) {
		args := createMockArgs()
		args.Marshalizer = nil

		v, err := NewValidatorKApp(args)
		assert.Equal(t, common.ErrNilMarshalizer, err)
		assert.Nil(t, v)
	})

	t.Run("Nil Pubkey Converter", func(t *testing.T) {
		args := createMockArgs()
		args.PubkeyConv = nil

		v, err := NewValidatorKApp(args)
		assert.Equal(t, common.ErrNilPubkeyConverter, err)
		assert.Nil(t, v)
	})

	t.Run("Nil Ratings Data", func(t *testing.T) {
		args := createMockArgs()
		args.RatingsData = nil

		v, err := NewValidatorKApp(args)
		assert.Equal(t, common.ErrNilRater, err)
		assert.Nil(t, v)
	})

	t.Run("Nil Fork Controller", func(t *testing.T) {
		args := createMockArgs()
		args.ForkController = nil

		v, err := NewValidatorKApp(args)
		assert.Equal(t, common.ErrNilForkController, err)
		assert.Nil(t, v)
	})

	t.Run("nil VersionsByEpochs is accepted", func(t *testing.T) {
		args := createMockArgs()
		args.VersionsByEpochs = nil

		v, err := NewValidatorKApp(args)
		assert.NoError(t, err)
		assert.NotNil(t, v)
	})

	t.Run("VersionsByEpochs not starting at epoch 0 is rejected", func(t *testing.T) {
		args := createMockArgs()
		args.VersionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 1, Version: "v1.0.0"},
		}

		v, err := NewValidatorKApp(args)
		assert.ErrorIs(t, err, common.ErrInvalidVersionsByEpochs)
		assert.Nil(t, v)
	})

	t.Run("VersionsByEpochs with duplicate StartEpoch is rejected", func(t *testing.T) {
		args := createMockArgs()
		args.VersionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 0, Version: "*"},
			{StartEpoch: 5, Version: "v1.0.0"},
			{StartEpoch: 5, Version: "v1.0.1"},
		}

		v, err := NewValidatorKApp(args)
		assert.ErrorIs(t, err, common.ErrInvalidVersionsByEpochs)
		assert.Nil(t, v)
	})

	t.Run("VersionsByEpochs with too long a version string is rejected", func(t *testing.T) {
		args := createMockArgs()
		args.VersionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 0, Version: fmt.Sprintf("v%0*d", core.MaxSoftwareVersionLengthInBytes, 0)},
		}

		v, err := NewValidatorKApp(args)
		assert.ErrorIs(t, err, common.ErrInvalidVersionsByEpochs)
		assert.Nil(t, v)
	})

	t.Run("unsorted but well-formed VersionsByEpochs is accepted", func(t *testing.T) {
		args := createMockArgs()
		args.VersionsByEpochs = []config.VersionByEpochs{
			{StartEpoch: 10, Version: "v2.0.0"},
			{StartEpoch: 0, Version: "*"},
			{StartEpoch: 5, Version: "v1.0.0"},
		}

		v, err := NewValidatorKApp(args)
		assert.NoError(t, err)
		assert.NotNil(t, v)
	})
}

func TestValidatorsKApp_Register(t *testing.T) {
	t.Parallel()

	t.Run("Successful registration", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		tc := &transaction.CreateValidatorContract{
			OwnerAddress: makeAddress("owner"),
			Config: &transaction.ValidatorConfig{
				RewardAddress:       makeAddress("reward"),
				Commission:          1000, // 10%
				MaxDelegationAmount: 1000000,
				BLSPublicKey:        []byte("blspubkey"),
			},
		}

		addStorageCacher(v)
		addContext(v)

		resultCode, err := v.Register(tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)

		// Verify that the validator was registered
		app, _ := v.getKApp()
		val, err := v.getValidator(app, tc.OwnerAddress)
		assert.Nil(t, err)
		assert.Equal(t, tc.Config.RewardAddress, val.RewardsAddress)
		assert.Equal(t, tc.Config.Commission, val.Commission)
		assert.Equal(t, tc.Config.MaxDelegationAmount, val.MaxDelegation)
		assert.Equal(t, tc.Config.BLSPublicKey, val.BlsPubKey)
	})

	t.Run("Invalid owner address", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		tc := &transaction.CreateValidatorContract{
			OwnerAddress: []byte("invalid"),
			Config: &transaction.ValidatorConfig{
				RewardAddress: []byte("reward"),
				Commission:    1000,
				BLSPublicKey:  []byte("blspubkey"),
			},
		}
		addContext(v)

		resultCode, err := v.Register(tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, process.ErrInvalidRcvAddr, err)
	})

	t.Run("Invalid commission", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		tc := &transaction.CreateValidatorContract{
			OwnerAddress: makeAddress("owner"),
			Config: &transaction.ValidatorConfig{
				RewardAddress: makeAddress("reward"),
				Commission:    20000, // 200%
				BLSPublicKey:  []byte("blspubkey"),
			},
		}
		addContext(v)

		resultCode, err := v.Register(tc)

		assert.Equal(t, transaction.Transaction_CommissionTooHigh, resultCode)
		assert.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("Validator already registered", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		tc := &transaction.CreateValidatorContract{
			OwnerAddress: makeAddress("owner"),
			Config: &transaction.ValidatorConfig{
				RewardAddress: makeAddress("reward"),
				Commission:    1000,
				BLSPublicKey:  []byte("blspubkey"),
			},
		}

		addStorageCacher(v)
		addContext(v)

		// Register once
		_, err := v.Register(tc)
		assert.NoError(t, err)

		// Try to register again
		resultCode, err := v.Register(tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, common.ErrAccountValidatorSet, err)
	})
}

func TestValidatorsKApp_UpdateValidator(t *testing.T) {
	t.Parallel()

	ownerAddress := makeAddress("owner")

	t.Run("Successful update", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		// First, register a validator
		registerValidator(t, v, ownerAddress, []byte("blspubkey"))

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				RewardAddress: makeAddress("newreward"),
				Commission:    2000, // 20%
				BLSPublicKey:  []byte("newblspubkey"),
			},
		}

		resultCode, err := v.UpdateValidator(ownerAddress, tc)
		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)

		// Verify that the validator was updated
		app, _ := v.getKApp()
		val, err := v.getValidator(app, ownerAddress)
		assert.Nil(t, err)
		assert.Equal(t, tc.Config.RewardAddress, val.RewardsAddress)
		assert.Equal(t, tc.Config.Commission, val.Commission)
		assert.Equal(t, tc.Config.BLSPublicKey, val.BlsPubKey)
	})

	t.Run("Update non-existent validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				RewardAddress: makeAddress("newreward"),
				Commission:    2000,
			},
		}

		resultCode, err := v.UpdateValidator([]byte("nonexistent"), tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, common.ErrValidatorNotFound, err)
	})

	t.Run("Invalid commission", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, ownerAddress, []byte("blspubkey"))

		tc := &transaction.ValidatorConfigContract{
			Config: &transaction.ValidatorConfig{
				Commission: 20000, // 200%
			},
		}

		resultCode, err := v.UpdateValidator(ownerAddress, tc)

		assert.Equal(t, transaction.Transaction_CommissionTooHigh, resultCode)
		assert.Equal(t, common.ErrInvalidValue, err)
		// Verify that the validator was not updated
		app, _ := v.getKApp()
		val, err := v.getValidator(app, ownerAddress)
		assert.Nil(t, err)
		assert.Equal(t, uint32(1000), val.Commission)
	})
}

func TestValidatorsKApp_Delegate(t *testing.T) {
	t.Parallel()

	validatorAddress := makeAddress("validator")

	t.Run("Successful delegation", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		bucketID := hex.EncodeToString([]byte("bucket1"))

		tc := &transaction.DelegateContract{
			ToAddress: validatorAddress,
			BucketID:  []byte("bucket1"),
		}

		// Setup mock account
		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{
					bucketID: {
						Value:         1000,
						UnstakedEpoch: core.DefaultUnstakedEpoch,
					},
				}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("sender"), 1000, 1, tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)
		assert.Len(t, update, 1)
		assert.Equal(t, validatorAddress, update[0])

		// Verify delegation
		app, _ := v.getKApp()
		pd, _ := v.getValidatorBuckets(app, validatorAddress)
		assert.Len(t, pd.Buckets, 1)
		assert.Equal(t, int64(1000), pd.Buckets[bucketID].Value)
	})

	t.Run("Delegation to non-existent validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		tc := &transaction.DelegateContract{
			ToAddress: []byte("nonexistent"),
			BucketID:  []byte("bucket1"),
		}

		// Setup mock account
		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("sender"), 1000, 1, tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, common.ErrValidatorNotFound, err)
		assert.Len(t, update, 0)
	})

	t.Run("Invalid bucket", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		tc := &transaction.DelegateContract{
			ToAddress: validatorAddress,
			BucketID:  []byte("invalidbucket"),
		}

		// Setup mock account with no buckets
		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("sender"), 1000, 1, tc)

		assert.Equal(t, transaction.Transaction_BucketIDInvalid, resultCode)
		assert.Equal(t, common.ErrInvalidValue, err)
		assert.Len(t, update, 0)
	})

	t.Run("Delegation fails when max buckets reached (EpochRewardsV2 enabled)", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		// Ensure EpochRewardsV2 is enabled (default in mock)
		v.forkController.(*mock.ForkControllerStub).EpochRewardsV2Value = true

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		// Pre-populate validator with max buckets
		app, err := v.getKApp()
		require.NoError(t, err)

		pd := &PeerData{Buckets: make(map[string]*PeerBucket)}
		for i := range MaxBucketsPerValidator {
			bucketKey := hex.EncodeToString(fmt.Appendf(nil, "existingbucket%d", i))
			pd.Buckets[bucketKey] = &PeerBucket{
				Value:            1000,
				DelegatedEpoch:   1,
				UndelegatedEpoch: core.DefaultUndelegatedEpoch,
				Address:          []byte("existingsender"),
			}
		}
		err = v.setValidatorBuckets(app, validatorAddress, pd)
		require.NoError(t, err)
		err = v.saveKApp(app)
		require.NoError(t, err)

		// Try to delegate a new bucket
		newBucketID := []byte("newbucket")
		tc := &transaction.DelegateContract{
			ToAddress: validatorAddress,
			BucketID:  newBucketID,
		}

		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{
					hex.EncodeToString(newBucketID): {
						Value:         500,
						UnstakedEpoch: core.DefaultUnstakedEpoch,
					},
				}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("newsender"), 2000, 2, tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, common.ErrValidatorMaxDelegatorsReached, err)
		assert.Len(t, update, 0)
	})

	t.Run("Delegation allowed when EpochRewardsV2 disabled (no bucket limit)", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		// Disable EpochRewardsV2 fork
		v.forkController.(*mock.ForkControllerStub).EpochRewardsV2Value = false

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		// Pre-populate validator with max buckets
		app, err := v.getKApp()
		require.NoError(t, err)

		pd := &PeerData{Buckets: make(map[string]*PeerBucket)}
		for i := range MaxBucketsPerValidator {
			bucketKey := hex.EncodeToString(fmt.Appendf(nil, "existingbucket%d", i))
			pd.Buckets[bucketKey] = &PeerBucket{
				Value:            1000,
				DelegatedEpoch:   1,
				UndelegatedEpoch: core.DefaultUndelegatedEpoch,
				Address:          []byte("existingsender"),
			}
		}
		err = v.setValidatorBuckets(app, validatorAddress, pd)
		require.NoError(t, err)
		err = v.saveKApp(app)
		require.NoError(t, err)

		// Try to delegate a new bucket - should succeed since fork is disabled
		newBucketID := []byte("newbucket")
		tc := &transaction.DelegateContract{
			ToAddress: validatorAddress,
			BucketID:  newBucketID,
		}

		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{
					hex.EncodeToString(newBucketID): {
						Value:         500,
						UnstakedEpoch: core.DefaultUnstakedEpoch,
					},
				}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("newsender"), 2000, 2, tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)
		assert.Len(t, update, 1)

		// Verify bucket count now exceeds max (only possible when fork disabled)
		app, _ = v.getKApp()
		pd, err = v.getValidatorBuckets(app, validatorAddress)
		require.NoError(t, err)
		assert.Equal(t, MaxBucketsPerValidator+1, len(pd.Buckets))
	})

	t.Run("Re-delegation allowed even when at max buckets (EpochRewardsV2 enabled)", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		// Enable EpochRewardsV2 fork
		v.forkController.(*mock.ForkControllerStub).EpochRewardsV2Value = true

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		// Pre-populate validator with max buckets, including the one we'll re-delegate
		app, err := v.getKApp()
		require.NoError(t, err)

		existingBucketID := []byte("existingbucket0")
		encodedExistingBucketID := hex.EncodeToString(existingBucketID)

		pd := &PeerData{Buckets: make(map[string]*PeerBucket)}
		// First bucket is the one we'll re-delegate
		pd.Buckets[encodedExistingBucketID] = &PeerBucket{
			Value:            500,
			DelegatedEpoch:   1,
			UndelegatedEpoch: core.DefaultUndelegatedEpoch,
			Address:          []byte("originalsender"),
		}
		// Fill remaining buckets to reach max
		for i := 1; i < MaxBucketsPerValidator; i++ {
			bucketKey := hex.EncodeToString(fmt.Appendf(nil, "existingbucket%d", i))
			pd.Buckets[bucketKey] = &PeerBucket{
				Value:            1000,
				DelegatedEpoch:   1,
				UndelegatedEpoch: core.DefaultUndelegatedEpoch,
				Address:          []byte("existingsender"),
			}
		}
		err = v.setValidatorBuckets(app, validatorAddress, pd)
		require.NoError(t, err)
		err = v.saveKApp(app)
		require.NoError(t, err)

		// Re-delegate the existing bucket with updated value
		tc := &transaction.DelegateContract{
			ToAddress: validatorAddress,
			BucketID:  existingBucketID,
		}

		senderAcc := &mock.UserAccountHandlerStub{
			GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
				return map[string]*kapps.UserBucket{
					encodedExistingBucketID: {
						Value:         1500, // Updated value
						UnstakedEpoch: core.DefaultUnstakedEpoch,
					},
				}
			},
		}
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
			return senderAcc, nil
		}

		resultCode, update, err := v.Delegate([]byte("originalsender"), 2000, 2, tc)

		// Should succeed because bucket already exists (re-delegation)
		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)
		assert.Len(t, update, 1)

		// Verify bucket count remains at max (no new bucket added)
		app, _ = v.getKApp()
		pd, err = v.getValidatorBuckets(app, validatorAddress)
		require.NoError(t, err)
		assert.Equal(t, MaxBucketsPerValidator, len(pd.Buckets))

		// Verify the re-delegated bucket has updated value
		assert.Equal(t, int64(1500), pd.Buckets[encodedExistingBucketID].Value)
	})
}

func TestValidatorsKApp_Undelegate(t *testing.T) {
	t.Parallel()

	validatorAddress := makeAddress("validator")

	t.Run("Successful undelegation", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))
		// First, delegate
		delegateBucket(t, v, validatorAddress, []byte("sender"), []byte("bucket1"), 1000)

		tc := &transaction.UndelegateContract{
			BucketID: []byte("bucket1"),
		}

		resultCode, err := v.Undelegate(2, validatorAddress, []byte("sender"), tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)

		// Verify undelegation
		app, _ := v.getKApp()
		pd, err := v.getValidatorBuckets(app, validatorAddress)
		assert.Nil(t, err)
		assert.Equal(t, uint32(2), pd.Buckets[hex.EncodeToString(tc.BucketID)].UndelegatedEpoch)
	})

	t.Run("Successful undelegation same epoch", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))
		// First, delegate
		delegateBucket(t, v, validatorAddress, []byte("sender"), []byte("bucket1"), 1000)

		tc := &transaction.UndelegateContract{
			BucketID: []byte("bucket1"),
		}

		resultCode, err := v.Undelegate(1, validatorAddress, []byte("sender"), tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)

		// Verify undelegation, bucket should be removed
		app, _ := v.getKApp()
		pd, err := v.getValidatorBuckets(app, validatorAddress)
		assert.Nil(t, err)
		assert.Len(t, pd.Buckets, 0)
	})

	t.Run("Undelegate non-existent bucket", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		tc := &transaction.UndelegateContract{
			BucketID: []byte("nonexistent"),
		}

		resultCode, err := v.Undelegate(1, validatorAddress, []byte("sender"), tc)

		assert.Equal(t, transaction.Transaction_Fail, resultCode)
		assert.Equal(t, common.ErrInvalidValue, err)
	})
}

func TestValidatorsKApp_GetValidatorsInfo(t *testing.T) {
	t.Parallel()

	t.Run("Get info for multiple validators", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		v1 := makeAddress("validator1")
		v2 := makeAddress("validator2")

		registerValidator(t, v, v1, []byte("blspubkey1"))
		registerValidator(t, v, v2, []byte("blspubkey2"))

		validators := [][]byte{v1, v2}

		validatorsInfo, err := v.GetValidatorsInfo(validators)

		assert.Nil(t, err)
		assert.Len(t, validatorsInfo, 2)
		assert.Equal(t, []byte("blspubkey1"), validatorsInfo[0].(*ValidatorAccountInfo).BlsPubKey)
		assert.Equal(t, []byte("blspubkey2"), validatorsInfo[1].(*ValidatorAccountInfo).BlsPubKey)
	})

	t.Run("Get info for non-existent validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)

		validators := [][]byte{[]byte("nonexistent")}

		validatorsInfo, err := v.GetValidatorsInfo(validators)

		assert.Equal(t, common.ErrValidatorNotFound, err)
		assert.Nil(t, validatorsInfo)
	})
}

func TestValidatorsKApp_Unjail(t *testing.T) {
	t.Parallel()

	validatorAddress := makeAddress("validator")

	t.Run("Successful unjail", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)

		// Register and jail the validator
		registerValidator(t, v, validatorAddress, []byte("blspubkey"))
		jailValidator(t, v, validatorAddress)

		tc := &transaction.UnjailContract{}

		resultCode, err := v.Unjail(validatorAddress, tc)

		assert.Equal(t, transaction.Transaction_Ok, resultCode)
		assert.Nil(t, err)

		// Verify that the validator has been unjailed
		peerAcc, err := v.loadPeerAccount([]byte("blspubkey"))
		assert.Nil(t, err)
		assert.Equal(t, state.List_waiting, peerAcc.GetList())
		assert.Equal(t, v.ratingsData.StartRating(), peerAcc.GetRating())
		assert.Equal(t, v.ratingsData.StartRating(), peerAcc.GetTempRating())
	})

	t.Run("Unjail non-jailed validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)

		// Register the validator (not jailed)
		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		tc := &transaction.UnjailContract{}

		resultCode, err := v.Unjail(validatorAddress, tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, common.ErrInvalidPeerList, err)
	})

	t.Run("Error loading peer account", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		addStorageCacher(v)
		addContext(v)

		// Register the validator
		registerValidator(t, v, validatorAddress, []byte("blspubkey"))

		// Mock an error when loading the peer account
		v.accountsCacher.(*mock.AccountsCacherStub).LoadPeerCalled = func(address []byte) (state.PeerAccountHandler, error) {
			return nil, errors.New("peer account loading error")
		}

		tc := &transaction.UnjailContract{}

		resultCode, err := v.Unjail(validatorAddress, tc)

		assert.Equal(t, transaction.Transaction_AccountError, resultCode)
		assert.Equal(t, "peer account loading error", err.Error())
	})
}

func addContext(v *validatorsKApp) {
	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		ContractID:     0,
		Block:          &block.Block{Header: &block.BlockHeader{Nonce: 1}},
		TxHash:         make([]byte, 32),
		IsScSimulation: true,
	})
	v.KAppController = &stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return ctx
		},
	}
}

func addStorageCacher(v *validatorsKApp) {
	rawData := make(map[string][]byte)
	v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
		return &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return rawData[string(key)]
			},
			SetStorageCalled: func(key []byte, value []byte) error {
				rawData[string(key)] = value
				return nil
			},
		}, nil
	}
}

// Helper function to register a validator for testing
func registerValidator(t *testing.T, v *validatorsKApp, owner, blsPubKey []byte) {
	tc := &transaction.CreateValidatorContract{
		OwnerAddress: owner,
		Config: &transaction.ValidatorConfig{
			RewardAddress: owner,
			Commission:    1000,
			BLSPublicKey:  blsPubKey,
			CanDelegate:   true,
		},
	}

	addContext(v)

	// Register once
	_, err := v.Register(tc)
	assert.NoError(t, err)
}

func delegateBucket(t *testing.T, v *validatorsKApp, validator, sender, bucketID []byte, amount int64) {
	tc := &transaction.DelegateContract{
		ToAddress: validator,
		BucketID:  bucketID,
	}

	// Setup mock account
	senderAcc := &mock.UserAccountHandlerStub{
		GetBucketsCalled: func(_ []byte, _ bool) map[string]*kapps.UserBucket {
			return map[string]*kapps.UserBucket{
				hex.EncodeToString(bucketID): {
					Value:         amount,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			}
		},
	}
	v.accountsCacher.(*mock.AccountsCacherStub).GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
		return senderAcc, nil
	}

	_, _, err := v.Delegate(sender, 1000, 1, tc)
	assert.NoError(t, err)
}

// Helper function to jail a validator for testing
func jailValidator(t *testing.T, v *validatorsKApp, validatorAddress []byte) {
	app, err := v.getKApp()
	require.Nil(t, err)

	val, err := v.getValidator(app, validatorAddress)
	require.Nil(t, err)

	peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
	require.Nil(t, err)

	peerAcc.SetListAndIndex(state.List_jailed, 0)
	err = v.accountsCacher.UpdatePeer(peerAcc)
	require.Nil(t, err)
}

// TestPendingRewards tests the pending rewards storage mechanism
func TestPendingRewards(t *testing.T) {
	t.Parallel()

	setupTest := func(t *testing.T) *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		// Setup mock KApp storage
		storage := make(map[string][]byte)
		loadKApp := func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return storage[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					if value == nil {
						delete(storage, string(key))
					} else {
						storage[string(key)] = value
					}
					return nil
				},
			}, nil
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = loadKApp
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = loadKApp

		return v
	}

	t.Run("GetPendingRewards - no pending rewards", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		rewards, err := v.GetPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), rewards)
	})

	t.Run("GetPendingRewards - with pending rewards", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		// Set some pending rewards first
		app, err := v.getKApp()
		require.NoError(t, err)
		err = v.setPendingRewards(app, userAddress, 500000)
		require.NoError(t, err)

		rewards, err := v.GetPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(500000), rewards)
	})

	t.Run("setPendingRewards - with zero amount clears entry", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		app, err := v.getKApp()
		require.NoError(t, err)

		// First set a non-zero amount
		err = v.setPendingRewards(app, userAddress, 100000)
		require.NoError(t, err)

		rewards, err := v.getPendingRewards(app, userAddress)
		require.NoError(t, err)
		assert.Equal(t, int64(100000), rewards)

		// Now set to zero - should clear the entry
		err = v.setPendingRewards(app, userAddress, 0)
		require.NoError(t, err)

		rewards, err = v.getPendingRewards(app, userAddress)
		require.NoError(t, err)
		assert.Equal(t, int64(0), rewards, "setting amount to 0 should clear the entry")
	})

	t.Run("addToPendingRewards - accumulates rewards", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		app, err := v.getKApp()
		require.NoError(t, err)

		// Add first amount
		err = v.addToPendingRewards(app, userAddress, 100000)
		require.NoError(t, err)

		// Add second amount
		err = v.addToPendingRewards(app, userAddress, 200000)
		require.NoError(t, err)

		rewards, err := v.getPendingRewards(app, userAddress)
		require.NoError(t, err)
		assert.Equal(t, int64(300000), rewards)
	})

	t.Run("addToPendingRewards - with zero amount does nothing", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		app, err := v.getKApp()
		require.NoError(t, err)

		// Set initial amount
		err = v.setPendingRewards(app, userAddress, 100000)
		require.NoError(t, err)

		// Add zero - should not change anything
		err = v.addToPendingRewards(app, userAddress, 0)
		require.NoError(t, err)

		rewards, err := v.getPendingRewards(app, userAddress)
		require.NoError(t, err)
		assert.Equal(t, int64(100000), rewards, "adding 0 should not change the value")
	})

	t.Run("getPendingRewards - invalid data length returns error", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		userAddress := makeAddress("user1")

		// Setup storage with invalid data (not 8 bytes)
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return []byte{0x01, 0x02, 0x03} // Invalid: only 3 bytes instead of 8
				},
			}, nil
		}

		app, err := v.getKApp()
		require.NoError(t, err)

		rewards, err := v.getPendingRewards(app, userAddress)
		assert.Equal(t, common.ErrInvalidValue, err)
		assert.Equal(t, int64(0), rewards)
	})

	t.Run("addToPendingRewards - error from getPendingRewards propagates", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		userAddress := makeAddress("user1")

		// Setup storage with invalid data to trigger error in getPendingRewards
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return []byte{0x01, 0x02} // Invalid data
				},
			}, nil
		}

		app, err := v.getKApp()
		require.NoError(t, err)

		err = v.addToPendingRewards(app, userAddress, 50000)
		assert.Equal(t, common.ErrInvalidValue, err)
	})
}

// TestClaimPendingRewards tests the claim pending rewards functionality
func TestClaimPendingRewards(t *testing.T) {
	t.Parallel()

	setupTest := func(t *testing.T) *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		// Setup mock KApp storage with save capability
		storage := make(map[string][]byte)
		loadKApp := func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return storage[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					if value == nil {
						delete(storage, string(key))
					} else {
						storage[string(key)] = value
					}
					return nil
				},
			}, nil
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = loadKApp
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = loadKApp

		// Mock SaveKApp - just return nil for now
		v.accountsCacher.(*mock.AccountsCacherStub).UpdateKappCalled = func(account state.AccountHandler) error {
			return nil
		}

		return v
	}

	t.Run("ClaimPendingRewards - no pending rewards returns zero", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		claimed, err := v.ClaimPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), claimed)
	})

	t.Run("ClaimPendingRewards - claims and clears pending rewards", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		// Set some pending rewards first
		app, err := v.getKApp()
		require.NoError(t, err)
		err = v.setPendingRewards(app, userAddress, 750000)
		require.NoError(t, err)
		err = v.saveKApp(app)
		require.NoError(t, err)

		// Claim rewards
		claimed, err := v.ClaimPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(750000), claimed)

		// Verify rewards are cleared
		remaining, err := v.GetPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), remaining, "pending rewards should be cleared after claim")
	})

	t.Run("ClaimPendingRewards - multiple claims", func(t *testing.T) {
		v := setupTest(t)
		userAddress := makeAddress("user1")

		// Set some pending rewards
		app, err := v.getKApp()
		require.NoError(t, err)
		err = v.setPendingRewards(app, userAddress, 500000)
		require.NoError(t, err)
		err = v.saveKApp(app)
		require.NoError(t, err)

		// First claim
		claimed1, err := v.ClaimPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(500000), claimed1)

		// Second claim should return 0
		claimed2, err := v.ClaimPendingRewards(userAddress)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), claimed2, "second claim should return 0")
	})

	t.Run("ClaimPendingRewards - error getting KApp", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return nil, errors.New("kapp not found")
		}

		userAddress := makeAddress("user1")
		claimed, err := v.ClaimPendingRewards(userAddress)
		assert.Error(t, err)
		assert.Equal(t, int64(0), claimed)
	})

	t.Run("ClaimPendingRewards - error from setPendingRewards", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		userAddress := makeAddress("user1")

		storage := make(map[string][]byte)
		setStorageError := errors.New("storage write error")

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return storage[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					// Allow writes initially, fail on clear (value == nil)
					if value == nil {
						return setStorageError
					}
					storage[string(key)] = value
					return nil
				},
			}, nil
		}

		// Set some pending rewards first
		app, err := v.getKApp()
		require.NoError(t, err)
		err = v.setPendingRewards(app, userAddress, 100000)
		require.NoError(t, err)

		// Claim should fail when trying to clear rewards
		claimed, err := v.ClaimPendingRewards(userAddress)
		assert.Equal(t, setStorageError, err)
		assert.Equal(t, int64(0), claimed)
	})

	t.Run("ClaimPendingRewards - error from saveKApp", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		userAddress := makeAddress("user1")

		storage := make(map[string][]byte)
		saveKAppError := errors.New("save kapp error")

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return storage[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					if value == nil {
						delete(storage, string(key))
					} else {
						storage[string(key)] = value
					}
					return nil
				},
			}, nil
		}

		v.accountsCacher.(*mock.AccountsCacherStub).UpdateKappCalled = func(account state.AccountHandler) error {
			return saveKAppError
		}

		// Set some pending rewards first
		app, err := v.getKApp()
		require.NoError(t, err)
		err = v.setPendingRewards(app, userAddress, 100000)
		require.NoError(t, err)

		// Claim should fail when trying to save KApp
		claimed, err := v.ClaimPendingRewards(userAddress)
		assert.Equal(t, saveKAppError, err)
		assert.Equal(t, int64(0), claimed)
	})
}

func TestGetPendingRewards_ConcurrentWithEpochRewardsWrites(t *testing.T) {
	t.Parallel()

	// block processing writes pending rewards into the cached validators KApp
	// (savePendingRewardsV2) while an external reader calls GetPendingRewards;
	// it must not read through the shared cached instance
	sharedApp, err := state.NewKAppAccount(kapps.ValidatorsKAppAddress)
	require.NoError(t, err)

	cacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return sharedApp, nil
		},
		LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return state.NewKAppAccount(address)
		},
	}

	v, err := NewValidatorKApp(createMockArgs())
	require.NoError(t, err)
	require.NoError(t, v.SetAccountsCacher(cacher))

	readerAddr := makeAddress("delegator-reader")
	writerAddr := makeAddress("delegator-writer")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := int64(1); i <= 500; i++ {
			_ = v.setPendingRewards(sharedApp, writerAddr, i)
		}
	}()

	for i := 0; i < 500; i++ {
		_, errGet := v.GetPendingRewards(readerAddr)
		require.NoError(t, errGet)
	}
	<-done
}

func TestValidatorsKApp_GetPendingRewardsTotal(t *testing.T) {
	v := setupValidatorsKApp(t)
	addFunctionalCacher(t, v)

	prefix := []byte(PENDING_REWARDS + kapps.Sp)
	mkVal := func(n uint64) []byte {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, n)
		return b
	}
	prew := func(name string, amount uint64) data.KeyValueHolder {
		key := append(append([]byte{}, prefix...), makeAddress(name)...)
		return keyValStorage.NewKeyValStorage(key, mkVal(amount))
	}

	leaves := []data.KeyValueHolder{
		prew("u1", 500000),
		prew("u2", 1500000),
		prew("u3", 7),
		// non-PREW entries must be ignored
		keyValStorage.NewKeyValStorage(append([]byte(VALIDATOR_PREFIX+kapps.Sp), makeAddress("v1")...), mkVal(999999)),
		keyValStorage.NewKeyValStorage(append([]byte(VALIDATOR_BLS_PREFIX+kapps.Sp), []byte("bls")...), []byte("x")),
	}

	trie := &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
			ch := make(chan data.KeyValueHolder, len(leaves))
			for _, l := range leaves {
				ch <- l
			}
			close(ch)
			return ch, nil
		},
	}
	app := &mock.KAppAccountHandlerStub{
		DataTrieCalled:    func() data.Trie { return trie },
		GetRootHashCalled: func() []byte { return []byte("root") },
	}
	v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = func(_ []byte) (state.KAppAccountHandler, error) {
		return app, nil
	}

	total, err := v.GetPendingRewardsTotal()
	require.NoError(t, err)
	assert.Equal(t, int64(2000007), total)
}

func TestValidatorsKApp_PendingRewards_ErrorPaths(t *testing.T) {
	t.Run("GetPendingRewardsTotal: nil data trie returns zero", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		app := &mock.KAppAccountHandlerStub{
			DataTrieCalled: func() data.Trie { return nil },
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = func(_ []byte) (state.KAppAccountHandler, error) {
			return app, nil
		}

		total, err := v.GetPendingRewardsTotal()
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("GetPendingRewardsTotal: GetAllLeavesOnChannel error propagates", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		expectedErr := errors.New("trie error")
		trie := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
				return nil, expectedErr
			},
		}
		app := &mock.KAppAccountHandlerStub{
			DataTrieCalled:    func() data.Trie { return trie },
			GetRootHashCalled: func() []byte { return []byte("root") },
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = func(_ []byte) (state.KAppAccountHandler, error) {
			return app, nil
		}

		total, err := v.GetPendingRewardsTotal()
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("GetPendingRewardsTotal: LoadKAppUncached error propagates", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		expectedErr := errors.New("load failed")
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = func(_ []byte) (state.KAppAccountHandler, error) {
			return nil, expectedErr
		}

		total, err := v.GetPendingRewardsTotal()
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("GetPendingRewardsTotal: short, out-of-range, and overflowing values are skipped", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		prefix := []byte(PENDING_REWARDS + kapps.Sp)
		mkVal := func(n uint64) []byte {
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, n)
			return b
		}
		prew := func(name string, value []byte) data.KeyValueHolder {
			key := append(append([]byte{}, prefix...), makeAddress(name)...)
			return keyValStorage.NewKeyValStorage(key, value)
		}
		leaves := []data.KeyValueHolder{
			prew("short", []byte{0x01}),                // < 8 bytes, skipped
			prew("neg", mkVal(uint64(1)<<63)),          // negative as int64, skipped
			prew("max", mkVal(uint64(math.MaxInt64))),  // fills the total
			prew("over", mkVal(uint64(math.MaxInt64))), // would overflow, skipped
		}
		trie := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
				ch := make(chan data.KeyValueHolder, len(leaves))
				for _, l := range leaves {
					ch <- l
				}
				close(ch)
				return ch, nil
			},
		}
		app := &mock.KAppAccountHandlerStub{
			DataTrieCalled:    func() data.Trie { return trie },
			GetRootHashCalled: func() []byte { return []byte("root") },
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppUncachedCalled = func(_ []byte) (state.KAppAccountHandler, error) {
			return app, nil
		}

		total, err := v.GetPendingRewardsTotal()
		require.NoError(t, err)
		assert.Equal(t, int64(math.MaxInt64), total)
	})
}
