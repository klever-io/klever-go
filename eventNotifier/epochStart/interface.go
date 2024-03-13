package epochStart

import "time"

// RequestHandler defines the methods through which request to data can be made
type RequestHandler interface {
	RequestHeader(hash []byte)
	RequestHeaderByNonce(nonce uint64)
	RequestStartOfEpochBlock(epoch uint32)
	RequestInterval() time.Duration
	SetNumPeersToQuery(key string, intra int, cross int) error
	GetNumPeersToQuery(key string) (int, int, error)
	IsInterfaceNil() bool
}
