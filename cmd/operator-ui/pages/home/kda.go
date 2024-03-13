package home

import (
	"math"
	"strconv"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/rivo/tview"
)

func AddDepositForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Deposit Transaction:")

	var sendForm struct {
		depositType int
		kda         string
		currencyID  string
		amount      float64
	}

	form.AddDropDown("Deposit Type", []string{"FPRDeposit", "KDAPool"}, 0, func(option string, optionIndex int) {
		sendForm.depositType = optionIndex
	})

	form.AddInputField("Currency", "KLV", 20, nil, func(v string) {
		sendForm.currencyID = v
	})

	form.AddInputField("KDA", "", 20, nil, func(v string) {
		sendForm.kda = v
	})

	form.AddInputField("Amount", "", 20, nil, func(amount string) {
		parsed, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return
		}
		sendForm.amount = parsed
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Deposit(address, sendForm.amount, models.DepositTXRequest{
			DepositType: int32(sendForm.depositType),
			KDA:         sendForm.kda,
			CurrencyID:  sendForm.currencyID,
		})

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = DepositPage

		form.SetFocus(0)

		goToResponse(app, pages, AddDepositForm)
	})

	return form
}

func AddCreateForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Create Asset Transaction:")

	sendForm := models.CreateAssetTXRequest{
		Properties: &models.PropertiesInfo{},
		Attributes: &models.AttributesInfo{},
		Staking:    &models.StakingInfo{},
	}

	form.AddInputField("Name", "", 30, nil, func(v string) {
		sendForm.Name = v
	})

	form.AddInputField("Ticker", "", 30, nil, func(v string) {
		sendForm.Name = v
	})

	form.AddDropDown("Type", []string{"Fungible", "NonFungible"}, 0, func(option string, optionIndex int) {
		sendForm.Type = uint32(optionIndex)
	})

	form.AddInputField("Owner Address", "", 80, nil, func(v string) {
		sendForm.OwnerAddress = v
	})

	form.AddInputField("Logo", "", 60, nil, func(v string) {
		sendForm.Logo = v
	})

	form.AddInputField("Precision", "", 10, nil, func(v string) {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return
		}

		sendForm.Precision = uint32(parsed)
	})

	form.AddInputField("Max Supply", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.MaxSupply = parsed
	})

	form.AddInputField("Initial Supply", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.InitialSupply = parsed
	})

	form.AddTextView("Properties: ", "", 0, 1, false, false)

	form.AddDropDown("Can Freeze", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanFreeze = optionIndex == 0
	})

	form.AddDropDown("Can Wipe", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanWipe = optionIndex == 0
	})

	form.AddDropDown("Can Pause", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanPause = optionIndex == 0
	})

	form.AddDropDown("Can Mint", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanMint = optionIndex == 0
	})

	form.AddDropDown("Can Burn", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanBurn = optionIndex == 0
	})

	form.AddDropDown("Can Change Owner", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanChangeOwner = optionIndex == 0
	})

	form.AddDropDown("Can Add Roles", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.CanAddRoles = optionIndex == 0
	})

	form.AddDropDown("Limit Transfer", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.Properties.LimitTransfer = optionIndex == 0
	})

	form.AddTextView("[white]Staking: ", "", 0, 1, true, false)

	form.AddDropDown("Interest Type", []string{"APR", "FPR"}, 0, func(option string, optionIndex int) {
		sendForm.Staking.InterestType = uint32(optionIndex)
	})

	form.AddInputField("APR Percentage", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}

		sendForm.Staking.APR = uint32(parsed * math.Pow10(2))
	})

	form.AddInputField("Epochs To Unstake", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToUnstake = uint32(parsed)
	})

	form.AddInputField("Epochs To Withdraw", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToWithdraw = uint32(parsed)
	})

	form.AddInputField("Epochs To Claim", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToClaim = uint32(parsed)
	})

	form.AddTextView("[white]Attributes: ", "", 0, 1, true, false)

	form.AddDropDown("Is Paused", []string{"Yes", "No"}, 1, func(option string, optionIndex int) {
		sendForm.Attributes.IsPaused = optionIndex == 0
	})

	form.AddDropDown("Is Paused", []string{"Yes", "No"}, 1, func(option string, optionIndex int) {
		sendForm.Attributes.IsPaused = optionIndex == 0
	})

	form.AddDropDown("NFT Mint Stoped", []string{"Yes", "No"}, 1, func(option string, optionIndex int) {
		sendForm.Attributes.IsNFTMintStopped = optionIndex == 0
	})

	form.AddDropDown("NFT Royalties Change Stopped", []string{"Yes", "No"}, 1, func(option string, optionIndex int) {
		sendForm.Attributes.IsRoyaltiesChangeStopped = optionIndex == 0
	})

	form.AddDropDown("NFT Metadata Change Stopped", []string{"Yes", "No"}, 1, func(option string, optionIndex int) {
		sendForm.Attributes.IsNFTMetadataChangeStopped = optionIndex == 0
	})

	names := []string{""}
	values := []string{""}
	counter := 0

	form.AddTextView("[white]URIs: ", "", 0, 1, true, false)

	initialLen := form.GetFormItemCount()

	addUri := func(i int) {
		var itemsToApply []tview.FormItem
		if counter > 0 {
			totalFormItems := form.GetFormItemCount()
			for j := 0; j < totalFormItems-(initialLen+counter*2); j++ {
				idx := totalFormItems - (j + 1)
				itemsToApply = append(itemsToApply, form.GetFormItem(idx))
				form.RemoveFormItem(idx)
			}
		}

		form.AddInputField("Name", "", 30, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("URI", "", 60, nil, func(v string) {
			values[i] = v
		})

		reverseArray(itemsToApply)

		for _, item := range itemsToApply {
			form.AddFormItem(item)
		}
	}

	addUri(counter)

	form.AddButton("Add URI", func() {
		counter++
		names = append(names, "")
		values = append(values, "")
		addUri(counter)
	})

	form.AddButton("Remove URI", func() {
		if len(names) > 1 {
			form.RemoveFormItem(initialLen + (counter * 2) + 1)
			form.RemoveFormItem(initialLen + (counter * 2))
			names = names[:len(names)-1]
			values = values[:len(values)-1]
			counter--
		}
	})

	roles := []*models.RolesInfo{{}}
	rolesCount := 0

	form.AddTextView("[white]Roles: ", "", 0, 1, true, false)

	addRoles := func(i int) {
		form.AddDropDown("Can Mint", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
			roles[i].HasRoleMint = optionIndex == 0
		})

		form.AddDropDown("Can Set ITO", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
			roles[i].HasRoleSetITOPrices = optionIndex == 0
		})

		form.AddInputField("Address", "", 80, nil, func(v string) {
			roles[i].Address = v
		})
	}

	addRoles(rolesCount)

	form.AddButton("Add Role", func() {
		rolesCount++
		roles = append(roles, &models.RolesInfo{})
		addRoles(rolesCount)
	})

	form.AddButton("Remove Role", func() {
		if len(roles) > 1 {
			rolesCount--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			roles = roles[:len(roles)-1]
		}
	})

	form.AddButton("Send", func() {
		uris := make(map[string]string)
		for i := range values {
			if values[i] != "" {
				uris[names[i]] = values[i]
			}
		}

		parsedRoles := make([]*models.RolesInfo, 0)
		for _, role := range roles {
			if role.Address != "" {
				parsedRoles = append(parsedRoles, role)
			}
		}
		sendForm.Roles = parsedRoles

		sendForm.URIs = uris
		sendForm.MaxSupply = int64(float64(sendForm.MaxSupply) * math.Pow10(int(sendForm.Precision)))
		sendForm.InitialSupply = int64(float64(sendForm.InitialSupply) * math.Pow10(int(sendForm.Precision)))

		hash, err := blockchain.CreateAsset(address, sendForm)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = CreatePage

		form.SetFocus(0)

		goToResponse(app, pages, AddCreateForm)
	})

	return form
}

func AddAssetTriggerForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Create Asset Transaction:")

	sendForm := models.AssetTriggerTXRequest{
		Staking: &models.StakingInfo{},
		KDAPool: &models.KDAPoolInfo{},
	}

	var amount float64
	var types []string
	for v := range transaction.ITOTriggerContract_EnumITOTriggerType_value {
		types = append(types, v)
	}

	form.AddDropDown("Type", types, 0, func(option string, optionIndex int) {
		sendForm.TriggerType = uint32(optionIndex)
	})

	form.AddInputField("Asset Id", "", 30, nil, func(v string) {
		sendForm.AssetID = v
	})

	form.AddInputField("Receiver", "", 80, nil, func(v string) {
		sendForm.Receiver = v
	})

	form.AddInputField("Amount", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}

		amount = parsed
	})

	form.AddInputField("MIME", "", 80, nil, func(v string) {
		sendForm.MIME = v
	})

	form.AddInputField("Logo", "", 60, nil, func(v string) {
		sendForm.Logo = v
	})

	form.AddTextView("[white]Staking: ", "", 0, 1, true, false)

	form.AddDropDown("Interest Type", []string{"APR", "FPR"}, 0, func(option string, optionIndex int) {
		sendForm.Staking.InterestType = uint32(optionIndex)
	})

	form.AddInputField("APR Percentage", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}

		sendForm.Staking.APR = uint32(parsed * math.Pow10(2))
	})

	form.AddInputField("Epochs To Unstake", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToUnstake = uint32(parsed)
	})

	form.AddInputField("Epochs To Withdraw", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToWithdraw = uint32(parsed)
	})

	form.AddInputField("Epochs To Claim", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return
		}

		sendForm.Staking.MinEpochsToClaim = uint32(parsed)
	})

	form.AddTextView("[white]KDA Pool: ", "", 0, 1, true, false)

	names := []string{""}
	values := []string{""}
	counter := 0

	form.AddTextView("[white]URIs: ", "", 0, 1, true, false)

	initialLen := form.GetFormItemCount()

	addUri := func(i int) {
		var itemsToApply []tview.FormItem
		if counter > 0 {
			totalFormItems := form.GetFormItemCount()
			for j := 0; j < totalFormItems-(initialLen+counter*2); j++ {
				idx := totalFormItems - (j + 1)
				itemsToApply = append(itemsToApply, form.GetFormItem(idx))
				form.RemoveFormItem(idx)
			}
		}

		form.AddInputField("Name", "", 30, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("URI", "", 60, nil, func(v string) {
			values[i] = v
		})

		reverseArray(itemsToApply)

		for _, item := range itemsToApply {
			form.AddFormItem(item)
		}
	}

	addUri(counter)

	form.AddButton("Add URI", func() {
		counter++
		names = append(names, "")
		values = append(values, "")
		addUri(counter)
	})

	form.AddButton("Remove URI", func() {
		if len(names) > 1 {
			form.RemoveFormItem(initialLen + (counter * 2) + 1)
			form.RemoveFormItem(initialLen + (counter * 2))
			names = names[:len(names)-1]
			values = values[:len(values)-1]
			counter--
		}
	})

	roles := []*models.RolesInfo{{}}
	rolesCount := 0

	form.AddTextView("[white]Roles: ", "", 0, 1, true, false)

	addRoles := func(i int) {
		form.AddDropDown("Can Mint", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
			roles[i].HasRoleMint = optionIndex == 0
		})

		form.AddDropDown("Can Set ITO", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
			roles[i].HasRoleSetITOPrices = optionIndex == 0
		})

		form.AddInputField("Address", "", 80, nil, func(v string) {
			roles[i].Address = v
		})
	}

	addRoles(rolesCount)

	form.AddButton("Send", func() {
		uris := make(map[string]string)
		for i := range values {
			if values[i] != "" {
				uris[names[i]] = values[i]
			}
		}

		for _, role := range roles {
			if role.Address != "" {
				sendForm.Role = role
			}
		}

		if sendForm.Amount > 0 {
			kda, err := blockchain.GetAssetData(sendForm.AssetID)
			if err != nil {
				ResponseError = err
				ResponseTX = ""
				ResponseLastPage = TriggerPage

				form.SetFocus(0)

				goToResponse(app, pages, AddAssetTriggerForm)
			}

			if kda.AssetType == kapps.KDAData_Fungible {
				sendForm.Amount = int64(amount * math.Pow10(int(kda.Precision)))
			}
		}

		sendForm.URIs = uris

		hash, err := blockchain.TriggerAsset(address, sendForm)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = TriggerPage

		form.SetFocus(0)

		goToResponse(app, pages, AddAssetTriggerForm)
	})

	return form
}

func reverseArray(arr []tview.FormItem) {
	if len(arr) < 2 {
		return
	}

	length := len(arr)

	for i := 0; i < length/2; i++ {
		arr[i], arr[length-i-1] = arr[length-i-1], arr[i]
	}
}
