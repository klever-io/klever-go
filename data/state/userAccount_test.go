package state_test

import (
	"encoding/hex"
	"errors"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

// forksBeforeV7 is a fork controller with FixAuditChangesV7 inactive.
var forksBeforeV7 = &mock.ForkControllerStub{}

func TestUserAccount_IncreaseNonce(t *testing.T) {
	account := state.NewEmptyUserAccount()
	account.IncreaseNonce(5)
	assert.Equal(t, uint64(5), account.Nonce)
}

func TestUserAccount_SetName(t *testing.T) {
	account := state.NewEmptyUserAccount()
	name := []byte("testname")
	account.SetName(name)
	assert.Equal(t, name, account.Name)
}

func TestUserAccount_AddToAllowance(t *testing.T) {
	account := state.NewEmptyUserAccount()
	err := account.AddToAllowance(100)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), account.Allowance)

	err = account.AddToAllowance(-10)
	assert.Error(t, err)
}

func TestUserAccount_AddToBalance(t *testing.T) {
	account := state.NewEmptyUserAccount()
	err := account.AddToBalance(100, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), account.Balance)

	err = account.AddToBalance(-10, nil, true)
	assert.Error(t, err)

	// Test overflow
	err = account.AddToBalance(math.MaxInt64, nil, true)
	assert.Error(t, err)
}

func TestUserAccount_SubFromBalance(t *testing.T) {
	account := state.NewEmptyUserAccount()
	account.Balance = 100

	err := account.SubFromBalance(50, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(50), account.Balance)

	err = account.SubFromBalance(60, nil, true)
	assert.Error(t, err)

	err = account.SubFromBalance(-10, nil, true)
	assert.Error(t, err)
}

func TestUserAccount_GetBalance(t *testing.T) {
	account := state.NewEmptyUserAccount()
	account.Balance = 100

	balance := account.GetBalance(nil, true)
	assert.Equal(t, int64(100), balance)
}

func TestUserAccount_Freeze(t *testing.T) {
	bucketID := []byte("bucket1")
	encodedBucketID := hex.EncodeToString(bucketID)

	account, _ := state.NewUserAccount([]byte("address"))
	userKDA, _ := account.GetUserKDA(kdautils.KLVIdentifier, nil, true)
	staking := &kapps.StakingData{}

	userBuckets := account.GetBuckets(kdautils.KLVIdentifier, true)
	assert.Len(t, userBuckets, 0)

	err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
	assert.NoError(t, err)
	assert.Equal(t, int64(100), userKDA.FrozenBalance)
	assert.Equal(t, int64(100), staking.TotalStaked)

	err = account.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
	assert.NoError(t, err)

	userBuckets = account.GetBuckets(kdautils.KLVIdentifier, true)
	assert.Len(t, userBuckets, 1)
	assert.Equal(t, int64(100), userBuckets[encodedBucketID].Value)
}

func TestUserAccount_Unfreeze(t *testing.T) {
	bucketID := []byte("bucket1")
	encodedBucketID := hex.EncodeToString(bucketID)
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{
		Buckets: map[string]*kapps.UserBucket{
			encodedBucketID: {
				Value:         100,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
		FrozenBalance: 100,
	}
	staking := &kapps.StakingData{
		TotalStaked:        100,
		MinEpochsToUnstake: 1,
	}

	_, value, err := account.Unfreeze(kdautils.KLVIdentifier, bucketID, 2, staking, userKDA, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), value)
	assert.Equal(t, int64(0), userKDA.FrozenBalance)
	assert.Equal(t, int64(0), staking.TotalStaked)

	err = account.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
	assert.NoError(t, err)

	userBuckets := account.GetBuckets(kdautils.KLVIdentifier, true)
	assert.Len(t, userBuckets, 1)
	assert.Equal(t, int64(100), userBuckets[encodedBucketID].Value)
	assert.Equal(t, uint32(2), userBuckets[encodedBucketID].UnstakedEpoch)

	// try unfreezing again
	_, _, err = account.Unfreeze(kdautils.KLVIdentifier, bucketID, 2, staking, userKDA, true)
	assert.Equal(t, state.ErrUnstakeNotAvailable, err)

}

func TestUserAccount_Claim(t *testing.T) {
	account := state.NewEmptyUserAccount()
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{
			Timestamp: 1000,
			Epoch:     1,
		},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {
				Value:         1_000_000,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR: []*kapps.APRData{
			{
				Timestamp: 2000,
				Value:     1000, // 10% yearly
			},
		},
	}
	kda := &kapps.KDAData{}
	forkController := &mock.ForkControllerStub{}

	gains, err := account.Claim(transaction.ClaimContract_StakingClaim, kdautils.KLVIdentifier, 2, 2000, staking, kda, userKDA, forkController)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), gains["KLV"]) // 0 as the 10% APR starts at 2000
	gains, err = account.Claim(transaction.ClaimContract_StakingClaim, kdautils.KLVIdentifier, 2, 3000, staking, kda, userKDA, forkController)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), gains["KLV"]) // Approximately 10% APR for 1000 seconds

	// check after forking
	forkController.BigBucketsComputeValue = true
	gains, err = account.Claim(transaction.ClaimContract_StakingClaim, kdautils.KLVIdentifier, 2, 3000, staking, kda, userKDA, forkController)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), gains["KLV"]) // Approximately 10% APR for 1000 seconds
}

func TestPermission_ChecksPermissionGrantedFor(t *testing.T) {
	tests := []struct {
		name          string
		permission    *state.Permission
		contractTypes []transaction.TXContract_ContractType
		expected      bool
	}{
		{
			name:          "Nil permission",
			permission:    nil,
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:      false,
		},
		{
			name:          "Owner permission",
			permission:    &state.Permission{Type: state.Permission_Owner},
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:      true,
		},
		{
			name:          "Granted permission",
			permission:    &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000001}},
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType},
			expected:      true,
		},
		{
			name:          "Not granted permission",
			permission:    &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000001}},
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_CreateAssetContractType},
			expected:      false,
		},
		{
			name:          "Multiple permissions - all granted",
			permission:    &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000011}},
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_CreateAssetContractType},
			expected:      true,
		},
		{
			name:          "Multiple permissions - one not granted",
			permission:    &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000011}},
			contractTypes: []transaction.TXContract_ContractType{transaction.TXContract_TransferContractType, transaction.TXContract_CreateValidatorContractType},
			expected:      false,
		},
		{
			name:          "Empty contract types",
			permission:    &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000001}},
			contractTypes: []transaction.TXContract_ContractType{},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.permission.CheckPermissionGrantedForContracts(tt.contractTypes...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPermission_CheckPermissionGrantedForInt64(t *testing.T) {
	tests := []struct {
		name       string
		permission *state.Permission
		ops        uint64
		expected   bool
	}{
		{
			name:       "Nil permission",
			permission: nil,
			ops:        1,
			expected:   false,
		},
		{
			name:       "Owner permission",
			permission: &state.Permission{Type: state.Permission_Owner},
			ops:        math.MaxInt64, // All bits set
			expected:   true,
		},
		{
			name:       "Granted permission",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000001}},
			ops:        1,
			expected:   true,
		},
		{
			name:       "Granted permission higher than 8 bits",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0x0, 0b00000001}},
			ops:        1 << transaction.TXContract_WithdrawContractType,
			expected:   true,
		},
		{
			name:       "Not granted permission",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000001}},
			ops:        2,
			expected:   false,
		},
		{
			name:       "Multiple permissions - all granted",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000011}},
			ops:        3,
			expected:   true,
		},
		{
			name:       "Multiple permissions - one not granted",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000011}},
			ops:        5,
			expected:   false,
		},
		{
			name:       "No operations requested",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b00000011}},
			ops:        0,
			expected:   true,
		},
		{
			name:       "All operations requested, not all granted",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0b11111111}},
			ops:        math.MaxInt64, // All bits set
			expected:   false,
		},
		{
			name:       "Operations beyond 8 bytes",
			permission: &state.Permission{Type: state.Permission_User, Operations: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
			ops:        uint64(0x7000000000000000), // Highest bit set
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.permission.CheckPermissionGrantedForUint64(tt.ops)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewUserAccount_ValidAddress(t *testing.T) {
	addr := []byte("valid-address")
	account, err := state.NewUserAccount(addr)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, addr, account.Address)
}

func TestNewUserAccount_NilAddress(t *testing.T) {
	account, err := state.NewUserAccount(nil)
	assert.Error(t, err)
	assert.Nil(t, account)
}

func TestNewUserAccount_EmptyAddress(t *testing.T) {
	account, err := state.NewUserAccount([]byte{})
	assert.Error(t, err)
	assert.Nil(t, account)
}

func TestUserAccount_GetPermission(t *testing.T) {
	t.Run("no permissions set, id 0 returns default owner", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		perm, hasCustom, err := account.GetPermission(0)
		assert.NoError(t, err)
		assert.False(t, hasCustom)
		assert.NotNil(t, perm)
		assert.Equal(t, int32(0), perm.ID)
		assert.Equal(t, state.Permission_Owner, perm.Type)
		assert.Equal(t, int64(1), perm.Threshold)
		assert.Len(t, perm.Signers, 1)
	})

	t.Run("no permissions set, non-zero id returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		perm, hasCustom, err := account.GetPermission(1)
		assert.Equal(t, state.ErrInvalidPermissionID, err)
		assert.False(t, hasCustom)
		assert.Nil(t, perm)
	})

	t.Run("custom permissions set, found by id", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customPerm := &state.Permission{
			ID:        2,
			Type:      state.Permission_User,
			Threshold: 1,
		}
		account.SetPermissions([]*state.Permission{customPerm})

		perm, hasCustom, err := account.GetPermission(2)
		assert.NoError(t, err)
		assert.True(t, hasCustom)
		assert.Equal(t, customPerm, perm)
	})

	t.Run("custom permissions set, id not found", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customPerm := &state.Permission{
			ID:        2,
			Type:      state.Permission_User,
			Threshold: 1,
		}
		account.SetPermissions([]*state.Permission{customPerm})

		perm, hasCustom, err := account.GetPermission(99)
		assert.Equal(t, state.ErrInvalidPermissionID, err)
		assert.False(t, hasCustom)
		assert.Nil(t, perm)
	})
}

