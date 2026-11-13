package accounts

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// singleSend is one plain send: a TransferContract transaction, or a contract sending a KDA. From
// FixAuditChangesV7 Transfer is a TransferBatch of one.
type singleSend struct {
	name  string
	cfg   config.EnableEpochs
	cType transaction.TXContract_ContractType
	kda   func() *kapps.KDAData
	fund  func(*testing.T, state.UserAccountHandler)
	// setup runs on the world before the send, for state beyond the sender's funds.
	setup func(*testing.T, *parityWorld)
	tc    func() *transaction.TransferContract
	// expectedErr is nil for a send that succeeds.
	expectedErr error
	// fixOnly marks a send whose outcome FixAuditChangesV7 itself changes, so it is compared with
	// transferLegacy under the same forks only.
	fixOnly bool
	// failsWhileMoving marks a failure the checks cannot foresee, which happens after value moved.
	// Every other failure moves nothing from FixAuditChangesV7.
	failsWhileMoving bool
}

// overflowingRoyaltySend sends an amount whose 10% percentage royalty (parityPercentage) added to it
// overflows int64, declaring that royalty.
func overflowingRoyaltySend() *transaction.TransferContract {
	return parityTransfer(string(parityFungible), overflowingAmount, overflowingRoyalty(), 0)
}

// overflowingAmount is an amount whose 10% royalty added to it overflows int64.
const overflowingAmount = int64(math.MaxInt64 - 5)

func overflowingRoyalty() int64 {
	royalty, err := parityKDA(kapps.KDAData_Fungible, parityPercentage(0))().GetTransferRoyaltyByAmount(overflowingAmount, true, true)
	if err != nil {
		panic(err)
	}
	return royalty
}

func sendTo(tc *transaction.TransferContract, address []byte) *transaction.TransferContract {
	tc.ToAddress = address
	return tc
}

