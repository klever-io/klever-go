package apiResolver

import "errors"

// ErrNilStatusMetrics signals that a nil status metrics was provided
var ErrNilStatusMetrics = errors.New("nil status metrics handler")

// ErrNilTotalStakedValueHandler signals that a nil total staked value handler has been provided
var ErrNilTotalStakedValueHandler = errors.New("nil total staked value handler")

// ErrNilSCQueryService signals that a nil SC query service has been provided
var ErrNilSCQueryService = errors.New("nil SC query service")

// ErrNilTransactionCostHandler signals that a nil transaction cost handler was provided
var ErrNilTransactionCostHandler = errors.New("nil transaction cost handler")
