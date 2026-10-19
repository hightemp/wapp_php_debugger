package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// specDir returns the path to the spec/ directory at the repository root.
func specDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "spec")
		if info, err := os.Stat(filepath.Join(candidate, "fixtures")); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("spec/ directory not found")
		}
		dir = parent
	}
}

func compileSchema(t *testing.T, relPath string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	schema, err := compiler.Compile(filepath.Join(specDir(t), relPath))
	if err != nil {
		t.Fatalf("compile %s: %v", relPath, err)
	}
	return schema
}

// decodeJSON parses JSON the way the validator expects (numbers as json.Number).
func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.More() {
		return nil, &json.SyntaxError{Offset: decoder.InputOffset()}
	}
	return value, nil
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