// parityContract is a smart-contract address: eight zero bytes, then the VM type.
func parityContract(name string) []byte {
	address := make([]byte, 32)
	copy(address[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	copy(address[core.NumInitCharactersForScAddress:], name)
	return address
}

func singleSends() []singleSend {
	fungible := parityKDA(kapps.KDAData_Fungible, nil)
	nonFungible := parityKDA(kapps.KDAData_NonFungible, nil)
	semiFungible := parityKDA(kapps.KDAData_SemiFungible, nil)
	fixedRoyalty := parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10})
	withAttributes := func(change func(*kapps.KDAData)) func() *kapps.KDAData {
		return func() *kapps.KDAData {
			kda := fungible()
			change(kda)
			return kda
		}
	}
	fundNFT := func(record []byte) func(*testing.T, state.UserAccountHandler) {
		return func(t *testing.T, acc state.UserAccountHandler) {
			require.NoError(t, acc.AddInternalKDA(parityFungibleNFT, parityNonce, record))
		}
	}
	fundSFT := func(t *testing.T, acc state.UserAccountHandler) {
		require.NoError(t, acc.AddToBalanceWithNonce(1_000, paritySemiFungible, parityNonce, true))
		require.NoError(t, acc.AddToBalance(1_000, paritySemiFungible, true))
	}
	send := func(assetID string, amount, kdaRoyalties, klvRoyalties int64) func() *transaction.TransferContract {
		return func() *transaction.TransferContract {
			return parityTransfer(assetID, amount, kdaRoyalties, klvRoyalties)
		}
	}
	klv := string(kdautils.KLVIdentifier)
	ft := string(parityFungible)

	return []singleSend{
		{name: "KLV", kda: fungible, fund: fundKLV(100), tc: send(klv, 60, 0, 0)},
		{name: "KLV, insufficient funds", kda: fungible, fund: fundKLV(100), tc: send(klv, 101, 0, 0), expectedErr: process.ErrInsufficientFunds},
		{name: "fungible", kda: fungible, fund: fundFungible(1_000), tc: send(ft, 1_000, 0, 0)},
		{name: "fungible, zero amount", kda: fungible, fund: fundFungible(1_000), tc: send(ft, 0, 0, 0), expectedErr: common.ErrInvalidValue},
		{name: "fungible, negative amount", kda: fungible, fund: fundFungible(1_000), tc: send(ft, -5, 0, 0), expectedErr: common.ErrInvalidValue},
		{name: "fungible, insufficient funds", kda: fungible, fund: fundFungible(1_000), tc: send(ft, 1_001, 0, 0), expectedErr: process.ErrInsufficientFunds},
		{
			name: "paused asset", kda: withAttributes(func(k *kapps.KDAData) { k.Attributes.IsPaused = true }),
			fund: fundFungible(1_000), tc: send(ft, 10, 0, 0), expectedErr: process.ErrAssetIsPaused,
		},
		{
			name: "transfer role denied", kda: withAttributes(func(k *kapps.KDAData) { k.Properties.LimitTransfer = true }),
			fund: fundFungible(1_000), tc: send(ft, 10, 0, 0), expectedErr: process.ErrKDATransferNotAllowed,
		},
		{
			name: "receiver address of the wrong length", kda: fungible, fund: fundFungible(1_000), expectedErr: process.ErrInvalidRcvAddr,
			tc: func() *transaction.TransferContract { return sendTo(parityTransfer(ft, 10, 0, 0), []byte("short")) },
		},
		{
			name: "sender is the receiver", kda: fungible, fund: fundFungible(1_000), expectedErr: process.ErrSameSenderAndReceiverAddress,
			tc: func() *transaction.TransferContract { return sendTo(parityTransfer(ft, 10, 0, 0), paritySender) },
		},
		{
			name: "transfer to an uninitialized contract", kda: fungible, fund: fundFungible(1_000), expectedErr: process.ErrContractAccountNotAllowed,
			tc: func() *transaction.TransferContract {
				return sendTo(parityTransfer(ft, 10, 0, 0), parityContract("uninitialized"))
			},
		},
		{
			name: "smart-contract payment to an uninitialized contract", cType: transaction.TXContract_SmartContractType,
			kda: fungible, fund: fundFungible(1_000),
			tc: func() *transaction.TransferContract {
				return sendTo(parityTransfer(ft, 10, 0, 0), parityContract("uninitialized"))
			},
		},
		{
			name: "receiver balance overflow", kda: fungible, fund: fundFungible(1_000), tc: send(ft, 10, 0, 0),
			expectedErr: state.ErrAddBalanceOverflow, failsWhileMoving: true,
			setup: func(t *testing.T, w *parityWorld) {
				receiver, err := w.accKapp.LoadUserAccount(parityReceiver)
				require.NoError(t, err)
				require.NoError(t, receiver.AddToBalance(math.MaxInt64, parityFungible, true))
			},
		},
		{name: "fixed royalty", kda: fixedRoyalty, fund: fundAll(fundFungible(1_000), fundKLV(10)), tc: send(ft, 100, 0, 10)},
		{name: "fixed royalty, wrong KLV royalty", kda: fixedRoyalty, fund: fundAll(fundFungible(1_000), fundKLV(10)), tc: send(ft, 100, 0, 9), expectedErr: common.ErrInvalidValue},
		{name: "fixed royalty, KLV short", kda: fixedRoyalty, fund: fundAll(fundFungible(1_000), fundKLV(9)), tc: send(ft, 100, 0, 10), expectedErr: process.ErrInsufficientFunds},
		{name: "fixed royalty, then the amount is short", kda: fixedRoyalty, fund: fundAll(fundFungible(50), fundKLV(10)), tc: send(ft, 100, 0, 10), expectedErr: process.ErrInsufficientFunds},
		{name: "percentage royalty with a split", kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent/2)), fund: fundFungible(1_000), tc: send(ft, 500, 50, 0)},
		{name: "percentage royalty, wrong KDA royalty", kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(0)), fund: fundFungible(1_000), tc: send(ft, 500, 49, 0), expectedErr: common.ErrInvalidValue},
		{name: "percentage royalty, amount plus royalty short", kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(0)), fund: fundFungible(1_000), tc: send(ft, 950, 95, 0), expectedErr: process.ErrInsufficientFunds},
		{
			// Amount plus royalty overflows int64, so the royalty check passes and SubFromBalance
			// refuses the royalty charge.
			name: "percentage royalty, amount plus royalty overflows", kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(0)),
			fund: fundFungible(5), tc: overflowingRoyaltySend, expectedErr: state.ErrInsufficientFunds,
		},
		{
			// The sender holds exactly the royalty: the charge passes, and the amount check then
			// refuses the transfer with an error receipt.
			name: "percentage royalty, amount plus royalty overflows, sender holds exactly the royalty",
			kda:  parityKDA(kapps.KDAData_Fungible, parityPercentage(0)), fund: fundFungible(overflowingRoyalty()),
			tc: overflowingRoyaltySend, expectedErr: process.ErrInsufficientFunds,
		},
		{
			name: "percentage royalty, amount plus royalty overflows, before FixMarketBuyOverflow", cfg: config.EnableEpochs{FixMarketBuyOverflow: 1000},
			kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(0)), fund: fundFungible(5), tc: overflowingRoyaltySend,
			expectedErr: state.ErrInsufficientFunds,
		},
		{
			name: "percentage royalty before FixMarketBuyOverflow", cfg: config.EnableEpochs{FixMarketBuyOverflow: 1000},
			kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent/2)), fund: fundFungible(1_000), tc: send(ft, 500, 50, 0),
		},
		{
			// Royalties are skipped for a contract sender.
			name: "percentage royalty asset sent by a contract", kda: parityKDA(kapps.KDAData_Fungible, parityPercentage(0)),
			fund: fundFungible(1_000), tc: send(ft, 500, 0, 0),
			setup: func(t *testing.T, w *parityWorld) {
				contract := parityContract("parity-sender")
				acc, err := w.accKapp.LoadUserAccount(contract)
				require.NoError(t, err)
				require.NoError(t, acc.AddToBalance(1_000, parityFungible, true))
				w.sender = contract
			},
		},
		{name: "NFT", kda: nonFungible, fund: fundNFT(commonMock.NFTHolderRecord(1)), tc: send("NONFUNGI-1234/7", 1, 0, 0)},
		{name: "NFT, amount other than one", kda: nonFungible, fund: fundNFT(commonMock.NFTHolderRecord(1)), tc: send("NONFUNGI-1234/7", 2, 0, 0), expectedErr: common.ErrInvalidValue},
		{name: "NFT not held", kda: nonFungible, fund: fundKLV(1), tc: send("NONFUNGI-1234/7", 1, 0, 0), expectedErr: common.ErrAssetNotFound},
		{name: "NFT spent record", kda: nonFungible, fund: fundNFT(commonMock.NFTHolderRecord(0)), tc: send("NONFUNGI-1234/7", 1, 0, 0), expectedErr: common.ErrAssetNotFound, fixOnly: true},
		{
			name: "NFT with a fixed royalty", kda: parityKDA(kapps.KDAData_NonFungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund: fundAll(fundNFT(commonMock.NFTHolderRecord(1)), fundKLV(10)), tc: send("NONFUNGI-1234/7", 1, 0, 10),
		},
		{name: "SFT", kda: semiFungible, fund: fundSFT, tc: send("SEMI-1234/7", 400, 0, 0)},
		{name: "SFT, insufficient funds at the nonce", kda: semiFungible, fund: fundSFT, tc: send("SEMI-1234/7", 1_001, 0, 0), expectedErr: process.ErrInsufficientFunds},
		{name: "SFT, nonce zero", kda: semiFungible, fund: fundSFT, tc: send("SEMI-1234/0", 10, 0, 0), expectedErr: state.ErrInvalidNonce},
		{name: "SFT with a percentage royalty", kda: parityKDA(kapps.KDAData_SemiFungible, parityPercentage(0)), fund: fundSFT, tc: send("SEMI-1234/7", 400, 40, 0)},
		{
			name: "SFT before EnableSmartContracts", cfg: config.EnableEpochs{SmartContracts: 1000},
			kda: semiFungible, fund: fundSFT, tc: send("SEMI-1234", 10, 0, 0), expectedErr: common.ErrAssetTypeInvalid,
		},
	}
}

