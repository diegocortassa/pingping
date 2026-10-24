package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/diegocortassa/pingping/internal/appinfo"
)

func TestMainMenuHasTheHelpEntry(t *testing.T) {
	app, _ := newTestApp(t)

	menu := app.win.MainMenu()
	if menu == nil {
		t.Fatal("the main menu is missing")
	}

	var help *fyne.Menu
	for _, item := range menu.Items {
		if item.Label == "Help" {
			help = item
		}
	}
	if help == nil {
		t.Fatal("the help menu is missing")
	}
	if len(help.Items) == 0 || help.Items[0].Label != "About" {
		t.Fatalf("the about item is missing in %v", help.Items)
	}
}

func TestAboutDialogShowsTheMetadata(t *testing.T) {
	texts := labelTexts(aboutContent())
	for _, want := range []string{appinfo.Name, appinfo.Version, appinfo.Author} {
		if !containsText(texts, want) {
			t.Fatalf("%q is missing in %v", want, texts)
		}
	}
}

func TestAboutDialogShowsTheIcon(t *testing.T) {
	content, ok := aboutContent().(*fyne.Container)
	if !ok {
		t.Fatal("the content of the about dialog is not a container")
	}
	if len(content.Objects) == 0 {
		t.Fatal("the content of the about dialog is empty")
	}
	if _, ok := content.Objects[0].(*fyne.Container); !ok {
		t.Fatalf("got %T, want the icon in a centered container", content.Objects[0])
	}
}

func TestShowAboutDoesNotFail(t *testing.T) {
	app, _ := newTestApp(t)
	app.showAbout()
}

// labelTexts collects the texts of the labels of a container.
func labelTexts(content fyne.CanvasObject) []string {
	box, ok := content.(*fyne.Container)
	if !ok {
		return nil
	}

	var texts []string
	for _, obj := range box.Objects {
		if label, ok := obj.(*widget.Label); ok {
			texts = append(texts, label.Text)
		}
	}
	return texts
}

// containsText reports whether the list holds a text that contains the part.
func containsText(texts []string, part string) bool {
	for _, text := range texts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}
