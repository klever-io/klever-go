package state

import "errors"

// ErrInsufficientFunds signals the funds are insufficient for the move balance operation but the
// transaction fee is covered by the current balance
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrAddBalanceOverflow signals overflow adding balance
var ErrAddBalanceOverflow = errors.New("add balance overflow")

// ErrInvalidValue signals that the value is invalid
var ErrInvalidValue = errors.New("invalid value")

// ErrInvalidClaimType -
var ErrInvalidClaimType = errors.New("invalid claim type")

// ErrInvalidStakeType -
var ErrInvalidStakeType = errors.New("invalid stake type")

// ErrNotStaked -
var ErrNotStaked = errors.New("not staked")

// ErrUnstakeNotAvailable -
var ErrUnstakeNotAvailable = errors.New("unstake not available")

// ErrUndelegatingNotAvailable -
var ErrUndelegatingNotAvailable = errors.New("undelegating not available")

// ErrClaimNotAvailable -
var ErrClaimNotAvailable = errors.New("claim not available")

// ErrWithdrawNotAvailable -
var ErrWithdrawNotAvailable = errors.New("withdraw not available")

// ErrInvalidPermissionID -
var ErrInvalidPermissionID = errors.New("invalid permission id")

// ErrInconsistentStakingData -
var ErrInconsistentStakingData = errors.New("inconsistent claim data")

// ErrNilTrackableDataTrie signals that a nil trackable data trie has been provided
var ErrNilTrackableDataTrie = errors.New("nil trackable data trie")

// ErrInvalidNonce signals that invalid nonce for kda
var ErrInvalidNonce = errors.New("invalid nonce for kda")