type sendOutcome struct {
	world *parityWorld
	// initial is the world's holdings just before the send.
	initial string
	code    transaction.Transaction_TXResultCode
	err     error
}

// TestTransfer_SingleSendMatchesLegacy checks every plain send gives the same outcome from
// FixAuditChangesV7 as before it. A failed send is compared on what the transaction keeps when it
// fails: the result code, the error and the preserved receipts (txProcessor resets the accounts and
// drops every other receipt). A successful send is compared on the state and every receipt.
func TestTransfer_SingleSendMatchesLegacy(t *testing.T) {
	for _, sc := range singleSends() {
		t.Run(sc.name, func(t *testing.T) {
			cType := sc.cType
			if cType == 0 {
				cType = transaction.TXContract_TransferContractType
			}

			run := func(cfg config.EnableEpochs, transfer func(*parityWorld, *transaction.TransferContract) (transaction.Transaction_TXResultCode, error)) sendOutcome {
				w := newParityWorld(t, cfg, sc.kda(), sc.fund)
				w.sender = paritySender
				if sc.setup != nil {
					sc.setup(t, w)
				}
				initial := w.snapshot()
				code, err := transfer(w, sc.tc())
				return sendOutcome{world: w, initial: initial, code: code, err: err}
			}

			fixed := run(sc.cfg, func(w *parityWorld, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
				require.True(t, w.accKapp.forkController.FixAuditChangesV7())
				return w.accKapp.Transfer(cType, w.sender, tc)
			})
			if sc.expectedErr == nil {
				require.NoError(t, fixed.err)
				require.Equal(t, transaction.Transaction_Ok, fixed.code)
			} else {
				require.ErrorIs(t, fixed.err, sc.expectedErr)
				if !sc.failsWhileMoving {
					// A failed check moves nothing, not even a royalty checked before it.
					require.Equal(t, fixed.initial, fixed.world.snapshot(), "a refused send moves nothing")
					require.Equal(t, fixed.world.ctx.Receipts().GetPreserved(), fixed.world.ctx.Receipts().Get(),
						"a refused send emits no transfer receipt")
				}
			}

			references := map[string]sendOutcome{
				"transferLegacy": run(sc.cfg, func(w *parityWorld, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
					return w.accKapp.transferLegacy(cType, w.sender, tc)
				}),
			}
			if !sc.fixOnly {
				legacyCfg := sc.cfg
				legacyCfg.FixAuditChangesV7 = 1000
				references["Transfer before FixAuditChangesV7"] = run(legacyCfg, func(w *parityWorld, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
					require.False(t, w.accKapp.forkController.FixAuditChangesV7())
					return w.accKapp.Transfer(cType, w.sender, tc)
				})
			}

			for name, ref := range references {
				require.Equal(t, ref.code, fixed.code, name)
				require.Equal(t, ref.err, fixed.err, name)
				require.Equal(t, ref.world.ctx.Receipts().GetPreserved(), fixed.world.ctx.Receipts().GetPreserved(), name)
				if sc.expectedErr == nil {
					require.Equal(t, ref.world.snapshot(), fixed.world.snapshot(), name)
					require.Equal(t, ref.world.ctx.Receipts().Get(), fixed.world.ctx.Receipts().Get(), name)
				}
			}
		})
	}
}

