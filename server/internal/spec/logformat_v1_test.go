package spec

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

type v1Manifest struct {
	Fixtures []struct {
		File           string `json:"file"`
		HeaderValid    bool   `json:"header_valid"`
		MessagesValid  int    `json:"messages_valid"`
		MessagesBroken int    `json:"messages_broken"`
	} `json:"fixtures"`
}

// Все реальные строки логов v1 должны проходить схему, а битые — не разбираться как JSON:
// так схема описывает формат как он есть, а не как хотелось бы.
func TestV1FixturesMatchSchemaAndManifest(t *testing.T) {
	header := compileSchema(t, "logformat/v1/header.schema.json")
	message := compileSchema(t, "logformat/v1/message.schema.json")

	var manifest v1Manifest
	if err := json.Unmarshal(readFile(t, filepath.Join(specDir(t), "fixtures/v1/manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) == 0 {
		t.Fatal("manifest has no fixtures")
	}

	for _, fixture := range manifest.Fixtures {
		t.Run(fixture.File, func(t *testing.T) {
			lines := bytes.Split(readFile(t, filepath.Join(specDir(t), "fixtures/v1", fixture.File)), []byte("\n"))

			headerValue, err := decodeJSON(bytes.TrimRight(lines[0], " "))
			if fixture.HeaderValid {
				if err != nil {
					t.Fatalf("header is not JSON: %v", err)
				}
				if err := header.Validate(headerValue); err != nil {
					t.Fatalf("header does not match schema: %v", err)
				}
			}

			valid, broken := 0, 0
			for number, line := range lines[1:] {
				if len(line) == 0 {
					continue
				}
				value, err := decodeJSON(line)
				if err != nil {
					broken++
					continue
				}
				if err := message.Validate(value); err != nil {
					t.Errorf("line %d does not match message schema: %v", number+2, err)
					continue
				}
				valid++
			}
			if valid != fixture.MessagesValid || broken != fixture.MessagesBroken {
				t.Fatalf("valid/broken = %d/%d, manifest says %d/%d", valid, broken, fixture.MessagesValid, fixture.MessagesBroken)
			}
		})
	}
}

func TestV1SchemasRejectWrongShapes(t *testing.T) {
	header := compileSchema(t, "logformat/v1/header.schema.json")
	message := compileSchema(t, "logformat/v1/message.schema.json")

	cases := []struct {
		name   string
		schema interface{ Validate(any) error }
		json   string
	}{
		{"message as header", header, `{"sType":"info","sMessage":"x","oVars":{},"iTimestamp":1}`},
		{"header without timestamp", header, `{"sURL":"","bIsCli":true,"bFinished":false}`},
		{"message without type", message, `{"sMessage":"x","oVars":{},"iTimestamp":1}`},
		{"message with object text", message, `{"sType":"info","sMessage":{"a":1},"oVars":{},"iTimestamp":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, err := decodeJSON([]byte(c.json))
			if err != nil {
				t.Fatal(err)
			}
			if c.schema.Validate(value) == nil {
				t.Fatal("schema accepted invalid document")
			}
		})
	}
}
