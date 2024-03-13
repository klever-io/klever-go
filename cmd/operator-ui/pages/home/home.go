package home

import (
	"fmt"
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/klever-io/klever-go/cmd/operator-ui/blockchain"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/rivo/tview"
)

var currentFocus = "categories"
var address = ""
var balance = ""

type PageName string

func (p PageName) String() string {
	return string(p)
}

const (
	SendPage             PageName = "send"
	FreezePage           PageName = "freeze"
	UnfreezePage         PageName = "unfreeze"
	DelegatePage         PageName = "delegate"
	UndelegatePage       PageName = "undelegate"
	ClaimPage            PageName = "claim"
	WithdrawPage         PageName = "withdraw"
	UnjailPage           PageName = "unjail"
	SetAccountNamePage   PageName = "set-account-name"
	UpdatePermissionPage PageName = "update-permission"

	//TODO:
	CreateValidatorPage PageName = "create-validator"
	ConfigValidatorPage PageName = "config-validator"

	CreatePage  PageName = "create"
	TriggerPage PageName = "trigger"
	DepositPage PageName = "deposit"

	ConfigITOPage         PageName = "config-ito"
	TriggerITOPage        PageName = "trigger-ito"
	SetITOPage            PageName = "set-ito-prices"
	CreateMarketplacePage PageName = "create-marketplace"
	ConfigMarketplacePage PageName = "config-marketplace"
	SellPage              PageName = "sell"
	BuyPage               PageName = "buy"
	CancelPage            PageName = "cancel-market"

	CreateProposalPage PageName = "create-proposal"
	VotePage           PageName = "vote"
)

func SetupVars(pemPath string, pass string) {
	WalletPubKeyConverter, _ := pubkeyConverter.NewBech32PubkeyConverter(32)

	pk, _, addr, err := blockchain.LoadKey(pemPath, 0, WalletPubKeyConverter, pass)
	if err != nil {
		log.Fatalln("cannot load you key: ", err.Error())
	}
	address = addr

	bl, err := blockchain.GetBalance(addr)
	if err != nil {
		log.Fatalln("cannot load your balance: ", err.Error())
	}

	balance = fmt.Sprintf("%v", bl)
	blockchain.PK = pk
}

func AddHome(mainPage *tview.Pages, app *tview.Application) tview.Primitive {
	appPages := setupHomeSubPages(app)
	categoriesList := components.List(" Categories ")
	transactions := components.List(" Transaction Type ")

	populateTransactions(categories[0], transactions, appPages, app)
	for i, category := range categories {
		xd := category
		run := []rune(fmt.Sprintf("%d", i+1))
		categoriesList.AddItem(xd, "", run[0], func() {
			transactions.Clear()
			app.SetFocus(transactions)
			currentFocus = "transactions"
			populateTransactions(xd, transactions, appPages, app)
		})
	}

	flex := tview.NewFlex().
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(components.AddressInfo(address, balance), 0, 2, false).
					AddItem(categoriesList, 0, 3, true).
					AddItem(transactions, 0, 4, false), 0, 2, true).
				AddItem(appPages, 0, 3, false), 0, 4, true).
			AddItem(components.Menu(), 2, 0, false), 0, 1, true)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Name() {
		case "Esc", "Rune[Esc]":
			switch currentFocus {
			case "transactions":
				app.SetFocus(categoriesList)
				currentFocus = "categories"
			case "pages":
				app.SetFocus(transactions)
				currentFocus = "transactions"
			default:
				app.SetFocus(categoriesList)
			}
		case "Rune[F1]", "F1":
			mainPage.SwitchToPage("login")
			currentFocus = "categories"
		case "Rune[F2]", "F2":
			appPages.SwitchToPage("send")
			currentFocus = "pages"
		case "Rune[F5]", "F5":
			bl, err := blockchain.GetBalance(address)
			if err != nil {
				log.Fatalln("cannot load your balance: ", err.Error())
			}

			balance = fmt.Sprintf("%v", bl)

			list := flex.GetItem(0).(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.Flex).GetItem(0).(*tview.List)
			list.SetItemText(1, "Balance:", balance)
		case "Rune[q]", "q":
			if currentFocus != "pages" {
				app.Stop()
			}
		}
		return event
	})

	flex.SetFullScreen(true)

	return flex
}

