package store

import (
	"context"
	"errors"
	"testing"
)

func TestCreateAndDeleteServers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	server, err := s.CreateServer(ctx, ServerInput{
		Name: "stage", Kind: "ssh", Environment: "stage", Config: `{"host":"stage-web","user":"deploy"}`,
		Permissions: &Permissions{ReadOnly: true, AllowDelete: false, AllowControl: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.ID == 0 || server.Kind != "ssh" || server.Environment != "stage" || server.Config != `{"host":"stage-web","user":"deploy"}` || !server.ReadOnly || server.AllowControl {
		t.Fatalf("server = %+v", server)
	}

	if _, err := s.CreateServer(ctx, ServerInput{Name: "stage", Kind: "ssh"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name: %v, want ErrConflict", err)
	}
	if _, err := s.CreateServer(ctx, ServerInput{Name: "x", Kind: "ftp"}); err == nil {
		t.Fatal("unknown kind must fail")
	}
	defaults, err := s.CreateServer(ctx, ServerInput{Name: "dev", Kind: "ssh"})
	if err != nil || defaults.Environment != "dev" || defaults.Config != "{}" || !defaults.AllowDelete || !defaults.AllowControl {
		t.Fatalf("defaults = %+v, %v", defaults, err)
	}

	project, err := s.CreateProject(ctx, ProjectInput{ServerID: server.ID, Name: "shop", RootPath: "/var/www/shop", LogsDir: "/var/www/shop/tmp/logs"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteServer(ctx, server.ID); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.GetProject(ctx, project.ID); found {
		t.Fatal("projects of a deleted server go with it")
	}
	if err := s.DeleteServer(ctx, server.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestTheLocalServerCannotBeDeleted(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.EnsureLocalServer(context.Background())
	if err := s.DeleteServer(context.Background(), id); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}
