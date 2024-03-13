package data

import (
	"bytes"
)

// BulkSizeThreshold is the constant for the maximum size of one bulk request that is sent to the elaticsearch database
const DefaultMaxBulkSize = 4194304 // 4MB

// BufferSlice extend structure bytes.Buffer with new methods
type BufferSlice struct {
	buffSlice         []*bytes.Buffer
	bulkSizeThreshold int
	idx               int
}

// NewBufferSlice will create a new buffer
func NewBufferSlice(bulkSizeThreshold int) *BufferSlice {
	if bulkSizeThreshold == 0 {
		bulkSizeThreshold = DefaultMaxBulkSize
	}

	return &BufferSlice{
		buffSlice:         make([]*bytes.Buffer, 0),
		bulkSizeThreshold: bulkSizeThreshold,
		idx:               0,
	}
}

// PutData will put meta bytes and serializeData in buffer
func (bs *BufferSlice) PutData(meta []byte, serializedData []byte) error {
	if len(bs.buffSlice) == 0 {
		bs.buffSlice = append(bs.buffSlice, &bytes.Buffer{})
	}

	currentBuff := bs.buffSlice[bs.idx]

	if bs.aNewElementIsNeeded(meta, serializedData) {
		currentBuff = &bytes.Buffer{}
		bs.buffSlice = append(bs.buffSlice, currentBuff)
		bs.idx++
	}

	if len(serializedData) > 0 {
		serializedData = append(serializedData, "\n"...)
	}

	currentBuff.Grow(len(meta) + len(serializedData))
	_, err := currentBuff.Write(meta)
	if err != nil {
		return err
	}
	_, err = currentBuff.Write(serializedData)
	if err != nil {
		return err
	}

	return nil
}

// Buffers will return the slice of buffers
func (bs *BufferSlice) Buffers() []*bytes.Buffer {
	return bs.buffSlice
}

func (bs *BufferSlice) aNewElementIsNeeded(meta []byte, serializedData []byte) bool {
	currentBuff := bs.buffSlice[bs.idx]

	buffLenWithCurrentAcc := currentBuff.Len() + len(meta) + len(serializedData)

	return buffLenWithCurrentAcc > bs.bulkSizeThreshold && currentBuff.Len() != 0
}