func TestUserAccount_Equal(t *testing.T) {
	t.Run("equal accounts", func(t *testing.T) {
		acc1, _ := state.NewUserAccount([]byte("address"))
		acc1.Balance = 100
		acc1.Nonce = 5

		acc2, _ := state.NewUserAccount([]byte("address"))
		acc2.Balance = 100
		acc2.Nonce = 5

		assert.True(t, acc1.Equal(acc2))
	})

	t.Run("different balance", func(t *testing.T) {
		acc1, _ := state.NewUserAccount([]byte("address"))
		acc1.Balance = 100

		acc2, _ := state.NewUserAccount([]byte("address"))
		acc2.Balance = 200

		assert.False(t, acc1.Equal(acc2))
	})

	t.Run("different nonce", func(t *testing.T) {
		acc1, _ := state.NewUserAccount([]byte("address"))
		acc1.Nonce = 1

		acc2, _ := state.NewUserAccount([]byte("address"))
		acc2.Nonce = 2

		assert.False(t, acc1.Equal(acc2))
	})

	t.Run("nil comparison with non-UserAccountHandler", func(t *testing.T) {
		acc1, _ := state.NewUserAccount([]byte("address"))
		// Pass nil as UserAccountHandler - Equal should return false
		assert.False(t, acc1.Equal(nil))
	})
}

func TestUserAccount_Delegate(t *testing.T) {
	t.Run("successful delegation", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)
		delegation := []byte("validator-address")

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         500,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}

		value, err := account.Delegate(bucketID, delegation, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(500), value)
		assert.Equal(t, delegation, userKDA.Buckets[encodedBucketID].Delegation)
	})

	t.Run("nil buckets returns ErrNotStaked", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{Buckets: nil}

		_, err := account.Delegate([]byte("bucket1"), []byte("del"), userKDA)
		assert.Equal(t, state.ErrNotStaked, err)
	})

	t.Run("bucket not found returns ErrNotStaked", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{},
		}

		_, err := account.Delegate([]byte("nonexistent"), []byte("del"), userKDA)
		assert.Equal(t, state.ErrNotStaked, err)
	})
}

func TestUserAccount_Undelegate(t *testing.T) {
	t.Run("successful undelegation", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)
		delegation := []byte("validator-address")

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         500,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
					Delegation:    delegation,
				},
			},
		}

		returnedDelegation, value, err := account.Undelegate(bucketID, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, delegation, returnedDelegation)
		assert.Equal(t, int64(500), value)
		assert.Nil(t, userKDA.Buckets[encodedBucketID].Delegation)
	})

	t.Run("nil buckets returns ErrNotStaked", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{Buckets: nil}

		_, _, err := account.Undelegate([]byte("bucket1"), userKDA)
		assert.Equal(t, state.ErrNotStaked, err)
	})

	t.Run("bucket not found returns ErrNotStaked", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{},
		}

		_, _, err := account.Undelegate([]byte("nonexistent"), userKDA)
		assert.Equal(t, state.ErrNotStaked, err)
	})

	t.Run("no delegation returns ErrUndelegatingNotAvailable", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         500,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
					Delegation:    nil,
				},
			},
		}

		_, _, err := account.Undelegate(bucketID, userKDA)
		assert.Equal(t, state.ErrUndelegatingNotAvailable, err)
	})
}

func TestUserAccount_Withdraw(t *testing.T) {
	t.Run("successful withdraw KLV", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		account.Balance = 0
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         200,
					StakedEpoch:   1,
					UnstakedEpoch: 2,
				},
			},
		}

		// epoch=5, minEpochsToWithdraw=2 => 5 >= 2+2 => eligible
		withdrawn, err := account.Withdraw(kdautils.KLVIdentifier, 5, 2, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(200), withdrawn)
		assert.Equal(t, int64(200), account.Balance)
		assert.Len(t, userKDA.Buckets, 0)
	})

	t.Run("withdraw not available - epoch too early", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         200,
					StakedEpoch:   1,
					UnstakedEpoch: 5,
				},
			},
		}

		// epoch=6, minEpochsToWithdraw=3 => 6 >= 5+3=8 => not eligible
		_, err := account.Withdraw(kdautils.KLVIdentifier, 6, 3, userKDA)
		assert.Equal(t, state.ErrWithdrawNotAvailable, err)
	})

	t.Run("withdraw non-KLV asset adds to userKDA balance", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)
		customAsset := []byte("MYTOKEN")

		userKDA := &kapps.UserKDA{
			Balance: 50,
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         100,
					StakedEpoch:   1,
					UnstakedEpoch: 2,
				},
			},
		}

		withdrawn, err := account.Withdraw(customAsset, 10, 2, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), withdrawn)
		// non-KLV goes to userKDA.Balance, not account.Balance
		assert.Equal(t, int64(0), account.Balance)
	})

	t.Run("withdraw multiple buckets, partial eligibility", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		b1 := hex.EncodeToString([]byte("b1"))
		b2 := hex.EncodeToString([]byte("b2"))

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				b1: {
					Value:         100,
					StakedEpoch:   1,
					UnstakedEpoch: 2,
				},
				b2: {
					Value:         300,
					StakedEpoch:   1,
					UnstakedEpoch: 10,
				},
			},
		}

		// epoch=5, minEpochsToWithdraw=2 => b1: 5>=2+2 yes, b2: 5>=10+2 no
		withdrawn, err := account.Withdraw(kdautils.KLVIdentifier, 5, 2, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), withdrawn)
		assert.Len(t, userKDA.Buckets, 1)
	})
}

func TestUserAccount_ClaimAllowance(t *testing.T) {
	t.Run("claim allowance returns allowance and resets it", func(t *testing.T) {
		account := state.NewEmptyUserAccount()
		_ = account.AddToAllowance(500)

		forkController := &mock.ForkControllerStub{}
		gains, err := account.Claim(
			transaction.ClaimContract_AllowanceClaim,
			kdautils.KLVIdentifier,
			1, 1000,
			&kapps.StakingData{},
			&kapps.KDAData{},
			&kapps.UserKDA{},
			forkController,
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(500), gains[string(kdautils.KLVIdentifier)])
		assert.Equal(t, int64(0), account.Allowance)
	})

	t.Run("allowance claim with non-KLV asset returns error", func(t *testing.T) {
		account := state.NewEmptyUserAccount()
		forkController := &mock.ForkControllerStub{}

		_, err := account.Claim(
			transaction.ClaimContract_AllowanceClaim,
			[]byte("OTHER"),
			1, 1000,
			&kapps.StakingData{},
			&kapps.KDAData{},
			&kapps.UserKDA{},
			forkController,
		)
		assert.Error(t, err)
	})
}

func TestUserAccount_ClaimMarket(t *testing.T) {
	account := state.NewEmptyUserAccount()
	forkController := &mock.ForkControllerStub{}

	gains, err := account.Claim(
		transaction.ClaimContract_MarketClaim,
		kdautils.KLVIdentifier,
		1, 1000,
		&kapps.StakingData{},
		&kapps.KDAData{},
		&kapps.UserKDA{},
		forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_AddToBalanceCustomAsset(t *testing.T) {
	t.Parallel()

	t.Run("success with custom asset", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("USDT")

		err := account.AddToBalance(100, customAsset, true)
		assert.NoError(t, err)

		balance := account.GetBalance(customAsset, true)
		assert.Equal(t, int64(100), balance)
	})

	t.Run("success with pre-passed userKDA option", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("MYTOKEN")

		userKDA, err := account.GetUserKDA(customAsset, nil, true)
		assert.NoError(t, err)

		err = account.AddToBalance(200, customAsset, true, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(200), userKDA.Balance)
	})

	t.Run("overflow check", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("TOKEN")

		userKDA, _ := account.GetUserKDA(customAsset, nil, true)
		userKDA.Balance = math.MaxInt64
		_ = account.SetUserKDA(customAsset, nil, userKDA)

		err := account.AddToBalance(1, customAsset, true)
		assert.Equal(t, state.ErrAddBalanceOverflow, err)
	})
}

func TestUserAccount_SubFromBalanceCustomAsset(t *testing.T) {
	t.Parallel()

	t.Run("success with custom asset", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("USDC")

		err := account.AddToBalance(500, customAsset, true)
		assert.NoError(t, err)

		err = account.SubFromBalance(200, customAsset, true)
		assert.NoError(t, err)

		balance := account.GetBalance(customAsset, true)
		assert.Equal(t, int64(300), balance)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("DAI")

		err := account.AddToBalance(100, customAsset, true)
		assert.NoError(t, err)

		err = account.SubFromBalance(200, customAsset, true)
		assert.Equal(t, state.ErrInsufficientFunds, err)
	})

	t.Run("success with pre-passed userKDA option", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("WBTC")

		userKDA, err := account.GetUserKDA(customAsset, nil, true)
		assert.NoError(t, err)
		userKDA.Balance = 1000
		_ = account.SetUserKDA(customAsset, nil, userKDA)

		err = account.SubFromBalance(300, customAsset, true, userKDA)
		assert.NoError(t, err)
		assert.Equal(t, int64(700), userKDA.Balance)
	})
}

func TestUserAccount_ClaimInvalidType(t *testing.T) {
	account := state.NewEmptyUserAccount()
	forkController := &mock.ForkControllerStub{}

	_, err := account.Claim(
		transaction.ClaimContract_EnumClaimType(99),
		kdautils.KLVIdentifier,
		1, 1000,
		&kapps.StakingData{},
		&kapps.KDAData{},
		&kapps.UserKDA{},
		forkController,
	)
	assert.Equal(t, state.ErrInvalidClaimType, err)
}

