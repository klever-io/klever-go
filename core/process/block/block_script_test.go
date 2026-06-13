package block_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	blproc "github.com/klever-io/klever-go/core/process/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Valid bech32 KLV addresses used as burn targets in the tests below.
const (
	testBurnAddr1 = "klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5"
	testBurnAddr2 = "klv1uag5q6w7dmty80ywy5857zz7gvcprylxagls30penwfhdgd9ms7q7pf47q"
)

// burnAccountsRecorder backs a UserAccountsState adapter: every LoadAccount returns a fresh
// account pre-funded with `balance`, and every SaveAccount is recorded so a test can assert
// the resulting (post-burn) balances.
type burnAccountsRecorder struct {
	balance int64
	saved   []state.UserAccountHandler
}

func newUserAccountsStub(balance int64) (*mock.AccountsStub, *burnAccountsRecorder) {
	rec := &burnAccountsRecorder{balance: balance}
	stub := &mock.AccountsStub{
		LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
			acc := state.NewUserAccountNoCheck(address)
			if balance > 0 {
				_ = acc.AddToBalance(balance, nil, false)
			}
			return acc, nil
		},
		SaveAccountCalled: func(account state.AccountHandler) error {
			rec.saved = append(rec.saved, account.(state.UserAccountHandler))
			return nil
		},
	}
	return stub, rec
}

func newScriptTestProcessor(t *testing.T, userAccounts state.AccountsAdapter) *blproc.MetaProcessorForTests {
	t.Helper()
	args := createMockMetaArguments()
	if userAccounts != nil {
		args.AccountsDB[state.UserAccountsState] = userAccounts
	}
	mpp, err := blproc.NewMetaProcessor(args)
	require.NoError(t, err)
	return blproc.NewMetaProcessorForTests(mpp)
}

func TestMetaProcessor_ExecuteInflationBurn(t *testing.T) {
	t.Run("wipes the full balance of every target and reports the total", func(t *testing.T) {
		restore := blproc.SetInflationBurnAddresses([]string{testBurnAddr1, testBurnAddr2})
		defer restore()

		stub, rec := newUserAccountsStub(1_000_000)
		mp := newScriptTestProcessor(t, stub)

		require.NoError(t, mp.ExecuteInflationBurn())

		require.Len(t, rec.saved, 2, "both funded targets should be saved with a wiped balance")
		for _, acc := range rec.saved {
			assert.Equal(t, int64(0), acc.GetBalance(nil, false))
		}
	})

	t.Run("targets with no balance are left untouched", func(t *testing.T) {
		restore := blproc.SetInflationBurnAddresses([]string{testBurnAddr1})
		defer restore()

		stub, rec := newUserAccountsStub(0)
		mp := newScriptTestProcessor(t, stub)

		require.NoError(t, mp.ExecuteInflationBurn())
		assert.Empty(t, rec.saved, "an empty balance must not trigger a SaveAccount")
	})

	t.Run("no configured targets is a safe no-op", func(t *testing.T) {
		restore := blproc.SetInflationBurnAddresses(nil)
		defer restore()

		stub, rec := newUserAccountsStub(1_000_000)
		mp := newScriptTestProcessor(t, stub)

		require.NoError(t, mp.ExecuteInflationBurn())
		assert.Empty(t, rec.saved)
	})
}

func TestMetaProcessor_RunScript(t *testing.T) {
	// Empty target list keeps BurnKLV a clean no-op so we exercise dispatch, not the burn itself.
	restore := blproc.SetInflationBurnAddresses(nil)
	defer restore()

	mp := newScriptTestProcessor(t, nil)

	ran, err := mp.RunScript(kapps.ScriptBurnKLV)
	require.NoError(t, err)
	assert.True(t, ran, "a registered, wired script must dispatch")

	ran, err = mp.RunScript("DefinitelyNotARegisteredScript")
	require.NoError(t, err)
	assert.False(t, ran, "an unknown script must be skipped, never erroring out of block processing")
}

// TestScriptExecutorsCoverRegistry fails if a script is added to kapps.scriptRegistry without a
// matching executor in runScript — the drift that would otherwise let a proposal be accepted at
// creation and then error at end-of-epoch execution.
func TestScriptExecutorsCoverRegistry(t *testing.T) {
	mp := newScriptTestProcessor(t, nil)

	executors := make(map[string]bool)
	for _, name := range mp.ScriptExecutorNames() {
		executors[name] = true
	}

	for _, name := range kapps.RegisteredScriptNames() {
		assert.Truef(t, executors[name], "script %q is registered but has no executor wired in runScript", name)
	}
}

func TestMetaProcessor_ApplyScript_RunsAndResetsTrigger(t *testing.T) {
	restore := blproc.SetInflationBurnAddresses([]string{testBurnAddr1})
	defer restore()

	stub, rec := newUserAccountsStub(1_000_000)
	mp := newScriptTestProcessor(t, stub)

	controller := &kapps.ProposalController{
		ActiveParameters: map[int32]*kapps.Parameter{
			int32(kapps.EnumParameter_ExecuteScript): {
				Type:  kapps.EnumType_String,
				Value: []byte(kapps.ScriptBurnKLV),
			},
		},
	}

	require.NoError(t, mp.ApplyScript(controller, []byte(kapps.ScriptBurnKLV)))

	require.Len(t, rec.saved, 1, "the one-time BurnKLV script should run when no prior proposal executed it")
	assert.Equal(t, int64(0), rec.saved[0].GetBalance(nil, false))
	assert.Equal(t, []byte(""), controller.ActiveParameters[int32(kapps.EnumParameter_ExecuteScript)].Value,
		"the transient trigger must be reset to empty after running")
}
