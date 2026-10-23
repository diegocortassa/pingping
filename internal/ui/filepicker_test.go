package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestListDirPutsTheFoldersFirstAndKeepsTheCSVFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pings-b-2026-10-21.csv"), "")
	writeFile(t, filepath.Join(dir, "pings-a-2026-10-21.CSV"), "")
	writeFile(t, filepath.Join(dir, "notes.txt"), "")
	writeFile(t, filepath.Join(dir, ".hidden.csv"), "")
	if err := os.Mkdir(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := listDir(dir, false)
	if err != nil {
		t.Fatalf("listDir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if !entries[0].isDir || entries[0].name != "logs" {
		t.Fatalf("first entry is %+v, want the logs folder", entries[0])
	}
	if entries[1].name != "pings-a-2026-10-21.CSV" || entries[2].name != "pings-b-2026-10-21.csv" {
		t.Fatalf("unexpected order: %+v", entries)
	}
}

func TestListDirCanShowAllTheFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes.txt"), "")
	writeFile(t, filepath.Join(dir, "pings-2026-10-21.csv"), "")

	entries, err := listDir(dir, true)
	if err != nil {
		t.Fatalf("listDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestPickerStartDirPrefersTheLogFolder(t *testing.T) {
	dir := t.TempDir()
	if got := pickerStartDir(dir); got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}
	if got := pickerStartDir(filepath.Join(dir, "missing")); got == filepath.Join(dir, "missing") {
		t.Fatalf("got the missing folder %q", got)
	}
}

func TestNewFilePickerListsTheLogs(t *testing.T) {
	app, _ := newTestApp(t)
	dir := app.cfg.LogDir()
	if err := os.MkdirAll(filepath.Join(dir, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "pings-1.1.1.1-2026-10-21.csv"), "")
	writeFile(t, filepath.Join(dir, "notes.txt"), "")

	p := app.newFilePicker()
	if p.win == nil || p.win.Content() == nil {
		t.Fatal("the picker window has no content")
	}
	if len(p.entries) != 2 {
		t.Fatalf("got %d entries, want the folder and the CSV file", len(p.entries))
	}
	if !p.entries[0].isDir || p.entries[0].name != "old" {
		t.Fatalf("first entry is %+v, want the old folder", p.entries[0])
	}

	p.showAll = true
	p.reload()
	if len(p.entries) != 3 {
		t.Fatalf("got %d entries with all files, want 3", len(p.entries))
	}
}

func TestFilePickerOpensTheSelectedLog(t *testing.T) {
	app, _ := newTestApp(t)
	dir := app.cfg.LogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pings-1.1.1.1-2026-10-21.csv")
	writeFile(t, path,
		"timestamp,target_id,address,rtt_ms,success,error\n"+
			"2026-10-21T08:00:00Z,t1,1.1.1.1,10.000,true,\n"+
			"2026-10-21T08:00:01Z,t1,1.1.1.1,12.000,true,\n")

	p := app.newFilePicker()
	var id int
	for i, entry := range p.entries {
		if entry.path == path {
			id = i
		}
	}
	p.selectEntry(widget.ListItemID(id))
	if p.status.Text != filepath.Base(path) {
		t.Fatalf("status is %q", p.status.Text)
	}
	p.openSelected()

	found := false
	for _, w := range app.fyneApp.Driver().AllWindows() {
		if strings.HasPrefix(w.Title(), "PingPing - ") {
			found = true
		}
	}
	if !found {
		t.Fatal("the log window was not opened")
	}
}

func TestFilePickerNavigatesRelativeToTheCurrentFolder(t *testing.T) {
	app, _ := newTestApp(t)
	dir := app.cfg.LogDir()
	if err := os.MkdirAll(filepath.Join(dir, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := app.newFilePicker()

	p.navigate("old")
	if p.dir != filepath.Join(dir, "old") {
		t.Fatalf("got %q, want the old folder", p.dir)
	}
	if !strings.HasSuffix(p.path.Text, "old") {
		t.Fatalf("the path field shows %q", p.path.Text)
	}
}
