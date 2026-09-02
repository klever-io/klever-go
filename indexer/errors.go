package indexer

import (
	"errors"
)

var ErrorParseResponse = "error parsing response"

var ErrorCouldNotCloseBody = "could not close body"

// ErrBackOff signals that an error was received from the server
var ErrBackOff = errors.New("back off something is not working well")

// ErrNoElasticUrlProvided -
var ErrNoElasticUrlProvided = errors.New("no elastic url provided")

// ErrCouldNotCreatePolicy -
var ErrCouldNotCreatePolicy = errors.New("could not create policy")

// ErrCouldNotUpdateMapping signals that a mapping property could not be added to an index
var ErrCouldNotUpdateMapping = errors.New("could not update mapping")

// ErrCouldNotCreateTemplate signals that an index template could not be written
var ErrCouldNotCreateTemplate = errors.New("could not create template")

// ErrCouldNotCreateIndex signals that an index could not be created
var ErrCouldNotCreateIndex = errors.New("could not create index")

// ErrCouldNotCreateAlias signals that an alias could not be created
var ErrCouldNotCreateAlias = errors.New("could not create alias")

// ErrNilPubkeyConverter signals that an operation has been attempted to or with a nil public key converter implementation
var ErrNilPubkeyConverter = errors.New("nil pubkey converter")

// ErrNilDataDispatcher signals that an operation has been attempted to or with a nil data dispatcher implementation
var ErrNilDataDispatcher = errors.New("nil data dispatcher")

// ErrNilElasticProcessor signals that an operation has been attempted to or with a nil elastic processor implementation
var ErrNilElasticProcessor = errors.New("nil elastic processor")

// ErrNilDatabaseClient signals that an operation has been attempted to or with a nil database client implementation
var ErrNilDatabaseClient = errors.New("nil database client")

// ErrNilOptions signals that structure that contains indexer options is nil
var ErrNilOptions = errors.New("nil options")

// ErrNegativeCacheSize signals that a invalid cache size has been provided
var ErrNegativeCacheSize = errors.New("negative cache size")

// ErrNilAccountsDB signals that a nil accounts database has been provided
var ErrNilAccountsDB = errors.New("nil accounts db")

// ErrNilKAppsDB signals that a nil kapp database has been provided
var ErrNilKAppsDB = errors.New("nil kapps db")

// ErrNilKAppController signals that a nil kapp database has been provided
var ErrNilKAppController = errors.New("nil kapp controller")

// ErrEmptyEnabledIndexes signals that an empty slice of enables indexes has been provided
var ErrEmptyEnabledIndexes = errors.New("empty enabled indexes slice")

// ErrCannotFindAccountInDb -
var ErrCannotFindAccountInDb = errors.New("cannot find account in database")

// ErrInvalidDataMapLen -
var ErrInvalidDataMapLen = errors.New("invalid indexer data len")

// ErrCannotParseKDA -
var ErrCannotParseKDA = errors.New("cannot parse kda")
