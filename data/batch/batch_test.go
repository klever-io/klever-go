package batch_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addRandom(b [][]byte, len int) [][]byte {
	for i := 0; i < len; i++ {
		data := []byte(fmt.Sprintf("%04d-abcdefghijklmnopqrstuvwxyz%f ", i, float64(i)*math.Pi))
		b = append(b, data)
	}
	return b
}

func TestGZIP(t *testing.T) {
	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.Nil(t, err)

	buffer := make([][]byte, 0)
	buffer = addRandom(buffer, 3000)
	ba := batch.New(buffer...)
	ba.Algo = batch.CType_GZip

	data, _ := internalMarshalizer.Marshal(ba)
	err = ba.Compress(internalMarshalizer)
	assert.Nil(t, err)

	newData, _ := internalMarshalizer.Marshal(ba)

	dataCompressed := batch.Batch{Algo: batch.CType_GZip}
	err = internalMarshalizer.Unmarshal(&dataCompressed, newData)
	assert.Nil(t, err)
	assert.True(t, dataCompressed.IsCompressed)
	err = dataCompressed.Decompress(internalMarshalizer)
	assert.Nil(t, err)
	assert.False(t, dataCompressed.IsCompressed)

	assert.Equal(t, buffer, dataCompressed.Data)

	fmt.Printf("Size: %d, New Size %d\n", len(data), len(newData))
}

// gzipBytes returns the gzip-compressed encoding of payload.
func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(payload)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// Regression: GHSA-74m6-4hjp-7226 / KLC-2352 — a compressed batch whose inflated payload
// would exceed MaxDecompressedBatchSize must be rejected without ever allocating past the cap.
func TestDecompress_RejectsBombOverHardCap(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	// One byte past the cap is enough to prove the LimitReader bound — we don't have to
	// allocate a full GB to exercise the defense.
	payload := bytes.Repeat([]byte{0}, batch.MaxDecompressedBatchSize+1)
	stream := gzipBytes(t, payload)

	bomb := &batch.Batch{
		IsCompressed: true,
		Algo:         batch.CType_GZip,
		Stream:       stream,
		DataSize:     int32(len(payload)), // honest field — but ignored by the cap
	}

	err = bomb.Decompress(internalMarshalizer)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, common.ErrDecompressionTooLarge),
		"expected ErrDecompressionTooLarge, got %v", err)
}

// A high-ratio "deflate of deflate" wire payload (a few KB) that still inflates past the
// hard cap on the first read must be caught the same way.
func TestDecompress_RejectsHighCompressionRatioBomb(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	// 16 MiB beyond the cap, all zero — gzips to <100 KB, well under any wire limit.
	overflow := batch.MaxDecompressedBatchSize + (16 << 20)
	payload := make([]byte, overflow)
	stream := gzipBytes(t, payload)
	require.Less(t, len(stream), 1<<20,
		"sanity: bomb payload should compress to under 1 MB, got %d", len(stream))

	bomb := &batch.Batch{
		IsCompressed: true,
		Algo:         batch.CType_GZip,
		Stream:       stream,
		DataSize:     int32(len(payload)), // #nosec G115 — test fixture
	}

	err = bomb.Decompress(internalMarshalizer)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, common.ErrDecompressionTooLarge),
		"expected ErrDecompressionTooLarge, got %v", err)
}

// LZ4 currently delegates to gzip; the same hard cap must apply on that branch too.
func TestDecompress_LZ4_AlsoBoundedByHardCap(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	payload := bytes.Repeat([]byte{0}, batch.MaxDecompressedBatchSize+1)
	stream := gzipBytes(t, payload)

	bomb := &batch.Batch{
		IsCompressed: true,
		Algo:         batch.CType_LZ4,
		Stream:       stream,
		DataSize:     int32(len(payload)),
	}

	err = bomb.Decompress(internalMarshalizer)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, common.ErrDecompressionTooLarge),
		"expected ErrDecompressionTooLarge on LZ4 branch, got %v", err)
}

// A compressed batch whose self-reported DataSize disagrees with the inflated payload
// must be rejected before re-Unmarshal — defense-in-depth against crafted streams that
// stay under the hard cap. The check runs unconditionally so DataSize=0 and negative
// values do not provide a bypass.
func TestDecompress_RejectsDataSizeMismatch(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	// Build a legitimate compressed batch we can clone and tamper.
	original := batch.New(addRandom(make([][]byte, 0), 50)...)
	original.Algo = batch.CType_GZip
	require.NoError(t, original.Compress(internalMarshalizer))

	tamperCases := map[string]int32{
		"DataSize+1":     original.DataSize + 1,
		"DataSize-1":     original.DataSize - 1,
		"DataSize=0":     0,  // attacker tries to bypass by clearing the field
		"DataSize=-1":    -1, // attacker tries an impossible negative value
		"DataSize=MaxIn": math.MaxInt32,
	}

	for name, lie := range tamperCases {
		t.Run(name, func(t *testing.T) {
			tampered := &batch.Batch{
				IsCompressed: true,
				Algo:         original.Algo,
				Stream:       original.Stream,
				DataSize:     lie,
			}

			err := tampered.Decompress(internalMarshalizer)
			require.Error(t, err)
			require.Truef(t, errors.Is(err, common.ErrDecompressedSizeMismatch),
				"expected ErrDecompressedSizeMismatch for %s, got %v", name, err)
		})
	}
}

func BenchmarkCompress(b *testing.B) {
	algos := []batch.CType{batch.CType_GZip, batch.CType_LZ4}
	sizes := []int{100, 1000, 3000}
	b.ReportAllocs()
	internalMarshalizer, _ := factory.NewMarshalizer(factory.ProtoMarshalizer)

	compressionResult := make(map[string]float64)

	for _, s := range sizes {
		for _, a := range algos {
			b.Run(fmt.Sprintf("%s-%d", a, s), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					buffer := addRandom(make([][]byte, 0), s)
					ba := batch.New(buffer...)
					ba.Algo = a
					size := ba.DataSize
					b.StartTimer()
					err := ba.Compress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
					b.StopTimer()

					compressionResult[b.Name()] = (1 - float64(ba.DataSize)/float64(size)) * 100
				}
			})
		}
	}
	b.Errorf("%v", compressionResult)
}

func BenchmarkDecompress(b *testing.B) {
	algos := []batch.CType{batch.CType_GZip, batch.CType_LZ4}
	sizes := []int{100, 1000, 3000}
	b.ReportAllocs()
	internalMarshalizer, _ := factory.NewMarshalizer(factory.ProtoMarshalizer)

	for _, s := range sizes {
		for _, a := range algos {
			b.Run(fmt.Sprintf("%s-%d", a, s), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					buffer := addRandom(make([][]byte, 0), s)
					ba := batch.New(buffer...)
					ba.Algo = a
					err := ba.Compress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
					b.StartTimer()
					err = ba.Decompress(internalMarshalizer)
					if err != nil {
						b.Errorf("Compress error: %v", err)
					}
				}
			})
		}
	}
}
