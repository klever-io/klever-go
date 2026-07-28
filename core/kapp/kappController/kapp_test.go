package kappcontroller

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

func testArgs(t *testing.T) ArgsNewKApp {
	t.Helper()

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &commonMock.EpochNotifierStub{})
	require.NoError(t, err)

	pubkeyConv, err := pubkeyConverter.NewBech32PubkeyConverter(32)
	require.NoError(t, err)

	return ArgsNewKApp{
		Hasher:         &commonMock.HasherMock{},
		Marshalizer:    &commonMock.ProtoMarshalizerMock{},
		PubkeyConv:     pubkeyConv,
		ForkController: forkController,
		RatingsData:    &commonMock.RatingsInfoMock{},
	}
}

// cacherOverProposalKApp builds an AccountsCacher whose KApp adapter resolves any
// address to the given KApp account stub, so InitKApps can run end to end without
// trie-backed storage.
func cacherOverProposalKApp(t *testing.T, proposalKApp state.KAppAccountHandler) state.AccountsCacher {
	t.Helper()

	cacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: &commonMock.AccountsStub{},
		Peers:    &commonMock.AccountsStub{},
		Kapps: &commonMock.AccountsStub{
			LoadAccountCalled: func(_ []byte) (state.AccountHandler, error) {
				return proposalKApp, nil
			},
		},
	})
	require.NoError(t, err)
	cacher.ResetAll(true)

	return cacher
}

// ReadOnly is construction-time state: there is deliberately no setter, so the
// only way the flag can be set is through ArgsNewKApp.
func TestNewKappController_ReadOnlyComesFromArgs(t *testing.T) {
	t.Parallel()

	for _, readOnly := range []bool{false, true} {
		args := testArgs(t)
		args.ReadOnly = readOnly

		controller, err := NewKappController(args)
		require.NoError(t, err)
		require.Equal(t, readOnly, controller.IsReadOnly())
	}
}

// Default must be writable, so production controllers built without the field
// are unaffected by the query-path guard.
func TestNewKappController_DefaultsToWritable(t *testing.T) {
	t.Parallel()

	controller, err := NewKappController(testArgs(t))
	require.NoError(t, err)
	require.False(t, controller.IsReadOnly())
}

// Regression: InitKApps used to discard the ActiveProposalController it built, so
// a controller never wired through node.go (the VM query controller in
// cmd/node/sc.go) was left with a nil one and nil-dereferenced on any query
// reaching AssetTrigger, Deposit, Proposal or Vote.
func TestInitKApps_KeepsProposalController(t *testing.T) {
	t.Parallel()

	expected, err := kapps.NewProposalController(testArgs(t).ForkController)
	require.NoError(t, err)

	started := false
	proposalKApp := &commonMock.KAppAccountHandlerStub{
		StartProposalsKAppCalled: func(_ core.ForkController) (kapps.ActiveProposalController, error) {
			started = true
			return expected, nil
		},
	}

	controller, err := NewKappController(testArgs(t))
	require.NoError(t, err)
	require.Nil(t, controller.GetProposalController(), "precondition: nothing set before InitKApps")

	require.NoError(t, controller.InitKApps(cacherOverProposalKApp(t, proposalKApp)))

	require.True(t, started, "InitKApps must start the proposals kapp")
	require.NotNil(t, controller.GetProposalController(),
		"InitKApps must keep the controller it built, not discard it")
	require.Same(t, expected, controller.GetProposalController(),
		"the kept controller must be the one StartProposalsKApp returned")
}

// Production and the tx simulator run InitKApps first and are then handed the
// shared instance by node.go - the later SetProposalController must win.
func TestInitKApps_SetProposalControllerOverwrites(t *testing.T) {
	t.Parallel()

	fromInit, err := kapps.NewProposalController(testArgs(t).ForkController)
	require.NoError(t, err)
	shared, err := kapps.NewProposalController(testArgs(t).ForkController)
	require.NoError(t, err)

	proposalKApp := &commonMock.KAppAccountHandlerStub{
		StartProposalsKAppCalled: func(_ core.ForkController) (kapps.ActiveProposalController, error) {
			return fromInit, nil
		},
	}

	controller, err := NewKappController(testArgs(t))
	require.NoError(t, err)
	require.NoError(t, controller.InitKApps(cacherOverProposalKApp(t, proposalKApp)))
	require.Same(t, fromInit, controller.GetProposalController())

	require.NoError(t, controller.SetProposalController(shared))
	require.Same(t, shared, controller.GetProposalController(),
		"node.go must be able to replace the InitKApps instance with the shared one")
}

// A failure starting the proposals kapp must abort InitKApps rather than leaving a
// half-initialised controller behind.
func TestInitKApps_StartProposalsError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("start proposals failed")
	proposalKApp := &commonMock.KAppAccountHandlerStub{
		StartProposalsKAppCalled: func(_ core.ForkController) (kapps.ActiveProposalController, error) {
			return nil, expectedErr
		},
	}

	controller, err := NewKappController(testArgs(t))
	require.NoError(t, err)

	err = controller.InitKApps(cacherOverProposalKApp(t, proposalKApp))
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, controller.GetProposalController(),
		"a failed start must not leave a proposal controller behind")
}

// initedController builds a controller and runs InitKApps, which is what wires the
// controller itself into each contained KApp (see the SetKAppController loop).
func initedController(t *testing.T, readOnly bool) kapp.KAppController {
	t.Helper()

	args := testArgs(t)
	args.ReadOnly = readOnly

	controller, err := NewKappController(args)
	require.NoError(t, err)

	proposalKApp := &commonMock.KAppAccountHandlerStub{
		StartProposalsKAppCalled: func(fc core.ForkController) (kapps.ActiveProposalController, error) {
			return kapps.NewProposalController(fc)
		},
	}
	require.NoError(t, controller.InitKApps(cacherOverProposalKApp(t, proposalKApp)))

	return controller
}

// End-to-end link between the construction-time flag and the enforcement point:
// InitKApps hands the controller to every contained KApp, so a controller built
// with ReadOnly makes its accounts KApp refuse mutations. This is what makes the
// cmd/node/sc.go wiring (ReadOnly args -> query controller) actually protective.
// The writable counterpart is covered in the accounts package, which can stub the
// dependencies a transfer needs once it gets past the guard.
func TestKappController_ReadOnly_PropagatesToAccountsKApp(t *testing.T) {
	t.Parallel()

	receiver := make([]byte, 32)
	copy(receiver, []byte("receiver"))

	accountsKApp := initedController(t, true).GetAccountsKApp()

	status, err := accountsKApp.Transfer(
		transaction.TXContract_TransferContractType,
		make([]byte, 32),
		&transaction.TransferContract{
			ToAddress: receiver,
			AssetID:   kdautils.KLVIdentifier,
			Amount:    100,
		},
	)
	require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation)
	require.Equal(t, transaction.Transaction_KAPPError, status)
}

// InitKApps must not disturb the mode: the flag is construction-time state, and the
// wiring pass runs after it.
func TestKappController_ReadOnly_SurvivesInitKApps(t *testing.T) {
	t.Parallel()

	for _, readOnly := range []bool{false, true} {
		require.Equal(t, readOnly, initedController(t, readOnly).IsReadOnly())
	}
}
