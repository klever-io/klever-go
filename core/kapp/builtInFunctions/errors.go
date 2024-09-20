package builtInFunctions

import (
	"errors"
)

// ErrNilAccountsAdapter defines the error when trying to use a nil AccountsAddapter
var ErrNilAccountsAdapter = errors.New("nil AccountsAdapter")

// ErrInsufficientFunds signals the funds are insufficient for the move balance operation but the
// transaction fee is covered by the current balance
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrNilValue signals the value is nil
var ErrNilValue = errors.New("nil value")

// ErrNilMarshalizer signals that an operation has been attempted to or with a nil Marshalizer implementation
var ErrNilMarshalizer = errors.New("nil Marshalizer")

// ErrInvalidRcvAddr signals that an invalid receiver address was provided
var ErrInvalidRcvAddr = errors.New("invalid receiver address")

// ErrNegativeValue signals that a negative value has been detected and it is not allowed
var ErrNegativeValue = errors.New("negative value")

// ErrWrongTypeAssertion signals that an type assertion failed
var ErrWrongTypeAssertion = errors.New("wrong type assertion")

// ErrInvalidArguments signals that invalid arguments were given to process built-in function
var ErrInvalidArguments = errors.New("invalid arguments to process built-in function")

// ErrOperationNotPermitted signals that operation is not permitted
var ErrOperationNotPermitted = errors.New("operation in account not permitted")

// ErrInvalidAddressLength signals that address length is invalid
var ErrInvalidAddressLength = errors.New("invalid address length")

// ErrNilVmInput signals that provided vm input is nil
var ErrNilVmInput = errors.New("nil vm input")

// ErrNilDnsAddresses signals that nil dns addresses map was provided
var ErrNilDnsAddresses = errors.New("nil dns addresses map")

// ErrCallerIsNotTheDNSAddress signals that called address is not the DNS address
var ErrCallerIsNotTheDNSAddress = errors.New("not a dns address")

// ErrUserNameChangeIsDisabled signals the user name change is not allowed
var ErrUserNameChangeIsDisabled = errors.New("user name change is disabled")

// ErrBuiltInFunctionCalledWithValue signals that builtin function was called with value that is not allowed
var ErrBuiltInFunctionCalledWithValue = errors.New("built in function called with tx value is not allowed")

// ErrAccountNotPayable will be sent when trying to send tokens to a non-payableCheck account
var ErrAccountNotPayable = errors.New("sending value to non payable contract")

// ErrNilUserAccount signals that nil user account was provided
var ErrNilUserAccount = errors.New("nil user account")

// ErrAddressIsNotKDASystemSC signals that destination is not a system sc address
var ErrAddressIsNotKDASystemSC = errors.New("destination is not system sc address")

// ErrOnlySystemAccountAccepted signals that only system account is accepted
var ErrOnlySystemAccountAccepted = errors.New("only system account is accepted")

// ErrNilGlobalSettingsHandler signals that nil pause handler has been provided
var ErrNilGlobalSettingsHandler = errors.New("nil pause handler")

// ErrNilRolesHandler signals that nil roles handler has been provided
var ErrNilRolesHandler = errors.New("nil roles handler")

// ErrKDATokenIsPaused signals that kda token is paused
var ErrKDATokenIsPaused = errors.New("kda token is paused")

// ErrKDAIsFrozenForAccount signals that account is frozen for given kda token
var ErrKDAIsFrozenForAccount = errors.New("account is frozen for this kda token")

// ErrCannotWipeAccountNotFrozen signals that account isn't frozen so the wipe is not possible
var ErrCannotWipeAccountNotFrozen = errors.New("cannot wipe because the account is not frozen for this kda token")

// ErrNilPayableHandler signals that nil payableHandler was provided
var ErrNilPayableHandler = errors.New("nil payableHandler was provided")

