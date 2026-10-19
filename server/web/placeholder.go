//go:build !embedui

// Package web serves the built UI. With the embedui tag (make build) it is embedded into the binary;
// without it a placeholder page is served so that go test and go vet do not depend on a UI build.
package web

import (
	"io/fs"
	"testing/fstest"
)

const placeholder = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>wapp-debugger</title></head>
<body><p>The UI is not embedded in this build. Build the binary with <code>make build</code>
or run <code>make ui-dev</code>. The API is available at <code>/api/v1/</code>.</p></body></html>
`

// UI returns the placeholder page.
func UI() fs.FS {
	return fstest.MapFS{"index.html": {Data: []byte(placeholder)}}
}