func TestUserAccount_ComputeClaimFPR(t *testing.T) {
	t.Run("basic FPR claim with BigBucketsCompute", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()

		bucketKey := "mybucket"
		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{
				Timestamp: 1000,
				Epoch:     1,
			},
			Buckets: map[string]*kapps.UserBucket{
				bucketKey: {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 500,
					TotalStaked: 5000,
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		// bucket value 1000 / totalStaked 5000 * totalAmount 500 = 100
		assert.Equal(t, int64(100), gains[string(kdautils.KLVIdentifier)])
	})

	t.Run("FPR claim without BigBucketsCompute", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.BigBucketsComputeValue = false

		bucketKey := "mybucket"
		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{
				Timestamp: 1000,
				Epoch:     1,
			},
			Buckets: map[string]*kapps.UserBucket{
				bucketKey: {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 500,
					TotalStaked: 5000,
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), gains[string(kdautils.KLVIdentifier)])
	})

	t.Run("FPR nil buckets returns nil", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets:   nil,
		}
		staking := &kapps.StakingData{
			InterestType: kapps.StakingData_FPRI,
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		assert.Nil(t, gains)
	})

	t.Run("FPR claim not available - min epochs not met", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 5},
			Buckets:   map[string]*kapps.UserBucket{"b1": {Value: 100, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch}},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 10,
		}

		_, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 6, 2000, userKDA, staking, forkController,
		)
		assert.Equal(t, state.ErrClaimNotAvailable, err)
	})

	t.Run("FPR legacy skip remains before FixAuditChangesV3", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.FixStakingBucketsValue = true
		forkController.FixAuditChangesV3Value = false

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				"active": {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
				"unstaked": {
					Value:         2000,
					StakedEpoch:   1,
					UnstakedEpoch: 3,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 1000,
					TotalStaked: 10000,
				},
				{
					Epoch:       4,
					TotalAmount: 1000,
					TotalStaked: 10000,
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 5, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(200), gains[string(kdautils.KLVIdentifier)])
	})

	t.Run("FPR claims only pre-unstake epochs with FixAuditChangesV3", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.FixStakingBucketsValue = true
		forkController.FixAuditChangesV3Value = true

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				"active": {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
				"unstaked": {
					Value:         2000,
					StakedEpoch:   1,
					UnstakedEpoch: 3,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 1000,
					TotalStaked: 10000,
				},
				{
					Epoch:       4,
					TotalAmount: 1000,
					TotalStaked: 10000,
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 5, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(400), gains[string(kdautils.KLVIdentifier)])
	})

	t.Run("FPR with ClaimKFI false uses KFI identifier", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.ClaimKFIValue = false

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				"b1": {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 500,
					TotalStaked: 5000,
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		// When ClaimKFI is false, gains go to KFI identifier
		assert.Equal(t, int64(100), gains[string(kdautils.KFIIdentifier)])
	})

	t.Run("FPR with KDAS distribution", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				"b1": {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:       2,
					TotalAmount: 0,
					TotalStaked: 5000,
					KDAS: map[string]*kapps.KDAFPRData{
						"MYTOKEN": {
							TotalAmount: 2000,
						},
					},
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		// 1000/5000 * 2000 = 400
		assert.Equal(t, int64(400), gains["MYTOKEN"])
	})

	t.Run("FPR with FPRComputeAndKdaFeeFlow clamps negative to zero", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.FPRComputeAndKdaFeeFlowValue = true

		userKDA := &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				"b1": {
					Value:         1000,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR: []*kapps.FPRData{
				{
					Epoch:        2,
					TotalAmount:  100,
					TotalStaked:  5000,
					TotalClaimed: 100, // already fully claimed
				},
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		// totalClaimed(100) + calculated amount > totalAmount(100) => clamped
		assert.Equal(t, int64(0), gains[string(kdautils.KLVIdentifier)])
	})
}

func TestUserAccount_ComputeAvailableClaim_InvalidStakeType(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_EnumInterestType(99),
	}
	_, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 1, 1000,
		&kapps.UserKDA{},
		staking,
		forkController,
	)
	assert.Equal(t, state.ErrInvalidStakeType, err)
}

func TestUserAccount_AddToBalanceWithNonce(t *testing.T) {
	t.Run("successful add with valid nonce", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(100, []byte("MYTOKEN"), []byte("1"), true)
		assert.NoError(t, err)
	})

	t.Run("negative value returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(-10, []byte("MYTOKEN"), []byte("1"), true)
		assert.Equal(t, state.ErrInvalidValue, err)
	})

	t.Run("zero value returns nil immediately", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(0, []byte("MYTOKEN"), []byte("1"), true)
		assert.NoError(t, err)
	})

	t.Run("invalid nonce string returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(100, []byte("MYTOKEN"), []byte("notanumber"), true)
		assert.Error(t, err)
	})

	t.Run("zero nonce returns ErrInvalidNonce", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(100, []byte("MYTOKEN"), []byte("0"), true)
		assert.Equal(t, state.ErrInvalidNonce, err)
	})

	t.Run("negative nonce returns ErrInvalidNonce", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(100, []byte("MYTOKEN"), []byte("-5"), true)
		assert.Equal(t, state.ErrInvalidNonce, err)
	})
}

func TestUserAccount_SubFromBalanceWithNonce(t *testing.T) {
	t.Run("successful subtract with valid nonce", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		// First add some balance
		err := account.AddToBalanceWithNonce(200, []byte("MYTOKEN"), []byte("1"), true)
		assert.NoError(t, err)

		err = account.SubFromBalanceWithNonce(100, []byte("MYTOKEN"), []byte("1"), true)
		assert.NoError(t, err)
	})

	t.Run("negative value returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.SubFromBalanceWithNonce(-10, []byte("MYTOKEN"), []byte("1"), true)
		assert.Equal(t, state.ErrInvalidValue, err)
	})

	t.Run("invalid nonce string returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.SubFromBalanceWithNonce(100, []byte("MYTOKEN"), []byte("abc"), true)
		assert.Error(t, err)
	})

	t.Run("zero nonce returns ErrInvalidNonce", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.SubFromBalanceWithNonce(100, []byte("MYTOKEN"), []byte("0"), true)
		assert.Equal(t, state.ErrInvalidNonce, err)
	})

	t.Run("insufficient funds returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.SubFromBalanceWithNonce(100, []byte("MYTOKEN"), []byte("1"), true)
		assert.Equal(t, state.ErrAddBalanceOverflow, err)
	})
}

func TestUserAccount_GetBalanceWithNonce(t *testing.T) {
	t.Run("returns zero for nonexistent asset", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		balance := account.GetBalanceWithNonce([]byte("MYTOKEN"), []byte("1"), true)
		assert.Equal(t, int64(0), balance)
	})

	t.Run("returns correct balance after add", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		err := account.AddToBalanceWithNonce(300, []byte("MYTOKEN"), []byte("1"), true)
		assert.NoError(t, err)

		balance := account.GetBalanceWithNonce([]byte("MYTOKEN"), []byte("1"), true)
		assert.Equal(t, int64(300), balance)
	})
}

func TestUserAccount_GetAllowance(t *testing.T) {
	account := state.NewEmptyUserAccount()
	assert.Equal(t, int64(0), account.GetAllowance())

	_ = account.AddToAllowance(42)
	assert.Equal(t, int64(42), account.GetAllowance())
}

func TestUserAccount_GetFrozenBalance(t *testing.T) {
	t.Run("returns zero for no frozen balance", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		frozenBalance := account.GetFrozenBalance(kdautils.KLVIdentifier, true)
		assert.Equal(t, int64(0), frozenBalance)
	})

	t.Run("returns frozen balance after freeze", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA, _ := account.GetUserKDA(kdautils.KLVIdentifier, nil, true)
		staking := &kapps.StakingData{}

		err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 500, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
		assert.NoError(t, err)
		err = account.SetUserKDA(kdautils.KLVIdentifier, nil, userKDA)
		assert.NoError(t, err)

		frozenBalance := account.GetFrozenBalance(kdautils.KLVIdentifier, true)
		assert.Equal(t, int64(500), frozenBalance)
	})
}

func TestUserAccount_SetOwnerAddress(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	owner := []byte("owner-address")
	account.SetOwnerAddress(owner)
	assert.Equal(t, owner, account.OwnerAddress)
}

func TestUserAccount_SetCodeHash(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	codeHash := []byte("code-hash-value")
	account.SetCodeHash(codeHash)
	assert.Equal(t, codeHash, account.GetCodeHash())
}

func TestUserAccount_GetCodeHash(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	assert.Nil(t, account.GetCodeHash())

	hash := []byte("hash123")
	account.SetCodeHash(hash)
	assert.Equal(t, hash, account.GetCodeHash())
}

func TestUserAccount_SetCodeMetadata(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	metadata := []byte("metadata-bytes")
	account.SetCodeMetadata(metadata)
	assert.Equal(t, metadata, account.CodeMetadata)
}

func TestUserAccount_SetPermissions(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	permissions := []*state.Permission{
		{ID: 0, Type: state.Permission_Owner, Threshold: 1},
		{ID: 1, Type: state.Permission_User, Threshold: 2},
	}
	account.SetPermissions(permissions)
	assert.Len(t, account.Permissions, 2)
	assert.Equal(t, int32(1), account.Permissions[1].ID)
}

func TestUserAccount_AddInternalKDA(t *testing.T) {
	t.Run("successful add", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		data := []byte("some-nft-data")
		err := account.AddInternalKDA([]byte("MYTOKEN"), []byte("1"), data)
		assert.NoError(t, err)
	})

	t.Run("empty data returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		err := account.AddInternalKDA([]byte("MYTOKEN"), []byte("1"), []byte{})
		assert.Error(t, err)
	})

	t.Run("nil data returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		err := account.AddInternalKDA([]byte("MYTOKEN"), []byte("1"), nil)
		assert.Error(t, err)
	})
}

func TestUserAccount_SubInternalKDA(t *testing.T) {
	t.Run("successful sub after add", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		data := []byte("some-nft-data")
		err := account.AddInternalKDA([]byte("MYTOKEN"), []byte("1"), data)
		assert.NoError(t, err)

		retrieved, err := account.SubInternalKDA([]byte("MYTOKEN"), []byte("1"), forksBeforeV7)
		assert.NoError(t, err)
		assert.Equal(t, data, retrieved)
	})

	t.Run("sub nonexistent returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		_, err := account.SubInternalKDA([]byte("MYTOKEN"), []byte("1"), forksBeforeV7)
		assert.Error(t, err)
	})
}

// From FixAuditChangesV7 SubInternalKDA takes only a record that holds the NFT, Balance 1 as Mint
// stores it, and refuses any other before clearing it (KLC-2835). Before the fork it takes any
// record.
func TestUserAccount_SubInternalKDA_HeldNFT(t *testing.T) {
	assetID, nonce := []byte("NFT-1234"), []byte("1")
	forksV7 := &mock.ForkControllerStub{FixAuditChangesV7Value: true}

	t.Run("takes a held record", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		assert.NoError(t, account.AddInternalKDA(assetID, nonce, mock.NFTHolderRecord(1)))

		data, err := account.SubInternalKDA(assetID, nonce, forksV7)

		assert.NoError(t, err)
		assert.Equal(t, mock.NFTHolderRecord(1), data)
		_, err = account.SubInternalKDA(assetID, nonce, forksV7)
		assert.ErrorIs(t, err, common.ErrNegativeValue, "the key is cleared")
	})

	refused := map[string][]byte{
		"a spent record":        mock.NFTHolderRecord(0),
		"a balance of 2":        mock.NFTHolderRecord(2),
		"an undecodable record": {0xff},
	}
	for name, record := range refused {
		t.Run("refuses "+name, func(t *testing.T) {
			account, _ := state.NewUserAccount([]byte("address"))
			assert.NoError(t, account.AddInternalKDA(assetID, nonce, record))

			_, err := account.SubInternalKDA(assetID, nonce, forksV7)

			assert.ErrorIs(t, err, common.ErrAssetNotFound)
			kept, err := account.SubInternalKDA(assetID, nonce, forksBeforeV7)
			assert.NoError(t, err)
			assert.Equal(t, record, kept, "the refused record is left as it was")
		})
	}

	t.Run("refuses a missing record", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))

		_, err := account.SubInternalKDA(assetID, nonce, forksV7)

		assert.Error(t, err)
	})
}

