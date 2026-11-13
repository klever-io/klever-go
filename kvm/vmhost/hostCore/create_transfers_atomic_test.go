package hostCore_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
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
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/vmcommon"
)

// These tests pay a deploy's two KDA transfers through payCreateTransfers against a real accounts
// KApp and read the accounts back with no transaction-level reset. The first transfer is valid
// unless a test says otherwise, and the second one fails. From FixAuditChangesV7 the failing
// transfer leaves the first one unpaid; before the fork the first one is already paid when the
// second one fails.

const (
	createFirstAsset  = "AAA-1"
	createSecondAsset = "BBB-2"
	createFirstFunds  = int64(1_000)
	createFirstAmount = int64(100)
)

var (
	createCaller          = bytes.Repeat([]byte{0x11}, 32)
	createContract        = bytes.Repeat([]byte{0x22}, 32)
	createRoyaltyReceiver = bytes.Repeat([]byte{0x33}, 32)
)

// royaltyCreateKDA is a fungible asset with a 10% transfer royalty paid to createRoyaltyReceiver.
func royaltyCreateKDA() *kapps.KDAData {
	kda := createKDA()
	kda.Royalties = &kapps.RoyaltiesData{
		Address:            createRoyaltyReceiver,
		TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
	}
	return kda
}

// createWorld is a real accounts KApp over live accounts: every load of an address hands back the
// same instance, as the production cacher does with the cache on.
type createWorld struct {
	forks    core.ForkController
	accKapp  kapp.AccountsKapp
	ctx      kapp.KappContext
	accounts map[string]state.UserAccountHandler
}

