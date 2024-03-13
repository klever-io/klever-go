package mock

import (
	"time"

	"github.com/klever-io/klever-go/core"
)

// RequestHandlerStub -
type RequestHandlerStub struct {
	RequestHeaderCalled             func(hash []byte)
	RequestHeaderByNonceCalled      func(nonce uint64)
	RequestTransactionHandlerCalled func(txHashes [][]byte)
	RequestScrHandlerCalled         func(txHashes [][]byte)
	RequestRewardTxHandlerCalled    func(txHashes [][]byte)
	RequestBlockHandlerCalled       func(blockHash []byte)
	RequestBlocksHandlerCalled      func(blocksHashes [][]byte)
	RequestTrieNodesCalled          func(hashes [][]byte, topic string)
	RequestStartOfEpochBlockCalled  func(epoch uint32)
	SetNumPeersToQueryCalled        func(key string, intra int, cross int) error
	GetNumPeersToQueryCalled        func(key string) (int, int, error)
}

// RequestInterval -
func (rhs *RequestHandlerStub) RequestInterval() time.Duration {
	return time.Second
}

// RequestStartOfEpochBlock -
func (rhs *RequestHandlerStub) RequestStartOfEpochBlock(epoch uint32) {
	if rhs.RequestStartOfEpochBlockCalled == nil {
		return
	}
	rhs.RequestStartOfEpochBlockCalled(epoch)
}

// SetEpoch -
func (rhs *RequestHandlerStub) SetEpoch(_ uint32) {
}

// RequestHeader -
func (rhs *RequestHandlerStub) RequestHeader(hash []byte) {
	if rhs.RequestHeaderCalled == nil {
		return
	}
	rhs.RequestHeaderCalled(hash)
}

// RequestHeaderByNonce -
func (rhs *RequestHandlerStub) RequestHeaderByNonce(nonce uint64) {
	if rhs.RequestHeaderByNonceCalled == nil {
		return
	}
	rhs.RequestHeaderByNonceCalled(nonce)
}

// RequestTransaction -
func (rhs *RequestHandlerStub) RequestTransaction(txHashes [][]byte) {
	if rhs.RequestTransactionHandlerCalled == nil {
		return
	}
	rhs.RequestTransactionHandlerCalled(txHashes)
}

// RequestUnsignedTransactions -
func (rhs *RequestHandlerStub) RequestUnsignedTransactions(txHashes [][]byte) {
	if rhs.RequestScrHandlerCalled == nil {
		return
	}
	rhs.RequestScrHandlerCalled(txHashes)
}

func (rhs *RequestHandlerStub) RequestTransactionTo(txHashes [][]byte, peer core.PeerID) {
	if rhs.RequestScrHandlerCalled == nil {
		return
	}
	rhs.RequestScrHandlerCalled(txHashes)
}

// RequestTrieNodes -
func (rhs *RequestHandlerStub) RequestTrieNodes(hashes [][]byte, topic string) {
	if rhs.RequestTrieNodesCalled == nil {
		return
	}
	rhs.RequestTrieNodesCalled(hashes, topic)
}

// SetNumPeersToQuery -
func (rhs *RequestHandlerStub) SetNumPeersToQuery(key string, intra int, cross int) error {
	if rhs.SetNumPeersToQueryCalled != nil {
		return rhs.SetNumPeersToQueryCalled(key, intra, cross)
	}

	return nil
}

// GetNumPeersToQuery -
func (rhs *RequestHandlerStub) GetNumPeersToQuery(key string) (int, int, error) {
	if rhs.GetNumPeersToQueryCalled != nil {
		return rhs.GetNumPeersToQueryCalled(key)
	}

	return 2, 2, nil
}

// ResetRequests -
func (rhs *RequestHandlerStub) ResetRequests() {
}

// IsInterfaceNil returns true if there is no value under the interface
func (rhs *RequestHandlerStub) IsInterfaceNil() bool {
	return rhs == nil
}