// Before FixAuditChangesV7 a spent NFT record is still sent, through Transfer as through
// transferLegacy: KLC-2835 refuses it only from the fork.
func TestTransfer_SpentNFTRecordIsSentBeforeFixAuditChangesV7(t *testing.T) {
	spent := commonMock.NFTHolderRecord(0)
	w := newParityWorld(t, config.EnableEpochs{FixAuditChangesV7: 1000}, parityKDA(kapps.KDAData_NonFungible, nil)(),
		func(t *testing.T, acc state.UserAccountHandler) {
			require.NoError(t, acc.AddInternalKDA(parityFungibleNFT, parityNonce, spent))
		})
	require.False(t, w.accKapp.forkController.FixAuditChangesV7())

	code, err := w.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, parityTransfer("NONFUNGI-1234/7", 1, 0, 0))
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	key := kdautils.ToKDAKey(parityFungibleNFT, parityNonce)
	// A key cleared in this block reads back as a negative value.
	senderRecord, err := w.accounts[string(paritySender)].RetrieveValue(key)
	require.ErrorIs(t, err, common.ErrNegativeValue, "the record leaves the sender")
	require.Empty(t, senderRecord)
	receiverRecord, err := w.accounts[string(parityReceiver)].RetrieveValue(key)
	require.NoError(t, err)
	require.Equal(t, spent, receiverRecord, "the receiver gets the record as it was")
}

