package processorNode

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

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
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

const maxEncodedAddressLength = 62

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

func (n *ProcessorNode) validateCreateTransactionInputs(
	sender string,
	senderUsername []byte,
	dataField [][]byte,
	contracts []json.RawMessage,
) error {
	if check.IfNil(n.AddressPubkeyConverter) {
		return common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.AccountsAdapter) {
		return common.ErrNilAccountsAdapter
	}
	if len(sender) > maxEncodedAddressLength {
		return fmt.Errorf("%w for sender", common.ErrInvalidAddressLength)
	}
	if len(senderUsername) > core.MaxUserNameLength {
		return common.ErrInvalidSenderUsernameLength
	}
	if len(dataField) > tools.MegabyteSize {
		return common.ErrDataFieldTooBig
	}
	if len(contracts) == 0 || len(contracts) > core.MaxLengthOfContracts {
		return common.ErrInvalidContract
	}

	return nil
}

// Helper function to add contracts to the transaction
func (n *ProcessorNode) addContractsToTransaction(
	tx *transaction.Transaction,
	txType uint32,
	senderAddress []byte,
	dataField [][]byte,
	contracts []json.RawMessage,
	activeParameters map[int32]*kapps.Parameter,
) error {
	for _, c := range contracts {
		txArgs := transaction.TXArgs{
			Type:             txType,
			Sender:           senderAddress,
			Data:             dataField,
			Contract:         c,
			NodeHelper:       n.Node,
			ActiveParameters: activeParameters,
		}

		if err := tx.AddTransaction(txArgs); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to compute the transaction cost
func (n *ProcessorNode) computeTransactionCost(tx *transaction.Transaction) error {
	cost, err := n.EconomicsData.ComputeTransactionCost(tx, true)
	if err != nil {
		return err
	}
	tx.RawData.BandwidthFee = cost.BandwidthFee
	tx.RawData.KAppFee = cost.KAppFee

	// Add up estimated gas into BandwidthFee
	if cost.GasMultiplier > 0 && cost.GasEstimated > 0 {
		value := cost.GasEstimated / cost.GasMultiplier
		if value > math.MaxInt64 {
			return common.ErrEstimateGasTooBig
		}

		// Add FreeBandwidth to BandwidthFee to cover SC execution costs
		tx.RawData.BandwidthFee, err = tools.SafeAddI64(
			tx.RawData.BandwidthFee,
			value,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// Helper function to compute the transaction hash
func (n *ProcessorNode) computeTransactionHash(tx *transaction.Transaction) ([]byte, error) {
	txHash, err := tools.CalculateHash(n.InternalMarshalizer, n.Hasher, tx.GetRaw())
	if err != nil {
		return nil, err
	}

	return txHash, nil
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
	if err := n.validateCreateTransactionInputs(sender, senderUsername, dataField, contracts); err != nil {
		return nil, nil, err
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

	if err := n.addContractsToTransaction(
		tx,
		txType,
		senderAddress,
		dataField,
		contracts,
		activeParameters,
	); err != nil {
		return nil, nil, err
	}

	// add tx version
	tx.RawData.Version = n.MinTransactionVersion
	if err := n.computeTransactionCost(tx); err != nil {
		return nil, nil, err
	}

	// review validation
	err = n.ValidateTransaction(tx, false)
	if err != nil {
		return nil, nil, err
	}

	txHash, err := n.computeTransactionHash(tx)
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

	status := api.TRANSACTION_STATUS_PENDING
	if tx.Block > 0 {
		status = api.TRANSACTION_STATUS_ON_CHAIN
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
		&procTx.InterceptedTransactionArgs{
			TxBuff:                 marshalizedTx,
			ProtoMarshalizer:       n.InternalMarshalizer,
			SignMarshalizer:        n.TxSignMarshalizer,
			Hasher:                 n.Hasher,
			KeyGen:                 n.NodeAccount.KeygenTxSign,
			Signer:                 txSingleSigner,
			PubkeyConv:             n.AddressPubkeyConverter,
			WhiteListerVerifiedTxs: whiteListerVerifiedTxs,
			ChainID:                n.ChainID,
			TxSignHasher:           n.TxSignHasher,
			FeeHandler:             n.FeeHandler,
			TxVersionChecker:       versioning.NewTxVersionChecker(n.MinTransactionVersion),
			ForkController:         n.ForkController,
		},
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
