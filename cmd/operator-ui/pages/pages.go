package pages

import (
	"github.com/klever-io/klever-go/cmd/operator-ui/pages/login"
	"github.com/rivo/tview"
)

func SetupPages(app *tview.Application) *tview.Pages {
	pages := tview.NewPages()

	pages.AddPage("login", login.AddLogin(app, pages), true, false)
	//set login as default page
	pages.SwitchToPage("login")

	return pages
}
