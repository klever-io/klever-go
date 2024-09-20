package transaction

import (
	"math/big"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
)

func (inTx *InterceptedTransaction) SetWhitelistHandler(handler process.WhiteListHandler) {
	inTx.whiteListerVerifiedTxs = handler
}

type SimulateTxProcessorExportTest struct {
	*simulateTxProcessor
}

func NewSimulateTxProcessorExportTest(args ArgsNewSimulateTxProcessor) (*SimulateTxProcessorExportTest, error) {
	processor, err := NewSimulateTxProcessor(args)
	if err != nil {
		return nil, err
	}
	return &SimulateTxProcessorExportTest{processor}, nil
}

func (s *SimulateTxProcessorExportTest) VMOutputCacher() storage.Cacher {
	return s.vmOutputCacher
}

func (s *SimulateTxProcessorExportTest) DeploySmartContract(ctx kapp.KappContext, tc *transaction.SmartContract, tx *transaction.Transaction, computedHash []byte, sw *tools.StopWatch) (*big.Int, error) {
	return s.deploySmartContract(ctx, tc, tx, computedHash, sw)
}

func (s *SimulateTxProcessorExportTest) ExecuteSmartContract(ctx kapp.KappContext, ownerAcc state.UserAccountHandler, tc *transaction.SmartContract, tx *transaction.Transaction, computedHash []byte, sw *tools.StopWatch) (*big.Int, error) {
	return s.executeSmartContract(ctx, ownerAcc, tc, tx, computedHash, sw)
}

func (s *SimulateTxProcessorExportTest) SetHasher(hasher hashing.Hasher) {
	s.hasher = hasher
}

func (s *SimulateTxProcessorExportTest) ScProcessor() process.SmartContractProcessor {
	return s.scProcessor
}

func NilSimulateTxProcessorExportTest() *SimulateTxProcessorExportTest {

	return &SimulateTxProcessorExportTest{
		simulateTxProcessor: nil,
	}
}
