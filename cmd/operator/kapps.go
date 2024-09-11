package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/tools"
	"github.com/spf13/cobra"
)

type ParsedPack struct {
	Kda   string           `form:"kda" json:"kda"`
	Packs []ParsedItemPack `form:"packs" json:"packs"`
}

type ParsedItemPack struct {
	Amount float64 `form:"amount" json:"amount"`
	Price  float64 `form:"price" json:"price"`
}

func subKapps() []*cobra.Command {
	var (
		ID                 string
		name               string
		kda                string
		currency           string
		amount             float64
		price              float64
		reservePrice       float64
		maxAmount          float64
		receiver           string
		status             int32
		limit              int64
		buyType            int32
		marketType         int32
		endTime            int64
		startTimeITO       int64
		endTimeITO         int64
		whitelistStartTime int64
		whitelistEndTime   int64
		whitelistStatus    int32
		packs              []string
		whitelist          map[string]string
	)

	cmdConfigITO := &cobra.Command{
		Use:   "config-ito",
		Short: "config an ITO transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedPacks := make([]ParsedPack, 0)
			for _, p := range packs {
				data := ParsedPack{}
				err := json.Unmarshal([]byte(p), &data)
				if err != nil {
					return err
				}

				parsedPacks = append(parsedPacks, data)
			}

			kdaInfo, err := getAssetData(kda)
			if err != nil {
				return err
			}
			parsedMaxAmount := maxAmount * math.Pow10(int(kdaInfo.Precision))

			packInfo := make(map[string]models.PackInfoRequest)
			for _, p := range parsedPacks {
				packPrecision := uint32(6)
				if p.Kda != string(kdautils.KLVIdentifier) && p.Kda != string(kdautils.KFIIdentifier) {
					packKDA, err := getAssetData(p.Kda)
					if err != nil {
						return err
					}

					packPrecision = packKDA.Precision
				}

				packItems := make([]models.PackItemRequest, 0)
				for _, pItem := range p.Packs {
					parsedItemAmount := pItem.Amount * math.Pow10(int(kdaInfo.Precision))
					parsedItemPrice := pItem.Price * math.Pow10(int(packPrecision))

					packItems = append(packItems, models.PackItemRequest{Amount: int64(parsedItemAmount), Price: int64(parsedItemPrice)})
				}

				packInfo[p.Kda] = models.PackInfoRequest{Packs: packItems}
			}

			parsedWhitelist := make(map[string]models.WhitelistInfoRequest)
			for key, value := range whitelist {
				parsedValue, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid param (%s): %w", key, err)
				}

				parsedWhitelist[key] = models.WhitelistInfoRequest{Limit: parsedValue}
			}

			configITORequest := models.ConfigITOTXRequest{
				KDA:                    kda,
				ReceiverAddress:        receiver,
				Status:                 status,
				MaxAmount:              int64(parsedMaxAmount),
				PackInfo:               packInfo,
				DefaultLimitPerAddress: limit,
				WhitelistStatus:        whitelistStatus,
				WhitelistInfo:          parsedWhitelist,
				WhitelistStartTime:     whitelistStartTime,
				WhitelistEndTime:       whitelistEndTime,
				StartTime:              startTimeITO,
				EndTime:                endTimeITO,
			}

			return configITO(signerAddress, configITORequest)
		},
	}
	cmdConfigITO.Flags().StringVar(&receiver, "receiver", "", "set an ITO receiver address")
	cmdConfigITO.Flags().StringVar(&kda, "kda", "", "set a KDA ID")
	cmdConfigITO.Flags().Float64Var(&maxAmount, "maxAmount", 0, "set an ITO max amount")
	cmdConfigITO.Flags().Int32Var(&status, "status", 0, "set ITO status")
	cmdConfigITO.Flags().Int64Var(&limit, "limitPerAddress", 0, "set limit per address")
	cmdConfigITO.Flags().StringArrayVarP(&packs, "packs", "", []string{}, `--packs='{"kda":"KLV", "packs": [{"amount": 1000, "price": 2},{"amount":2000, "price": 1}]}'`)
	cmdConfigITO.Flags().Int64Var(&startTimeITO, "startTime", 0, "set ITO start time")
	cmdConfigITO.Flags().Int64Var(&endTimeITO, "endTime", 0, "set ITO end time")
	cmdConfigITO.Flags().StringToStringVar(&whitelist, "whitelist", nil, "--whitelist 'klv...=1,klv...=2, klv...=3'")
	cmdConfigITO.Flags().Int32Var(&whitelistStatus, "whitelistStatus", 0, "set ITO whitelist status")
	cmdConfigITO.Flags().Int64Var(&whitelistStartTime, "whitelistStartTime", 0, "set ITO whitelist start time")
	cmdConfigITO.Flags().Int64Var(&whitelistEndTime, "whitelistEndTime", 0, "set ITO whitelist end time")

	cmdTriggerITO := &cobra.Command{
		Use:   "trigger-ito [TRIGGER_TYPE]",
		Short: "trigger an ITO",
		RunE: func(cmd *cobra.Command, args []string) error {
			triggerType, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid trigger type %s: %w", args[0], err)
			}

			parsedPacks := make([]ParsedPack, 0)
			for _, p := range packs {
				data := ParsedPack{}
				err := json.Unmarshal([]byte(p), &data)
				if err != nil {
					return err
				}

				parsedPacks = append(parsedPacks, data)
			}

			kdaInfo, err := getAssetData(kda)
			if err != nil {
				return err
			}
			parsedMaxAmount := maxAmount * math.Pow10(int(kdaInfo.Precision))

			packInfo := make(map[string]models.PackInfoRequest)
			for _, p := range parsedPacks {
				packPrecision := uint32(6)
				if p.Kda != string(kdautils.KLVIdentifier) && p.Kda != string(kdautils.KFIIdentifier) {
					packKDA, err := getAssetData(p.Kda)
					if err != nil {
						return err
					}

					packPrecision = packKDA.Precision
				}

				packItems := make([]models.PackItemRequest, 0)
				for _, pItem := range p.Packs {
					parsedItemAmount := pItem.Amount * math.Pow10(int(kdaInfo.Precision))
					parsedItemPrice := pItem.Price * math.Pow10(int(packPrecision))

					packItems = append(packItems, models.PackItemRequest{Amount: int64(parsedItemAmount), Price: int64(parsedItemPrice)})
				}

				packInfo[p.Kda] = models.PackInfoRequest{Packs: packItems}
			}

			parsedWhitelist := make(map[string]models.WhitelistInfoRequest)
			for key, value := range whitelist {
				parsedValue, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid param (%s): %w", key, err)
				}

				parsedWhitelist[key] = models.WhitelistInfoRequest{Limit: parsedValue}
			}

			triggerRequest := models.ITOTriggerTXRequest{
				TriggerType:            tools.SafeU64ToU32(triggerType),
				KDA:                    kda,
				ReceiverAddress:        receiver,
				Status:                 status,
				MaxAmount:              int64(parsedMaxAmount),
				PackInfo:               packInfo,
				DefaultLimitPerAddress: limit,
				WhitelistStatus:        whitelistStatus,
				WhitelistInfo:          parsedWhitelist,
				WhitelistStartTime:     whitelistStartTime,
				WhitelistEndTime:       whitelistEndTime,
				StartTime:              startTimeITO,
				EndTime:                endTimeITO,
			}

			return newITOTrigger(signerAddress, triggerRequest)
		},
	}
	cmdTriggerITO.Flags().StringVar(&receiver, "receiver", "", "set an ITO receiver address")
	cmdTriggerITO.Flags().StringVar(&kda, "kda", "", "set a KDA ID")
	cmdTriggerITO.Flags().Float64Var(&maxAmount, "maxAmount", 0, "set an ITO max amount")
	cmdTriggerITO.Flags().Int32Var(&status, "status", 0, "set ITO status")
	cmdTriggerITO.Flags().Int64Var(&limit, "limitPerAddress", 0, "set limit per address")
	cmdTriggerITO.Flags().StringArrayVarP(&packs, "packs", "", []string{}, `--packs='{"kda":"KLV", "packs": [{"amount": 1000, "price": 2},{"amount":2000, "price": 1}]}'`)
	cmdTriggerITO.Flags().Int64Var(&startTimeITO, "startTime", 0, "set ITO start time")
	cmdTriggerITO.Flags().Int64Var(&endTimeITO, "endTime", 0, "set ITO end time")
	cmdTriggerITO.Flags().StringToStringVar(&whitelist, "whitelist", nil, "--whitelist 'klv...=1,klv...=2, klv...=3'")
	cmdTriggerITO.Flags().Int32Var(&whitelistStatus, "whitelistStatus", 0, "set ITO whitelist status")
	cmdTriggerITO.Flags().Int64Var(&whitelistStartTime, "whitelistStartTime", 0, "set ITO whitelist start time")
	cmdTriggerITO.Flags().Int64Var(&whitelistEndTime, "whitelistEndTime", 0, "set ITO whitelist end time")

	cmdSetITOPrices := &cobra.Command{
		Use:   "set-ito-prices",
		Short: "set ITO prices transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedPacks := make([]ParsedPack, 0)
			for _, p := range packs {
				data := ParsedPack{}
				err := json.Unmarshal([]byte(p), &data)
				if err != nil {
					return err
				}

				parsedPacks = append(parsedPacks, data)
			}

			return setITOPrices(signerAddress, kda, parsedPacks, "")
		},
	}
	cmdSetITOPrices.Flags().StringVar(&kda, "kda", "", "set a KDA ID")
	cmdSetITOPrices.Flags().StringArrayVarP(&packs, "packs", "", []string{}, `--packs='{"kda":"KLV", "packs": [{"amount": 1000, "price": 2},{"amount":2000, "price": 1}]}'`)

	cmdBuy := &cobra.Command{
		Use:   "buy",
		Short: "buy transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			return buy(signerAddress, ID, kda, amount, buyType, "")
		},
	}
	cmdBuy.Flags().StringVar(&ID, "id", "", "set a buy ID")
	cmdBuy.Flags().StringVar(&kda, "kda", "", "set a KDA ID")
	cmdBuy.Flags().Float64Var(&amount, "amount", 0, "set a buy amount")
	cmdBuy.Flags().Int32Var(&buyType, "type", 0, "set a buy type")

	cmdSell := &cobra.Command{
		Use:   "sell",
		Short: "sell transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			if endTime == 0 {
				endTime = time.Now().AddDate(0, 0, 1).Unix()
			}

			return sell(signerAddress, kda, currency, ID, price, reservePrice, endTime, marketType, "")
		},
	}
	cmdSell.Flags().StringVar(&kda, "kda", "", "set a KDA ID")
	cmdSell.Flags().StringVar(&currency, "currency", "", "set a sell currency")
	cmdSell.Flags().StringVar(&ID, "market", "", "set a sell market ID")
	cmdSell.Flags().Float64Var(&price, "price", 0, "set a sell price")
	cmdSell.Flags().Float64Var(&reservePrice, "reserve", 0, "set a sell reserve price")
	cmdSell.Flags().Int64Var(&endTime, "endTime", 0, "set a sell end time")
	cmdSell.Flags().Int32Var(&marketType, "type", 0, "set a market type")

	cmdCancelMarket := &cobra.Command{
		Use:   "cancel-market [MARKET_ID]",
		Short: "cancel-market transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cancelMarket(signerAddress, args[0], "")
		},
	}

	cmdCreateMarketplace := &cobra.Command{
		Use:   "create-marketplace",
		Short: "create marketplace transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createMarketplace(signerAddress, name, receiver, amount, "")
		},
	}
	cmdCreateMarketplace.Flags().StringVar(&name, "name", "", "set a marketplace name")
	cmdCreateMarketplace.Flags().StringVar(&receiver, "referral", "", "set a referral address")
	cmdCreateMarketplace.Flags().Float64Var(&amount, "percentage", 0, "set a referral percentage")

	cmdConfigMarketplace := &cobra.Command{
		Use:   "config-marketplace",
		Short: "config marketplace transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			return configMarketplace(signerAddress, ID, name, receiver, amount, "")
		},
	}
	cmdConfigMarketplace.Flags().StringVar(&ID, "id", "", "set a marketplace ID")
	cmdConfigMarketplace.Flags().StringVar(&name, "name", "", "set a marketplace name")
	cmdConfigMarketplace.Flags().StringVar(&receiver, "referral", "", "set a referral address")
	cmdConfigMarketplace.Flags().Float64Var(&amount, "percentage", 0, "set a referral percentage")

	return []*cobra.Command{cmdConfigITO, cmdTriggerITO, cmdSetITOPrices, cmdBuy, cmdSell, cmdCancelMarket, cmdCreateMarketplace, cmdConfigMarketplace}

}

func init() {
	cmdKapps := &cobra.Command{
		Use:   "kapps",
		Short: "kapps actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdKapps.AddCommand(subKapps()...)
	rootCmd.AddCommand(cmdKapps)
}
