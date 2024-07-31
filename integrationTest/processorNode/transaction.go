package processorNode

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	procTx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/versioning"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// GetTransaction gets the transaction based on the given hash. It will search in the cache and the storage and
// will return the transaction in a format which can be respected by all types of transactions (normal, reward or unsigned)
func (n *ProcessorNode) GetTransaction(txHash []byte, withResults bool) (*api.Transaction, error) {
	tx, err := n.optionallyGetTransactionFromPool(txHash)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		return n.prepareNormalTx(tx)
	}

	return n.getTransactionFromStorage(txHash)
}

// CreateTransaction will return a transaction from all the required fields
func (n *ProcessorNode) CreateTransaction(
	txType uint32,
	sender string,
	nonce uint64,
	senderUsername []byte,
	dataField [][]byte,
	permID int32,
	contracts []json.RawMessage,
) (*transaction.Transaction, []byte, error) {
	if check.IfNil(n.AddressPubkeyConverter) {
		return nil, nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.AccountsAdapter) {
		return nil, nil, common.ErrNilAccountsAdapter
	}
	if len(sender) > 96 { //todo: add variable in processor node
		return nil, nil, fmt.Errorf("%w for sender", common.ErrInvalidAddressLength)
	}
	if len(senderUsername) > core.MaxUserNameLength {
		return nil, nil, common.ErrInvalidSenderUsernameLength
	}
	if len(dataField) > tools.MegabyteSize {
		return nil, nil, common.ErrDataFieldTooBig
	}
	if len(contracts) == 0 || len(contracts) > core.MaxLengthOfContracts {
		return nil, nil, common.ErrInvalidContract
	}

	senderAddress, err := TestAddressPubkeyConverter.Decode(sender)
	if err != nil {
		return nil, nil, common.ErrBech32ConvertError
	}

	tx := transaction.NewBaseTransaction(senderAddress, nonce, dataField, 0, 0)
	_ = tx.SetChainID(n.ChainID)
	tx.RawData.PermissionID = permID

	activeParameters, err := n.GetProposalParameters()
	if err != nil {
		return nil, nil, err
	}

	for _, c := range contracts {
		txArgs := transaction.TXArgs{
			Type:             txType,
			Sender:           senderAddress,
			Data:             dataField,
			Contract:         c,
			NodeHelper:       n.Node,
			ActiveParameters: activeParameters,
		}

		err = tx.AddTransaction(txArgs)
		if err != nil {
			return nil, nil, err
		}
	}

	// add tx version
	tx.RawData.Version = n.MinTransactionVersion
	cost, err := n.EconomicsData.ComputeTransactionCost(tx, true)
	if err != nil {
		return nil, nil, err
	}
	tx.RawData.BandwidthFee = cost.BandwidthFee
	tx.RawData.KAppFee = cost.KAppFee
	// add up estimated gas into BW
	if cost.GasMultiplier > 0 && cost.GasEstimated > 0 {
		tx.RawData.BandwidthFee += int64(cost.GasEstimated / cost.GasMultiplier)
	}

	// review validation
	err = n.ValidateTransaction(tx, false)
	if err != nil {
		return nil, nil, err
	}

	var txHash []byte
	txHash, err = tools.CalculateHash(n.InternalMarshalizer, n.Hasher, tx.GetRaw())
	if err != nil {
		return nil, nil, err
	}

	tx.Signature = make([][]byte, 1)

	return tx, txHash, nil
}

func (n *ProcessorNode) unmarshalTransaction(txBytes []byte) (*api.Transaction, error) {
	var tx transaction.Transaction
	err := n.InternalMarshalizer.Unmarshal(&tx, txBytes)
	if err != nil {
		return nil, err
	}
	return n.prepareNormalTx(&tx)
}

func (n *ProcessorNode) prepareNormalTx(tx *transaction.Transaction) (*api.Transaction, error) {

	hash, err := tools.CalculateHash(n.InternalMarshalizer, n.Hasher, tx.RawData)
	if err != nil {
		return nil, err
	}

	signature := make([]string, 0)
	for _, s := range tx.Signature {
		signature = append(signature, hex.EncodeToString(s))
	}

	status := "pending"
	if tx.Block > 0 {
		status = "onChain"
	}

	return &api.Transaction{
		Transaction: tx,
		Hash:        hex.EncodeToString(hash),
		Signature:   signature,
		Status:      status,
	}, nil
}

