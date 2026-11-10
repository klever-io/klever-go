package builtInFunctions_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/kapp/ito"
	"github.com/klever-io/klever-go/core/process"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
)

const (
	buyGasCost     = uint64(100)
	buyGasProvided = uint64(1_000)
)

var (
	buyCurrencyID = []byte("RST-1234")
	buyITOID      = []byte("ITO-1234")
	buyContract   = makeBuyAddress("contract")
	buyReceiver   = makeBuyAddress("receiver")
	buyOther      = makeBuyAddress("other")
)

func makeBuyAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func buyInput(buyType transaction.BuyContract_EnumBuyType) *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  buyContract,
			GasProvided: buyGasProvided,
			Arguments: [][]byte{
				big.NewInt(int64(buyType)).Bytes(),
				buyITOID,
				buyCurrencyID,
				big.NewInt(1_000_000).Bytes(),
				big.NewInt(1_000_000).Bytes(),
			},
		},
	}
}

// kleverBuyResult is one kleverBuy of 1,000,000 units at a price of 1,000,000, called by buyContract.
type kleverBuyResult struct {
	vmOutput *vmcommon.VMOutput
	err      error
	debited  map[string]int64
	credited map[string]int64
}

// runKleverBuy drives the kleverBuy builtin against a real ITO kapp that sells for currency and
// pays buyReceiver.
func runKleverBuy(t *testing.T, currency *kapps.KDAData) kleverBuyResult {
	t.Helper()

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
	require.NoError(t, err)

	marshalizer := &mock.ProtoMarshalizerMock{}
	itoKapp, err := ito.NewITOKApp(&ito.ArgsNewITOKApp{
		Marshalizer:    marshalizer,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	})
	require.NoError(t, err)

	itoData, err := marshalizer.Marshal(&kapps.ITOData{
		IsActive:        true,
		StartTime:       1,
		EndTime:         100_00,
		ReceiverAddress: buyReceiver,
		PackData: map[string]*kapps.PackData{
			string(buyCurrencyID): {Packs: []*kapps.Pack{{Amount: 1_000_000, Price: 1_000_000}}},
		},
	})
	require.NoError(t, err)

	asset := &kapps.KDAData{Precision: 6, AssetType: kapps.KDAData_Fungible}

	result := kleverBuyResult{debited: make(map[string]int64), credited: make(map[string]int64)}
	account := func(address []byte) state.UserAccountHandler {
		return &mock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return address },
			SubFromBalanceCalled: func(value int64, assetID []byte, _ bool, _ ...*kapps.UserKDA) error {
				result.debited[string(address)] += value
				return nil
			},
			AddToBalanceCalled: func(value int64, assetID []byte, _ bool, _ ...*kapps.UserKDA) error {
				result.credited[string(address)] += value
				return nil
			},
		}
	}

	accountsCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) { return account(address), nil },
		LoadUserCalled:        func(address []byte) (state.UserAccountHandler, error) { return account(address), nil },
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				DataTrieTrackerCalled: func() state.DataTrieTracker {
					return &mock.DataTrieTrackerStub{
						RetrieveValueCalled: func(key []byte) ([]byte, error) { return itoData, nil },
						SaveKeyValueCalled:  func(key, value []byte) error { return nil },
					}
				},
			}, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error { return nil },
	}
	require.NoError(t, itoKapp.SetAccountsCacher(accountsCacher))

	kappController := &stub.KAppControllerStub{
		GetITOKAppCalled: func() kapp.ITOKapp { return itoKapp },
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				Block: &block.Block{Header: &block.BlockHeader{Timestamp: 100}},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					if string(assetID) == string(buyCurrencyID) {
						return nil, currency, nil
					}
					return nil, asset, nil
				},
				MintCalled: func(sender []byte, tc *transaction.AssetTriggerContract) (transaction.Transaction_TXResultCode, error) {
					return transaction.Transaction_Ok, nil
				},
			}
		},
	}
	require.NoError(t, itoKapp.SetKAppController(kappController))

	buyFunc, err := builtInFunctions.NewKleverBuyFunc(buyGasCost, 0, 0, marshalizer, accountsCacher, forkController, kappController)
	require.NoError(t, err)

	result.vmOutput, result.err = buyFunc.ProcessBuiltinFunction(buyInput(transaction.BuyContract_ITOBuy))

	return result
}

