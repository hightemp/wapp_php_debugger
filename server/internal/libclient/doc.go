// Package libclient is the client for the PHP library HTTP mode (spec/library-http.yaml) for servers without SSH.
// The code is generated into client.gen.go (make generate).
package libclient

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../spec/library-http.yaml
