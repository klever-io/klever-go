package components

import (
	"fmt"

	"github.com/atotto/clipboard"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func GetCustomForm(title string) *tview.Form {
	form := tview.NewForm()

	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		item, _ := form.GetFocusedItemIndex()
		if item == -1 {
			return ev
		}

		if ev.Key() == tcell.KeyCtrlV {
			if text, err := clipboard.ReadAll(); err == nil {
				v, ok := form.GetFormItem(item).(*tview.InputField)
				if !ok {
					return ev
				}
				v.SetText(text)
			}
		}

		return ev
	})

	form.SetBorderPadding(1, 1, 1, 1)
	form.SetTitleColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorMediumPurple)
	form.SetFieldBackgroundColor(tcell.ColorMediumPurple)
	form.SetButtonBackgroundColor(tcell.ColorMediumPurple)
	form.SetButtonTextColor(tcell.ColorWhite)

	form.SetBorder(true)
	form.SetTitle(fmt.Sprintf(" %s ", title))
	form.SetButtonsAlign(tview.AlignCenter).SetTitleAlign(tview.AlignLeft)

	return form
}
