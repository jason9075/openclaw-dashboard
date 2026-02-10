package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:assets
var assets embed.FS

// GetAssets returns the file system containing the UI assets.
// The root of the returned FS will contain index.html, style.css, etc.
func GetAssets() (fs.FS, error) {
	return fs.Sub(assets, "assets")
}
