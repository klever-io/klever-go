package transaction_test

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	tdata "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
)

// Helper function to create args for NewSimulateTxProcessor
func createArgs() transaction.ArgsNewSimulateTxProcessor {
	senderAccount, _ := state.NewUserAccount(sender)

	loadUser := func(address []byte) (state.UserAccountHandler, error) {
		if bytes.Equal(address, sender) {
			return senderAccount, nil
		}

		return mock.NewAccountWrapMock(address), nil
	}

	vmOutputCacher := mock.NewCacherMock()

	scProcessor := mock.NewSmartContractProcessorStub()
	scProcessor.LastBlockCalled = func() data.HeaderHandler {
		return &block.Block{}
	}
	scProcessor.DeploySmartContractCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
		vmOutputCacher.Put(ctx.TxHash(), &vmcommon.VMOutput{Logs: make([]*vmcommon.LogEntry, 0)}, 0)
		return vmcommon.Ok, nil
	}
	scProcessor.ExecuteSmartContractTransactionCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
		vmOutputCacher.Put(ctx.TxHash(), &vmcommon.VMOutput{Logs: make([]*vmcommon.LogEntry, 0)}, 0)
		return vmcommon.Ok, nil
	}

	return transaction.ArgsNewSimulateTxProcessor{
		Hasher:      &mock.HasherMock{},
		Marshalizer: &mock.MarshalizerMock{},
		AccountsCacher: &mock.AccountsCacherStub{
			GetExistingUserCalled: loadUser,
			LoadUserCalled:        loadUser,
		},
		KAppsController: &mock.KappsControllerMock{},
		PubkeyConv:      &mock.PubkeyConverterMock{},
		ScProcessor:     scProcessor,
		EconomicsFee:    &mock.EconomicsHandlerStub{},
		ForkController:  &mock.ForkControllerStub{},
		VMOutputCacher:  vmOutputCacher,
	}
}

// Helper function to create a new simulateTxProcessor with mocks
func newMockSimulateTxProcessor() *transaction.SimulateTxProcessorExportTest {
	args := createArgs()

	txProc, _ := transaction.NewSimulateTxProcessorExportTest(args)
	return txProc
}

// Test NewSimulateTxProcessor
func TestNewSimulateTxProcessor(t *testing.T) {
	t.Run("Valid creation", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		assert.NotNil(t, txProc)
	})

	t.Run("Nil Hasher", func(t *testing.T) {
		args := createArgs()
		args.Hasher = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilHasher, err)
	})

	t.Run("Nil Marshalizer", func(t *testing.T) {
		args := createArgs()
		args.Marshalizer = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilMarshalizer, err)
	})

	t.Run("Nil AccountsCacher", func(t *testing.T) {
		args := createArgs()
		args.AccountsCacher = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilAccountsAdapter, err)
	})

	t.Run("Nil KAppsController", func(t *testing.T) {
		args := createArgs()
		args.KAppsController = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilKAppsController, err)
	})

	// Add similar tests for other nil dependencies
	t.Run("Nil PubkeyConverter", func(t *testing.T) {
		args := createArgs()
		args.PubkeyConv = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilPubkeyConverter, err)
	})

	t.Run("Nil SmartContractProcessor", func(t *testing.T) {
		args := createArgs()
		args.ScProcessor = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilSmartContractProcessor, err)
	})

	t.Run("Nil EconomicsFeeHandler", func(t *testing.T) {
		args := createArgs()
		args.EconomicsFee = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, process.ErrNilEconomicsFeeHandler, err)
	})

	t.Run("Nil ForkController", func(t *testing.T) {
		args := createArgs()
		args.ForkController = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, common.ErrNilForkController, err)
	})

	t.Run("Nil Cacher", func(t *testing.T) {
		args := createArgs()
		args.VMOutputCacher = nil
		txProc, err := transaction.NewSimulateTxProcessor(args)
		assert.Nil(t, txProc)
		assert.Equal(t, common.ErrNilCacher, err)
	})
}

func createBaseTx() *tdata.Transaction {
	return &tdata.Transaction{
		RawData: &tdata.Transaction_Raw{
			Sender: sender,
		},
	}
}

