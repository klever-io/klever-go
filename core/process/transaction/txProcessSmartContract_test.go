package transaction_test

import (
	"testing"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/vmcommon"

	"github.com/klever-io/klever-go/common"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/klever-io/klever-go/core/process"

	"github.com/stretchr/testify/assert"

	"github.com/klever-io/klever-go/core/process/transaction"
	proto "github.com/klever-io/klever-go/data/transaction"
	gproto "google.golang.org/protobuf/proto"

	"github.com/klever-io/klever-go/common/mock"
)

func TestTXProcessor_validateSCTransaction(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		// setup
		Name      string
		AfterFork bool
		ExecData  []byte
		// assert
		ExpectedError      error
		ExpectedResultCode proto.Transaction_TXResultCode
	}{
		{
			Name:      "Should pass",
			AfterFork: true,
			ExecData:  []byte{1},
		},
		{
			Name:               "Should fail on no data",
			AfterFork:          true,
			ExpectedError:      process.ErrInvalidContractOrRawDataSize,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			Name:               "Should fail before fork",
			AfterFork:          false,
			ExpectedError:      process.ErrInvalidTransactionType,
			ExpectedResultCode: proto.Transaction_ContractNotFound,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			forkController := mock.ForkControllerStub{
				EnableSmartContractsValue: scenario.AfterFork,
			}
			txProc := transaction.NewTxProcessorExportTest()
			txProc.SetForkController(&forkController)

			kappContext := mock.KAppContextStub{
				GetExecDataCalled: func() []byte {
					return scenario.ExecData
				},
			}
			tx := &proto.Transaction{}
			err := txProc.ValidateSCTransaction(&kappContext, tx)
			assert.Equal(t, err, scenario.ExpectedError)
			assert.Equal(t, tx.ResultCode, scenario.ExpectedResultCode)
		})
	}
}

func TestTXProcessor_smartContract(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		// setup
		name                    string
		ExecData                []byte
		TxContractType          proto.TXContract_ContractType
		SmartContractActionType proto.SmartContract_SCType
		ActionResultCode        vmcommon.ReturnCode
		ActionError             error
		GetUserError            error
		// assert
		ExpectedError      error
		ExpectedResultCode proto.Transaction_TXResultCode
		MustCallInvokeSC   bool
		MustCallDeploySC   bool
	}{
		{
			name:               "Should fail on no data",
			ExpectedError:      process.ErrInvalidContractOrRawDataSize,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			name:               "Should fail on invalid contract type",
			ExecData:           []byte{1},
			TxContractType:     proto.TXContract_BuyContractType,
			ExpectedError:      common.ErrInvalidContract,
			ExpectedResultCode: proto.Transaction_ContractInvalid,
		},
		{
			name:                    "Should fail on invalid smart contract action type",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: 100,
			ExpectedError:           common.ErrSmartContractTypeInvalid,
			ExpectedResultCode:      proto.Transaction_ParameterInvalid,
		},
		{
			name:                    "Should call deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ExpectedError:           nil,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should call invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ExpectedError:           nil,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallInvokeSC:        true,
		},
		{
			name:                    "Should handle error on deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ActionError:             vmcommon.ErrInvalidVMType,
			ExpectedError:           vmcommon.ErrInvalidVMType,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should handle error on invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ActionError:             vmcommon.ErrInvalidVMType,
			ExpectedError:           vmcommon.ErrInvalidVMType,
			ExpectedResultCode:      proto.Transaction_Ok,
			MustCallInvokeSC:        true,
		},
		{
			name:                    "Should handle not ok result code on deploySC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCDeploy,
			ActionResultCode:        vmcommon.VMContractInvalid,
			ExpectedResultCode:      proto.Transaction_ContractInvalid,
			MustCallDeploySC:        true,
		},
		{
			name:                    "Should handle not ok result code on invokeSC",
			ExecData:                []byte{1},
			TxContractType:          proto.TXContract_SmartContractType,
			SmartContractActionType: proto.SmartContract_SCInvoke,
			ActionResultCode:        vmcommon.VMContractInvalid,
			ExpectedResultCode:      proto.Transaction_ContractInvalid,
			MustCallInvokeSC:        true,
		},
		{
			name:               "Should fail if invalid address on invokeSC",
			ExecData:           []byte{1},
			TxContractType:     proto.TXContract_SmartContractType,
			GetUserError:       common.ErrAccountNotFound,
			ExpectedError:      common.ErrAccountNotFound,
			ExpectedResultCode: proto.Transaction_AccountError,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// scenario setup
			deploySCCalled := false
			invokeSCCalled := false

			// environment setup
			scProcessor := &mock.SmartContractProcessorStub{}
			forkController := &mock.ForkControllerStub{}
			accountsCacher := &mock.AccountsCacherStub{}
			forkController.EnableSmartContractsValue = true
			txProc := transaction.NewTxProcessorExportTest()
			txProc.SetForkController(forkController)
			txProc.SetSCProcessor(scProcessor)
			txProc.SetAccountsCacher(accountsCacher)

			// stub configuration
			kappContext := mock.KAppContextStub{
				GetExecDataCalled: func() []byte {
					return scenario.ExecData
				},
			}
			scProcessor.DeploySmartContractCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
				deploySCCalled = true
				return scenario.ActionResultCode, scenario.ActionError
			}
			scProcessor.ExecuteSmartContractTransactionCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
				invokeSCCalled = true
				return scenario.ActionResultCode, scenario.ActionError
			}
			accountsCacher.GetExistingUserCalled = func(address []byte) (state.UserAccountHandler, error) {
				if scenario.GetUserError != nil {
					return nil, scenario.GetUserError
				}
				return &mock.UserAccountHandlerStub{}, nil
			}

			// setup TX
			userAccount := mock.UserAccountHandlerStub{}
			txContract := &proto.TXContract{
				Type:      scenario.TxContractType,
				Parameter: &anypb.Any{},
			}
			txContractData := &proto.SmartContract{
				Type: scenario.SmartContractActionType,
			}
			err := anypb.MarshalFrom(txContract.Parameter, txContractData, gproto.MarshalOptions{})
			assert.Nil(t, err)
			tx := &proto.Transaction{
				RawData: &proto.Transaction_Raw{
					Contract: []*proto.TXContract{txContract},
				},
			}

			// validate empty data
			err = txProc.SmartContract(&kappContext, &userAccount, tx)
			if scenario.ExpectedError != nil {
				assert.Equal(t, scenario.ExpectedError, err)
			}
			assert.Equal(t, scenario.ExpectedResultCode, tx.ResultCode)
			assert.Equal(t, scenario.MustCallDeploySC, deploySCCalled)
			assert.Equal(t, scenario.MustCallInvokeSC, invokeSCCalled)
		})
	}
}