func (n *ProcessorNode) optionallyGetTransactionFromPool(hash []byte) (*transaction.Transaction, error) {
	txsPool := n.DataPool.Transactions()
	txObj, found := txsPool.SearchFirstData(hash)
	if !found || txObj == nil {
		return nil, nil
	}

	tx, ok := txObj.(*transaction.Transaction)
	if !ok {
		log.Debug("optionallyGetTransactionFromPool", "error", common.ErrWrongTypeAssertion.Error())
		return nil, common.ErrWrongTypeAssertion
	}

	return tx, nil
}

func (n *ProcessorNode) getTransactionFromStorage(hash []byte) (*api.Transaction, error) {
	txsStorer := n.Store.GetStorer(retriever.TransactionUnit)
	txBytes, err := txsStorer.SearchFirst(hash)
	if err != nil || txBytes == nil {
		return nil, common.ErrTransactionNotFound
	}

	return n.unmarshalTransaction(txBytes)
}

// SendTransaction sends the provided transaction to the network
func (n *ProcessorNode) SendTransaction(tx *transaction.Transaction) (string, error) {
	// compute hash
	txHash, err := tools.CalculateHash(n.InternalMarshalizer, n.Hasher, tx.GetRaw())
	if err != nil {
		return "", err
	}

	err = n.ValidateTransaction(tx, true)
	if err != nil {
		return "", err
	}

	n.addTransactionsToSendPipe([]*transaction.Transaction{tx})

	return hex.EncodeToString(txHash), nil
}

// SendBulkTransactions sends the provided transactions as a bulk, optimizing transfer between nodes
func (n *ProcessorNode) SendBulkTransactions(txs []*transaction.Transaction) ([]string, error) {
	if len(txs) == 0 {
		return nil, common.ErrNoTxToProcess
	}

	txsHashes := make([]string, 0)
	for i, tx := range txs {

		err := n.ValidateTransaction(tx, true)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction %d: %w", i, err)
		}

		txHash, err := tools.CalculateHash(n.InternalMarshalizer, n.Hasher, tx.GetRaw())
		if err != nil {
			return nil, err
		}

		txsHashes = append(txsHashes, hex.EncodeToString(txHash))
	}

	n.addTransactionsToSendPipe(txs)

	return txsHashes, nil
}

func (n *ProcessorNode) commonTransactionValidation(
	tx *transaction.Transaction,
	whiteListerVerifiedTxs process.WhiteListHandler,
	whiteListRequest process.WhiteListHandler,
	checkSignature bool,
) (process.TxValidator, process.TxValidatorHandler, error) {

	txSingleSigner := n.TxSingleSigner
	if !checkSignature {
		txSingleSigner = &disabledSig.DisabledSingleSig{}
	}

	txValidator, err := dataValidators.NewTxValidator(
		n.AccountsAdapter,
		n.Store.GetStorer(retriever.TransactionUnit),
		n.DataPool,
		whiteListRequest,
		n.AddressPubkeyConverter,
		txSingleSigner,
		n.NodeAccount.KeygenTxSign,
		n.KappController,
		core.MaxTxNonceDeltaAllowed,
	)

	if err != nil {
		return nil, nil, err
	}

	marshalizedTx, err := n.InternalMarshalizer.Marshal(tx)
	if err != nil {
		return nil, nil, err
	}

	intTx, err := procTx.NewInterceptedTransaction(
		marshalizedTx,
		n.InternalMarshalizer,
		n.TxSignMarshalizer,
		n.Hasher,
		n.NodeAccount.KeygenTxSign,
		txSingleSigner,
		n.AddressPubkeyConverter,
		whiteListerVerifiedTxs,
		n.ChainID,
		n.TxSignHasher,
		n.FeeHandler,
		versioning.NewTxVersionChecker(n.MinTransactionVersion),
	)
	if err != nil {
		return nil, nil, err
	}

	err = txValidator.CheckDup(intTx.Hash())
	if err != nil {
		return nil, nil, err
	}

	err = intTx.CheckValidity()
	if err != nil {
		return nil, nil, err
	}

	if checkSignature {
		err = intTx.CheckTXSignature()
		if err != nil {
			return nil, nil, err
		}
	}

	return txValidator, intTx, nil
}

// ValidateTransaction will validate a transaction
func (n *ProcessorNode) ValidateTransaction(tx *transaction.Transaction, checkSignature bool) error {

	txValidator, intTx, err := n.commonTransactionValidation(tx, n.WhiteListerVerifiedTxs, n.WhiteListHandler, checkSignature)
	if err != nil {
		return err
	}

	return txValidator.CheckTxValidity(intTx)
}