// A split larger than the royalty left to pay is refused with an InvalidRoyalties error receipt,
// with the fork on as off. The comparisons with transferLegacy cannot show this receipt going
// missing, since both paths share planSplitRoyalty.
func TestTransfer_SplitLargerThanTheRoyaltyEmitsAnInvalidRoyaltiesReceipt(t *testing.T) {
	for _, fixV7 := range []bool{false, true} {
		cfg := config.EnableEpochs{}
		if !fixV7 {
			cfg.FixAuditChangesV7 = 1000
		}
		// The 200% split (80) exceeds the royalty left to pay (40).
		w := newParityWorld(t, cfg, parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent*2))(), fundFungible(1_000))
		code, err := w.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, parityTransfer(string(parityFungible), 400, 40, 0))
		require.ErrorIs(t, err, common.ErrInvalidValue, "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code, "FixAuditChangesV7=%v", fixV7)

		preserved := w.ctx.Receipts().GetPreserved()
		require.Len(t, preserved, 1, "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, []byte(common.ErrFieldInvalidRoyalties), preserved[0].Data[1], "FixAuditChangesV7=%v", fixV7)
	}
}

// The one place a single send reports a different error from FixAuditChangesV7: a move that fails
// (the royalty receiver's credit overflows) ahead of a check that fails (the sender cannot cover
// the amount). Legacy moves value step by step and fails on the overflow; the batch makes every
// check first and fails on the amount, moving nothing. Both fail the transaction.
func TestTransfer_MoveFailureAheadOfAFailingCheckReportsTheCheck(t *testing.T) {
	kda := parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10})
	tc := func() *transaction.TransferContract { return parityTransfer(string(parityFungible), 100, 0, 10) }
	world := func(cfg config.EnableEpochs) *parityWorld {
		// KLV for the royalty, none of the asset sent; the royalty receiver is one credit from overflow.
		w := newParityWorld(t, cfg, kda(), fundKLV(10))
		royalty, err := w.accKapp.LoadUserAccount(parityRoyalty)
		require.NoError(t, err)
		require.NoError(t, royalty.AddToBalance(math.MaxInt64, kdautils.KLVIdentifier, true))
		return w
	}

	legacy := world(config.EnableEpochs{FixAuditChangesV7: 1000})
	code, err := legacy.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, tc())
	require.ErrorIs(t, err, state.ErrAddBalanceOverflow)
	require.Equal(t, transaction.Transaction_BalanceError, code)

	fixed := world(config.EnableEpochs{})
	initial := fixed.snapshot()
	code, err = fixed.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, tc())
	require.ErrorIs(t, err, process.ErrInsufficientFunds)
	require.Equal(t, transaction.Transaction_OutOfFunds, code)
	require.Equal(t, initial, fixed.snapshot(), "the batch moves nothing")
}
