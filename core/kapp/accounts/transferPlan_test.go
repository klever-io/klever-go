package accounts

import (
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
)

// These tests pin the FixAuditChangesV7 transfer (planTransfer then applyTransferPlan) to the
// legacy one (transferLegacy) it replaces. Both run on identical fixtures: a successful batch must
// leave the same state and the same receipts in the same order as the legacy transfers applied one
// after the other, and a failing one must report the same result code, error and error receipts
// while leaving no state change at all, where legacy leaves the transfers before the failing one
// applied.
//
// Legacy runs twice: through Transfer with FixAuditChangesV7 off, which is what blocks before the
// fork replay, and through transferLegacy under the batch's own forks, so a check the fork itself
// adds (a spent NFT record, KLC-2835) is held to the same order and outcome.

var (
	parityFungible     = []byte("FUNGI-1234")
	parityFungibleNFT  = []byte("NONFUNGI-1234")
	paritySemiFungible = []byte("SEMI-1234")
	parityNonce        = []byte("7")
	paritySender       = makeAddress("parity-sender")
	parityReceiver     = makeAddress("parity-receiver")
	parityRoyalty      = makeAddress("parity-royalty")
	paritySplit        = makeAddress("parity-split")
	paritySplitB       = makeAddress("parity-split-b")
	paritySplitC       = makeAddress("parity-split-c")
)

// parityWorld is one accountsKapp over live accounts, as the production cacher serves them with the
// cache on: every load of an address hands back the same instance.
type parityWorld struct {
	accKapp  *accountsKapp
	ctx      kapp.KappContext
	accounts map[string]state.UserAccountHandler
	// sender is who TestTransfer_SingleSendMatchesLegacy sends from.
	sender []byte
}

type parityScenario struct {
	name string
	cfg  config.EnableEpochs
	kda  func() *kapps.KDAData
	fund func(t *testing.T, sender state.UserAccountHandler)
	tcs  func() []*transaction.TransferContract
	// expectedErr is nil for a batch that succeeds.
	expectedErr error
	// sameForksOnly skips the comparison with Transfer before FixAuditChangesV7, for a scenario whose
	// outcome the fork itself changes.
	sameForksOnly bool
}

func newParityWorld(t *testing.T, cfg config.EnableEpochs, kda *kapps.KDAData, fund func(*testing.T, state.UserAccountHandler)) *parityWorld {
	t.Helper()

	emptyTrie := &commonMock.TrieStub{GetCalled: func([]byte) ([]byte, error) { return nil, nil }}
	w := &parityWorld{accounts: make(map[string]state.UserAccountHandler)}
	load := func(address []byte) (state.UserAccountHandler, error) {
		if acc, ok := w.accounts[string(address)]; ok {
			return acc, nil
		}
		acc, err := state.NewUserAccount(address)
		if err != nil {
			return nil, err
		}
		acc.SetDataTrie(emptyTrie)
		w.accounts[string(address)] = acc
		return acc, nil
	}

	sender, err := load(paritySender)
	require.NoError(t, err)
	fund(t, sender)
	_, err = load(parityReceiver)
	require.NoError(t, err)

	cacher := &commonMock.AccountsCacherStub{
		LoadUserCalled: load,
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			if acc, ok := w.accounts[string(address)]; ok {
				return acc, nil
			}
			return nil, common.ErrAccNotFound
		},
		UpdateUserCalled: func(state.AccountHandler) error { return nil },
	}

	w.ctx = kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: paritySender,
		ContractID:     transferContractID,
		ContractType:   transaction.TXContract_TransferContractType,
		Block:          &block.Block{},
	})
	controller := &kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return w.ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func([]byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, kda, nil
				},
			}
		},
	}

	w.accKapp = setupAccountsKapp(t, cfg)
	require.NoError(t, w.accKapp.SetAccountsCacher(cacher))
	require.NoError(t, w.accKapp.SetKAppController(controller))

	return w
}