// ErrActionNotAllowed signals that action is not allowed
var ErrActionNotAllowed = errors.New("action is not allowed")

// ErrOnlyFungibleTokensHaveBalanceTransfer signals that only fungible tokens have balance transfer
var ErrOnlyFungibleTokensHaveBalanceTransfer = errors.New("only fungible tokens have balance transfer")

// ErrNFTTokenDoesNotExist signals that NFT token does not exist
var ErrNFTTokenDoesNotExist = errors.New("NFT token does not exist")

// ErrNFTDoesNotHaveMetadata signals that NFT does not have metadata
var ErrNFTDoesNotHaveMetadata = errors.New("NFT does not have metadata")

// ErrInvalidNFTQuantity signals that invalid NFT quantity was provided
var ErrInvalidNFTQuantity = errors.New("invalid NFT quantity")

// ErrNewNFTDataOnSenderAddress signals that a new NFT data was found on the sender address
var ErrNewNFTDataOnSenderAddress = errors.New("new NFT data on sender")

// ErrNilContainerElement signals when trying to add a nil element in the container
var ErrNilContainerElement = errors.New("element cannot be nil")

// ErrInvalidContainerKey signals that an element does not exist in the container's map
var ErrInvalidContainerKey = errors.New("element does not exist in container")

// ErrContainerKeyAlreadyExists signals that an element was already set in the container's map
var ErrContainerKeyAlreadyExists = errors.New("provided key already exists in container")

// ErrWrongTypeInContainer signals that a wrong type of object was found in container
var ErrWrongTypeInContainer = errors.New("wrong type of object inside container")

// ErrEmptyFunctionName signals that an empty function name has been provided
var ErrEmptyFunctionName = errors.New("empty function name")

// ErrInsufficientQuantityKDA signals the funds are insufficient for the KDA transfer
var ErrInsufficientQuantityKDA = errors.New("insufficient quantity")

// ErrNilTransactionHandler signals that a nil transaction handler has been provided
var ErrNilTransactionHandler = errors.New("nil transaction handler")

// ErrAddressIsNotAllowed signals that sender is not allowed to do the action
var ErrAddressIsNotAllowed = errors.New("address is not allowed to do the action")

// ErrInvalidNumOfArgs signals that the number of arguments is invalid
var ErrInvalidNumOfArgs = errors.New("invalid number of arguments")

// ErrInvalidNonce signals that invalid nonce for kda
var ErrInvalidNonce = errors.New("invalid nonce for kda")

// ErrTokenHasValidMetadata signals that token has a valid metadata
var ErrTokenHasValidMetadata = errors.New("token has valid metadata")

// ErrInvalidTokenID signals that invalid tokenID was provided
var ErrInvalidTokenID = errors.New("invalid tokenID")

// ErrNilKDAData signals that KDA data does not exist
var ErrNilKDAData = errors.New("nil kda data")

// ErrInvalidMetadata signals that invalid metadata was provided
var ErrInvalidMetadata = errors.New("invalid metadata")

// ErrInvalidLiquidityForKDA signals that liquidity is invalid for KDA
var ErrInvalidLiquidityForKDA = errors.New("invalid liquidity for KDA")

// ErrTooManyTransferAddresses signals that too many transfer address roles has been added
var ErrTooManyTransferAddresses = errors.New("too many transfer addresses")

// ErrInvalidMaxNumAddresses signals that there is an invalid max number of addresses
var ErrInvalidMaxNumAddresses = errors.New("invalid max number of addresses")

// ErrNilEnableEpochsHandler signals that a nil enable epochs handler was provided
var ErrNilEnableEpochsHandler = errors.New("nil enable epochs handler")

// ErrNilActiveHandler signals that a nil active handler has been provided
var ErrNilActiveHandler = errors.New("nil active handler")

// ErrChangeOwnerAddressFailed signals that change owner address failed
var ErrChangeOwnerAddressFailed = errors.New("change owner address failed")
