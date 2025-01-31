package transaction_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/versioning"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")
var errSignerMockVerifySigFails = errors.New("errSignerMockVerifySigFails")

const defaultSigSize = 64

func makeAddress(addr string) []byte {
	result := make([]byte, 32)
	copy(result, addr)
	return result
}

func mockSig(size int) []byte {
	// random 64 bytes signature
	randSig := make([]byte, size)
	_, _ = rand.Read(randSig)
	return randSig
}

var senderAddress = makeAddress("12345678123456781234567812345678")
var recvAddress = makeAddress("12345678123456781234567812345679")
var token = "KDA"

var sigOk = []byte{191, 150, 24, 156, 89, 18, 71, 123, 244, 251, 51, 26, 55, 130, 91, 227, 104, 159, 51, 243, 201, 219, 75, 212, 173, 18, 167, 48, 22, 49, 94, 136, 109, 173, 4, 140, 86, 193, 35, 146, 217, 154, 232, 45, 10, 117, 14, 144, 24, 177, 224, 125, 161, 190, 78, 156, 145, 162, 252, 143, 180, 218, 92, 9}

func createMockPubkeyConverter() *cryptoMock.PubkeyConverterMock {
	return cryptoMock.NewPubkeyConverterMock(32)
}

func createDummySigner() crypto.SingleSigner {
	return &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			if !bytes.Equal(sig, sigOk) {
				return errSignerMockVerifySigFails
			}
			return nil
		},
		SigSizeStub: func() int {
			return 64
		},
	}
}

func createKeyGenMock() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			if string(b) == "" {
				return nil, errSingleSignKeyGenMock
			}

			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

func createFreeTxFeeHandler() *mock.FeeHandlerStub {
	return &mock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*dataTransaction.CostResponse, error) {
			return &dataTransaction.CostResponse{}, nil
		},
	}
}

func createInterceptedProtoTxFromPlainTxWithFork(tx *dataTransaction.Transaction, txFeeHandler process.EconomicsDataHandler, chainID []byte, minTxVersion uint32, forkController core.ForkController) (*transaction.InterceptedTransaction, error) {
	marshalizer := &mock.ProtoMarshalizerMock{}
	txBuff, err := marshalizer.Marshal(tx)
	if err != nil {
		return nil, err
	}

	return transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			txBuff,
			marshalizer,
			marshalizer,
			mock.HasherMock{},
			createKeyGenMock(),
			createDummySigner(),
			&mock.PubkeyConverterStub{
				LenCalled: func() int {
					return 32
				},
			},
			&mock.WhiteListHandlerStub{},
			chainID,
			mock.HasherMock{},
			txFeeHandler,
			versioning.NewTxVersionChecker(minTxVersion),
			forkController,
		},
	)
}

func createInterceptedTxFromPlainTx(tx *dataTransaction.Transaction, txFeeHandler process.EconomicsDataHandler, chainID []byte, minTxVersion uint32) (*transaction.InterceptedTransaction, error) {
	forkController := &mock.ForkControllerStub{}
	return createInterceptedTxFromPlainTxWithFork(tx, txFeeHandler, chainID, minTxVersion, forkController)
}

func createInterceptedTxFromPlainTxWithFork(tx *dataTransaction.Transaction, txFeeHandler process.EconomicsDataHandler, chainID []byte, minTxVersion uint32, forkController core.ForkController) (*transaction.InterceptedTransaction, error) {
	marshalizer := &mock.MarshalizerMock{}
	txBuff, err := marshalizer.Marshal(tx)
	if err != nil {
		return nil, err
	}

	return transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			txBuff,
			marshalizer,
			marshalizer,
			mock.HasherMock{},
			createKeyGenMock(),
			createDummySigner(),
			&mock.PubkeyConverterStub{
				LenCalled: func() int {
					return 32
				},
			},
			&mock.WhiteListHandlerStub{},
			chainID,
			mock.HasherMock{},
			txFeeHandler,
			versioning.NewTxVersionChecker(minTxVersion),
			forkController,
		},
	)
}

