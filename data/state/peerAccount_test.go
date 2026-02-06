package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewEmptyPeerAccount(t *testing.T) {
	t.Parallel()

	acc := state.NewEmptyPeerAccount()

	assert.NotNil(t, acc)
	assert.Equal(t, int64(0), acc.AccumulatedFees)
}

func TestNewPeerAccount_NilAddressContainerShouldErr(t *testing.T) {
	t.Parallel()

	acc, err := state.NewPeerAccount(nil)
	assert.True(t, check.IfNil(acc))
	assert.Equal(t, common.ErrNilAddress, err)
}

func TestNewPeerAccount_OkParamsShouldWork(t *testing.T) {
	t.Parallel()

	acc, err := state.NewPeerAccount(make([]byte, 32))
	assert.Nil(t, err)
	assert.False(t, check.IfNil(acc))
}

func TestPeerAccount_SetInvalidBLSPublicKey(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	pubKey := []byte("")

	err := acc.SetBLSPublicKey(pubKey)
	assert.Equal(t, common.ErrNilBLSPublicKey, err)
}

func TestPeerAccount_SetAndGetBLSPublicKey(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	pubKey := []byte("BLSpubKey")

	err := acc.SetBLSPublicKey(pubKey)
	assert.Nil(t, err)
	assert.Equal(t, pubKey, acc.GetBLSPublicKey())
}

func TestPeerAccount_GetNonce(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	assert.Equal(t, uint64(0), acc.GetNonce())
}

func TestPeerAccount_SetRootHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rootHash []byte
	}{
		{"valid hash", []byte("roothash123")},
		{"empty hash", []byte{}},
		{"nil hash", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetRootHash(tt.rootHash)
			assert.Equal(t, tt.rootHash, acc.GetRootHash())
		})
	}
}

func TestPeerAccount_SetOwnerAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		address   []byte
		expectErr error
	}{
		{"valid address", make([]byte, 32), nil},
		{"empty address", []byte{}, common.ErrEmptyAddress},
		{"nil address", nil, common.ErrEmptyAddress},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			err := acc.SetOwnerAddress(tt.address)
			if tt.expectErr != nil {
				assert.Equal(t, tt.expectErr, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.address, acc.GetOwnerAddress())
			}
		})
	}
}

func TestPeerAccount_SetRevoked(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	assert.False(t, acc.GetRevoked())
	acc.SetRevoked()
	assert.True(t, acc.GetRevoked())
}

func TestPeerAccount_IncreaseNonce(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	acc.IncreaseNonce(10)
	assert.Equal(t, uint64(0), acc.GetNonce())
}

func TestPeerAccount_SuccessRateGetters(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))

	acc.IncreaseLeaderSuccessRate(5)
	assert.Equal(t, uint32(5), acc.GetLeaderSuccessRateSuccess())

	acc.DecreaseLeaderSuccessRate(3)
	assert.Equal(t, uint32(3), acc.GetLeaderSuccessRateFailure())

	acc.IncreaseValidatorSuccessRate(7)
	assert.Equal(t, uint32(7), acc.GetValidatorSuccessRateSuccess())

	acc.DecreaseValidatorSuccessRate(2)
	assert.Equal(t, uint32(2), acc.GetValidatorSuccessRateFailure())

	assert.Equal(t, uint32(0), acc.GetTotalLeaderSuccessRateSuccess())
	assert.Equal(t, uint32(0), acc.GetTotalLeaderSuccessRateFailure())
	assert.Equal(t, uint32(0), acc.GetTotalValidatorSuccessRateSuccess())
	assert.Equal(t, uint32(0), acc.GetTotalValidatorSuccessRateFailure())

	acc.ResetAtNewEpoch()

	assert.Equal(t, uint32(5), acc.GetTotalLeaderSuccessRateSuccess())
	assert.Equal(t, uint32(3), acc.GetTotalLeaderSuccessRateFailure())
	assert.Equal(t, uint32(7), acc.GetTotalValidatorSuccessRateSuccess())
	assert.Equal(t, uint32(2), acc.GetTotalValidatorSuccessRateFailure())
}

