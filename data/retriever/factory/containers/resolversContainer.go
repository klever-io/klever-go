package containers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/container"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
)

var _ retriever.ResolversContainer = (*resolversContainer)(nil)

// resolversContainer is a resolvers holder organized by type
type resolversContainer struct {
	objects *container.MutexMap
}

// NewResolversContainer will create a new instance of a container
func NewResolversContainer() *resolversContainer {
	return &resolversContainer{
		objects: container.NewMutexMap(),
	}
}

// Get returns the object stored at a certain key.
// Returns an error if the element does not exist
func (rc *resolversContainer) Get(key string) (retriever.Resolver, error) {
	value, ok := rc.objects.Get(key)
	if !ok {
		return nil, fmt.Errorf("%w in resolvers container for key %v", common.ErrInvalidContainerKey, key)
	}

	resolver, ok := value.(retriever.Resolver)
	if !ok {
		return nil, common.ErrWrongTypeInContainer
	}

	return resolver, nil
}

// Add will add an object at a given key. Returns
// an error if the element already exists
func (rc *resolversContainer) Add(key string, resolver retriever.Resolver) error {
	if check.IfNil(resolver) {
		return common.ErrNilContainerElement
	}

	ok := rc.objects.Insert(key, resolver)
	if !ok {
		return common.ErrContainerKeyAlreadyExists
	}

	return nil
}

// AddMultiple will add objects with given keys. Returns
// an error if one element already exists, lengths mismatch or an interceptor is nil
func (rc *resolversContainer) AddMultiple(keys []string, resolvers []retriever.Resolver) error {
	if len(keys) != len(resolvers) {
		return common.ErrLenMismatch
	}

	for idx, key := range keys {
		err := rc.Add(key, resolvers[idx])
		if err != nil {
			return err
		}
	}

	return nil
}

// Replace will add (or replace if it already exists) an object at a given key
func (rc *resolversContainer) Replace(key string, resolver retriever.Resolver) error {
	if check.IfNil(resolver) {
		return common.ErrNilContainerElement
	}

	rc.objects.Set(key, resolver)
	return nil
}

// Remove will remove an object at a given key
func (rc *resolversContainer) Remove(key string) {
	rc.objects.Remove(key)
}

// Len returns the length of the added objects
func (rc *resolversContainer) Len() int {
	return rc.objects.Len()
}

// ResolverKeys will return the contained resolvers keys in a concatenated string
func (rc *resolversContainer) ResolverKeys() string {
	keys := rc.objects.Keys()

	stringKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		key, ok := k.(string)
		if !ok {
			continue
		}

		stringKeys = append(stringKeys, key)
	}

	sort.Slice(stringKeys, func(i, j int) bool {
		return strings.Compare(stringKeys[i], stringKeys[j]) < 0
	})

	return strings.Join(stringKeys, ", ")
}

// Iterate will call the provided handler for each and every key-value pair
func (rc *resolversContainer) Iterate(handler func(key string, resolver retriever.Resolver) bool) {
	if handler == nil {
		return
	}

	for _, keyVal := range rc.objects.Keys() {
		key, ok := keyVal.(string)
		if !ok {
			continue
		}

		val, ok := rc.objects.Get(key)
		if !ok {
			continue
		}

		resolver, ok := val.(retriever.Resolver)
		if !ok {
			continue
		}

		shouldContinue := handler(key, resolver)
		if !shouldContinue {
			return
		}
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (rc *resolversContainer) IsInterfaceNil() bool {
	return rc == nil
}
