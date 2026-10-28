package validators

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-2818: an elected validator that rotates its BLS key mid-epoch must be paid
// the whole epoch once. The pre-rotation snapshot is copied onto the new key, and
// credits that still arrive on the old elected key must not be added on top of it.

func TestRotationRewards_ElectedValidatorIsPaidOnce(t *testing.T) {
	t.Parallel()

	const beforeRotation = int64(1000)
	const afterRotation = int64(5000)

	t.Run("V7 on: post-rotation fees reach the current key and epoch end pays the sum once", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-v7")
		v := newRotationKApp(t, true)
		oldKey := electForRotation(t, v, owner)

		creditConsensus(t, v, beforeRotation, oldKey.pub)
		newKey := rotateTo(t, v, owner)

		oldPeer := requirePeer(t, v, oldKey.pub)
		newPeer := requirePeer(t, v, newKey.pub)
		assert.True(t, oldPeer.GetRevoked())
		assert.Equal(t, int64(0), oldPeer.GetAccumulatedFees())
		assert.Equal(t, uint32(0), oldPeer.GetLeaderSuccessRateSuccess())
		assert.Equal(t, beforeRotation, newPeer.GetAccumulatedFees())
		assert.Equal(t, uint32(1), newPeer.GetLeaderSuccessRateSuccess())
		assert.Equal(t, state.List_jailed, newPeer.GetList())
		assert.Zero(t, newPeer.GetRating())
		assert.Zero(t, newPeer.GetTempRating())

		ratingBefore := oldPeer.GetTempRating()
		creditConsensus(t, v, afterRotation, oldKey.pub)

		oldPeer = requirePeer(t, v, oldKey.pub)
		newPeer = requirePeer(t, v, newKey.pub)
		assert.Equal(t, int64(0), oldPeer.GetAccumulatedFees())
		assert.Equal(t, uint32(0), oldPeer.GetLeaderSuccessRateSuccess())
		assert.Greater(t, oldPeer.GetTempRating(), ratingBefore)
		assert.Equal(t, beforeRotation+afterRotation, newPeer.GetAccumulatedFees())
		assert.Equal(t, uint32(2), newPeer.GetLeaderSuccessRateSuccess())
		assert.Zero(t, newPeer.GetTempRating())

		assert.Equal(t, beforeRotation+afterRotation, pendingAfterEpoch(t, v, owner, newKey.pub))
		assert.Equal(t, int64(0), requirePeer(t, v, oldKey.pub).GetAccumulatedFees())
	})

	t.Run("V7 off: post-rotation fees stay on the revoked key and epoch end pays the snapshot", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-pre")
		v := newRotationKApp(t, false)
		oldKey := electForRotation(t, v, owner)

		creditConsensus(t, v, beforeRotation, oldKey.pub)
		newKey := rotateTo(t, v, owner)
		creditConsensus(t, v, afterRotation, oldKey.pub)

		oldPeer := requirePeer(t, v, oldKey.pub)
		newPeer := requirePeer(t, v, newKey.pub)
		assert.Equal(t, beforeRotation+afterRotation, oldPeer.GetAccumulatedFees())
		assert.Equal(t, beforeRotation, newPeer.GetAccumulatedFees())
		assert.Equal(t, beforeRotation, pendingAfterEpoch(t, v, owner, newKey.pub))
	})

	t.Run("V7 on: a second rotation in the same epoch is still paid once", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-twice")
		v := newRotationKApp(t, true)
		first := electForRotation(t, v, owner)

		creditConsensus(t, v, beforeRotation, first.pub)
		second := rotateTo(t, v, owner)
		creditConsensus(t, v, afterRotation, first.pub)
		third := rotateTo(t, v, owner)
		creditConsensus(t, v, int64(2000), first.pub)

		assert.Equal(t, int64(0), requirePeer(t, v, first.pub).GetAccumulatedFees())
		assert.Equal(t, int64(0), requirePeer(t, v, second.pub).GetAccumulatedFees())
		assert.Equal(t, beforeRotation+afterRotation+2000, requirePeer(t, v, third.pub).GetAccumulatedFees())
		assert.Equal(t, beforeRotation+afterRotation+2000, pendingAfterEpoch(t, v, owner, third.pub))
	})

	t.Run("V7 on: a block shared with another signer does not pay the rotated validator twice", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-share")
		other := makeAddress("other-share")
		v := newRotationKApp(t, true)
		setLeaderRewardPercent(v, 3000)
		oldKey := electForRotation(t, v, owner)
		otherKey := electForRotation(t, v, other)

		newKey := rotateTo(t, v, owner)
		const blockFees = int64(1000)
		creditConsensus(t, v, blockFees, oldKey.pub, otherKey.pub)

		revokedFees := requirePeer(t, v, oldKey.pub).GetAccumulatedFees()
		currentFees := requirePeer(t, v, newKey.pub).GetAccumulatedFees()
		otherFees := requirePeer(t, v, otherKey.pub).GetAccumulatedFees()
		assert.Equal(t, int64(0), revokedFees)
		assert.Greater(t, currentFees, int64(0))
		assert.Greater(t, otherFees, int64(0))
		assert.Equal(t, blockFees, revokedFees+currentFees+otherFees)
	})
}

