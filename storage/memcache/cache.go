package memcache

import (
	"sync"
	"time"

	cacher "github.com/patrickmn/go-cache"
)

type MemoryCacher struct {
	mut   sync.RWMutex
	cache *cacher.Cache
	exp   time.Duration
}

func NewCache(expiration time.Duration, cleanUpInterval time.Duration) (*MemoryCacher, error) {
	c := cacher.New(expiration, cleanUpInterval)

	return &MemoryCacher{
		mut:   sync.RWMutex{},
		cache: c,
		exp:   expiration,
	}, nil
}

func (mc *MemoryCacher) Set(key string, data interface{}) {
	mc.mut.Lock()
	defer mc.mut.Unlock()

	mc.cache.Set(key, data, cacher.DefaultExpiration)
}

func (mc *MemoryCacher) Get(key string) (interface{}, bool) {
	mc.mut.Lock()
	defer mc.mut.Unlock()

	return mc.cache.Get(key)
}

func (mc *MemoryCacher) IsInterfaceNil() bool {
	return mc == nil
}
