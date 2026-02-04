package transaction_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/data/state"
	dt "github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
)

func mockProcessor(economicHandlerStub *mock.EconomicsHandlerStub) *transaction.TxProcessorExportTest {
	// environment setup
	scProcessor := &mock.SmartContractProcessorStub{}
	forkController := mock.NewForkControllerStub()
	accountsCacher := &mock.AccountsCacherStub{}
	maxGasLimitPerTxValue := uint64(100)
	economicsFees := &mock.EconomicsHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*dt.CostResponse, error) {
			cost := &dt.CostResponse{BandwidthFee: 100, KAppFee: 0, GasMultiplier: 1}
			if tx.GetBandwidthFee() < cost.BandwidthFee ||
				tx.GetKAppFee() < cost.KAppFee {
				return nil, fmt.Errorf("%w: (%d/%d) (%d/%d)", process.ErrInvalidTransactionFees,
					tx.GetBandwidthFee(), cost.BandwidthFee,
					tx.GetKAppFee(), cost.KAppFee,
				)
			}

			return cost, nil
		},
		MaxGasLimitPerTxValue: maxGasLimitPerTxValue,
	}
	if economicHandlerStub != nil {
		economicsFees = economicHandlerStub
	}

	txProc := transaction.NewTxProcessorExportTest()
	txProc.SetForkController(forkController)
	txProc.SetSCProcessor(scProcessor)
	txProc.SetAccountsCacher(accountsCacher)
	txProc.SetEconomicsFee(economicsFees)

	return txProc
}

var defaultSigner = signing.NewKeyGenerator(ed25519.NewEd25519())

func NewUserAccountHandler(address []byte, nonce uint64) state.UserAccountHandler {
	return NewUserAccountHandlerWithBalance(address, nonce, 10000)
}

func NewUserAccountHandlerWithBalance(address []byte, nonce uint64, balance int64) state.UserAccountHandler {
	data := state.NewUserAccountNoCheck(address)
	data.Nonce = nonce
	data.Balance = balance

	return data
}

