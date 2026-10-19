package sshconn

import (
	"reflect"
	"strings"
	"testing"
)

const sampleConfig = `
Host shop-stage
    HostName 10.0.0.5
    User deploy
    Port 2222
    IdentityFile ~/.ssh/stage_key
    ProxyJump bastion

Host bastion
    HostName bastion.example.com
    User jump

Host legacy
    HostName old.example.com
    ProxyJump none

Host *.internal
    User ops
    UserKnownHostsFile ~/.ssh/known_hosts_internal
`

func resolver(t *testing.T) *Resolver {
	t.Helper()
	r, err := NewResolver(strings.NewReader(sampleConfig), "/home/dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolveAnAlias(t *testing.T) {
	got, err := resolver(t).Resolve("shop-stage")
	if err != nil {
		t.Fatal(err)
	}
	want := Target{
		Alias: "shop-stage", Host: "10.0.0.5", Port: 2222, User: "deploy",
		IdentityFiles:   []string{"/home/dev/.ssh/stage_key"},
		KnownHostsFiles: []string{"/home/dev/.ssh/known_hosts", "/home/dev/.ssh/known_hosts2"},
		Jumps:           []Target{{Alias: "bastion", Host: "bastion.example.com", Port: 22, User: "jump", IdentityFiles: defaultIdentities("/home/dev"), KnownHostsFiles: []string{"/home/dev/.ssh/known_hosts", "/home/dev/.ssh/known_hosts2"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestResolveAHostWithoutAnEntry(t *testing.T) {
	got, err := resolver(t).Resolve("plain.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "plain.example.com" || got.Port != 22 || got.User != "dev" || len(got.Jumps) != 0 {
		t.Fatalf("got %+v", got)
	}
	if !reflect.DeepEqual(got.IdentityFiles, defaultIdentities("/home/dev")) {
		t.Fatalf("identities %v", got.IdentityFiles)
	}
}

func TestResolveWildcardsAndKnownHosts(t *testing.T) {
	got, err := resolver(t).Resolve("db.internal")
	if err != nil {
		t.Fatal(err)
	}
	if got.User != "ops" || !reflect.DeepEqual(got.KnownHostsFiles, []string{"/home/dev/.ssh/known_hosts_internal"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestProxyJumpForms(t *testing.T) {
	r := resolver(t)
	if got, _ := r.Resolve("legacy"); len(got.Jumps) != 0 {
		t.Fatalf("ProxyJump none: %+v", got.Jumps)
	}
	jumps, err := r.parseJumps("admin@gate.example.com:2200,bastion")
	if err != nil {
		t.Fatal(err)
	}
	if len(jumps) != 2 || jumps[0].Host != "gate.example.com" || jumps[0].User != "admin" || jumps[0].Port != 2200 || jumps[1].Host != "bastion.example.com" {
		t.Fatalf("jumps %+v", jumps)
	}
}

func TestUserAndPortOverrides(t *testing.T) {
	got, err := resolver(t).Resolve("root@shop-stage:2022")
	if err != nil {
		t.Fatal(err)
	}
	if got.User != "root" || got.Port != 2022 || got.Host != "10.0.0.5" || len(got.Jumps) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestAddress(t *testing.T) {
	if got := (Target{Host: "::1", Port: 22}).Address(); got != "[::1]:22" {
		t.Fatal(got)
	}
}

func TestBadPort(t *testing.T) {
	r, _ := NewResolver(strings.NewReader("Host x\n    Port many\n"), "/home/dev", "dev")
	if _, err := r.Resolve("x"); err == nil {
		t.Fatal("want an error for a bad port")
	}
}
