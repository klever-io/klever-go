package txsimulator

import (
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	txSimData "github.com/klever-io/klever-go/core/process/txsimulator/data"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

// ArgsTxSimulator holds the arguments required for creating a new transaction simulator
type ArgsTxSimulator struct {
	TransactionProcessor   TransactionProcessor
	AddressPubKeyConverter core.PubkeyConverter
	VMOutputCacher         storage.Cacher
	Hasher                 hashing.Hasher
	Marshalizer            marshal.Marshalizer
}

type transactionSimulator struct {
	mutOperation           sync.Mutex
	txProcessor            TransactionProcessor
	addressPubKeyConverter core.PubkeyConverter
	vmOutputCacher         storage.Cacher
	hasher                 hashing.Hasher
	marshalizer            marshal.Marshalizer
}

// NewTransactionSimulator returns a new instance of a transactionSimulator
func NewTransactionSimulator(args ArgsTxSimulator) (*transactionSimulator, error) {
	if check.IfNil(args.TransactionProcessor) {
		return nil, common.ErrNilTxSimulatorProcessor
	}
	if check.IfNil(args.AddressPubKeyConverter) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.VMOutputCacher) {
		return nil, common.ErrNilCacher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}

	return &transactionSimulator{
		txProcessor:            args.TransactionProcessor,
		addressPubKeyConverter: args.AddressPubKeyConverter,
		vmOutputCacher:         args.VMOutputCacher,
		marshalizer:            args.Marshalizer,
		hasher:                 args.Hasher,
	}, nil
}

// ProcessTx will process the transaction in a special environment, where state-writing is not allowed
func (ts *transactionSimulator) ProcessTx(tx *transaction.Transaction) (*txSimData.SimulationResults, error) {
	ts.mutOperation.Lock()
	defer ts.mutOperation.Unlock()

	txResult := transaction.Transaction_SUCCESS
	failReason := ""

	err := ts.txProcessor.ProcessTransaction(tx)
	if err != nil {
		failReason = err.Error()
		txResult = transaction.Transaction_FAILED
	}

	results := &txSimData.SimulationResults{
		Result:     txResult,
		FailReason: failReason,
	}

	vmOutput, ok := ts.getVMOutputOfTx(tx)
	if ok {
		results.VMOutput = vmOutput
	}

	return results, nil
}

func (ts *transactionSimulator) getVMOutputOfTx(tx *transaction.Transaction) (*vmcommon.VMOutput, bool) {
	txHash, err := tools.CalculateHash(ts.marshalizer, ts.hasher, tx.RawData)
	if err != nil {
		return nil, false
	}

	defer ts.vmOutputCacher.Remove(txHash)

	vmOutputI, ok := ts.vmOutputCacher.Get(txHash)
	if !ok {
		return nil, false
	}

	vmOutput, ok := vmOutputI.(*vmcommon.VMOutput)
	if !ok {
		return nil, false
	}

	return vmOutput, true
}

// IsInterfaceNil returns true if there is no value under the interface
func (ts *transactionSimulator) IsInterfaceNil() bool {
	return ts == nil
}
