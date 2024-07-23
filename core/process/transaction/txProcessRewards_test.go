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

var peerAddressA = []byte("1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08")
var peerAddressB = []byte("51f3e4d40ec83d109c3d346b5adfb87bbaee1b3369166d0e3bca472b0f38caab0327a01eca784c474a5e2126aec2e604a3082320301afda05765b4f7eb9f69cd67c94d2d4acc713f814611f15b91888ffda86d135eaaf18f1efac5bbeb1dd08f")

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

	c.UpdateForkController(config.EnableEpochs{
		FixDelegationSameEpoch: 100_000,
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

	assert.Equal(t, rewardsA, c.GetAllowance(userA))
	assert.Equal(t, rewardsB, c.GetAllowance(userB))

}

func TestRewards_ValidatorRewards_RemainFees(t *testing.T) {
	c := NewController(t)

	initialBalance := int64(100_000_000_000)
	freezeBalance := int64(100_000_000)

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
