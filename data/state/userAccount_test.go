package state_test

import (
	"encoding/hex"
	"math"
	"testing"

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