func TestUserAccount_Freeze_InvalidValue(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{Buckets: make(map[string]*kapps.UserBucket)}
	staking := &kapps.StakingData{}

	err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 0, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
	assert.Equal(t, state.ErrInvalidValue, err)

	err = account.Freeze(kdautils.KLVIdentifier, []byte("b1"), -1, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
	assert.Equal(t, state.ErrInvalidValue, err)
}

func TestUserAccount_Freeze_NilBucketsCreatesMap(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{}
	staking := &kapps.StakingData{}
	assert.Nil(t, userKDA.Buckets)

	err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
	assert.NoError(t, err)
	assert.NotNil(t, userKDA.Buckets)
	assert.Equal(t, int64(100), userKDA.FrozenBalance)
}

func TestUserAccount_Freeze_NonKLVAssetUseStringBucketID(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	assetID := []byte("MYTOKEN-1234")
	userKDA := &kapps.UserKDA{Buckets: make(map[string]*kapps.UserBucket)}
	staking := &kapps.StakingData{}

	err := account.Freeze(assetID, []byte("b1"), 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 1, BlockTime: 1000, NewStakingFlow: true})
	assert.NoError(t, err)
	// Non-KLV/KFI assets use string(assetID) as bucket key instead of hex-encoded bucketID
	assert.NotNil(t, userKDA.Buckets[string(assetID)])
	assert.Equal(t, int64(100), userKDA.Buckets[string(assetID)].Value)
}

func TestUserAccount_Freeze_ExistingBucketAccumulatesValue(t *testing.T) {
	bucketID := []byte("b1")
	encodedBucketID := hex.EncodeToString(bucketID)

	t.Run("re-freeze with newStakingFlow and unstaked bucket adds old value to toAdd", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         50,
					StakedEpoch:   1,
					UnstakedEpoch: 5, // not DefaultUnstakedEpoch → unstaked
				},
			},
		}
		staking := &kapps.StakingData{}

		err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 2, BlockTime: 2000, NewStakingFlow: true})
		assert.NoError(t, err)
		assert.Equal(t, int64(150), userKDA.Buckets[encodedBucketID].Value)
		// toAdd = value(100) + oldValue(50) = 150
		assert.Equal(t, int64(150), userKDA.FrozenBalance)
		assert.Equal(t, int64(150), staking.TotalStaked)
	})

	t.Run("re-freeze without newStakingFlow does not add old value to toAdd", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         50,
					StakedEpoch:   1,
					UnstakedEpoch: 5,
				},
			},
		}
		staking := &kapps.StakingData{}

		err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 2, BlockTime: 2000})
		assert.NoError(t, err)
		assert.Equal(t, int64(150), userKDA.Buckets[encodedBucketID].Value)
		// toAdd = value(100) only (no accumulation since newStakingFlow is false)
		assert.Equal(t, int64(100), userKDA.FrozenBalance)
		assert.Equal(t, int64(100), staking.TotalStaked)
	})
}

func TestUserAccount_Freeze_StakeHistory(t *testing.T) {
	assetID := []byte("FPR")
	encodedBucketID := string(assetID)

	activeBucket := func(value int64, stakedEpoch uint32, history []*kapps.StakeSegment) *kapps.UserKDA {
		return &kapps.UserKDA{
			LastClaim: &kapps.LastClaim{Epoch: 1},
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         value,
					StakedEpoch:   stakedEpoch,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
					History:       history,
				},
			},
		}
	}

	t.Run("top up records the stake it replaced", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)

		bucket := userKDA.Buckets[encodedBucketID]
		assert.Equal(t, int64(150), bucket.Value)
		assert.Equal(t, uint32(3), bucket.StakedEpoch)
		if !assert.Len(t, bucket.History, 1) {
			return
		}
		assert.Equal(t, uint32(1), bucket.History[0].StakedEpoch)
		assert.Equal(t, int64(100), bucket.History[0].Value)
	})

	t.Run("top up records nothing while the fork is disabled", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true})
		assert.NoError(t, err)
		assert.Empty(t, userKDA.Buckets[encodedBucketID].History)
	})

	t.Run("re-freezing an unstaked bucket keeps what it earned and closes at the unfreeze", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)
		userKDA.Buckets[encodedBucketID].UnstakedEpoch = 5

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 8, BlockTime: 8000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)

		bucket := userKDA.Buckets[encodedBucketID]
		if !assert.Len(t, bucket.History, 1) {
			return
		}

		// The 100 staked from epoch 1 still prices the epochs it earned, and the interval closes
		// at the unfreeze -- epoch 5 earns its pool, epochs 6 to 8 match no segment at all.
		// Dropping this would forfeit exactly the rewards this change preserves.
		assert.Equal(t, uint32(1), bucket.History[0].StakedEpoch)
		assert.Equal(t, uint32(5), bucket.History[0].ThroughEpoch)
		assert.Equal(t, int64(100), bucket.History[0].Value)
	})

	t.Run("a same-epoch unfreeze and re-freeze records no dead segment", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)
		userKDA.Buckets[encodedBucketID].UnstakedEpoch = 3

		// Unfreezing and re-freezing inside epoch 3 would leave a zero segment covering (3, 3],
		// the same empty interval the same-epoch top-up guard below exists to avoid. The 100
		// staked from epoch 1 still has to survive: without the zero segment its coverage runs to
		// the new StakedEpoch, which is the unfreeze epoch -- and the unfreeze epoch still earns
		// its pool, so that is the correct upper bound.
		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)

		history := userKDA.Buckets[encodedBucketID].History
		if !assert.Len(t, history, 1) {
			return
		}
		assert.Equal(t, uint32(1), history[0].StakedEpoch)
		assert.Equal(t, int64(100), history[0].Value)
	})

	t.Run("repeated same-epoch unfreeze and re-freeze cycles do not grow the history", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)

		// MinEpochsToUnstake is issuer-controlled and may be zero, so a bucket can be unfrozen and
		// re-frozen repeatedly inside one epoch. Every pass after the first rewrites a bucket
		// already staked at that epoch, so neither branch has anything left to record.
		for i := 0; i < 50; i++ {
			userKDA.Buckets[encodedBucketID].UnstakedEpoch = 3

			err := account.Freeze(assetID, nil, 1, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true, KeepStakeHistory: true})
			assert.NoError(t, err)
		}

		assert.Len(t, userKDA.Buckets[encodedBucketID].History, 1)
	})

	t.Run("a same-epoch top up records no dead segment", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 3, nil)

		// bucket.StakedEpoch equals the block epoch, so a segment here would cover (3, 3] -- an
		// interval bucketStakeAt can never satisfy. It would be dead weight in a persisted trie
		// leaf, re-marshalled on every later touch of the account.
		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)
		assert.Empty(t, userKDA.Buckets[encodedBucketID].History)
	})

	t.Run("repeated top ups in one epoch do not grow the history", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 3, nil)

		for i := 0; i < 50; i++ {
			err := account.Freeze(assetID, nil, 1, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 5, BlockTime: 5000, NewStakingFlow: true, KeepStakeHistory: true})
			assert.NoError(t, err)
		}

		// Only the first freeze replaces a bucket staked in an earlier epoch. The other 49 rewrite
		// a bucket already staked at epoch 5, so the list is bounded by distinct epochs rather
		// than by the number of top-ups -- which an issuer controls through MinEpochsToClaim.
		assert.Len(t, userKDA.Buckets[encodedBucketID].History, 1)
	})

	t.Run("segments already covered by a claim are dropped", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 6, []*kapps.StakeSegment{
			{StakedEpoch: 1, ThroughEpoch: 4, Value: 40},
			{StakedEpoch: 4, ThroughEpoch: 6, Value: 70},
		})
		userKDA.LastClaim.Epoch = 5

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 7, BlockTime: 7000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)

		// The (1,4] segment ends at or before the claim cursor, so it is retired; (4,6] straddles
		// it and survives. Each decision reads that segment's own ThroughEpoch.
		history := userKDA.Buckets[encodedBucketID].History
		if !assert.Len(t, history, 2) {
			return
		}
		assert.Equal(t, uint32(4), history[0].StakedEpoch)
		assert.Equal(t, uint32(6), history[0].ThroughEpoch)
		assert.Equal(t, uint32(6), history[1].StakedEpoch)
		assert.Equal(t, uint32(7), history[1].ThroughEpoch)
	})

	t.Run("a segment ending exactly at the claim cursor is dropped", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 6, []*kapps.StakeSegment{
			{StakedEpoch: 1, ThroughEpoch: 5, Value: 40},
		})
		userKDA.LastClaim.Epoch = 5

		// (1,5] is fully paid once the cursor reaches 5, so retention is > and not >=.
		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 7, BlockTime: 7000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)

		history := userKDA.Buckets[encodedBucketID].History
		if !assert.Len(t, history, 1) {
			return
		}
		assert.Equal(t, uint32(6), history[0].StakedEpoch)
		assert.Equal(t, uint32(7), history[0].ThroughEpoch)
	})

	t.Run("top up records nothing when the claim already covers it", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)
		userKDA.LastClaim.Epoch = 9

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 9, BlockTime: 9000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)
		assert.Empty(t, userKDA.Buckets[encodedBucketID].History)
	})

	t.Run("top up without a recorded last claim keeps the segment", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		userKDA := activeBucket(100, 1, nil)
		userKDA.LastClaim = nil

		err := account.Freeze(assetID, nil, 50, &kapps.StakingData{}, userKDA, state.FreezeOptions{BlockEpoch: 3, BlockTime: 3000, NewStakingFlow: true, KeepStakeHistory: true})
		assert.NoError(t, err)
		if !assert.Len(t, userKDA.Buckets[encodedBucketID].History, 1) {
			return
		}
		assert.Equal(t, uint32(1), userKDA.Buckets[encodedBucketID].History[0].StakedEpoch)
	})

	t.Run("reward epochs older than the whole history pay nothing", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()

		userKDA := activeBucket(150, 6, []*kapps.StakeSegment{
			{StakedEpoch: 4, ThroughEpoch: 6, Value: 100},
		})
		userKDA.LastClaim.Epoch = 1

		// Epoch 3 sits below the segment's start, so it must pay nothing. Epoch 5 sits inside
		// (4,6] and must pay, which is what makes the first assertion mean something: without
		// the positive control, dropping the lower-bound check entirely would still pass.
		below := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR:              []*kapps.FPRData{{Epoch: 3, TotalAmount: 500, TotalStaked: 5000}},
		}
		gains, err := account.ComputeAvailableClaim(assetID, 8, 8000, userKDA, below, forkController)
		assert.NoError(t, err)
		assert.Zero(t, gains[string(kdautils.KLVIdentifier)])

		inside := &kapps.StakingData{
			InterestType:     kapps.StakingData_FPRI,
			MinEpochsToClaim: 0,
			FPR:              []*kapps.FPRData{{Epoch: 5, TotalAmount: 500, TotalStaked: 5000}},
		}
		gains, err = account.ComputeAvailableClaim(assetID, 8, 8000, userKDA, inside, forkController)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), gains[string(kdautils.KLVIdentifier)]) // 500/5000 * 100
	})
}