//------- NewInterceptedTransaction

func TestNewInterceptedTransaction_NilBufferShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			nil,
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilBuffer, err)
}

func TestNewInterceptedTransaction_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			nil,
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedTransaction_NilSignMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			nil,
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedTransaction_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			nil,
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewInterceptedTransaction_NilKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			nil,
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, common.ErrNilKeyGen, err)
}

func TestNewInterceptedTransaction_NilSignerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			nil,
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, common.ErrNilSingleSigner, err)
}

func TestNewInterceptedTransaction_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			nil,
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewInterceptedTransaction_NilWhiteListerVerifiedTxsShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			nil,
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilWhiteListHandler, err)
}

func TestNewInterceptedTransaction_InvalidChainIDShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			nil,
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrInvalidChainID, err)
}

func TestNewInterceptedTransaction_NilTxSignHasherShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			nil,
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewInterceptedTransaction_NilTxFeeHandlerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			nil,
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilEconomicsFeeHandler, err)
}

func TestNewInterceptedTransaction_NilTxVersionCheckerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			nil,
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilTransactionVersionChecker, err)
}

func TestNewInterceptedTransaction_NilForkControllerShouldErr(t *testing.T) {
	t.Parallel()

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerMock{},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			versioning.NewTxVersionChecker(1),
			nil,
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, process.ErrNilForkController, err)
}

func TestNewInterceptedTransaction_UnmarshalingTxFailsShouldErr(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected error")

	txi, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			make([]byte, 0),
			&mock.MarshalizerStub{
				UnmarshalCalled: func(obj interface{}, buff []byte) error {
					return errExpected
				},
			},
			&mock.MarshalizerMock{},
			mock.HasherMock{},
			&cryptoMock.SingleSignKeyGenMock{},
			&cryptoMock.SignerMock{},
			createMockPubkeyConverter(),
			&mock.WhiteListHandlerStub{},
			[]byte("chainID"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			versioning.NewTxVersionChecker(1),
			&mock.ForkControllerStub{},
		},
	)

	assert.Nil(t, txi)
	assert.Equal(t, errExpected, err)
}

func TestNewInterceptedTransaction_ShouldWork(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)

	assert.False(t, check.IfNil(txi))
	assert.Nil(t, err)
	assert.Equal(t, tx, txi.Transaction())
}

//------- CheckValidity

func TestInterceptedTransaction_CheckValidityNilSenderAddressShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)
	tx.RawData.Sender = nil

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)

	err = txi.CheckValidity()
	assert.Equal(t, process.ErrInvalidSndAddr, err)
}

func TestInterceptedTransaction_CheckContractShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)

	err := txi.CheckValidity()
	assert.Equal(t, process.ErrInvalidTransactionNoContract, err)
}

func TestInterceptedTransaction_CheckValidityInvalidSenderAddressShouldErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		address       []byte
		expectedError error
	}{
		{
			name:          "invalid sender address by len",
			address:       append(senderAddress, 0),
			expectedError: process.ErrInvalidSndAddr,
		},
		{
			name:          "invalid sender zero address",
			address:       core.ZeroAddress,
			expectedError: process.ErrInvalidSndAddr,
		},
		{
			name:          "invalid sender block hole address",
			address:       core.BlackHoleAddress,
			expectedError: process.ErrInvalidSndAddr,
		},
		{
			name:          "invalid sender sc address",
			address:       []byte{0, 0, 0, 0, 0, 0, 0, 0, 5, 0, 10, 20, 148, 158, 139, 252, 167, 146, 233, 170, 147, 178, 217, 87, 118, 29, 225, 200, 240, 52, 195, 230},
			expectedError: process.ErrInvalidSndAddr,
		},
		{
			name:          "valid sender address",
			address:       senderAddress,
			expectedError: nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			minTxVersion := uint32(1)
			chainID := []byte("chain")
			tx := dataTransaction.NewBaseTransaction(tt.address, 0, [][]byte{[]byte("data")}, 1, 100)
			tx.Signature = append(tx.Signature, sigOk)
			_ = tx.SetChainID(chainID)

			err := AddTransfer(tx, createMockPubkeyConverter(), nil, recvAddress, token, 10)
			assert.Nil(t, err)

			txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
			assert.Nil(t, err)

			err = txi.CheckValidity()
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestInterceptedTransaction_CheckValidityInvalidSenderShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction([]byte(""), 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	_ = AddTransfer(tx, createMockPubkeyConverter(), []byte(""), recvAddress, token, 10)

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)

	err := txi.CheckValidity()

	assert.NotNil(t, err)
}

func TestInterceptedTransaction_CheckValidityWrongChainIDShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	correctChainID := []byte("correct")
	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), correctChainID, minTxVersion)
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, process.ErrInvalidChainID, err)
}

