package dataValidators_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/require"
)

// frozenSender is one of the consensus-frozen accounts (root / minter) from
// common/frozenAccounts.go, decoded to its raw public key.
var frozenSender, _ = hex.DecodeString("54ea28e527d4136508be955374afa54a8c25c19a48c674f412f7ce02db0f4e1b")

// thawedSender is the swap-service account released again at the FixAuditChangesV4 fork.
var thawedSender, _ = hex.DecodeString("bb687dbba23e1844fec674a32cb8809f0d3207506c53fc3d637e40dc56708d63")

func newFrozenTestValidator(t *testing.T, whiteListed bool) process.TxValidator {
	v, err := dataValidators.NewTxValidator(
		getAccAdapter(1_000_000),
		storageTest,
		getTxPoolsHolder(),
		&mock.WhiteListHandlerStub{
			IsWhiteListedCalled: func(process.InterceptedData) bool {
				return whiteListed
			},
		},
		mock.NewPubkeyConverterMock(32),
		&cryptoMock.SingleSignerStub{},
		&cryptoMock.KeyGenMock{
			PublicKeyFromByteArrayMock: func([]byte) (crypto.PublicKey, error) {
				return &cryptoMock.PublicKeyMock{}, nil
			},
		},
		getKAppController(),
		core.MaxTxNonceDeltaAllowed,
	)
	require.NoError(t, err)

	return v
}

func interceptedTxFrom(sender []byte) interface {
	process.InterceptedData
	process.TxValidatorHandler
} {
	return struct {
		process.InterceptedData
		process.TxValidatorHandler
	}{
		InterceptedData:    &mock.InterceptedDataStub{},
		TxValidatorHandler: getTxValidatorHandler(sender, 0, 0),
	}
}

// TestTxValidator_CheckTxValidity_FrozenSender pins the mempool admission side of the
// account freeze: a frozen sender's tx never enters the pool, with the same generic
// error the API returns, so the validator (which also backs create / estimate) cannot
// be used to enumerate the frozen set.
func TestTxValidator_CheckTxValidity_FrozenSender(t *testing.T) {
	t.Parallel()

	t.Run("frozen sender is refused with the generic error", func(t *testing.T) {
		t.Parallel()

		err := newFrozenTestValidator(t, false).CheckTxValidity(interceptedTxFrom(frozenSender))
		require.Equal(t, process.ErrWrongTransaction, err)
	})

	t.Run("thawed sender is admitted", func(t *testing.T) {
		t.Parallel()

		err := newFrozenTestValidator(t, false).CheckTxValidity(interceptedTxFrom(thawedSender))
		require.NoError(t, err)
	})

	t.Run("epoch-start bootstrap with no fork controller does not panic", func(t *testing.T) {
		t.Parallel()

		// The bootstrap interceptors run with a disabled kApp controller whose fork
		// controller is nil, and a disabled accounts adapter that finds no account; the
		// thawed entry is the one that consults a fork flag.
		v, err := dataValidators.NewTxValidator(
			&mock.AccountsStub{
				GetExistingAccountCalled: func([]byte) (state.AccountHandler, error) {
					return nil, nil
				},
			},
			storageTest,
			getTxPoolsHolder(),
			&mock.WhiteListHandlerStub{},
			mock.NewPubkeyConverterMock(32),
			&cryptoMock.SingleSignerStub{},
			&cryptoMock.KeyGenMock{
				PublicKeyFromByteArrayMock: func([]byte) (crypto.PublicKey, error) {
					return &cryptoMock.PublicKeyMock{}, nil
				},
			},
			disabled.NewKAppsController(),
			core.MaxTxNonceDeltaAllowed,
		)
		require.NoError(t, err)

		require.NotPanics(t, func() {
			require.NoError(t, v.CheckTxValidity(interceptedTxFrom(thawedSender)))
		})
	})

	t.Run("whitelisted tx from a frozen sender is admitted", func(t *testing.T) {
		t.Parallel()

		// Whitelisted data was requested to process a block; refusing it would stall
		// the node on blocks that already carry the tx (history or pre-activation).
		err := newFrozenTestValidator(t, true).CheckTxValidity(interceptedTxFrom(frozenSender))
		require.NoError(t, err)
	})
}
