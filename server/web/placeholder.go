//go:build !embedui

// Package web отдаёт собранный UI. С тегом embedui (make build) он встроен в бинарник;
// без тега — заглушка, чтобы go test и go vet не зависели от сборки UI.
package web

import (
	"io/fs"
	"testing/fstest"
)

const placeholder = `<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><title>wapp-debugger</title></head>
<body><p>Интерфейс не встроен в эту сборку. Соберите бинарник командой <code>make build</code>
или запустите <code>make ui-dev</code>. API доступен по адресу <code>/api/v1/</code>.</p></body></html>
`

// UI возвращает страницу-заглушку.
func UI() fs.FS {
	return fstest.MapFS{"index.html": {Data: []byte(placeholder)}}
}
