package node

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/network/api/models"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// GetTransaction gets the transaction based on the given hash. It will search in the cache and the storage and
// will return the transaction in a format which can be respected by all types of transactions (normal, reward or unsigned)
func (n *Node) GetTransaction(txHash string, withResults bool) (*api.Transaction, error) {
	hash, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, err
	}

	tx, err := n.optionallyGetTransactionFromPool(hash)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		return n.prepareNormalTx(tx)
	}

	stx, err := n.getTransactionFromStorage(hash)
	if err != nil {
		return nil, err
	}

	if withResults && stx.ResultCode != transaction.Transaction_AccountError {
		// check if any contract has SC txs
		for _, c := range stx.RawData.Contract {
			if c.Type == transaction.TXContract_SmartContractType {
				// skip error
				stx.Logs, _ = n.getLogsFromStorage(hash)
				break
			}
		}

	}

	return stx, nil
}

// TXPool gets the transaction list from mem pool
func (n *Node) TXPool(sender string, page int, pageSize int) ([]*api.Transaction, int, error) {
	var data []interface{}
	var total int

	txsPool := n.dataPool.Transactions()

	if len(sender) > 0 {
		addr, err := n.addressPubkeyConverter.Decode(sender)
		if err != nil {
			return nil, 0, err
		}
		data, total = txsPool.GetSenderPaginated("0", addr, page, pageSize)
	} else {
		data, total = txsPool.GetPaginated("0", page, pageSize)
	}

	txs := make([]*api.Transaction, len(data))
	var err error
	for i, txObj := range data {
		if txObj == nil {
			continue
		}
		tx, ok := txObj.(*transaction.Transaction)
		if !ok || tx == nil {
			continue
		}
		txs[i], err = n.prepareNormalTx(tx)
		if err != nil {
			continue
		}
	}

	return txs, total, nil
}

// CreateTransaction will return a transaction from all the required fields
func (n *Node) CreateTransaction(
	txType uint32,
	base *transaction.TXBaseInfo,
	contracts []json.RawMessage,
	skipValidate bool,
) (*transaction.Transaction, []byte, error) {
	if check.IfNil(n.addressPubkeyConverter) {
		return nil, nil, common.ErrNilPubkeyConverter
	}
	if check.IfNil(n.accounts) {
		return nil, nil, common.ErrNilAccountsAdapter
	}
	if len(base.Sender) > n.encodedAddressLength {
		return nil, nil, fmt.Errorf("%w for sender", common.ErrInvalidAddressLength)
	}
	if len(base.SenderUsername) > core.MaxUserNameLength {
		return nil, nil, common.ErrInvalidSenderUsernameLength
	}
	if len(base.DataField) > tools.MegabyteSize {
		return nil, nil, common.ErrDataFieldTooBig
	}
	if len(contracts) == 0 || len(contracts) > core.MaxLengthOfContracts {
		return nil, nil, common.ErrInvalidContract
	}

	senderAddress, err := n.addressPubkeyConverter.Decode(base.Sender)
	if err != nil {
		return nil, nil, common.ErrBech32ConvertError
	}

	tx := transaction.NewBaseTransaction(senderAddress, base.Nonce, base.DataField, 0, 0)
	err = tx.SetChainID(n.chainID)
	if err != nil {
		return nil, nil, err
	}
	tx.RawData.PermissionID = base.PermID

	activeParameters, err := n.GetProposalParameters()
	if err != nil {
		return nil, nil, err
	}

	for _, c := range contracts {
		var contract models.ContractInfo

		if err := json.Unmarshal(c, &contract); err != nil {
			return nil, nil, err
		}

		contractType := txType
		if contract.ContractType != nil {
			contractType = *contract.ContractType
		}

		txArgs := transaction.TXArgs{
			Type:             contractType,
			Sender:           senderAddress,
			Data:             base.DataField,
			Contract:         c,
			NodeHelper:       n,
			ActiveParameters: activeParameters,
		}

		err = tx.AddTransaction(txArgs)
		if err != nil {
			return nil, nil, err
		}
	}

	// add tx version
	tx.RawData.Version = n.minTransactionVersion

	cost, err := n.feeHandler.ComputeTransactionCost(tx, true)
	if err != nil {
		return tx, nil, err
	}

	tx.RawData.BandwidthFee = cost.BandwidthFee
	tx.RawData.KAppFee = cost.KAppFee
	// add up estimated gas into BW
	if cost.GasMultiplier > 0 && cost.GasEstimated > 0 {
		tx.RawData.BandwidthFee += int64(cost.GasEstimated / cost.GasMultiplier)
	}

	// compute KDAFee if any
	if len(base.KDAFee) > 0 && base.KDAFee != string(kdautils.KLVIdentifier) {
		tx.RawData.KDAFee = &transaction.Transaction_KDAFee{
			KDA: []byte(base.KDAFee),
		}

		totalFees := tx.RawData.BandwidthFee + tx.RawData.KAppFee

		// check if KDA has a feePool set
		kdaAmount, err := n.kappController.GetKDAFeesPoolKApp().Compute(totalFees, tx.RawData.KDAFee)
		if err != nil {
			return nil, nil, err
		}

		if kdaAmount <= 0 {
			return tx, nil, common.ErrAssetPoolAmountError
		}

		tx.RawData.KDAFee.Amount = kdaAmount
	}

	if skipValidate { // skip validation to only estimate fee
		return tx, nil, nil
	}

	// review validation
	err = n.ValidateTransaction(tx, false)
	if err != nil {
		return tx, nil, err
	}

	var txHash []byte
	txHash, err = tools.CalculateHash(n.internalMarshalizer, n.hasher, tx.GetRaw())
	if err != nil {
		return tx, nil, err
	}

	return tx, txHash, nil

}