func newCreateWorld(t *testing.T, fixV7 bool, first, second *kapps.KDAData, secondFunds int64) *createWorld {
	t.Helper()

	cfg := config.EnableEpochs{}
	if !fixV7 {
		cfg.FixAuditChangesV7 = 1000
	}
	forks, err := fork.NewForkController(cfg, &commommock.EpochNotifierStub{})
	require.NoError(t, err)
	require.Equal(t, fixV7, forks.FixAuditChangesV7())

	emptyTrie := &commommock.TrieStub{GetCalled: func([]byte) ([]byte, error) { return nil, nil }}
	w := &createWorld{forks: forks, accounts: make(map[string]state.UserAccountHandler)}
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

	caller, err := load(createCaller)
	require.NoError(t, err)
	require.NoError(t, caller.AddToBalance(createFirstFunds, []byte(createFirstAsset), true))
	if secondFunds > 0 {
		require.NoError(t, caller.AddToBalance(secondFunds, []byte(createSecondAsset), true))
	}

	w.ctx = kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: createCaller,
		ContractID:     0,
		ContractType:   transaction.TXContract_SmartContractType,
		Block:          &block.Block{},
		TX:             &transaction.Transaction{},
	})

	kdas := map[string]*kapps.KDAData{createFirstAsset: first, createSecondAsset: second}
	accKapp, err := accountsKapp.NewAccountKApp(&accountsKapp.ArgsNewAccountKApp{
		Marshalizer:    &commommock.ProtoMarshalizerMock{},
		PubkeyConv:     commommock.NewPubkeyConverterMock(32),
		ForkController: forks,
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
	w.accKapp = accKapp

	return w
}

func createKDA() *kapps.KDAData {
	return &kapps.KDAData{
		AssetType:  kapps.KDAData_Fungible,
		Attributes: &kapps.AttributesData{},
		Properties: &kapps.PropertiesData{},
	}
}

func pausedCreateKDA() *kapps.KDAData {
	kda := createKDA()
	kda.Attributes.IsPaused = true
	return kda
}

func (w *createWorld) balance(address []byte, assetID string) int64 {
	acc, ok := w.accounts[string(address)]
	if !ok {
		return 0
	}
	return acc.GetBalance([]byte(assetID), true)
}

// createTransfers is a deploy's transfers: the first asset, then secondValue of the second one
// with its royalties.
func createTransfers(secondValue *big.Int, secondRoyalties int64) []*vmcommon.KDATransfer {
	return []*vmcommon.KDATransfer{
		{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(createFirstAmount)},
		{KDATokenName: []byte(createSecondAsset), KDAValue: secondValue, KDARoyalties: secondRoyalties},
	}
}

func (w *createWorld) pay(transfers []*vmcommon.KDATransfer) error {
	return hostCore.PayCreateTransfersForTest(w.forks, func() kapp.AccountsKapp { return w.accKapp }, createCaller, createContract, transfers)
}

func requireNoneExecuted(t *testing.T, transfers []*vmcommon.KDATransfer, msgAndArgs ...interface{}) {
	t.Helper()
	for _, transfer := range transfers {
		require.False(t, transfer.IsExecuted(), msgAndArgs...)
	}
}

func TestPayCreateTransfers_FailingTransferLeavesNoOtherTransferPaid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		second          func() *kapps.KDAData
		secondFunds     int64
		secondRoyalties int64
		expectedErr     error
	}{
		{name: "insufficient funds", second: createKDA, secondFunds: 10, expectedErr: process.ErrInsufficientFunds},
		{name: "paused asset", second: pausedCreateKDA, secondFunds: 1_000, expectedErr: process.ErrAssetIsPaused},
		{
			name: "transfer role denied",
			second: func() *kapps.KDAData {
				kda := createKDA()
				kda.Properties.LimitTransfer = true
				return kda
			},
			secondFunds: 1_000,
			expectedErr: process.ErrKDATransferNotAllowed,
		},
		{
			// A 10% transfer royalty on 50 is 5; the transfer declares none.
			name:        "royalty mismatch",
			second:      royaltyCreateKDA,
			secondFunds: 1_000,
			expectedErr: common.ErrInvalidValue,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			legacy := newCreateWorld(t, false, createKDA(), tc.second(), tc.secondFunds)
			legacyTransfers := createTransfers(big.NewInt(50), tc.secondRoyalties)
			legacyErr := legacy.pay(legacyTransfers)

			fixed := newCreateWorld(t, true, createKDA(), tc.second(), tc.secondFunds)
			transfers := createTransfers(big.NewInt(50), tc.secondRoyalties)
			err := fixed.pay(transfers)

			// The reported outcome is unchanged.
			require.ErrorIs(t, legacyErr, tc.expectedErr)
			require.Equal(t, legacyErr, err)
			require.Equal(t, legacy.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().GetPreserved(),
				"the same error receipts")

			// From FixAuditChangesV7 nothing is paid or marked executed.
			require.Equal(t, createFirstFunds, fixed.balance(createCaller, createFirstAsset))
			require.Zero(t, fixed.balance(createContract, createFirstAsset))
			require.Equal(t, tc.secondFunds, fixed.balance(createCaller, createSecondAsset))
			require.Zero(t, fixed.balance(createContract, createSecondAsset))
			require.Equal(t, fixed.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().Get(), "no transfer receipt")
			for _, transfer := range transfers {
				require.False(t, transfer.IsExecuted(), "%s must not be marked executed", transfer.KDATokenName)
			}

			// Before the fork the first transfer is already paid: only the transaction reset undoes it.
			require.Equal(t, createFirstFunds-createFirstAmount, legacy.balance(createCaller, createFirstAsset))
			require.Equal(t, createFirstAmount, legacy.balance(createContract, createFirstAsset))
			require.True(t, legacyTransfers[0].IsExecuted())
			require.False(t, legacyTransfers[1].IsExecuted())

			// No royalty is paid on either side of the fork.
			require.Zero(t, fixed.balance(createRoyaltyReceiver, createSecondAsset))
			require.Zero(t, legacy.balance(createRoyaltyReceiver, createSecondAsset))
		})
	}
}

// When both transfers fail, each with its own error, the first one in the order given is the one
// reported, from FixAuditChangesV7 as before it.
func TestPayCreateTransfers_ReportsTheFirstFailingTransferInOrder(t *testing.T) {
	t.Parallel()

	var preserved [][]*transaction.Transaction_Receipt
	for _, fixV7 := range []bool{false, true} {
		// The first asset is paused; the second one is short of funds.
		w := newCreateWorld(t, fixV7, pausedCreateKDA(), createKDA(), 10)
		transfers := createTransfers(big.NewInt(50), 0)
		require.ErrorIs(t, w.pay(transfers), process.ErrAssetIsPaused, "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, createFirstFunds, w.balance(createCaller, createFirstAsset), "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, int64(10), w.balance(createCaller, createSecondAsset), "FixAuditChangesV7=%v", fixV7)
		requireNoneExecuted(t, transfers, "FixAuditChangesV7=%v", fixV7)
		preserved = append(preserved, w.ctx.Receipts().GetPreserved())
	}
	require.Equal(t, preserved[0], preserved[1], "the same error receipts")
}

