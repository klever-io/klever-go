package transaction_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"

	"github.com/stretchr/testify/assert"
)

var accountForValidator, _ = addressConverter.Decode("klv1g5ys9yu6knlhs7khks8q4wpaxx78a59hrtnmqtkcjsq37nsw0d4s5lf6hm")

var peerAddress = validBLSKey("peerAddress")

func TestCreateValidatorTxProcessor_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	// create validator with invalid owner address
	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: []byte("invalid"),
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	// create validator with invalid reward address
	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       []byte("invalid"),
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	// create validator with invalid comission
	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Commission:          100000000,
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// create validator with invalid max delegation amount
	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: -100_000_000_000_000_000,
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// create validator with invalid logo
	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Logo:                "https://invalid.com/thisisareallylongurlthatis251charactersanditwillneverexistontheinternetbecauseitistoolongandinvalidthisisareallylongurlthatis251charactersanditwillneverexistontheinternetbecauseitistoolongandinvalidthisisareallylongurlthatis251charactersanditwillneverexistontheinternetbecauseitistoolongandinvalid",
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// create validator with invalid uri
	URIs := make(map[string]string)
	for i := 1; i <= 11; i++ {
		key := fmt.Sprintf("uri%d", i)
		value := fmt.Sprintf("http://example.com/%d", i)
		URIs[key] = value
	}

	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			URIs:                URIs,
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// create validator with invalid name
	createValidatorContract = transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
			Name:                "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijk",
		},
	}

	createValidatorTx, _ = createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err = c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

}

func TestCreateValidatorTxProcessor_CreateTwoValidatorsWithTheSameOwner_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	createValidatorContract2 := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx2, _ := createTransactionMock(&createValidatorContract2, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash2, err := c.execTx.PreProcessTransaction(createValidatorTx2)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash2, createValidatorTx2)
	assert.Equal(t, common.ErrAccountValidatorSet, err)
}

func TestCreateValidatorTxProcessor_CreateTwoValidatorsWithTheSameBLSPubkey_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(accountForValidator, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	// first validator freeze tx
	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	// second validator freeze tx
	freezeContract2 := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx2, _ := createTransactionMock(&freezeContract2, transaction.TXContract_FreezeContractType, accountForValidator, 0)
	_, freezeHashTx2, err := c.execTx.PreProcessTransaction(freezeTx2)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx2, freezeTx2)
	assert.Equal(t, nil, err)

	// create first validator tx
	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	// create second validator tx
	createValidatorContract2 := transaction.CreateValidatorContract{
		OwnerAddress: accountForValidator,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx2, _ := createTransactionMock(&createValidatorContract2, transaction.TXContract_CreateValidatorContractType, accountForValidator, 0)
	_, createValidatorTxHash2, err := c.execTx.PreProcessTransaction(createValidatorTx2)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash2, createValidatorTx2)
	assert.Equal(t, common.ErrAccountValidatorSet, err)
}

func TestCreateValidatorTxProcessor_ShouldWork(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)
}

func TestConfigValidatorTxProcessor_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	// config validator with invalid reward address
	configValidatorContract := transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			RewardAddress: []byte("invalid reward address"),
		},
	}

	configValidatorTx, _ := createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, testOwnerAddress, 0)
	_, configValidatorTxHash, err := c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, process.ErrInvalidRcvAddr, err)

	// config validator with invalid comission
	configValidatorContract = transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			Commission: 1000000,
		},
	}

	configValidatorTx, _ = createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, testOwnerAddress, 0)
	_, configValidatorTxHash, err = c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)

	// config validator with invalid delegation amount
	configValidatorContract = transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			MaxDelegationAmount: -10000,
		},
	}

	configValidatorTx, _ = createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, testOwnerAddress, 0)
	_, configValidatorTxHash, err = c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, common.ErrInvalidValue, err)
}