func TestInterceptedTransaction_CheckValidityInvalidVersionShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(2)
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)

	correctChainID := []byte("correct")
	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), correctChainID, minTxVersion)

	err := txi.CheckValidity()
	_ = err
	assert.Equal(t, process.ErrInvalidTransactionVersion, err)
}

func TestInterceptedTransaction_TransactionWithNilChainIDShouldErr(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, process.ErrInvalidChainID, err)
}

func TestInterceptedTransaction_CheckValidityPriorForkShouldWork(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_CheckSizeValidityShouldWork(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	tx.Signature = append(tx.Signature, sigOk)

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_CheckSizeValidityMultiTransactionsShouldWork(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	tx.Signature = append(tx.Signature, sigOk)

	for i := 0; i < 20; i++ {
		err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 100000)
		require.Nil(t, err)
	}

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_CheckSizeValidityShouldFail(t *testing.T) {
	t.Parallel()

	chainID := make([]byte, core.MaxTxSize+1)

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	tx.Signature = append(tx.Signature, sigOk)

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	require.NotNil(t, err)
	assert.Equal(t, common.ErrInvalidTransactionRawSize, err)
}

func TestInterceptedTransaction_CheckSizeValidityShouldWork_MultipleSigners(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")

	tx := dataTransaction.NewBaseTransaction(sender, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	signatures := make([][]byte, 0)
	for i := 0; i < core.MaxPermissionSigners; i++ {
		signatures = append(signatures, mockSig(defaultSigSize))
	}
	tx.Signature = signatures

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})

	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_CheckSizeValidityShouldWork_SmartContractOverhead(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")
	data := make([][]byte, 1)
	data[0] = bytes.Repeat([]byte{255}, core.MaxDataSize-1)

	tx := dataTransaction.NewBaseTransaction(sender, math.MaxInt64, data, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	signatures := make([][]byte, 0)
	for i := 0; i < core.MaxPermissionSigners; i++ {
		signatures = append(signatures, mockSig(defaultSigSize))
	}
	tx.Signature = signatures

	err := AddDeploySmartContract(tx, createMockPubkeyConverter(), senderAddress)
	assert.Nil(t, err)

	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_MaxDataSizeShouldFail(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")
	data := make([][]byte, 1)
	data[0] = bytes.Repeat([]byte{255}, core.MaxDataSize)

	tx := dataTransaction.NewBaseTransaction(sender, math.MaxInt64, data, math.MaxInt64, math.MaxInt64)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	tx.SetChainID(chainID)
	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64
	tx.Signature = append(tx.Signature, sigOk)

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, common.ErrDataFieldTooBig, err)
}

func TestInterceptedTransaction_MaxContractSizeShouldFail(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")
	data := make([][]byte, 1)

	tx := dataTransaction.NewBaseTransaction(sender, math.MaxInt64, data, math.MaxInt64, math.MaxInt64)
	// add invalid contract data
	tx.RawData.Contract = []*dataTransaction.TXContract{
		{
			Type: dataTransaction.TXContract_TransferContractType,
			Parameter: &anypb.Any{
				Value: make([]byte, dataTransaction.ContractMaxSizes[dataTransaction.TXContract_TransferContractType]+1),
			},
		},
	}

	tx.SetChainID(chainID)
	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64
	tx.Signature = append(tx.Signature, sigOk)

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, common.ErrInvalidContractSize, err)
}

func TestInterceptedTransaction_InvalidContractMarshalShouldFail(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")
	data := make([][]byte, 1)

	tx := dataTransaction.NewBaseTransaction(sender, math.MaxInt64, data, math.MaxInt64, math.MaxInt64)
	// add invalid contract data
	tx.RawData.Contract = []*dataTransaction.TXContract{
		{
			Type: dataTransaction.TXContract_TransferContractType,
			Parameter: &anypb.Any{
				Value: make([]byte, 10),
			},
		},
	}

	tx.SetChainID(chainID)
	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64
	tx.Signature = append(tx.Signature, sigOk)

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Contains(t, err.Error(), "mismatched message type:")
}

func TestInterceptedTransaction_CheckSizeValidityShouldFail_MultipleSigners(t *testing.T) {
	t.Parallel()

	chainID := []byte("chain")

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainID)
	tx.RawData.PermissionID = math.MaxInt32
	signatures := make([][]byte, 0)
	for i := 0; i < core.MaxPermissionSigners+1; i++ {
		signatures = append(signatures, mockSig(defaultSigSize))
	}
	tx.Signature = signatures

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	tx.GasLimit = math.MaxUint64
	tx.GasMultiplier = math.MaxUint64

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})

	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, common.ErrExceedsMaxSignatures, err)
}

