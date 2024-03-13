package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func AddressInfo(address, balance string) tview.Primitive {
	address = address[:10] + "....." + address[len(address)-10:]
	addresInfo := tview.NewList()
	addresInfo.SetShortcutColor(tcell.ColorMediumPurple)
	addresInfo.SetSelectedFocusOnly(true)
	addresInfo.SetSelectedStyle(tcell.StyleDefault)
	addresInfo.SetSecondaryTextColor(tcell.ColorMediumPurple)
	addresInfo.AddItem("Address:", address, 0, nil)
	addresInfo.AddItem("Balance:", balance, 0, nil)

	addresInfo.SetBorder(true).SetTitle(" Address info ")
	return addresInfo
}
