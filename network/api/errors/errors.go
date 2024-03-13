package errors

import "errors"

// ErrNilAppContext signals that no context was passed to the routing system
var ErrNilAppContext = errors.New("nil app context")

// ErrInvalidAppContext signals an invalid context passed to the routing system
var ErrInvalidAppContext = errors.New("invalid app context")

// ErrInvalidJSONRequest signals an error in json request formatting
var ErrInvalidJSONRequest = errors.New("invalid json request")

// ErrTooManyRequests signals that too many requests were simultaneously received
var ErrTooManyRequests = errors.New("too many requests")

// ErrCouldNotGetAccount signals that a requested account could not be retrieved
var ErrCouldNotGetAccount = errors.New("could not get requested account")

// ErrCouldNotGetAsset signals that a requested asset could not be retrieved
var ErrCouldNotGetAsset = errors.New("could not get requested asset")

// ErrCouldNotGetMarketplace signals that a requested marketplace could not be retrieved
var ErrCouldNotGetMarketplace = errors.New("could not get requested marketplace")

// ErrCouldNotGetAssets signals that a requested asset could not be retrieved
var ErrCouldNotGetAssets = errors.New("could not get assets")

// ErrGetBalance signals an error in getting the balance for an account
var ErrGetBalance = errors.New("get balance error")

// ErrGetAvailableClaim signals an error in getting the rewards an account asset
var ErrGetAvailableClaim = errors.New("could not get rewards for requested account asset")

// ErrGetAvailableClaim signals an error in getting the rewards an account asset list
var ErrGetAvailableClaimList = errors.New("could not get rewards for requested account asset list")

// ErrEmptyAddress signals an empty address was provided
var ErrEmptyAddress = errors.New("address is empty")

// ErrEmptyAssetId signals an empty assetId was provided
var ErrEmptyAssetId = errors.New("assetId is empty")

// ErrValidation signals an error in validation
var ErrValidation = errors.New("validation error")

// ErrQueryError signals a general query error
var ErrQueryError = errors.New("query error")

// ErrGetPidInfo signals that an error occurred while getting peer ID info
var ErrGetPidInfo = errors.New("error getting peer id info")

// ErrGetProposalParameters signals that an error occurred while getting proposal parameters
var ErrGetProposalParameters = errors.New("error getting proposal parameters")

// ErrInvalidBlockNonce signals an invalid block nonce was provided
var ErrInvalidBlockNonce = errors.New("invalid block nonce")

// ErrInvalidQueryParameter signals and invalid query parameter was provided
var ErrInvalidQueryParameter = errors.New("invalid query parameter")

// ErrGetBlock signals an error happening when trying to fetch a block
var ErrGetBlock = errors.New("getting block failed")

// ErrValidationEmptyBlockHash signals an empty block hash was provided
var ErrValidationEmptyBlockHash = errors.New("block hash is empty")

// ErrValidationEmptyTxHash signals an empty tx hash was provided
var ErrValidationEmptyTxHash = errors.New("TxHash is empty")

// ErrGetTransaction signals an error happening when trying to fetch a transaction
var ErrGetTransaction = errors.New("getting transaction failed")
