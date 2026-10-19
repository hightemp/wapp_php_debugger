package api

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func specPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "../../../spec/openapi.yaml")
}

func operationIDs(doc *openapi3.T) string {
	var ids []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			// oapi-codegen переводит operationId в CamelCase перед встраиванием
			ids = append(ids, strings.ToLower(op.OperationID))
		}
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// Сгенерированный код содержит копию спецификации: если она отличается от spec/openapi.yaml,
// значит после правки спецификации забыли выполнить make generate.
func TestGeneratedCodeMatchesSpec(t *testing.T) {
	embedded, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	current, err := openapi3.NewLoader().LoadFromFile(specPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := operationIDs(embedded), operationIDs(current); got != want {
		t.Fatalf("generated operations %s, spec has %s: run make generate", got, want)
	}
	if got, want := schemaFields(embedded), schemaFields(current); got != want {
		t.Fatalf("generated schemas differ from spec: run make generate\ngenerated: %s\nspec:      %s", got, want)
	}
}

// schemaFields — отпечаток схем: имя схемы и отсортированные имена её свойств.
func schemaFields(doc *openapi3.T) string {
	var parts []string
	for name, ref := range doc.Components.Schemas {
		var fields []string
		for field := range ref.Value.Properties {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		parts = append(parts, name+"("+strings.Join(fields, ",")+")")
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