func TestUserAccount_Unfreeze_NilBuckets(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{}
	staking := &kapps.StakingData{}

	_, _, err := account.Unfreeze(kdautils.KLVIdentifier, []byte("b1"), 5, staking, userKDA, true)
	assert.Equal(t, state.ErrNotStaked, err)
}

func TestUserAccount_Unfreeze_BucketNotFound(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{
		Buckets: make(map[string]*kapps.UserBucket),
	}
	staking := &kapps.StakingData{}

	_, _, err := account.Unfreeze(kdautils.KLVIdentifier, []byte("b1"), 5, staking, userKDA, true)
	assert.Equal(t, state.ErrNotStaked, err)
}

func TestUserAccount_Unfreeze_NonKLVAsset(t *testing.T) {
	assetID := []byte("MYTOKEN-1234")
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{
		Buckets: map[string]*kapps.UserBucket{
			string(assetID): {
				Value:         100,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
		FrozenBalance: 100,
	}
	staking := &kapps.StakingData{
		TotalStaked:        100,
		MinEpochsToUnstake: 1,
	}

	_, value, err := account.Unfreeze(assetID, []byte("b1"), 3, staking, userKDA, false)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), value)
}

func TestUserAccount_Unfreeze_NonAPRI_SubtractsBucketValue(t *testing.T) {
	bucketID := []byte("b1")
	encodedBucketID := hex.EncodeToString(bucketID)
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{
		Buckets: map[string]*kapps.UserBucket{
			encodedBucketID: {
				Value:         100,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
		FrozenBalance: 100,
	}
	staking := &kapps.StakingData{
		TotalStaked:        100,
		MinEpochsToUnstake: 1,
		InterestType:       kapps.StakingData_FPRI,
	}

	// newStakingFlow=false → else branch: subtracts bucket.Value from FrozenBalance and TotalStaked
	_, value, err := account.Unfreeze(kdautils.KLVIdentifier, bucketID, 3, staking, userKDA, false)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), value)
	assert.Equal(t, int64(0), userKDA.FrozenBalance)
	assert.Equal(t, int64(0), staking.TotalStaked)
}

func TestUserAccount_Claim_APR_MaxSupplyExceeded(t *testing.T) {
	account := state.NewEmptyUserAccount()
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{
			Timestamp: 1000,
			Epoch:     1,
		},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {
				Value:         1_000_000,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR: []*kapps.APRData{
			{Timestamp: 2000, Value: 1000},
		},
	}
	kda := &kapps.KDAData{
		MaxSupply:         10,
		CirculatingSupply: 10,
	}
	forkController := &mock.ForkControllerStub{}

	_, err := account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, 3000,
		staking, kda, userKDA, forkController,
	)
	assert.Equal(t, common.ErrMaxSupplyExceeded, err)
}

func TestUserAccount_Claim_APR_UpdatesCirculatingSupply(t *testing.T) {
	account := state.NewEmptyUserAccount()
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{
			Timestamp: 1000,
			Epoch:     1,
		},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {
				Value:         1_000_000,
				StakedEpoch:   1,
				UnstakedEpoch: core.DefaultUnstakedEpoch,
			},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR: []*kapps.APRData{
			{Timestamp: 2000, Value: 1000},
		},
	}
	kda := &kapps.KDAData{
		MaxSupply:         1_000_000,
		CirculatingSupply: 0,
	}
	forkController := &mock.ForkControllerStub{}

	gains, err := account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, 3000,
		staking, kda, userKDA, forkController,
	)
	assert.NoError(t, err)
	klvGains := gains["KLV"]
	assert.True(t, klvGains > 0)
	assert.Equal(t, klvGains, kda.CirculatingSupply)
	assert.Equal(t, klvGains, kda.MintedValue)
}

const aprClaimStart = int64(1000)

func aprClaimInputs(stake int64, aprs ...*kapps.APRData) (*kapps.UserKDA, *kapps.StakingData) {
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Timestamp: aprClaimStart, Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: stake, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          aprs,
	}

	return userKDA, staking
}

func claimOneYearAtFullAPR(t *testing.T, kda *kapps.KDAData, fixV6 bool) (map[string]int64, error) {
	t.Helper()

	account := state.NewEmptyUserAccount()
	userKDA, staking := aprClaimInputs(1_000_000, &kapps.APRData{Timestamp: aprClaimStart, Value: core.HundredPercent})

	return account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, aprClaimStart+core.OneYearTimestamp,
		staking, kda, userKDA, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixV6),
	)
}

