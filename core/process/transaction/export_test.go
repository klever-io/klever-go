package transaction

import (
	"math/big"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/ed25519"
	"github.com/klever-io/klever-go/crypto/signing/ed25519/singlesig"
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

type TxProcessorExportTest struct {
	*txProcessor
}

func NewTxProcessorExportTest() *TxProcessorExportTest {
	keyGen := signing.NewKeyGenerator(ed25519.NewEd25519())
	txSingleSigner := &singlesig.Ed25519Signer{}

	return &TxProcessorExportTest{
		txProcessor: &txProcessor{
			baseTxProcessor: &baseTxProcessor{
				keyGen:       keyGen,
				singleSigner: txSingleSigner,
			},
		},
	}
}

func (txProc *TxProcessorExportTest) ValidateSCTransaction(ctx kapp.KappContext, tx *transaction.Transaction) error {
	return txProc.validateSCTransaction(ctx, tx)
}

func (txProc *TxProcessorExportTest) SmartContract(ctx kapp.KappContext, owner state.UserAccountHandler, tx *transaction.Transaction) error {
	return txProc.smartContract(ctx, owner, tx)
}

func (txProc *TxProcessorExportTest) SetForkController(forkController core.ForkController) {
	txProc.forkController = forkController
}

func (txProc *TxProcessorExportTest) SetSCProcessor(scProcessor process.SmartContractProcessor) {
	txProc.scProcessor = scProcessor
}

func (txProc *TxProcessorExportTest) SetAccountsCacher(accountsCacher state.AccountsCacher) {
	txProc.accountsCacher = accountsCacher
}

func (txProc *TxProcessorExportTest) GetAccountsCacher() *mock.AccountsCacherStub {
	return txProc.accountsCacher.(*mock.AccountsCacherStub)
}

func (txProc *TxProcessorExportTest) CheckTxValues(tx *transaction.Transaction, acntSnd state.UserAccountHandler, txHash []byte) error {
	return txProc.checkTxValues(tx, acntSnd, txHash)
}

func (txProc *TxProcessorExportTest) SetEconomicsFee(economicsFee process.EconomicsDataHandler) {
	txProc.economicsFee = economicsFee
}

func (txProc *TxProcessorExportTest) GetEconomicsFee() *mock.EconomicsHandlerStub {
	return txProc.economicsFee.(*mock.EconomicsHandlerStub)
}

func (txProc *TxProcessorExportTest) ValidatePermissionOperation(tx *transaction.Transaction, permission *state.Permission, txHash []byte) ([][]byte, error) {
	return txProc.validatePermission(tx, permission, txHash)
}

func (txProc *TxProcessorExportTest) SingleSigner() crypto.SingleSigner {
	return txProc.singleSigner
}
