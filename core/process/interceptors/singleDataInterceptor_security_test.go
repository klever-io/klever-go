package interceptors_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors"
	"github.com/stretchr/testify/require"
)

// KLC-2357 (M5) regressions for SingleDataInterceptor. Helpers
// (blacklistInjectionPayload, assertCleanBlacklistReason) live in
// multiDataInterceptor_security_test.go in this same _test package.

func TestSingleDataInterceptor_BlacklistReasonOnFactoryCreateFailure_IsSanitized(t *testing.T) {
	t.Parallel()

	errPoison := errors.New("decode failed: " + blacklistInjectionPayload)

	var reasons []string
	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(_ []byte) (process.InterceptedData, error) { return nil, errPoison },
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{DataField: []byte("payload"), PeerField: core.PeerID("originator")}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Equal(t, errPoison, err)

	assertCleanBlacklistReason(t, reasons, process.BlacklistReasonFactoryCreateFailed)
}

func TestSingleDataInterceptor_BlacklistReasonOnWrongVersion_IsSanitized(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("%s: %w", blacklistInjectionPayload, process.ErrInvalidTransactionVersion)

	item := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error { return wrapped },
		IdentifiersCalled:   func() [][]byte { return [][]byte{[]byte("id")} },
	}

	var reasons []string
	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(_ []byte) (process.InterceptedData, error) { return item, nil },
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{DataField: []byte("payload"), PeerField: core.PeerID("originator")}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Equal(t, wrapped, err)

	assertCleanBlacklistReason(t, reasons, process.BlacklistReasonWrongVersion)
}