func TestPeerAccount_GetListString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		list     state.List
		expected string
	}{
		{"waiting list", state.List_waiting, "waiting"},
		{"eligible list", state.List_eligible, "eligible"},
		{"leaving list", state.List_leaving, "leaving"},
		{"inactive list", state.List_inactive, "inactive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetList(tt.list)
			assert.Equal(t, tt.expected, acc.GetListString())
		})
	}
}

func TestPeerAccount_ResetAtNewEpoch(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))
	acc.SetRating(100)
	acc.SetTempRating(200)
	acc.AddToAccumulatedFees(1000)
	acc.IncreaseLeaderSuccessRate(10)
	acc.DecreaseLeaderSuccessRate(5)
	acc.IncreaseValidatorSuccessRate(20)
	acc.DecreaseValidatorSuccessRate(3)
	acc.IncreaseValidatorIgnoredSignaturesRate(2)
	acc.IncreaseNumSelectedInSuccessBlocks()
	acc.IncreaseNumSelectedInSuccessBlocks()

	acc.ResetAtNewEpoch()

	assert.Equal(t, int64(0), acc.GetAccumulatedFees())
	assert.Equal(t, uint32(200), acc.GetRating())
	assert.Equal(t, uint32(10), acc.GetTotalLeaderSuccessRateSuccess())
	assert.Equal(t, uint32(5), acc.GetTotalLeaderSuccessRateFailure())
	assert.Equal(t, uint32(20), acc.GetTotalValidatorSuccessRateSuccess())
	assert.Equal(t, uint32(3), acc.GetTotalValidatorSuccessRateFailure())
	assert.Equal(t, uint32(2), acc.GetTotalValidatorIgnoredSignaturesRate())
	assert.Equal(t, uint32(0), acc.GetLeaderSuccessRateSuccess())
	assert.Equal(t, uint32(0), acc.GetLeaderSuccessRateFailure())
	assert.Equal(t, uint32(0), acc.GetValidatorSuccessRateSuccess())
	assert.Equal(t, uint32(0), acc.GetValidatorSuccessRateFailure())
	assert.Equal(t, uint32(0), acc.GetValidatorIgnoredSignaturesRate())
	assert.Equal(t, uint32(0), acc.GetNumSelectedInSuccessBlocks())
}

func TestPeerAccount_AddToAccumulatedFees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		initial  int64
		add      int64
		expected int64
	}{
		{"add positive", 100, 50, 150},
		{"add negative", 100, -30, 70},
		{"add zero", 100, 0, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.AddToAccumulatedFees(tt.initial)
			acc.AddToAccumulatedFees(tt.add)
			assert.Equal(t, tt.expected, acc.GetAccumulatedFees())
		})
	}
}

func TestPeerAccount_LeaderSuccessRate(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))

	acc.IncreaseLeaderSuccessRate(10)
	assert.Equal(t, uint32(10), acc.GetLeaderSuccessRateSuccess())

	acc.IncreaseLeaderSuccessRate(5)
	assert.Equal(t, uint32(15), acc.GetLeaderSuccessRateSuccess())

	acc.DecreaseLeaderSuccessRate(3)
	assert.Equal(t, uint32(3), acc.GetLeaderSuccessRateFailure())

	acc.DecreaseLeaderSuccessRate(7)
	assert.Equal(t, uint32(10), acc.GetLeaderSuccessRateFailure())
}

