package block

import "github.com/klever-io/klever-go/data"

type blockProcessor interface {
	removeStartOfEpochBlockDataFromPools(headerHandler data.HeaderHandler) error
}
