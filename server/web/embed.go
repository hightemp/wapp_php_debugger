//go:build embedui

// Package web отдаёт собранный UI, встроенный в бинарник (сборка с тегом embedui).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// UI возвращает собранный Svelte-UI (make ui-build кладёт его в server/web/dist).
func UI() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
