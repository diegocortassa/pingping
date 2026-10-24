package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/diegocortassa/pingping/internal/config"
)

// graphWindowOptions are the selectable lengths of the graph window.
var graphWindowOptions = []struct {
	Label string
	Value time.Duration
}{
	{"15 minutes", 15 * time.Minute},
	{"30 minutes", 30 * time.Minute},
	{"1 hour", time.Hour},
	{"2 hours", 2 * time.Hour},
	{"6 hours", 6 * time.Hour},
	{"24 hours", 24 * time.Hour},
}

// icmModeOptions are the selectable ICMP socket modes.
var icmModeOptions = []string{
	string(config.ICModeAuto),
	string(config.ICModeUnprivileged),
	string(config.ICModePrivileged),
}

// addTarget opens the dialog that creates a new target.
func (a *App) addTarget() {
	a.targetDialog(nil)
}

// editTarget opens the dialog that changes the selected target.
func (a *App) editTarget() {
	target, ok := a.selectedTarget()
	if !ok {
		dialog.ShowInformation("No target selected", "Select a target in the list first.", a.win)
		return
	}
	a.targetDialog(&target)
}

// removeTarget asks for confirmation and deletes the selected target.
func (a *App) removeTarget() {
	target, ok := a.selectedTarget()
	if !ok {
		dialog.ShowInformation("No target selected", "Select a target in the list first.", a.win)
		return
	}
	dialog.NewConfirm("Remove target",
		fmt.Sprintf("Stop monitoring %s and delete its history?", target.Address),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			a.deleteTarget(target)
		}, a.win).Show()
}

// targetDialog shows the form that creates or edits a target.
func (a *App) targetDialog(existing *config.Target) {
	address := widget.NewEntry()
	interval := widget.NewEntry()
	timeout := widget.NewEntry()
	enabled := widget.NewCheck("Monitor this address", nil)

	address.Validator = func(text string) error {
		if strings.TrimSpace(text) == "" {
			return errors.New("the address must not be empty")
		}
		return nil
	}
	interval.Validator = durationValidator
	timeout.Validator = durationValidator

	title := "Add target"
	if existing != nil {
		title = "Edit target"
		address.SetText(existing.Address)
		interval.SetText(existing.Interval.String())
		timeout.SetText(existing.Timeout.String())
		enabled.SetChecked(existing.Enabled)
	} else {
		interval.SetText(config.Duration(config.DefaultInterval).String())
		timeout.SetText(config.Duration(config.DefaultTimeout).String())
		enabled.SetChecked(true)
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Address", address),
		widget.NewFormItem("Interval", interval),
		widget.NewFormItem("Timeout", timeout),
		widget.NewFormItem("Enabled", enabled),
	}

	dialog.NewForm(title, "Save", "Cancel", items, func(confirmed bool) {
		if !confirmed {
			return
		}
		target := config.Target{
			Address:  strings.TrimSpace(address.Text),
			Interval: config.Duration(parseDuration(interval.Text)),
			Timeout:  config.Duration(parseDuration(timeout.Text)),
			Enabled:  enabled.Checked,
		}
		if existing != nil {
			target.ID = existing.ID
			a.updateTarget(target)
			return
		}
		target.ID = config.NewID()
		a.cfg.Targets = append(a.cfg.Targets, target)
		a.mon.Upsert(target)
		a.saveConfig()
		a.rebuild()
	}, a.win).Show()
}

// updateTarget replaces a target and restarts its probe loop.
func (a *App) updateTarget(target config.Target) {
	for i := range a.cfg.Targets {
		if a.cfg.Targets[i].ID != target.ID {
			continue
		}
		a.cfg.Targets[i] = target
		break
	}
	a.mon.Upsert(target)
	a.saveConfig()
	a.rebuild()
}

// deleteTarget stops a target and forgets its history.
func (a *App) deleteTarget(target config.Target) {
	kept := make([]config.Target, 0, len(a.cfg.Targets))
	for _, t := range a.cfg.Targets {
		if t.ID != target.ID {
			kept = append(kept, t)
		}
	}
	a.cfg.Targets = kept
	a.mon.Remove(target.ID)
	a.hist.Forget(target.ID)
	a.saveConfig()
	a.rebuild()
}

