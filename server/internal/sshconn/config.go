// Package sshconn connects to project servers over SSH (plan 7.4): targets from ~/.ssh/config,
// ssh-agent and key files, host keys from known_hosts with confirmation of new ones, ProxyJump
// chains, keepalive and one shared connection per server.
package sshconn

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// Target is a resolved SSH destination: what ssh would connect to for a host or an alias.
type Target struct {
	Alias           string // what the user wrote: an alias of ~/.ssh/config or a host name
	Host            string
	Port            int
	User            string
	IdentityFiles   []string
	KnownHostsFiles []string
	Jumps           []Target // ProxyJump hosts, in order
}

// Address is host:port for net.Dial.
func (t Target) Address() string {
	return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}

// Resolver reads targets from an ssh config, like ssh -G.
type Resolver struct {
	config *ssh_config.Config
	home   string
	user   string
}

// NewResolver parses an ssh config (nil means none); home and user fill in ~ and the default user.
func NewResolver(config io.Reader, home, user string) (*Resolver, error) {
	r := &Resolver{home: home, user: user}
	if config == nil {
		config = strings.NewReader("")
	}
	parsed, err := ssh_config.Decode(config)
	if err != nil {
		return nil, fmt.Errorf("ssh config: %w", err)
	}
	r.config = parsed
	return r, nil
}

// Resolve turns "alias", "host", "user@host" or "user@host:port" into a target.
func (r *Resolver) Resolve(destination string) (Target, error) {
	return r.resolve(destination, 0)
}

const maxJumpDepth = 5

func (r *Resolver) resolve(destination string, depth int) (Target, error) {
	if depth > maxJumpDepth {
		return Target{}, errors.New("ProxyJump chain is too long or loops")
	}
	user, alias, port, err := splitDestination(destination)
	if err != nil {
		return Target{}, err
	}
	t := Target{Alias: alias, Host: alias, Port: 22, User: r.user}
	if host := r.get(alias, "HostName"); host != "" {
		t.Host = strings.ReplaceAll(host, "%h", alias)
	}
	if value := r.get(alias, "Port"); value != "" {
		if t.Port, err = strconv.Atoi(value); err != nil || t.Port < 1 || t.Port > 65535 {
			return Target{}, fmt.Errorf("ssh config: Host %s: bad Port %q", alias, value)
		}
	}
	if value := r.get(alias, "User"); value != "" {
		t.User = value
	}
	if user != "" {
		t.User = user
	}
	if port != 0 {
		t.Port = port
	}
	for _, file := range r.getAll(alias, "IdentityFile") {
		t.IdentityFiles = append(t.IdentityFiles, r.expand(file))
	}
	if len(t.IdentityFiles) == 0 {
		t.IdentityFiles = defaultIdentities(r.home)
	}
	knownHosts := r.get(alias, "UserKnownHostsFile")
	if knownHosts == "" {
		knownHosts = "~/.ssh/known_hosts ~/.ssh/known_hosts2"
	}
	for _, file := range strings.Fields(knownHosts) {
		t.KnownHostsFiles = append(t.KnownHostsFiles, r.expand(file))
	}
	if jumps := r.get(alias, "ProxyJump"); jumps != "" && !strings.EqualFold(jumps, "none") {
		if t.Jumps, err = r.parseJumpsAt(jumps, depth+1); err != nil {
			return Target{}, err
		}
	}
	return t, nil
}

func (r *Resolver) parseJumps(spec string) ([]Target, error) {
	return r.parseJumpsAt(spec, 1)
}

// parseJumpsAt resolves "host1,user@host2:port"; every jump host is looked up in the config too.
func (r *Resolver) parseJumpsAt(spec string, depth int) ([]Target, error) {
	var jumps []Target
	for _, part := range strings.Split(spec, ",") {
		jump, err := r.resolve(strings.TrimSpace(part), depth)
		if err != nil {
			return nil, err
		}
		// A jump host reached through its own ProxyJump comes first, as ssh does it
		jumps = append(jumps, jump.Jumps...)
		jump.Jumps = nil
		jumps = append(jumps, jump)
	}
	return jumps, nil
}

func (r *Resolver) get(alias, key string) string {
	value, err := r.config.Get(alias, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (r *Resolver) getAll(alias, key string) []string {
	values, err := r.config.GetAll(alias, key)
	if err != nil {
		return nil
	}
	return values
}

func (r *Resolver) expand(path string) string {
	if path == "~" {
		return r.home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(r.home, path[2:])
	}
	return strings.ReplaceAll(path, "%d", r.home)
}

// defaultIdentities are the key files ssh tries when the config names none.
func defaultIdentities(home string) []string {
	return []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}
}

// splitDestination splits "user@host:port" ("[::1]:22" for IPv6); port 0 means not given.
func splitDestination(destination string) (user, host string, port int, err error) {
	host = destination
	if at := strings.LastIndex(host, "@"); at >= 0 {
		user, host = host[:at], host[at+1:]
	}
	if strings.HasPrefix(host, "[") || strings.Count(host, ":") == 1 {
		h, p, splitErr := net.SplitHostPort(host)
		if splitErr == nil {
			if port, err = strconv.Atoi(p); err != nil || port < 1 || port > 65535 {
				return "", "", 0, fmt.Errorf("bad port in %q", destination)
			}
			host = h
		}
	}
	if host == "" {
		return "", "", 0, fmt.Errorf("no host in %q", destination)
	}
	return user, host, port, nil
}

// ServerSettings is the config of an ssh server in the viewer (spec/openapi.yaml ServerConfig).
type ServerSettings struct {
	Host      string   `json:"host"` // a host name or an alias from ~/.ssh/config
	Port      int      `json:"port,omitempty"`
	User      string   `json:"user,omitempty"`
	JumpHosts []string `json:"jump_hosts,omitempty"`
}

// ParseServerSettings reads the stored config of a server.
func ParseServerSettings(config string) (ServerSettings, error) {
	var settings ServerSettings
	if err := json.Unmarshal([]byte(config), &settings); err != nil {
		return ServerSettings{}, fmt.Errorf("server config: %w", err)
	}
	return settings, nil
}

// ResolveServer resolves the host through the ssh config; user and port set in the viewer win over
// it, and jump hosts set in the viewer replace its ProxyJump.
func (r *Resolver) ResolveServer(settings ServerSettings) (Target, error) {
	if strings.TrimSpace(settings.Host) == "" {
		return Target{}, errors.New("server config: no host")
	}
	target, err := r.Resolve(settings.Host)
	if err != nil {
		return Target{}, err
	}
	if settings.User != "" {
		target.User = settings.User
	}
	if settings.Port != 0 {
		target.Port = settings.Port
	}
	if len(settings.JumpHosts) > 0 {
		if target.Jumps, err = r.parseJumps(strings.Join(settings.JumpHosts, ",")); err != nil {
			return Target{}, err
		}
	}
	return target, nil
}
