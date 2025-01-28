package transaction

import "errors"

// ErrNilActiveParameters signals that the active paramaters is a nil pointer and is required for that operation
var ErrNilActiveParameters = errors.New("received nil pointer when trying to get active paramaters")

// ErrMinKLVBucketAmountNotFound signals that the minnium KLV bucket amount was not found in active parameters and is required for that operation
var ErrMinKLVBucketAmountNotFound = errors.New("failed to get minimum KLV bucket amount")