// TestKleverBuy_BuyerIsTheCallingContract pins which address the transfer restriction is checked for.
func TestKleverBuy_BuyerIsTheCallingContract(t *testing.T) {
	t.Parallel()

	limitTransfer := func(transferRole []byte) *kapps.KDAData {
		return &kapps.KDAData{
			ID:         buyCurrencyID,
			AssetType:  kapps.KDAData_Fungible,
			Attributes: &kapps.AttributesData{},
			Properties: &kapps.PropertiesData{LimitTransfer: true},
			Roles:      []*kapps.RolesData{{Address: transferRole, HasRoleTransfer: true}},
		}
	}

	t.Run("the transfer role on the calling contract pays", func(t *testing.T) {
		t.Parallel()

		buy := runKleverBuy(t, limitTransfer(buyContract))
		require.NoError(t, buy.err)
		require.NotNil(t, buy.vmOutput)
		assert.Equal(t, vmcommon.Ok, buy.vmOutput.ReturnCode)
		assert.Equal(t, map[string]int64{string(buyContract): 1_000_000}, buy.debited, "only the calling contract is debited")
		assert.Equal(t, map[string]int64{string(buyReceiver): 1_000_000}, buy.credited)
	})

	t.Run("the transfer role on another address does not cover the calling contract", func(t *testing.T) {
		t.Parallel()

		buy := runKleverBuy(t, limitTransfer(buyOther))
		require.ErrorIs(t, buy.err, process.ErrKDATransferNotAllowed)
		assert.Nil(t, buy.vmOutput, "a refused buy fails the contract call")
		assert.Empty(t, buy.debited)
		assert.Empty(t, buy.credited)
	})
}

// itoKappBuyStub answers Buy with a fixed result.
type itoKappBuyStub struct {
	kapp.ITOKapp
	sender []byte
	status transaction.Transaction_TXResultCode
	err    error
	onBuy  func()
}

func (s *itoKappBuyStub) Buy(sender []byte, _ *transaction.BuyContract) (transaction.Transaction_TXResultCode, error) {
	s.sender = sender
	if s.onBuy != nil {
		s.onBuy()
	}
	return s.status, s.err
}

func TestKleverBuy_RefusalFailsTheCall(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, itoKapp *itoKappBuyStub) (*vmcommon.VMOutput, error) {
		t.Helper()

		buyFunc, err := builtInFunctions.NewKleverBuyFunc(
			buyGasCost,
			0,
			0,
			&mock.ProtoMarshalizerMock{},
			&mock.AccountsCacherStub{},
			mock.NewForkControllerStub(),
			&stub.KAppControllerStub{
				GetITOKAppCalled: func() kapp.ITOKapp { return itoKapp },
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &stub.KDAKappStub{
						GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
					}
				},
			},
		)
		require.NoError(t, err)

		return buyFunc.ProcessBuiltinFunction(buyInput(transaction.BuyContract_ITOBuy))
	}

	t.Run("a refusal code without an error still fails the call", func(t *testing.T) {
		t.Parallel()

		itoKapp := &itoKappBuyStub{status: transaction.Transaction_AssetPaused}
		vmOutput, err := run(t, itoKapp)
		require.ErrorContains(t, err, transaction.Transaction_AssetPaused.String())
		assert.Nil(t, vmOutput)
		assert.Equal(t, buyContract, itoKapp.sender)
	})

	t.Run("a refusal error is returned as is", func(t *testing.T) {
		t.Parallel()

		itoKapp := &itoKappBuyStub{status: transaction.Transaction_KDATransferNotAllowed, err: process.ErrKDATransferNotAllowed}
		vmOutput, err := run(t, itoKapp)
		require.ErrorIs(t, err, process.ErrKDATransferNotAllowed)
		assert.Nil(t, vmOutput)
	})
}

