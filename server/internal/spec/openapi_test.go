package spec

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

var snakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func loadOpenAPI(t *testing.T, name string) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filepath.Join(specDir(t), name))
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	if err := doc.Validate(context.Background(), openapi3.EnableExamplesValidation()); err != nil {
		t.Fatalf("validate %s: %v", name, err)
	}
	return doc
}

type operationRef struct {
	method, path string
	op           *openapi3.Operation
}

func operations(doc *openapi3.T) []operationRef {
	var result []operationRef
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			result = append(result, operationRef{method, path, op})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path+result[i].method < result[j].path+result[j].method })
	return result
}

// collectPropertyNames обходит все схемы документа, включая вложенные, и возвращает имена свойств.
func collectPropertyNames(doc *openapi3.T) map[string]string {
	names := map[string]string{}
	seen := map[*openapi3.Schema]bool{}
	var walk func(where string, ref *openapi3.SchemaRef)
	walk = func(where string, ref *openapi3.SchemaRef) {
		if ref == nil || ref.Value == nil || seen[ref.Value] {
			return
		}
		schema := ref.Value
		seen[schema] = true
		for name, property := range schema.Properties {
			names[name] = where
			walk(where+"."+name, property)
		}
		walk(where+"[]", schema.Items)
		if schema.AdditionalProperties.Schema != nil {
			walk(where+"{}", schema.AdditionalProperties.Schema)
		}
		for _, list := range [][]*openapi3.SchemaRef{schema.AllOf, schema.OneOf, schema.AnyOf} {
			for _, item := range list {
				walk(where, item)
			}
		}
	}
	for name, ref := range doc.Components.Schemas {
		walk(name, ref)
	}
	for _, o := range operations(doc) {
		for _, p := range o.op.Parameters {
			// Заголовки называются по правилам HTTP (Last-Event-ID), snake_case — для query и path
			if p.Value.In != openapi3.ParameterInHeader {
				names[p.Value.Name] = o.method + " " + o.path + " parameter"
			}
		}
	}
	return names
}

func checkCommonRules(t *testing.T, doc *openapi3.T) {
	t.Helper()
	ids := map[string]bool{}
	for _, o := range operations(doc) {
		where := o.method + " " + o.path
		if o.op.OperationID == "" || ids[o.op.OperationID] {
			t.Errorf("%s: operationId missing or duplicated (%q)", where, o.op.OperationID)
		}
		ids[o.op.OperationID] = true
		if o.op.Summary == "" {
			t.Errorf("%s: summary is required", where)
		}
		hasSuccess := false
		for code := range o.op.Responses.Map() {
			if strings.HasPrefix(code, "2") {
				hasSuccess = true
			}
		}
		if !hasSuccess {
			t.Errorf("%s: no 2xx response", where)
		}
		problem := o.op.Responses.Default()
		if problem == nil || problem.Value.Content.Get("application/problem+json") == nil {
			t.Errorf("%s: default response must be application/problem+json", where)
		}
	}
	for name, where := range collectPropertyNames(doc) {
		if !snakeCase.MatchString(name) {
			t.Errorf("%s: name %q is not snake_case (п. 10.17)", where, name)
		}
	}
}

func TestViewerOpenAPI(t *testing.T) {
	doc := loadOpenAPI(t, "openapi.yaml")
	checkCommonRules(t, doc)
	for _, o := range operations(doc) {
		if !strings.HasPrefix(o.path, "/api/v1/") {
			t.Errorf("%s %s: viewer API paths start with /api/v1/", o.method, o.path)
		}
	}
	for _, required := range []string{"listServers", "listProjects", "listRequests", "getRequest", "listMessages", "getMessage", "streamEvents", "getSettings"} {
		found := false
		for _, o := range operations(doc) {
			found = found || o.op.OperationID == required
		}
		if !found {
			t.Errorf("operation %s is missing", required)
		}
	}
}

func TestLibraryHTTPOpenAPI(t *testing.T) {
	doc := loadOpenAPI(t, "library-http.yaml")
	checkCommonRules(t, doc)
	for _, o := range operations(doc) {
		security := doc.Security
		if o.op.Security != nil {
			security = *o.op.Security
		}
		if len(security) == 0 {
			t.Errorf("%s %s: every library endpoint requires the token (п. 4.1)", o.method, o.path)
		}
	}
	for name, scheme := range doc.Components.SecuritySchemes {
		if scheme.Value.Type != "apiKey" || scheme.Value.In != "header" || scheme.Value.Name != "X-Debugger-Token" {
			t.Errorf("security scheme %s: token must be passed only in the X-Debugger-Token header", name)
		}
	}
}
