//go:build embedui

// Package web serves the built UI embedded into the binary (built with the embedui tag).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// UI returns the built Svelte UI (make ui-build puts it into server/web/dist).
func UI() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
