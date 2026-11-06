package headerCheck_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var versionsCorrectlyConstructed = []config.VersionByEpochs{
	{
		StartEpoch: 0,
		Version:    "*",
	},
	{
		StartEpoch: 1,
		Version:    "v1",
	},
	{
		StartEpoch: 5,
		Version:    "v2",
	},
}

const defaultVersion = "default"

func TestNewHeaderIntegrityVerifier_InvalidReferenceChainIDShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		nil,
		make([]config.VersionByEpochs, 0),
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.Equal(t, headerCheck.ErrInvalidReferenceChainID, err)
}

func TestNewHeaderIntegrityVerifier_InvalidVersionElementOnEpochValuesEqualShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		[]config.VersionByEpochs{
			{
				StartEpoch: 0,
				Version:    "",
			},
			{
				StartEpoch: 0,
				Version:    "",
			},
		},
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrInvalidVersionOnEpochValues))
}

func TestNewHeaderIntegrityVerifier_InvalidVersionElementOnStringTooLongShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		[]config.VersionByEpochs{
			{
				StartEpoch: 0,
				Version:    strings.Repeat("a", core.MaxSoftwareVersionLengthInBytes+1),
			},
		},
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrInvalidVersionStringTooLong))
}

func TestNewHeaderIntegrityVerifierr_InvalidDefaultVersionShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		defaultVersion,
		nil,
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrNilCacher))
}

func TestNewHeaderIntegrityVerifier_NilCacherShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		"",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrInvalidSoftwareVersion))
}

func TestNewHeaderIntegrityVerifier_EmptyListShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		make([]config.VersionByEpochs, 0),
		"",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrEmptyVersionsByEpochsList))
}

func TestNewHeaderIntegrityVerifier_ZerothElementIsNotOnEpochZeroShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		[]config.VersionByEpochs{
			{
				StartEpoch: 1,
				Version:    "",
			},
		},
		"",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.True(t, errors.Is(err, headerCheck.ErrInvalidVersionOnEpochValues))
}

func TestNewHeaderIntegrityVerifier_ShouldWork(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	require.False(t, check.IfNil(hdrIntVer))
	require.NoError(t, err)
}

func TestHeaderIntegrityVerifier_PopulatedReservedShouldErr(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{
		Header: &block.BlockHeader{
			Reserved: []byte("r"),
		},
	}
	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		make([]config.VersionByEpochs, 0),
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	err := hdrIntVer.Verify(hdr)
	require.Equal(t, process.ErrReservedFieldNotSupportedYet, err)
}

func TestHeaderIntegrityVerifier_VerifySoftwareVersionEmptyVersionInHeaderShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		make([]config.VersionByEpochs, 0),
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	err := hdrIntVer.Verify(&block.Block{Header: &block.BlockHeader{}})
	require.True(t, errors.Is(err, headerCheck.ErrInvalidSoftwareVersion))
}

func TestHeaderIntegrityVerifierr_VerifySoftwareVersionWrongVersionShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		[]config.VersionByEpochs{
			{
				StartEpoch: 0,
				Version:    "v1",
			},
			{
				StartEpoch: 1,
				Version:    "v2",
			},
		},
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	err := hdrIntVer.Verify(
		&block.Block{Header: &block.BlockHeader{
			ChainID:         []byte("chainID"),
			SoftwareVersion: []byte("v3"),
			Epoch:           1,
		}},
	)
	require.True(t, errors.Is(err, headerCheck.ErrSoftwareVersionMismatch))
}

func TestHeaderIntegrityVerifier_VerifySoftwareVersionWildcardShouldWork(t *testing.T) {
	t.Parallel()

	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		[]config.VersionByEpochs{
			{
				StartEpoch: 0,
				Version:    "v1",
			},
			{
				StartEpoch: 1,
				Version:    "*",
			},
		},
		defaultVersion,
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	err := hdrIntVer.Verify(
		&block.Block{Header: &block.BlockHeader{
			ChainID:         []byte("chainID"),
			SoftwareVersion: []byte("v3"),
			Epoch:           1,
		}},
	)

	assert.Nil(t, err)
}

func TestHeaderIntegrityVerifier_VerifyHdrChainIDAndReferenceChainIDMismatchShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		"software",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	mb := &block.Block{Header: &block.BlockHeader{
		ChainID:         []byte("different-chainID"),
		SoftwareVersion: []byte("software"),
	}}

	err := hdrIntVer.Verify(mb)
	require.True(t, errors.Is(err, headerCheck.ErrInvalidChainID))
}

func TestHeaderIntegrityVerifier_VerifyShouldWork(t *testing.T) {
	t.Parallel()

	expectedChainID := []byte("#chainID")
	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		expectedChainID,
		versionsCorrectlyConstructed,
		"software",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	mb := &block.Block{Header: &block.BlockHeader{
		ChainID:         expectedChainID,
		SoftwareVersion: []byte("software"),
	}}

	err := hdrIntVer.Verify(mb)
	require.NoError(t, err)
}

func TestHeaderIntegrityVerifier_VerifyNotWildcardShouldWork(t *testing.T) {
	t.Parallel()

	expectedChainID := []byte("#chainID")
	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		expectedChainID,
		versionsCorrectlyConstructed,
		"software",
		&mock.CacherStub{},
		&mock.ForkControllerStub{},
	)
	mb := &block.Block{Header: &block.BlockHeader{
		ChainID:         expectedChainID,
		SoftwareVersion: []byte("v1"),
		Epoch:           1,
	}}

	err := hdrIntVer.Verify(mb)
	require.NoError(t, err)
}