// showSettings opens the dialog with the application settings.
func (a *App) showSettings() {
	window := widget.NewSelect(graphWindowLabels(), nil)
	window.SetSelected(labelForDuration(a.cfg.GraphWindow.Duration()))

	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(a.cfg.LogRetentionDays))
	retention.Validator = func(text string) error {
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return errors.New("the retention must be a number of days")
		}
		if value < 0 {
			return errors.New("the retention must not be negative")
		}
		return nil
	}

	mode := widget.NewSelect(icmModeOptions, nil)
	mode.SetSelected(string(a.cfg.ICMode))

	folder := widget.NewLabel(a.cfg.LogDir())
	folder.Wrapping = fyne.TextWrapWord
	open := widget.NewButtonWithIcon("Open log folder", theme.FolderOpenIcon(), func() {
		link, err := fileURL(a.cfg.LogDir())
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if err := a.fyneApp.OpenURL(link); err != nil {
			dialog.ShowError(err, a.win)
		}
	})

	cfgFile := widget.NewLabel(a.cfgPath)
	cfgFile.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Graph window", window),
		widget.NewFormItem("Keep logs for (days)", retention),
		widget.NewFormItem("ICMP mode", mode),
		widget.NewFormItem("Log folder", container.NewVBox(folder, open)),
		widget.NewFormItem("Configuration file", cfgFile),
	)

	var dlg *dialog.CustomDialog
	form.OnSubmit = func() {
		a.applySettings(window, retention, mode)
		dlg.Hide()
	}
	form.OnCancel = func() { dlg.Hide() }

	dlg = dialog.NewCustomWithoutButtons("Settings", form, a.win)
	dlg.Show()
}

// applySettings stores the changed settings and applies them.
func (a *App) applySettings(window *widget.Select, retention *widget.Entry, mode *widget.Select) {
	a.cfg.GraphWindow = config.Duration(durationForLabel(window.Selected))
	a.cfg.LogRetentionDays, _ = strconv.Atoi(strings.TrimSpace(retention.Text))
	a.cfg.ICMode = config.ICMode(mode.Selected)

	a.hist.SetWindow(a.cfg.GraphWindow.Duration())
	a.store.SetRetention(time.Duration(a.cfg.LogRetentionDays) * 24 * time.Hour)
	a.mon.SetICMode(a.cfg.ICMode)
	for _, t := range a.cfg.Targets {
		a.mon.Upsert(t)
	}

	a.saveConfig()
	a.rebuild()
}

// selectedTarget returns the target selected in the table.
func (a *App) selectedTarget() (config.Target, bool) {
	row := a.selectedRow - 1
	if row < 0 || row >= len(a.rows) {
		return config.Target{}, false
	}
	return a.rows[row], true
}

// saveConfig writes the configuration to disk.
func (a *App) saveConfig() {
	if err := config.Save(a.cfgPath, *a.cfg); err != nil {
		dialog.ShowError(err, a.win)
	}
}

// durationValidator checks that a field holds a positive duration.
func durationValidator(text string) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil {
		return errors.New("not a valid duration, for example 5s or 1m")
	}
	if parsed <= 0 {
		return errors.New("the duration must be positive")
	}
	return nil
}

// parseDuration parses a duration and returns zero when it is invalid.
func parseDuration(text string) time.Duration {
	parsed, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil {
		return 0
	}
	return parsed
}

// graphWindowLabels returns the selectable graph window labels.
func graphWindowLabels() []string {
	labels := make([]string, 0, len(graphWindowOptions))
	for _, option := range graphWindowOptions {
		labels = append(labels, option.Label)
	}
	return labels
}

// labelForDuration returns the label of a graph window length.
func labelForDuration(value time.Duration) string {
	for _, option := range graphWindowOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return graphWindowOptions[2].Label
}

// durationForLabel returns the length of a graph window label.
func durationForLabel(label string) time.Duration {
	for _, option := range graphWindowOptions {
		if option.Label == label {
			return option.Value
		}
	}
	return time.Hour
}
