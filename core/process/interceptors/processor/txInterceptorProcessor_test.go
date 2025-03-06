package processor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	testscommon "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors/processor"
	"github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func createMockTxArgument() *processor.ArgTxInterceptorProcessor {
	return &processor.ArgTxInterceptorProcessor{
		TxDataCache: testscommon.NewShardedDataStub(),
		TxValidator: &mock.TxValidatorStub{},
	}
}

func TestNewTxInterceptorProcessor_NilArgumentShouldErr(t *testing.T) {
	t.Parallel()

	txip, err := processor.NewTxInterceptorProcessor(nil)

	assert.Nil(t, txip)
	assert.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewTxInterceptorProcessor_NilDataPoolShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockTxArgument()
	arg.TxDataCache = nil
	txip, err := processor.NewTxInterceptorProcessor(arg)

	assert.Nil(t, txip)
	assert.Equal(t, common.ErrNilDataPoolHolder, err)
}

func TestNewTxInterceptorProcessor_NilTxValidatorShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockTxArgument()
	arg.TxValidator = nil
	txip, err := processor.NewTxInterceptorProcessor(arg)

	assert.Nil(t, txip)
	assert.Equal(t, process.ErrNilTxValidator, err)
}

func TestNewTxInterceptorProcessor_ShouldWork(t *testing.T) {
	t.Parallel()

	txip, err := processor.NewTxInterceptorProcessor(createMockTxArgument())

	assert.False(t, check.IfNil(txip))
	assert.Nil(t, err)
}

//------- Validate

func TestTxInterceptorProcessor_Validate(t *testing.T) {
	t.Parallel()

	dupError := errors.New("checkDup error")
	expectedErr := errors.New("tx validation error")

	tests := []struct {
		name                  string
		checkTxValidityErr    error
		checkDupErr           error
		hasRequestedItem      bool
		inputTxData           process.InterceptedData
		expectedErrorContains string
		expectedNilError      bool
	}{
		{
			name:                  "Nil transaction should return ErrWrongTypeAssertion",
			checkTxValidityErr:    nil,
			checkDupErr:           nil,
			hasRequestedItem:      false,
			inputTxData:           nil,
			expectedErrorContains: process.ErrWrongTypeAssertion.Error(),
			expectedNilError:      false,
		},
		{
			name:               "CheckDup error should return error",
			checkTxValidityErr: nil,
			checkDupErr:        dupError,
			hasRequestedItem:   false,
			inputTxData: &struct {
				mock.InterceptedDataStub
				mock.InterceptedTxHandlerStub
			}{},
			expectedErrorContains: dupError.Error(),
			expectedNilError:      false,
		},
		{
			name:               "CheckDup error but allowed by requested items",
			checkTxValidityErr: nil,
			checkDupErr:        dupError,
			hasRequestedItem:   true,
			inputTxData: &struct {
				mock.InterceptedDataStub
				mock.InterceptedTxHandlerStub
			}{},
			expectedErrorContains: "",
			expectedNilError:      true,
		},
		{
			name:               "Tx validation error should return error",
			checkTxValidityErr: expectedErr,
			checkDupErr:        nil,
			hasRequestedItem:   false,
			inputTxData: &struct {
				mock.InterceptedDataStub
				mock.InterceptedTxHandlerStub
			}{},
			expectedErrorContains: expectedErr.Error(),
			expectedNilError:      false,
		},
		{
			name:               "Tx validation success should return nil error",
			checkTxValidityErr: nil,
			checkDupErr:        nil,
			hasRequestedItem:   false,
			inputTxData: &struct {
				mock.InterceptedDataStub
				mock.InterceptedTxHandlerStub
			}{},
			expectedErrorContains: "",
			expectedNilError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arg := createMockTxArgument()
			arg.TxValidator = &mock.TxValidatorStub{
				CheckTxValidityCalled: func(txValidatorHandler process.TxValidatorHandler) error {
					return tt.checkTxValidityErr
				},
				CheckDupCalled: func(hash []byte) error {
					return tt.checkDupErr
				},
			}

			if tt.hasRequestedItem {
				arg.RequestedItemsHandler = &testscommon.RequestedItemsHandlerStub{
					HasCalled: func(key string) bool {
						return true
					},
				}
			}

			txip, _ := processor.NewTxInterceptorProcessor(arg)
			err := txip.Validate(tt.inputTxData, "")

			if tt.expectedNilError {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.True(t, strings.Contains(err.Error(), tt.expectedErrorContains))
			}
		})
	}
}

//------- Save

func TestTxInterceptorProcessor_SaveNilDataShouldErr(t *testing.T) {
	t.Parallel()

	txip, _ := processor.NewTxInterceptorProcessor(createMockTxArgument())

	err := txip.Save(nil, "", "")

	assert.Equal(t, process.ErrWrongTypeAssertion, err)
}

func TestTxInterceptorProcessor_SaveShouldWork(t *testing.T) {
	t.Parallel()

	addedWasCalled := false
	txInterceptedData := &struct {
		mock.InterceptedDataStub
		mock.InterceptedTxHandlerStub
	}{
		InterceptedDataStub: mock.InterceptedDataStub{
			HashCalled: func() []byte {
				return make([]byte, 0)
			},
		},
		InterceptedTxHandlerStub: mock.InterceptedTxHandlerStub{
			TransactionCalled: func() data.TransactionHandler {
				return &transaction.Transaction{}
			},
		},
	}
	arg := createMockTxArgument()
	txDataCache := arg.TxDataCache.(*testscommon.ShardedDataStub)
	txDataCache.AddDataCalled = func(key []byte, data interface{}, sizeInBytes int, cacheId string) {
		addedWasCalled = true
	}

	txip, _ := processor.NewTxInterceptorProcessor(arg)

	err := txip.Save(txInterceptedData, "", "")

	assert.Nil(t, err)
	assert.True(t, addedWasCalled)
}

//------- IsInterfaceNil

func TestTxInterceptorProcessor_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var txip *processor.TxInterceptorProcessor

	assert.True(t, check.IfNil(txip))
}
