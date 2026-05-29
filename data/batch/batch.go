//go:generate protoc -I=proto -I=$GOPATH/src -I=$GOPATH/src/github.com/klever-io/klever-go/protobuf --go_out=. batch.proto
package batch

import (
	bytes "bytes"
	"compress/gzip"
	"io"

	//lz4 "github.com/DataDog/golz4"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools/marshal"
)

// MaxBatchWireSize caps the COMPRESSED wire payload at the multi-data interceptor
// pre-Unmarshal site. Equal to libp2p's DefaultMaxMessageSize; coordinated with
// the tighter outbound network/p2p/libp2p.maxSendBuffSize (1 MiB − 64 KiB framing).
// Does NOT bound Decompress output — see MaxDecompressedBatchSize for that.
// See GHSA-74m6-4hjp-7226 / KLC-2352 (CWE-409), GHSA-w342-mj6g-v9c4.
const MaxBatchWireSize = 1 << 20 // 1 MiB

// MaxDecompressedBatchSize caps the INFLATED output of Batch.Decompress.
// Sized at 2 MiB — ~2× the worst-case legitimate marshaled batch, which is a
// single oversized-element chunk carrying one tx near core.MaxDataSize (1 MiB)
// plus tx + Batch proto framing. Distinct from MaxBatchWireSize because a
// highly-compressible payload below the wire cap can legitimately inflate
// past 1 MiB on Decompress. Bounds transient slice-header amplification to
// ~24 MB before MaxItemsPerBatch fires.
// See GHSA-74m6-4hjp-7226 / KLC-2352 (CWE-409), GHSA-w342-mj6g-v9c4.
const MaxDecompressedBatchSize = 1 << 21 // 2 MiB

// MaxItemsPerBatch caps Batch entries. A byte cap alone isn't enough: 2 MiB of
// mostly-empty proto entries still encodes ~1 M items.
// See GHSA-w342-mj6g-v9c4 and GHSA-74m6-4hjp-7226 / KLC-2353.
const MaxItemsPerBatch = 8192

// MaxHashArrayBuffSize is the tighter pre-Unmarshal cap for resolver hash-array
// requests. Sized at 512 KiB — ~1.9× the security ceiling
// (MaxItemsPerBatch × ~34 B proto framing for 32-byte hash entries ≈ 272 KiB).
// Defense-in-depth for GHSA-w342-mj6g-v9c4.
const MaxHashArrayBuffSize = 1 << 19 // 512 KiB

// New returns a new batch from given buffers
func New(buffs ...[]byte) *Batch {
	return &Batch{
		Data: buffs,
	}
}

func compressGzip(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func decompressGzip(data []byte, max int64) ([]byte, error) {
	rdata := bytes.NewReader(data)

	reader, err := gzip.NewReader(rdata)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()

	// Read at most max+1 bytes so we can detect overruns without ever allocating
	// past the cap, even when the attacker advertises a small DataSize.
	limited := io.LimitReader(reader, max+1)
	result, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(result)) > max {
		return nil, common.ErrDecompressionTooLarge
	}

	return result, nil
}

func compressLZ4(data []byte) ([]byte, error) {
	// use gzip for now
	return compressGzip(data)
	/*output := make([]byte, lz4.CompressBound(data))
	outSize, err := lz4.Compress(output, data)
	if err != nil {
		return nil, err
	}

	return output[:outSize], nil*/
}

func decompressLZ4(_ int32, data []byte, max int64) ([]byte, error) {
	return decompressGzip(data, max)
	/*output := make([]byte, dataSize)
	_, err := lz4.Uncompress(output, data)
	if err != nil {
		return nil, err
	}

	return output, nil*/
}

func (ba *Batch) Compress(m marshal.Marshalizer) error {
	if ba.IsCompressed {
		return common.ErrAlreadyCompressed
	}

	data, err := m.Marshal(ba)
	if err != nil {
		return err
	}

	var result []byte
	if ba.Algo == CType_LZ4 {
		result, err = compressLZ4(data)
		if err != nil {
			return err
		}
	} else {
		result, err = compressGzip(data)
		if err != nil {
			return err
		}
	}

	ba.Stream = make([]byte, len(result))
	copy(ba.Stream, result)
	ba.IsCompressed = true
	ba.DataSize = int32(len(data)) // #nosec G115
	ba.Data = nil
	return nil
}

func (ba *Batch) Decompress(m marshal.Marshalizer) error {
	if !ba.IsCompressed {
		return common.ErrNotCompressed
	}

	var result []byte
	var err error
	if ba.Algo == CType_LZ4 {
		result, err = decompressLZ4(ba.DataSize, ba.Stream, MaxDecompressedBatchSize)
		if err != nil {
			return err
		}
	} else {
		result, err = decompressGzip(ba.Stream, MaxDecompressedBatchSize)
		if err != nil {
			return err
		}
	}

	// Reject batches whose self-reported DataSize disagrees with the inflated payload.
	// Compress writes DataSize = len(marshaled batch); the sole production caller
	// (core/partitioning/simpleDataPacker.PackDataInChunks) guards against compressing
	// empty chunks, so legitimate traffic always satisfies len(result) == ba.DataSize.
	// The comparison runs unconditionally — gating it on DataSize > 0 would let an
	// attacker bypass this defense-in-depth check by setting DataSize=0 on the wire.
	if int64(len(result)) != int64(ba.DataSize) {
		return common.ErrDecompressedSizeMismatch
	}

	// decode
	err = m.Unmarshal(ba, result)
	if err != nil {
		return err
	}

	if len(ba.Data) > MaxItemsPerBatch {
		return common.ErrTooManyItemsInBatch
	}

	ba.Stream = nil
	ba.IsCompressed = false
	return nil
}
