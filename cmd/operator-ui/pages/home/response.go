package home

import (
	"github.com/atotto/clipboard"
	"github.com/klever-io/klever-go/cmd/operator-ui/components"
	"github.com/rivo/tview"
)

var ResponseError error
var ResponseTX string
var ResponseLastPage PageName

func AddResponsePage(app *tview.Application, pages *tview.Pages, currentPage Form) *tview.Modal {
	var buttons []string
	var text string

	if ResponseError != nil {
		buttons = []string{"Try Again", "Quit"}
		text = "Cannot create transaction: " + ResponseError.Error()
	} else {
		buttons = []string{"Ok", "Copy"}
		text = "Successfully transaction: " + ResponseTX
	}

	model := components.CreateModal()

	model.SetText(text).
		AddButtons(buttons).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonIndex == 0 {
				if buttonLabel == "Ok" {
					pages.RemovePage(ResponseLastPage.String())
					pages.AddPage(ResponseLastPage.String(), currentPage(app, pages), true, false)
				}

				pages.SwitchToPage(ResponseLastPage.String())
			} else {
				if buttonLabel == "Copy" {
					_ = clipboard.WriteAll(ResponseTX)
				} else {
					app.Stop()
				}

			}
		})

	return model
}
