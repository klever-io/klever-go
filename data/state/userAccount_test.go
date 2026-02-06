package state_test

import (
	"encoding/hex"
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

	err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, 1, 1000, staking, userKDA, true)
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

	t.Run("FPR skips unstaked buckets with FixStakingBuckets", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		forkController := mock.NewForkControllerStub()
		forkController.FixStakingBucketsValue = true

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
					UnstakedEpoch: 3, // already unstaked
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
			},
		}

		gains, err := account.ComputeAvailableClaim(
			kdautils.KLVIdentifier, 5, 2000, userKDA, staking, forkController,
		)
		assert.NoError(t, err)
		// Only the active bucket (1000/10000 * 1000 = 100)
		assert.Equal(t, int64(100), gains[string(kdautils.KLVIdentifier)])
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

		err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 500, 1, 1000, staking, userKDA, true)
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

		retrieved, err := account.SubInternalKDA([]byte("MYTOKEN"), []byte("1"))
		assert.NoError(t, err)
		assert.Equal(t, data, retrieved)
	})

	t.Run("sub nonexistent returns error", func(t *testing.T) {
		account, _ := state.NewUserAccount([]byte("address"))
		_, err := account.SubInternalKDA([]byte("MYTOKEN"), []byte("1"))
		assert.Error(t, err)
	})
}

func TestUserAccount_Freeze_InvalidValue(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{Buckets: make(map[string]*kapps.UserBucket)}
	staking := &kapps.StakingData{}

	err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 0, 1, 1000, staking, userKDA, true)
	assert.Equal(t, state.ErrInvalidValue, err)

	err = account.Freeze(kdautils.KLVIdentifier, []byte("b1"), -1, 1, 1000, staking, userKDA, true)
	assert.Equal(t, state.ErrInvalidValue, err)
}

func TestUserAccount_Freeze_NilBucketsCreatesMap(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	userKDA := &kapps.UserKDA{}
	staking := &kapps.StakingData{}
	assert.Nil(t, userKDA.Buckets)

	err := account.Freeze(kdautils.KLVIdentifier, []byte("b1"), 100, 1, 1000, staking, userKDA, true)
	assert.NoError(t, err)
	assert.NotNil(t, userKDA.Buckets)
	assert.Equal(t, int64(100), userKDA.FrozenBalance)
}

func TestUserAccount_Freeze_NonKLVAssetUseStringBucketID(t *testing.T) {
	account, _ := state.NewUserAccount([]byte("address"))
	assetID := []byte("MYTOKEN-1234")
	userKDA := &kapps.UserKDA{Buckets: make(map[string]*kapps.UserBucket)}
	staking := &kapps.StakingData{}

	err := account.Freeze(assetID, []byte("b1"), 100, 1, 1000, staking, userKDA, true)
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

		err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, 2, 2000, staking, userKDA, true)
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

		err := account.Freeze(kdautils.KLVIdentifier, bucketID, 100, 2, 2000, staking, userKDA, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(150), userKDA.Buckets[encodedBucketID].Value)
		// toAdd = value(100) only (no accumulation since newStakingFlow is false)
		assert.Equal(t, int64(100), userKDA.FrozenBalance)
		assert.Equal(t, int64(100), staking.TotalStaked)
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
