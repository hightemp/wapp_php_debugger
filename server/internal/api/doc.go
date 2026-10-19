// Package api is the viewer HTTP API defined by spec/openapi.yaml.
// Types and the server interface are generated into openapi.gen.go (make generate); handlers live here.
package api

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../spec/openapi.yaml
