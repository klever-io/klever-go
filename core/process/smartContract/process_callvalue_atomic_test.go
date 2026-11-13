package smartContract

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	commommock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	accountsKapp "github.com/klever-io/klever-go/core/kapp/accounts"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
)

// These tests pay a two-asset call value through processSCPayment against a real accounts KApp and
// read the accounts back with no transaction-level reset. The first asset in DeterministicMap order
// is always valid and the second one fails. From FixAuditChangesV7 the failing asset leaves the
// first one unpaid; before the fork the first one is already paid when the second one fails.

const (
	callValueValidAsset   = "AAA-1"
	callValueFailingAsset = "BBB-2"
	callValueSenderFunds  = int64(1_000)
	callValueValidAmount  = int64(100)
)

var (
	callValueSender   = bytes.Repeat([]byte{0x11}, 32)
	callValueContract = bytes.Repeat([]byte{0x22}, 32)
)

// callValueWorld is a real accounts KApp over live accounts: every load of an address hands back
// the same instance, as the production cacher does with the cache on.
type callValueWorld struct {
	sc       *scProcessor
	ctx      kapp.KappContext
	accounts map[string]state.UserAccountHandler
}

func newCallValueWorld(t *testing.T, fixV7 bool, failing *kapps.KDAData, failingFunds int64) *callValueWorld {
	t.Helper()

	return newCallValueWorldWith(t, fixV7, callValueKDA(), failing, failingFunds)
}

// newCallValueWorldWith is newCallValueWorld with the first asset's KDA given too.
func newCallValueWorldWith(t *testing.T, fixV7 bool, first, failing *kapps.KDAData, failingFunds int64) *callValueWorld {
	t.Helper()

	cfg := config.EnableEpochs{}
	if !fixV7 {
		cfg.FixAuditChangesV7 = 1000
	}
	forkController, err := fork.NewForkController(cfg, &commommock.EpochNotifierStub{})
	require.NoError(t, err)
	require.Equal(t, fixV7, forkController.FixAuditChangesV7())

	emptyTrie := &commommock.TrieStub{GetCalled: func([]byte) ([]byte, error) { return nil, nil }}
	w := &callValueWorld{accounts: make(map[string]state.UserAccountHandler)}
	load := func(address []byte) (state.UserAccountHandler, error) {
		if acc, ok := w.accounts[string(address)]; ok {
			return acc, nil
		}
		acc, errNew := state.NewUserAccount(address)
		if errNew != nil {
			return nil, errNew
		}
		acc.SetDataTrie(emptyTrie)
		w.accounts[string(address)] = acc
		return acc, nil
	}

	sender, err := load(callValueSender)
	require.NoError(t, err)
	require.NoError(t, sender.AddToBalance(callValueSenderFunds, []byte(callValueValidAsset), true))
	if failingFunds > 0 {
		require.NoError(t, sender.AddToBalance(failingFunds, []byte(callValueFailingAsset), true))
	}

	w.ctx = kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: callValueSender,
		ContractID:     0,
		ContractType:   transaction.TXContract_SmartContractType,
		Block:          &block.Block{},
		TX:             &transaction.Transaction{},
	})

	kdas := map[string]*kapps.KDAData{
		callValueValidAsset:   first,
		callValueFailingAsset: failing,
	}

	accKapp, err := accountsKapp.NewAccountKApp(&accountsKapp.ArgsNewAccountKApp{
		Marshalizer:    &commommock.ProtoMarshalizerMock{},
		PubkeyConv:     commommock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	})
	require.NoError(t, err)
	require.NoError(t, accKapp.SetAccountsCacher(&commommock.AccountsCacherStub{
		LoadUserCalled:   load,
		UpdateUserCalled: func(state.AccountHandler) error { return nil },
	}))
	require.NoError(t, accKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return w.ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					kda, ok := kdas[string(assetID)]
					if !ok {
						return nil, nil, fmt.Errorf("unexpected asset %s", assetID)
					}
					return nil, kda, nil
				},
			}
		},
	}))

	w.sc = &scProcessor{
		blockChainHook: &contextmock.BlockchainHookStub{
			GetKAppControllerCalled: func() kapp.KAppController {
				return &kvmStub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp { return accKapp },
				}
			},
		},
		forkController: forkController,
	}

	return w
}

func callValueKDA() *kapps.KDAData {
	return &kapps.KDAData{
		AssetType:  kapps.KDAData_Fungible,
		Attributes: &kapps.AttributesData{},
		Properties: &kapps.PropertiesData{},
	}
}

func (w *callValueWorld) balance(address []byte, assetID string) int64 {
	acc, ok := w.accounts[string(address)]
	if !ok {
		return 0
	}
	return acc.GetBalance([]byte(assetID), true)
}

// pay pays a call value of the valid asset and failingAmount of the failing one, with the
// failing asset's royalties.
func (w *callValueWorld) pay(failingAmount, failingRoyalties int64) error {
	tc := &transaction.SmartContract{
		Type:    transaction.SmartContract_SCInvoke,
		Address: callValueContract,
		CallValue: map[string]*transaction.CallValue{
			callValueFailingAsset: {Amount: failingAmount, KDARoyalties: failingRoyalties},
			callValueValidAsset:   {Amount: callValueValidAmount},
		},
	}

	return w.sc.processSCPayment(tc, w.accounts[string(callValueSender)])
}

