package transaction_test

import (
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var userA, _ = addressConverter.Decode("klv1g5khe6ec5yhwqd773gghc55q0kvpanevgqzj8h5r5sj2eeke2heqnt5dfp")
var userB, _ = addressConverter.Decode("klv1u75daf827dd864ug63mh5xvntd06h9wjk7rau3sd2c7vktqegfjqe7kvnz")
var userC, _ = addressConverter.Decode("klv1fdrecm058y8kr3yxvnccf7daffpavsn7df7dmg27yn56d0a6qjqsm2h8q9")
var validatorA, _ = addressConverter.Decode("klv10gq6xsegedacd084vmpr2xus950j3d6lhqjfe8ue2xkmfwtkzavqnqhz99")
var validatorB, _ = addressConverter.Decode("klv15zssmvht00ugvge5le9n885kahc5ykxzvmxx6xwz5ya2an562yyssfa0c5")

var peerAddressA = validBLSKey("peerAddressA")
var peerAddressB = validBLSKey("peerAddressB")

func CreateValidators(c *Controller, blk *block.Block) {
	// Create Validator A
	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorA, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: validatorA,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressA,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorA, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)

	// Create Validator B

	freezeContract = transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ = createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorB, 0)
	_, freezeHashTx, err = c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: validatorB,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressB,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorB, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)
}

func TestRewards_ValidatorRewards_BeforeFixDelegationSameEpochFork(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

	// V1: Disable both FixDelegationSameEpoch and EpochRewardsV2
	c.UpdateForkController(config.EnableEpochs{
		FixDelegationSameEpoch: 100_000,
		EpochRewardsV2:         100_000,
	})

	// Create Users
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userB, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	// Create Validators

	blk := c.CreateBlockHeader(0, 1, 1)

	CreateValidators(c, blk)

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}, {OwnerAddress: validatorB, PublicKey: peerAddressB}}

	// Users need to Freeze KLV

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  freezeBalance,
	}

	blk = c.CreateBlockHeader(0, 1, 2)

	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	bucketUserB := c.RunFreezeTX(blk, userB, freezeContract.AssetID, freezeContract.Amount)

	// User A and B need to Delegate his current bucket to validator A

	blk = c.CreateBlockHeader(0, 1, 3)

	c.CheckBalance(userA, kdautils.KLVIdentifier, initialBalance-freezeBalance)
	c.CheckFrozenBalance(userA, kdautils.KLVIdentifier, freezeBalance)

	c.CheckBalance(userB, kdautils.KLVIdentifier, initialBalance-freezeBalance)
	c.CheckFrozenBalance(userB, kdautils.KLVIdentifier, freezeBalance)

	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorA)

	// Change Delegation of User B before change epoch

	blk = c.CreateBlockHeader(0, 1, 4)

	c.RunDelegateTX(blk, userB, bucketUserB, validatorB)

	rewardsA := int64(100)
	rewardsB := int64(50)

	err := c.AddFeesToPeer(peerAddressA, rewardsA)
	require.Nil(t, err)
	err = c.AddFeesToPeer(peerAddressB, rewardsB)
	require.Nil(t, err)

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Nil(t, err)

	assert.Equal(t, rewardsA/2, c.GetAllowance(userA))
	assert.Equal(t, (rewardsB + (rewardsA / 2)), c.GetAllowance(userB))
}