// marketKappBuyStub accepts every Buy and Claim, and runs onCall for each.
type marketKappBuyStub struct {
	kapp.MarketKapp
	onCall func()
}

func (s *marketKappBuyStub) Buy(_ []byte, _ *transaction.BuyContract) (transaction.Transaction_TXResultCode, error) {
	if s.onCall != nil {
		s.onCall()
	}
	return transaction.Transaction_Ok, nil
}

func (s *marketKappBuyStub) Claim(_ []byte, _ *transaction.ClaimContract) (transaction.Transaction_TXResultCode, error) {
	if s.onCall != nil {
		s.onCall()
	}
	return transaction.Transaction_Ok, nil
}

// From FixAuditChangesV7 a buy pays per stored record only for an ITO buy of an NFT. A market buy
// pays the flat cost even when its ID loads as an NFT, and so does an ITO buy whose asset loads as
// no record. With only the flat cost provided, any charge would surface as ErrNotEnoughGas.
func TestKleverBuy_FlatCostWithoutNFTRecords(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, buyType transaction.BuyContract_EnumBuyType, asset *kapps.KDAData) (*vmcommon.VMOutput, error) {
		t.Helper()

		buyFunc, err := builtInFunctions.NewKleverBuyFunc(
			buyGasCost,
			10,
			0,
			&mock.ProtoMarshalizerMock{},
			&mock.AccountsCacherStub{},
			mock.NewForkControllerStub(),
			&stub.KAppControllerStub{
				GetITOKAppCalled:    func() kapp.ITOKapp { return &itoKappBuyStub{status: transaction.Transaction_Ok} },
				GetMarketKAppCalled: func() kapp.MarketKapp { return &marketKappBuyStub{} },
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &stub.KDAKappStub{
						GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, asset, nil
						},
					}
				},
				GetProposalControllerCalled: func() kapps.ActiveProposalController {
					return &mock.ProposalControllerStub{
						GetParameterIntCalled: func(_ kapps.EnumParameter) int64 { return 1_000_000 },
					}
				},
			},
		)
		require.NoError(t, err)

		input := buyInput(buyType)
		input.GasProvided = buyGasCost

		return buyFunc.ProcessBuiltinFunction(input)
	}

	t.Run("a market buy whose ID loads as an NFT", func(t *testing.T) {
		t.Parallel()

		vmOutput, err := run(t, transaction.BuyContract_MarketBuy, &kapps.KDAData{AssetType: kapps.KDAData_NonFungible})
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
	})

	t.Run("an ITO buy whose asset loads as no record", func(t *testing.T) {
		t.Parallel()

		vmOutput, err := run(t, transaction.BuyContract_ITOBuy, nil)
		require.NoError(t, err)
		require.Zero(t, vmOutput.GasRemaining)
	})
}

const (
	createdAccountCost = uint64(1_000)
	// the VM's part of createdAccountCost when it comes from the gas config
	transferValue = uint64(400)
	gasSlack      = uint64(7)
	// accounts the cacher created before the builtin runs; only the ones it creates during it count
	createdEarlier = uint64(5)
)

// createdAccountsCacher returns a cacher stub whose created-account counter starts at createdEarlier,
// and a create function that adds n to it, as a kapp's LoadUserAccount does for each missing payee.
func createdAccountsCacher(n uint64) (*mock.AccountsCacherStub, func()) {
	created := createdEarlier
	cacher := &mock.AccountsCacherStub{CreatedUsersCalled: func() uint64 { return created }}
	return cacher, func() { created += n }
}

// buyKAppController answers ITO and market calls with ito and market, and finds no asset, so an ITO
// buy pays no stored records.
func buyKAppController(ito *itoKappBuyStub, market *marketKappBuyStub) *stub.KAppControllerStub {
	return &stub.KAppControllerStub{
		GetITOKAppCalled:    func() kapp.ITOKapp { return ito },
		GetMarketKAppCalled: func() kapp.MarketKapp { return market },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &stub.KDAKappStub{
				GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, common.ErrNotFoundInKApp
				},
			}
		},
	}
}

