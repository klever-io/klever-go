package preprocess

type processResults struct {
	txHashes [][]byte
	size     int64
}

func NewProcessResults(txHashes [][]byte, size int64) *processResults {
	return &processResults{
		txHashes: txHashes,
		size:     size,
	}
}

// Hashes returns the hashes of the processed results
func (pr *processResults) Hashes() [][]byte {
	return pr.txHashes
}

// Length returns the length of the processed results
func (pr *processResults) Length() int {
	return len(pr.txHashes)
}

// Size returns the size of the processed results
func (pr *processResults) Size() int64 {
	return pr.size
}

// IsInterfaceNil returns true if there is no value under the interface
func (pr *processResults) IsInterfaceNil() bool {
	return pr == nil
}
