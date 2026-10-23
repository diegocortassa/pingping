package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// fileEntry is a directory or a file listed by the open log picker.
type fileEntry struct {
	name  string
	path  string
	isDir bool
}

// listDir returns the directories and the files of dir, directories first and
// then by name. Dot files are skipped, and when allFiles is false only CSV
// files are kept.
func listDir(dir string, allFiles bool) ([]fileEntry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var entries []fileEntry
	for _, item := range items {
		name := item.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			entries = append(entries, fileEntry{name: name, path: path, isDir: true})
			continue
		}
		if !allFiles && !strings.EqualFold(filepath.Ext(name), ".csv") {
			continue
		}
		entries = append(entries, fileEntry{name: name, path: path})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
	})
	return entries, nil
}

// filePicker is a resizable file requester shown in its own window.
type filePicker struct {
	app      *App
	win      fyne.Window
	dir      string
	showAll  bool
	entries  []fileEntry
	selected int
	path     *widget.Entry
	list     *widget.List
	open     *widget.Button
	status   *widget.Label
}

// showOpenLog asks for a log file with a picker window and opens it.
func (a *App) showOpenLog() {
	a.newFilePicker().win.Show()
}

func (a *App) newFilePicker() *filePicker {
	p := &filePicker{app: a, dir: pickerStartDir(a.cfg.LogDir()), selected: -1}
	p.win = a.fyneApp.NewWindow("Open log")
	p.win.Resize(fyne.NewSize(900, 560))
	p.build()
	p.navigate(p.dir)
	return p
}

func (p *filePicker) build() {
	p.path = widget.NewEntry()
	p.path.OnSubmitted = func(text string) { p.navigate(strings.TrimSpace(text)) }
	up := widget.NewButtonWithIcon("", theme.MoveUpIcon(), p.up)
	logs := widget.NewButton("Logs", func() { p.navigate(p.app.cfg.LogDir()) })
	pathBar := container.NewBorder(nil, nil, container.NewHBox(up, logs), nil, p.path)

	all := widget.NewCheck("All files", func(on bool) {
		p.showAll = on
		p.reload()
	})

	p.list = widget.NewList(
		func() int { return len(p.entries) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.FileIcon()), widget.NewLabel(""))
		},
		p.updateRow,
	)
	p.list.OnSelected = p.selectEntry

	p.open = widget.NewButton("Open", p.openSelected)
	p.open.Disable()
	cancel := widget.NewButton("Cancel", p.win.Close)
	p.status = widget.NewLabel("")
	buttons := container.NewHBox(all, cancel, p.open)

	p.win.SetContent(container.NewBorder(pathBar, container.NewBorder(nil, nil, p.status, buttons), nil, nil, p.list))
}

func (p *filePicker) updateRow(id widget.ListItemID, obj fyne.CanvasObject) {
	row := obj.(*fyne.Container)
	icon := row.Objects[0].(*widget.Icon)
	label := row.Objects[1].(*widget.Label)
	entry := p.entries[id]
	label.SetText(entry.name)
	if entry.isDir {
		icon.SetResource(theme.FolderIcon())
	} else {
		icon.SetResource(theme.FileIcon())
	}
}

// navigate shows the content of dir, or opens it when it is a file.
func (p *filePicker) navigate(dir string) {
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(p.dir, dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		p.fail(err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		p.fail(err)
		return
	}
	if !info.IsDir() {
		p.openFile(abs)
		return
	}
	p.dir = abs
	p.path.SetText(abs)
	p.reload()
}

func (p *filePicker) reload() {
	entries, err := listDir(p.dir, p.showAll)
	if err != nil {
		p.fail(err)
		return
	}
	p.entries = entries
	p.selected = -1
	p.list.UnselectAll()
	p.list.Refresh()
	p.open.Disable()
	p.status.SetText("")
}

func (p *filePicker) selectEntry(id widget.ListItemID) {
	entry := p.entries[id]
	if entry.isDir {
		p.navigate(entry.path)
		return
	}
	p.selected = int(id)
	p.status.SetText(entry.name)
	p.open.Enable()
}

func (p *filePicker) openSelected() {
	if p.selected < 0 || p.selected >= len(p.entries) {
		return
	}
	entry := p.entries[p.selected]
	if entry.isDir {
		p.navigate(entry.path)
		return
	}
	p.openFile(entry.path)
}

func (p *filePicker) openFile(path string) {
	if err := p.app.openLog(path); err != nil {
		p.fail(err)
		return
	}
	p.win.Close()
}

func (p *filePicker) up() {
	dir := filepath.Dir(p.dir)
	if dir != p.dir {
		p.navigate(dir)
	}
}

func (p *filePicker) fail(err error) {
	dialog.ShowError(err, p.win)
}

// pickerStartDir is the directory where the open log picker starts, the log
// folder when it exists and the home folder otherwise.
func pickerStartDir(logDir string) string {
	if info, err := os.Stat(logDir); err == nil && info.IsDir() {
		return logDir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return logDir
}