func TestHeaderIntegrityVerifier_GetVersionShouldWork(t *testing.T) {
	t.Parallel()

	numPutCalls := uint32(0)
	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		defaultVersion,
		&mock.CacherStub{
			PutCalled: func(key []byte, value interface{}, sizeInBytes int) bool {
				atomic.AddUint32(&numPutCalls, 1)
				epoch := binary.BigEndian.Uint32(key)
				switch epoch {
				case 0:
					assert.Equal(t, "*", value.(string))
				case 1:
					assert.Equal(t, "v1", value.(string))
				case 2:
					assert.Equal(t, "v1", value.(string))
				case 3:
					assert.Equal(t, "v1", value.(string))
				case 4:
					assert.Equal(t, "v1", value.(string))
				case 5:
					assert.Equal(t, "v2", value.(string))
				case 6:
					assert.Equal(t, "v2", value.(string))
				case 1000:
					assert.Equal(t, "v2", value.(string))
				case 1200:
					assert.Equal(t, "v2", value.(string))
				default:
					assert.Fail(t, fmt.Sprintf("unexpected case for epoch %d", epoch))
				}

				return false
			},
		},
		&mock.ForkControllerStub{},
	)

	assert.Equal(t, defaultVersion, hdrIntVer.GetVersion(0))
	assert.Equal(t, "v1", hdrIntVer.GetVersion(1))
	assert.Equal(t, "v1", hdrIntVer.GetVersion(2))
	assert.Equal(t, "v1", hdrIntVer.GetVersion(3))
	assert.Equal(t, "v1", hdrIntVer.GetVersion(4))
	assert.Equal(t, "v2", hdrIntVer.GetVersion(5))
	assert.Equal(t, "v2", hdrIntVer.GetVersion(6))
	assert.Equal(t, "v2", hdrIntVer.GetVersion(1000))
	assert.Equal(t, "v2", hdrIntVer.GetVersion(1200))
	assert.Equal(t, uint32(9), atomic.LoadUint32(&numPutCalls))
}

func TestHeaderIntegrityVerifier_ExistsInInternalCacheShouldReturn(t *testing.T) {
	t.Parallel()

	cachedVersion := "cached version"
	hdrIntVer, _ := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		defaultVersion,
		&mock.CacherStub{
			GetCalled: func(key []byte) (value interface{}, ok bool) {
				return cachedVersion, true
			},
		},
		&mock.ForkControllerStub{},
	)

	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(0))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(1))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(2))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(500))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(999))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(1000))
	assert.Equal(t, cachedVersion, hdrIntVer.GetVersion(1200))
}

func TestNewHeaderIntegrityVerifier_NilForkControllerShouldErr(t *testing.T) {
	t.Parallel()

	hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chainID"),
		versionsCorrectlyConstructed,
		defaultVersion,
		&mock.CacherStub{},
		nil,
	)
	require.True(t, check.IfNil(hdrIntVer))
	require.Equal(t, process.ErrNilForkController, err)
}

// headerWithUnknownField decodes a header whose bytes carry a field number the schema does not know.
func headerWithUnknownField(t *testing.T, hdr *block.BlockHeader) *block.BlockHeader {
	t.Helper()
	buff, err := proto.Marshal(hdr)
	require.NoError(t, err)
	buff = mock.AppendUnknownField(buff, 99, 32)

	decoded := &block.BlockHeader{}
	require.NoError(t, proto.Unmarshal(buff, decoded))
	require.NotEmpty(t, decoded.ProtoReflect().GetUnknown())
	return decoded
}

func TestHeaderIntegrityVerifier_VerifyUnknownHeaderFields(t *testing.T) {
	t.Parallel()

	activation := uint32(5)
	chainID := []byte("chainID")
	newHeader := func(epoch uint32) *block.BlockHeader {
		return &block.BlockHeader{ChainID: chainID, SoftwareVersion: []byte("software"), Epoch: epoch}
	}

	tests := []struct {
		name   string
		header *block.BlockHeader
		err    error
	}{
		{name: "unknown field before activation", header: headerWithUnknownField(t, newHeader(activation-1))},
		{name: "unknown field from activation", header: headerWithUnknownField(t, newHeader(activation)), err: process.ErrHeaderUnknownFields},
		{name: "unknown field after activation", header: headerWithUnknownField(t, newHeader(activation+1)), err: process.ErrHeaderUnknownFields},
		{name: "clean header from activation", header: newHeader(activation)},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The value says the current epoch is pre-fork: the check must follow the header's epoch.
			hdrIntVer, err := headerCheck.NewHeaderIntegrityVerifier(
				chainID,
				[]config.VersionByEpochs{{StartEpoch: 0, Version: "*"}},
				"software",
				&mock.CacherStub{},
				&mock.ForkControllerStub{FixAuditChangesV7Epoch: &activation, FixAuditChangesV7Value: false},
			)
			require.NoError(t, err)

			err = hdrIntVer.Verify(&block.Block{Header: tt.header})
			require.Equal(t, tt.err, err)
		})
	}
}
