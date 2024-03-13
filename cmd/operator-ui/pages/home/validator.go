package home

import (
	"strconv"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/rivo/tview"
)

func AddConfigValidatorForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Config Validator Transaction:")

	var sendForm struct {
		name          string
		logo          string
		blsKey        string
		rewardAddr    string
		commission    float64
		maxDelegation float64
		canDelegate   bool
	}

	form.AddInputField("Name", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Logo", "", 80, nil, func(v string) {
		sendForm.logo = v
	})

	form.AddInputField("BLS Key", "", 80, nil, func(v string) {
		sendForm.blsKey = v
	})

	form.AddInputField("Reward Address", "", 80, nil, func(v string) {
		sendForm.rewardAddr = v
	})

	form.AddInputField("Comission", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.commission = parsed
	})

	form.AddInputField("Max Delegate", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.maxDelegation = parsed
	})

	names := []string{""}
	values := []string{""}
	counter := 0

	form.AddTextView("URIs: ", "", 0, 1, false, false)

	addUri := func(i int) {
		form.AddInputField("Name", "", 30, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("URI", "", 60, nil, func(v string) {
			values[i] = v
		})
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
			counter--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			names = names[:len(names)-1]
			values = values[:len(values)-1]
		}
	})

	form.AddButton("Send", func() {
		uris := make(map[string]string)
		for i := range values {
			if values[i] != "" {
				uris[names[i]] = values[i]
			}
		}

		hash, err := blockchain.ValidatorConfig(address, sendForm.blsKey, sendForm.rewardAddr,
			sendForm.logo, sendForm.commission, sendForm.maxDelegation, sendForm.canDelegate, uris, sendForm.name)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = ConfigValidatorPage

		form.SetFocus(0)

		goToResponse(app, pages, AddConfigValidatorForm)
	})

	return form
}

func AddCreateValidatorForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Create Validator Transaction:")

	var sendForm struct {
		name          string
		logo          string
		blsKey        string
		ownerAddr     string
		rewardAddr    string
		commission    float64
		maxDelegation float64
		canDelegate   bool
	}

	form.AddInputField("Name", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Logo", "", 80, nil, func(v string) {
		sendForm.logo = v
	})

	form.AddInputField("BLS Key", "", 80, nil, func(v string) {
		sendForm.blsKey = v
	})

	form.AddInputField("Owner Address", "", 80, nil, func(v string) {
		sendForm.ownerAddr = v
	})

	form.AddInputField("Reward Address", "", 80, nil, func(v string) {
		sendForm.rewardAddr = v
	})

	form.AddInputField("Comission", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.commission = parsed
	})

	form.AddInputField("Max Delegate", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.maxDelegation = parsed
	})

	names := []string{""}
	values := []string{""}
	counter := 0

	form.AddTextView("URIs: ", "", 0, 1, false, false)

	addUri := func(i int) {
		form.AddInputField("Name", "", 30, nil, func(v string) {
			names[i] = v
		})

		form.AddInputField("URI", "", 60, nil, func(v string) {
			values[i] = v
		})
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
			counter--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			names = names[:len(names)-1]
			values = values[:len(values)-1]
		}
	})

	form.AddButton("Send", func() {
		uris := make(map[string]string)
		for i := range values {
			if values[i] != "" {
				uris[names[i]] = values[i]
			}
		}

		hash, err := blockchain.CreateValidator(address, sendForm.blsKey, sendForm.ownerAddr, sendForm.rewardAddr,
			sendForm.logo, sendForm.commission, sendForm.maxDelegation, sendForm.canDelegate, uris, sendForm.name)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = CreateValidatorPage

		form.SetFocus(0)

		goToResponse(app, pages, AddCreateValidatorForm)
	})

	return form
}
