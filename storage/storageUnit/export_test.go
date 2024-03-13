package storageUnit

import (
	"github.com/klever-io/klever-go/storage"
)

func (u *Unit) GetBlomFilter() storage.BloomFilter {
	return u.bloomFilter
}
