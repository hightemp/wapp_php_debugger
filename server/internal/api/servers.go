package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

var fingerprintPattern = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]+=*$`)

func (s *server) CreateServer(ctx context.Context, request CreateServerRequestObject) (CreateServerResponseObject, error) {
	body := request.Body
	fail := func(detail string) (CreateServerResponseObject, error) {
		return CreateServerdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem(detail), StatusCode: http.StatusBadRequest}, nil
	}
	if strings.TrimSpace(body.Name) == "" {
		return fail("name must not be empty")
	}
	switch body.Kind {
	case ServerKindLocal:
		return fail("there is one local server and it exists already")
	case ServerKindSsh:
		if body.Config == nil || body.Config.Host == nil || strings.TrimSpace(*body.Config.Host) == "" {
			return fail("an ssh server needs config.host: a host name or an alias from ~/.ssh/config")
		}
	default:
		return fail("servers of kind " + string(body.Kind) + " are not supported yet")
	}

	input := store.ServerInput{Name: strings.TrimSpace(body.Name), Kind: string(body.Kind)}
	if body.Environment != nil {
		input.Environment = string(*body.Environment)
	}
	if body.Config != nil {
		encoded, err := json.Marshal(body.Config)
		if err != nil {
			return nil, err
		}
		input.Config = string(encoded)
	}
	if p := body.Permissions; p != nil {
		input.Permissions = &store.Permissions{ReadOnly: p.ReadOnly, AllowDelete: p.AllowDelete, AllowControl: p.AllowControl}
	}
	created, err := s.deps.Store.CreateServer(ctx, input)
	if failure, ok := storeProblem(err, "server"); ok {
		return CreateServerdefaultApplicationProblemPlusJSONResponse{Body: failure, StatusCode: failure.Status}, nil
	}
	if err != nil {
		return nil, err
	}
	dto, err := s.serverDTO(created)
	if err != nil {
		return nil, err
	}
	return CreateServer201JSONResponse(dto), nil
}

func (s *server) DeleteServer(ctx context.Context, request DeleteServerRequestObject) (DeleteServerResponseObject, error) {
	srv, found, err := s.deps.Store.GetServer(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	if !found {
		return DeleteServerdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusNotFound, "Not Found", "no such server"), StatusCode: http.StatusNotFound}, nil
	}
	if srv.Kind == "local" {
		return DeleteServerdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusConflict, "Conflict", "the local server cannot be removed"), StatusCode: http.StatusConflict}, nil
	}
	projects, err := s.deps.Store.ListProjects(ctx, srv.ID)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if s.deps.Projects != nil {
			s.deps.Projects.Remove(project.ID)
		}
	}
	if s.deps.SSH != nil {
		s.deps.SSH.Forget(srv.ID)
	}
	if err := s.deps.Store.DeleteServer(ctx, srv.ID); err != nil {
		return nil, err
	}
	return DeleteServer204Response{}, nil
}

func (s *server) CheckServer(ctx context.Context, request CheckServerRequestObject) (CheckServerResponseObject, error) {
	srv, found, err := s.deps.Store.GetServer(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	if !found {
		return CheckServerdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusNotFound, "Not Found", "no such server"), StatusCode: http.StatusNotFound}, nil
	}
	switch {
	case srv.Kind == "local":
		return CheckServer200JSONResponse(ServerCheck{Ok: true, Steps: []ServerCheckStep{{Name: "local", Ok: true}}}), nil
	case srv.Kind == "ssh" && s.deps.SSH != nil:
		projects, err := s.deps.Store.ListProjects(ctx, srv.ID)
		if err != nil {
			return nil, err
		}
		check, err := s.deps.SSH.Check(ctx, srv, projects)
		if err != nil {
			return nil, err
		}
		return CheckServer200JSONResponse(check), nil
	default:
		return CheckServerdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusNotImplemented, "Not Implemented", "checks of "+srv.Kind+" servers are not supported yet"), StatusCode: http.StatusNotImplemented}, nil
	}
}

func (s *server) TrustHostKey(ctx context.Context, request TrustHostKeyRequestObject) (TrustHostKeyResponseObject, error) {
	fail := func(status int, title, detail string) (TrustHostKeyResponseObject, error) {
		return TrustHostKeydefaultApplicationProblemPlusJSONResponse{Body: problem(status, title, detail), StatusCode: status}, nil
	}
	if !fingerprintPattern.MatchString(request.Body.Fingerprint) {
		return fail(http.StatusBadRequest, "Bad Request", "fingerprint must look like SHA256:…, as checkServer shows it")
	}
	srv, found, err := s.deps.Store.GetServer(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	if !found {
		return fail(http.StatusNotFound, "Not Found", "no such server")
	}
	if srv.Kind != "ssh" || s.deps.SSH == nil {
		return fail(http.StatusBadRequest, "Bad Request", "only ssh servers have host keys")
	}
	err = s.deps.SSH.TrustHostKey(ctx, srv, request.Body.Fingerprint)
	if errors.Is(err, ErrHostKeyMismatch) || errors.Is(err, ErrHostKeyChanged) {
		return fail(http.StatusConflict, "Conflict", err.Error())
	}
	if err != nil {
		return fail(http.StatusBadGateway, "Bad Gateway", err.Error())
	}
	// The connection may be waiting out a back-off after the unknown host: start the projects again
	if err := s.restartProjects(ctx, srv.ID); err != nil {
		return nil, err
	}
	return TrustHostKey204Response{}, nil
}

// restartProjects restarts the import of every project of the server (new settings or a trusted key).
func (s *server) restartProjects(ctx context.Context, serverID int64) error {
	if s.deps.Projects == nil {
		return nil
	}
	projects, err := s.deps.Store.ListProjects(ctx, serverID)
	if err != nil {
		return err
	}
	for _, project := range projects {
		s.deps.Projects.Add(project)
	}
	return nil
}
