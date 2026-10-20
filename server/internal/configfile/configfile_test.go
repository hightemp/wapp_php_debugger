package configfile

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "viewer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := store.New(db)
	if _, err := s.EnsureLocalServer(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func seed(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.EnsureLocalProject(ctx, "local-shop", "/srv/shop", "/srv/shop/tmp/logs"); err != nil {
		t.Fatal(err)
	}
	stage, err := s.CreateServer(ctx, store.ServerInput{Name: "stage", Kind: "ssh", Environment: "stage",
		Config: `{"host":"stage-web","user":"deploy","jump_hosts":["bastion"]}`, Permissions: &store.Permissions{ReadOnly: true}})
	if err != nil {
		t.Fatal(err)
	}
	days := 14
	if _, err := s.CreateProject(ctx, store.ProjectInput{ServerID: stage.ID, Name: "shop", RootPath: "/var/www/shop",
		LogsDir: "/var/www/shop/tmp/logs", LocalPath: "/home/dev/shop", Editor: "phpstorm", RetentionDays: &days}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateServer(ctx, store.ServerInput{Name: "hosting", Kind: "http", Environment: "prod", Config: `{"base_url":"https://shop.example.test"}`}); err != nil {
		t.Fatal(err)
	}
}

func TestExportHasNoSecretsAndImportsBackIntoAnEmptyViewer(t *testing.T) {
	ctx := context.Background()
	source := newStore(t)
	seed(t, source)
	data, err := Export(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"version: 1", "name: stage", "kind: ssh", "host: stage-web", "read_only: true", "local_path: /home/dev/shop", "days: 14", "base_url: https://shop.example.test", "name: local-shop"} {
		if !strings.Contains(text, want) {
			t.Errorf("export lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "token") {
		t.Fatalf("a token in the export:\n%s", text)
	}

	target := newStore(t)
	result, err := Import(ctx, target, data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.ServersCreated, ",") != "hosting,stage" || len(result.Projects) != 2 ||
		strings.Join(result.NeedsToken, ",") != "hosting" || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v", result)
	}
	again, err := Export(ctx, target)
	if err != nil || string(again) != text {
		t.Fatalf("export after import differs:\n%s\n---\n%s", again, text)
	}

	second, err := Import(ctx, target, data)
	if err != nil || len(second.ServersCreated) != 0 || len(second.Projects) != 0 || len(second.Skipped) != 4 {
		t.Fatalf("importing twice adds nothing: %+v, %v", second, err)
	}
}

func TestImportRejectsABrokenFileWhole(t *testing.T) {
	cases := map[string]string{
		"not yaml":      "servers: [",
		"other version": "version: 2\nservers: []\n",
		"unknown kind":  "version: 1\nservers:\n  - {name: x, kind: ftp}\n",
		"ssh no host":   "version: 1\nservers:\n  - {name: x, kind: ssh}\n",
		"http bad url":  "version: 1\nservers:\n  - {name: x, kind: http, config: {base_url: shop.test}}\n",
		"relative logs": "version: 1\nservers:\n  - name: x\n    kind: ssh\n    config: {host: h}\n    projects:\n      - {name: p, root_path: /r, logs_dir: logs}\n",
		"two on http":   "version: 1\nservers:\n  - name: x\n    kind: http\n    config: {base_url: 'https://a.test'}\n    projects:\n      - {name: a, root_path: /a, logs_dir: /}\n      - {name: b, root_path: /b, logs_dir: /b}\n",
		"secret key":    "version: 1\nservers:\n  - {name: x, kind: http, config: {base_url: 'https://a.test', token: abc}}\n",
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			if _, err := Import(context.Background(), s, []byte(data)); err == nil {
				t.Fatal("accepted")
			}
			servers, _ := s.ListServers(context.Background())
			if len(servers) != 1 {
				t.Fatalf("a rejected file left servers behind: %+v", servers)
			}
		})
	}
}
