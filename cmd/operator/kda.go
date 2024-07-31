package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/spf13/cobra"
)

type ParsedRoyalty struct {
	Address                   string  `form:"address" json:"address"`
	PercentTransferPercentage float64 `form:"percentTransferPercentage" json:"percentTransferPercentage"`
	PercentTransferFixed      float64 `form:"percentTransferFixed" json:"percentTransferFixed"`
	PercentMarketPercentage   float64 `form:"percentMarketPercentage" json:"percentMarketPercentage"`
	PercentMarketFixed        float64 `form:"percentMarketFixed" json:"percentMarketFixed"`
	PercentITOPercentage      float64 `form:"percentITOPercentage" json:"percentITOPercentage"`
	PercentITOFixed           float64 `form:"percentITOFixed" json:"percentITOFixed"`
}

func subKDA() []*cobra.Command {
	var (
		name                        string
		ticker                      string
		ownerAddress                string
		adminAddress                string
		logo                        string
		uris                        map[string]string
		precision                   uint32
		initialSupply               float64
		maxSupply                   float64
		royaltiesAddress            string
		royaltiesMarketFixed        float64
		royaltiesMarketPercentage   float64
		royaltiesITOFixed           float64
		royaltiesITOPercentage      float64
		royaltiesTransferFixed      float64
		royaltiesTransferPercentage []string
		splitRoyalties              []string
		canFreeze                   bool
		canWipe                     bool
		canPause                    bool
		canMint                     bool
		canBurn                     bool
		canChangeOwner              bool
		canAddRoles                 bool
		limitTransfer               bool
		isPaused                    bool
		isNFTMintStopped            bool
		isRoyaltiesChangeStopped    bool
		interestType                uint32
		apr                         float64
		minEpochsToClaim            uint32
		minEpochsToUnstake          uint32
		minEpochsToWithdraw         uint32
		addRolesMint                []string
		addRolesSetITOPrices        []string
		addRolesDeposit             []string
		kdaID                       string
		nftID                       string
		receiver                    string
		amount                      float64
		mime                        string
		value                       int64
		staking                     map[string]string
		kdaPool                     map[string]string
	)

	cmdCreate := &cobra.Command{
		Use:   "create [KDA_TYPE]",
		Short: "create a new KDA",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kdaType, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid kda type %s: %w", args[0], err)
			}

			if len(name) < core.MinLengthForAssetName || len(name) > core.MaxLengthForAssetName {
				return fmt.Errorf("invalid name %s length %d", name, len(name))
			}

			if !kdautils.IsAssetNameHumanReadable([]byte(name)) {
				return fmt.Errorf("invalid kda name provided")
			}

			if len(ticker) < core.MinLengthForAssetTicker || len(ticker) > core.MaxLengthForAssetTicker {
				return fmt.Errorf("invalid ticker %s length %d", ticker, len(ticker))
			}

			if !kdautils.IsTickerValid([]byte(ticker)) {
				return fmt.Errorf("invalid kda ticker provided")
			}

			if precision < core.MinNumberOfDecimals || precision > core.MaxNumberOfDecimals {
				return fmt.Errorf("invalid precision provided")
			}

			newRoles := make(map[string]models.RolesInfo)
			for _, address := range addRolesMint {
				newRoles[address] = models.RolesInfo{
					HasRoleMint: true,
				}
			}
			for _, address := range addRolesSetITOPrices {
				if entry, ok := newRoles[address]; ok {
					entry.HasRoleSetITOPrices = true
					newRoles[address] = entry
				} else {
					newRoles[address] = models.RolesInfo{
						HasRoleSetITOPrices: true,
					}
				}
			}

			for _, address := range addRolesDeposit {
				if entry, ok := newRoles[address]; ok {
					entry.HasRoleDeposit = true
					newRoles[address] = entry
				} else {
					newRoles[address] = models.RolesInfo{
						HasRoleDeposit: true,
					}
				}
			}

			parsedRoles := make([]*models.RolesInfo, 0)
			for address, role := range newRoles {
				parsedRoles = append(parsedRoles, &models.RolesInfo{
					Address:             address,
					HasRoleMint:         role.HasRoleMint,
					HasRoleSetITOPrices: role.HasRoleSetITOPrices,
				})
			}

			transferPercentage := make([]*models.RoyaltyData, 0)
			for _, royaltyData := range royaltiesTransferPercentage {
				data := models.RoyaltyDataTX{}
				err := json.Unmarshal([]byte(royaltyData), &data)
				if err != nil {
					return err
				}

				finalRoyaltyData := &models.RoyaltyData{
					Amount:     int64(data.Amount * math.Pow10(int(precision))),
					Percentage: uint32(data.Percentage * math.Pow10(2)),
				}

				transferPercentage = append(transferPercentage, finalRoyaltyData)
			}

			royaltiesInfo := make(map[string]*models.RoyaltySplitInfo, 0)
			for _, r := range splitRoyalties {
				data := ParsedRoyalty{}
				err := json.Unmarshal([]byte(r), &data)
				if err != nil {
					return err
				}

				royaltiesInfo[data.Address] = &models.RoyaltySplitInfo{
					PercentTransferPercentage: uint32(data.PercentTransferPercentage * math.Pow10(2)),
					PercentTransferFixed:      uint32(data.PercentTransferFixed * math.Pow10(2)),
					PercentMarketPercentage:   uint32(data.PercentMarketPercentage * math.Pow10(2)),
					PercentMarketFixed:        uint32(data.PercentMarketFixed * math.Pow10(2)),
					PercentITOPercentage:      uint32(data.PercentITOPercentage * math.Pow10(2)),
					PercentITOFixed:           uint32(data.PercentITOFixed * math.Pow10(2)),
				}
			}

			if kdaType == uint64(kapps.KDAData_Fungible) {
				initialSupply = initialSupply * math.Pow10(int(precision))
				maxSupply = maxSupply * math.Pow10(int(precision))
			}

			kdaRequest := models.CreateAssetTXRequest{
				Type:          uint32(kdaType),
				OwnerAddress:  signerAddress,
				AdminAddress:  adminAddress,
				Name:          name,
				Ticker:        ticker,
				Precision:     precision,
				InitialSupply: int64(initialSupply),
				MaxSupply:     int64(maxSupply),
				Logo:          logo,
				URIs:          uris,
				Royalties: &models.RoyaltiesInfo{
					Address:            royaltiesAddress,
					TransferPercentage: transferPercentage,
					TransferFixed:      int64(royaltiesTransferFixed * math.Pow10(6)),
					MarketPercentage:   uint32(royaltiesMarketPercentage * math.Pow10(2)),
					MarketFixed:        int64(royaltiesMarketFixed * math.Pow10(6)),
					ITOPercentage:      uint32(royaltiesITOPercentage * math.Pow10(2)),
					ITOFixed:           int64(royaltiesITOFixed * math.Pow10(6)),
					SplitRoyalties:     royaltiesInfo,
				},
				Attributes: &models.AttributesInfo{
					IsPaused:                 isPaused,
					IsNFTMintStopped:         isNFTMintStopped,
					IsRoyaltiesChangeStopped: isRoyaltiesChangeStopped,
				},
				Properties: &models.PropertiesInfo{
					CanMint:        canMint,
					CanFreeze:      canFreeze,
					CanBurn:        canBurn,
					CanPause:       canPause,
					CanWipe:        canWipe,
					CanChangeOwner: canChangeOwner,
					CanAddRoles:    canAddRoles,
					LimitTransfer:  limitTransfer,
				},
				Staking: &models.StakingInfo{
					InterestType:        interestType,
					APR:                 uint32(apr * math.Pow10(2)),
					MinEpochsToClaim:    minEpochsToClaim,
					MinEpochsToUnstake:  minEpochsToUnstake,
					MinEpochsToWithdraw: minEpochsToWithdraw,
				},
				Roles: parsedRoles,
			}

			return newKDA(signerAddress, kdaRequest)
		},
	}
	cmdCreate.Flags().StringVar(&name, "name", "", "KDA name")
	cmdCreate.Flags().StringVar(&ticker, "ticker", "", "KDA ticker")
	cmdCreate.Flags().StringVar(&ownerAddress, "ownerAddress", "", "KDA ownerAddress")
	cmdCreate.Flags().StringVar(&adminAddress, "adminAddress", "", "KDA adminAddress")
	cmdCreate.Flags().StringVar(&logo, "logo", "", "KDA logo")
	cmdCreate.Flags().StringToStringVar(&uris, "uris", nil, "KDA uris")
	cmdCreate.Flags().Uint32Var(&precision, "precision", 0, "KDA precision")
	cmdCreate.Flags().Float64Var(&initialSupply, "initialSupply", 0, "KDA initialSupply")
	cmdCreate.Flags().Float64Var(&maxSupply, "maxSupply", 0, "KDA maxSupply")
	cmdCreate.Flags().StringVar(&royaltiesAddress, "royaltiesAddress", "", "KDA royaltiesAddress")
	cmdCreate.Flags().Float64Var(&royaltiesTransferFixed, "royaltiesTransferFixed", 0, "KDA royaltiesTransferFixed")
	cmdCreate.Flags().StringArrayVarP(&royaltiesTransferPercentage, "royaltiesTransferPercentage", "", []string{}, `--royaltiesTransferPercentage='{"amount": 100, "percentage": 50}'`)
	cmdCreate.Flags().Float64Var(&royaltiesMarketFixed, "royaltiesMarketFixed", 0, "KDA royaltiesMarketFixed")
	cmdCreate.Flags().Float64Var(&royaltiesMarketPercentage, "royaltiesMarketPercentage", 0, "KDA royaltiesMarketPercentage")
	cmdCreate.Flags().Float64Var(&royaltiesITOFixed, "royaltiesITOFixed", 0, "KDA royaltiesITOFixed")
	cmdCreate.Flags().Float64Var(&royaltiesITOPercentage, "royaltiesITOPercentage", 0, "KDA royaltiesITOPercentage")
	cmdCreate.Flags().StringArrayVarP(&splitRoyalties, "splitRoyalties", "", []string{}, `--splitRoyalties='{"address":"klv...", "percentTransferPercentage": 1, "percentTransferFixed": 1, "percentMarketPercentage": 1, "percentMarketFixed": 1}, "percentITOPercentage": 1, "percentITOFixed": 1}'`)
	cmdCreate.Flags().BoolVar(&canFreeze, "canFreeze", false, "KDA canFreeze")
	cmdCreate.Flags().BoolVar(&canWipe, "canWipe", false, "KDA canWipe")
	cmdCreate.Flags().BoolVar(&canPause, "canPause", false, "KDA canPause")
	cmdCreate.Flags().BoolVar(&canMint, "canMint", false, "KDA canMint")
	cmdCreate.Flags().BoolVar(&canBurn, "canBurn", false, "KDA canBurn")
	cmdCreate.Flags().BoolVar(&canChangeOwner, "canChangeOwner", false, "KDA canChangeOwner")
	cmdCreate.Flags().BoolVar(&canAddRoles, "canAddRoles", false, "KDA canAddRoles")
	cmdCreate.Flags().BoolVar(&limitTransfer, "limitTransfer", false, "KDA limitTransfer")
	cmdCreate.Flags().BoolVar(&isPaused, "isPaused", false, "KDA isPaused")
	cmdCreate.Flags().BoolVar(&isNFTMintStopped, "isNFTMintStopped", false, "KDA isNFTMintStopped")
	cmdCreate.Flags().BoolVar(&isRoyaltiesChangeStopped, "isRoyaltiesChangeStopped", false, "KDA isRoyaltiesChangeStopped")
	cmdCreate.Flags().Float64Var(&apr, "apr", 0, "KDA apr")
	cmdCreate.Flags().Uint32Var(&interestType, "interestType", 0, "KDA interest type")
	cmdCreate.Flags().Uint32Var(&minEpochsToClaim, "minEpochsToClaim", 0, "KDA minEpochsToClaim")
	cmdCreate.Flags().Uint32Var(&minEpochsToUnstake, "minEpochsToUnstake", 0, "KDA minEpochsToUnstake")
	cmdCreate.Flags().Uint32Var(&minEpochsToWithdraw, "minEpochsToWithdraw", 0, "KDA minEpochsToWithdraw")
	cmdCreate.Flags().StringSliceVar(&addRolesMint, "addRolesMint", nil, "KDA addRolesMint")
	cmdCreate.Flags().StringSliceVar(&addRolesSetITOPrices, "addRolesSetITOPrices", nil, "KDA addRolesSetITOPrices")
	cmdCreate.Flags().StringSliceVar(&addRolesDeposit, "addRolesDeposit", nil, "KDA addRolesDeposit")

	cmdTrigger := &cobra.Command{
		Use:   "trigger [TRIGGER_TYPE]",
		Short: "trigger a KDA",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			triggerType, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid trigger type %s: %w", args[0], err)
			}

			parsedAmount := amount
			kda, err := getAssetData(kdaID)
			if err != nil {
				return err
			}

			if kda.AssetType != kapps.KDAData_NonFungible {
				parsedAmount = amount * math.Pow10(int(kda.Precision))
			}

			if len(addRolesMint) == 1 &&
				len(addRolesSetITOPrices) == 1 &&
				addRolesMint[0] != addRolesSetITOPrices[0] {
				return fmt.Errorf("can only set one address roler per trigger")
			}

			role := &models.RolesInfo{}
			switch len(addRolesMint) {
			case 0:
			case 1:
				role.Address = addRolesMint[0]
				role.HasRoleMint = true
			default:
				return fmt.Errorf("can only add one roler per trigger")
			}

			switch len(addRolesSetITOPrices) {
			case 0:
			case 1:
				role.Address = addRolesSetITOPrices[0]
				role.HasRoleSetITOPrices = true
			default:
				return fmt.Errorf("can only add one roler per trigger")
			}

			switch len(addRolesDeposit) {
			case 0:
			case 1:
				role.Address = addRolesDeposit[0]
				role.HasRoleDeposit = true
			default:
				return fmt.Errorf("can only add one role per trigger")
			}

			var stakingInfo *models.StakingInfo
			if len(staking) > 0 {
				stakingInfo = &models.StakingInfo{}
				apr, err := strconv.ParseFloat(staking["apr"], 64)
				if err != nil {
					return fmt.Errorf("invalid apr %s: %w", staking["apr"], err)
				}

				claim, err := strconv.ParseUint(staking["claim"], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid claim min epochs %s: %w", staking["claim"], err)
				}

				unstake, err := strconv.ParseUint(staking["unstake"], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid unstake min epochs %s: %w", staking["unstake"], err)
				}

				withdraw, err := strconv.ParseUint(staking["withdraw"], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid withdraw min epochs %s: %w", staking["withdraw"], err)
				}

				stakingInfo.APR = uint32(apr * math.Pow10(2))
				stakingInfo.MinEpochsToClaim = uint32(claim)
				stakingInfo.MinEpochsToUnstake = uint32(unstake)
				stakingInfo.MinEpochsToWithdraw = uint32(withdraw)
			}

			var kdaPoolInfo *models.KDAPoolInfo
			if len(kdaPool) > 0 {
				kdaPoolInfo = &models.KDAPoolInfo{}

				active, err := strconv.ParseBool(kdaPool["active"])
				if err != nil {
					return fmt.Errorf("invalid acrtive %s: %w", kdaPool["active"], err)
				}

				fixedRatioKLV, err := strconv.ParseInt(kdaPool["fixedRatioKLV"], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid fixed ratio klv %s: %w", kdaPool["fixedRatioKLV"], err)
				}

				fixedRatioKDA, err := strconv.ParseInt(kdaPool["fixedRatioKDA"], 10, 32)
				if err != nil {
					return fmt.Errorf("invalid fixed ratio klv %s: %w", kdaPool["fixedRatioKDA"], err)
				}

				adminAddress, exist := kdaPool["adminAddress"]
				if !exist {
					return fmt.Errorf("invalid admim klv address")
				}

				kdaPoolInfo.Active = active
				kdaPoolInfo.AdminAddress = adminAddress
				kdaPoolInfo.FRatioKLV = fixedRatioKLV
				kdaPoolInfo.FRatioKDA = fixedRatioKDA

			}

			if len(nftID) > 0 {
				kdaID += kapps.Sp + nftID
			}

			var royalties *models.RoyaltiesInfo
			transferPercentage := make([]*models.RoyaltyData, 0)
			for _, royaltyData := range royaltiesTransferPercentage {
				data := models.RoyaltyDataTX{}
				err := json.Unmarshal([]byte(royaltyData), &data)
				if err != nil {
					return err
				}

				finalRoyaltyData := &models.RoyaltyData{
					Amount:     int64(data.Amount * math.Pow10(int(precision))),
					Percentage: uint32(data.Percentage * math.Pow10(2)),
				}

				transferPercentage = append(transferPercentage, finalRoyaltyData)
			}

			royaltiesInfo := make(map[string]*models.RoyaltySplitInfo, 0)
			for _, r := range splitRoyalties {
				data := ParsedRoyalty{}
				err := json.Unmarshal([]byte(r), &data)
				if err != nil {
					return err
				}

				royaltiesInfo[data.Address] = &models.RoyaltySplitInfo{
					PercentTransferPercentage: uint32(data.PercentTransferPercentage * math.Pow10(2)),
					PercentTransferFixed:      uint32(data.PercentTransferFixed * math.Pow10(2)),
					PercentMarketPercentage:   uint32(data.PercentMarketPercentage * math.Pow10(2)),
					PercentMarketFixed:        uint32(data.PercentMarketFixed * math.Pow10(2)),
					PercentITOPercentage:      uint32(data.PercentITOPercentage * math.Pow10(2)),
					PercentITOFixed:           uint32(data.PercentITOFixed * math.Pow10(2)),
				}
			}

			royalties = &models.RoyaltiesInfo{
				Address:            royaltiesAddress,
				TransferPercentage: transferPercentage,
				TransferFixed:      int64(royaltiesTransferFixed * math.Pow10(6)),
				MarketPercentage:   uint32(royaltiesMarketPercentage * math.Pow10(2)),
				MarketFixed:        int64(royaltiesMarketFixed * math.Pow10(6)),
				ITOPercentage:      uint32(royaltiesITOPercentage * math.Pow10(2)),
				ITOFixed:           int64(royaltiesITOFixed * math.Pow10(6)),
				SplitRoyalties:     royaltiesInfo,
			}

			triggerRequest := models.AssetTriggerTXRequest{
				TriggerType: uint32(triggerType),
				AssetID:     kdaID,
				Amount:      int64(parsedAmount),
				Receiver:    receiver,
				MIME:        mime,
				Logo:        logo,
				URIs:        uris,
				Role:        role,
				Value:       value,
				Royalties:   royalties,
				Staking:     stakingInfo,
				KDAPool:     kdaPoolInfo,
			}

			return kdaTrigger(signerAddress, triggerRequest)
		},
	}

	cmdTrigger.Flags().StringVar(&kdaID, "kdaID", "", "Trigger KDA ID")
	cmdTrigger.Flags().StringVar(&nftID, "nftID", "", "Trigger NFT ID")
	cmdTrigger.Flags().Float64Var(&amount, "amount", 0, "Trigger amount")
	cmdTrigger.Flags().StringVar(&receiver, "receiver", "", "Trigger receiver")
	cmdTrigger.Flags().StringVar(&mime, "mime", "", "Trigger mime")
	cmdTrigger.Flags().StringVar(&logo, "logo", "", "Trigger logo")
	cmdTrigger.Flags().Int64Var(&value, "value", 0, "Value used for SFT maxSupply")
	cmdTrigger.Flags().StringToStringVar(&uris, "uris", nil, "Trigger uris")
	cmdTrigger.Flags().StringVar(&royaltiesAddress, "royaltiesAddress", "", "KDA royaltiesAddress")
	cmdTrigger.Flags().Float64Var(&royaltiesTransferFixed, "royaltiesTransferFixed", 0, "KDA royaltiesTransferFixed")
	cmdTrigger.Flags().StringArrayVarP(&royaltiesTransferPercentage, "royaltiesTransferPercentage", "", []string{}, `--royaltiesTransferPercentage='{"amount": 100, "percentage": 50}'`)
	cmdTrigger.Flags().Float64Var(&royaltiesMarketFixed, "royaltiesMarketFixed", 0, "KDA royaltiesMarketFixed")
	cmdTrigger.Flags().Float64Var(&royaltiesMarketPercentage, "royaltiesMarketPercentage", 0, "KDA royaltiesMarketPercentage")
	cmdTrigger.Flags().Float64Var(&royaltiesITOFixed, "royaltiesITOFixed", 0, "KDA royaltiesITOFixed")
	cmdTrigger.Flags().Float64Var(&royaltiesITOPercentage, "royaltiesITOPercentage", 0, "KDA royaltiesITOPercentage")
	cmdTrigger.Flags().StringArrayVarP(&splitRoyalties, "splitRoyalties", "", []string{}, `--splitRoyalties='{"address":"klv...", "percentTransferPercentage": 1, "percentTransferFixed": 1, "percentMarketPercentage": 1, "percentMarketFixed": 1}, "percentITOPercentage": 1, "percentITOFixed": 1}'`)
	cmdTrigger.Flags().StringSliceVar(&addRolesMint, "addRolesMint", nil, "Trigger addRolesMint")
	cmdTrigger.Flags().StringSliceVar(&addRolesSetITOPrices, "addRolesSetITOPrices", nil, "Trigger addRolesSetITOPrices")
	cmdTrigger.Flags().StringSliceVar(&addRolesDeposit, "addRolesDeposit", nil, "Trigger addRolesDeposit")
	cmdTrigger.Flags().StringToStringVar(&staking, "updateStaking", nil, "--updateStaking 'apr=10,claim=1,unstake=5,withdraw=7' (claim/unstake/withdraw are the min epochs to make the respective actions)")
	cmdTrigger.Flags().StringToStringVar(&kdaPool, "updateKdaPool", nil, "--updateKdaPool 'active=false,adminAddress=klv-address,fixedRatioKLV=1,fixedRatioKDA=100")

	var (
		currency    string
		depositType int32
	)

	cmdDeposit := &cobra.Command{
		Use:   "deposit [amount]",
		Short: "Trigger a deposit request on KDA",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			amountParsed, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %s: %w", args[0], err)
			}

			parsedAmount := amountParsed
			kda, err := getAssetData(currency)
			if err != nil {
				return err
			}

			if kda.AssetType != kapps.KDAData_NonFungible {
				parsedAmount = amountParsed * math.Pow10(int(kda.Precision))
			}

			depositRequest := models.DepositTXRequest{
				DepositType: depositType,
				KDA:         kdaID,
				CurrencyID:  currency,
				Amount:      int64(parsedAmount),
			}

			return deposit(signerAddress, depositRequest)
		},
	}
	cmdDeposit.Flags().StringVar(&kdaID, "kdaID", "", "Deposit to KDA ID")
	cmdDeposit.Flags().StringVar(&currency, "currencyID", "", "Currency to share")
	cmdDeposit.Flags().Int32Var(&depositType, "depositType", 0, "Deposit type")

	cmdWithdraw := &cobra.Command{
		Use:   "withdraw [amount]",
		Short: "Trigger a withdraw request on KDA Pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			amountParsed, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %s: %w", args[0], err)
			}

			parsedAmount := amountParsed
			kda, err := getAssetData(currency)
			if err != nil {
				return err
			}

			if kda.AssetType != kapps.KDAData_NonFungible {
				parsedAmount = amountParsed * math.Pow10(int(kda.Precision))
			}

			withdrawRequest := models.WithdrawTXRequest{
				WithdrawType: int32(transaction.DepositContract_KDAPool),
				KDA:          kdaID,
				CurrencyID:   currency,
				Amount:       int64(parsedAmount),
			}

			return withdrawFromKDAPool(signerAddress, withdrawRequest)
		},
	}

	cmdWithdraw.Flags().StringVar(&kdaID, "kdaID", "", "Deposit to KDA ID")
	cmdWithdraw.Flags().StringVar(&currency, "currencyID", "", "Currency to share")

	return []*cobra.Command{cmdCreate, cmdTrigger, cmdDeposit, cmdWithdraw}

}

