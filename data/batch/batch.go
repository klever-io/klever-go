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

func decompressGzip(data []byte) ([]byte, error) {
	rdata := bytes.NewReader(data)

	reader, err := gzip.NewReader(rdata)
	if err != nil {
		return nil, err
	}

	result, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if err := reader.Close(); err != nil {
		return nil, err
	}

	return result, nil
}

func compressLZ4(data []byte) ([]byte, error) {
	return compressGzip(data)
}

func decompressLZ4(dataSize int32, data []byte) ([]byte, error) {
	return decompressGzip(data)
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
	ba.DataSize = int32(len(data))
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
		result, err = decompressLZ4(ba.DataSize, ba.Stream)
		if err != nil {
			return err
		}
	} else {
		result, err = decompressGzip(ba.Stream)
		if err != nil {
			return err
		}
	}

	// decode
	err = m.Unmarshal(ba, result)
	if err != nil {
		return err
	}

	ba.Stream = nil
	ba.IsCompressed = false
	return nil
}
