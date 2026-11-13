package builtInFunctions_test

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/accounts"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
)

// A multi-transfer settles each transfer straight into the accounts cacher, and there is no
// sub-transaction rollback (KLC-2686). These tests run the builtin against a real accounts kapp
// and read the accounts back without any transaction-level reset: once FixAuditChangesV7 is
// active, a transfer that fails must leave every earlier transfer of the same call unapplied
// (KLC-2688). Each case is also run with the fork off, to show the earlier transfer does apply
// there and to check that both paths report the same error and preserved receipts.

const (
	atomicGasCost       = uint64(10)
	atomicStartBalance  = int64(1_000)
	atomicFirstAmount   = int64(100)
	atomicFungibleA     = "FTA-0001"
	atomicFungibleB     = "FTB-0001"
	atomicPaused        = "PAUSED-0001"
	atomicLimited       = "LIMITED-0001"
	atomicNonFungible   = "NFT-0001"
	atomicSemiFungible  = "SFT-0001"
	atomicNFTNonce      = uint64(7)
	atomicNFTInternalID = "7"
	atomicContractID    = 2
)

// atomicNFTPayload is a holder record that holds the NFT, which SubInternalKDA requires from FixAuditChangesV7.
var atomicNFTPayload = mock.NFTHolderRecord(1)

// atomicCallerAddress is a contract address: only contracts reach the transfer builtin.
func atomicCallerAddress() []byte {
	addr := make([]byte, 32)
	copy(addr[core.NumInitCharactersForScAddress-core.VMTypeLen:], common.WasmVirtualMachine)
	copy(addr[core.NumInitCharactersForScAddress:], "atomic-caller")
	return addr
}

func atomicRecipientAddress() []byte {
	addr := make([]byte, 32)
	copy(addr, "atomic-recipient")
	return addr
}

type atomicFixture struct {
	builtin   vmcommon.BuiltinFunction
	caller    state.UserAccountHandler
	recipient state.UserAccountHandler
	ctx       kapp.KappContext
}

func newAtomicFixture(t *testing.T, fixV7 bool) *atomicFixture {
	t.Helper()

	fork := mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixV7)

	caller, err := state.NewUserAccount(atomicCallerAddress())
	require.NoError(t, err)
	recipient, err := state.NewUserAccount(atomicRecipientAddress())
	require.NoError(t, err)

	emptyTrie := &mock.TrieStub{GetCalled: func([]byte) ([]byte, error) { return nil, nil }}
	caller.SetDataTrie(emptyTrie)
	recipient.SetDataTrie(emptyTrie)

	for _, asset := range []string{atomicFungibleA, atomicFungibleB, atomicPaused, atomicLimited} {
		require.NoError(t, caller.AddToBalance(atomicStartBalance, []byte(asset), true))
	}
	require.NoError(t, caller.AddInternalKDA([]byte(atomicNonFungible), []byte(atomicNFTInternalID), atomicNFTPayload))
	// A nonce-less semi-fungible balance, which a transfer without a nonce reads.
	require.NoError(t, caller.AddToBalance(atomicStartBalance, []byte(atomicSemiFungible), true))

	live := map[string]state.UserAccountHandler{
		string(caller.AddressBytes()):    caller,
		string(recipient.AddressBytes()): recipient,
	}
	loadUser := func(address []byte) (state.UserAccountHandler, error) {
		acc, ok := live[string(address)]
		if !ok {
			return nil, fmt.Errorf("unexpected account load %x", address)
		}
		return acc, nil
	}
	cacher := &mock.AccountsCacherStub{
		LoadUserCalled:        loadUser,
		GetExistingUserCalled: loadUser,
		UpdateUserCalled:      func(state.AccountHandler) error { return nil },
	}

	assets := map[string]*kapps.KDAData{
		atomicFungibleA:    newAtomicKDA(kapps.KDAData_Fungible),
		atomicFungibleB:    newAtomicKDA(kapps.KDAData_Fungible),
		atomicPaused:       newAtomicKDA(kapps.KDAData_Fungible),
		atomicLimited:      newAtomicKDA(kapps.KDAData_Fungible),
		atomicNonFungible:  newAtomicKDA(kapps.KDAData_NonFungible),
		atomicSemiFungible: newAtomicKDA(kapps.KDAData_SemiFungible),
	}
	assets[atomicPaused].Attributes.IsPaused = true
	assets[atomicLimited].Properties.LimitTransfer = true

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: caller.AddressBytes(),
		ContractID:     atomicContractID,
		ContractType:   transaction.TXContract_SmartContractType,
		Block:          &block.Block{},
	})

	accountsKapp, err := accounts.NewAccountKApp(&accounts.ArgsNewAccountKApp{
		Marshalizer:    &mock.ProtoMarshalizerMock{},
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: fork,
	})
	require.NoError(t, err)

	controller := &stub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetAccountsKAppCalled:       func() kapp.AccountsKapp { return accountsKapp },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					kda, ok := assets[string(assetID)]
					if !ok {
						return nil, nil, fmt.Errorf("unexpected asset %s", assetID)
					}
					return nil, kda, nil
				},
			}
		},
	}
	require.NoError(t, accountsKapp.SetAccountsCacher(cacher))
	require.NoError(t, accountsKapp.SetKAppController(controller))

	builtin := builtInFunctions.NewKDATransferFunc(atomicGasCost, controller, fork)
	require.NoError(t, builtin.SetPayableChecker(&PayableHandlerStub{}))

	return &atomicFixture{builtin: builtin, caller: caller, recipient: recipient, ctx: ctx}
}

