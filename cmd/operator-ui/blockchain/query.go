package blockchain

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/klever-io/klever-go/indexer/data"

	logger "github.com/klever-io/klever-go-logger"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/kapps"
	json "github.com/nwidger/jsoncolor"
)

var (
	nodeAPI  = getEnv("KLEVER_NODE", "https://node.testnet.klever.finance")
	proxyAPI = getEnv("KLEVER_PROXY", "https://node.testnet.klever.finance")
	log      = logger.GetOrCreate("operator-ui")

	walletPubKeyConverter, _    = pubkeyConverter.NewBech32PubkeyConverter(txSignPubkeyLen)
	validatorPubKeyConverter, _ = pubkeyConverter.NewHexPubkeyConverter(blsPubkeyLen)
)

func getEnv(env string, callback string) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}

	return callback
}

func GetBalance(addr string) (float64, error) {
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

	err := utils.GetURL(fmt.Sprintf("%s/address/%s", nodeAPI, addr), &result)
	if err != nil {
		return 0, err
	}

	return float64(result.Data.Account.Balance) / math.Pow10(6), nil
}

const blsPubkeyLen = 96
const txSignPubkeyLen = 32

var (
	WalletPubKeyConverter, _    = pubkeyConverter.NewBech32PubkeyConverter(txSignPubkeyLen)
	ValidatorPubKeyConverter, _ = pubkeyConverter.NewHexPubkeyConverter(blsPubkeyLen)
)

type Account struct {
	*data.AccountInfo
	Assets map[string]*data.AccountKDA `json:"assets"`
}

func GetAccountData(addr string) (*Account, error) {
	result := struct {
		Data struct {
			Account *Account `json:"account"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/address/%s", proxyAPI, addr), &result)
	if err != nil {
		return nil, err
	}

	return result.Data.Account, nil
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

func getAccountNonce(addr string) (uint64, error) {
	result := struct {
		Data struct {
			Nonce     uint64 `json:"nonce"`
			TxPending uint64 `json:"txPending"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/address/%s/nonce", nodeAPI, addr), &result)
	if err != nil {
		return 0, err
	}

	return result.Data.Nonce + result.Data.TxPending, nil
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

func fecthTX(hash string) (*api.Transaction, error) {
	result := struct {
		Data struct {
			TX      *api.Transaction `json:"transaction"`
			Receipt [][]string       `json:"receipt"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	log.Info("getting transaction", "hash", hash)

	err := utils.GetURL(fmt.Sprintf("%s/transaction/%s", nodeAPI, hash), &result)
	if err != nil {
		return nil, err
	}

	if result.Data.TX == nil {
		return nil, fmt.Errorf("invalid TX, if the transaction hash is correct, please try with a archive node")
	}

	return result.Data.TX, nil
}

func getTXByID(hash string) error {
	tx, err := fecthTX(hash)
	if err != nil {
		return err
	}

	blockResult := struct {
		Data struct {
			Block *api.Block `json:"block"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err = utils.GetURL(fmt.Sprintf("%s/block/by-nonce/%d", nodeAPI, tx.Block), &blockResult)
	if err != nil {
		return err
	}

	cp, err := indexer.NewCommonProcessor(walletPubKeyConverter, validatorPubKeyConverter)
	if err != nil {
		return err
	}
	txDecoded := cp.BuildTransaction(tx.Transaction, hash, blockResult.Data.Block)
	if err != nil {
		return err
	}

	if txDecoded.BlockNum <= 0 {
		txDecoded.Status = "pending"
		txDecoded.ResultCode = ""
	}

	err = cp.DecodeContract(txDecoded, tx.Transaction, nil, nil, blockResult.Data.Block.Block.GetTimestamp())
	if err != nil {
		return err
	}

	// Make a custom formatter with indent set
	// create custom formatter
	f := json.NewFormatter()
	f.Indent = "    "

	// marshal v with custom formatter,
	// dst contains colorized output
	dst, err := json.MarshalIndentWithFormatter(txDecoded, "", "    ", f)
	if err != nil {
		return err
	}

	// print colorized output to stdout
	fmt.Println(string(dst))

	return nil
}

func GetAssetData(assetID string) (*kapps.KDAData, error) {
	result := struct {
		Data struct {
			Asset kapps.KDAData `json:"asset"`
		} `json:"data"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}{}

	err := utils.GetURL(fmt.Sprintf("%s/asset/%s", nodeAPI, assetID), &result)
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
		kda, err := GetAssetData(kdaID)
		if err != nil {
			return 0, 0, err
		}

		precision = kda.Precision
	}

	allowance := float64(result.Data.Allowance) / math.Pow10(int(precision))
	stakingRewards := float64(result.Data.StakingRewards) / math.Pow10(int(precision))

	return int64(allowance), int64(stakingRewards), nil
}