// snapshot renders every account's holdings of the assets these tests move. Accounts that hold
// nothing are left out: loading one moves nothing.
func (w *parityWorld) snapshot() string {
	addresses := make([]string, 0, len(w.accounts))
	for address := range w.accounts {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	out := ""
	for _, address := range addresses {
		acc := w.accounts[address]
		nft, _ := acc.RetrieveValue(kdautils.ToKDAKey(parityFungibleNFT, parityNonce))
		holdings := fmt.Sprintf("klv=%d fungible=%d sft=%d sftNonceless=%d nft=%q",
			acc.GetBalance(kdautils.KLVIdentifier, true),
			acc.GetBalance(parityFungible, true),
			acc.GetBalanceWithNonce(paritySemiFungible, parityNonce, true),
			acc.GetBalance(paritySemiFungible, true),
			nft,
		)
		if holdings != `klv=0 fungible=0 sft=0 sftNonceless=0 nft=""` {
			out += fmt.Sprintf("%x %s\n", address, holdings)
		}
	}
	return out
}

func parityKDA(assetType kapps.KDAData_EnumAssetType, royalties *kapps.RoyaltiesData) func() *kapps.KDAData {
	return func() *kapps.KDAData {
		return &kapps.KDAData{
			AssetType:  assetType,
			Attributes: &kapps.AttributesData{},
			Properties: &kapps.PropertiesData{},
			Royalties:  royalties,
		}
	}
}

// parityPercentage is a 10% transfer royalty paid to parityRoyalty, with splitPercent of it split
// to parityRoyalty's split address.
func parityPercentage(splitPercent uint32) *kapps.RoyaltiesData {
	royalties := &kapps.RoyaltiesData{
		Address:            parityRoyalty,
		TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
	}
	if splitPercent > 0 {
		royalties.SplitRoyalties = map[string]*kapps.RoyaltySplitData{
			hex.EncodeToString(paritySplit): {PercentTransferPercentage: splitPercent},
		}
	}
	return royalties
}

func parityTransfer(assetID string, amount, kdaRoyalties, klvRoyalties int64) *transaction.TransferContract {
	return &transaction.TransferContract{
		ToAddress:    parityReceiver,
		AssetID:      []byte(assetID),
		Amount:       amount,
		KDARoyalties: kdaRoyalties,
		KLVRoyalties: klvRoyalties,
	}
}

func fundFungible(amount int64) func(*testing.T, state.UserAccountHandler) {
	return func(t *testing.T, acc state.UserAccountHandler) {
		require.NoError(t, acc.AddToBalance(amount, parityFungible, true))
	}
}

func fundKLV(amount int64) func(*testing.T, state.UserAccountHandler) {
	return func(t *testing.T, acc state.UserAccountHandler) {
		require.NoError(t, acc.AddToBalance(amount, kdautils.KLVIdentifier, true))
	}
}

func fundAll(funds ...func(*testing.T, state.UserAccountHandler)) func(*testing.T, state.UserAccountHandler) {
	return func(t *testing.T, acc state.UserAccountHandler) {
		for _, fund := range funds {
			fund(t, acc)
		}
	}
}

func parityScenarios() []parityScenario {
	fixMBOOff := config.EnableEpochs{FixMarketBuyOverflow: 1000}
	tenPercentOf := func(amount int64) int64 { return amount / 10 }

	return []parityScenario{
		{
			name: "fungible, percentage royalty with a split",
			kda:  parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent/2)),
			fund: fundFungible(1_000),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 400, tenPercentOf(400), 0),
					parityTransfer(string(parityFungible), 300, tenPercentOf(300), 0),
				}
			},
		},
		{
			// The second transfer is only covered by the royalty the first one pays back.
			name: "fixed royalty paid back to the sender",
			kda: parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{
				Address:       paritySender,
				TransferFixed: 10,
				SplitRoyalties: map[string]*kapps.RoyaltySplitData{
					hex.EncodeToString(paritySplit): {PercentTransferFixed: core.HundredPercent / 2},
				},
			}),
			fund: fundAll(fundFungible(1_000), fundKLV(15)),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 100, 0, 10),
					parityTransfer(string(parityFungible), 100, 0, 10),
				}
			},
		},
		{
			name: "KLV transfer covering its own fixed royalty",
			kda:  parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund: fundKLV(100),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{parityTransfer(string(kdautils.KLVIdentifier), 90, 0, 10)}
			},
		},
		{
			name:        "KLV transfer short of its own fixed royalty",
			kda:         parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund:        fundKLV(100),
			expectedErr: process.ErrInsufficientFunds,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{parityTransfer(string(kdautils.KLVIdentifier), 91, 0, 10)}
			},
		},
		{
			// Before FixMarketBuyOverflow a royalty fully taken by its splits is never charged, so
			// the second transfer fits the 600 the first leaves only if the first royalty is not
			// counted.
			name: "royalty fully split before FixMarketBuyOverflow",
			cfg:  fixMBOOff,
			kda:  parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent)),
			fund: fundFungible(1_000),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 400, tenPercentOf(400), 0),
					parityTransfer(string(parityFungible), 540, tenPercentOf(540), 0),
				}
			},
		},
		{
			// The 200% split (80) exceeds the royalty left to pay (40): the split check refuses it
			// with an ErrFieldInvalidRoyalties receipt.
			name:        "split larger than the royalty",
			kda:         parityKDA(kapps.KDAData_Fungible, parityPercentage(core.HundredPercent*2)),
			fund:        fundFungible(1_000),
			expectedErr: common.ErrInvalidValue,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{parityTransfer(string(parityFungible), 400, tenPercentOf(400), 0)}
			},
		},
		{
			// The nonce-less balance must cover amount plus royalty, and pays the royalty.
			name: "semi-fungible, royalty charged to the nonce-less balance",
			kda:  parityKDA(kapps.KDAData_SemiFungible, parityPercentage(0)),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddToBalanceWithNonce(1_000, paritySemiFungible, parityNonce, true))
				require.NoError(t, acc.AddToBalance(1_000, paritySemiFungible, true))
			},
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer("SEMI-1234/7", 400, tenPercentOf(400), 0),
					parityTransfer("SEMI-1234/7", 500, tenPercentOf(500), 0),
				}
			},
		},
		{
			// With no nonce the balance check reads the nonce-less balance, which covers the
			// amount, and SubFromBalanceWithNonce then rejects the missing nonce.
			name: "semi-fungible without a nonce",
			kda:  parityKDA(kapps.KDAData_SemiFungible, nil),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddToBalanceWithNonce(1_000, paritySemiFungible, parityNonce, true))
				require.NoError(t, acc.AddToBalance(100, paritySemiFungible, true))
			},
			expectedErr: fmt.Errorf("strconv.ParseInt: parsing \"\": invalid syntax"),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer("SEMI-1234/7", 400, 0, 0),
					parityTransfer("SEMI-1234", 50, 0, 0),
				}
			},
		},
		{
			name: "non-fungible with a fixed royalty",
			kda:  parityKDA(kapps.KDAData_NonFungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddInternalKDA(parityFungibleNFT, parityNonce, commonMock.NFTHolderRecord(1)))
				fundKLV(20)(t, acc)
			},
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{parityTransfer("NONFUNGI-1234/7", 1, 0, 10)}
			},
		},
		{
			name: "same non-fungible unit twice",
			kda:  parityKDA(kapps.KDAData_NonFungible, nil),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddInternalKDA(parityFungibleNFT, parityNonce, commonMock.NFTHolderRecord(1)))
			},
			expectedErr: common.ErrNegativeValue,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer("NONFUNGI-1234/7", 1, 0, 0),
					parityTransfer("NONFUNGI-1234/7", 1, 0, 0),
				}
			},
		},
		{
			// Splits are paid in sorted key order (KLC-2799), so the receipt order shows a plan
			// that walks the split map in any other order.
			name: "fungible, percentage royalty with three splits",
			kda: parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{
				Address:            parityRoyalty,
				TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
				SplitRoyalties: map[string]*kapps.RoyaltySplitData{
					hex.EncodeToString(paritySplitC): {PercentTransferPercentage: core.HundredPercent / 5},
					hex.EncodeToString(paritySplit):  {PercentTransferPercentage: core.HundredPercent / 4},
					hex.EncodeToString(paritySplitB): {PercentTransferPercentage: core.HundredPercent / 10},
				},
			}),
			fund: fundFungible(10_000),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 4_000, tenPercentOf(4_000), 0),
					parityTransfer(string(parityFungible), 3_000, tenPercentOf(3_000), 0),
				}
			},
		},
		{
			// From FixAuditChangesV7 a holder record that no longer holds the NFT is not sent
			// (KLC-2835). Legacy charges the fixed royalty before SubInternalKDA refuses the record;
			// the batch must refuse it before charging anything.
			name: "spent non-fungible record with a fixed royalty",
			kda:  parityKDA(kapps.KDAData_NonFungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddInternalKDA(parityFungibleNFT, parityNonce, commonMock.NFTHolderRecord(0)))
				fundKLV(20)(t, acc)
			},
			expectedErr:   common.ErrAssetNotFound,
			sameForksOnly: true,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{parityTransfer("NONFUNGI-1234/7", 1, 0, 10)}
			},
		},
		{
			// The second transfer and its royalty exceed what the first transfer and its royalty
			// leave. Legacy refuses it in the royalty check, with no error receipt; a plan that
			// ignored the first royalty would refuse it later, in the amount check, with one.
			name:        "percentage royalty, second transfer exceeds what the first leaves",
			kda:         parityKDA(kapps.KDAData_Fungible, parityPercentage(0)),
			fund:        fundFungible(1_000),
			expectedErr: process.ErrInsufficientFunds,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 500, tenPercentOf(500), 0),
					parityTransfer(string(parityFungible), 420, tenPercentOf(420), 0),
				}
			},
		},
		{
			name: "semi-fungible, second transfer exceeds what the first leaves",
			kda:  parityKDA(kapps.KDAData_SemiFungible, nil),
			fund: func(t *testing.T, acc state.UserAccountHandler) {
				require.NoError(t, acc.AddToBalanceWithNonce(1_000, paritySemiFungible, parityNonce, true))
			},
			expectedErr: process.ErrInsufficientFunds,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer("SEMI-1234/7", 600, 0, 0),
					parityTransfer("SEMI-1234/7", 600, 0, 0),
				}
			},
		},
		{
			// The second transfer only fits because the first transfer's percentage royalty is
			// paid back to the sender.
			name: "percentage royalty paid back to the sender",
			kda: parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{
				Address:            paritySender,
				TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
			}),
			fund: fundFungible(1_000),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 500, tenPercentOf(500), 0),
					parityTransfer(string(parityFungible), 450, tenPercentOf(450), 0),
				}
			},
		},
		{
			// Half of each percentage royalty is split back to the sender. The second transfer fits
			// the 475 the first leaves (1000 - 500 - 50 + 25) only with that credit.
			name: "percentage royalty split back to the sender",
			kda: parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{
				Address:            parityRoyalty,
				TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
				SplitRoyalties: map[string]*kapps.RoyaltySplitData{
					hex.EncodeToString(paritySender): {PercentTransferPercentage: core.HundredPercent / 2},
				},
			}),
			fund: fundFungible(1_000),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 500, tenPercentOf(500), 0),
					parityTransfer(string(parityFungible), 430, tenPercentOf(430), 0),
				}
			},
		},
		{
			// The first transfer's KLV fixed royalty comes out of the KLV the second transfer
			// sends: 10 + 90 + 10 is exactly the 110 KLV held.
			name: "fixed KLV royalty, then a KLV transfer",
			kda:  parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund: fundAll(fundFungible(1_000), fundKLV(110)),
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 100, 0, 10),
					parityTransfer(string(kdautils.KLVIdentifier), 90, 0, 10),
				}
			},
		},
		{
			name:        "fixed KLV royalty, then a KLV transfer one short",
			kda:         parityKDA(kapps.KDAData_Fungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10}),
			fund:        fundAll(fundFungible(1_000), fundKLV(109)),
			expectedErr: process.ErrInsufficientFunds,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 100, 0, 10),
					parityTransfer(string(kdautils.KLVIdentifier), 90, 0, 10),
				}
			},
		},
		{
			name:        "second transfer exceeds what the first one leaves",
			kda:         parityKDA(kapps.KDAData_Fungible, nil),
			fund:        fundFungible(1_000),
			expectedErr: process.ErrInsufficientFunds,
			tcs: func() []*transaction.TransferContract {
				return []*transaction.TransferContract{
					parityTransfer(string(parityFungible), 600, 0, 0),
					parityTransfer(string(parityFungible), 600, 0, 0),
				}
			},
		},
	}
}