func TestCheckTxValues(t *testing.T) {
	t.Parallel()

	// sign
	defaultKey, _ := defaultSigner.PrivateKeyFromByteArray(make([]byte, 32))
	defaultSender, _ := defaultKey.GeneratePublic().ToByteArray()

	signTX := func(tx *dt.Transaction, txHash []byte, singleSigner crypto.SingleSigner) {
		signature, _ := singleSigner.Sign(defaultKey, txHash)
		tx.Signature = append(tx.Signature, signature)
	}

	scenarios := []struct {
		name             string
		tx               *dt.Transaction_Raw
		txHash           []byte
		acntSender       state.UserAccountHandler
		preSetup         func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction)
		postCheck        func(t *testing.T, txProc *transaction.TxProcessorExportTest, tx *dt.Transaction)
		economicsHandler *mock.EconomicsHandlerStub
		ExpectedError    error
	}{
		{
			name:          "higher nonce",
			tx:            &dt.Transaction_Raw{Sender: defaultSender, Nonce: 3},
			acntSender:    NewUserAccountHandler(defaultSender, 2),
			txHash:        []byte{1},
			ExpectedError: process.ErrHigherNonceInTransaction,
		},
		{
			name:          "lower nonce",
			tx:            &dt.Transaction_Raw{Sender: defaultSender, Nonce: 1, BandwidthFee: 100},
			acntSender:    NewUserAccountHandler(defaultSender, 2),
			txHash:        []byte{1},
			ExpectedError: process.ErrLowerNonceInTransaction,
		},
		{
			name:          "invalid permission",
			tx:            &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 100, PermissionID: 10},
			acntSender:    NewUserAccountHandler(defaultSender, 2),
			txHash:        []byte{1},
			ExpectedError: state.ErrInvalidPermissionID,
		},
		{
			name:       "invalid signature",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 100},
			acntSender: NewUserAccountHandler(defaultSender, 2),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				tx.Signature = [][]byte{make([]byte, 64)}
			},
			ExpectedError: fmt.Errorf("signature verification failed: %w", common.ErrInvalidSignature),
		},
		{
			name: "signature threshold not met",
			tx:   &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 100},
			acntSender: func() state.UserAccountHandler {
				secondKey, _ := defaultSigner.PrivateKeyFromByteArray(bytes.Repeat([]byte{1}, 32))
				secondPub, _ := secondKey.GeneratePublic().ToByteArray()
				acnt := NewUserAccountHandler(defaultSender, 2)
				acnt.SetPermissions([]*state.Permission{
					{
						ID:        0,
						Type:      state.Permission_Owner,
						Threshold: 2,
						Signers: []*state.Key{
							{Address: defaultSender, Weight: 1},
							{Address: secondPub, Weight: 1},
						},
					},
				})
				return acnt
			}(),
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			ExpectedError: fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, 1, 2),
		},
		{
			name:       "invalid tx fee values",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 10},
			acntSender: NewUserAccountHandler(defaultSender, 2),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			ExpectedError: fmt.Errorf("%w: (%d/%d) (%d/%d)", process.ErrInvalidTransactionFees, 10, 100, 0, 0),
		},
		{
			name:       "no balance to pay fees",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 100},
			acntSender: NewUserAccountHandlerWithBalance(defaultSender, 2, 10),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			ExpectedError: fmt.Errorf("%w, has: %d, wanted: %d", process.ErrInsufficientFee, 10, 100),
		},
		{
			name:       "valid tx zero gas limit",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 100},
			acntSender: NewUserAccountHandler(defaultSender, 2),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			postCheck: func(t *testing.T, txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				assert.Equal(t, uint64(0), tx.GetGasLimit())
			},
			ExpectedError: nil,
		},
		{
			name:       "update gas limit higher than allowed",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 1000},
			acntSender: NewUserAccountHandlerWithBalance(defaultSender, 2, 1000),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			economicsHandler: &mock.EconomicsHandlerStub{
				ComputeGasCalled: func(tx *dt.Transaction, computedCost *dt.CostResponse) (uint64, uint64, error) {
					return 0, 0, fmt.Errorf("%w, gasLimit: %d, maxGasLimit: %d", process.ErrInvalidMaxGasLimitPerTx, 900, 100)
				},
			},
			ExpectedError: fmt.Errorf("%w, gasLimit: %d, maxGasLimit: %d", process.ErrInvalidMaxGasLimitPerTx, 900, 100),
		},
		{
			name:       "valid tx valid gas limit",
			tx:         &dt.Transaction_Raw{Sender: defaultSender, Nonce: 2, BandwidthFee: 150},
			acntSender: NewUserAccountHandler(defaultSender, 2),
			txHash:     []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner())
			},
			postCheck: func(t *testing.T, txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				assert.Equal(t, uint64(50), tx.GetGasLimit())
			},
			economicsHandler: &mock.EconomicsHandlerStub{
				ComputeGasCalled: func(tx *dt.Transaction, computedCost *dt.CostResponse) (uint64, uint64, error) {
					return uint64(50), 0, nil
				},
			},
			ExpectedError: nil,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			txProc := mockProcessor(scenario.economicsHandler)

			tx := &dt.Transaction{RawData: scenario.tx}
			// pre setup
			if scenario.preSetup != nil {
				scenario.preSetup(txProc, tx)
			}

			err := txProc.CheckTxValues(tx, scenario.acntSender, scenario.txHash)
			assert.Equal(t, scenario.ExpectedError, err)

			// post check
			if scenario.postCheck != nil {
				scenario.postCheck(t, txProc, tx)
			}
		})
	}

}

