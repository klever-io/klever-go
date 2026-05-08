package prewarmer

import (
	"github.com/klever-io/klever-go/data/state"
)

// HookProvider is the slice of BlockChainHookImpl this package consumes,
// expressed as an interface so production wiring stays decoupled from the
// concrete struct (and so tests can stub it).
type HookProvider interface {
	GetUserAccount(address []byte) (state.UserAccountHandler, error)
	GetCode(account state.UserAccountHandler) []byte
	GetCompiledCode(codeHash []byte) (bool, []byte)
	SaveCompiledCode(codeHash []byte, code []byte)
}

// hookAdapter exposes a HookProvider as the prewarmer's CodeFetcher +
// CompiledCodeStore. Wire it into the Args at construction:
//
//	adapter := prewarmer.NewHookAdapter(blockChainHook)
//	pw, _ := prewarmer.New(prewarmer.Args{
//	    CodeFetcher:   adapter,
//	    CompiledStore: adapter,
//	    Compiler:      prewarmer.NewExecutorCompiler(executor),
//	    ...
//	})
type hookAdapter struct {
	hook HookProvider
}

// NewHookAdapter wraps a BlockChainHookImpl-compatible hook for the prewarmer.
func NewHookAdapter(hook HookProvider) *hookAdapter {
	return &hookAdapter{hook: hook}
}

// FetchCode resolves an address to (codeHash, bytecode). Returns nil hash if
// the account has no code; the prewarmer treats that as a no-op.
func (a *hookAdapter) FetchCode(address []byte) ([]byte, []byte, error) {
	account, err := a.hook.GetUserAccount(address)
	if err != nil {
		return nil, nil, err
	}
	if account == nil {
		return nil, nil, nil
	}
	codeHash := account.GetCodeHash()
	if len(codeHash) == 0 {
		return nil, nil, nil
	}
	code := a.hook.GetCode(account)
	return codeHash, code, nil
}

// HasCompiledCode reports whether the cache already holds the AOT bytes for
// codeHash. The prewarmer skips compile when this returns true.
func (a *hookAdapter) HasCompiledCode(codeHash []byte) bool {
	found, _ := a.hook.GetCompiledCode(codeHash)
	return found
}

// SaveCompiledCode persists the AOT bytes via the underlying hook.
func (a *hookAdapter) SaveCompiledCode(codeHash, code []byte) {
	a.hook.SaveCompiledCode(codeHash, code)
}
