package peer

import "github.com/klever-io/klever-go/data/retriever"

// DataPool indicates the main functionality needed in order to fetch the required blocks from the pool
type DataPool interface {
	Headers() retriever.HeadersPool
	IsInterfaceNil() bool
}
