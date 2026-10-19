// Package api — HTTP API просмотрщика по контракту spec/openapi.yaml.
// Типы и интерфейс сервера генерируются в openapi.gen.go (make generate), обработчики пишутся здесь.
package api

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../spec/openapi.yaml
