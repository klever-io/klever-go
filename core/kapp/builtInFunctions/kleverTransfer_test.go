package builtInFunctions_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/vmcommon"
)

func TestInstantiationKdaTransferStruct(t *testing.T) {
	funcGasCost := uint64(100)
	kdaTransferInstance := builtInFunctions.NewKDATransferFunc(
		funcGasCost,
		&mock.KappsControllerMock{},
		mock.NewForkControllerStub(),
	)

	val := reflect.ValueOf(kdaTransferInstance).Elem()

	funcGasCostField := val.FieldByName("funcGasCost")
	require.True(t, funcGasCostField.IsValid(), "funcGasCost field should be valid")
	require.Equal(t, funcGasCost, funcGasCostField.Uint(), "funcGasCost should be set correctly")

	payableHandlerField := val.FieldByName("payableHandler")
	require.True(t, payableHandlerField.IsValid(), "payableHandler field should be valid")
	payableHandler := payableHandlerField.Elem()
	require.False(t, payableHandler.IsNil(), "payableHandler should not be nil")

	kappControllerField := val.FieldByName("kappController")
	require.True(t, kappControllerField.IsValid(), "kappController field should be valid")
	actualKappController := kappControllerField.Elem()
	require.False(
		t,
		actualKappController.IsNil(),
		"kappController should be set correctly",
	)
}

func TestKdaTransferStructIsNilMethod(t *testing.T) {
	kdaTransferInstance := builtInFunctions.NewKDATransferFunc(
		0,
		&mock.KappsControllerMock{},
		mock.NewForkControllerStub(),
	)

	require.False(t, kdaTransferInstance.IsInterfaceNil())
}

func TestGetTokenIdentifier(t *testing.T) {
	tokenName := "TTS"
	tiBytes := []byte(tokenName)

	t.Run("with nonce 0 (fungible token)", func(t *testing.T) {
		identifier := builtInFunctions.GetTokenIdentifier(&vmcommon.KDATransfer{
			KDATokenName:  tiBytes,
			KDATokenNonce: 0,
		})

		require.True(t, bytes.Equal(tiBytes, identifier))
	})

	t.Run("with nonce greater than 0 (non/semi fungible token)", func(t *testing.T) {
		nonce := 8
		identifier := builtInFunctions.GetTokenIdentifier(&vmcommon.KDATransfer{
			KDATokenName:  tiBytes,
			KDATokenNonce: uint64(nonce),
		})

		expectedIdentifier := []byte(fmt.Sprintf("%s%s%d", tokenName, "/", nonce))

		require.True(t, bytes.Equal(expectedIdentifier, identifier))
	})
}

type PayableHandlerStub struct {
	CheckPayableCalled   func(vmInput *vmcommon.ContractCallInput, dstAddress []byte, minLenArguments int) error
	IsInterfaceNilCalled func() bool
}

func (s *PayableHandlerStub) CheckPayable(
	vmInput *vmcommon.ContractCallInput,
	dstAddress []byte,
	minLenArguments int,
) error {
	if s.CheckPayableCalled != nil {
		return s.CheckPayableCalled(vmInput, dstAddress, minLenArguments)
	}

	return nil
}

func (s *PayableHandlerStub) IsInterfaceNil() bool {
	if s.IsInterfaceNilCalled != nil {
		return s.IsInterfaceNilCalled()
	}

	return s == nil
}

func vmInputTransferCreation(
	transfers []*vmcommon.KDATransfer,
	gasCost uint64,
) *vmcommon.ContractCallInput {
	return &vmcommon.ContractCallInput{
		RecipientAddr: []byte("receiver_address"),
		VMInput: vmcommon.VMInput{
			CallerAddr:   []byte("caller_address"),
			GasProvided:  gasCost,
			KDATransfers: transfers,
		},
	}
}

