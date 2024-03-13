package home

import (
	"strconv"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/klever-io/klever-go/kapps"
	"github.com/rivo/tview"
)

func AddCreateProposalForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Create Proposal Transaction:")

	var sendForm struct {
		description string
		duration    int
	}

	form.AddTextArea("Description", "", 80, 4, 200, func(v string) {
		sendForm.description = v
	})

	form.AddInputField("Epochs Duration", "", 20, nil, func(v string) {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return
		}
		sendForm.duration = parsed
	})

	param := []int32{0}
	values := []string{""}
	counter := 0

	form.AddTextView("Parameters: ", "", 0, 1, false, false)

	addParam := func(i int) {
		var options []string
		for v := range kapps.EnumParameter_value {
			options = append(options, v)
		}

		form.AddDropDown("Parameter", options, 0, func(option string, optionIndex int) {
			param[i] = int32(optionIndex)
		})

		form.AddInputField("Value", "", 32, nil, func(v string) {
			values[i] = v
		})
	}

	addParam(counter)

	form.AddButton("Add Parameter", func() {
		counter++
		param = append(param, 0)
		values = append(values, "")
		addParam(counter)
	})

	form.AddButton("Remove Parameter", func() {
		if len(param) > 1 {
			counter--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			param = param[:len(param)-1]
			values = values[:len(values)-1]
		}
	})

	form.AddButton("Send", func() {
		parameters := make(map[int32]string)

		for i := range values {
			if values[i] != "" {
				parameters[param[i]] = values[i]
			}
		}

		hash, err := blockchain.Proposal(address, sendForm.description,
			parameters, uint32(sendForm.duration))

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = CreateProposalPage

		form.SetFocus(0)

		goToResponse(app, pages, AddCreateProposalForm)
	})

	return form
}

func AddVoteForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Vote Transaction:")

	var sendForm struct {
		proposalId uint64
		amount     float64
		voteType   uint64
	}

	form.AddInputField("Proposal ID", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return
		}
		sendForm.proposalId = parsed
	})

	form.AddInputField("Amount", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.amount = parsed
	})

	form.AddDropDown("Vote Type", []string{"Yes", "No"}, 0, func(option string, optionIndex int) {
		sendForm.voteType = uint64(optionIndex)
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Vote(address, sendForm.proposalId, sendForm.amount, sendForm.voteType)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = VotePage

		form.SetFocus(0)

		goToResponse(app, pages, AddVoteForm)
	})

	return form
}
