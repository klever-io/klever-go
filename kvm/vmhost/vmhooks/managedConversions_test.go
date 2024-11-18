package vmhooks

import (
	"encoding/binary"
	"testing"

	"github.com/klever-io/klever-go/kapps"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/stretchr/testify/assert"
)

func TestWriteSplitRoyalties(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{}

	destionation := make([]byte, SplitRoyaltiesLen)

	royaties := &kapps.RoyaltySplitData{
		PercentTransferPercentage: 1,
		PercentTransferFixed:      2,
		PercentMarketPercentage:   3,
		PercentMarketFixed:        4,
		PercentITOPercentage:      5,
		PercentITOFixed:           6,
	}

	writeSplitRoyalties(mockManagedTypes, "addrs", royaties, destionation)

	assert.Equal(t, destionation[4:8], []byte{0, 0, 0, 1})
	assert.Equal(t, destionation[8:12], []byte{0, 0, 0, 2})
	assert.Equal(t, destionation[12:16], []byte{0, 0, 0, 3})
	assert.Equal(t, destionation[16:20], []byte{0, 0, 0, 4})
	assert.Equal(t, destionation[20:24], []byte{0, 0, 0, 5})
	assert.Equal(t, destionation[24:28], []byte{0, 0, 0, 6})
}

func TestWriteLastClaim(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntFromInt64Called: func(value int64) int32 {
			if value == 0 {
				return 0
			}
			return 1
		},
	}

	tests := []struct {
		name      string
		lastClaim *kapps.LastClaim
		expected  []byte
	}{
		{
			name: "Non-nil LastClaim",
			lastClaim: &kapps.LastClaim{
				Timestamp: 1234567890,
				Epoch:     42,
			},
			expected: []byte{0, 0, 0, 1, 0, 0, 0, 42},
		},
		{
			name:      "Nil LastClaim",
			lastClaim: nil,
			expected:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "Different values",
			lastClaim: &kapps.LastClaim{
				Timestamp: 987654321,
				Epoch:     255,
			},
			expected: []byte{0, 0, 0, 1, 0, 0, 0, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := writeLastClaim(mockManagedTypes, tt.lastClaim)

			assert.Equal(t, LastClaimLen, len(result), "Result length should be RoyaltiesLen")
			assert.Equal(t, tt.expected, result, "Result should match expected bytes")
		})
	}
}

func TestWriteLastClaimEndianness(t *testing.T) {
	lastClaim := &kapps.LastClaim{
		Timestamp: 1234567890,
		Epoch:     42,
	}

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntFromInt64Called: func(value int64) int32 {
			return 1
		},
	}

	result := writeLastClaim(mockManagedTypes, lastClaim)

	assert.Equal(t, uint32(1), binary.BigEndian.Uint32(result[0:4]), "Timestamp handle should be in big-endian")
	assert.Equal(t, uint32(42), binary.BigEndian.Uint32(result[4:8]), "Epoch should be in big-endian")
}