func TestValidatePermission(t *testing.T) {
	t.Parallel()

	defaultSigner := signing.NewKeyGenerator(ed25519.NewEd25519())
	defaultKey, _ := defaultSigner.PrivateKeyFromByteArray(make([]byte, 32))
	defaultSenderPubKey, _ := defaultKey.GeneratePublic().ToByteArray()

	secondSenderKey, _ := defaultSigner.PrivateKeyFromByteArray(bytes.Repeat([]byte{1}, 32))
	secondSenderPubKey, _ := secondSenderKey.GeneratePublic().ToByteArray()

	signTX := func(tx *dt.Transaction, txHash []byte, singleSigner crypto.SingleSigner, signers ...crypto.PrivateKey) {
		for _, key := range signers {
			signature, _ := singleSigner.Sign(key, txHash)
			tx.Signature = append(tx.Signature, signature)
		}
	}

	transferContract := &dt.TXContract{Type: dt.TXContract_TransferContractType}
	assetContract := &dt.TXContract{Type: dt.TXContract_AssetTriggerContractType}

	scenarios := []struct {
		name          string
		tx            *dt.Transaction
		permission    *state.Permission
		txHash        []byte
		preSetup      func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction)
		ExpectedError error
	}{
		{
			name: "valid owner permission",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 1,
				Signers:   []*state.Key{{Address: defaultSenderPubKey, Weight: 1}},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: nil,
		},
		{
			name: "invalid signers public key",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 1,
				Signers:   []*state.Key{{Address: make([]byte, 30), Weight: 1}},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: fmt.Errorf("failed to load signer public keys: %w", fmt.Errorf("invalid signer address: %w", crypto.ErrInvalidParam)),
		},
		{
			name: "invalid signature for owner permission",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 1,
				Signers:   []*state.Key{{Address: defaultSenderPubKey, Weight: 1}},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				tx.Signature = [][]byte{make([]byte, 64)} // Invalid signature
			},
			ExpectedError: fmt.Errorf("signature verification failed: %w", common.ErrInvalidSignature),
		},
		{
			name: "insufficient weight for threshold",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 2,
				Signers:   []*state.Key{{Address: defaultSenderPubKey, Weight: 1}},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: fmt.Errorf("%w: (%d/%d)", common.ErrSignatureThreshold, 1, 2),
		},
		{
			name: "valid custom permission",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey, Contract: []*dt.TXContract{transferContract}}},
			permission: &state.Permission{
				Type:       state.Permission_User,
				Threshold:  1,
				Signers:    []*state.Key{{Address: defaultSenderPubKey, Weight: 1}},
				Operations: dt.EncodeContractPermissions(dt.TXContract_TransferContractType),
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: nil,
		},
		{
			name: "invalid operation for user permission",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey, Contract: []*dt.TXContract{assetContract}}},
			permission: &state.Permission{
				Type:       state.Permission_User,
				Threshold:  1,
				Signers:    []*state.Key{{Address: defaultSenderPubKey, Weight: 1}},
				Operations: dt.EncodeContractPermissions(dt.TXContract_TransferContractType),
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: common.ErrNoPermission,
		},
		{
			name: "multiple signers valid",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 2,
				Signers: []*state.Key{
					{Address: defaultSenderPubKey, Weight: 1},
					{Address: secondSenderPubKey, Weight: 1}, // Another signer
				},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey, secondSenderKey)
			},
			ExpectedError: nil,
		},
		{
			name: "no valid signers",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 1,
				Signers:   []*state.Key{{Address: make([]byte, 32), Weight: 1}}, // Different signer
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey)
			},
			ExpectedError: fmt.Errorf("signature verification failed: %w", common.ErrInvalidSignature),
		},
		{
			name: "duplicated signers",
			tx:   &dt.Transaction{RawData: &dt.Transaction_Raw{Sender: defaultSenderPubKey}},
			permission: &state.Permission{
				Type:      state.Permission_Owner,
				Threshold: 2,
				Signers: []*state.Key{
					{Address: defaultSenderPubKey, Weight: 1},
					{Address: secondSenderPubKey, Weight: 1}, // Another signer
				},
			},
			txHash: []byte{1},
			preSetup: func(txProc *transaction.TxProcessorExportTest, tx *dt.Transaction) {
				signTX(tx, []byte{1}, txProc.SingleSigner(), defaultKey, defaultKey)
			},
			ExpectedError: fmt.Errorf("signature verification failed: %w", common.ErrInvalidSignature),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			txProc := mockProcessor(nil)

			// pre setup
			if scenario.preSetup != nil {
				scenario.preSetup(txProc, scenario.tx)
			}

			_, err := txProc.ValidatePermissionOperation(scenario.tx, scenario.permission, scenario.txHash)
			assert.Equal(t, scenario.ExpectedError, err)
		})
	}
}