func TestPeerAccount_ValidatorSuccessRate(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))

	acc.IncreaseValidatorSuccessRate(20)
	assert.Equal(t, uint32(20), acc.GetValidatorSuccessRateSuccess())

	acc.IncreaseValidatorSuccessRate(15)
	assert.Equal(t, uint32(35), acc.GetValidatorSuccessRateSuccess())

	acc.DecreaseValidatorSuccessRate(5)
	assert.Equal(t, uint32(5), acc.GetValidatorSuccessRateFailure())

	acc.DecreaseValidatorSuccessRate(10)
	assert.Equal(t, uint32(15), acc.GetValidatorSuccessRateFailure())
}

func TestPeerAccount_IncreaseValidatorIgnoredSignaturesRate(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))

	acc.IncreaseValidatorIgnoredSignaturesRate(5)
	assert.Equal(t, uint32(5), acc.GetValidatorIgnoredSignaturesRate())

	acc.IncreaseValidatorIgnoredSignaturesRate(3)
	assert.Equal(t, uint32(8), acc.GetValidatorIgnoredSignaturesRate())
}

func TestPeerAccount_IncreaseNumSelectedInSuccessBlocks(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewPeerAccount(make([]byte, 32))

	acc.IncreaseNumSelectedInSuccessBlocks()
	assert.Equal(t, uint32(1), acc.GetNumSelectedInSuccessBlocks())

	acc.IncreaseNumSelectedInSuccessBlocks()
	acc.IncreaseNumSelectedInSuccessBlocks()
	assert.Equal(t, uint32(3), acc.GetNumSelectedInSuccessBlocks())
}

func TestPeerAccount_SetList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list state.List
	}{
		{"waiting list", state.List_waiting},
		{"eligible list", state.List_eligible},
		{"leaving list", state.List_leaving},
		{"inactive list", state.List_inactive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetList(tt.list)
			assert.Equal(t, tt.list, acc.GetList())
		})
	}
}

func TestPeerAccount_SetListAndIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		list  state.List
		index uint32
	}{
		{"waiting with index 0", state.List_waiting, 0},
		{"eligible with index 5", state.List_eligible, 5},
		{"leaving with index 10", state.List_leaving, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetListAndIndex(tt.list, tt.index)
			assert.Equal(t, tt.list, acc.GetList())
			assert.Equal(t, tt.index, acc.GetIndex())
		})
	}
}

func TestPeerAccount_SetRating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rating uint32
	}{
		{"rating 0", 0},
		{"rating 50", 50},
		{"rating 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetRating(tt.rating)
			assert.Equal(t, tt.rating, acc.GetRating())
		})
	}
}

func TestPeerAccount_SetTempRating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tempRating uint32
	}{
		{"temp rating 0", 0},
		{"temp rating 75", 75},
		{"temp rating 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetTempRating(tt.tempRating)
			assert.Equal(t, tt.tempRating, acc.GetTempRating())
		})
	}
}

func TestPeerAccount_SetConsecutiveProposerMisses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		misses uint32
	}{
		{"zero misses", 0},
		{"five misses", 5},
		{"ten misses", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, _ := state.NewPeerAccount(make([]byte, 32))
			acc.SetConsecutiveProposerMisses(tt.misses)
			assert.Equal(t, tt.misses, acc.GetConsecutiveProposerMisses())
		})
	}
}

