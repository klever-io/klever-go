package login

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/klever-io/klever-go/cmd/operator-ui/pages/home"
	"github.com/rivo/tview"
)

func AddLogin(app *tview.Application, pages *tview.Pages) tview.Primitive {
	pg := tview.NewPages()

	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "/"
	}
	root := tview.NewTreeNode(fmt.Sprintf("Looking for files at: %s", rootDir)).
		SetColor(tcell.ColorMediumPurple)
	tree := tview.NewTreeView().
		SetRoot(root).SetCurrentNode(root)
	tree.SetBorder(true).SetTitle(" Select your .pem - (q, esc) quit ")
	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Name() {
		case "Esc", "Rune[Esc]", "Rune[q]", "q":
			app.Stop()
		}
		return event
	})

	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	add := func(target *tview.TreeNode, path string) {
		if filepath.Ext(path) == ".pem" {
			text := fmt.Sprintf("Are you sure to select [black]%s [white]file?", path)
			view := tview.NewTextView()
			view.SetText(text)
			view.SetBackgroundColor(tcell.ColorMediumPurple)
			view.SetDynamicColors(true)

			form := tview.NewForm()
			form.SetLabelColor(tcell.ColorWhite)
			form.SetBackgroundColor(tcell.ColorMediumPurple)
			form.SetFieldBackgroundColor(tcell.ColorBlack)
			form.SetButtonBackgroundColor(tcell.ColorBlack)
			form.SetButtonTextColor(tcell.ColorWhite)
			form.SetButtonsAlign(tview.AlignCenter).SetTitleAlign(tview.AlignLeft)

			var formRes struct {
				pass string
			}

			form.AddInputField("Your password:", "", 40, nil, func(pass string) {
				formRes.pass = pass
			})

			form.AddButton("Confirm", func() {
				home.SetupVars(path, formRes.pass)
				pages.AddPage("home", home.AddHome(pages, app), true, false)
				pg.SwitchToPage("home")
				pages.SwitchToPage("home")
			})

			form.AddButton("Cancel", func() {
				form.SetFocus(0)
				pg.SwitchToPage("home")
			})

			layout := tview.NewFlex().SetDirection(tview.FlexRow)
			layout.AddItem(view, 0, 1, false)
			layout.AddItem(form, 0, 2, true)

			mainLayout := tview.NewFlex().SetDirection(tview.FlexColumn)
			mainLayout.AddItem(layout, 0, 1, true)
			mainLayout.SetBackgroundColor(tcell.ColorMediumPurple)
			mainLayout.SetBorder(true)

			pg.RemovePage("modal")
			pg.AddPage("modal", modal(mainLayout, 50, 10), true, true)
			pg.SwitchToPage("modal")

			return
		}

		files, err := os.ReadDir(path)
		if err != nil {
			log.Fatalln("cannot read dir: ", err)
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) == ".pem" {
				node := tview.NewTreeNode(file.Name()).
					SetColor(tcell.ColorMediumPurple).
					SetReference(filepath.Join(path, file.Name()))
				if file.IsDir() {
					node.SetColor(tcell.ColorPurple)
				}
				target.AddChild(node)
			}
		}
	}

	add(root, rootDir)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference == nil {
			return
		}
		children := node.GetChildren()
		if len(children) == 0 {
			path := reference.(string)
			add(node, path)
		} else {
			node.SetExpanded(!node.IsExpanded())
		}
	})

	pg.AddPage("home", tree, true, true)

	return pg
}