// The legacy loop pays a transfer before it reads the next transfer's value, so an invalid value
// is reported only once the transfers before it pass their checks. The batch reports the same
// error and pays nothing.
func TestPayCreateTransfers_InvalidValueAfterEarlierTransfers(t *testing.T) {
	t.Parallel()

	oversized := new(big.Int).Lsh(big.NewInt(1), 64)

	for _, tc := range []struct {
		name        string
		first       func() *kapps.KDAData
		expectedErr error
		firstPaid   bool
	}{
		{name: "earlier transfer valid", first: createKDA, expectedErr: common.ErrInvalidValue, firstPaid: true},
		{name: "earlier transfer fails its checks", first: pausedCreateKDA, expectedErr: process.ErrAssetIsPaused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var errs []error
			var preserved [][]*transaction.Transaction_Receipt
			for _, fixV7 := range []bool{false, true} {
				w := newCreateWorld(t, fixV7, tc.first(), createKDA(), 1_000)
				transfers := createTransfers(oversized, 0)
				err := w.pay(transfers)
				require.ErrorIs(t, err, tc.expectedErr, "FixAuditChangesV7=%v", fixV7)
				errs = append(errs, err)
				preserved = append(preserved, w.ctx.Receipts().GetPreserved())

				expectedFirst := createFirstFunds
				legacyPaidFirst := tc.firstPaid && !fixV7
				if legacyPaidFirst {
					expectedFirst -= createFirstAmount
				}
				require.Equal(t, expectedFirst, w.balance(createCaller, createFirstAsset), "FixAuditChangesV7=%v", fixV7)
				require.Equal(t, legacyPaidFirst, transfers[0].IsExecuted(), "FixAuditChangesV7=%v", fixV7)
				require.False(t, transfers[1].IsExecuted(), "FixAuditChangesV7=%v", fixV7)
			}
			require.Equal(t, errs[0], errs[1])
			require.Equal(t, preserved[0], preserved[1], "the same error receipts")
		})
	}
}

// The accounts KApp is fetched only once there is a transfer to pay, as before the fork: a deploy
// with no transfer, or with every transfer already executed, never reads it.
func TestPayCreateTransfers_FetchesTheAccountsKAppOnlyToPay(t *testing.T) {
	t.Parallel()

	executed := &vmcommon.KDATransfer{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(1)}
	executed.SetExecuted()

	for _, fixV7 := range []bool{false, true} {
		for name, transfers := range map[string][]*vmcommon.KDATransfer{
			"no transfer":             nil,
			"every transfer executed": {executed},
		} {
			t.Run(fmt.Sprintf("FixAuditChangesV7=%v/%s", fixV7, name), func(t *testing.T) {
				w := newCreateWorld(t, fixV7, createKDA(), createKDA(), 0)
				fetched := 0
				err := hostCore.PayCreateTransfersForTest(w.forks, func() kapp.AccountsKapp {
					fetched++
					return w.accKapp
				}, createCaller, createContract, transfers)
				require.NoError(t, err)
				require.Zero(t, fetched)
			})
		}
	}
}

func TestPayCreateTransfers_PaysEveryTransferWhenAllAreValid(t *testing.T) {
	t.Parallel()

	for _, fixV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV7=%v", fixV7), func(t *testing.T) {
			t.Parallel()

			w := newCreateWorld(t, fixV7, createKDA(), createKDA(), 1_000)
			executed := &vmcommon.KDATransfer{KDATokenName: []byte(createSecondAsset), KDAValue: big.NewInt(500)}
			executed.SetExecuted()
			transfers := append(createTransfers(big.NewInt(50), 0), executed)

			require.NoError(t, w.pay(transfers))

			require.Equal(t, createFirstFunds-createFirstAmount, w.balance(createCaller, createFirstAsset))
			require.Equal(t, createFirstAmount, w.balance(createContract, createFirstAsset))
			require.Equal(t, int64(950), w.balance(createCaller, createSecondAsset), "an executed transfer is not paid again")
			require.Equal(t, int64(50), w.balance(createContract, createSecondAsset))
			require.Len(t, w.ctx.Receipts().Get(), 2, "one transfer receipt per transfer paid")
			for _, transfer := range transfers {
				require.True(t, transfer.IsExecuted(), "%s must be marked executed", transfer.KDATokenName)
			}
		})
	}
}

