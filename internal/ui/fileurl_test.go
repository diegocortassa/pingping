package ui

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestPathURL(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "windows drive",
			path: "C:/Users/dcortassa/AppData/Roaming/pingping/data/logs",
			want: "file:///C:/Users/dcortassa/AppData/Roaming/pingping/data/logs",
		},
		{
			name: "unix path",
			path: "/home/user/.config/pingping/data/logs",
			want: "file:///home/user/.config/pingping/data/logs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pathURL(test.path)
			if got.String() != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			if got.Scheme != "file" {
				t.Fatalf("got scheme %q, want file", got.Scheme)
			}
			if got.Host != "" {
				t.Fatalf("got host %q, want none", got.Host)
			}
		})
	}
}

func TestPathURLIsParsable(t *testing.T) {
	for _, path := range []string{
		"C:/Users/dcortassa/AppData/Roaming/pingping/data/logs",
		"/home/user/.config/pingping/data/logs",
	} {
		link := pathURL(path)
		parsed, err := url.Parse(link.String())
		if err != nil {
			t.Fatalf("parse %q: %v", link, err)
		}
		if parsed.Scheme != "file" {
			t.Fatalf("got scheme %q, want file", parsed.Scheme)
		}
		if parsed.Host != "" {
			t.Fatalf("got host %q, want none", parsed.Host)
		}
		if parsed.Path != link.Path {
			t.Fatalf("got path %q, want %q", parsed.Path, link.Path)
		}
	}
}

func TestFileURL(t *testing.T) {
	dir := t.TempDir()

	link, err := fileURL(dir)
	if err != nil {
		t.Fatalf("file url: %v", err)
	}
	if link.Scheme != "file" {
		t.Fatalf("got scheme %q, want file", link.Scheme)
	}

	absolute, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if link.Path != filepath.ToSlash(absolute) {
		t.Fatalf("got path %q, want %q", link.Path, filepath.ToSlash(absolute))
	}
}
