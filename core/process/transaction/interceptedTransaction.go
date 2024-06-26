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
	chainID                []byte
}

// NewInterceptedTransaction returns a new instance of InterceptedTransaction
func NewInterceptedTransaction(
	txBuff []byte,
	protoMarshalizer marshal.Marshalizer,
	signMarshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	keyGen crypto.KeyGenerator,
	signer crypto.SingleSigner,
	pubkeyConv core.PubkeyConverter,
	whiteListerVerifiedTxs process.WhiteListHandler,
	chainID []byte,
	txSignHasher hashing.Hasher,
	feeHandler process.EconomicsDataHandler,
	txVersionChecker process.TxVersionCheckerHandler,
) (*InterceptedTransaction, error) {

	if txBuff == nil {
		return nil, process.ErrNilBuffer
	}
	if check.IfNil(protoMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(signMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(keyGen) {
		return nil, common.ErrNilKeyGen
	}
	if check.IfNil(signer) {
		return nil, common.ErrNilSingleSigner
	}
	if check.IfNil(pubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(whiteListerVerifiedTxs) {
		return nil, process.ErrNilWhiteListHandler
	}
	if len(chainID) == 0 {
		return nil, process.ErrInvalidChainID
	}
	if check.IfNil(txSignHasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(feeHandler) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(txVersionChecker) {
		return nil, process.ErrNilTransactionVersionChecker
	}

	tx, err := createTx(protoMarshalizer, txBuff)
	if err != nil {
		return nil, err
	}

	inTx := &InterceptedTransaction{
		tx:                     tx,
		protoMarshalizer:       protoMarshalizer,
		signMarshalizer:        signMarshalizer,
		hasher:                 hasher,
		singleSigner:           signer,
		pubkeyConv:             pubkeyConv,
		keyGen:                 keyGen,
		whiteListerVerifiedTxs: whiteListerVerifiedTxs,
		chainID:                chainID,
		txSignHasher:           txSignHasher,
		feeHandler:             feeHandler,
		txVersionChecker:       txVersionChecker,
	}

	txHeader, err := signMarshalizer.Marshal(tx.GetRaw())
	if err != nil {
		return nil, err
	}

	err = inTx.processFields(txBuff, txHeader)
	if err != nil {
		return nil, err
	}

	return inTx, nil
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
	err := inTx.integrity(inTx.tx)
	if err != nil {
		return err
	}

	return nil
}

// CheckTXSignature checks if the received transaction has valid signature
func (inTx *InterceptedTransaction) CheckTXSignature() error {
	// check signature structure
	if len(inTx.tx.Signature) == 0 ||
		len(inTx.tx.Signature) > core.MaxPermissionSigners {
		return common.ErrInvalidSignatureLength
	}

	dupCheck := make(map[string]bool)
	for _, s1 := range inTx.tx.Signature {
		// check DUP sign
		if dupCheck[string(s1)] {
			// same signature was added more than one time
			return common.ErrDupSignature
		}
		dupCheck[string(s1)] = true
	}

	return nil
}

func (inTx *InterceptedTransaction) processFields(txBuff []byte, txHeader []byte) error {
	inTx.hash = inTx.txSignHasher.Compute(string(txHeader))
	return nil
}

// integrity checks for not nil fields and negative value
func (inTx *InterceptedTransaction) integrity(tx *transaction.Transaction) error {
	err := inTx.txVersionChecker.CheckTxVersion(tx)
	if err != nil {
		return err
	}

	if !bytes.Equal(tx.RawData.GetChainID(), inTx.chainID) {
		return process.ErrInvalidChainID
	}

	if len(tx.RawData.Sender) != inTx.pubkeyConv.Len() ||
		bytes.Equal(tx.RawData.Sender, core.ZeroAddress) ||
		bytes.Equal(tx.RawData.Sender, core.BlackHoleAddress) {
		return process.ErrInvalidSndAddr
	}

	if len(tx.RawData.Contract) == 0 || len(tx.RawData.Contract) > core.MaxLengthOfContracts {
		return process.ErrInvalidTransactionNoContract
	}

	if err := tx.Validate(); err != nil {
		return err
	}

	_, err = inTx.feeHandler.CheckValidityTxValues(tx, false)
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

// ValidatePermission -
func (inTx *InterceptedTransaction) ValidatePermission(permission []byte) error {
	if inTx.tx != nil {
		return inTx.tx.ValidatePermission(permission)
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
