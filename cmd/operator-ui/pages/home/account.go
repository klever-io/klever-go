package home

import (
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/rivo/tview"
)

func AddSendForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Send Transaction:")

	var sendForm struct {
		toAddress string
		Amount    float64
		KDA       string
	}

	form.AddInputField("To Address", "", 80, nil, func(toAddress string) {
		sendForm.toAddress = toAddress
	})

	form.AddInputField("Amount", "", 20, nil, func(amount string) {
		parsed, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return
		}
		sendForm.Amount = parsed
	})

	form.AddInputField("KDA", "KLV", 20, nil, func(kda string) {
		sendForm.KDA = kda
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Send(address, sendForm.toAddress,
			sendForm.Amount, sendForm.KDA)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = SendPage

		form.SetFocus(0)

		goToResponse(app, pages, AddSendForm)
	})

	return form
}

func AddSetAccoutNameForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Set Name Transaction:")

	var sendForm struct {
		name string
	}

	form.AddInputField("Name", "", 40, nil, func(v string) {
		sendForm.name = v
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.SetAccountName(address, sendForm.name)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = SetAccountNamePage

		form.SetFocus(0)

		goToResponse(app, pages, AddSetAccoutNameForm)
	})

	return form
}

func AddFreezeForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		Amount float64
		KDA    string
	}

	form := components.GetCustomForm("Freeze Transaction:")

	form.AddInputField("Amount", "", 20, nil, func(amount string) {
		parsed, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return
		}
		sendForm.Amount = parsed
	})

	form.AddInputField("KDA", "KLV", 20, nil, func(kda string) {
		sendForm.KDA = kda
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Freeze(address, sendForm.KDA, sendForm.Amount)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = FreezePage

		form.SetFocus(0)

		goToResponse(app, pages, AddFreezeForm)
	})

	return form
}

func AddUnfreezeForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		BucketId string
		KDA      string
		All      bool
	}

	form := components.GetCustomForm("Unfreeze Transaction:")

	form.AddInputField("BucketId", "", 100, nil, func(bucketID string) {
		sendForm.BucketId = bucketID
	})

	form.AddDropDown("All Buckets", []string{"No", "Yes"}, 0, func(option string, optionIndex int) {
		sendForm.All = optionIndex == 1
	})

	form.AddInputField("KDA", "KLV", 20, nil, func(kda string) {
		sendForm.KDA = kda
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Unfreeze(address, sendForm.BucketId, sendForm.KDA, sendForm.All)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = UnfreezePage

		form.SetFocus(0)

		goToResponse(app, pages, AddUnfreezeForm)
	})

	return form
}

func AddDelegateForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		ToAddress string
		BucketID  string
	}

	form := components.GetCustomForm("Delegate Transaction:")

	form.AddInputField("To Address", "", 100, nil, func(to string) {
		sendForm.ToAddress = to
	})

	form.AddInputField("BucketID", "", 20, nil, func(bucketID string) {
		sendForm.BucketID = bucketID
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Delegate(address, sendForm.ToAddress, sendForm.BucketID)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = DelegatePage

		form.SetFocus(0)

		goToResponse(app, pages, AddDelegateForm)
	})

	return form
}

func AddUndelegateForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		BucketID string
	}

	form := components.GetCustomForm("Undelegate Transaction:")

	form.AddInputField("BucketID", "", 20, nil, func(bucketID string) {
		sendForm.BucketID = bucketID
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Undelegate(address, sendForm.BucketID)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = UndelegatePage

		form.SetFocus(0)

		goToResponse(app, pages, AddUndelegateForm)
	})

	return form
}

func AddClaimForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		Id        string
		ClaimType int
	}

	form := components.GetCustomForm("Claim Transaction:")

	form.AddInputField("KDA", "", 100, nil, func(id string) {
		sendForm.Id = id
	})

	form.AddDropDown("Claim Type", []string{"StakingClaim", "AllowanceClaim", "MarketClaim"}, 0, func(option string, optionIndex int) {
		sendForm.ClaimType = optionIndex
	})

	form.AddButton("Send", func() {

		hash, err := blockchain.Claim(address, sendForm.Id, int32(sendForm.ClaimType))

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = ClaimPage

		form.SetFocus(0)

		goToResponse(app, pages, AddClaimForm)
	})

	return form
}

func AddWithdrawForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	var sendForm struct {
		KDA  string
		Type int32
	}

	form := components.GetCustomForm("Withdraw Transaction:")

	form.AddInputField("KDA", "KLV", 20, nil, func(kda string) {
		sendForm.KDA = kda
	})

	form.AddDropDown("Withdraw Type", []string{"Staking", "KDAPool"}, 0, func(option string, index int) {
		if index == 0 {
			sendForm.Type = int32(transaction.WithdrawContract_Staking)
		} else {
			sendForm.Type = int32(transaction.WithdrawContract_KDAPool)
		}
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Withdraw(address, sendForm.KDA, sendForm.Type)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = WithdrawPage

		form.SetFocus(0)

		goToResponse(app, pages, AddWithdrawForm)
	})

	return form
}

func AddUnjailForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Unjail Transaction:")

	form.AddButton("Send", func() {
		hash, err := blockchain.Unjail(address)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = UnjailPage

		form.SetFocus(0)

		goToResponse(app, pages, AddUnjailForm)
	})

	return form
}

func AddUpdatePermissionForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Update Permission Transaction:")

	counter := 0

	results := []models.PermissionTXRequest{
		{},
	}

	addField := func(i int) {
		form.AddDropDown("Type", []string{"Owner", "User"}, 0, func(option string, optionIndex int) {
			results[i].Type = int32(optionIndex)
		})

		form.AddInputField("Name", "", 80, nil, func(v string) {
			results[i].PermissionName = v
		})

		form.AddInputField("Threshold", "", 80, nil, func(v string) {
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return
			}

			results[i].Threshold = parsed
		})

		form.AddInputField("Operations", "", 80, nil, func(v string) {
			results[i].Operations = v
		})

		form.AddInputField("Signers (address=weight;address2=weight2...)", "", 80, nil, func(v string) {
			splits := strings.Split(v, ";")

			for _, split := range splits {
				x := strings.Split(split, "=")
				if len(x) > 2 {
					addr := x[0]
					weight := x[1]

					parsed, err := strconv.ParseInt(weight, 10, 64)
					if err != nil {
						return
					}

					results[i].Signers = append(results[i].Signers, models.SignerTXRequest{
						Address: addr,
						Weight:  parsed,
					})
				}

			}
		})
	}

	addField(counter)

	form.AddButton("Add Permission", func() {
		counter++
		results = append(results, models.PermissionTXRequest{})
		addField(counter)
	})

	form.AddButton("Remove Permission", func() {
		if len(results) > 1 {
			counter--
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			form.RemoveFormItem(form.GetFormItemCount() - 1)
			results = results[:len(results)-1]
		}
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.SetPermission(address, results)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = UpdatePermissionPage

		form.SetFocus(0)

		goToResponse(app, pages, AddUpdatePermissionForm)
	})

	return form
}
