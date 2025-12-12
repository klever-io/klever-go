package main

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	json "github.com/nwidger/jsoncolor"
)

func accountBalance(addr string) error {
	result := struct {
		Data struct {
			Account struct {
				Address []byte `json:"address"`
				Balance int64  `json:"balance"`
			} `json:"account"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	log.Info("checking balance", "address", addr)

	err := utils.GetURL(fmt.Sprintf("%s/address/%s", nodeAPI, addr), &result)
	if err != nil {
		return err
	}

	log.Info(addr, "balance", fmt.Sprintf("%f", float64(result.Data.Account.Balance)/math.Pow10(6)))

	return nil
}

func getAccountData(addr string) (*state.UserAccountData, error) {
	result := struct {
		Data struct {
			Account state.UserAccountData `json:"account"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/address/%s", nodeAPI, addr), &result)
	if err != nil {
		return nil, err
	}

	return &result.Data.Account, nil
}

type AccountNonce struct {
	Nonce             uint64 `json:"nonce"`
	FirstPendingNonce uint64 `json:"firstPendingNonce"`
	TxPending         uint64 `json:"txPending"`
}

func getAccountNonce(addr string) (uint64, error) {
	result := struct {
		Data  AccountNonce `json:"data"`
		Error string       `json:"error"`
		Code  string       `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/address/%s/nonce", nodeAPI, addr), &result)
	if err != nil {
		return 0, err
	}

	// compute nonce with check type
	var nonce uint64
	switch nonceCheck {
	case NONCE_CHECK_CURRENT:
		nonce = result.Data.Nonce
	case NONCE_CHECK_FIRST_PENDING:
		nonce = result.Data.FirstPendingNonce
	case NONCE_CHECK_PENDING:
		nonce = result.Data.Nonce + result.Data.TxPending
	default:
		return 0, fmt.Errorf("invalid nonce check option")
	}

	return nonce, nil
}

func getAccount(addr string) error {
	data, err := getAccountData(addr)
	if err != nil {
		return err
	}

	allowance, stakingRewards, err := getAllowanceData(addr, string(kdautils.KLVIdentifier))
	if err != nil {
		return err
	}

	rootHash := hex.EncodeToString(data.RootHash)

	bbn := new(big.Float).Quo(new(big.Float).SetInt64(data.Balance), big.NewFloat(math.Pow10(6)))

	result := map[string]interface{}{
		"Address":        addr,
		"RootHash":       rootHash,
		"Name":           data.Name,
		"Nonce":          data.Nonce,
		"Balance":        bbn.String(),
		"Allowance":      fmt.Sprintf("%d", allowance),
		"StakingRewards": fmt.Sprintf("%d", stakingRewards),
	}

	// Make a custom formatter with indent set
	// create custom formatter
	f := json.NewFormatter()
	f.Indent = "    "

	// marshal v with custom formatter,
	// dst contains colorized output
	dst, err := json.MarshalIndentWithFormatter(result, "", "    ", f)
	if err != nil {
		return err
	}

	// print colorized output to stdout
	fmt.Println(string(dst))

	return nil
}

func fetchTX(hash string) (*api.Transaction, error) {
	result := struct {
		Data struct {
			TX      *api.Transaction `json:"transaction"`
			Receipt [][]string       `json:"receipt"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	log.Info("getting transaction", "hash", hash)

	err := utils.GetURL(fmt.Sprintf("%s/transaction/%s?withResults=true", nodeAPI, hash), &result)
	if err != nil {
		return nil, err
	}

	if result.Data.TX == nil {
		return nil, fmt.Errorf("invalid TX, if the transaction hash is correct, please try with a archive node")
	}

	hasSmartContract := false
	// check if any contract is a SmartContract
	for _, contract := range result.Data.TX.RawData.Contract {
		if contract.Type == transaction.TXContract_SmartContractType {
			hasSmartContract = true
			break
		}
	}
	// check if SmartContract transaction with status ok
	// fail if no logs are found
	if hasSmartContract &&
		result.Data.TX.Result == transaction.Transaction_SUCCESS &&
		result.Data.TX.ResultCode == transaction.Transaction_Ok &&
		result.Data.TX.Logs == nil {
		return nil, fmt.Errorf("invalid TX logs, if the transaction hash is correct, please try with a archive node")
	}

	return result.Data.TX, nil
}

func getTXByID(hash string) error {
	tx, err := fetchTX(hash)
	if err != nil {
		return err
	}

	// respond with error if await is set and tx still pending
	if await && tx.Status == api.TRANSACTION_STATUS_PENDING {
		return fmt.Errorf("transaction is still pending")
	}

	blockResult, err := fetchBlockByNonce(tx.Block)
	if err != nil {
		return err
	}

	return formatAndDumpTX(tx, hash, blockResult)
}

func fetchBlockByNonce(nonce uint64) (*api.Block, error) {
	result := struct {
		Data struct {
			Block *api.Block `json:"block"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/block/by-nonce/%d", nodeAPI, nonce), &result)
	if err != nil {
		return nil, err
	}

	if result.Data.Block == nil {
		return nil, fmt.Errorf("invalid block, if the block nonce is correct, please try with a archive node")
	}

	return result.Data.Block, nil
}

func formatAndDumpRawTX(tx *transaction.Transaction) error {
	return formatAndDumpTX(&api.Transaction{
		Transaction: tx,
	}, "", &api.Block{
		Block:        &block.Block{},
		Hash:         "",
		Status:       "",
		Transactions: nil,
	})
}

func formatAndDumpTX(tx *api.Transaction, hash string, block *api.Block) error {
	cp, err := indexer.NewCommonProcessor(walletPubKeyConverter, validatorPubKeyConverter)
	if err != nil {
		return err
	}
	txDecoded := cp.BuildTransaction(tx.Transaction, hash, block)
	if err != nil {
		return err
	}

	if txDecoded.BlockNum <= 0 {
		txDecoded.Status = api.TRANSACTION_STATUS_PENDING
		txDecoded.ResultCode = ""
	}

	// indexer does not index or populate logs, we have to manually populate for the operator
	if txDecoded.Logs == nil && tx.Logs != nil {
		events := make([]*data.Event, 0)
		if tx.Logs.Events != nil {
			for _, event := range tx.Logs.Events {
				events = append(events, &data.Event{
					Address:    event.Address,
					Identifier: event.Identifier,
					Topics:     event.Topics,
					Data:       event.Data,
				})
			}
		}
		txDecoded.Logs = &data.Logs{
			Address:    tx.Logs.Address,
			ContractID: tx.Logs.ContractID,
			Events:     events,
		}
	}

	err = cp.DecodeContract(txDecoded, tx.Transaction, nil, nil, nil, block.GetTimestamp())
	if err != nil {
		return err
	}

	return DumpAsJson(txDecoded)
}

func getAssetData(assetID string) (*kapps.KDAData, error) {
	kdaNonNonce := strings.Split(assetID, "/")[0]

	result := struct {
		Data struct {
			Asset kapps.KDAData `json:"asset"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/asset/%s", nodeAPI, kdaNonNonce), &result)
	if err != nil {
		return nil, err
	}

	return &result.Data.Asset, nil
}

func getAllowanceData(addr, kdaID string) (int64, int64, error) {
	result := struct {
		Data struct {
			Allowance      int64 `json:"allowance"`
			StakingRewards int64 `json:"stakingRewards"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/address/%s/allowance?asset=%s", nodeAPI, addr, kdaID), &result)
	if err != nil {
		return 0, 0, err
	}

	precision := uint32(6)
	if len(kdaID) > 0 && kdaID != string(kdautils.KLVIdentifier) && kdaID != string(kdautils.KFIIdentifier) {
		kda, err := getAssetData(kdaID)
		if err != nil {
			return 0, 0, err
		}

		precision = kda.Precision
	}

	allowance := float64(result.Data.Allowance) / math.Pow10(int(precision))
	stakingRewards := float64(result.Data.StakingRewards) / math.Pow10(int(precision))

	return int64(allowance), int64(stakingRewards), nil
}
