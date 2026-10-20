package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/hightemp/wapp_php_debugger/server/internal/api"
	"github.com/hightemp/wapp_php_debugger/server/internal/secrets"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// httpRuntime is api.HTTPRuntime: tokens in the secrets file, the connection state from the polling.
type httpRuntime struct {
	secrets  *secrets.File
	client   *http.Client
	onStatus func(serverID int64, status sshconn.Status)

	mu       sync.Mutex
	statuses map[int64]sshconn.Status
}

var _ api.HTTPRuntime = (*httpRuntime)(nil)

func newHTTPRuntime(secretsFile string, client *http.Client, onStatus func(int64, sshconn.Status)) *httpRuntime {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second} // certificates are checked, always
	}
	return &httpRuntime{secrets: secrets.NewFile(secretsFile), client: client, onStatus: onStatus, statuses: map[int64]sshconn.Status{}}
}

func tokenKey(serverID int64) string { return fmt.Sprintf("server/%d/token", serverID) }

func (r *httpRuntime) SetToken(serverID int64, token string) error {
	return r.secrets.Set(tokenKey(serverID), token)
}

func (r *httpRuntime) Forget(serverID int64) error {
	r.mu.Lock()
	delete(r.statuses, serverID)
	r.mu.Unlock()
	return r.secrets.Delete(tokenKey(serverID))
}

func (r *httpRuntime) Status(serverID int64) sshconn.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	if status, ok := r.statuses[serverID]; ok {
		return status
	}
	return sshconn.Status{State: sshconn.StateUnknown}
}

// record turns the result of a request into the connection state and publishes a change.
func (r *httpRuntime) record(serverID int64, err error) {
	status := sshconn.Status{State: sshconn.StateOnline}
	// The library answered these itself
	answered := err == nil || errors.Is(err, source.ErrNotFound) || errors.Is(err, source.ErrOffsetBeyondEnd) || errors.Is(err, source.ErrRateLimited)
	if !answered {
		status = sshconn.Status{State: sshconn.StateOffline, Error: err.Error()}
	}
	r.mu.Lock()
	previous, known := r.statuses[serverID]
	changed := !known || previous.State != status.State || previous.Error != status.Error
	if changed {
		status.Since = time.Now()
		r.statuses[serverID] = status
	}
	r.mu.Unlock()
	if changed && r.onStatus != nil {
		r.onStatus(serverID, status)
	}
}

// baseURL reads config.base_url of an http server.
func baseURL(server store.Server) (string, error) {
	var config struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.Unmarshal([]byte(server.Config), &config); err != nil {
		return "", err
	}
	return config.BaseURL, nil
}

// source makes the source of a project on the server; its requests feed the connection state.
func (r *httpRuntime) source(server store.Server) (*source.HTTP, error) {
	base, err := baseURL(server)
	if err != nil {
		return nil, err
	}
	token, found, err := r.secrets.Get(tokenKey(server.ID))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no token for this server: set it again in the server settings")
	}
	return source.NewHTTP(source.HTTPOptions{BaseURL: base, Token: token, Client: r.client,
		OnResult: func(err error) { r.record(server.ID, err) }})
}

// Check asks the library for its status: the address, the token, the logs directory and PHP.
func (r *httpRuntime) Check(ctx context.Context, server store.Server, _ []store.Project) (api.ServerCheck, error) {
	check := api.ServerCheck{Ok: true}
	optional := true
	add := func(s api.ServerCheckStep, err error, detail string) bool {
		s.Ok = err == nil
		if err != nil {
			detail = err.Error()
			if s.Optional == nil {
				check.Ok = false
			}
		}
		if detail != "" {
			s.Detail = &detail
		}
		check.Steps = append(check.Steps, s)
		return err == nil
	}

	src, err := r.source(server)
	if !add(api.ServerCheckStep{Name: "config"}, err, "") {
		return check, nil
	}
	raw, _ := baseURL(server)
	var plain error
	if base, err := url.Parse(raw); err == nil && base.Scheme != "https" {
		plain = errors.New("plain HTTP: the token and the logs travel unencrypted")
	}
	add(api.ServerCheckStep{Name: "tls", Optional: &optional}, plain, "")

	status, err := src.Status(ctx)
	detail := ""
	if err == nil {
		state := "logging off"
		if status.Enabled {
			state = "logging on"
		}
		detail = fmt.Sprintf("library %s, %s", status.LibraryVersion, state)
	}
	if !add(api.ServerCheckStep{Name: "library"}, err, detail) {
		return check, nil
	}
	var writable error
	if !status.LogsDirWritable {
		writable = errors.New("the library cannot write to its logs directory")
	}
	add(api.ServerCheckStep{Name: "logs_dir_writable"}, writable, "")
	if status.PhpVersion != nil {
		add(api.ServerCheckStep{Name: "php", Optional: &optional}, nil, "PHP "+*status.PhpVersion)
	}
	return check, nil
}
