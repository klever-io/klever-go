package home

import (
	"strconv"
	"strings"
	"time"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/rivo/tview"
)

func AddConfigITOForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Sell Transaction:")

	var sendForm models.ConfigITOTXRequest
	var maxAmount float64

	form.AddInputField("KDA", "", 20, nil, func(v string) {
		sendForm.KDA = v
	})

	form.AddInputField("Receiver", "", 80, nil, func(v string) {
		sendForm.ReceiverAddress = v
	})

	form.AddInputField("MaxAmount", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		maxAmount = parsed
	})

	form.AddDropDown("ITO Status", []string{"Default", "Active", "Paused"}, 0, func(option string, optionIndex int) {
		sendForm.Status = int32(optionIndex) // #nosec G115
	})

	form.AddInputField("ITO Start At (DD/MM/YYYY)", time.Now().Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.StartTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("ITO End At (DD/MM/YYYY)", time.Now().Add(7*24*time.Hour).Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.EndTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("Default Limit", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return
		}

		sendForm.DefaultLimitPerAddress = parsed
	})

	form.AddDropDown("Whitelist Status", []string{"Default", "Active", "Paused"}, 0, func(option string, optionIndex int) {
		sendForm.WhitelistStatus = int32(optionIndex) // #nosec G115
	})

	form.AddInputField("Whitelist Start At (DD/MM/YYYY)", time.Now().Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.WhitelistStartTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("Whitelist End At (DD/MM/YYYY)", time.Now().Add(7*24*time.Hour).Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.WhitelistEndTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddTextView("[white]Whitelist: ", "", 0, 1, true, false)

	initialLen := form.GetFormItemCount()

	names := []string{""}
	values := []int64{0}
	counter := 0

	addWl := func(i int) {
		var itemsToApply []tview.FormItem
		if counter > 0 {
			totalFormItems := form.GetFormItemCount()
			for j := 0; j < totalFormItems-(initialLen+counter*2); j++ {
				idx := totalFormItems - (j + 1)
				itemsToApply = append(itemsToApply, form.GetFormItem(idx))
				form.RemoveFormItem(idx)
			}
		}

		form.AddInputField("Address", "", 80, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("Limit (Need Precision)", "", 30, nil, func(v string) {
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}
			values[i] = parsed
		})

		reverseArray(itemsToApply)

		for _, item := range itemsToApply {
			form.AddFormItem(item)
		}
	}

	addWl(counter)

	form.AddButton("Add WL", func() {
		counter++
		names = append(names, "")
		values = append(values, 0)
		addWl(counter)
	})

	form.AddButton("Remove WL", func() {
		if len(names) > 1 {
			form.RemoveFormItem(initialLen + (counter * 2) + 1)
			form.RemoveFormItem(initialLen + (counter * 2))
			names = names[:len(names)-1]
			values = values[:len(values)-1]
			counter--
		}
	})

	packAssets := []string{""}
	packValues := []string{""}
	packCount := 0

	form.AddTextView("[white]Packs: ", "", 0, 1, true, false)

	addRoles := func(i int) {
		form.AddInputField("Asset", "", 30, nil, func(v string) {
			packAssets[i] = v
		})

		form.AddInputField("Info: amount=price;amount2=price2... (Need Precision)", "", 30, nil, func(v string) {
			packValues[i] = v
		})
	}

	addRoles(packCount)

	form.AddButton("Add Pack", func() {
		packCount++
		packAssets = append(packAssets, "")
		packValues = append(packValues, "")
		addRoles(packCount)
	})

	form.AddButton("Remove Pack", func() {
		if len(packValues) > 1 {
			packCount--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			packValues = packValues[:len(packValues)-1]
			packAssets = packAssets[:len(packAssets)-1]
		}
	})

	form.AddButton("Send", func() {
		wlUsers := make(map[string]models.WhitelistInfoRequest)
		for i := range values {
			if values[i] != 0 {
				wlUsers[names[i]] = models.WhitelistInfoRequest{Limit: values[i]}
			}
		}

		packInfo := make(map[string]models.PackInfoRequest)

		for i := range packValues {
			splits := strings.Split(packValues[i], ";")
			var pack []models.PackItemRequest

			for _, split := range splits {
				x := strings.Split(split, "=")
				if len(x) > 2 {
					amount := x[0]
					parsedAmount, err := strconv.ParseInt(amount, 10, 64)
					if err != nil {
						return
					}

					price := x[1]
					parsedPrice, err := strconv.ParseInt(price, 10, 64)
					if err != nil {
						return
					}

					pack = append(pack, models.PackItemRequest{Price: parsedPrice, Amount: parsedAmount})
				}
			}

			packInfo[packAssets[i]] = models.PackInfoRequest{Packs: pack}
		}

		sendForm.PackInfo = packInfo

		sendForm.WhitelistInfo = wlUsers
		hash, err := blockchain.ConfigITO(address, maxAmount, sendForm)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = ConfigITOPage

		form.SetFocus(0)

		goToResponse(app, pages, AddConfigITOForm)
	})

	return form
}

func AddSetITOForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Sell Transaction:")

	var sendForm models.SetITOPricesTXRequest

	form.AddInputField("KDA", "", 20, nil, func(v string) {
		sendForm.KDA = v
	})

	packAssets := []string{""}
	packValues := []string{""}
	packCount := 0

	form.AddTextView("[white]Packs: ", "", 0, 1, true, false)

	addRoles := func(i int) {
		form.AddInputField("Asset", "", 30, nil, func(v string) {
			packAssets[i] = v
		})

		form.AddInputField("Info: amount=price;amount2=price2... (Need Precision)", "", 30, nil, func(v string) {
			packValues[i] = v
		})
	}

	addRoles(packCount)

	form.AddButton("Add Pack", func() {
		packCount++
		packAssets = append(packAssets, "")
		packValues = append(packValues, "")
		addRoles(packCount)
	})

	form.AddButton("Remove Pack", func() {
		if len(packValues) > 1 {
			packCount--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			packValues = packValues[:len(packValues)-1]
			packAssets = packAssets[:len(packAssets)-1]
		}
	})

	form.AddButton("Send", func() {
		packInfo := make(map[string]models.PackInfoRequest)

		for i := range packValues {
			splits := strings.Split(packValues[i], ";")
			var pack []models.PackItemRequest

			for _, split := range splits {
				x := strings.Split(split, "=")
				if len(x) > 2 {
					amount := x[0]
					parsedAmount, err := strconv.ParseInt(amount, 10, 64)
					if err != nil {
						return
					}

					price := x[1]
					parsedPrice, err := strconv.ParseInt(price, 10, 64)
					if err != nil {
						return
					}

					pack = append(pack, models.PackItemRequest{Price: parsedPrice, Amount: parsedAmount})
				}
			}

			packInfo[packAssets[i]] = models.PackInfoRequest{Packs: pack}
		}

		sendForm.PackInfo = packInfo

		hash, err := blockchain.ITOSetPrices(address, sendForm)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = ConfigITOPage

		form.SetFocus(0)

		goToResponse(app, pages, AddConfigITOForm)
	})

	return form
}

func AddTriggerITOForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Sell Transaction:")

	var sendForm models.ITOTriggerTXRequest
	var maxAmount float64

	var types []string
	for v := range transaction.ITOTriggerContract_EnumITOTriggerType_value {
		types = append(types, v)
	}

	form.AddDropDown("Type", types, 0, func(option string, optionIndex int) {
		sendForm.TriggerType = uint32(optionIndex) // #nosec G115
	})

	form.AddInputField("KDA", "", 20, nil, func(v string) {
		sendForm.KDA = v
	})

	form.AddInputField("Receiver", "", 80, nil, func(v string) {
		sendForm.ReceiverAddress = v
	})

	form.AddInputField("MaxAmount", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		maxAmount = parsed
	})

	form.AddDropDown("ITO Status", []string{"Default", "Active", "Paused"}, 0, func(option string, optionIndex int) {
		sendForm.Status = int32(optionIndex) // #nosec G115
	})

	form.AddInputField("ITO Start At (DD/MM/YYYY)", time.Now().Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.StartTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("ITO End At (DD/MM/YYYY)", time.Now().Add(7*24*time.Hour).Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.EndTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("Default Limit", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return
		}

		sendForm.DefaultLimitPerAddress = parsed
	})

	form.AddDropDown("Whitelist Status", []string{"Default", "Active", "Paused"}, 0, func(option string, optionIndex int) {
		sendForm.WhitelistStatus = int32(optionIndex) // #nosec G115
	})

	form.AddInputField("Whitelist Start At (DD/MM/YYYY)", time.Now().Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.WhitelistStartTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddInputField("Whitelist End At (DD/MM/YYYY)", time.Now().Add(7*24*time.Hour).Format("02/01/2006"), 20, nil, func(v string) {
		parts := strings.Split(v, "/")
		if len(parts) != 3 {
			return
		}

		year, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		month, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}

		day, err := strconv.Atoi(parts[0])
		if err != nil {
			return
		}

		sendForm.WhitelistEndTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddTextView("[white]Whitelist: ", "", 0, 1, true, false)

	initialLen := form.GetFormItemCount()

	names := []string{""}
	values := []int64{0}
	counter := 0

	addWl := func(i int) {
		var itemsToApply []tview.FormItem
		if counter > 0 {
			totalFormItems := form.GetFormItemCount()
			for j := 0; j < totalFormItems-(initialLen+counter*2); j++ {
				idx := totalFormItems - (j + 1)
				itemsToApply = append(itemsToApply, form.GetFormItem(idx))
				form.RemoveFormItem(idx)
			}
		}

		form.AddInputField("Address", "", 80, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("Limit (Need Precision)", "", 30, nil, func(v string) {
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}
			values[i] = parsed
		})

		reverseArray(itemsToApply)

		for _, item := range itemsToApply {
			form.AddFormItem(item)
		}
	}

	addWl(counter)

	form.AddButton("Add WL", func() {
		counter++
		names = append(names, "")
		values = append(values, 0)
		addWl(counter)
	})

	form.AddButton("Remove WL", func() {
		if len(names) > 1 {
			form.RemoveFormItem(initialLen + (counter * 2) + 1)
			form.RemoveFormItem(initialLen + (counter * 2))
			names = names[:len(names)-1]
			values = values[:len(values)-1]
			counter--
		}
	})

	packAssets := []string{""}
	packValues := []string{""}
	packCount := 0

	form.AddTextView("[white]Packs: ", "", 0, 1, true, false)

	addRoles := func(i int) {
		form.AddInputField("Asset", "", 30, nil, func(v string) {
			packAssets[i] = v
		})

		form.AddInputField("Info: amount=price;amount2=price2... (Need Precision)", "", 30, nil, func(v string) {
			packValues[i] = v
		})
	}

	addRoles(packCount)

	form.AddButton("Add Pack", func() {
		packCount++
		packAssets = append(packAssets, "")
		packValues = append(packValues, "")
		addRoles(packCount)
	})

	form.AddButton("Remove Pack", func() {
		if len(packValues) > 1 {
			packCount--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			packValues = packValues[:len(packValues)-1]
			packAssets = packAssets[:len(packAssets)-1]
		}
	})

	form.AddButton("Send", func() {
		wlUsers := make(map[string]models.WhitelistInfoRequest)
		for i := range values {
			if values[i] != 0 {
				wlUsers[names[i]] = models.WhitelistInfoRequest{Limit: values[i]}
			}
		}

		packInfo := make(map[string]models.PackInfoRequest)

		for i := range packValues {
			splits := strings.Split(packValues[i], ";")
			var pack []models.PackItemRequest

			for _, split := range splits {
				x := strings.Split(split, "=")
				if len(x) > 2 {
					amount := x[0]
					parsedAmount, err := strconv.ParseInt(amount, 10, 64)
					if err != nil {
						return
					}

					price := x[1]
					parsedPrice, err := strconv.ParseInt(price, 10, 64)
					if err != nil {
						return
					}

					pack = append(pack, models.PackItemRequest{Price: parsedPrice, Amount: parsedAmount})
				}
			}

			packInfo[packAssets[i]] = models.PackInfoRequest{Packs: pack}
		}

		sendForm.PackInfo = packInfo

		sendForm.WhitelistInfo = wlUsers
		hash, err := blockchain.ITOTrigger(address, maxAmount, sendForm)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = TriggerITOPage

		form.SetFocus(0)

		goToResponse(app, pages, AddTriggerITOForm)
	})

	return form
}
