package tools

import "errors"

// ErrNilHasher is raised when a valid hasher is expected but used nil
var ErrNilHasher = errors.New("hasher is nil")

// ErrNilFile signals that a nil file has been provided
var ErrNilFile = errors.New("nil file provided")

// ErrEmptyFile signals that a empty file has been provided
var ErrEmptyFile = errors.New("empty file provided")

// ErrInvalidIndex signals that an invalid private key index has been provided
var ErrInvalidIndex = errors.New("invalid private key index")

// ErrPemFileIsInvalid signals that a pem file is invalid
var ErrPemFileIsInvalid = errors.New("pem file is invalid")

// ErrNilPemBLock signals that the pem block is nil
var ErrNilPemBLock = errors.New("nil pem block")

// ErrAdditionOverflow signals that uint64 addition overflowed
var ErrAdditionOverflow = errors.New("uint64 addition overflowed")

// ErrSubtractionOverflow signals that uint64 subtraction overflowed
var ErrSubtractionOverflow = errors.New("uint64 subtraction overflowed")

// ErrNilMarshalizer signals that a nil marshalizer has been provided
var ErrNilMarshalizer = errors.New("nil marshalizer")
