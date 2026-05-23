package interceptors_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-2357 (M5): blacklist reasons must be fixed enumerators from
// core/process, never err.Error() concatenations the attacker controls.

// Shared with singleDataInterceptor_security_test.go (same _test package).
const blacklistInjectionPayload = "\r\n2026-05-03 INFO  fake admin login from 10.0.0.1\r\n\x1b[31malert: pwn\x1b[0m"

func assertCleanBlacklistReason(t *testing.T, reasons []string, want string) {
	t.Helper()
	require.NotEmpty(t, reasons, "expected at least one BlacklistPeer call")
	for i, r := range reasons {
		assert.Equal(t, want, r, "reason[%d] must be the fixed enumerator", i)
		assert.NotContains(t, r, "fake admin login", "reason[%d] must not carry attacker err string", i)
		assert.False(t, strings.ContainsAny(r, "\r\n\x1b"), "reason[%d] must not contain control chars", i)
	}
}

func TestMultiDataInterceptor_BlacklistReasonOnFactoryCreateFailure_IsSanitized(t *testing.T) {
	t.Parallel()

	errPoison := errors.New("decode failed: " + blacklistInjectionPayload)

	var reasons []string
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(_ []byte) (process.InterceptedData, error) { return nil, errPoison },
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: [][]byte{[]byte("payload")}})
	msg := &mock.P2PMessageMock{DataField: dataField, PeerField: core.PeerID("originator")}

	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Equal(t, errPoison, err)

	assertCleanBlacklistReason(t, reasons, process.BlacklistReasonFactoryCreateFailed)
}

func TestMultiDataInterceptor_BlacklistReasonOnWrongVersion_IsSanitized(t *testing.T) {
	t.Parallel()

	// fmt.Errorf with %w preserves the attacker payload in .Error() while
	// satisfying errors.Is(target) — same shape as production wrapping.
	wrapped := fmt.Errorf("%s: %w", blacklistInjectionPayload, process.ErrInvalidChainID)

	item := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error { return wrapped },
		IdentifiersCalled:   func() [][]byte { return [][]byte{[]byte("id")} },
	}

	var reasons []string
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(_ []byte) (process.InterceptedData, error) { return item, nil },
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: [][]byte{[]byte("payload")}})
	msg := &mock.P2PMessageMock{DataField: dataField, PeerField: core.PeerID("originator")}

	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Equal(t, wrapped, err)

	assertCleanBlacklistReason(t, reasons, process.BlacklistReasonWrongVersion)
}

func TestMultiDataInterceptor_BlacklistReasonOnUnmarshalFailure_IsSanitized(t *testing.T) {
	t.Parallel()

	errUnmarshal := errors.New("unmarshal failed: " + blacklistInjectionPayload)

	var reasons []string
	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(_ interface{}, _ []byte) error { return errUnmarshal },
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	msg := &mock.P2PMessageMock{DataField: []byte("payload"), PeerField: core.PeerID("originator")}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Equal(t, errUnmarshal, err)

	assertCleanBlacklistReason(t, reasons, process.BlacklistReasonUnmarshalable)
}

// TestMultiDataInterceptor_BlacklistReasonOnDecompressFailure_IsEnumerator
// covers the decompress error branch: an attacker can flip IsCompressed=true
// and ship malformed compressed bytes. The resulting decompressGzip error
// must not flow into the BlacklistPeer reason — only the fixed enumerator.
func TestMultiDataInterceptor_BlacklistReasonOnDecompressFailure_IsEnumerator(t *testing.T) {
	t.Parallel()

	// Build a Batch with IsCompressed=true and a Stream that is a truncated
	// gzip header — decompressGzip will fail with "unexpected EOF" or similar.
	// The exact error string doesn't matter; the assertion is that it does
	// not reach the blacklist reason.
	var truncated bytes.Buffer
	gzw := gzip.NewWriter(&truncated)
	_, _ = gzw.Write([]byte("anything"))
	_ = gzw.Close()
	corrupted := truncated.Bytes()[:3] // gzip magic bytes only — header truncated

	var reasons []string
	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, _ []byte) error {
			b := obj.(*batch.Batch)
			b.IsCompressed = true
			b.Stream = corrupted
			b.DataSize = 1
			return nil
		},
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(_ core.PeerID, reason string, _ time.Duration) {
			reasons = append(reasons, reason)
		},
	}
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	msg := &mock.P2PMessageMock{DataField: []byte("anything"), PeerField: core.PeerID("originator")}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Error(t, err)

	require.Len(t, reasons, 2)
	for i, r := range reasons {
		assert.Equal(t, process.BlacklistReasonDecompressFailed, r)
		assert.False(t, strings.ContainsAny(r, "\r\n\x1b"), "reason[%d] must not contain control chars", i)
	}
}