func TestRewards_ValidatorRewards_AfterFixDelegationSameEpochFork(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

	// V1: Disable EpochRewardsV2 by setting it to a high epoch
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 100_000,
	})

	// Create Users
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userB, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	// Create Validators

	blk := c.CreateBlockHeader(0, 1, 1)

	CreateValidators(c, blk)

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}, {OwnerAddress: validatorB, PublicKey: peerAddressB}}

	// Users need to Freeze KLV

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  freezeBalance,
	}

	blk = c.CreateBlockHeader(0, 1, 2)

	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	bucketUserB := c.RunFreezeTX(blk, userB, freezeContract.AssetID, freezeContract.Amount)

	// User A and B need to Delegate his current bucket to validator A

	blk = c.CreateBlockHeader(0, 1, 3)

	c.CheckBalance(userA, kdautils.KLVIdentifier, initialBalance-freezeBalance)
	c.CheckFrozenBalance(userA, kdautils.KLVIdentifier, freezeBalance)

	c.CheckBalance(userB, kdautils.KLVIdentifier, initialBalance-freezeBalance)
	c.CheckFrozenBalance(userB, kdautils.KLVIdentifier, freezeBalance)

	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorA)

	// Change Delegation of User B before change epoch

	blk = c.CreateBlockHeader(0, 1, 4)

	c.RunDelegateTX(blk, userB, bucketUserB, validatorB)

	rewardsA := int64(100)
	rewardsB := int64(50)

	err := c.AddFeesToPeer(peerAddressA, rewardsA)
	require.Nil(t, err)
	err = c.AddFeesToPeer(peerAddressB, rewardsB)
	require.Nil(t, err)

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Nil(t, err)

	// V1: rewards go to user allowance
	assert.Equal(t, rewardsA, c.GetAllowance(userA))
	assert.Equal(t, rewardsB, c.GetAllowance(userB))
}

func TestRewards_ValidatorRewards_V2_EpochRewards(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

	// Enable V2 epoch rewards
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 0, // Enable from epoch 0
	})

	// Create Users
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userB, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	// Create Validators
	blk := c.CreateBlockHeader(0, 1, 1)
	CreateValidators(c, blk)

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}, {OwnerAddress: validatorB, PublicKey: peerAddressB}}

	// Users need to Freeze KLV
	blk = c.CreateBlockHeader(0, 1, 2)
	bucketUserA := c.RunFreezeTX(blk, userA, kdautils.KLVIdentifier, freezeBalance)
	bucketUserB := c.RunFreezeTX(blk, userB, kdautils.KLVIdentifier, freezeBalance)

	// User A and B delegate to validator A
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorA)

	// Change Delegation of User B before epoch change
	blk = c.CreateBlockHeader(0, 1, 4)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorB)

	rewardsA := int64(100)
	rewardsB := int64(50)

	err := c.AddFeesToPeer(peerAddressA, rewardsA)
	require.Nil(t, err)
	err = c.AddFeesToPeer(peerAddressB, rewardsB)
	require.Nil(t, err)

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Nil(t, err)

	// V2: rewards go to pending rewards (NOT to user allowance)
	assert.Equal(t, int64(0), c.GetAllowance(userA), "V2 should NOT add rewards to user allowance")
	assert.Equal(t, int64(0), c.GetAllowance(userB), "V2 should NOT add rewards to user allowance")

	// V2: rewards are stored in pending rewards trie
	assert.Equal(t, rewardsA, c.GetPendingRewards(userA))
	assert.Equal(t, rewardsB, c.GetPendingRewards(userB))
}

func TestRewards_ValidatorRewards_RemainFees(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

	// V1: Disable EpochRewardsV2 by setting it to a high epoch
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 100_000,
	})

	// Create Users
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userB, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userC, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	// Create Validators
	blk := c.CreateBlockHeader(0, 1, 1)
	CreateValidators(c, blk)

	// Users need to Freeze KLV
	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  freezeBalance,
	}

	blk = c.CreateBlockHeader(0, 1, 2)
	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	bucketUserB := c.RunFreezeTX(blk, userB, freezeContract.AssetID, freezeContract.Amount)
	bucketUserC := c.RunFreezeTX(blk, userC, freezeContract.AssetID, freezeContract.Amount)

	// User A, B and C delegate their current bucket to validator A
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorA)
	c.RunDelegateTX(blk, userC, bucketUserC, validatorA)

	// Simulate rewards distribution
	rewardsA := int64(100)

	err := c.AddFeesToPeer(peerAddressA, rewardsA)
	require.Nil(t, err)

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Nil(t, err)

	assert.Equal(t, int64(33), c.GetAllowance(userA))
	assert.Equal(t, int64(33), c.GetAllowance(userB))
	assert.Equal(t, int64(33), c.GetAllowance(userC))
	assert.Equal(t, int64(1), c.GetAllowance(testReferralAddress))
}

