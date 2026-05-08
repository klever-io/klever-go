package prewarmer

import "errors"

var (
	ErrNilCodeFetcher   = errors.New("prewarmer: nil code fetcher")
	ErrNilCompiledStore = errors.New("prewarmer: nil compiled-code store")
	ErrNilCompiler      = errors.New("prewarmer: nil compiler")
)
