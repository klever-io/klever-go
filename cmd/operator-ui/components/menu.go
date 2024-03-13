package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func Menu() tview.Primitive {
	return tview.NewTextView().
		SetTextColor(tcell.ColorMediumPurple).
		SetText("(F1) logout (F2) send transaction (F5) update balance \n(esc) back (q) to quit")
}
