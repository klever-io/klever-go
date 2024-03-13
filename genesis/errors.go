package genesis

import "errors"

// ErrInvalidEntireSupply signals that the provided entire supply is invalid
var ErrInvalidEntireSupply = errors.New("invalid entire supply")

// ErrEntireSupplyMismatch signals that the provided entire supply mismatches the computed one
var ErrEntireSupplyMismatch = errors.New("entire supply mismatch")

// ErrEmptyAddress signals that an empty address was found in genesis file
var ErrEmptyAddress = errors.New("empty address")

// ErrInvalidAddress signals that an invalid address was found
var ErrInvalidAddress = errors.New("invalid address")

// ErrInvalidPubKey signals that an invalid public key has been provided
var ErrInvalidPubKey = errors.New("invalid public key")

// ErrEmptyDelegationAddress signals that the delegation address is empty
var ErrEmptyDelegationAddress = errors.New("empty delegation address")

// ErrInvalidDelegationAddress signals that the delegation address is invalid
var ErrInvalidDelegationAddress = errors.New("invalid delegation address")

// ErrInvalidSupply signals that the supply field is invalid
var ErrInvalidSupply = errors.New("invalid supply")

// ErrInvalidBalance signals that the balance field is invalid
var ErrInvalidBalance = errors.New("invalid balance")

// ErrInvalidStakingBalance signals that the staking balance field is invalid
var ErrInvalidStakingBalance = errors.New("invalid staking balance")

// ErrInvalidDelegationValue signals that the delegation value field is invalid
var ErrInvalidDelegationValue = errors.New("invalid delegation value")

// ErrSupplyMismatch signals that the supply value provided is not valid when summing the other fields
var ErrSupplyMismatch = errors.New("supply value mismatch")

// ErrDuplicateAddress signals that the same address was found more than one time
var ErrDuplicateAddress = errors.New("duplicate address")

// ErrNilPubkeyConverter signals that the provided public key converter is nil
var ErrNilPubkeyConverter = errors.New("nil pubkey converter")

// ErrNilAccountsParser signals that the provided accounts parser is nil
var ErrNilAccountsParser = errors.New("nil accounts parser")

// ErrStakingValueIsNotEnough signals that the staking value provided is not enough for provided node(s)
var ErrStakingValueIsNotEnough = errors.New("staking value is not enough")

// ErrDelegationValueIsNotEnough signals that the delegation value provided is not enough for provided node(s)
var ErrDelegationValueIsNotEnough = errors.New("delegation value is not enough")

// ErrNodeNotStaked signals that no one staked for the provided node
var ErrNodeNotStaked = errors.New("for the provided node, no one staked")

// ErrInvalidInitialNodePrice signals that the provided initial node price is invalid
var ErrInvalidInitialNodePrice = errors.New("invalid initial node price")

// ErrNilDelegationHandler signals that a nil delegation handler has been used
var ErrNilDelegationHandler = errors.New("nil delegation handler")

// ErrWrongTypeAssertion signals that a wrong type assertion occurred
var ErrWrongTypeAssertion = errors.New("wrong type assertion")

// ErrNilNodesSetup signals that a nil nodes setup handler has been provided
var ErrNilNodesSetup = errors.New("nil nodes setup")

// ErrNilTrieStorageManager signals that a nil trie storage manager has been provided
var ErrNilTrieStorageManager = errors.New("nil trie storage manager")

// ErrSignatureMismatch signals a signature mismatch occurred
var ErrSignatureMismatch = errors.New("signature mismatch")

// ErrEmptyPubKey signals that empty public key has been provided
var ErrEmptyPubKey = errors.New("empty public key")

// ErrNilKeyGenerator signals that nil key generator has been provided
var ErrNilKeyGenerator = errors.New("nil key generator")

// ErrPermissionThreshold signals invalid permission threshold
var ErrPermissionThreshold = errors.New("invalid permission threshold")

// ErrInvalidSignerAddress signals that the signer address is invalid
var ErrInvalidSignerAddress = errors.New("invalid signer address")
