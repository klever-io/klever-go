package common

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/klever-io/klever-go/kapps"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
)

func CheckTransaction(hash string, times int) (string, *data.Transaction, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return "", nil, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving transaction.\n", times)

	url := fmt.Sprintf("%s/transaction/%s", os.Getenv("KLEVER_PROXY"), hash)

	var body struct {
		Data struct {
			Transaction data.Transaction `json:"transaction"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return "", nil, err
	}

	if body.Error != "" {
		return CheckTransaction(hash, times+1)
	}

	fmt.Println()

	return body.Data.Transaction.ResultCode, &body.Data.Transaction, nil
}

func CheckTransactionOnNode(hash string, times int) (bool, *transaction.Transaction, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return false, nil, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving transaction.\n", times)

	url := fmt.Sprintf("%s/transaction/%s", os.Getenv("KLEVER_NODE"), hash)

	var body struct {
		Data struct {
			Transaction transaction.Transaction `json:"transaction"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return false, nil, err
	}

	if body.Data.Transaction.ResultCode != 0 || body.Data.Transaction.Result != 0 {
		return false, &body.Data.Transaction, nil
	}

	if body.Error != "" || len(body.Data.Transaction.Receipts) == 0 {
		return CheckTransactionOnNode(hash, times+1)
	}

	fmt.Println()

	return true, &body.Data.Transaction, nil
}

func CheckTransactionWithResultsOnNode(hash string, times int) (bool, *api.Transaction, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return false, nil, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving transaction.\n", times)

	url := fmt.Sprintf("%s/transaction/%s?withResults=true", os.Getenv("KLEVER_NODE"), hash)

	var body struct {
		Data struct {
			Transaction api.Transaction `json:"transaction"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return false, nil, err
	}

	if body.Data.Transaction.ResultCode != 0 || body.Data.Transaction.Result != 0 {
		return false, &body.Data.Transaction, nil
	}

	if body.Error != "" ||
		len(body.Data.Transaction.Receipts) == 0 ||
		body.Data.Transaction.Logs == nil ||
		len(body.Data.Transaction.Logs.Events) == 0 {
		return CheckTransactionWithResultsOnNode(hash, times+1)
	}

	fmt.Println()

	return true, &body.Data.Transaction, nil
}

func GetAssetOnNode(asset string, times int) (bool, *kapps.KDAData, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return false, nil, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving asset.\n", times)

	url := fmt.Sprintf("%s/asset/%s", os.Getenv("KLEVER_NODE"), asset)

	var body struct {
		Data struct {
			Asset kapps.KDAData `json:"asset"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return false, nil, err
	}

	if body.Error != "" {
		return GetAssetOnNode(asset, times+1)
	}

	return true, &body.Data.Asset, nil
}

func CheckTransactions(times int, hashes ...string) ([]string, []*data.Transaction, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return nil, nil, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving %d transactions.\n", times, len(hashes))

	txs := make([]*data.Transaction, len(hashes))
	results := make([]string, len(hashes))

	var wg sync.WaitGroup

	wg.Add(len(hashes))

	var err []error
	for idx, hash := range hashes {

		go func(hash string, idx int) {
			defer wg.Done()

			url := fmt.Sprintf("%s/transaction/%s", os.Getenv("KLEVER_PROXY"), hash)

			var body struct {
				Data struct {
					Transaction data.Transaction `json:"transaction"`
					Error       string           `json:"error"`
				} `json:"data"`
				Error string `json:"error"`
			}

			time.Sleep(TX_SLEEP_TIME)

			if getErr := utils.GetURL(url, &body); getErr != nil {
				err = append(err, getErr)
				return
			}

			if body.Error != "" {
				err = append(err, errors.New(body.Error))
				return
			}

			txs[idx] = &body.Data.Transaction
			results[idx] = body.Data.Transaction.ResultCode
		}(hash, idx)
	}

	wg.Wait()

	if len(err) > 0 {
		return CheckTransactions(times+1, hashes...)
	}

	fmt.Println()

	return results, txs, nil
}

func GetBalanceOnNode(address, asset string, times int) (int64, error) {
	if timesInt, err := strconv.Atoi(os.Getenv("RETRY")); err == nil && times == timesInt {
		return 0, fmt.Errorf("exceed trying times request")
	}

	fmt.Printf("%d time retrieving balance.\n", times)

	url := fmt.Sprintf("%s/address/%s/balance?asset=%s", os.Getenv("KLEVER_NODE"), address, asset)

	var body struct {
		Data struct {
			Balance int64 `json:"balance"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return 0, err
	}

	if body.Error != "" {
		return GetBalanceOnNode(address, asset, times+1)
	}

	return body.Data.Balance, nil
}