func TestKleverTransferProcessBuiltinFunction(t *testing.T) {
	funcGasCost := uint64(1000)

	transferExecuted := &vmcommon.KDATransfer{}
	transferExecuted.SetExecuted()

	cases := []struct {
		name                string
		kdaTransferInstance vmcommon.BuiltinFunction
		mock                func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput
		transfers           []*vmcommon.KDATransfer
		expectedError       error
	}{
		{
			name: "nil ContractCallInput",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&mock.KappsControllerMock{},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				return nil
			},
			expectedError: builtInFunctions.ErrNilVmInput,
		},
		{
			name: "input not payable",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&mock.KappsControllerMock{},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{
					CheckPayableCalled: func(vmInput *vmcommon.ContractCallInput, destAddr []byte, minLenArgs int) error {
						return builtInFunctions.ErrAccountNotPayable
					},
				}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
			expectedError: builtInFunctions.ErrAccountNotPayable,
			transfers: []*vmcommon.KDATransfer{
				{
					KDAValue: big.NewInt(100),
				},
			},
		},
		{
			name: "transfer kapp returns error",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_AccountError, fmt.Errorf(
									"transfer error",
								)
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTError"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
			},
			expectedError: fmt.Errorf("transfer error"),
		},
		{
			name:          "transfer kapp returns transaction code not Ok",
			expectedError: fmt.Errorf("KDA Transfer error: %s", transaction.Transaction_AccountError.String()),
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_AccountError, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTCodeNotOk"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
			},
		},
		{
			name: "transfer with negative value",
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTNeg"),
					KDAValue:      big.NewInt(-1),
					KDATokenNonce: 0,
				},
			},
			expectedError: common.ErrInvalidValue,
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&mock.KappsControllerMock{},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
		},
		{
			// KLR-02: an oversized value (2^64) must be rejected, not truncated to 0 and silently
			// skipped while still reaching the recipient contract as call value.
			name: "transfer with oversized value",
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTOversized"),
					KDAValue:      new(big.Int).Lsh(big.NewInt(1), 64),
					KDATokenNonce: 0,
				},
			},
			expectedError: common.ErrInvalidValue,
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&mock.KappsControllerMock{},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
		},
		{
			// KLR-02: a nil value is not a valid zero transfer; it must be rejected.
			name: "transfer with nil value",
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTNilValue"),
					KDAValue:      nil,
					KDATokenNonce: 0,
				},
			},
			expectedError: common.ErrInvalidValue,
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&mock.KappsControllerMock{},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
		},
		{
			name: "not enough gas provided",
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTSucess"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
				{
					KDATokenName:  []byte("SFTOrNFTSucess"),
					KDAValue:      big.NewInt(10),
					KDATokenNonce: 8,
				},
			},
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_Ok, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
			expectedError: common.ErrNotEnoughGas,
		},
		{
			name: "successful transfers",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_Ok, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost*uint64(len(transfers)))
			},
			transfers: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("FTSucess"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
				{
					KDATokenName:  []byte("SFTOrNFTSucess"),
					KDAValue:      big.NewInt(1),
					KDATokenNonce: 8,
				},
				{
					KDATokenName:  []byte("FTZeroAmount"),
					KDAValue:      big.NewInt(0),
					KDATokenNonce: 0,
				},
			},
		},
		{
			name: "successful transfers with executed transfers",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_Ok, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost*uint64(len(transfers)-1))
			},
			transfers: []*vmcommon.KDATransfer{
				transferExecuted,
				{
					KDATokenName:  []byte("FTSucess"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
				{
					KDATokenName:  []byte("SFTOrNFTSucess"),
					KDAValue:      big.NewInt(1),
					KDATokenNonce: 8,
				},
				{
					KDATokenName:  []byte("FTZeroAmount"),
					KDAValue:      big.NewInt(0),
					KDATokenNonce: 0,
				},
			},
		},
		{
			name: "successful transfers with all executed",
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_Ok, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost*0)
			},
			transfers: []*vmcommon.KDATransfer{
				transferExecuted,
				transferExecuted,
			},
		},
		{
			name:          "not enought gas with executed transfers",
			expectedError: common.ErrNotEnoughGas,
			kdaTransferInstance: builtInFunctions.NewKDATransferFunc(
				funcGasCost,
				&stub.KAppControllerStub{
					GetAccountsKAppCalled: func() kapp.AccountsKapp {
						return &stub.KAppAccountsStub{
							TransferCalled: func(
								cType transaction.TXContract_ContractType,
								sender []byte,
								tc *transaction.TransferContract,
							) (transaction.Transaction_TXResultCode, error) {
								return transaction.Transaction_Ok, nil
							},
						}
					},
				},
				mock.NewForkControllerStub(),
			),
			mock: func(kdaTransferInstance vmcommon.AcceptPayableChecker, transfers []*vmcommon.KDATransfer) *vmcommon.ContractCallInput {
				require.NoError(t, kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{}))

				return vmInputTransferCreation(transfers, funcGasCost)
			},
			transfers: []*vmcommon.KDATransfer{
				transferExecuted,
				transferExecuted,
				{
					KDATokenName:  []byte("FTSucess"),
					KDAValue:      big.NewInt(1000),
					KDATokenNonce: 0,
				},
				{
					KDATokenName:  []byte("SFTOrNFTSucess"),
					KDAValue:      big.NewInt(1),
					KDATokenNonce: 8,
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			vmInput := tt.mock(tt.kdaTransferInstance.(vmcommon.AcceptPayableChecker), tt.transfers)
			vmOutput, err := tt.kdaTransferInstance.ProcessBuiltinFunction(vmInput)

			if tt.expectedError != nil {
				req.Nil(vmOutput, "vmOutput should be nil")
				req.Error(err, "kleverTransfer ProcessBuiltinFunction should return not nil error")
				req.Equal(tt.expectedError.Error(), err.Error())
				return
			}

			// gas remaining should be zero
			req.Equal(uint64(0), vmOutput.GasRemaining)
			req.NotNil(vmOutput, "vmOutput should not be nil")
			req.NoError(err, "kleverTransfer ProcessBuiltinFunction should not return error")
			for _, transfer := range tt.transfers {
				req.True(
					transfer.IsExecuted(),
					"transfer of %s token should be marked as executed",
					transfer.KDATokenName,
				)
			}
		})
	}
}

