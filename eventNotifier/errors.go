package eventNotifier

import "errors"

// ErrNilArgsNewEpochStartTrigger signals that nil arguments were provided
var ErrNilArgsNewEpochStartTrigger = errors.New("nil arguments for start of epoch trigger")

// ErrNilEpochStartNotifier signals that nil epoch start notifier has been provided
var ErrNilEpochStartNotifier = errors.New("nil epoch start notifier")

// ErrWrongTypeAssertion signals wrong type assertion
var ErrWrongTypeAssertion = errors.New("wrong type assertion")

// ErrNilMarshalizer signals that nil marshalizer has been provided
var ErrNilMarshalizer = errors.New("nil marshalizer")

// ErrNilStorage signals that nil storage has been provided
var ErrNilStorage = errors.New("nil storage")

// ErrNilTriggerStorage signals that nil meta header storage has been provided
var ErrNilTriggerStorage = errors.New("nil trigger storage")

// ErrNilBlockStorage signals that nil blocks storage has been provided
var ErrNilBlockStorage = errors.New("nil blocks storage")

// ErrNilStatusHandler signals that a nil status handler has been provided
var ErrNilStatusHandler = errors.New("nil app status handler")

// ErrNilStorageService signals that nil storage service has been provided
var ErrNilStorageService = errors.New("nil storage service")

// ErrNilHasher signals that nil hasher has been provided
var ErrNilHasher = errors.New("nil hasher")

// ErrNilHeaderHandler signals that a nil header handler has been provided
var ErrNilHeaderHandler = errors.New("nil header handler")

// ErrNilStratEpochHeader -
var ErrNilStratEpochHeader = errors.New("nil start epoch header")
