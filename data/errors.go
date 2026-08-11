package data

import (
	"errors"
)

// ErrNilCacher signals that a nil cache has been provided
var ErrNilCacher = errors.New("nil cacher")

// ErrContextClosing signals that the parent context requested the closing of its children
var ErrContextClosing = errors.New("context closing")

// ErrNilTrieIteratorChannels signals that a nil set of trie iterator channels has been provided
var ErrNilTrieIteratorChannels = errors.New("nil trie iterator channels")

// ErrNilMarshalizer is raised when the NewTrie() function is called, but a marshalizer isn't provided
var ErrNilMarshalizer = errors.New("no marshalizer provided")

// ErrNilDatabase is raised when a database operation is called, but no database is provided
var ErrNilDatabase = errors.New("no database provided")

// ErrInvalidCacheSize is raised when the given size for the cache is invalid
var ErrInvalidCacheSize = errors.New("cache size is invalid")

// ErrInvalidValue signals that an invalid value has been provided such as NaN to an integer field
var ErrInvalidValue = errors.New("invalid value")

// ErrNilThrottler signals that nil throttler has been provided
var ErrNilThrottler = errors.New("nil throttler")

// ErrTimeIsOut signals that time is out
var ErrTimeIsOut = errors.New("time is out")

// ErrLeafSizeTooBig signals that the value size of the leaf is too big
var ErrLeafSizeTooBig = errors.New("leaf size too big")

// ErrNilValue signals the value is nil
var ErrNilValue = errors.New("nil value")

// ErrNilSignature signals that a operation has been attempted with a nil signature
var ErrNilSignature = errors.New("nil signature")

// ErrNegativeValue signals that a negative value has been detected and it is not allowed
var ErrNegativeValue = errors.New("negative value")

// ErrNilTxHash signals that an operation has been attempted with a nil hash
var ErrNilTxHash = errors.New("nil transaction hash")
