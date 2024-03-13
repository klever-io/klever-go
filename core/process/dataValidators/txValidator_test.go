package dataValidators_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/memorydb"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

var storageTest = initStore().GetStorer(retriever.TransactionUnit)

func initStore() *retriever.ChainStorer {
	store := retriever.NewChainStorer()
	store.AddStorer(retriever.TransactionUnit, generateTestUnit())
	store.AddStorer(retriever.BlockUnit, generateTestUnit())
	return store
}

func getKAppController() kapp.KAppController {
	marshalizerMock := &mock.ProtoMarshalizerMock{}

	accMock := &mock.AccountsStub{
		CommitCalled: func() ([]byte, error) {
			return nil, nil
		},
	}

	accCacher, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: accMock,
			Kapps:    accMock,
			Peers:    accMock,
		},
	)
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:         &mock.HasherStub{},
		Marshalizer:    marshalizerMock,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
		AccountsCacher: accCacher,
		RatingsData:    &mock.RatingsInfoMock{},
	}

	kAppController, _ := kappcontroller.NewKappController(argsKapp)

	_ = kAppController.GetValidatorsKApp().SetAccountsCacher(accCacher)
	return kAppController
}

func generateTestUnit() storage.Storer {
	storer, _ := storageUnit.NewStorageUnit(
		generateTestCache(),
		memorydb.New(),
	)

	return storer
}

func generateTestCache() storage.Cacher {
	cache, _ := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 1000, Shards: 1, SizeInBytes: 0})
	return cache
}

func getAccAdapter(balance int64) *mock.AccountsStub {
	accDB := &mock.AccountsStub{}
	accDB.GetExistingAccountCalled = func(address []byte) (handler state.AccountHandler, e error) {
		acc, _ := state.NewUserAccount(address)
		acc.Balance = balance

		return acc, nil
	}

	return accDB
}

func generateTxValidator() process.TxValidator {
	adb := getAccAdapter(0)

	txValidator, _ := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{
			VerifyCalled: func(public crypto.PublicKey, msg, sig []byte) error {
				return nil
			},
		},
		&cryptoMock.KeyGenMock{
			PublicKeyFromByteArrayMock: func(b []byte) (crypto.PublicKey, error) {
				return &cryptoMock.PublicKeyMock{}, nil
			},
		},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	return txValidator
}

func getTxPoolsHolder() retriever.PoolsHolder {
	return &mock.PoolsHolderStub{
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
	}
}

func getTxValidatorHandler(
	sndAddr []byte,
	fee int64,
	nonce uint64,
) process.TxValidatorHandler {
	return &mock.TxValidatorHandlerStub{
		SenderAddressCalled: func() []byte {
			return sndAddr
		},
		FeeCalled: func() int64 {
			return fee
		},
		KDAFeeCalled: func() data.KDAFeeHandler {
			return nil
		},
		NonceCalled: func() uint64 {
			return nonce
		},
		SignatureCalled: func() [][]byte {
			return [][]byte{
				bytes.Repeat([]byte{0x00}, 32),
			}
		},
	}
}

func TestNewTxValidator_NilAccountsShouldErr(t *testing.T) {
	t.Parallel()

	txValidator, err := dataValidators.NewTxValidator(
		nil,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	assert.Nil(t, txValidator)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestTxValidator_NewValidatorNilWhiteListHandlerShouldErr(t *testing.T) {
	t.Parallel()

	adb := getAccAdapter(0)
	txValidator, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		nil,
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	assert.Nil(t, txValidator)
	assert.Equal(t, common.ErrNilWhiteListHandler, err)
}

func TestNewTxValidator_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	adb := getAccAdapter(0)
	txValidator, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		nil,
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	assert.Nil(t, txValidator)
	assert.True(t, errors.Is(err, common.ErrNilPubkeyConverter))
}

func TestNewTxValidator_ShouldWork(t *testing.T) {
	t.Parallel()

	adb := getAccAdapter(0)
	txValidator, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	assert.Nil(t, err)
	assert.NotNil(t, txValidator)

	result := txValidator.IsInterfaceNil()
	assert.Equal(t, false, result)
}

func TestTxValidator_CheckTxValidityAccountBalanceIsLessThanTxTotalValueShouldReturnFalse(t *testing.T) {
	t.Parallel()

	fee := int64(1000)
	accountBalance := int64(10)
	nonce := uint64(10)

	adb := getAccAdapter(accountBalance)
	txValidator, err := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)
	assert.Nil(t, err)

	addressMock := []byte("address")
	txValidatorHandler := getTxValidatorHandler(addressMock, fee, nonce)

	result := txValidator.CheckTxValidity(txValidatorHandler)
	assert.NotNil(t, result)
	assert.True(t, errors.Is(result, process.ErrInsufficientFunds))
}

