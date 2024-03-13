package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func List(title string) *tview.List {
	list := tview.NewList()
	list.SetShortcutColor(tcell.ColorMediumPurple)
	list.ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(title)

	return list
}