func TestInterceptedTransaction_OkValsGettersShouldWork(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
	assert.Nil(t, err)

	assert.Equal(t, tx, txi.Transaction())
}

func TestInterceptedTransaction_GetSenderAddress(t *testing.T) {
	t.Parallel()

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
	assert.Nil(t, err)
	result := txi.SenderAddress()
	assert.NotNil(t, result)
}

func TestInterceptedTransaction_InvalidTransactionSize(t *testing.T) {
	t.Parallel()

	dataHex := "0a9b01080112201df232332e62e12414ce5605ee58c1341de9099683306eb188f69a8cb8825ce3326312610a2a747970652e676f6f676c65617069732e636f6d2f70726f746f2e5472616e73666572436f6e747261637412330a20e360b34b852a780dc298835e3296ddb0b74a75d181e25769fa210ce232bcf0d4120a474f48414e2d315a445518bfc5b20b520068c0843d70c0843d78018201033130381240e3363519d3ebc30ad57943eac548c74bc5215b01d7d5a9d289f0fc15554ae5be099d5e3633b4a6a445321c78e63b61e1c284aa3d92224db5358b669c9b36960730015207696e76616c6964"
	data, err := hex.DecodeString(dataHex)
	require.Nil(t, err)

	chainID := []byte("108")

	tx := &dataTransaction.Transaction{}
	marshalizer := &mock.ProtoMarshalizerMock{}
	err = marshalizer.Unmarshal(tx, data)
	require.Nil(t, err)

	txi, err := createInterceptedProtoTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, common.ErrInvalidTransactionSize, err)
}

// ------- IsInterfaceNil
func TestInterceptedTransaction_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var txi *transaction.InterceptedTransaction

	assert.True(t, check.IfNil(txi))
}

func TestInterceptedTransaction_Type(t *testing.T) {
	t.Parallel()

	expectedType := "intercepted tx"

	intx := &transaction.InterceptedTransaction{}

	assert.Equal(t, expectedType, intx.Type())
}