// legacyRun is the outcome of applying a scenario's transfers one at a time.
type legacyRun struct {
	world *parityWorld
	code  transaction.Transaction_TXResultCode
	err   error
}

func runLegacy(t *testing.T, sc parityScenario, cfg config.EnableEpochs, transfer func(*accountsKapp, *transaction.TransferContract) (transaction.Transaction_TXResultCode, error)) legacyRun {
	run := legacyRun{world: newParityWorld(t, cfg, sc.kda(), sc.fund)}
	for _, tc := range sc.tcs() {
		run.code, run.err = transfer(run.world.accKapp, tc)
		if run.err != nil {
			break
		}
	}
	return run
}

func TestTransferBatch_MatchesLegacyTransfers(t *testing.T) {
	for _, sc := range parityScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			fixed := newParityWorld(t, sc.cfg, sc.kda(), sc.fund)
			require.True(t, fixed.accKapp.forkController.FixAuditChangesV7())
			initial := fixed.snapshot()
			code, err := fixed.accKapp.TransferBatch(transaction.TXContract_TransferContractType, paritySender, sc.tcs())

			// transferLegacy under the batch's own forks.
			runs := map[string]legacyRun{
				"transferLegacy": runLegacy(t, sc, sc.cfg, func(a *accountsKapp, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
					return a.transferLegacy(transaction.TXContract_TransferContractType, paritySender, tc)
				}),
			}
			if !sc.sameForksOnly {
				// Transfer before the fork: the two runs differ in FixAuditChangesV7 alone.
				legacyCfg := sc.cfg
				legacyCfg.FixAuditChangesV7 = 1000
				runs["Transfer before FixAuditChangesV7"] = runLegacy(t, sc, legacyCfg, func(a *accountsKapp, tc *transaction.TransferContract) (transaction.Transaction_TXResultCode, error) {
					require.False(t, a.forkController.FixAuditChangesV7())
					return a.Transfer(transaction.TXContract_TransferContractType, paritySender, tc)
				})
			}

			for name, legacy := range runs {
				if sc.expectedErr == nil {
					require.NoError(t, legacy.err, name)
					require.NoError(t, err)
					require.Equal(t, transaction.Transaction_Ok, code)
					require.NotEqual(t, initial, fixed.snapshot(), "the batch must move value")
					require.Equal(t, legacy.world.snapshot(), fixed.snapshot(), name)
					require.Equal(t, legacy.world.ctx.Receipts().Get(), fixed.ctx.Receipts().Get(), name)
					continue
				}

				require.EqualError(t, legacy.err, sc.expectedErr.Error(), name)
				require.Equal(t, legacy.err, err, name)
				require.Equal(t, legacy.code, code, name)
				require.Equal(t, legacy.world.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().GetPreserved(), name)
				require.Equal(t, fixed.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().Get(),
					"a rejected batch emits no transfer receipt")
				require.Equal(t, initial, fixed.snapshot(), "a rejected batch moves nothing")
			}
		})
	}
}

