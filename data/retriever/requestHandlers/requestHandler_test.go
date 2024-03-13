package requestHandlers

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var timeoutSendRequests = time.Second * 2

func createResolversFinderStubThatShouldNotBeCalled(tb testing.TB) *mock.ResolversFinderStub {
	return &mock.ResolversFinderStub{
		ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
			assert.Fail(tb, "ChainResolverCalled should not have been called")
			return nil, nil
		},
	}
}

func TestNewResolverRequestHandlerNilFinder(t *testing.T) {
	t.Parallel()

	rrh, err := NewResolverRequestHandler(
		nil,
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	assert.Nil(t, rrh)
	assert.Equal(t, common.ErrNilResolverFinder, err)
}

func TestNewResolverRequestHandlerNilRequestedItemsHandler(t *testing.T) {
	t.Parallel()

	rrh, err := NewResolverRequestHandler(
		&mock.ResolversFinderStub{},
		nil,
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	assert.Nil(t, rrh)
	assert.Equal(t, common.ErrNilRequestedItemsHandler, err)
}

func TestNewResolverRequestHandlerMaxTxRequestTooSmall(t *testing.T) {
	t.Parallel()

	rrh, err := NewResolverRequestHandler(
		&mock.ResolversFinderStub{},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		0,
		time.Second,
	)

	assert.Nil(t, rrh)
	assert.Equal(t, common.ErrInvalidMaxTxRequest, err)
}

func TestNewResolverRequestHandler(t *testing.T) {
	t.Parallel()

	rrh, err := NewResolverRequestHandler(
		&mock.ResolversFinderStub{},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	assert.Nil(t, err)
	assert.NotNil(t, rrh)
}

//------- RequestTransaction

func TestResolverRequestHandler_RequestTransactionErrorWhenGettingChainResolverShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not panic")
		}
	}()

	errExpected := errors.New("expected error")
	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return nil, errExpected
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTransaction(make([][]byte, 0))
}

func TestResolverRequestHandler_RequestTransactionWrongResolverShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not panic")
		}
	}()

	wrongTxResolver := &mock.HeaderResolverStub{}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return wrongTxResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTransaction(make([][]byte, 0))
}