// From FixAuditChangesV7 a contract's buy pays a transfer for each account the accounts cacher
// creates while the kapp runs, such as a royalty receiver's (KLC-2938). Before the fork it pays the
// flat cost only. One gas short of the charge fails the call after the kapp has run; the transaction
// then discards what the kapp wrote.
func TestKleverBuy_ChargesATransferPerCreatedAccountFromV7(t *testing.T) {
	t.Parallel()

	for _, buyType := range []transaction.BuyContract_EnumBuyType{transaction.BuyContract_ITOBuy, transaction.BuyContract_MarketBuy} {
		for _, created := range []uint64{0, 21} {
			for _, fixV7 := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/created=%d/FixAuditChangesV7=%t", buyType, created, fixV7), func(t *testing.T) {
					t.Parallel()

					cost := buyGasCost
					if fixV7 {
						cost += created * createdAccountCost
					}
					run := func(gasProvided uint64) (*vmcommon.VMOutput, error) {
						cacher, create := createdAccountsCacher(created)
						kappController := buyKAppController(
							&itoKappBuyStub{status: transaction.Transaction_Ok, onBuy: create}, &marketKappBuyStub{onCall: create})
						buyFunc, err := builtInFunctions.NewKleverBuyFunc(buyGasCost, 10, createdAccountCost, &mock.ProtoMarshalizerMock{},
							cacher, mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixV7), kappController)
						require.NoError(t, err)

						input := buyInput(buyType)
						input.GasProvided = gasProvided
						return buyFunc.ProcessBuiltinFunction(input)
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

// A ResetAll or SaveAll while the kapp runs restarts the cacher's count below where it was. The buy
// then pays for the accounts counted since the reset instead of a wrapped delta.
func TestKleverBuy_CreatedAccountsAfterACountReset(t *testing.T) {
	t.Parallel()

	created := createdEarlier
	cacher := &mock.AccountsCacherStub{CreatedUsersCalled: func() uint64 { return created }}
	resetThenCreateTwo := func() { created = 2 }
	buyFunc, err := builtInFunctions.NewKleverBuyFunc(buyGasCost, 10, createdAccountCost, &mock.ProtoMarshalizerMock{}, cacher,
		mock.NewForkControllerStub().SetFork("FixAuditChangesV7", true),
		buyKAppController(&itoKappBuyStub{status: transaction.Transaction_Ok, onBuy: resetThenCreateTwo}, &marketKappBuyStub{}))
	require.NoError(t, err)

	input := buyInput(transaction.BuyContract_ITOBuy)
	input.GasProvided = buyGasCost + 2*createdAccountCost + gasSlack
	vmOutput, err := buyFunc.ProcessBuiltinFunction(input)
	require.NoError(t, err)
	require.Equal(t, gasSlack, vmOutput.GasRemaining)
}

// The contract transfer a buy pays per created account, the VM's TransferValue and the Transfer
// builtin, follows the gas schedule, as the flat cost does.
func TestKleverBuy_CreatedAccountCostFollowsTheGasConfig(t *testing.T) {
	t.Parallel()

	cacher, create := createdAccountsCacher(4)
	buyFunc, err := builtInFunctions.NewKleverBuyFunc(buyGasCost, 10, 0, &mock.ProtoMarshalizerMock{}, cacher,
		mock.NewForkControllerStub().SetFork("FixAuditChangesV7", true),
		buyKAppController(&itoKappBuyStub{status: transaction.Transaction_Ok, onBuy: create}, &marketKappBuyStub{}))
	require.NoError(t, err)
	buyFunc.SetNewGasConfig(&vmcommon.GasCost{
		BuiltInCost:       vmcommon.BuiltInCost{Buy: buyGasCost, Transfer: createdAccountCost - transferValue},
		BaseOperationCost: vmcommon.BaseOperationCost{StorePerByte: 10},
		TransferValue:     transferValue,
	})

	input := buyInput(transaction.BuyContract_ITOBuy)
	input.GasProvided = buyGasCost + 4*createdAccountCost + gasSlack
	vmOutput, err := buyFunc.ProcessBuiltinFunction(input)
	require.NoError(t, err)
	require.Equal(t, gasSlack, vmOutput.GasRemaining)
}