// Two transfers of one asset, each affordable alone but not together: the batch counts the first
// against the second. Before the fork the first is paid when the second fails.
func TestPayCreateTransfers_SameAssetTwiceBeyondTheBalance(t *testing.T) {
	t.Parallel()

	twice := func() []*vmcommon.KDATransfer {
		return []*vmcommon.KDATransfer{
			{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(600)},
			{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(600)},
		}
	}

	legacy := newCreateWorld(t, false, createKDA(), createKDA(), 0)
	legacyTransfers := twice()
	legacyErr := legacy.pay(legacyTransfers)

	fixed := newCreateWorld(t, true, createKDA(), createKDA(), 0)
	transfers := twice()
	err := fixed.pay(transfers)

	require.ErrorIs(t, legacyErr, process.ErrInsufficientFunds)
	require.Equal(t, legacyErr, err)
	require.Equal(t, legacy.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().GetPreserved())

	require.Equal(t, createFirstFunds, fixed.balance(createCaller, createFirstAsset))
	require.Zero(t, fixed.balance(createContract, createFirstAsset))
	requireNoneExecuted(t, transfers)

	require.Equal(t, createFirstFunds-600, legacy.balance(createCaller, createFirstAsset))
	require.Equal(t, int64(600), legacy.balance(createContract, createFirstAsset))
	require.True(t, legacyTransfers[0].IsExecuted())
	require.False(t, legacyTransfers[1].IsExecuted())
}

// An invalid third value after a second transfer the balance cannot cover: the legacy loop fails on
// the second transfer before it reads the third value, so both report insufficient funds.
func TestPayCreateTransfers_InvalidValueAfterAnInsufficientTransfer(t *testing.T) {
	t.Parallel()

	oversized := new(big.Int).Lsh(big.NewInt(1), 64)
	var errs []error
	for _, fixV7 := range []bool{false, true} {
		w := newCreateWorld(t, fixV7, createKDA(), createKDA(), 1_000)
		transfers := []*vmcommon.KDATransfer{
			{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(600)},
			{KDATokenName: []byte(createFirstAsset), KDAValue: big.NewInt(600)},
			{KDATokenName: []byte(createSecondAsset), KDAValue: oversized},
		}
		err := w.pay(transfers)
		require.ErrorIs(t, err, process.ErrInsufficientFunds, "FixAuditChangesV7=%v", fixV7)
		errs = append(errs, err)

		expectedFirst := createFirstFunds
		if !fixV7 {
			expectedFirst -= 600
		}
		require.Equal(t, expectedFirst, w.balance(createCaller, createFirstAsset), "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, !fixV7, transfers[0].IsExecuted(), "FixAuditChangesV7=%v", fixV7)
		require.False(t, transfers[1].IsExecuted(), "FixAuditChangesV7=%v", fixV7)
		require.False(t, transfers[2].IsExecuted(), "FixAuditChangesV7=%v", fixV7)
	}
	require.Equal(t, errs[0], errs[1])
}

// A transfer with a correct royalty pays the royalty receiver, the same with the fork on as off.
func TestPayCreateTransfers_PaysARoyalty(t *testing.T) {
	t.Parallel()

	var receipts [][]*transaction.Transaction_Receipt
	for _, fixV7 := range []bool{false, true} {
		// A 10% transfer royalty on 50 is 5.
		w := newCreateWorld(t, fixV7, createKDA(), royaltyCreateKDA(), 1_000)
		require.NoError(t, w.pay(createTransfers(big.NewInt(50), 5)), "FixAuditChangesV7=%v", fixV7)

		require.Equal(t, int64(1_000-50-5), w.balance(createCaller, createSecondAsset), "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, int64(50), w.balance(createContract, createSecondAsset), "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, int64(5), w.balance(createRoyaltyReceiver, createSecondAsset), "FixAuditChangesV7=%v", fixV7)
		receipts = append(receipts, w.ctx.Receipts().Get())
	}
	require.Equal(t, receipts[0], receipts[1], "the same receipts with the fork on as off")
}