// DecodeTransaction sends the provided transaction to the network
func (n *Node) DecodeTransaction(tx *transaction.Transaction) (*indexerData.Transaction, error) {
	cp, err := indexer.NewCommonProcessor(n.addressPubkeyConverter, n.validatorPubkeyConverter)
	if err != nil {
		return nil, err
	}

	var parsedData []string
	for _, d := range tx.RawData.Data {
		parsedData = append(parsedData, hex.EncodeToString(d))
	}

	decodedTX := &indexerData.Transaction{
		Sender:       n.addressPubkeyConverter.Encode(tx.RawData.Sender),
		Nonce:        tx.RawData.Nonce,
		PermissionID: tx.RawData.PermissionID,
		Data:         parsedData,
		KAppFee:      tx.RawData.KAppFee,
		BandwidthFee: tx.RawData.BandwidthFee,
		ResultCode:   tx.ResultCode.String(),
		Version:      tx.RawData.Version,
		ChainID:      string(tx.RawData.ChainID),
	}

	err = cp.DecodeContract(decodedTX, tx, nil, nil, time.Now().Unix())
	if err != nil {
		return nil, err
	}

	hash, err := tools.CalculateHash(n.txSignMarshalizer, n.txSignHasher, tx.RawData)
	if err != nil {
		return nil, err
	}

	decodedTX.Hash = hex.EncodeToString(hash)

	return decodedTX, nil
}

func (n *Node) unmarshalTransaction(txBytes []byte) (*api.Transaction, error) {
	var tx transaction.Transaction
	err := n.internalMarshalizer.Unmarshal(&tx, txBytes)
	if err != nil {
		return nil, err
	}
	return n.prepareNormalTx(&tx)
}

func (n *Node) prepareNormalTx(tx *transaction.Transaction) (*api.Transaction, error) {

	hash, err := tools.CalculateHash(n.internalMarshalizer, n.hasher, tx.RawData)
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

func (n *Node) optionallyGetTransactionFromPool(hash []byte) (*transaction.Transaction, error) {
	txsPool := n.dataPool.Transactions()
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

func (n *Node) getTransactionFromStorage(hash []byte) (*api.Transaction, error) {
	txsStorer := n.store.GetStorer(retriever.TransactionUnit)
	txBytes, err := txsStorer.SearchFirst(hash)
	if err != nil || txBytes == nil {
		return nil, common.ErrTransactionNotFound
	}

	return n.unmarshalTransaction(txBytes)
}

func (n *Node) getLogsFromStorage(hash []byte) (*api.Logs, error) {
	logsStorer := n.store.GetStorer(retriever.TxLogsUnit)
	logsBytes, err := logsStorer.SearchFirst(hash)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", common.ErrTransactionNotFound, err)
	}

	return n.unmarshalLogs(logsBytes)
}

func (n *Node) unmarshalLogs(logsBytes []byte) (*api.Logs, error) {
	if len(logsBytes) == 0 {
		return nil, nil
	}

	var log transaction.Log
	err := n.internalMarshalizer.Unmarshal(&log, logsBytes)
	if err != nil {
		return nil, err
	}

	return n.prepareNormalLog(&log)
}

func (n *Node) prepareNormalLog(log *transaction.Log) (*api.Logs, error) {

	events := make([]*api.Events, 0)
	for _, e := range log.Events {
		topics := make([]string, 0)
		for _, t := range e.Topics {
			topics = append(topics, hex.EncodeToString(t))
		}

		data := make([]string, 0)
		for _, d := range e.Data {
			data = append(data, hex.EncodeToString(d))
		}

		address := ""
		if len(e.Address) > 0 {
			address = n.addressPubkeyConverter.Encode(e.Address)
		}

		events = append(events, &api.Events{
			Address:    address,
			Identifier: string(e.Identifier),
			Topics:     topics,
			Data:       data,
		})
	}

	return &api.Logs{
		Address:    n.addressPubkeyConverter.Encode(log.Address),
		ContractID: log.ContractID,
		Events:     events,
	}, nil
}