func newAtomicKDA(assetType kapps.KDAData_EnumAssetType) *kapps.KDAData {
	return &kapps.KDAData{
		AssetType:  assetType,
		Attributes: &kapps.AttributesData{},
		Properties: &kapps.PropertiesData{},
	}
}

func (f *atomicFixture) run(transfers []*vmcommon.KDATransfer) (*vmcommon.VMOutput, error) {
	return f.builtin.ProcessBuiltinFunction(&vmcommon.ContractCallInput{
		RecipientAddr: f.recipient.AddressBytes(),
		VMInput: vmcommon.VMInput{
			CallerAddr:   f.caller.AddressBytes(),
			GasProvided:  atomicGasCost * uint64(len(transfers)),
			KDATransfers: transfers,
		},
	})
}

func (f *atomicFixture) balance(account state.UserAccountHandler, asset string) int64 {
	return account.GetBalance([]byte(asset), true)
}

func (f *atomicFixture) ownsNFT(account state.UserAccountHandler) bool {
	data, err := account.RetrieveValue(kdaKey(atomicNonFungible, atomicNFTInternalID))
	return err == nil && len(data) > 0
}

func kdaKey(asset, internalID string) []byte {
	return []byte(kapps.KDAPrefix + kapps.Sp + asset + kapps.Sp + internalID)
}

func fungible(asset string, amount *big.Int) *vmcommon.KDATransfer {
	return &vmcommon.KDATransfer{KDATokenName: []byte(asset), KDAValue: amount}
}

func nft() *vmcommon.KDATransfer {
	return &vmcommon.KDATransfer{
		KDATokenName:  []byte(atomicNonFungible),
		KDATokenNonce: atomicNFTNonce,
		KDAValue:      big.NewInt(1),
	}
}

func TestKleverTransferMultiTransferIsAllOrNothing(t *testing.T) {
	t.Parallel()

	oversized := new(big.Int).Lsh(big.NewInt(1), 64)

	cases := []struct {
		name string
		// first is the transfer expected to be applied when the fork is off.
		first  func() *vmcommon.KDATransfer
		second func() *vmcommon.KDATransfer
		// firstApplied reports whether first has settled on the accounts.
		firstApplied func(f *atomicFixture) bool
		expectedErr  error
	}{
		{
			name:        "insufficient funds",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicFungibleB, big.NewInt(atomicStartBalance+1)) },
			expectedErr: process.ErrInsufficientFunds,
		},
		{
			name:        "paused asset",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicPaused, big.NewInt(1)) },
			expectedErr: process.ErrAssetIsPaused,
		},
		{
			name:        "transfer role denied",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicLimited, big.NewInt(1)) },
			expectedErr: process.ErrKDATransferNotAllowed,
		},
		{
			name:        "oversized amount",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicFungibleB, oversized) },
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:        "negative amount",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicFungibleB, big.NewInt(-1)) },
			expectedErr: common.ErrInvalidValue,
		},
		{
			// Each transfer fits the balance alone; together they exceed it.
			name:        "same asset twice exceeds the balance",
			first:       func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicStartBalance-1)) },
			second:      func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(2)) },
			expectedErr: process.ErrInsufficientFunds,
			firstApplied: func(f *atomicFixture) bool {
				return f.balance(f.recipient, atomicFungibleA) == atomicStartBalance-1
			},
		},
		{
			name:         "same non-fungible unit twice",
			first:        nft,
			second:       nft,
			expectedErr:  common.ErrNegativeValue,
			firstApplied: func(f *atomicFixture) bool { return f.ownsNFT(f.recipient) },
		},
		{
			// The nonce-less balance covers the amount; the missing nonce is what fails.
			name:  "semi-fungible without a nonce",
			first: func() *vmcommon.KDATransfer { return fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)) },
			second: func() *vmcommon.KDATransfer {
				return &vmcommon.KDATransfer{KDATokenName: []byte(atomicSemiFungible), KDAValue: big.NewInt(1)}
			},
			expectedErr: strconv.ErrSyntax,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			firstApplied := tc.firstApplied
			if firstApplied == nil {
				firstApplied = func(f *atomicFixture) bool {
					return f.balance(f.recipient, atomicFungibleA) == atomicFirstAmount
				}
			}

			legacy := newAtomicFixture(t, false)
			legacyFirst := tc.first()
			_, legacyErr := legacy.run([]*vmcommon.KDATransfer{legacyFirst, tc.second()})
			require.ErrorIs(t, legacyErr, tc.expectedErr)
			require.True(t, firstApplied(legacy), "without the fork the first transfer settles before the second fails")
			require.True(t, legacyFirst.IsExecuted())

			fixed := newAtomicFixture(t, true)
			first, second := tc.first(), tc.second()
			vmOutput, err := fixed.run([]*vmcommon.KDATransfer{first, second})
			require.Nil(t, vmOutput)
			require.ErrorIs(t, err, tc.expectedErr)
			require.Equal(t, legacyErr.Error(), err.Error(), "the fork must not change the reported error")

			require.False(t, firstApplied(fixed), "the first transfer must not settle when the second fails")
			for _, asset := range []string{atomicFungibleA, atomicFungibleB, atomicPaused, atomicLimited} {
				require.Equal(t, atomicStartBalance, fixed.balance(fixed.caller, asset), "caller %s", asset)
				require.Zero(t, fixed.balance(fixed.recipient, asset), "recipient %s", asset)
			}
			require.True(t, fixed.ownsNFT(fixed.caller))
			require.False(t, fixed.ownsNFT(fixed.recipient))
			require.False(t, first.IsExecuted())
			require.False(t, second.IsExecuted())

			require.Equal(t, fixed.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().Get(),
				"only error receipts, no transfer receipt for an unapplied transfer")
			require.Equal(t, legacy.ctx.Receipts().GetPreserved(), fixed.ctx.Receipts().GetPreserved(),
				"a failed transaction keeps the same error receipts with the fork on or off")
		})
	}
}