func TestUserAccount_Claim_APR_CreditsGainWithinRemainingSupply(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{MaxSupply: 2_000_000, CirculatingSupply: 1_000_000, MintedValue: 1_000_000}

	gains, err := claimOneYearAtFullAPR(t, kda, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(1_000_000), gains["KLV"])
	assert.Equal(t, int64(2_000_000), kda.CirculatingSupply)
	assert.Equal(t, int64(2_000_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_DropsGainAboveRemainingSupply(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{
		MaxSupply:         math.MaxInt64 - 100_000,
		CirculatingSupply: math.MaxInt64 - 500_000,
		MintedValue:       math.MaxInt64 - 500_000,
	}

	_, err := claimOneYearAtFullAPR(t, kda, true)
	assert.ErrorIs(t, err, state.ErrAPRGainDropped)
	assert.ErrorIs(t, err, common.ErrMaxSupplyExceeded)
	assert.Equal(t, int64(math.MaxInt64-500_000), kda.CirculatingSupply)
	assert.Equal(t, int64(math.MaxInt64-500_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_KeepsLegacySupplyArithmeticWithoutV6(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{
		MaxSupply:         math.MaxInt64 - 100_000,
		CirculatingSupply: math.MaxInt64 - 500_000,
		MintedValue:       math.MaxInt64 - 500_000,
	}

	_, err := claimOneYearAtFullAPR(t, kda, false)
	assert.NoError(t, err)
	assert.Less(t, kda.CirculatingSupply, int64(0))
	assert.Less(t, kda.MintedValue, int64(0))
}

func TestUserAccount_Claim_APR_ForfeitsGainOverflowingUncappedSupply(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{
		MaxSupply:         0,
		CirculatingSupply: math.MaxInt64 - 500_000,
		MintedValue:       math.MaxInt64 - 500_000,
	}

	gains, err := claimOneYearAtFullAPR(t, kda, true)
	assert.ErrorIs(t, err, state.ErrAPRGainDropped)
	assert.ErrorIs(t, err, common.ErrInt64Overflow)
	assert.Zero(t, gains["KLV"])
	assert.Equal(t, int64(math.MaxInt64-500_000), kda.CirculatingSupply)
	assert.Equal(t, int64(math.MaxInt64-500_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_ForfeitsSaturatedGain(t *testing.T) {
	t.Parallel()

	account := state.NewEmptyUserAccount()
	userKDA, staking := aprClaimInputs(1_000_000_000_000_000_000, &kapps.APRData{Timestamp: aprClaimStart, Value: core.MaxStakingAPR})
	kda := &kapps.KDAData{MaxSupply: 0, CirculatingSupply: 1_000_000_000_000_000_000, MintedValue: 1_000_000_000_000_000_000}

	gains, err := account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, aprClaimStart+core.OneYearTimestamp,
		staking, kda, userKDA, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", true),
	)
	assert.ErrorIs(t, err, state.ErrAPRGainDropped)
	assert.ErrorIs(t, err, common.ErrInt64Overflow)
	assert.Zero(t, gains["KLV"])
	assert.Equal(t, int64(1_000_000_000_000_000_000), kda.CirculatingSupply)
	assert.Equal(t, int64(1_000_000_000_000_000_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_SaturatesMintedValue(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{
		MaxSupply:         0,
		CirculatingSupply: 1_000,
		MintedValue:       math.MaxInt64 - 500_000,
	}

	gains, err := claimOneYearAtFullAPR(t, kda, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(1_000_000), gains["KLV"])
	assert.Equal(t, int64(1_001_000), kda.CirculatingSupply)
	assert.Equal(t, int64(math.MaxInt64), kda.MintedValue)
}

func TestUserAccount_Claim_APR_ForfeitsGainOnNegativeSupply(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{MaxSupply: 0, CirculatingSupply: -1, MintedValue: 1_000}

	gains, err := claimOneYearAtFullAPR(t, kda, true)
	assert.ErrorIs(t, err, state.ErrAPRGainDropped)
	assert.ErrorIs(t, err, state.ErrNegativeCirculatingSupply)
	assert.Zero(t, gains["KLV"])
	assert.Equal(t, int64(-1), kda.CirculatingSupply)
	assert.Equal(t, int64(1_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_CreditsGainWithNegativeMintedValue(t *testing.T) {
	t.Parallel()

	kda := &kapps.KDAData{MaxSupply: 0, CirculatingSupply: 1_000, MintedValue: -1_000_000_000}

	gains, err := claimOneYearAtFullAPR(t, kda, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(1_000_000), gains["KLV"])
	assert.Equal(t, int64(1_001_000), kda.CirculatingSupply)
	assert.Equal(t, int64(-999_000_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_SkipsOverflowingGainOnNegativeSupply(t *testing.T) {
	t.Parallel()

	account := state.NewEmptyUserAccount()
	userKDA, staking := aprClaimInputs(1_000_000_000_000_000, &kapps.APRData{Timestamp: aprClaimStart, Value: math.MaxUint32})
	kda := &kapps.KDAData{MaxSupply: 0, CirculatingSupply: -1, MintedValue: 1_000}

	gains, err := account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, aprClaimStart+9*86400,
		staking, kda, userKDA, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", true),
	)
	assert.ErrorIs(t, err, state.ErrAPRGainDropped)
	assert.ErrorIs(t, err, common.ErrInt64Overflow)
	assert.Zero(t, gains["KLV"])
	assert.Equal(t, int64(-1), kda.CirculatingSupply)
	assert.Equal(t, int64(1_000), kda.MintedValue)
}

func TestUserAccount_Claim_APR_ZeroGainAboveMaxSupply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		fixV6         bool
		expectedError error
	}{
		{name: "with fix", fixV6: true, expectedError: nil},
		{name: "before fix", fixV6: false, expectedError: common.ErrMaxSupplyExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := state.NewEmptyUserAccount()
			userKDA, staking := aprClaimInputs(1_000_000, &kapps.APRData{Timestamp: aprClaimStart, Value: core.HundredPercent})
			userKDA.LastClaim = nil
			kda := &kapps.KDAData{MaxSupply: 10, CirculatingSupply: 20, MintedValue: 20}

			_, err := account.Claim(
				transaction.ClaimContract_StakingClaim,
				kdautils.KLVIdentifier, 2, aprClaimStart+core.OneYearTimestamp,
				staking, kda, userKDA, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", tt.fixV6),
			)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, int64(20), kda.CirculatingSupply)
			assert.Equal(t, int64(20), kda.MintedValue)
		})
	}
}

func TestUserAccount_ComputeClaimAPR_SameGainWithAndWithoutV6(t *testing.T) {
	t.Parallel()

	account := state.NewEmptyUserAccount()
	compute := func(fixV6 bool) int64 {
		userKDA, staking := aprClaimInputs(1_000_000, &kapps.APRData{Timestamp: aprClaimStart, Value: 1000})
		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 2, aprClaimStart+86400, userKDA, staking, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", fixV6),
		)
		assert.NoError(t, err)

		return gains["KLV"]
	}

	withV6 := compute(true)
	assert.Equal(t, int64(273), withV6)
	assert.Equal(t, compute(false), withV6)
}

func TestUserAccount_ComputeClaimAPR_GainLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		stake         int64
		aprs          []*kapps.APRData
		claimAt       int64
		fixV6         bool
		expectedGain  int64
		expectedError error
	}{
		{
			name:          "saturated gain with fix",
			stake:         10_000_000_000_000_000,
			aprs:          []*kapps.APRData{{Timestamp: aprClaimStart, Value: math.MaxUint32}},
			claimAt:       aprClaimStart + 2*86400,
			fixV6:         true,
			expectedError: common.ErrInt64Overflow,
		},
		{
			name:         "saturated gain before fix",
			stake:        10_000_000_000_000_000,
			aprs:         []*kapps.APRData{{Timestamp: aprClaimStart, Value: math.MaxUint32}},
			claimAt:      aprClaimStart + 2*86400,
			expectedGain: math.MaxInt64,
		},
		{
			name:  "overflowing segment sum with fix",
			stake: 5_000_000_000_000_000_000,
			aprs: []*kapps.APRData{
				{Timestamp: aprClaimStart, Value: core.HundredPercent},
				{Timestamp: aprClaimStart + core.OneYearTimestamp, Value: core.HundredPercent},
			},
			claimAt:       aprClaimStart + 2*core.OneYearTimestamp,
			fixV6:         true,
			expectedError: common.ErrInt64Overflow,
		},
		{
			name:  "overflowing segment sum before fix",
			stake: 5_000_000_000_000_000_000,
			aprs: []*kapps.APRData{
				{Timestamp: aprClaimStart, Value: core.HundredPercent},
				{Timestamp: aprClaimStart + core.OneYearTimestamp, Value: core.HundredPercent},
			},
			claimAt:      aprClaimStart + 2*core.OneYearTimestamp,
			expectedGain: -8_446_744_073_709_551_616,
		},
		{
			name:          "negative gain with fix",
			stake:         1_000_000_000_000,
			aprs:          []*kapps.APRData{{Timestamp: aprClaimStart - 500, Value: 1000}},
			claimAt:       aprClaimStart - 100,
			fixV6:         true,
			expectedError: common.ErrNegativeValue,
		},
		{
			name:          "single gain at 2^63",
			stake:         math.MaxInt64,
			aprs:          []*kapps.APRData{{Timestamp: aprClaimStart, Value: core.HundredPercent}},
			claimAt:       aprClaimStart + core.OneYearTimestamp,
			fixV6:         true,
			expectedError: common.ErrInt64Overflow,
		},
		{
			name:         "single gain below 2^63",
			stake:        1<<63 - 1<<44,
			aprs:         []*kapps.APRData{{Timestamp: aprClaimStart, Value: core.HundredPercent}},
			claimAt:      aprClaimStart + core.OneYearTimestamp,
			fixV6:        true,
			expectedGain: 1<<63 - 1<<44,
		},
		{
			name:  "segment sum at 2^63",
			stake: 1 << 61,
			aprs: []*kapps.APRData{
				{Timestamp: aprClaimStart, Value: core.HundredPercent},
				{Timestamp: aprClaimStart + core.OneYearTimestamp, Value: 3 * core.HundredPercent},
			},
			claimAt:       aprClaimStart + 2*core.OneYearTimestamp,
			fixV6:         true,
			expectedError: common.ErrInt64Overflow,
		},
		{
			name:  "segment sum below 2^63",
			stake: 1<<61 - 1<<43,
			aprs: []*kapps.APRData{
				{Timestamp: aprClaimStart, Value: core.HundredPercent},
				{Timestamp: aprClaimStart + core.OneYearTimestamp, Value: 3 * core.HundredPercent},
			},
			claimAt:      aprClaimStart + 2*core.OneYearTimestamp,
			fixV6:        true,
			expectedGain: 1<<63 - 1<<45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := state.NewEmptyUserAccount()
			userKDA, staking := aprClaimInputs(tt.stake, tt.aprs...)

			gains, err := account.ComputeAvailableClaim(
				kdautils.KLVIdentifier, 2, tt.claimAt, userKDA, staking, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", tt.fixV6),
			)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedGain, gains["KLV"])
		})
	}
}

func TestAPRGainDropReason(t *testing.T) {
	t.Parallel()

	apr := &kapps.StakingData{InterestType: kapps.StakingData_APRI}
	gains := map[string]int64{"KLV": 1_000}

	tests := []struct {
		name       string
		staking    *kapps.StakingData
		kda        *kapps.KDAData
		gains      map[string]int64
		computeErr error
		fixV6      bool
		reason     error
	}{
		{name: "gain overflowing int64", staking: apr, kda: &kapps.KDAData{}, computeErr: common.ErrInt64Overflow, fixV6: true, reason: common.ErrInt64Overflow},
		{name: "gain on negative supply", staking: apr, kda: &kapps.KDAData{CirculatingSupply: -1}, gains: gains, fixV6: true, reason: state.ErrNegativeCirculatingSupply},
		{name: "gain overflowing uncapped supply", staking: apr, kda: &kapps.KDAData{CirculatingSupply: math.MaxInt64 - 999}, gains: gains, fixV6: true, reason: common.ErrInt64Overflow},
		{name: "gain fitting uncapped supply", staking: apr, kda: &kapps.KDAData{CirculatingSupply: math.MaxInt64 - 1_000}, gains: gains, fixV6: true},
		{name: "gain above the cap", staking: apr, kda: &kapps.KDAData{MaxSupply: 10, CirculatingSupply: 10}, gains: gains, fixV6: true, reason: common.ErrMaxSupplyExceeded},
		{name: "gain filling the cap", staking: apr, kda: &kapps.KDAData{MaxSupply: 10_000, CirculatingSupply: 9_000}, gains: gains, fixV6: true},
		{name: "zero gain on negative supply", staking: apr, kda: &kapps.KDAData{CirculatingSupply: -1}, fixV6: true},
		{name: "other computation error", staking: apr, kda: &kapps.KDAData{CirculatingSupply: -1}, gains: gains, computeErr: common.ErrNegativeValue, fixV6: true},
		{name: "FPR staking", staking: &kapps.StakingData{InterestType: kapps.StakingData_FPRI}, kda: &kapps.KDAData{CirculatingSupply: -1}, gains: gains, fixV6: true},
		{name: "before fix", staking: apr, kda: &kapps.KDAData{CirculatingSupply: -1}, gains: gains, computeErr: common.ErrInt64Overflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := state.APRGainDropReason(
				tt.staking, tt.kda, tt.gains, kdautils.KLVIdentifier, tt.computeErr, mock.NewForkControllerStub().SetFork("FixAuditChangesV6", tt.fixV6),
			)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestUserAccount_GetBalanceWithNonce_CustomAsset(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	customAsset := []byte("TOKEN")

	_ = account.AddToBalance(500, customAsset, true)
	balance := account.GetBalanceWithNonce(customAsset, nil, true)
	assert.Equal(t, int64(500), balance)
}

func TestUserAccount_GetFrozenBalance_CustomAsset(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	customAsset := []byte("TOKEN")
	userKDA, _ := account.GetUserKDA(customAsset, nil, true)
	userKDA.FrozenBalance = 200
	_ = account.SetUserKDA(customAsset, nil, userKDA)

	frozen := account.GetFrozenBalance(customAsset, true)
	assert.Equal(t, int64(200), frozen)
}

func TestUserAccount_GetBuckets_CustomAsset(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	customAsset := []byte("TOKEN")
	userKDA, _ := account.GetUserKDA(customAsset, nil, true)
	userKDA.Buckets = map[string]*kapps.UserBucket{"b1": {Value: 100}}
	_ = account.SetUserKDA(customAsset, nil, userKDA)

	buckets := account.GetBuckets(customAsset, true)
	assert.Len(t, buckets, 1)
	assert.Equal(t, int64(100), buckets["b1"].Value)
}

func TestUserAccount_NewUserAccountNoCheck(t *testing.T) {
	t.Parallel()

	acc := state.NewUserAccountNoCheck([]byte("addr"))
	assert.NotNil(t, acc)
	assert.Equal(t, []byte("addr"), acc.AddressBytes())
}

func TestUserAccount_GetUserKDA_NilAssetIDDefaultsToKLV(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	// Add a custom asset balance to create dirty data so GetUserKDA doesn't early-return
	_ = account.AddToBalance(500, []byte("CUSTOM"), true)

	// nil assetID should default to KLV; KLV has no data in trie, returns empty
	userKDA, err := account.GetUserKDA(nil, nil, true)
	assert.NoError(t, err)
	assert.NotNil(t, userKDA)
	assert.Equal(t, int64(0), userKDA.Balance)
}

func TestUserAccount_SetUserKDA_NilAssetIDDefaultsToKLV(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{
		Balance:   999,
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	err := account.SetUserKDA(nil, nil, userKDA)
	assert.NoError(t, err)

	// Verify by reading back with KLV identifier
	got, err := account.GetUserKDA(kdautils.KLVIdentifier, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(999), got.Balance)
}

func TestUserAccount_GetUserKDA_UnmarshalError(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	// Save invalid proto data into dirty cache
	key := kdautils.ToKDAKey([]byte("BAD"), nil)
	_ = account.SaveKeyValue(key, []byte("not-valid-proto-data"))

	_, err := account.GetUserKDA([]byte("BAD"), nil, true)
	assert.Error(t, err)
}

func TestUserAccount_GetBalanceWithNonce_ErrorReturnsZero(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	key := kdautils.ToKDAKey([]byte("BAD"), nil)
	_ = account.SaveKeyValue(key, []byte("not-valid-proto"))

	balance := account.GetBalanceWithNonce([]byte("BAD"), nil, true)
	assert.Equal(t, int64(0), balance)
}

func TestUserAccount_GetFrozenBalance_ErrorReturnsZero(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	key := kdautils.ToKDAKey([]byte("BAD"), nil)
	_ = account.SaveKeyValue(key, []byte("not-valid-proto"))

	frozen := account.GetFrozenBalance([]byte("BAD"), true)
	assert.Equal(t, int64(0), frozen)
}

func TestUserAccount_GetBuckets_ErrorReturnsNil(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	key := kdautils.ToKDAKey([]byte("BAD"), nil)
	_ = account.SaveKeyValue(key, []byte("not-valid-proto"))

	buckets := account.GetBuckets([]byte("BAD"), true)
	assert.Nil(t, buckets)
}

func TestUserAccount_Equal_DifferentBaseAccount(t *testing.T) {
	t.Parallel()

	acc1, _ := state.NewUserAccount([]byte("address-1"))
	acc2, _ := state.NewUserAccount([]byte("address-2"))

	// Same proto data but different baseAccount addresses
	assert.False(t, acc1.Equal(acc2))
}

func TestUserAccount_Equal_WrongType(t *testing.T) {
	t.Parallel()

	acc1, _ := state.NewUserAccount([]byte("address"))
	peerAcc, _ := state.NewPeerAccount([]byte("address"))

	// PeerAccount doesn't implement UserAccountHandler, so Equal returns false
	// We test nil which is also not a *userAccount
	assert.False(t, acc1.Equal(nil))
	_ = peerAcc // peerAccount is not UserAccountHandler, can't pass directly
}

// Additional tests for coverage improvements

func TestUserAccount_ClaimStaking_ComputeAvailableClaimError(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	// Invalid stake type should cause ComputeAvailableClaim to fail
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_EnumInterestType(99),
	}
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	_, err := account.Claim(
		transaction.ClaimContract_StakingClaim,
		kdautils.KLVIdentifier, 2, 1000,
		staking, &kapps.KDAData{}, userKDA, forkController,
	)
	assert.Equal(t, state.ErrInvalidStakeType, err)
}

func TestUserAccount_ComputeClaimAPR_NilLastClaim(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: nil,
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_ComputeClaimAPR_ZeroTimestamp(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Timestamp: 0, Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_ComputeClaimAPR_NilBuckets(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Timestamp: 1000, Epoch: 1},
		Buckets:   nil,
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_ComputeClaimAPR_UnstakedBucket(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Timestamp: 1000, Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: 5}, // unstaked
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_ComputeClaimAPR_ZeroAmount(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Timestamp: 1000, Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: 0, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_CalculateFPRAmount_BigBucketsWithZeroTotalAmount(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()
	forkController.BigBucketsComputeValue = true

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 0,
		FPR: []*kapps.FPRData{
			{
				Epoch:       2,
				TotalAmount: 0, // zero total amount
				TotalStaked: 5000,
			},
		},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), gains[string(kdautils.KLVIdentifier)])
}

func TestUserAccount_CalculateFPRAmount_BigBucketsWithZeroTotalStaked(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()
	forkController.BigBucketsComputeValue = true

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 0,
		FPR: []*kapps.FPRData{
			{
				Epoch:       2,
				TotalAmount: 100,
				TotalStaked: 0, // zero total staked
			},
		},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), gains[string(kdautils.KLVIdentifier)])
}

func TestUserAccount_CalculateFPRAmount_BigBucketsCappedByClaimed(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()
	forkController.BigBucketsComputeValue = true

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 5000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 0,
		FPR: []*kapps.FPRData{
			{
				Epoch:        2,
				TotalAmount:  100,
				TotalStaked:  5000,
				TotalClaimed: 90, // already claimed 90, can only get 10 more
			},
		},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	// calculated = 100/5000 * 5000 = 100, but totalClaimed(90) + 100 > totalAmount(100)
	// so capped to totalAmount - totalClaimed = 100 - 90 = 10
	assert.Equal(t, int64(10), gains[string(kdautils.KLVIdentifier)])
}

func TestUserAccount_CalculateFPRAmount_FPRComputeAndKdaFeeFlowNegativeClamped(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	forkController := mock.NewForkControllerStub()
	forkController.BigBucketsComputeValue = true
	forkController.FPRComputeAndKdaFeeFlowValue = true

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 0,
		FPR: []*kapps.FPRData{
			{
				Epoch:        2,
				TotalAmount:  50,
				TotalStaked:  5000,
				TotalClaimed: 60, // claimed more than total (edge case)
			},
		},
	}

	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 3, 2000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	// calculated would be negative after capping, but FPRComputeAndKdaFeeFlow clamps to 0
	assert.Equal(t, int64(0), gains[string(kdautils.KLVIdentifier)])
}

func TestUserAccount_AddToBalanceWithNonce_RoundTrip(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	assetID := []byte("MYTOKEN")
	nonce := []byte("1")

	err := account.AddToBalanceWithNonce(100, assetID, nonce, true)
	assert.NoError(t, err)

	balance := account.GetBalanceWithNonce(assetID, nonce, true)
	assert.Equal(t, int64(100), balance)
}

func TestUserAccount_SubFromBalanceWithNonce_RoundTrip(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	assetID := []byte("MYTOKEN")
	nonce := []byte("1")

	err := account.AddToBalanceWithNonce(200, assetID, nonce, true)
	assert.NoError(t, err)

	err = account.SubFromBalanceWithNonce(100, assetID, nonce, true)
	assert.NoError(t, err)

	balance := account.GetBalanceWithNonce(assetID, nonce, true)
	assert.Equal(t, int64(100), balance)
}

func TestUserAccount_SubInternalKDA_AddThenSubThenSubAgain(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	assetID := []byte("KDA-TOKEN")
	internalID := []byte("internal-123")
	kdaData := []byte("kda-data")

	err := account.AddInternalKDA(assetID, internalID, kdaData)
	assert.NoError(t, err)

	retrieved, err := account.SubInternalKDA(assetID, internalID, forksBeforeV7)
	assert.NoError(t, err)
	assert.Equal(t, kdaData, retrieved)

	// Sub again on removed key returns error
	_, err = account.SubInternalKDA(assetID, internalID, forksBeforeV7)
	assert.Error(t, err)
}

func TestUserAccount_SubFromBalance_CustomAssetRoundTrip(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	customAsset := []byte("TOKEN")

	err := account.AddToBalance(500, customAsset, true)
	assert.NoError(t, err)

	err = account.SubFromBalance(200, customAsset, true)
	assert.NoError(t, err)

	balance := account.GetBalance(customAsset, true)
	assert.Equal(t, int64(300), balance)
}

func TestUserAccount_SubInternalKDA_AssetNotFound(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))

	// Set a data trie that returns nil for all keys
	account.SetDataTrie(&mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) { return nil, nil },
	})

	// SubInternalKDA for a missing key returns ErrAssetNotFound
	_, err := account.SubInternalKDA([]byte("ASSET"), []byte("nonce"), forksBeforeV7)
	assert.Equal(t, common.ErrAssetNotFound, err)
}

func TestUserAccount_Equal_TypeAssertionAndBaseAccount(t *testing.T) {
	t.Parallel()

	addr1 := make([]byte, 32)
	addr1[0] = 1
	acc1, _ := state.NewUserAccount(addr1)
	acc1.Balance = 500

	// AccountWrapMock implements UserAccountHandler but is NOT *userAccount
	// so Equal returns false on type assertion
	wrapMock := &mock.AccountWrapMock{}
	assert.False(t, acc1.Equal(wrapMock))

	// Two real userAccounts with different addresses have different baseAccount
	addr2 := make([]byte, 32)
	addr2[0] = 2
	acc2, _ := state.NewUserAccount(addr2)
	acc2.Balance = 500
	assert.False(t, acc1.Equal(acc2))

	// Same address, same data → should be equal
	acc3, _ := state.NewUserAccount(addr1)
	acc3.Balance = 500
	assert.True(t, acc1.Equal(acc3))

	// Same address, different balance → not equal (proto difference)
	acc4, _ := state.NewUserAccount(addr1)
	acc4.Balance = 999
	assert.False(t, acc1.Equal(acc4))
}

func TestUserAccount_AddToBalanceWithNonce_Validations(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	assetID := []byte("TOKEN")

	// zero value is a no-op
	err := account.AddToBalanceWithNonce(0, assetID, []byte("1"), true)
	assert.NoError(t, err)

	// invalid nonce string
	err = account.AddToBalanceWithNonce(100, assetID, []byte("abc"), true)
	assert.Error(t, err)

	// zero nonce → ErrInvalidNonce
	err = account.AddToBalanceWithNonce(100, assetID, []byte("0"), true)
	assert.Equal(t, state.ErrInvalidNonce, err)

	// negative nonce → ErrInvalidNonce
	err = account.AddToBalanceWithNonce(100, assetID, []byte("-1"), true)
	assert.Equal(t, state.ErrInvalidNonce, err)
}

func TestUserAccount_AddToBalanceWithNonce_OverflowAndPreloadedKDA(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	assetID := []byte("TOKEN")
	nonce := []byte("1")

	// Pre-load a UserKDA with max balance and pass it directly via vararg
	userKDA := &kapps.UserKDA{
		Balance:   math.MaxInt64,
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}
	err := account.AddToBalanceWithNonce(1, assetID, nonce, true, userKDA)
	assert.Equal(t, state.ErrAddBalanceOverflow, err)

	// Passing pre-loaded KDA with room to add succeeds and updates the KDA in-place
	userKDA2 := &kapps.UserKDA{
		Balance:   100,
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}
	err = account.AddToBalanceWithNonce(50, assetID, []byte("5"), true, userKDA2)
	assert.NoError(t, err)
	assert.Equal(t, int64(150), userKDA2.Balance)
}

func TestUserAccount_SubFromBalanceWithNonce_Validations(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	assetID := []byte("TOKEN")

	// negative value → ErrInvalidValue
	err := account.SubFromBalanceWithNonce(-100, assetID, []byte("1"), true)
	assert.Equal(t, state.ErrInvalidValue, err)

	// zero nonce → ErrInvalidNonce
	err = account.SubFromBalanceWithNonce(100, assetID, []byte("0"), true)
	assert.Equal(t, state.ErrInvalidNonce, err)
}

func TestUserAccount_Withdraw_NoEligibleBuckets(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	bucketID := []byte("bucket1")
	encodedBucketID := hex.EncodeToString(bucketID)

	userKDA := &kapps.UserKDA{
		Buckets: map[string]*kapps.UserBucket{
			encodedBucketID: {
				Value:         100,
				StakedEpoch:   1,
				UnstakedEpoch: 10,
			},
		},
	}

	// epoch=5, minEpochsToWithdraw=10 → 5 < 10+10 → not eligible
	_, err := account.Withdraw(kdautils.KLVIdentifier, 5, 10, userKDA)
	assert.Equal(t, state.ErrWithdrawNotAvailable, err)
}

func TestUserAccount_ComputeAvailableClaim_APRIWithZeroTimestamp(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	forkController := mock.NewForkControllerStub()

	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{
			Timestamp: 0,
			Epoch:     1,
		},
		Buckets: map[string]*kapps.UserBucket{
			"KLV": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType: kapps.StakingData_APRI,
		APR:          []*kapps.APRData{{Timestamp: 2000, Value: 1000}},
	}

	// Zero timestamp → nil gains (no claim computed)
	gains, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 2, 3000, userKDA, staking, forkController,
	)
	assert.NoError(t, err)
	assert.Nil(t, gains)
}

func TestUserAccount_ComputeAvailableClaim_FPRIEdgeCases(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount(make([]byte, 32))
	forkController := mock.NewForkControllerStub()

	// Epoch not yet met for claim
	userKDA := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 10},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 1000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 5,
		FPR: []*kapps.FPRData{
			{Epoch: 12, TotalAmount: 100, TotalStaked: 5000},
		},
	}

	// blockEpoch(11) - lastClaimEpoch(10) = 1 < minEpochsToClaim(5)
	_, err := account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 11, 3000, userKDA, staking, forkController,
	)
	assert.Equal(t, state.ErrClaimNotAvailable, err)

	// Inconsistent data: bucket.Value > TotalStaked (old path without BigBucketsCompute)
	forkController.BigBucketsComputeValue = false
	userKDA2 := &kapps.UserKDA{
		LastClaim: &kapps.LastClaim{Epoch: 1},
		Buckets: map[string]*kapps.UserBucket{
			"b1": {Value: 10000, StakedEpoch: 1, UnstakedEpoch: core.DefaultUnstakedEpoch},
		},
	}
	staking2 := &kapps.StakingData{
		InterestType:     kapps.StakingData_FPRI,
		MinEpochsToClaim: 0,
		FPR: []*kapps.FPRData{
			{Epoch: 2, TotalAmount: 100, TotalStaked: 5000},
		},
	}

	_, err = account.ComputeAvailableClaim(
		kdautils.KLVIdentifier, 3, 3000, userKDA2, staking2, forkController,
	)
	assert.Equal(t, state.ErrInconsistentStakingData, err)
}