func setupHomeSubPages(app *tview.Application) *tview.Pages {
	pages := tview.NewPages()

	pages.AddPage(SendPage.String(), AddSendForm(app, pages), true, false)
	pages.AddPage(FreezePage.String(), AddFreezeForm(app, pages), true, false)
	pages.AddPage(UnfreezePage.String(), AddUnfreezeForm(app, pages), true, false)
	pages.AddPage(DelegatePage.String(), AddDelegateForm(app, pages), true, false)
	pages.AddPage(UndelegatePage.String(), AddUndelegateForm(app, pages), true, false)
	pages.AddPage(ClaimPage.String(), AddClaimForm(app, pages), true, false)
	pages.AddPage(WithdrawPage.String(), AddWithdrawForm(app, pages), true, false)
	pages.AddPage(UnjailPage.String(), AddUnjailForm(app, pages), true, false)
	pages.AddPage(SetAccountNamePage.String(), AddSetAccoutNameForm(app, pages), true, false)
	pages.AddPage(UpdatePermissionPage.String(), AddUpdatePermissionForm(app, pages), true, false)
	pages.AddPage(CreateValidatorPage.String(), AddCreateValidatorForm(app, pages), true, false)
	pages.AddPage(ConfigValidatorPage.String(), AddConfigValidatorForm(app, pages), true, false)
	pages.AddPage(VotePage.String(), AddVoteForm(app, pages), true, false)
	pages.AddPage(CreateProposalPage.String(), AddCreateProposalForm(app, pages), true, false)
	pages.AddPage(CreateMarketplacePage.String(), AddCreateMarketplaceForm(app, pages), true, false)
	pages.AddPage(ConfigMarketplacePage.String(), AddConfigMarketplaceForm(app, pages), true, false)
	pages.AddPage(DepositPage.String(), AddDepositForm(app, pages), true, false)
	pages.AddPage(CreatePage.String(), AddCreateForm(app, pages), true, false)
	pages.AddPage(TriggerPage.String(), AddAssetTriggerForm(app, pages), true, false)

	pages.AddPage(SellPage.String(), AddSellForm(app, pages), true, false)
	pages.AddPage(BuyPage.String(), AddBuyForm(app, pages), true, false)
	pages.AddPage(CancelPage.String(), AddCancelForm(app, pages), true, false)
	pages.AddPage(ConfigITOPage.String(), AddConfigITOForm(app, pages), true, false)
	pages.AddPage(TriggerITOPage.String(), AddTriggerITOForm(app, pages), true, false)
	pages.AddPage(SetITOPage.String(), AddSetITOForm(app, pages), true, false)

	//set Send as default page
	pages.SwitchToPage(SendPage.String())

	return pages
}

type Form func(*tview.Application, *tview.Pages) *tview.Form

func goToResponse(app *tview.Application, pages *tview.Pages, current Form) {
	pages.RemovePage("Response")
	pages.AddPage("Response", AddResponsePage(app, pages, current), false, false)
	pages.SwitchToPage("Response")
}

func populateTransactions(category string, list *tview.List, pages *tview.Pages, app *tview.Application) {
	subcategories := categoriesItems[category]

	for i, subcategorie := range subcategories {
		page := subcategorie
		run := []rune(fmt.Sprintf("%d", i+1))
		list.AddItem(subcategorie, "", run[0], func() {
			pages.SwitchToPage(page)
			app.SetFocus(pages)
			currentFocus = "pages"
		})
	}

	var defaultPage string
	if len(subcategories) > 0 {
		defaultPage = subcategories[0]
	}
	pages.SwitchToPage(defaultPage)
	app.SetFocus(pages)
	currentFocus = "pages"
}

var categories = []string{
	"account",
	"kapps",
	"ito",
	"kda",
	"gov",
	"validator",
}

var categoriesItems = map[string][]string{
	"account": {SendPage.String(), FreezePage.String(), UnfreezePage.String(), DelegatePage.String(),
		UndelegatePage.String(), ClaimPage.String(), WithdrawPage.String(), UnjailPage.String(),
		SetAccountNamePage.String(), UpdatePermissionPage.String()},
	"kda":       {CreatePage.String(), TriggerPage.String(), DepositPage.String()},
	"kapps":     {CreateMarketplacePage.String(), ConfigMarketplacePage.String(), SellPage.String(), BuyPage.String(), CancelPage.String()},
	"ito":       {ConfigITOPage.String(), TriggerITOPage.String(), SetITOPage.String()},
	"gov":       {CreateProposalPage.String(), VotePage.String()},
	"validator": {CreateValidatorPage.String(), ConfigValidatorPage.String()},
}
