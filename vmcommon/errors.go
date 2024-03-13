package vmcommon

import "errors"

// ErrSubtractionOverflow signals that uint64 subtraction overflowed
var ErrSubtractionOverflow = errors.New("uint64 subtraction overflowed")

// ErrInvalidVMType signals that invalid vm type was provided
var ErrInvalidVMType = errors.New("invalid VM type")

// ErrNilTransferIndexer signals that the provided transfer indexer is nil
var ErrNilTransferIndexer = errors.New("nil NextOutputTransferIndexProvider")

// ErrTransfersNotIndexed signals that transfers were found unindexed
var ErrTransfersNotIndexed = errors.New("unindexed transfers found")
