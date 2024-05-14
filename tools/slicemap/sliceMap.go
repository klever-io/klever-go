package slicemap

import (
	"crypto/sha1" // #nosec G505: Blocklisted import sha1 hash is what is desired in this case
	"fmt"
)

// SliceMap represents a byte slice into map of string
type SliceMap struct {
	m map[string][]byte
}

// Key retuns key of a byteslice
func (s *SliceMap) Key(buf []byte) string {
	// #nosec G401: Blocklisted call to sha1 hash is what is desired in this case
	h := sha1.New()
	h.Write(buf)
	sum := h.Sum(nil)
	return fmt.Sprintf("%x", sum)
}

// Value retrives value of key
func (s *SliceMap) Value(key []byte) (value []byte, ok bool) {
	value, ok = s.m[s.Key(key)]
	return
}

// Add a new key value into map
func (s *SliceMap) Add(key, value []byte) {
	if s.m == nil {
		s.m = make(map[string][]byte)
	}
	s.m[s.Key(key)] = value
}

// Clear delete all keys from map
func (s *SliceMap) Clear() {
	if s.m != nil {
		for k := range s.m {
			delete(s.m, k)
		}
	}
}

// CopyFrom clone slice
func (s *SliceMap) CopyFrom(f *SliceMap) {
	if s.m == nil {
		s.m = make(map[string][]byte)
	}
	for k, v := range f.m {
		s.m[k] = v
	}
}

// Index returns counter postion of key
func (s *SliceMap) Index(key []byte) (int, error) {
	i := 0
	q := s.Key(key)
	for k := range s.m {
		if k == q {
			return i, nil
		}
		i++
	}
	return -1, fmt.Errorf("key not found")
}

// Size retuns length of list
func (s *SliceMap) Size() int {
	return len(s.m)
}

// CheckIndex compare key postion
func (s *SliceMap) CheckIndex(index int, key []byte) bool {
	if len(s.m) < index {
		return false
	}
	i := 0
	q := s.Key(key)
	for k := range s.m {
		if k == q {
			return i == index
		}
		i++
	}
	return false
}

// Clone create a copy of current map
func (s *SliceMap) Clone() *SliceMap {
	clone := &SliceMap{}
	clone.CopyFrom(s)
	return clone
}

// Has check if map has key
func (s *SliceMap) Has(k []byte) bool {
	_, ok := s.Value(k)
	return ok
}
