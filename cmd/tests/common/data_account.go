package common

import (
	"errors"
	"fmt"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/indexer/data"
)

type DataAccount struct {
	*data.AccountInfo
	Assets map[string]*data.AccountKDA `json:"assets"`
}

func GetAccount(address string) (*DataAccount, error) {
	url := fmt.Sprintf("%s/address/%s", Args.ProxyUrl, address)

	var body struct {
		Data struct {
			Account DataAccount `json:"account"`
		} `json:"data"`
		Error string `json:"error"`
	}

	if err := utils.GetURL(url, &body); err != nil {
		return nil, err
	}

	if body.Error != "" {
		return nil, errors.New(body.Error)
	}

	return &body.Data.Account, nil
}

func GetAllowance(address, token string) (map[string]int64, error) {
	url := fmt.Sprintf("%s/address/%s/allowance?asset=%s", Args.ProxyUrl, address, token)

	var body struct {
		Data struct {
			Result struct {
				Rewards []struct {
					AssetID   string `json:"assetId"`
					Precision uint32 `json:"precision"`
					Rewards   int64  `json:"rewards"`
				} `json:"allStakingRewards"`
			} `json:"result"`
		} `json:"data"`
		Error string `json:"error"`
	}

	if err := utils.GetURL(url, &body); err != nil {
		return nil, err
	}

	if body.Error != "" {
		return nil, errors.New(body.Error)
	}

	rewards := make(map[string]int64, 0)

	for _, r := range body.Data.Result.Rewards {
		rewards[r.AssetID] = r.Rewards
	}

	return rewards, nil
}
