package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/diegocortassa/pingping/assets/icons"
	"github.com/diegocortassa/pingping/internal/appinfo"
)

// showAbout opens the dialog with the information about the application.
func (a *App) showAbout() {
	dialog.NewCustom("About "+appinfo.Name, "Close", aboutContent(), a.win).Show()
}

// aboutContent builds the content of the about dialog.
func aboutContent() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(icons.App())
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(64, 64))

	name := widget.NewLabelWithStyle(appinfo.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	version := widget.NewLabelWithStyle("Version "+appinfo.Version, fyne.TextAlignCenter, fyne.TextStyle{})
	author := widget.NewLabelWithStyle("Author "+appinfo.Author, fyne.TextAlignCenter, fyne.TextStyle{})
	description := widget.NewLabel(appinfo.Description)
	description.Wrapping = fyne.TextWrapWord
	description.Alignment = fyne.TextAlignCenter

	return container.NewVBox(
		container.NewCenter(icon),
		name,
		version,
		author,
		description,
	)
}