// Every transfer settles, and each one leaves its transfer log, in the order of the transfers, the
// same with the fork on as off.
func TestKleverTransferMultiTransferAppliesEveryTransferWhenAllAreValid(t *testing.T) {
	t.Parallel()

	logs := make(map[bool][]*vmcommon.LogEntry)
	for _, fixV7 := range []bool{false, true} {
		t.Run(fmt.Sprintf("FixAuditChangesV7=%v", fixV7), func(t *testing.T) {
			f := newAtomicFixture(t, fixV7)
			// The two FTA transfers spend the whole balance, which the batch tally must allow.
			transfers := []*vmcommon.KDATransfer{
				fungible(atomicFungibleA, big.NewInt(atomicStartBalance-atomicFirstAmount)),
				fungible(atomicFungibleB, big.NewInt(atomicFirstAmount)),
				fungible(atomicFungibleA, big.NewInt(atomicFirstAmount)),
				nft(),
			}

			vmOutput, err := f.run(transfers)
			require.NoError(t, err)
			require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)

			require.Zero(t, f.balance(f.caller, atomicFungibleA))
			require.Equal(t, atomicStartBalance, f.balance(f.recipient, atomicFungibleA))
			require.Equal(t, atomicStartBalance-atomicFirstAmount, f.balance(f.caller, atomicFungibleB))
			require.Equal(t, atomicFirstAmount, f.balance(f.recipient, atomicFungibleB))
			require.False(t, f.ownsNFT(f.caller))
			require.True(t, f.ownsNFT(f.recipient))
			for _, transfer := range transfers {
				require.True(t, transfer.IsExecuted())
			}
			require.Len(t, f.ctx.Receipts().Get(), len(transfers))

			require.Len(t, vmOutput.Logs, len(transfers), "one transfer log per transfer")
			for i, transfer := range transfers {
				topics := vmOutput.Logs[i].Topics
				require.Equal(t, builtInFunctions.GetTokenIdentifier(transfer), topics[0], "log %d token", i)
				require.Equal(t, transfer.KDAValue.Bytes(), topics[2], "log %d value", i)
			}
			logs[fixV7] = vmOutput.Logs
		})
	}
	require.Equal(t, logs[false], logs[true], "the same transfer logs with the fork on as off")
}

// A transfer the caller already settled is skipped by the apply loop, so validation must not count
// it against the balance either.
func TestKleverTransferMultiTransferSkipsExecutedTransfersInValidation(t *testing.T) {
	t.Parallel()

	f := newAtomicFixture(t, true)
	settled := fungible(atomicFungibleA, big.NewInt(atomicStartBalance))
	settled.SetExecuted()
	pending := fungible(atomicFungibleA, big.NewInt(atomicStartBalance))

	_, err := f.run([]*vmcommon.KDATransfer{settled, pending})
	require.NoError(t, err)
	require.Zero(t, f.balance(f.caller, atomicFungibleA))
	require.Equal(t, atomicStartBalance, f.balance(f.recipient, atomicFungibleA))
}