// A single Transfer is a batch of one from FixAuditChangesV7 on: an NFT the sender does not hold
// fails before its fixed royalty is charged. Before the fork the royalty is charged first, and the
// transaction-level reset is what undoes it.
func TestTransfer_ChecksBeforeMovingFromFixAuditChangesV7(t *testing.T) {
	kda := parityKDA(kapps.KDAData_NonFungible, &kapps.RoyaltiesData{Address: parityRoyalty, TransferFixed: 10})
	tc := parityTransfer("NONFUNGI-1234/7", 1, 0, 10)

	for _, fixV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV7=%v", fixV7), func(t *testing.T) {
			cfg := config.EnableEpochs{}
			if !fixV7 {
				cfg.FixAuditChangesV7 = 1000
			}
			w := newParityWorld(t, cfg, kda(), fundKLV(20))
			require.Equal(t, fixV7, w.accKapp.forkController.FixAuditChangesV7())

			code, err := w.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, tc)
			require.ErrorIs(t, err, common.ErrAssetNotFound)
			require.Equal(t, transaction.Transaction_BalanceError, code)

			expectedKLV := int64(10)
			if fixV7 {
				expectedKLV = 20
			}
			sender, err := w.accKapp.LoadUserAccount(paritySender)
			require.NoError(t, err)
			require.Equal(t, expectedKLV, sender.GetBalance(kdautils.KLVIdentifier, true))
		})
	}
}

// With the accounts cache off every load is a fresh copy. Each plan must be applied to accounts
// loaded after the plans before it were saved, or the later one would overwrite their debits.
func TestTransferBatch_CacheOffAppliesEveryTransfer(t *testing.T) {
	store := map[string]int64{string(paritySender): readOnlySenderStart}
	cacher, err := state.NewAccountsCacher(readOnlyCacherArgs(fundedUserAccountsAdapter(store)))
	require.NoError(t, err)
	cacher.ResetAll(false)

	accKapp := setupAccountsKapp(t, config.EnableEpochs{})
	require.NoError(t, accKapp.SetAccountsCacher(cacher))
	require.NoError(t, accKapp.SetKAppController(fungibleKDAController()))

	code, err := accKapp.TransferBatch(transaction.TXContract_TransferContractType, paritySender, []*transaction.TransferContract{
		readOnlyTransferContract(parityReceiver),
		readOnlyTransferContract(parityReceiver),
	})
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	require.Equal(t, readOnlySenderStart-2*readOnlyTransferValue, store[string(paritySender)],
		"the sender must be debited for both transfers")
	require.Equal(t, 2*readOnlyTransferValue, store[string(parityReceiver)])
}