func TestResolverRequestHandler_RequestTransactionShouldRequestTransactions(t *testing.T) {
	t.Parallel()

	chTxRequested := make(chan struct{})
	txResolver := &mock.HashSliceResolverStub{
		RequestDataFromHashArrayCalled: func(hashes [][]byte, epoch uint32) error {
			chTxRequested <- struct{}{}
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return txResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTransaction([][]byte{[]byte("txHash")})

	select {
	case <-chTxRequested:
	case <-time.After(timeoutSendRequests):
		assert.Fail(t, "timeout while waiting to call RequestDataFromHashArray")
	}

	time.Sleep(time.Second)
}

func TestResolverRequestHandler_RequestTransactionErrorsOnRequestShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not panic")
		}
	}()

	errExpected := errors.New("expected error")
	chTxRequested := make(chan struct{})
	txResolver := &mock.HashSliceResolverStub{
		RequestDataFromHashArrayCalled: func(hashes [][]byte, epoch uint32) error {
			chTxRequested <- struct{}{}
			return errExpected
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return txResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTransaction([][]byte{[]byte("txHash")})

	select {
	case <-chTxRequested:
	case <-time.After(timeoutSendRequests):
		assert.Fail(t, "timeout while waiting to call RequestDataFromHashArray")
	}

	time.Sleep(time.Second)
}

//------- RequestHeader

func TestResolverRequestHandler_RequestMetadHeaderHashAlreadyRequestedShouldNotRequest(t *testing.T) {
	t.Parallel()

	rrh, _ := NewResolverRequestHandler(
		createResolversFinderStubThatShouldNotBeCalled(t),
		&mock.RequestedItemsHandlerStub{
			HasCalled: func(key string) bool {
				return true
			},
		},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestHeader(make([]byte, 0))
}

func TestResolverRequestHandler_RequestMetadHeaderHashNotHeaderResolverShouldNotRequest(t *testing.T) {
	t.Parallel()

	wasCalled := false
	mbResolver := &mock.ResolverStub{
		RequestDataFromHashCalled: func(hash []byte, epoch uint32) error {
			wasCalled = true
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return mbResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestHeader([]byte("hdrHash"))

	assert.False(t, wasCalled)
}

func TestResolverRequestHandler_RequestHeaderShouldCallRequestOnResolver(t *testing.T) {
	t.Parallel()

	wasCalled := false
	mbResolver := &mock.HeaderResolverStub{
		RequestDataFromHashCalled: func(hash []byte, epoch uint32) error {
			wasCalled = true
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return mbResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestHeader([]byte("hdrHash"))

	assert.True(t, wasCalled)
}

//------- RequestHeaderByNonce

func TestResolverRequestHandler_RequestHeaderHashAlreadyRequestedShouldNotRequest(t *testing.T) {
	t.Parallel()

	rrh, _ := NewResolverRequestHandler(
		createResolversFinderStubThatShouldNotBeCalled(t),
		&mock.RequestedItemsHandlerStub{
			HasCalled: func(key string) bool {
				return true
			},
		},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestHeaderByNonce(0)
}

func TestResolverRequestHandler_RequestHeaderByNonceShouldRequest(t *testing.T) {
	t.Parallel()

	wasCalled := false
	hdrResolver := &mock.HeaderResolverStub{
		RequestDataFromNonceCalled: func(nonce uint64, epoch uint32) error {
			wasCalled = true
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, e error) {
				return hdrResolver, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		100,
		time.Second,
	)

	rrh.RequestHeaderByNonce(0)

	assert.True(t, wasCalled)
}

func TestRequestTrieNodes_ShouldWork(t *testing.T) {
	t.Parallel()

	chTxRequested := make(chan struct{})
	resolverMock := &mock.HashSliceResolverStub{
		RequestDataFromHashArrayCalled: func(hash [][]byte, epoch uint32) error {
			chTxRequested <- struct{}{}
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (retriever.Resolver, error) {
				return resolverMock, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTrieNodes([][]byte{[]byte("hash")}, "topic")
	select {
	case <-chTxRequested:
	case <-time.After(timeoutSendRequests):
		assert.Fail(t, "timeout while waiting to call RequestDataFromHashArray")
	}

	time.Sleep(time.Second)
}

func TestRequestTrieNodes_NilResolver(t *testing.T) {
	t.Parallel()

	localError := errors.New("test error")
	called := false
	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
				called = true
				return nil, localError
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestTrieNodes([][]byte{[]byte("hash")}, "topic")
	assert.True(t, called)
}

func TestRequestStartOfEpochBlock_MissingResolver(t *testing.T) {
	t.Parallel()

	called := false
	localError := errors.New("test error")
	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
				called = true
				return nil, localError
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestStartOfEpochBlock(0)
	assert.True(t, called)
}

func TestRequestStartOfEpochBlock_WrongResolver(t *testing.T) {
	t.Parallel()

	called := false
	resolverMock := &mock.HashSliceResolverStub{}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
				called = true
				return resolverMock, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestStartOfEpochBlock(0)
	assert.True(t, called)
}

func TestRequestStartOfEpochBlock_RequestDataFromEpochError(t *testing.T) {
	t.Parallel()

	called := false
	localError := errors.New("test error")
	resolverMock := &mock.HeaderResolverStub{
		RequestDataFromEpochCalled: func(identifier []byte) error {
			called = true
			return localError
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
				return resolverMock, nil
			},
		},
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestStartOfEpochBlock(0)
	assert.True(t, called)
}

func TestRequestStartOfEpochBlock_AddError(t *testing.T) {
	t.Parallel()

	called := false
	localError := errors.New("test error")
	resolverMock := &mock.HeaderResolverStub{
		RequestDataFromEpochCalled: func(identifier []byte) error {
			return nil
		},
	}

	rrh, _ := NewResolverRequestHandler(
		&mock.ResolversFinderStub{
			ChainResolverCalled: func(baseTopic string) (resolver retriever.Resolver, err error) {
				return resolverMock, nil
			},
		},
		&mock.RequestedItemsHandlerStub{
			AddCalled: func(key string) error {
				called = true
				return localError
			},
		},
		&mock.WhiteListHandlerStub{},
		1,
		time.Second,
	)

	rrh.RequestStartOfEpochBlock(0)
	assert.True(t, called)
}
