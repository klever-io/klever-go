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

// Regression: GHSA-w342-mj6g-v9c4 — inflated entry count over MaxItemsPerBatch.
func TestDecompress_RejectsItemCountBomb(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	bomb := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: make([][]byte, batch.MaxItemsPerBatch+1),
	}
	require.NoError(t, bomb.Compress(internalMarshalizer))

	tampered := &batch.Batch{
		IsCompressed: true,
		Algo:         bomb.Algo,
		Stream:       bomb.Stream,
		DataSize:     bomb.DataSize,
	}

	err = tampered.Decompress(internalMarshalizer)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, common.ErrTooManyItemsInBatch),
		"expected ErrTooManyItemsInBatch, got %v", err)
}

// Pins the cap boundary against off-by-one regressions.
func TestDecompress_AcceptsItemCountAtCap(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	ok := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: make([][]byte, batch.MaxItemsPerBatch),
	}
	require.NoError(t, ok.Compress(internalMarshalizer))
	require.NoError(t, ok.Decompress(internalMarshalizer))
	require.Equal(t, batch.MaxItemsPerBatch, len(ok.Data))
}

// Pins the Decompress cap boundary against off-by-one regressions.
// Builds a Batch whose marshaled-pre-compression size equals exactly
// MaxDecompressedBatchSize — one repeated-bytes entry of size N has framing
// of tag(1B) + length-varint(varies) + N. For N up to 2^21 − 1 the length
// varint is 3 B, so entrySize = MaxDecompressedBatchSize − 4.
func TestDecompress_AcceptsAtDecompressedBoundary(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	const entrySize = batch.MaxDecompressedBatchSize - 4

	b := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: [][]byte{make([]byte, entrySize)},
	}
	require.NoError(t, b.Compress(internalMarshalizer))
	require.Equal(t, int32(batch.MaxDecompressedBatchSize), b.DataSize, // #nosec G115 — fits int32 by construction
		"sanity: marshaled-pre-compression size must equal MaxDecompressedBatchSize")

	incoming := &batch.Batch{
		IsCompressed: true,
		Algo:         b.Algo,
		Stream:       b.Stream,
		DataSize:     b.DataSize,
	}
	require.NoError(t, incoming.Decompress(internalMarshalizer),
		"Decompress must accept an inflated payload of exactly MaxDecompressedBatchSize")
	require.Equal(t, 1, len(incoming.Data))
	require.Equal(t, entrySize, len(incoming.Data[0]))
}

// Regression for the F1 finding: a max-cap single-tx batch's marshaled size
// EXCEEDS MaxBatchWireSize by the size of proto framing. That payload must
// still round-trip cleanly through Compress → Decompress — which is the whole
// reason MaxDecompressedBatchSize is sized above MaxBatchWireSize. If
// MaxDecompressedBatchSize ever drops to MaxBatchWireSize, this test fails.
func TestDecompress_AcceptsMaxCapSingleTxBatch(t *testing.T) {
	t.Parallel()

	internalMarshalizer, err := factory.NewMarshalizer(factory.ProtoMarshalizer)
	require.NoError(t, err)

	// One entry sized at core.MaxDataSize (1 MiB) — proxy for a max-cap tx.
	// Marshaled batch is 1 + 3 + (1 << 20) = 1 MiB + 4 bytes, i.e. just above
	// MaxBatchWireSize. The all-zero content is realistic for SC-deploy
	// bytecode with padding and is the case that triggers the F1 gap.
	const txInner = 1 << 20

	b := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: [][]byte{make([]byte, txInner)},
	}
	require.NoError(t, b.Compress(internalMarshalizer))
	require.Greater(t, b.DataSize, int32(batch.MaxBatchWireSize), // #nosec G115 — fits int32 by construction
		"sanity: a max-cap single-tx batch's marshaled size DOES exceed MaxBatchWireSize — that's the framing gap MaxDecompressedBatchSize accommodates")

	incoming := &batch.Batch{
		IsCompressed: true,
		Algo:         b.Algo,
		Stream:       b.Stream,
		DataSize:     b.DataSize,
	}
	require.NoError(t, incoming.Decompress(internalMarshalizer),
		"Decompress must accept a max-cap single-tx batch (DataSize > MaxBatchWireSize but ≤ MaxDecompressedBatchSize)")
	require.Equal(t, 1, len(incoming.Data))
	require.Equal(t, txInner, len(incoming.Data[0]))
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
