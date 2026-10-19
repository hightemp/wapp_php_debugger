package store

import (
	"context"
	"errors"
	"testing"
)

func TestServersAndProjects(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	local, err := s.EnsureLocalServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := s.EnsureLocalServer(ctx)
	if again != local {
		t.Fatal("the local server is created once")
	}

	servers, err := s.ListServers(ctx)
	if err != nil || len(servers) != 1 || servers[0].Kind != "local" || servers[0].Name != "local" || servers[0].ReadOnly {
		t.Fatalf("servers = %+v, %v", servers, err)
	}

	created, err := s.CreateProject(ctx, ProjectInput{ServerID: local, Name: "shop", RootPath: "/var/www/shop", LogsDir: "/var/www/shop/tmp/logs", Editor: "phpstorm"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "shop" || created.Editor != "phpstorm" || created.LocalPath != "" || created.CreatedAt == 0 {
		t.Fatalf("created = %+v", created)
	}
	if _, err := s.CreateProject(ctx, ProjectInput{ServerID: local, Name: "shop", RootPath: "/x", LogsDir: "/x/logs"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name err = %v, want ErrConflict", err)
	}
	if _, err := s.CreateProject(ctx, ProjectInput{ServerID: 999, Name: "x", RootPath: "/x", LogsDir: "/x/logs"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown server err = %v, want ErrNotFound", err)
	}

	local2 := "/home/dev/shop"
	updated, err := s.UpdateProject(ctx, created.ID, ProjectPatch{LocalPath: &local2, RetentionDays: intPtr(7)})
	if err != nil || updated.LocalPath != local2 || updated.RetentionDays == nil || *updated.RetentionDays != 7 || updated.Name != "shop" {
		t.Fatalf("updated = %+v, %v", updated, err)
	}
	empty := ""
	cleared, _ := s.UpdateProject(ctx, created.ID, ProjectPatch{LocalPath: &empty})
	if cleared.LocalPath != "" {
		t.Fatal("an empty local path clears it")
	}

	projects, err := s.ListProjects(ctx, 0)
	if err != nil || len(projects) != 1 {
		t.Fatalf("projects = %+v, %v", projects, err)
	}
	if byServer, _ := s.ListProjects(ctx, 999); len(byServer) != 0 {
		t.Fatal("filter by server")
	}

	if err := s.DeleteProject(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.GetProject(ctx, created.ID); found {
		t.Fatal("deleted")
	}
	if err := s.DeleteProject(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v", err)
	}
	if _, err := s.UpdateProject(ctx, created.ID, ProjectPatch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing err = %v", err)
	}
}

func TestGetServerPermissions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	local, _ := s.EnsureLocalServer(ctx)
	server, found, err := s.GetServer(ctx, local)
	if err != nil || !found || !server.AllowDelete || !server.AllowControl || server.Environment != "local" || server.Config != "{}" {
		t.Fatalf("server = %+v, %v", server, err)
	}
}

// The v1 library keeps dws_debugger.json next to Debugger.class.php, which is not always the project root.
func TestProjectConfigPathDefaultsToTheRoot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	local, _ := s.EnsureLocalServer(ctx)
	plain, _ := s.CreateProject(ctx, ProjectInput{ServerID: local, Name: "a", RootPath: "/var/www/a", LogsDir: "/var/www/a/logs"})
	if plain.ConfigPath != "/var/www/a/dws_debugger.json" {
		t.Fatalf("default config path = %q", plain.ConfigPath)
	}
	custom, _ := s.CreateProject(ctx, ProjectInput{ServerID: local, Name: "b", RootPath: "/var/www/b", LogsDir: "/var/www/b/logs", ConfigPath: "/var/www/b/lib/dws_debugger.json"})
	if custom.ConfigPath != "/var/www/b/lib/dws_debugger.json" {
		t.Fatalf("custom config path = %q", custom.ConfigPath)
	}
}

func intPtr(v int) *int { return &v }