func TestRotationRewards_ValidatorThatIsNotElected(t *testing.T) {
	t.Parallel()

	const fees = int64(1000)

	for _, v7 := range []bool{true, false} {
		t.Run("paid snapshot is the same with the gate "+boolName(v7), func(t *testing.T) {
			t.Parallel()

			owner := makeAddress("owner-waiting-" + boolName(v7))
			v := newRotationKApp(t, v7)
			key := newTestBLSKey(t)
			resultCode, err := v.Register(owner, newRegisterContractWithProof(owner, key.pub, key.proof(t, owner)))
			require.Equal(t, transaction.Transaction_Ok, resultCode)
			require.NoError(t, err)

			peer := requirePeer(t, v, key.pub)
			require.NotEqual(t, state.List_elected, peer.GetList())
			peer.AddToAccumulatedFees(fees)

			next := rotateTo(t, v, owner)

			newPeer := requirePeer(t, v, next.pub)
			assert.NotEqual(t, state.List_jailed, newPeer.GetList())
			assert.Equal(t, fees, newPeer.GetAccumulatedFees())
			assert.Equal(t, fees, pendingAfterEpoch(t, v, owner, next.pub))
		})
	}
}

func TestDecreaseAll_RevokedKeyCountersFollowTheCurrentKey(t *testing.T) {
	t.Parallel()

	t.Run("V7 on: miss counters land on the current key and rating stays on the elected key", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-miss")
		v := newRotationKApp(t, true)
		oldKey := electForRotation(t, v, owner)
		newKey := rotateTo(t, v, owner)
		ratingBefore := requirePeer(t, v, oldKey.pub).GetTempRating()

		require.NoError(t, v.DecreaseAll([][]byte{oldKey.pub}, 10, 21))

		oldPeer := requirePeer(t, v, oldKey.pub)
		newPeer := requirePeer(t, v, newKey.pub)
		assert.Equal(t, uint32(0), oldPeer.GetLeaderSuccessRateFailure())
		assert.Equal(t, uint32(0), oldPeer.GetValidatorSuccessRateFailure())
		assert.Greater(t, newPeer.GetLeaderSuccessRateFailure(), uint32(0))
		assert.Greater(t, newPeer.GetValidatorSuccessRateFailure(), uint32(0))
		assert.Less(t, oldPeer.GetTempRating(), ratingBefore)
		assert.Zero(t, newPeer.GetTempRating())
	})

	t.Run("V7 on: a proposer miss on the current key is cleared by a block the old key leads", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-streak")
		v := newRotationKApp(t, true)
		oldKey := electForRotation(t, v, owner)
		newKey := rotateTo(t, v, owner)

		require.NoError(t, v.DecreaseTempRating(owner, true))
		assert.Equal(t, uint32(1), requirePeer(t, v, newKey.pub).GetConsecutiveProposerMisses())
		assert.Equal(t, uint32(0), requirePeer(t, v, oldKey.pub).GetConsecutiveProposerMisses())

		creditConsensus(t, v, 1000, oldKey.pub)
		assert.Equal(t, uint32(0), requirePeer(t, v, newKey.pub).GetConsecutiveProposerMisses())
		assert.Equal(t, uint32(0), requirePeer(t, v, oldKey.pub).GetConsecutiveProposerMisses())
	})

	t.Run("V7 on: the elected key rating is saved while the peer cache is off", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-uncached")
		v := newRotationKApp(t, true)
		v.accountsCacher.ResetAll(false)
		oldKey := electForRotation(t, v, owner)
		ratingBefore := requirePeer(t, v, oldKey.pub).GetTempRating()
		rotateTo(t, v, owner)

		creditConsensus(t, v, 1000, oldKey.pub)
		assert.Greater(t, requirePeer(t, v, oldKey.pub).GetTempRating(), ratingBefore)
	})

	t.Run("V7 off: miss counters stay on the elected key", func(t *testing.T) {
		t.Parallel()

		owner := makeAddress("owner-miss-pre")
		v := newRotationKApp(t, false)
		oldKey := electForRotation(t, v, owner)
		newKey := rotateTo(t, v, owner)

		require.NoError(t, v.DecreaseAll([][]byte{oldKey.pub}, 10, 21))

		oldPeer := requirePeer(t, v, oldKey.pub)
		newPeer := requirePeer(t, v, newKey.pub)
		assert.Greater(t, oldPeer.GetLeaderSuccessRateFailure(), uint32(0))
		assert.Equal(t, uint32(0), newPeer.GetLeaderSuccessRateFailure())
		assert.Equal(t, uint32(0), newPeer.GetValidatorSuccessRateFailure())
	})
}

