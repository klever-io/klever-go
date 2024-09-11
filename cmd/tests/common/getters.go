package common

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/klever-io/klever-go/cmd/operator/utils"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/network/api/asset"
)

type PoolResponse struct {
	Data struct {
		Pool struct {
			KDA        string  `json:"KDA"`
			KLVBalance float64 `json:"KLVBalance"`
			KDABalance float64 `json:"KDABalance"`
			FRatioKDA  int32   `json:"FRatioKDA"`
			FRatioKLV  int32   `json:"FRatioKLV"`
		} `json:"pool"`
	} `json:"data"`
}

func GetAssetId(tx *data.Transaction) (string, error) {
	if len(tx.Receipts) > 1 {
		for _, receipts := range tx.Receipts {
			if receipts["type"] == float64(1) { // CreateKDA
				return receipts["assetId"].(string), nil
			}
		}

	}

	return "", errors.New("cannot found assetId")
}

func GetMarketplaceId(tx *data.Transaction) (string, error) {
	if len(tx.Receipts) > 1 {
		for _, receipts := range tx.Receipts {
			if receipts["type"] == float64(10) { // CreateKDA
				return receipts["marketplaceId"].(string), nil
			}
		}

	}

	return "", errors.New("cannot found assetId")
}

func GetOrderId(tx *data.Transaction) (string, error) {
	if len(tx.Receipts) > 1 {
		for _, receipts := range tx.Receipts {
			if receipts["type"] == float64(15) { // Sell
				return receipts["orderId"].(string), nil
			}
		}

	}

	return "", errors.New("cannot found assetId")
}

func GetKdaPoolId(tx *data.Transaction) (string, error) {
	if len(tx.Receipts) > 1 {
		for _, receipts := range tx.Receipts {
			if receipts["type"] == float64(22) {
				return receipts["poolId"].(string), nil
			}
		}

	}

	return "", errors.New("cannot found poolId")
}

func GetPool(assetId string) (*PoolResponse, error) {
	url := fmt.Sprintf("%s/asset/pool/%s", Args.NodeUrl, assetId)

	var resp *PoolResponse

	if err := utils.GetURL(url, &resp); err != nil {
		return &PoolResponse{}, err
	}

	return resp, nil
}

func GetRemainingTimeToEpoch(plusEpoch uint64) (time.Duration, error) {
	var resp struct {
		Data struct {
			Overview struct {
				EpochNumber      uint64 `json:"epochNumber"`
				SlotPerEpoch     uint64 `json:"slotsPerEpoch"`
				SlotAtEpochStart uint64 `json:"slotAtEpochStart"`
				CurrentSlot      uint64 `json:"currentSlot"`
				SlotDuration     uint64 `json:"slotDuration"`
			} `json:"overview"`
		} `json:"data"`
	}

	url := fmt.Sprintf("%s/node/overview", Args.NodeUrl)
	if err := utils.GetURL(url, &resp); err != nil {
		return 0, err
	}

	remainingEpochsSlots := (resp.Data.Overview.EpochNumber + plusEpoch - resp.Data.Overview.EpochNumber - 1) * resp.Data.Overview.SlotPerEpoch
	epochFinishSlot := resp.Data.Overview.SlotAtEpochStart + resp.Data.Overview.SlotPerEpoch
	slotsRemained := epochFinishSlot - resp.Data.Overview.CurrentSlot + remainingEpochsSlots

	if epochFinishSlot < resp.Data.Overview.CurrentSlot {
		time.Sleep(1 * time.Second)
		return GetRemainingTimeToEpoch(plusEpoch)
	}

	// #nosec G115
	return time.Duration(slotsRemained*resp.Data.Overview.SlotDuration) * time.Millisecond, nil
}

func GetAsset(assetId string) (*data.Asset, error) {

	url := fmt.Sprintf("%s/asset/%s", os.Getenv("KLEVER_PROXY"), assetId)

	var body struct {
		Data struct {
			Asset *data.Asset `json:"asset"`
			Error string      `json:"error"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return nil, err
	}

	return body.Data.Asset, nil
}

func GetNFT(nftID string, owner string) (*asset.NFTAsset, error) {
	url := fmt.Sprintf("%s/asset/nft/%s/%s", os.Getenv("KLEVER_NODE"), owner, nftID)

	var body struct {
		Data struct {
			Asset *asset.NFTAsset `json:"asset"`
			Error string          `json:"error"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return nil, err
	}

	return body.Data.Asset, nil
}

func GetITO(assetId string) (*data.ITOInfo, error) {

	url := fmt.Sprintf("%s/ito/%s", os.Getenv("KLEVER_PROXY"), assetId)

	var body struct {
		Data struct {
			ITO   *data.ITOInfo `json:"ito"`
			Error string        `json:"error"`
		} `json:"data"`
		Error string `json:"error"`
	}

	time.Sleep(TX_SLEEP_TIME)

	if err := utils.GetURL(url, &body); err != nil {
		return nil, err
	}

	return body.Data.ITO, nil
}
