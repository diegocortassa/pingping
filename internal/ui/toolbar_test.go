package ui

import (
	"testing"

	"fyne.io/fyne/v2/widget"
)

func TestToolbarHoldsOnlyTheTargetActions(t *testing.T) {
	app, _ := newTestApp(t)

	if app.toolbar == nil {
		t.Fatal("the toolbar is missing")
	}

	var labels []string
	for _, obj := range app.toolbar.Objects {
		if button, ok := obj.(*widget.Button); ok {
			labels = append(labels, button.Text)
		}
	}

	want := []string{"Add", "Edit", "Remove"}
	if len(labels) != len(want) {
		t.Fatalf("got %v, want %v", labels, want)
	}
	for i, label := range want {
		if labels[i] != label {
			t.Fatalf("got %v, want %v", labels, want)
		}
	}
}

func TestSettingsIsOnlyInTheMenu(t *testing.T) {
	app, _ := newTestApp(t)

	menu := app.win.MainMenu()
	if menu == nil {
		t.Fatal("the main menu is missing")
	}

	found := false
	for _, item := range menu.Items {
		for _, entry := range item.Items {
			if entry.Label == "Settings" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the settings item is missing in the menu")
	}
}
