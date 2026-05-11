//go:build !dev

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFiles embed.FS

// staticHandler returns a file server backed by the embedded static/ tree —
// used in production builds so no external static/ directory is required.
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("embed: cannot sub static/ — " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
