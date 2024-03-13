package main

import (
	"log"

	"github.com/klever-io/klever-go/cmd/operator-ui/pages"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	initialPages := pages.SetupPages(app)

	if err := app.SetRoot(initialPages, true).SetFocus(initialPages).EnableMouse(true).Run(); err != nil {
		log.Fatalln("cannot start app")
	}
}