func TestInterceptedTransaction_Fee(t *testing.T) {
	t.Parallel()

	kappFee := int64(150)
	bwFee := int64(100)
	sndAddr := []byte("snd")

	tx := dataTransaction.NewBaseTransaction(sndAddr, 0, nil, kappFee, bwFee)
	tx.Signature = append(tx.Signature, sigOk)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	require.Nil(t, err)

	marshalizer := &mock.MarshalizerMock{}
	txBuff, _ := marshalizer.Marshal(tx)

	txin, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			txBuff,
			marshalizer,
			marshalizer,
			mock.HasherMock{},
			createKeyGenMock(),
			createDummySigner(),
			&mock.PubkeyConverterStub{},
			&mock.WhiteListHandlerStub{},
			[]byte("T"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			versioning.NewTxVersionChecker(1),
			&mock.ForkControllerStub{},
		},
	)
	require.Nil(t, err)

	assert.Equal(t, kappFee+bwFee, txin.Fee())
}

func TestInterceptedTransaction_String(t *testing.T) {
	t.Parallel()

	value := int64(150)
	sndAddr := []byte("snd")

	tx := dataTransaction.NewBaseTransaction(sndAddr, 0, nil, value, 100)
	tx.Signature = append(tx.Signature, sigOk)
	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	require.Nil(t, err)

	marshalizer := &mock.MarshalizerMock{}
	txBuff, _ := marshalizer.Marshal(tx)

	txin, err := transaction.NewInterceptedTransaction(
		&transaction.InterceptedTransactionArgs{
			txBuff,
			marshalizer,
			marshalizer,
			mock.HasherMock{},
			createKeyGenMock(),
			createDummySigner(),
			&mock.PubkeyConverterStub{},
			&mock.WhiteListHandlerStub{},
			[]byte("T"),
			mock.HasherMock{},
			createFreeTxFeeHandler(),
			versioning.NewTxVersionChecker(1),
			&mock.ForkControllerStub{},
		},
	)
	require.Nil(t, err)

	expectedFormat := fmt.Sprintf(
		"sender=%s, nonce=0, fees=%d",
		logger.DisplayByteSlice(sndAddr), value,
	)

	assert.Equal(t, expectedFormat, txin.String())
}

func AddTransfer(tx *dataTransaction.Transaction,
	addressPubkeyConverter core.PubkeyConverter,
	sender, receiver []byte,
	token string, amount int64) error {

	receiverAddress := addressPubkeyConverter.Encode(receiver)

	contract := &models.TransferTXRequest{
		Receiver: receiverAddress,
		Amount:   amount,
		KDA:      token,
	}

	return AddTransaction(tx, sender, contract, addressPubkeyConverter, dataTransaction.TXContract_TransferContractType)
}

func AddDeploySmartContract(tx *dataTransaction.Transaction,
	addressPubkeyConverter core.PubkeyConverter,
	sender []byte) error {

	contract := &models.SmartContractRequest{
		SCType:    int32(dataTransaction.SmartContract_SCDeploy),
		Address:   "",
		CallValue: make(map[string]int64, 0),
	}

	return AddTransaction(tx, sender, contract, addressPubkeyConverter, dataTransaction.TXContract_SmartContractType)
}

func AddTransaction(tx *dataTransaction.Transaction, sender []byte, contract interface{}, addressPubkeyConverter core.PubkeyConverter, txType dataTransaction.TXContract_ContractType) error {
	cJson, err := json.Marshal(contract)
	if err != nil {
		return err
	}

	nodeHelper := mock.NewNodeHelperMock()
	nodeHelper.GetAddressPCKCalled = func() core.PubkeyConverter {
		return addressPubkeyConverter
	}
	nodeHelper.GetValidatorPCKCalled = func() core.PubkeyConverter {
		return addressPubkeyConverter
	}
	nodeHelper.GetEncodedAddressLengthCalled = func() int {
		return addressPubkeyConverter.Len() * 2
	}
	nodeHelper.GetAssetCalled = func(address string) (*kapps.KDAData, error) {
		return &kapps.KDAData{}, nil
	}

	txArgs := dataTransaction.TXArgs{
		Type:       uint32(txType),
		Sender:     senderAddress,
		Data:       nil,
		Contract:   cJson,
		NodeHelper: nodeHelper,
	}

	return tx.AddTransaction(txArgs)
}