// Test ProcessTransaction
func TestProcessTransaction(t *testing.T) {
	t.Run("Valid transaction", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		tx := createBaseTx()
		_ = tx.PushContract(tdata.TXContract_SmartContractType, &tdata.SmartContract{Type: tdata.SmartContract_SCDeploy})
		err := txProc.ProcessTransaction(tx)
		assert.NoError(t, err)
	})

	t.Run("Nil transaction", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		err := txProc.ProcessTransaction(nil)
		assert.Equal(t, process.ErrNilTransaction, err)
	})

	t.Run("Multiple contracts", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		tx := &tdata.Transaction{
			RawData: &tdata.Transaction_Raw{
				Contract: []*tdata.TXContract{
					{Type: tdata.TXContract_SmartContractType},
					{Type: tdata.TXContract_SmartContractType},
				},
			},
		}
		err := txProc.ProcessTransaction(tx)
		assert.Equal(t, process.ErrSmartContractFailMaxContracts, err)
	})

	t.Run("Invalid Hasher", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		txProc.SetHasher(nil)
		err := txProc.ProcessTransaction(createBaseTx())
		assert.Equal(t, tools.ErrNilHasher, err)
	})

	t.Run("Error loading account owner", func(t *testing.T) {
		args := createArgs()
		args.AccountsCacher = &mock.AccountsCacherStub{
			LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return nil, common.ErrNilTrie
			},
		}
		txProc, _ := transaction.NewSimulateTxProcessorExportTest(args)

		err := txProc.ProcessTransaction(createBaseTx())
		assert.Equal(t, common.ErrNilTrie, err)
	})

	t.Run("Error creating KApp context, invalid last block", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		scProcessor := txProc.ScProcessor().(*mock.SmartContractProcessorStub)
		scProcessor.LastBlockCalled = func() data.HeaderHandler {
			return nil
		}

		err := txProc.ProcessTransaction(createBaseTx())
		assert.Equal(t, process.ErrNilBlockChain, err)
	})

	t.Run("Invalid SCType", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		tx := createBaseTx()
		_ = tx.PushContract(tdata.TXContract_SmartContractType, &tdata.SmartContract{Type: 9999}) // invalid type
		err := txProc.ProcessTransaction(tx)
		assert.Equal(t, common.ErrSmartContractTypeInvalid, err)
	})

	t.Run("Invalid consume gas", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		tx := createBaseTx()
		_ = tx.PushContract(tdata.TXContract_SmartContractType, &tdata.SmartContract{Type: tdata.SmartContract_SCInvoke})

		overflowValue := big.NewInt(0).SetUint64(math.MaxUint64)
		overflowValue.Add(overflowValue, big.NewInt(1))

		txProc.ScProcessor().(*mock.SmartContractProcessorStub).ExecuteSmartContractTransactionCalled = func(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error) {
			txProc.VMOutputCacher().Put(ctx.TxHash(), &vmcommon.VMOutput{
				Logs: []*vmcommon.LogEntry{
					{
						Identifier:  []byte(core.TotalConsumedGasString),
						Topics:      [][]byte{overflowValue.Bytes()},
						IsSystemLog: true,
					},
				}}, 0)
			return vmcommon.Ok, nil
		}

		err := txProc.ProcessTransaction(tx)
		assert.Equal(t, process.ErrInvalidMaxGasLimitPerTx, err)
	})

}

// Test GetVmOutputs
func TestGetVmOutputs(t *testing.T) {
	t.Run("Existing output", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		expectedOutput := &vmcommon.VMOutput{}
		txProc.VMOutputCacher().Put([]byte("hash"), expectedOutput, 0)

		output, err := txProc.GetVmOutputs([]byte("hash"))
		assert.NoError(t, err)
		assert.Equal(t, expectedOutput, output)
	})

	t.Run("Non-existing output", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		output, err := txProc.GetVmOutputs([]byte("nonexistent"))
		assert.Error(t, err)
		assert.Nil(t, output)
	})
}

// Test GetTotalConsumedGasByContract
func TestGetTotalConsumedGasByContract(t *testing.T) {
	t.Run("Valid gas consumption", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		expectedOutput := &vmcommon.VMOutput{
			Logs: []*vmcommon.LogEntry{
				{
					Identifier:  []byte(core.TotalConsumedGasString),
					Topics:      [][]byte{big.NewInt(100).Bytes()},
					IsSystemLog: true,
				},
			},
		}
		txProc.VMOutputCacher().Put([]byte("hash"), expectedOutput, 0)

		gas, err := txProc.GetTotalConsumedGasByContract([]byte("hash"))
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(100), gas)
	})

	t.Run("Non-existing output", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		gas, err := txProc.GetTotalConsumedGasByContract([]byte("nonexistent"))
		assert.Equal(t, err, process.ErrNilVMOutput)
		assert.Nil(t, gas)
	})
}

// Test deploySmartContract
func TestDeploySmartContract(t *testing.T) {
	t.Run("Successful deployment", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
			TxHash: []byte("hash"),
		})
		tc := &tdata.SmartContract{}
		tx := &tdata.Transaction{}
		sw := tools.NewStopWatch()

		gas, err := txProc.DeploySmartContract(ctx, tc, tx, ctx.TxHash(), sw)
		assert.NoError(t, err)
		assert.Equal(t, gas.Int64(), int64(0))
	})
}

// Test executeSmartContract
func TestExecuteSmartContract(t *testing.T) {
	t.Run("Successful execution", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
			TxHash: []byte("hash"),
		})
		ownerAcc := mock.NewAccountWrapMock(sender)
		tc := &tdata.SmartContract{}
		tx := &tdata.Transaction{}
		sw := tools.NewStopWatch()

		gas, err := txProc.ExecuteSmartContract(ctx, ownerAcc, tc, tx, ctx.TxHash(), sw)
		assert.NoError(t, err)
		assert.NotNil(t, gas)
	})
}

// Test IsInterfaceNil
func TestIsInterfaceNil(t *testing.T) {
	t.Run("Non-nil", func(t *testing.T) {
		txProc := newMockSimulateTxProcessor()
		assert.False(t, txProc.IsInterfaceNil())
	})

	t.Run("Nil", func(t *testing.T) {
		txProc := transaction.NilSimulateTxProcessorExportTest()
		assert.True(t, txProc.IsInterfaceNil())
	})
}
