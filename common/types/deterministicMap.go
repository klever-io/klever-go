package types

import (
	"cmp"
	"sort"
)

// deterministicMap is a map that iterates over its keys in a deterministic order.
// this map is not intended to be a fancy way of storing data or ordering it. It is
// intended to be used in blockchain maps that impacts storage and execution order.
//
// This only exists to fill the gap of Go's unordered maps.
// This is not a general purpose map and should not be used as such.
//
// The deterministic iteration was implemented in a struct to make it more reusable as possible.
// others solutions of deterministic iteration are not reusable and are implemented in the code,
// making it harder to maintain.
type deterministicMap[K cmp.Ordered, V any] struct {
	// data is the underlying map.
	data       map[K]V
	sortedKeys []K // Store keys to avoid re-sorting every time
}

// NewDeterministicMap creates a new deterministicMap.
func NewDeterministicMap[K cmp.Ordered, V any](data map[K]V) deterministicMap[K, V] {
	return deterministicMap[K, V]{
		data: data,
	}.fillSortedKeys()
}

// fillSortedKeys fills the sortedKeys slice with the keys of the map.
func (dm deterministicMap[K, V]) fillSortedKeys() deterministicMap[K, V] {
	// manually extract dict keys (since Go doesn't have a built-in way to do this)
	keys := make([]K, 0, len(dm.data))
	for key := range dm.data {
		keys = append(keys, key)
	}
	// sort the keys
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	dm.sortedKeys = keys
	return dm
}

// Each iterates over the map in a deterministic order.
func (dm deterministicMap[K, V]) Each(f func(key K, value V) error) error {
	for _, key := range dm.sortedKeys {
		err := f(key, dm.data[key])
		if err != nil {
			return err
		}
	}
	return nil
}
