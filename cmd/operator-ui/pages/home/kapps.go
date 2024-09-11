package home

import (
	"strconv"
	"strings"
	"time"

	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/rivo/tview"
)

func AddSellForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Sell Transaction:")

	var sendForm struct {
		kdaID, currency, mktID string
		price, reservePrice    float64
		endTime                int64
		mktType                int32
	}

	form.AddInputField("Marketplace Id", "", 30, nil, func(v string) {
		sendForm.mktID = v
	})

	form.AddInputField("KDA", "", 20, nil, func(v string) {
		sendForm.kdaID = v
	})

	form.AddInputField("Currency", "KLV", 20, nil, func(v string) {
		sendForm.currency = v
	})

	form.AddInputField("Price", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.price = parsed
	})

	form.AddInputField("Reserve Price", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.reservePrice = parsed
	})

	form.AddDropDown("Type", []string{"BuyItNow", "Auction"}, 0, func(option string, optionIndex int) {
		sendForm.mktType = int32(optionIndex) // #nosec G115
	})

	form.AddInputField("End At (DD/MM/YYYY)", time.Now().Add(7*24*time.Hour).Format("02/01/2006"), 20, nil, func(v string) {
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

		sendForm.endTime = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Sell(address, sendForm.kdaID, sendForm.currency, sendForm.mktID, sendForm.price,
			sendForm.reservePrice, sendForm.endTime, sendForm.mktType)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = SellPage

		form.SetFocus(0)

		goToResponse(app, pages, AddSellForm)
	})

	return form
}

func AddBuyForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Buy Transaction:")

	var sendForm struct {
		orderId  string
		currency string
		amount   float64
		buyType  int32
	}

	form.AddInputField("Order Id", "", 30, nil, func(v string) {
		sendForm.orderId = v
	})

	form.AddInputField("Currency", "KLV", 20, nil, func(v string) {
		sendForm.currency = v
	})

	form.AddInputField("Amount", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.amount = parsed
	})

	form.AddDropDown("Type", []string{"ITOBuy", "MarketBuy"}, 1, func(option string, optionIndex int) {
		sendForm.buyType = int32(optionIndex) // #nosec G115
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.Buy(address, sendForm.orderId, sendForm.currency, sendForm.amount, sendForm.buyType)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = BuyPage

		form.SetFocus(0)

		goToResponse(app, pages, AddBuyForm)
	})

	return form
}

func AddCancelForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Cancel Order Transaction:")

	var sendForm struct {
		id string
	}

	form.AddInputField("Order Id", "", 30, nil, func(v string) {
		sendForm.id = v
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.CancelMarket(address, sendForm.id)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = CancelPage

		form.SetFocus(0)

		goToResponse(app, pages, AddCancelForm)
	})

	return form
}

func AddCreateMarketplaceForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Create Marketplace Transaction:")

	var sendForm struct {
		name            string
		referralAddress string
		percentage      float64
	}

	form.AddInputField("Name", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Referral Address", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Percentage", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.percentage = parsed
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.CreateMarketplace(address, sendForm.name, sendForm.referralAddress, sendForm.percentage)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = CreateMarketplacePage

		form.SetFocus(0)

		goToResponse(app, pages, AddCreateMarketplaceForm)
	})

	return form
}

func AddConfigMarketplaceForm(app *tview.Application, pages *tview.Pages) *tview.Form {
	form := components.GetCustomForm("Config Marketplace Transaction:")

	var sendForm struct {
		id              string
		name            string
		referralAddress string
		percentage      float64
	}

	form.AddInputField("Marketplace ID", "", 20, nil, func(v string) {
		sendForm.id = v
	})

	form.AddInputField("Name", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Referral Address", "", 80, nil, func(v string) {
		sendForm.name = v
	})

	form.AddInputField("Percentage", "", 20, nil, func(v string) {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		sendForm.percentage = parsed
	})

	form.AddButton("Send", func() {
		hash, err := blockchain.ConfigMarketplace(address, sendForm.id, sendForm.name, sendForm.referralAddress, sendForm.percentage)

		ResponseError = err
		ResponseTX = hash
		ResponseLastPage = ConfigMarketplacePage

		form.SetFocus(0)

		goToResponse(app, pages, AddConfigMarketplaceForm)
	})

	return form
}