func TestConfigValidatorTxProcessor_ChangeBLSPubkeyWithoutBeenValidatorOwner_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(accountForValidator, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	newBLS := validBLSKey("newBLS")

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	configValidatorContract := transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			BLSPublicKey: newBLS,
		},
	}

	configValidatorTx, _ := createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, accountForValidator, 0)
	_, configValidatorTxHash, err := c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, common.ErrValidatorNotFound, err)
}

func TestConfigValidatorTxProcessor_ChangeBLSPubkeyForOtherThatIsAlreadyInUse_ShouldError(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	c.AddUser(accountForValidator, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	peerAddress2 := validBLSKey("peerAddress2")

	// first validator freeze tx
	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	// second validator freeze tx
	freezeContract2 := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx2, _ := createTransactionMock(&freezeContract2, transaction.TXContract_FreezeContractType, accountForValidator, 0)
	_, freezeHashTx2, err := c.execTx.PreProcessTransaction(freezeTx2)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx2, freezeTx2)
	assert.Equal(t, nil, err)

	// create first validator tx
	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	// create second validator tx
	createValidatorContract2 := transaction.CreateValidatorContract{
		OwnerAddress: accountForValidator,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress2,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx2, _ := createTransactionMock(&createValidatorContract2, transaction.TXContract_CreateValidatorContractType, accountForValidator, 0)
	_, createValidatorTxHash2, err := c.execTx.PreProcessTransaction(createValidatorTx2)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash2, createValidatorTx2)
	assert.Equal(t, nil, err)

	// change second validator bls pubkey for the same of the first
	configValidatorContract := transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			BLSPublicKey: peerAddress2,
		},
	}

	configValidatorTx, _ := createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, testOwnerAddress, 0)
	_, configValidatorTxHash, err := c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, common.ErrAccountValidatorNotOwner, err)
}

func TestConfigValidatorTxProcessor_ShouldWork(t *testing.T) {
	c := NewController(t)
	c.AddUser(testOwnerAddress, 10_000_000_000, kdautils.KLVIdentifier)
	blk := createBlockHeader()

	newBLS := validBLSKey("newBLS")

	freezeContract := transaction.FreezeContract{
		AssetID: kdautils.KLVIdentifier,
		Amount:  MinFreezeAmount,
	}

	freezeTx, _ := createTransactionMock(&freezeContract, transaction.TXContract_FreezeContractType, testOwnerAddress, 0)
	_, freezeHashTx, err := c.execTx.PreProcessTransaction(freezeTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, freezeHashTx, freezeTx)
	assert.Equal(t, nil, err)

	createValidatorContract := transaction.CreateValidatorContract{
		OwnerAddress: testOwnerAddress,
		Config: &transaction.ValidatorConfig{
			BLSPublicKey:        peerAddress,
			RewardAddress:       testReferralAddress,
			CanDelegate:         true,
			MaxDelegationAmount: 100_000_000_000,
		},
	}

	createValidatorTx, _ := createTransactionMock(&createValidatorContract, transaction.TXContract_CreateValidatorContractType, testOwnerAddress, 0)
	_, createValidatorTxHash, err := c.execTx.PreProcessTransaction(createValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, createValidatorTxHash, createValidatorTx)
	assert.Equal(t, nil, err)

	configValidatorContract := transaction.ValidatorConfigContract{
		Config: &transaction.ValidatorConfig{
			MaxDelegationAmount: 10,
			BLSPublicKey:        newBLS,
			RewardAddress:       accountForValidator,
			CanDelegate:         false,
			Commission:          1,
			URIs:                map[string]string{"test": "test"},
			Logo:                "test",
			Name:                "test",
		},
	}

	configValidatorTx, _ := createTransactionMock(&configValidatorContract, transaction.TXContract_ValidatorConfigContractType, testOwnerAddress, 0)
	_, configValidatorTxHash, err := c.execTx.PreProcessTransaction(configValidatorTx)
	assert.Nil(c.t, err)

	err = c.execTx.ProcessTransaction(blk, configValidatorTxHash, configValidatorTx)
	assert.Equal(t, nil, err)
}