func TestUserAccount_Withdraw_NonKLVWithMultipleBuckets(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	customAsset := []byte("MYTOKEN")
	bA := hex.EncodeToString([]byte("bucketA"))
	bB := hex.EncodeToString([]byte("bucketB"))
	bC := hex.EncodeToString([]byte("bucketC"))

	userKDA := &kapps.UserKDA{
		Balance: 50,
		Buckets: map[string]*kapps.UserBucket{
			bA: {
				Value:         100,
				StakedEpoch:   1,
				UnstakedEpoch: 2,
			},
			bB: {
				Value:         200,
				StakedEpoch:   1,
				UnstakedEpoch: 10,
			},
			bC: {
				Value:         300,
				StakedEpoch:   1,
				UnstakedEpoch: 3,
			},
		},
	}

	// epoch=15, minEpochsToWithdraw=5
	// bucketA: 15 >= 2+5=7 => eligible
	// bucketB: 15 >= 10+5=15 => eligible (edge case)
	// bucketC: 15 >= 3+5=8 => eligible
	withdrawn, err := account.Withdraw(customAsset, 15, 5, userKDA)
	assert.NoError(t, err)
	assert.Equal(t, int64(600), withdrawn)
	assert.Equal(t, int64(650), userKDA.Balance)
	assert.Equal(t, int64(0), account.Balance)
	assert.Len(t, userKDA.Buckets, 0)
}

