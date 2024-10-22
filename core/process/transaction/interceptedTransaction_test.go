package transaction_test

import (
	"bytes"
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
)

var errSingleSignKeyGenMock = errors.New("errSingleSignKeyGenMock")
var errSignerMockVerifySigFails = errors.New("errSignerMockVerifySigFails")

var senderAddress = []byte("12345678123456781234567812345678")
var recvAddress = []byte("1234567812345679")
var token = "KDA"
var sigOk = []byte("signature")

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

	minTxVersion := uint32(1)
	chainID := []byte("chain")
	tx := dataTransaction.NewBaseTransaction(nil, 0, [][]byte{[]byte("data")}, 1, 100)
	tx.Signature = append(tx.Signature, sigOk)
	_ = tx.SetChainID(chainID)

	err := AddTransfer(tx, createMockPubkeyConverter(), append(senderAddress, 0), recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTx(tx, createFreeTxFeeHandler(), chainID, minTxVersion)
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Equal(t, process.ErrInvalidSndAddr, err)
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

func TestInterceptedTransaction_CheckValidityOkValsShouldWork(t *testing.T) {
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

	chainId := make([]byte, core.MaxLengthForAssetTicker)

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainId)
	tx.RawData.PermissionID = math.MaxInt32
	tx.Signature = append(tx.Signature, sigOk)

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainId, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedTransaction_CheckSizeValidityShoulFail(t *testing.T) {
	t.Parallel()

	chainId := make([]byte, core.MaxTxSize+1)

	tx := dataTransaction.NewBaseTransaction(senderAddress, math.MaxInt64, [][]byte{}, math.MaxInt64, math.MaxInt64)
	tx.RawData.KDAFee = &dataTransaction.Transaction_KDAFee{
		KDA:    make([]byte, core.MaxLengthForAssetTicker),
		Amount: math.MaxInt64,
	}
	tx.SetChainID(chainId)
	tx.RawData.PermissionID = math.MaxInt32
	tx.Signature = append(tx.Signature, sigOk)

	err := AddTransfer(tx, createMockPubkeyConverter(), senderAddress, recvAddress, token, 10)
	assert.Nil(t, err)

	txi, err := createInterceptedTxFromPlainTxWithFork(tx, createFreeTxFeeHandler(), chainId, 1, &mock.ForkControllerStub{
		EnableSmartContractsValue: true,
	})
	assert.Nil(t, err)

	err = txi.CheckValidity()
	require.NotNil(t, err)
	assert.Equal(t, common.ErrInvalidTransactionSize, err)
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

	return AddTransaction(tx, sender, contract, addressPubkeyConverter)
}

func AddTransaction(tx *dataTransaction.Transaction, sender []byte, contract interface{}, addressPubkeyConverter core.PubkeyConverter) error {
	cJson, _ := json.Marshal(contract)

	nodeHelper := mock.NewNodeHelperMock()
	nodeHelper.GetAddressPCKCalled = func() core.PubkeyConverter {
		return addressPubkeyConverter
	}
	nodeHelper.GetValidatorPCKCalled = func() core.PubkeyConverter {
		return addressPubkeyConverter
	}
	nodeHelper.GetAssetCalled = func(address string) (*kapps.KDAData, error) {
		return &kapps.KDAData{}, nil
	}

	txArgs := dataTransaction.TXArgs{
		Type:       uint32(dataTransaction.TXContract_TransferContractType),
		Sender:     senderAddress,
		Data:       nil,
		Contract:   cJson,
		NodeHelper: nodeHelper,
	}

	return tx.AddTransaction(txArgs)
}

func TestInterceptedTransaction_CheckTXSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		signatures    [][]byte
		expectedError error
	}{
		{
			name:          "EmptySignature",
			signatures:    [][]byte{},
			expectedError: common.ErrInvalidSignatureLength,
		},
		{
			name:          "TooManySignatures",
			signatures:    make([][]byte, core.MaxPermissionSigners+1),
			expectedError: common.ErrInvalidSignatureLength,
		},
		{
			name:          "DuplicateSignature",
			signatures:    [][]byte{[]byte("sig1"), []byte("sig1")},
			expectedError: common.ErrDupSignature,
		},
		{
			name:          "ValidSignatures",
			signatures:    [][]byte{[]byte("sig1"), []byte("sig2")},
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
