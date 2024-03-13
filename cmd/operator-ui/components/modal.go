package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func CreateModal() *tview.Modal {
	model := tview.NewModal()

	model.SetBackgroundColor(tcell.ColorMediumPurple)

	return model
}