func TestInterceptedTransaction_CheckTXSignature(t *testing.T) {
	t.Parallel()

	sig1 := mockSig(defaultSigSize)
	sig2 := mockSig(defaultSigSize)

	tests := []struct {
		name          string
		signatures    [][]byte
		expectedError error
	}{
		{
			name:          "EmptySignature",
			signatures:    [][]byte{},
			expectedError: common.ErrNoSignatures,
		},
		{
			name:          "TooManySignatures",
			signatures:    make([][]byte, core.MaxPermissionSigners+1),
			expectedError: common.ErrExceedsMaxSignatures,
		},
		{
			name:          "InvalidSignatureLength",
			signatures:    [][]byte{sig1, make([]byte, 0)},
			expectedError: common.ErrInvalidSignatureLength,
		},
		{
			name:          "DuplicateSignature",
			signatures:    [][]byte{sig1, sig1},
			expectedError: common.ErrDupSignature,
		},
		{
			name:          "ValidSignatures",
			signatures:    [][]byte{sig1, sig2},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
			tx.Signature = tt.signatures

			txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), []byte("chainID"), 1)

			err := txi.CheckTXSignature()
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestInterceptedTransaction_Info(t *testing.T) {
	t.Parallel()

	expectedNonce := uint64(5)
	expectedPermissionID := int32(1)
	expectedSignatures := [][]byte{[]byte("sig1"), []byte("sig2")}
	expectedKDAFee := &dataTransaction.Transaction_KDAFee{
		KDA:    []byte("KLV"),
		Amount: 100,
	}

	tx := dataTransaction.NewBaseTransaction(senderAddress, expectedNonce, [][]byte{[]byte("data")}, 1, 100)
	tx.RawData.PermissionID = expectedPermissionID
	tx.Signature = expectedSignatures
	tx.RawData.KDAFee = expectedKDAFee

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), []byte("chainID"), 1)

	hash := txi.Hash()
	assert.Equal(t, "bf134f9da01202af6a10a042b2c1759ae3fcc7fb3249f6dd7e209a411ec65cfe", hex.EncodeToString(hash))
	assert.Equal(t, expectedNonce, txi.Nonce())
	assert.Equal(t, expectedPermissionID, txi.PermissionID())
	assert.Equal(t, expectedSignatures, txi.Signature())
	assert.Equal(t, expectedKDAFee, txi.KDAFee())

}

func TestInterceptedTransaction_ValidatePermissionOperation(t *testing.T) {
	t.Parallel()

	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.RawData.Contract = []*dataTransaction.TXContract{
		{Type: 0},
	}

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), []byte("chainID"), 1)

	err := txi.ValidatePermissionOperation([]byte{0})
	assert.Equal(t, common.ErrNoPermission, err)

	err = txi.ValidatePermissionOperation([]byte{1})
	assert.Nil(t, err)
}

func TestInterceptedTransaction_Identifiers(t *testing.T) {
	t.Parallel()

	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, [][]byte{[]byte("data")}, 1, 100)

	txi, _ := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), []byte("chainID"), 1)

	identifiers := txi.Identifiers()
	require.Len(t, identifiers, 1)
	assert.Equal(t, txi.Hash(), identifiers[0])
}

func TestInterceptedTransaction_CheckValidity_TxFieldsNilPointerShouldNotPanic(t *testing.T) {
	t.Parallel()

	chainID := []byte("chainID")
	tx := dataTransaction.NewBaseTransaction(senderAddress, 0, nil, 0, 0)
	tx.SetChainID(chainID)

	tx.Signature = append(tx.Signature, sigOk)

	contracts := []*dataTransaction.TXContract{
		{
			Type: dataTransaction.TXContract_TransferContractType,
		},
	}
	tx.GetRaw().Contract = contracts

	forkController := &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	}

	intx, err := createInterceptedTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainID, 1, forkController)
	require.Nil(t, err)

	// CheckValidity will validate Tx with nil pointer fields for Contract.Parameter and Contract.TypeURL
	err = intx.CheckValidity()
	assert.Equal(t, common.ErrInvalidTransactionType, err)
}