func init() {
	cmdKDA := &cobra.Command{
		Use:   "kda",
		Short: "kda actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdKDA.AddCommand(subKDA()...)
	rootCmd.AddCommand(cmdKDA)
}

func newKDA(fromAddr string, createAsset models.CreateAssetTXRequest) error {
	data, err := buildRequest(transaction.TXContract_CreateAssetContractType, fromAddr, []interface{}{createAsset})
	if err != nil {
		return err
	}

	log.Info("requesting new asset", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func kdaTrigger(fromAddr string, assetTrigger models.AssetTriggerTXRequest) error {
	data, err := buildRequest(transaction.TXContract_AssetTriggerContractType, fromAddr, []interface{}{assetTrigger})
	if err != nil {
		return err
	}

	log.Info("requesting asset trigger", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func deposit(fromAddr string, depositReq models.DepositTXRequest) error {
	data, err := buildRequest(transaction.TXContract_DepositContractType, fromAddr, []interface{}{depositReq})
	if err != nil {
		return err
	}

	log.Info("requesting deposit", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}

func withdrawFromKDAPool(fromAddr string, withdraw models.WithdrawTXRequest) error {
	data, err := buildRequest(transaction.TXContract_WithdrawContractType, fromAddr, []interface{}{withdraw})
	if err != nil {
		return err
	}

	log.Info("requesting withdraw", "data", string(data))
	_, err = sendSignAndBroadcast(data)
	return err
}
