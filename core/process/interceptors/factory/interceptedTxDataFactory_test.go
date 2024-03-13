package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transaction"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
)

func TestNewInterceptedTxDataFactory_NilArgumentShouldErr(t *testing.T) {
	t.Parallel()

	imh, err := NewInterceptedTxDataFactory(nil)

	assert.Nil(t, imh)
	assert.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewInterceptedTxDataFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.ProtoMarshalizer = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedTxDataFactory_NilSignMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.TxSignMarshalizer = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedTxDataFactory_NilTxSignHasherShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.TxSignHasher = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewInterceptedTxDataFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.Hasher = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewInterceptedTxDataFactory_InvalidChainIDShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.ChainID = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, process.ErrInvalidChainID, err)
}

func TestNewInterceptedTxDataFactory_InvalidMinTransactionVersionShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.MinTransactionVersion = 0

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, process.ErrInvalidTransactionVersion, err)
}

func TestNewInterceptedTxDataFactory_NilKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.AccountKeyGen = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilKeyGen, err)
}

func TestNewInterceptedTxDataFactory_NilAdrConvShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.AddressPubkeyConv = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewInterceptedTxDataFactory_NilSignerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.Signer = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilSingleSigner, err)
}

func TestNewInterceptedTxDataFactory_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.EpochNotifier = nil

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.Nil(t, imh)
	assert.Equal(t, common.ErrNilEpochNotifier, err)
}

func TestInterceptedTxDataFactory_ShouldWorkAndCreate(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()

	imh, err := NewInterceptedTxDataFactory(arg)
	assert.NotNil(t, imh)
	assert.Nil(t, err)
	assert.False(t, imh.IsInterfaceNil())

	marshalizer := &mock.MarshalizerMock{}
	emptyTx := dataTransaction.NewBaseTransaction([]byte(" "), 0, [][]byte{[]byte(" ")}, 0, 0)
	emptyTxBuff, _ := marshalizer.Marshal(emptyTx)
	interceptedData, err := imh.Create(emptyTxBuff)
	assert.Nil(t, err)

	_, ok := interceptedData.(*transaction.InterceptedTransaction)
	assert.True(t, ok)
}
