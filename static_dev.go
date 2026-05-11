//go:build dev

package main

import "net/http"

// staticHandler returns a file server that reads directly from the static/
// directory on disk — used in development so edits are reflected immediately
// without rebuilding.
func staticHandler() http.Handler {
	return http.FileServer(http.Dir("static"))
}