func Test_ValidatorRewards_Comission_20(t *testing.T) {
	initialBalance := int64(100_000_000_000)

	c := NewController(t)

	// V1: Disable EpochRewardsV2 by setting it to a high epoch
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 100_000,
	})

	// we want to create a user to delegate to validator A only because we don't want this validator get any remaining Fees
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	//Validator A Self Stake
	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorA, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: validatorA,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressA,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Commission:          20, // 20 Represents a comission of 0.2%
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorA, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)

	blk = c.CreateBlockHeader(0, 1, 2)

	//User A Freezes and delegate the bucket to Validator A
	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)

	rewardsA := int64(1_000_000) // 1 KLV of reward
	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsA))

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}
	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Equal(c.t, nil, err)

	expectedValidatorReward := int64(2000)

	assert.Equal(t, expectedValidatorReward, c.GetAllowance(testReferralAddress)) // 2000 is 0.2% of 1000000
	assert.Equal(t, rewardsA-expectedValidatorReward, c.GetAllowance(userA))      // userA receives 998000

	rewardsB := int64(100)

	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsB))

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(4, validatorInfo)
	require.Equal(c.t, nil, err)

	assert.Equal(t, expectedValidatorReward, c.GetAllowance(testReferralAddress))     // validatorA receives 0 because can't get fraction of klv
	assert.Equal(t, rewardsA+rewardsB-expectedValidatorReward, c.GetAllowance(userA)) // userA receives 100
}

func Test_ValidatorRewards_Comission_2000(t *testing.T) {
	initialBalance := int64(100_000_000_000)

	c := NewController(t)

	// V1: Disable EpochRewardsV2 by setting it to a high epoch
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 100_000,
	})

	// we want to create a user to delegate to validator A only because we don't want this validator get any remaining Fees
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	//Validator A Self Stake
	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorA, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: validatorA,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressA,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Commission:          2000, // 2000 Represents a comission of 20%
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorA, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)

	blk = c.CreateBlockHeader(0, 1, 2)

	//User A Freezes and delegate the bucket to Validator A
	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)

	rewardsA := int64(1_000_000) // 1 KLV of reward
	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsA))

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}
	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Equal(c.t, nil, err)

	expectedValidatorReward := int64(200000)

	assert.Equal(t, expectedValidatorReward, c.GetAllowance(testReferralAddress)) // 200000 is 20% of 1000000
	assert.Equal(t, rewardsA-expectedValidatorReward, c.GetAllowance(userA))      // userA receives 800000
}

func TestRewards_ValidatorRewards_V2_RemainFees(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

	// Enable V2 epoch rewards
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 0,
	})

	// Create Users
	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userB, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(userC, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	// Create Validators
	blk := c.CreateBlockHeader(0, 1, 1)
	CreateValidators(c, blk)

	// Users need to Freeze KLV
	blk = c.CreateBlockHeader(0, 1, 2)
	bucketUserA := c.RunFreezeTX(blk, userA, kdautils.KLVIdentifier, freezeBalance)
	bucketUserB := c.RunFreezeTX(blk, userB, kdautils.KLVIdentifier, freezeBalance)
	bucketUserC := c.RunFreezeTX(blk, userC, kdautils.KLVIdentifier, freezeBalance)

	// User A, B and C delegate their current bucket to validator A
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)
	c.RunDelegateTX(blk, userB, bucketUserB, validatorA)
	c.RunDelegateTX(blk, userC, bucketUserC, validatorA)

	// Simulate rewards distribution
	rewardsA := int64(100)

	err := c.AddFeesToPeer(peerAddressA, rewardsA)
	require.Nil(t, err)

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Nil(t, err)

	// V2: rewards go to pending rewards (NOT to user allowance)
	assert.Equal(t, int64(0), c.GetAllowance(userA), "V2 should NOT add rewards to user allowance")
	assert.Equal(t, int64(0), c.GetAllowance(userB), "V2 should NOT add rewards to user allowance")
	assert.Equal(t, int64(0), c.GetAllowance(userC), "V2 should NOT add rewards to user allowance")

	// V2: rewards are stored in pending rewards trie
	assert.Equal(t, int64(33), c.GetPendingRewards(userA))
	assert.Equal(t, int64(33), c.GetPendingRewards(userB))
	assert.Equal(t, int64(33), c.GetPendingRewards(userC))
	assert.Equal(t, int64(1), c.GetPendingRewards(testReferralAddress)) // remaining fees go to validator reward address
}

