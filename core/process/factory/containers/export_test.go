package containers

import (
	"github.com/klever-io/klever-go/core/container"
)

func (ic *interceptorsContainer) Insert(key string, value interface{}) bool {
	return ic.objects.Insert(key, value)
}

func (ic *interceptorsContainer) Objects() *container.MutexMap {
	return ic.objects
}
