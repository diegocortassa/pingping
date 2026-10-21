package ui

import (
	"net/url"
	"path/filepath"
	"strings"
)

// fileURL builds the file URL of a path.
//
// A Windows path such as C:\Users\test\logs is not a URL path, the drive
// letter needs a leading slash and the separators have to be slashes,
// otherwise the parser reads the drive letter as the host and the rest as
// the port.
func fileURL(path string) (*url.URL, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return pathURL(filepath.ToSlash(absolute)), nil
}

// pathURL builds the file URL of an absolute path that uses slashes.
func pathURL(path string) *url.URL {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &url.URL{Scheme: "file", Path: path}
}