func Test_ValidatorRewards_V2_Commission_20(t *testing.T) {
	initialBalance := int64(100_000_000_000)

	c := NewController(t)

	// Enable V2 epoch rewards
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 0,
	})

	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	// Validator A Self Stake
	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorA, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: validatorA,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressA,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Commission:          20, // 20 Represents a commission of 0.2%
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorA, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)

	blk = c.CreateBlockHeader(0, 1, 2)

	// User A Freezes and delegate the bucket to Validator A
	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)

	rewardsA := int64(1_000_000) // 1 KLV of reward
	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsA))

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}
	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Equal(c.t, nil, err)

	expectedValidatorReward := int64(2000)

	// V2: rewards go to pending rewards (NOT to user allowance)
	assert.Equal(t, int64(0), c.GetAllowance(testReferralAddress), "V2 should NOT add rewards to user allowance")
	assert.Equal(t, int64(0), c.GetAllowance(userA), "V2 should NOT add rewards to user allowance")

	// V2: rewards are stored in pending rewards trie
	assert.Equal(t, expectedValidatorReward, c.GetPendingRewards(testReferralAddress)) // 2000 is 0.2% of 1000000
	assert.Equal(t, rewardsA-expectedValidatorReward, c.GetPendingRewards(userA))      // userA receives 998000

	// Second epoch of rewards
	rewardsB := int64(100)
	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsB))

	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(4, validatorInfo)
	require.Equal(c.t, nil, err)

	// V2: rewards accumulate in pending rewards
	assert.Equal(t, expectedValidatorReward, c.GetPendingRewards(testReferralAddress))     // validatorA receives 0 for second epoch because can't get fraction of klv
	assert.Equal(t, rewardsA+rewardsB-expectedValidatorReward, c.GetPendingRewards(userA)) // userA receives 100 more
}

func Test_ValidatorRewards_V2_Commission_2000(t *testing.T) {
	initialBalance := int64(100_000_000_000)

	c := NewController(t)

	// Enable V2 epoch rewards
	c.UpdateForkController(config.EnableEpochs{
		EpochRewardsV2: 0,
	})

	c.AddUser(userA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	// Validator A Self Stake
	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, validatorA, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	require.Equal(c.t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: validatorA,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddressA,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Commission:          2000, // 2000 Represents a commission of 20%
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, validatorA, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	require.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	require.Equal(c.t, nil, err)

	blk = c.CreateBlockHeader(0, 1, 2)

	// User A Freezes and delegate the bucket to Validator A
	bucketUserA := c.RunFreezeTX(blk, userA, freezeContract.AssetID, freezeContract.Amount)
	blk = c.CreateBlockHeader(0, 1, 3)
	c.RunDelegateTX(blk, userA, bucketUserA, validatorA)

	rewardsA := int64(1_000_000) // 1 KLV of reward
	require.NoError(t, c.AddFeesToPeer(peerAddressA, rewardsA))

	validatorInfo := []*state.ValidatorInfo{{OwnerAddress: validatorA, PublicKey: peerAddressA}}
	err = c.kappController.GetValidatorsKApp().ProcessEconomicsEndOfEpoch(3, validatorInfo)
	require.Equal(c.t, nil, err)

	expectedValidatorReward := int64(200000)

	// V2: rewards go to pending rewards (NOT to user allowance)
	assert.Equal(t, int64(0), c.GetAllowance(testReferralAddress), "V2 should NOT add rewards to user allowance")
	assert.Equal(t, int64(0), c.GetAllowance(userA), "V2 should NOT add rewards to user allowance")

	// V2: rewards are stored in pending rewards trie
	assert.Equal(t, expectedValidatorReward, c.GetPendingRewards(testReferralAddress)) // 200000 is 20% of 1000000
	assert.Equal(t, rewardsA-expectedValidatorReward, c.GetPendingRewards(userA))      // userA receives 800000
}