type callValueFailure struct {
	name             string
	failing          func() *kapps.KDAData
	failingFunds     int64
	failingAmount    int64
	failingRoyalties int64
	expectedErr      error
	expectedCode     transaction.Transaction_TXResultCode
}

func callValueFailures() []callValueFailure {
	return []callValueFailure{
		{
			name:          "insufficient funds",
			failing:       callValueKDA,
			failingFunds:  10,
			failingAmount: 50,
			expectedErr:   process.ErrInsufficientFunds,
			expectedCode:  transaction.Transaction_OutOfFunds,
		},
		{
			name: "paused asset",
			failing: func() *kapps.KDAData {
				kda := callValueKDA()
				kda.Attributes.IsPaused = true
				return kda
			},
			failingFunds:  1_000,
			failingAmount: 50,
			expectedErr:   process.ErrAssetIsPaused,
			expectedCode:  transaction.Transaction_AssetPaused,
		},
		{
			name: "transfer role denied",
			failing: func() *kapps.KDAData {
				kda := callValueKDA()
				kda.Properties.LimitTransfer = true
				return kda
			},
			failingFunds:  1_000,
			failingAmount: 50,
			expectedErr:   process.ErrKDATransferNotAllowed,
			expectedCode:  transaction.Transaction_KDATransferNotAllowed,
		},
		{
			// A 10% transfer royalty on 50 is 5; the call value declares none.
			name: "royalty mismatch",
			failing: func() *kapps.KDAData {
				kda := callValueKDA()
				kda.Royalties = &kapps.RoyaltiesData{
					Address:            bytes.Repeat([]byte{0x33}, 32),
					TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
				}
				return kda
			},
			failingFunds:     1_000,
			failingAmount:    50,
			failingRoyalties: 0,
			expectedErr:      common.ErrInvalidValue,
			expectedCode:     transaction.Transaction_ParameterInvalid,
		},
	}
}

func TestProcessSCPayment_FailingAssetLeavesNoOtherAssetPaid(t *testing.T) {
	t.Parallel()

	for _, tc := range callValueFailures() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			legacy := newCallValueWorld(t, false, tc.failing(), tc.failingFunds)
			legacyErr := legacy.pay(tc.failingAmount, tc.failingRoyalties)

			fixed := newCallValueWorld(t, true, tc.failing(), tc.failingFunds)
			err := fixed.pay(tc.failingAmount, tc.failingRoyalties)

			// The reported outcome is unchanged.
			expected := fmt.Sprintf("result code: %d, %v", tc.expectedCode, tc.expectedErr)
			require.EqualError(t, legacyErr, expected)
			require.EqualError(t, err, expected)
			require.Equal(t, legacy.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().GetPreserved(),
				"the same error receipts")

			// From FixAuditChangesV7 nothing is paid.
			require.Equal(t, callValueSenderFunds, fixed.balance(callValueSender, callValueValidAsset))
			require.Zero(t, fixed.balance(callValueContract, callValueValidAsset))
			require.Equal(t, tc.failingFunds, fixed.balance(callValueSender, callValueFailingAsset))
			require.Zero(t, fixed.balance(callValueContract, callValueFailingAsset))
			require.Equal(t, fixed.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().Get(),
				"no transfer receipt")

			// Before the fork the first asset is already paid: only the transaction reset undoes it.
			require.Equal(t, callValueSenderFunds-callValueValidAmount, legacy.balance(callValueSender, callValueValidAsset))
			require.Equal(t, callValueValidAmount, legacy.balance(callValueContract, callValueValidAsset))
		})
	}
}

// When both assets fail, each with its own error, the first one in DeterministicMap order is the
// one reported, from FixAuditChangesV7 as before it.
func TestProcessSCPayment_ReportsTheFirstFailingAssetInOrder(t *testing.T) {
	t.Parallel()

	paused := func() *kapps.KDAData {
		kda := callValueKDA()
		kda.Attributes.IsPaused = true
		return kda
	}
	expected := fmt.Sprintf("result code: %d, %v", transaction.Transaction_AssetPaused, process.ErrAssetIsPaused)

	var preserved [][]*transaction.Transaction_Receipt
	for _, fixV7 := range []bool{false, true} {
		// The first asset is paused; the second one is short of funds.
		w := newCallValueWorldWith(t, fixV7, paused(), callValueKDA(), 10)
		require.EqualError(t, w.pay(50, 0), expected, "FixAuditChangesV7=%v", fixV7)
		require.Zero(t, w.balance(callValueContract, callValueFailingAsset))
		preserved = append(preserved, w.ctx.Receipts().GetPreserved())
	}
	require.Equal(t, preserved[0], preserved[1], "the same error receipts")
}

func TestProcessSCPayment_PaysEveryAssetWhenAllAreValid(t *testing.T) {
	t.Parallel()

	for _, fixV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV7=%v", fixV7), func(t *testing.T) {
			t.Parallel()

			w := newCallValueWorld(t, fixV7, callValueKDA(), 1_000)
			require.NoError(t, w.pay(50, 0))

			require.Equal(t, callValueSenderFunds-callValueValidAmount, w.balance(callValueSender, callValueValidAsset))
			require.Equal(t, callValueValidAmount, w.balance(callValueContract, callValueValidAsset))
			require.Equal(t, int64(950), w.balance(callValueSender, callValueFailingAsset))
			require.Equal(t, int64(50), w.balance(callValueContract, callValueFailingAsset))
			require.Len(t, w.ctx.Receipts().Get(), 2, "one transfer receipt per asset")
		})
	}
}