func TestUserAccount_Unfreeze_Scenarios(t *testing.T) {
	t.Parallel()

	t.Run("already unstaked bucket returns error", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         100,
					StakedEpoch:   1,
					UnstakedEpoch: 5,
				},
			},
		}
		staking := &kapps.StakingData{
			MinEpochsToUnstake: 2,
		}

		_, _, err := account.Unfreeze(kdautils.KLVIdentifier, bucketID, 10, staking, userKDA, true)
		assert.Equal(t, state.ErrUnstakeNotAvailable, err)
	})

	t.Run("epoch too early for unstake returns error", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         100,
					StakedEpoch:   5,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			MinEpochsToUnstake: 10,
		}

		// blockEpoch(7) - stakedEpoch(5) = 2 < minEpochsToUnstake(10)
		_, _, err := account.Unfreeze(kdautils.KLVIdentifier, bucketID, 7, staking, userKDA, true)
		assert.Equal(t, state.ErrUnstakeNotAvailable, err)
	})

	t.Run("non-KLV non-KFI uses string assetID as bucket key", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		customAsset := []byte("CUSTOM-ASSET")
		bucketID := []byte("bucket1")

		userKDA := &kapps.UserKDA{
			FrozenBalance: 100,
			Buckets: map[string]*kapps.UserBucket{
				string(customAsset): {
					Value:         100,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			TotalStaked:        100,
			MinEpochsToUnstake: 2,
		}

		delegation, value, err := account.Unfreeze(customAsset, bucketID, 5, staking, userKDA, false)
		assert.NoError(t, err)
		assert.Nil(t, delegation)
		assert.Equal(t, int64(100), value)
		assert.Equal(t, uint32(5), userKDA.Buckets[string(customAsset)].UnstakedEpoch)
		assert.Equal(t, int64(0), userKDA.FrozenBalance)
		assert.Equal(t, int64(0), staking.TotalStaked)
	})

	t.Run("APRI with newStakingFlow resets FrozenBalance", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		bucketID := []byte("bucket1")
		encodedBucketID := hex.EncodeToString(bucketID)

		userKDA := &kapps.UserKDA{
			FrozenBalance: 500,
			Buckets: map[string]*kapps.UserBucket{
				encodedBucketID: {
					Value:         100,
					StakedEpoch:   1,
					UnstakedEpoch: core.DefaultUnstakedEpoch,
				},
			},
		}
		staking := &kapps.StakingData{
			TotalStaked:        500,
			MinEpochsToUnstake: 2,
			InterestType:       kapps.StakingData_APRI,
		}

		_, _, err := account.Unfreeze(kdautils.KLVIdentifier, bucketID, 5, staking, userKDA, true)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), userKDA.FrozenBalance)
		assert.Equal(t, int64(0), staking.TotalStaked)
	})
}

func TestUserAccount_Freeze_NewStakingFlowAccumulation(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))
	bucketID := []byte("bucket1")
	encodedBucketID := hex.EncodeToString(bucketID)

	// Create bucket with existing UnstakedEpoch != DefaultUnstakedEpoch and value=50
	userKDA := &kapps.UserKDA{
		Buckets: map[string]*kapps.UserBucket{
			encodedBucketID: {
				Value:         50,
				StakedEpoch:   1,
				UnstakedEpoch: 5,
			},
		},
	}
	staking := &kapps.StakingData{}

	// Freeze with newStakingFlow=true and value=100
	err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, staking, userKDA, state.FreezeOptions{BlockEpoch: 10, BlockTime: 5000, NewStakingFlow: true})
	assert.NoError(t, err)

	// Verify: toAdd includes oldValue (50+100=150), bucket.Value = old+new (150), FrozenBalance += 150
	assert.Equal(t, int64(150), userKDA.Buckets[encodedBucketID].Value)
	assert.Equal(t, int64(150), userKDA.FrozenBalance)
	assert.Equal(t, int64(150), staking.TotalStaked)
	assert.Equal(t, uint32(core.DefaultUnstakedEpoch), userKDA.Buckets[encodedBucketID].UnstakedEpoch)
}

func TestUserAccount_SubFromBalanceWithNonce_OverflowAndPreloadedKDA(t *testing.T) {
	t.Parallel()

	t.Run("overflow when newBalance goes negative", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		assetID := []byte("MYTOKEN")
		nonce := []byte("1")

		err := account.AddToBalanceWithNonce(100, assetID, nonce, true)
		assert.NoError(t, err)

		err = account.SubFromBalanceWithNonce(150, assetID, nonce, true)
		assert.Equal(t, state.ErrAddBalanceOverflow, err)

		balance := account.GetBalanceWithNonce(assetID, nonce, true)
		assert.Equal(t, int64(100), balance)
	})

	t.Run("preloaded userKDA bypasses GetUserKDA call", func(t *testing.T) {
		t.Parallel()

		account, _ := state.NewUserAccount([]byte("address"))
		assetID := []byte("MYTOKEN")
		nonce := []byte("1")

		preloadedKDA := &kapps.UserKDA{
			Balance:   200,
			LastClaim: &kapps.LastClaim{},
			Buckets:   make(map[string]*kapps.UserBucket),
		}

		err := account.SubFromBalanceWithNonce(50, assetID, nonce, true, preloadedKDA)
		assert.NoError(t, err)

		assert.Equal(t, int64(150), preloadedKDA.Balance)
	})
}

func TestUserAccount_SetUserKDA_NilAssetIDMapsToKLV(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))

	userKDA := &kapps.UserKDA{
		Balance:   500,
		LastClaim: &kapps.LastClaim{},
		Buckets:   make(map[string]*kapps.UserBucket),
	}

	err := account.SetUserKDA(nil, nil, userKDA)
	assert.NoError(t, err)

	retrieved, err := account.GetUserKDA(kdautils.KLVIdentifier, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, int64(500), retrieved.Balance)
}

func TestUserAccount_SubInternalKDA_RetrieveValueError(t *testing.T) {
	t.Parallel()

	account, _ := state.NewUserAccount([]byte("address"))

	expectedErr := errors.New("retrieval error")
	account.SetDataTrie(&mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			return nil, expectedErr
		},
	})

	assetID := []byte("TOKEN")
	internalID := []byte("internal-1")

	_, err := account.SubInternalKDA(assetID, internalID, forksBeforeV7)
	assert.Equal(t, expectedErr, err)
}

// The fungible balance helpers refuse an asset ID holding the key separator: it names an NFT unit,
// whose holder record they would otherwise read and write (KLC-2876).
func TestUserAccount_BalanceRefusesACompoundAssetID(t *testing.T) {
	collectionID, nonce := []byte("NFT-1234"), []byte("1")
	compoundID := []byte(string(collectionID) + kapps.Sp + string(nonce))

	helpers := map[string]func(state.UserAccountHandler) error{
		"AddToBalance":   func(acc state.UserAccountHandler) error { return acc.AddToBalance(1, compoundID, true) },
		"SubFromBalance": func(acc state.UserAccountHandler) error { return acc.SubFromBalance(1, compoundID, true) },
	}
	for name, apply := range helpers {
		t.Run(name, func(t *testing.T) {
			account, _ := state.NewUserAccount([]byte("address"))
			assert.NoError(t, account.AddInternalKDA(collectionID, nonce, mock.NFTHolderRecord(1)))

			assert.ErrorIs(t, apply(account), state.ErrCompoundAssetID)

			record, err := account.SubInternalKDA(collectionID, nonce, forksBeforeV7)
			assert.NoError(t, err)
			assert.Equal(t, mock.NFTHolderRecord(1), record, "the NFT record is untouched")
		})
	}
}
