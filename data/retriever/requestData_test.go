package retriever_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDataType_StringVals(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		r retriever.RequestDataType
		s string
	}{
		{retriever.RequestDataType_HashType, "HashType"},
		{retriever.RequestDataType_HashArrayType, "HashArrayType"},
		{retriever.RequestDataType_NonceType, "NonceType"},
		{retriever.RequestDataType_EpochType, "EpochType"},
	}

	for _, tc := range tcs {
		t.Run(tc.s, func(t *testing.T) {
			rd := tc.r.String()
			assert.Equal(t, tc.s, rd)
		})
	}
}

func TestRequestDataType_UnknownType(t *testing.T) {
	t.Parallel()

	var requestData retriever.RequestDataType = 6
	rd := requestData.String()

	assert.Equal(t, fmt.Sprintf("%d", 6), rd)
}

func TestRequestData_UnmarshalNilMarshalizer(t *testing.T) {
	t.Parallel()

	requestData := retriever.RequestData{}

	err := requestData.UnmarshalWith(nil, &mock.P2PMessageMock{})
	require.Equal(t, common.ErrNilMarshalizer, err)
}

func TestRequestData_UnmarshalNilMessageP2P(t *testing.T) {
	t.Parallel()

	requestData := retriever.RequestData{}

	err := requestData.UnmarshalWith(&mock.MarshalizerMock{}, nil)
	require.Equal(t, common.ErrNilMessage, err)
}

func TestRequestData_UnmarshalNilMessageData(t *testing.T) {
	t.Parallel()

	requestData := retriever.RequestData{}

	err := requestData.UnmarshalWith(&mock.MarshalizerMock{}, &mock.P2PMessageMock{})
	require.Equal(t, common.ErrNilDataToProcess, err)
}

func TestRequestData_CannotUnmarshal(t *testing.T) {
	t.Parallel()

	localErr := errors.New("err")
	requestData := retriever.RequestData{}

	err := requestData.UnmarshalWith(&mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return localErr
		},
	}, &mock.P2PMessageMock{
		DataField: []byte("data"),
	})
	require.Equal(t, localErr, err)
}

func TestRequestData_UnmarshalOk(t *testing.T) {
	t.Parallel()

	requestData := retriever.RequestData{}

	err := requestData.UnmarshalWith(&mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return nil
		},
	}, &mock.P2PMessageMock{
		DataField: []byte("data"),
	})
	require.Nil(t, err)
}
