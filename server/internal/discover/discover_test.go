package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byRoot(found []Found) map[string]Found {
	out := map[string]Found{}
	for _, f := range found {
		out[f.Root] = f
	}
	return out
}

func TestFindsProjectsByTheirLibraryConfig(t *testing.T) {
	www := t.TempDir()
	write(t, filepath.Join(www, "shop", "dws_debugger.json"), `{"enabled": true, "logs_dir": "var/logs"}`)
	write(t, filepath.Join(www, "blog", "public", "dws_debugger.json"), `{"project_root": "..", "enabled": false}`)
	write(t, filepath.Join(www, "legacy", "dws_debugger.json"), `{"bIsOnFlag": true, "sMessagesFilesPath": "/srv/legacy-logs"}`)
	write(t, filepath.Join(www, "broken", "dws_debugger.json"), `{not json`)
	write(t, filepath.Join(www, "shop", "vendor", "lib", "dws_debugger.json"), `{}`)
	write(t, filepath.Join(www, "shop", "node_modules", "x", "dws_debugger.json"), `{}`)

	found, truncated, err := Find(context.Background(), Local{}, []string{www}, Options{})
	if err != nil || truncated {
		t.Fatalf("err=%v truncated=%v", err, truncated)
	}
	projects := byRoot(found)
	if len(projects) != 4 {
		t.Fatalf("found %+v", found)
	}
	shop := projects[filepath.Join(www, "shop")]
	if shop.Name != "shop" || shop.LogsDir != filepath.Join(www, "shop", "var", "logs") || !shop.Enabled ||
		shop.ConfigPath != filepath.Join(www, "shop", "dws_debugger.json") {
		t.Errorf("shop = %+v", shop)
	}
	blog := projects[filepath.Join(www, "blog")]
	if blog.LogsDir != filepath.Join(www, "blog", "tmp", "logs") || blog.Enabled || blog.ConfigPath != filepath.Join(www, "blog", "public", "dws_debugger.json") {
		t.Errorf("blog: project_root up from the config, default logs dir: %+v", blog)
	}
	if legacy := projects[filepath.Join(www, "legacy")]; legacy.LogsDir != "/srv/legacy-logs" || !legacy.Enabled {
		t.Errorf("legacy v1 keys: %+v", legacy)
	}
	if broken := projects[filepath.Join(www, "broken")]; broken.LogsDir != filepath.Join(www, "broken", "tmp", "logs") || broken.Problem == "" {
		t.Errorf("a broken config is still a project, with the problem named: %+v", broken)
	}
}

func TestStaysWithinTheLimits(t *testing.T) {
	www := t.TempDir()
	write(t, filepath.Join(www, "a", "b", "c", "d", "e", "dws_debugger.json"), `{}`)
	write(t, filepath.Join(www, "a", "dws_debugger.json"), `{}`)

	found, _, _ := Find(context.Background(), Local{}, []string{www}, Options{MaxDepth: 2})
	if len(found) != 1 || found[0].Root != filepath.Join(www, "a") {
		t.Fatalf("depth 2: %+v", found)
	}

	for i := range 30 {
		if err := os.MkdirAll(filepath.Join(www, "many", string(rune('a'+i%26))+string(rune('a'+i/26))), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, truncated, _ := Find(context.Background(), Local{}, []string{www}, Options{MaxDirs: 10})
	if !truncated {
		t.Fatal("the walk past MaxDirs is not reported")
	}
}

func TestDoesNotFollowSymlinks(t *testing.T) {
	www := t.TempDir()
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret", "dws_debugger.json"), `{}`)
	if err := os.Symlink(outside, filepath.Join(www, "link")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(www, filepath.Join(www, "loop")); err != nil {
		t.Fatal(err)
	}
	found, _, err := Find(context.Background(), Local{}, []string{www}, Options{})
	if err != nil || len(found) != 0 {
		t.Fatalf("found %+v, %v", found, err)
	}
}

func TestAMissingRootIsAnError(t *testing.T) {
	if _, _, err := Find(context.Background(), Local{}, []string{filepath.Join(t.TempDir(), "nope")}, Options{}); err == nil {
		t.Fatal("a missing root went unnoticed")
	}
}
