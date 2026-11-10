package builtInFunctions_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

const claimGasCost = uint64(100)

// accountsKappClaimStub accepts every staking claim, and runs onCall for each.
type accountsKappClaimStub struct {
	kapp.AccountsKapp
	onCall func()
}

func (s *accountsKappClaimStub) ClaimStaking(_ []byte, _ *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	if s.onCall != nil {
		s.onCall()
	}
	return transaction.Transaction_Ok, nil
}

func newClaimFunc(t *testing.T, transferCost uint64, fixV7 bool, cacher state.AccountsCacher, create func()) vmcommon.BuiltinFunction {
	t.Helper()
	kappController := buyKAppController(&itoKappBuyStub{}, &marketKappBuyStub{onCall: create})
	kappController.GetAccountsKAppCalled = func() kapp.AccountsKapp { return &accountsKappClaimStub{onCall: create} }
	claimFunc, err := builtInFunctions.NewKleverClaimFunc(claimGasCost, transferCost, &mock.ProtoMarshalizerMock{},
		cacher, mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixV7), kappController)
	require.NoError(t, err)
	return claimFunc
}

func claimInput(claimType transaction.ClaimContract_EnumClaimType, gasProvided uint64) *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  buyContract,
			GasProvided: gasProvided,
			Arguments:   [][]byte{big.NewInt(int64(claimType)).Bytes(), []byte("order-1")},
		},
	}
}

// From FixAuditChangesV7 a contract's claim pays a transfer for each account the accounts cacher
// creates while the kapp runs, such as the royalty receivers a market claim pays when it settles an
// expired auction (KLC-2938). Before the fork it pays the flat cost only. One gas short of the charge
// fails the call after the kapp has run; the transaction then discards what the kapp wrote.
func TestKleverClaim_ChargesATransferPerCreatedAccountFromV7(t *testing.T) {
	t.Parallel()

	for _, claimType := range []transaction.ClaimContract_EnumClaimType{transaction.ClaimContract_MarketClaim, transaction.ClaimContract_StakingClaim} {
		for _, created := range []uint64{0, 22} {
			for _, fixV7 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/created=%d/FixAuditChangesV7=%t", claimType, created, fixV7), func(t *testing.T) {
					t.Parallel()

					cost := claimGasCost
					if fixV7 {
						cost += created * createdAccountCost
					}
					run := func(gasProvided uint64) (*vmcommon.VMOutput, error) {
						cacher, create := createdAccountsCacher(created)
						return newClaimFunc(t, createdAccountCost, fixV7, cacher, create).ProcessBuiltinFunction(claimInput(claimType, gasProvided))
					}

					// a little more than the cost, so a charge that differs shows in what is left
					vmOutput, err := run(cost + gasSlack)
					require.NoError(t, err)
					require.Equal(t, gasSlack, vmOutput.GasRemaining)

					if !fixV7 || created == 0 {
						return
					}
					vmOutput, err = run(cost - 1)
					require.ErrorIs(t, err, common.ErrNotEnoughGas)
					require.Nil(t, vmOutput)
				})
			}
		}
	}
}

// The contract transfer a claim pays per created account, the VM's TransferValue and the Transfer
// builtin, follows the gas schedule, as the flat cost does.
func TestKleverClaim_CreatedAccountCostFollowsTheGasConfig(t *testing.T) {
	t.Parallel()

	cacher, create := createdAccountsCacher(4)
	claimFunc := newClaimFunc(t, 0, true, cacher, create)
	claimFunc.SetNewGasConfig(&vmcommon.GasCost{
		BuiltInCost:   vmcommon.BuiltInCost{Claim: claimGasCost, Transfer: createdAccountCost - transferValue},
		TransferValue: transferValue,
	})

	vmOutput, err := claimFunc.ProcessBuiltinFunction(
		claimInput(transaction.ClaimContract_MarketClaim, claimGasCost+4*createdAccountCost+gasSlack))
	require.NoError(t, err)
	require.Equal(t, gasSlack, vmOutput.GasRemaining)
}