func TestKleverTransferSetNewGasConfig(t *testing.T) {
	initialFuncGasCost := uint64(100)
	kdaTransferInstance := builtInFunctions.NewKDATransferFunc(
		initialFuncGasCost,
		&mock.KappsControllerMock{},
		mock.NewForkControllerStub(),
	)
	newFuncGasCost := uint64(20)
	t.Run("Setting new gas correctly", func(t *testing.T) {
		gasCost := &vmcommon.GasCost{
			BuiltInCost: vmcommon.BuiltInCost{
				Transfer: newFuncGasCost,
			},
		}

		kdaTransferInstance.SetNewGasConfig(gasCost)

		kdaValue := reflect.ValueOf(kdaTransferInstance).Elem()
		funcGasCostField := kdaValue.FieldByName("funcGasCost")

		require.True(t, funcGasCostField.IsValid(), "funcGasCost field should be valid")

		updatedFuncGasCost := funcGasCostField.Uint()
		require.Equal(t, newFuncGasCost, updatedFuncGasCost)
	})

	t.Run("Setting new gas with nil GasCost", func(t *testing.T) {
		kdaTransferInstance.SetNewGasConfig(nil)

		kdaValue := reflect.ValueOf(kdaTransferInstance).Elem()
		funcGasCostField := kdaValue.FieldByName("funcGasCost")

		require.True(t, funcGasCostField.IsValid(), "funcGasCost field should be valid")

		funcGasCost := funcGasCostField.Uint()
		require.Equal(t, funcGasCost, newFuncGasCost)
	})
}

func TestKleverTransferSetPayableChecker(t *testing.T) {
	funcGasCost := uint64(100)
	kdaTransferInstance := builtInFunctions.NewKDATransferFunc(
		funcGasCost,
		&mock.KappsControllerMock{},
		mock.NewForkControllerStub(),
	)

	err := kdaTransferInstance.SetPayableChecker(nil)
	require.Equal(
		t,
		builtInFunctions.ErrNilPayableHandler,
		err,
		"Expected ErrNilPayableHandler when payableHandler is nil",
	)

	err = kdaTransferInstance.SetPayableChecker(&PayableHandlerStub{})
	require.NoError(t, err, "Expected no error when payableHandler is valid")

	val := reflect.ValueOf(kdaTransferInstance).Elem()
	payableHandlerField := val.FieldByName("payableHandler")
	require.True(t, payableHandlerField.IsValid(), "payableHandler field should be valid")
	require.False(t, payableHandlerField.IsNil(), "payableHandler should not be nil")
}

