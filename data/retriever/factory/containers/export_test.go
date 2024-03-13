package containers

import "github.com/klever-io/klever-go/core/container"

func (rc *resolversContainer) Insert(key string, value interface{}) bool {
	return rc.objects.Insert(key, value)
}

func (rc *resolversContainer) Objects() *container.MutexMap {
	return rc.objects
}