func TestPeerAccount_CopyFrom(t *testing.T) {
	t.Parallel()

	t.Run("copy from valid peer account", func(t *testing.T) {
		oldAcc, _ := state.NewPeerAccount(make([]byte, 32))
		oldAcc.ShardId = 1
		oldAcc.SetList(state.List_eligible)
		oldAcc.SetListAndIndex(state.List_eligible, 5)
		oldAcc.AddToAccumulatedFees(1000)
		oldAcc.IncreaseLeaderSuccessRate(10)
		oldAcc.DecreaseLeaderSuccessRate(2)
		oldAcc.IncreaseValidatorSuccessRate(20)
		oldAcc.DecreaseValidatorSuccessRate(5)
		oldAcc.IncreaseValidatorIgnoredSignaturesRate(3)
		oldAcc.SetRating(100)
		oldAcc.SetTempRating(150)
		oldAcc.IncreaseNumSelectedInSuccessBlocks()
		oldAcc.IncreaseNumSelectedInSuccessBlocks()
		oldAcc.SetConsecutiveProposerMisses(2)

		oldAcc.ResetAtNewEpoch()
		oldAcc.IncreaseLeaderSuccessRate(5)
		oldAcc.DecreaseLeaderSuccessRate(1)

		newAcc, _ := state.NewPeerAccount(make([]byte, 32))
		err := newAcc.CopyFrom(oldAcc)

		assert.Nil(t, err)
		assert.Equal(t, oldAcc.GetShardId(), newAcc.GetShardId())
		assert.Equal(t, oldAcc.GetList(), newAcc.GetList())
		assert.Equal(t, oldAcc.GetIndex(), newAcc.GetIndex())
		assert.Equal(t, oldAcc.GetAccumulatedFees(), newAcc.GetAccumulatedFees())
		assert.Equal(t, oldAcc.GetLeaderSuccessRateSuccess(), newAcc.GetLeaderSuccessRateSuccess())
		assert.Equal(t, oldAcc.GetLeaderSuccessRateFailure(), newAcc.GetLeaderSuccessRateFailure())
		assert.Equal(t, oldAcc.GetValidatorSuccessRateSuccess(), newAcc.GetValidatorSuccessRateSuccess())
		assert.Equal(t, oldAcc.GetValidatorSuccessRateFailure(), newAcc.GetValidatorSuccessRateFailure())
		assert.Equal(t, oldAcc.GetValidatorIgnoredSignaturesRate(), newAcc.GetValidatorIgnoredSignaturesRate())
		assert.Equal(t, oldAcc.GetRating(), newAcc.GetRating())
		assert.Equal(t, oldAcc.GetTempRating(), newAcc.GetTempRating())
		assert.Equal(t, oldAcc.GetNumSelectedInSuccessBlocks(), newAcc.GetNumSelectedInSuccessBlocks())
		assert.Equal(t, oldAcc.GetConsecutiveProposerMisses(), newAcc.GetConsecutiveProposerMisses())
		assert.Equal(t, oldAcc.GetTotalLeaderSuccessRateSuccess(), newAcc.GetTotalLeaderSuccessRateSuccess())
		assert.Equal(t, oldAcc.GetTotalLeaderSuccessRateFailure(), newAcc.GetTotalLeaderSuccessRateFailure())
		assert.Equal(t, oldAcc.GetTotalValidatorSuccessRateSuccess(), newAcc.GetTotalValidatorSuccessRateSuccess())
		assert.Equal(t, oldAcc.GetTotalValidatorSuccessRateFailure(), newAcc.GetTotalValidatorSuccessRateFailure())
		assert.Equal(t, oldAcc.GetTotalValidatorIgnoredSignaturesRate(), newAcc.GetTotalValidatorIgnoredSignaturesRate())
	})

	t.Run("copy from wrong type", func(t *testing.T) {
		newAcc, _ := state.NewPeerAccount(make([]byte, 32))
		wrongHandler := &mock.PeerAccountHandlerMock{}

		err := newAcc.CopyFrom(wrongHandler)
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})
}

func TestValidatorInfo_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	t.Run("nil validator info", func(t *testing.T) {
		var vi *state.ValidatorInfo
		assert.True(t, vi.IsInterfaceNil())
	})

	t.Run("valid validator info", func(t *testing.T) {
		vi := &state.ValidatorInfo{
			PublicKey: []byte("validator-pubkey"),
			Index:     0,
			Rating:    100,
		}
		assert.False(t, vi.IsInterfaceNil())
	})

	t.Run("empty validator info", func(t *testing.T) {
		vi := &state.ValidatorInfo{}
		assert.False(t, vi.IsInterfaceNil())
	})
}