// TestGetValidatedTransferValue is the KLR-02 regression for the post-fork value extraction boundary:
// out-of-range, negative and nil values must be rejected instead of silently truncated by Int64().
func TestGetValidatedTransferValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		transfer    *vmcommon.KDATransfer
		expected    int64
		expectedErr error
	}{
		{
			name:        "nil transfer",
			transfer:    nil,
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:        "nil value",
			transfer:    &vmcommon.KDATransfer{KDAValue: nil},
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:        "negative value",
			transfer:    &vmcommon.KDATransfer{KDAValue: big.NewInt(-1)},
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:        "2^64 is rejected, not truncated to 0",
			transfer:    &vmcommon.KDATransfer{KDAValue: new(big.Int).Lsh(big.NewInt(1), 64)},
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:        "2^64+1 is rejected, not truncated to 1",
			transfer:    &vmcommon.KDATransfer{KDAValue: new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))},
			expectedErr: common.ErrInvalidValue,
		},
		{
			name:     "zero value is accepted",
			transfer: &vmcommon.KDATransfer{KDAValue: big.NewInt(0)},
			expected: 0,
		},
		{
			name:     "math.MaxInt64 is accepted and exact",
			transfer: &vmcommon.KDATransfer{KDAValue: big.NewInt(math.MaxInt64)},
			expected: math.MaxInt64,
		},
		{
			name:     "regular value is accepted",
			transfer: &vmcommon.KDATransfer{KDAValue: big.NewInt(1000)},
			expected: 1000,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			value, err := builtInFunctions.GetValidatedTransferValue(tt.transfer)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Zero(t, value)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, value)
		})
	}
}

// TestKleverTransferOversizedValueForkGating pins the consensus-safety boundary of the KLR-02 fix:
// before FixAuditChangesV3 the oversized value keeps its legacy behaviour (truncated to 0, real
// transfer skipped, forwarded value left untouched), and only after the fork is it rejected. This is
// what keeps historical block replay deterministic across the activation epoch.
func TestKleverTransferOversizedValueForkGating(t *testing.T) {
	t.Parallel()

	funcGasCost := uint64(100)
	oversized := new(big.Int).Lsh(big.NewInt(1), 64) // 2^64

	newInstance := func(forkEnabled bool, accountTransferCalled *bool) vmcommon.BuiltinFunction {
		fork := mock.NewForkControllerStub().SetFork("FixAuditChangesV3", forkEnabled)
		instance := builtInFunctions.NewKDATransferFunc(
			funcGasCost,
			&stub.KAppControllerStub{
				GetAccountsKAppCalled: func() kapp.AccountsKapp {
					return &stub.KAppAccountsStub{
						TransferCalled: func(
							_ transaction.TXContract_ContractType,
							_ []byte,
							_ *transaction.TransferContract,
						) (transaction.Transaction_TXResultCode, error) {
							*accountTransferCalled = true
							return transaction.Transaction_Ok, nil
						},
					}
				},
			},
			fork,
		)
		require.NoError(t, instance.SetPayableChecker(&PayableHandlerStub{}))
		return instance
	}

	t.Run("fork disabled keeps legacy truncation", func(t *testing.T) {
		var accountTransferCalled bool
		instance := newInstance(false, &accountTransferCalled)

		transfer := &vmcommon.KDATransfer{KDATokenName: []byte("FTOld"), KDAValue: oversized, KDATokenNonce: 0}
		vmInput := vmInputTransferCreation([]*vmcommon.KDATransfer{transfer}, funcGasCost)

		vmOutput, err := instance.ProcessBuiltinFunction(vmInput)
		require.NoError(t, err)
		require.Equal(t, vmcommon.Ok, vmOutput.ReturnCode)
		require.False(t, accountTransferCalled, "2^64 truncates to 0, so the account transfer is skipped")
		require.True(t, transfer.IsExecuted())
		require.Zero(t, oversized.Cmp(transfer.KDAValue), "legacy path must not normalize the forwarded value")
	})

	t.Run("fork enabled rejects oversized value", func(t *testing.T) {
		var accountTransferCalled bool
		instance := newInstance(true, &accountTransferCalled)

		transfer := &vmcommon.KDATransfer{KDATokenName: []byte("FTNew"), KDAValue: oversized, KDATokenNonce: 0}
		vmInput := vmInputTransferCreation([]*vmcommon.KDATransfer{transfer}, funcGasCost)

		vmOutput, err := instance.ProcessBuiltinFunction(vmInput)
		require.ErrorIs(t, err, common.ErrInvalidValue)
		require.Nil(t, vmOutput)
		require.False(t, accountTransferCalled)
	})
}
