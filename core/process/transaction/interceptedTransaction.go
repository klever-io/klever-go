package transaction

import (
	"bytes"
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.TxValidatorHandler = (*InterceptedTransaction)(nil)
var _ process.InterceptedData = (*InterceptedTransaction)(nil)

// InterceptedTransaction holds and manages a transaction based struct with extended functionality
type InterceptedTransaction struct {
	tx                     *transaction.Transaction
	protoMarshalizer       marshal.Marshalizer
	signMarshalizer        marshal.Marshalizer
	hasher                 hashing.Hasher
	txSignHasher           hashing.Hasher
	keyGen                 crypto.KeyGenerator
	singleSigner           crypto.SingleSigner
	pubkeyConv             core.PubkeyConverter
	hash                   []byte
	whiteListerVerifiedTxs process.WhiteListHandler
	feeHandler             process.EconomicsDataHandler
	txVersionChecker       process.TxVersionCheckerHandler
	forkController         core.ForkController
	chainID                []byte
}

type InterceptedTransactionArgs struct {
	TxBuff                 []byte
	ProtoMarshalizer       marshal.Marshalizer
	SignMarshalizer        marshal.Marshalizer
	Hasher                 hashing.Hasher
	KeyGen                 crypto.KeyGenerator
	Signer                 crypto.SingleSigner
	PubkeyConv             core.PubkeyConverter
	WhiteListerVerifiedTxs process.WhiteListHandler
	ChainID                []byte
	TxSignHasher           hashing.Hasher
	FeeHandler             process.EconomicsDataHandler
	TxVersionChecker       process.TxVersionCheckerHandler
	ForkController         core.ForkController
}

// NewInterceptedTransaction returns a new instance of InterceptedTransaction
func NewInterceptedTransaction(
	args *InterceptedTransactionArgs,
) (*InterceptedTransaction, error) {
	err := ValidateInterceptedTransactionArgs(args)
	if err != nil {
		return nil, err
	}

	tx, err := createTx(args.ProtoMarshalizer, args.TxBuff)
	if err != nil {
		return nil, err
	}

	inTx := &InterceptedTransaction{
		tx:                     tx,
		protoMarshalizer:       args.ProtoMarshalizer,
		signMarshalizer:        args.SignMarshalizer,
		hasher:                 args.Hasher,
		singleSigner:           args.Signer,
		pubkeyConv:             args.PubkeyConv,
		keyGen:                 args.KeyGen,
		whiteListerVerifiedTxs: args.WhiteListerVerifiedTxs,
		chainID:                args.ChainID,
		txSignHasher:           args.TxSignHasher,
		feeHandler:             args.FeeHandler,
		txVersionChecker:       args.TxVersionChecker,
		forkController:         args.ForkController,
	}

	txHeader, err := args.SignMarshalizer.Marshal(tx.GetRaw())
	if err != nil {
		return nil, err
	}

	err = inTx.processFields(args.TxBuff, txHeader)
	if err != nil {
		return nil, err
	}

	return inTx, nil
}

func ValidateInterceptedTransactionArgs(args *InterceptedTransactionArgs) error {
	if args.TxBuff == nil {
		return process.ErrNilBuffer
	}
	if check.IfNil(args.ProtoMarshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(args.SignMarshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(args.KeyGen) {
		return common.ErrNilKeyGen
	}
	if check.IfNil(args.Signer) {
		return common.ErrNilSingleSigner
	}
	if check.IfNil(args.PubkeyConv) {
		return common.ErrNilPubkeyConverter
	}
	if check.IfNil(args.WhiteListerVerifiedTxs) {
		return process.ErrNilWhiteListHandler
	}
	if len(args.ChainID) == 0 {
		return process.ErrInvalidChainID
	}
	if check.IfNil(args.TxSignHasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(args.FeeHandler) {
		return process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.TxVersionChecker) {
		return process.ErrNilTransactionVersionChecker
	}
	if check.IfNil(args.ForkController) {
		return process.ErrNilForkController
	}
	return nil
}

func createTx(marshalizer marshal.Marshalizer, txBuff []byte) (*transaction.Transaction, error) {
	tx := &transaction.Transaction{}
	err := marshalizer.Unmarshal(tx, txBuff)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// CheckValidity checks if the received transaction is valid (not nil fields, fees and so on)
func (inTx *InterceptedTransaction) CheckValidity() error {
	err := inTx.integrity()
	if err != nil {
		return err
	}

	return nil
}

// CheckTXSignature checks if the received transaction has valid signature
func (inTx *InterceptedTransaction) CheckTXSignature() error {
	// check if disabledSignature check (size == 0)
	if inTx.singleSigner.SignatureSize() == 0 {
		return nil
	}

	// check signature structure
	if len(inTx.tx.Signature) == 0 {
		return common.ErrNoSignatures
	}

	if len(inTx.tx.Signature) > core.MaxPermissionSigners {
		return common.ErrExceedsMaxSignatures
	}

	dupCheck := make(map[string]bool)
	for _, s1 := range inTx.tx.Signature {
		// check DUP sign
		if dupCheck[string(s1)] {
			// same signature was added more than one time
			return common.ErrDupSignature
		}
		dupCheck[string(s1)] = true

		// check signature length
		if len(s1) != inTx.singleSigner.SignatureSize() {
			return common.ErrInvalidSignatureLength
		}
	}

	return nil
}

func (inTx *InterceptedTransaction) processFields(txBuff []byte, txHeader []byte) error {
	inTx.hash = inTx.txSignHasher.Compute(string(txHeader))
	return nil
}

// integrity checks for not nil fields and negative value
func (inTx *InterceptedTransaction) integrity() error {
	err := inTx.txVersionChecker.CheckTxVersion(inTx.tx)
	if err != nil {
		return err
	}

	if !bytes.Equal(inTx.tx.RawData.GetChainID(), inTx.chainID) {
		return process.ErrInvalidChainID
	}

	if len(inTx.tx.RawData.Sender) != inTx.pubkeyConv.Len() ||
		bytes.Equal(inTx.tx.RawData.Sender, core.ZeroAddress) ||
		bytes.Equal(inTx.tx.RawData.Sender, core.BlackHoleAddress) {
		return process.ErrInvalidSndAddr
	}

	if len(inTx.tx.RawData.Contract) == 0 || len(inTx.tx.RawData.Contract) > core.MaxLengthOfContracts {
		return process.ErrInvalidTransactionNoContract
	}

	if err := inTx.CheckTXSignature(); err != nil {
		return err
	}

	// ensure that transaction is prepared for processing (remove unsigned fields)
	inTx.tx.PrepareForProcessing()

	// Validate Transaction Size
	err = inTx.validateTransactionSize()
	if err != nil {
		return err
	}

	if err := inTx.tx.Validate(inTx.forkController); err != nil {
		return err
	}

	transactionCost, err := inTx.feeHandler.CheckValidityTxValues(inTx.tx)
	if err != nil {
		return err
	}

	// validate fees
	gasLimit, gasMultiplier, err := inTx.feeHandler.ComputeGas(inTx.tx, transactionCost)
	if err != nil {
		return err
	}
	inTx.tx.GasLimit = gasLimit
	inTx.tx.GasMultiplier = gasMultiplier

	return err
}

// Transaction returns the transaction pointer that actually holds the data
func (inTx *InterceptedTransaction) Transaction() data.TransactionHandler {
	return inTx.tx
}

// Hash gets the hash of this transaction
func (inTx *InterceptedTransaction) Hash() []byte {
	return inTx.hash
}

// Nonce returns the transaction nonce
func (inTx *InterceptedTransaction) Nonce() uint64 {
	if inTx.tx != nil && inTx.tx.RawData != nil {
		return inTx.tx.RawData.Nonce
	}
	return 0
}

// PermissionID returns the transaction permission ID
func (inTx *InterceptedTransaction) PermissionID() int32 {
	if inTx.tx != nil && inTx.tx.RawData != nil {
		return inTx.tx.RawData.PermissionID
	}
	return 0
}

// ValidatePermissionOperation -
func (inTx *InterceptedTransaction) ValidatePermissionOperation(permission []byte) error {
	if inTx.tx != nil {
		return inTx.tx.ValidatePermissionOperation(permission)
	}
	return common.ErrInvalidTransactionType
}

// Signature returns the transaction Signature
func (inTx *InterceptedTransaction) Signature() [][]byte {
	if inTx.tx != nil {
		return inTx.tx.Signature
	}
	return nil
}

// SenderAddress returns the transaction sender address
func (inTx *InterceptedTransaction) SenderAddress() []byte {
	if inTx.tx != nil && inTx.tx.RawData != nil {
		return inTx.tx.RawData.Sender
	}
	return nil
}

// Fee returns the estimated cost of the transaction
func (inTx *InterceptedTransaction) Fee() int64 {
	if inTx.tx != nil && inTx.tx.RawData != nil {
		return inTx.tx.RawData.BandwidthFee + inTx.tx.RawData.KAppFee
	}
	return 0
}

// KDAFee -
func (inTx *InterceptedTransaction) KDAFee() data.KDAFeeHandler {
	if inTx.tx != nil && inTx.tx.RawData != nil {
		return inTx.tx.RawData.KDAFee
	}
	return nil
}

// Type returns the type of this intercepted data
func (inTx *InterceptedTransaction) Type() string {
	return "intercepted tx"
}

// String returns the transaction's most important fields as string
func (inTx *InterceptedTransaction) String() string {
	if inTx == nil || inTx.tx == nil || inTx.tx.RawData == nil {
		return ""
	}
	fees := inTx.tx.RawData.KAppFee
	return fmt.Sprintf("sender=%s, nonce=%d, fees=%d",
		logger.DisplayByteSlice(inTx.tx.RawData.Sender),
		inTx.tx.GetNonce(),
		fees,
	)
}

// Identifiers returns the identifiers used in requests
func (inTx *InterceptedTransaction) Identifiers() [][]byte {
	return [][]byte{inTx.hash}
}

// IsInterfaceNil returns true if there is no value under the interface
func (inTx *InterceptedTransaction) IsInterfaceNil() bool {
	return inTx == nil
}

func (inTx *InterceptedTransaction) validateDataFieldSize(txClone *transaction.Transaction) error {
	// check data field size
	dataSize := 0
	for _, data := range txClone.GetRawData().Data {
		dataSize += len(data)
	}

	maxSize := core.MaxDataSize
	if !inTx.forkController.EnableSmartContracts() {
		maxSize = core.MaxDataSizeOld
	}

	if dataSize >= maxSize {
		return common.ErrDataFieldTooBig
	}

	return nil
}

func (inTx *InterceptedTransaction) validateTransactionSize() error {
	txClone := inTx.tx.Clone()

	if err := inTx.validateDataFieldSize(txClone); err != nil {
		return err
	}

	// remove data field from size check
	txClone.RawData.Data = nil

	// check contracts field size
	contractsSize := 0
	for _, contract := range txClone.GetContracts() {
		// only check contract size after fork
		if inTx.forkController.EnableSmartContracts() &&
			!transaction.IsContractSizeValid(contract.GetParameter().Value, contract.GetType()) {
			return common.ErrInvalidContractSize
		}

		if !transaction.IsValidTypeURL(contract.GetParameter().TypeUrl, contract.GetType()) {
			return common.ErrInvalidContractTypeURL
		}

		contractsSize += len(contract.GetParameter().Value) + len(contract.GetParameter().TypeUrl) + core.ContractSizeOverhead
	}

	// remove signature field from size check (signatures are checked separately)
	txClone.Signature = nil

	// check raw data size without data and contracts
	transactionRaw, err := inTx.protoMarshalizer.Marshal(txClone.RawData)
	if err != nil {
		return err
	}

	// remove expected contract size and check if transaction raw is bigger than allowed
	if len(transactionRaw)-contractsSize >= core.MaxTransactionRaw {
		return common.ErrInvalidTransactionRawSize
	}

	// remove transaction raw from invalid fields checker (prevent unchecked values)
	txClone.RawData = nil
	transaction, err := inTx.protoMarshalizer.Marshal(txClone)
	if err != nil {
		return err
	}

	// transaction must be empty after `PrepareForProcessing`, removing raw data and signatures
	if len(transaction) > 0 && string(transaction) != common.EmptyJSonMarshalData {
		return common.ErrInvalidTransactionSize
	}

	return nil
}
