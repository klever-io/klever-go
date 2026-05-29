package partitioning

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ retriever.DataPacker = (*SimpleDataPacker)(nil)

var log = logger.GetOrCreate("SimpleDataPacker")

// SimpleDataPacker can split a large slice of byte slices in chunks <= maxPacketSize
// If one element still exceeds maxPacketSize, it will be returned alone
// It does the marshaling of the resulted (smaller) slice of byte slices
// This is a simpler version of a data packer that does not marshall in a repetitive manner currentChunk slice
// as the SizeDataPacker does. This limitation is lighter in terms of CPU cycles and memory used but is not as precise
// as SizeDataPacker.
type SimpleDataPacker struct {
	marshalizer marshal.Marshalizer
}

// NewSimpleDataPacker creates a new SizeDataPacker instance
func NewSimpleDataPacker(marshalizer marshal.Marshalizer) (*SimpleDataPacker, error) {
	if marshalizer == nil || marshalizer.IsInterfaceNil() {
		return nil, common.ErrNilMarshalizer
	}

	return &SimpleDataPacker{
		marshalizer: marshalizer,
	}, nil
}

// PackDataInChunks packs the provided data into smaller chunks
// limit is expressed in bytes
func (sdp *SimpleDataPacker) PackDataInChunks(data [][]byte, limit int) ([][]byte, error) {
	if limit < minimumMaxPacketSizeInBytes {
		return nil, common.ErrInvalidValue
	}
	if data == nil {
		return nil, common.ErrNilInputData
	}

	totalSize := 0
	compressedSize := 0

	returningBuff := make([][]byte, 0)

	currentChunk := make([][]byte, 0)
	lenChunk := 0
	for _, element := range data {
		isBuffToLarge := lenChunk+len(element) >= limit
		chunkNotEmpty := len(currentChunk) > 0
		if isBuffToLarge && chunkNotEmpty {
			ba := &batch.Batch{Data: currentChunk}
			err := ba.Compress(sdp.marshalizer)
			if err != nil {
				return nil, err
			}
			marshaledChunk, err := sdp.marshalizer.Marshal(ba)
			if err != nil {
				return nil, err
			}
			compressedSize += len(marshaledChunk)
			returningBuff = append(returningBuff, marshaledChunk)
			currentChunk = make([][]byte, 0)
			totalSize += lenChunk
			lenChunk = 0
		}

		currentChunk = append(currentChunk, element)
		lenChunk += len(element)
	}

	if len(currentChunk) > 0 {
		totalSize += lenChunk
		ba := &batch.Batch{Data: currentChunk}
		err := ba.Compress(sdp.marshalizer)
		if err != nil {
			return nil, err
		}

		marshaledElements, err := sdp.marshalizer.Marshal(ba)
		if err != nil {
			return nil, err
		}
		compressedSize += len(marshaledElements)
		returningBuff = append(returningBuff, marshaledElements)
	}

	log.Trace("PackDataInChunks", "totalSize", totalSize, "compressed", compressedSize)
	return returningBuff, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (sdp *SimpleDataPacker) IsInterfaceNil() bool {
	return sdp == nil
}