func TestTxValidator_CheckTxValidityAccountNotExitsShouldReturnFalse(t *testing.T) {
	t.Parallel()

	accDB := &mock.AccountsStub{}
	accDB.GetExistingAccountCalled = func(address []byte) (handler state.AccountHandler, e error) {
		return nil, errors.New("cannot find account")
	}
	txValidator, _ := dataValidators.NewTxValidator(
		accDB,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	addressMock := []byte("address")
	txValidatorHandler := getTxValidatorHandler(addressMock, 0, 0)

	result := txValidator.CheckTxValidity(txValidatorHandler)
	assert.True(t, errors.Is(result, process.ErrAccountNotFound))
}

func TestTxValidator_CheckTxValidityAccountNotExitsButWhiteListedShouldReturnTrue(t *testing.T) {
	t.Parallel()

	accDB := &mock.AccountsStub{}
	accDB.GetExistingAccountCalled = func(address []byte) (handler state.AccountHandler, e error) {
		return nil, errors.New("cannot find account")
	}

	txValidator, _ := dataValidators.NewTxValidator(
		accDB,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{
			IsWhiteListedCalled: func(interceptedData process.InterceptedData) bool {
				return true
			},
		},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	addressMock := []byte("address")
	txValidatorHandler := getTxValidatorHandler(addressMock, 0, 0)

	interceptedTx := struct {
		process.InterceptedData
		process.TxValidatorHandler
	}{
		InterceptedData:    nil,
		TxValidatorHandler: txValidatorHandler,
	}

	// interceptedTx needs to be of type InterceptedData & TxValidatorHandler
	result := txValidator.CheckTxValidity(interceptedTx)
	assert.Nil(t, result)
}

func TestTxValidator_CheckTxValidityWrongAccountTypeShouldReturnFalse(t *testing.T) {
	t.Parallel()

	accDB := &mock.AccountsStub{}
	accDB.GetExistingAccountCalled = func(address []byte) (handler state.AccountHandler, e error) {
		return state.NewPeerAccount(address)
	}

	txValidator, _ := dataValidators.NewTxValidator(
		accDB,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)

	addressMock := []byte("address")
	txValidatorHandler := getTxValidatorHandler(addressMock, 0, 0)

	result := txValidator.CheckTxValidity(txValidatorHandler)
	assert.True(t, errors.Is(result, process.ErrWrongTypeAssertion))
}

func TestTxValidator_CheckTxValidityTxIsOkShouldReturnTrue(t *testing.T) {
	t.Parallel()

	txValidator := generateTxValidator()

	addressMock := []byte("address")
	txValidatorHandler := getTxValidatorHandler(addressMock, 0, 0)

	interceptedTx := struct {
		process.InterceptedData
		process.TxValidatorHandler
	}{
		InterceptedData:    &mock.InterceptedDataStub{},
		TxValidatorHandler: txValidatorHandler,
	}

	result := txValidator.CheckTxValidity(interceptedTx)
	assert.Nil(t, result)
}

//------- IsInterfaceNil

func TestTxValidator_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	adb := getAccAdapter(0)
	txValidator, _ := dataValidators.NewTxValidator(
		adb,
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)
	_ = txValidator
	txValidator = nil

	assert.True(t, check.IfNil(txValidator))
}