// newRotationKApp leaves the V6 proof checks on. That fork is already live, so the
// payout change is switched by FixAuditChangesV7 alone.
func newRotationKApp(t *testing.T, v7 bool) *validatorsKApp {
	t.Helper()

	v := newProofTestKApp(t, true)
	v.forkController.(*mock.ForkControllerStub).FixAuditChangesV7Value = v7

	return v
}

func electForRotation(t *testing.T, v *validatorsKApp, owner []byte) testBLSKey {
	t.Helper()

	key := newTestBLSKey(t)
	resultCode, err := v.Register(owner, newRegisterContractWithProof(owner, key.pub, key.proof(t, owner)))
	require.Equal(t, transaction.Transaction_Ok, resultCode)
	require.NoError(t, err)

	peer := requirePeer(t, v, key.pub)
	peer.SetList(state.List_elected)
	peer.SetRating(startRating)
	peer.SetTempRating(startRating)
	require.NoError(t, v.accountsCacher.UpdatePeer(peer))

	return key
}

func rotateTo(t *testing.T, v *validatorsKApp, owner []byte) testBLSKey {
	t.Helper()

	next := newTestBLSKey(t)
	resultCode, err := v.UpdateValidator(owner, newRotateContract(next.pub, next.proof(t, owner)))
	require.Equal(t, transaction.Transaction_Ok, resultCode)
	require.NoError(t, err)

	return next
}

func creditConsensus(t *testing.T, v *validatorsKApp, fees int64, pubKeys ...[]byte) {
	t.Helper()

	group := make([]sharding.Validator, len(pubKeys))
	var bitmap byte
	for i, pub := range pubKeys {
		key := append([]byte(nil), pub...)
		group[i] = &mock.ValidatorMock{PubKeyCalled: func() []byte { return key }}
		bitmap |= 1 << uint(i)
	}

	require.NoError(t, v.UpdateValidatorInfoOnSuccessfulBlock(group, []byte{bitmap}, fees))
}

func pendingAfterEpoch(t *testing.T, v *validatorsKApp, owner, currentKey []byte) int64 {
	t.Helper()

	err := v.ProcessEconomicsEndOfEpoch(10, []*state.ValidatorInfo{{
		OwnerAddress: owner,
		PublicKey:    currentKey,
	}})
	require.NoError(t, err)

	app, err := v.getKApp()
	require.NoError(t, err)
	pending, err := v.getPendingRewards(app, owner)
	require.NoError(t, err)
	val, err := v.getValidator(app, owner)
	require.NoError(t, err)
	require.Equal(t, pending, val.TotalRewards)

	return pending
}

func requirePeer(t *testing.T, v *validatorsKApp, pub []byte) state.PeerAccountHandler {
	t.Helper()

	peer, err := v.loadPeerAccount(pub)
	require.NoError(t, err)

	return peer
}

func setLeaderRewardPercent(v *validatorsKApp, percent uint64) {
	v.KAppController.(*stub.KAppControllerStub).GetProposalControllerCalled = func() kapps.ActiveProposalController {
		return &mock.ProposalControllerStub{
			GetParameterUintCalled: func(param kapps.EnumParameter) uint64 {
				if param == kapps.EnumParameter_LeaderValidatorRewardsPercentage {
					return percent
				}
				return 0
			},
		}
	}
}

func boolName(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
