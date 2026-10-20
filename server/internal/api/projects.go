package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/sshconn"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func forbidden(detail string) Problem {
	return problem(http.StatusForbidden, "Forbidden", detail)
}

func badRequestProblem(detail string) Problem {
	return problem(http.StatusBadRequest, "Bad Request", detail)
}

// storeProblem maps store errors to Problem Details; ok=false means the error is unexpected.
func storeProblem(err error, what string) (Problem, bool) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return notFound(what), true
	case errors.Is(err, store.ErrConflict):
		return problem(http.StatusConflict, "Conflict", "a project with this name or logs directory already exists on the server"), true
	default:
		return Problem{}, false
	}
}

func (s *server) serverDTO(srv store.Server) (Server, error) {
	var config ServerConfig
	if err := json.Unmarshal([]byte(srv.Config), &config); err != nil {
		return Server{}, err
	}
	connection := ServerConnection{State: Online} // the local server is always reachable
	if srv.Kind != "local" {
		connection = ServerConnection{State: Unknown}
		var status sshconn.Status
		switch {
		case srv.Kind == "ssh" && s.deps.SSH != nil:
			status = s.deps.SSH.Status(srv.ID)
		case srv.Kind == "http" && s.deps.HTTP != nil:
			status = s.deps.HTTP.Status(srv.ID)
		}
		if status.State != "" {
			connection.State = ConnectionState(status.State)
			if status.Error != "" {
				connection.Error = &status.Error
			}
			if !status.Since.IsZero() {
				checkedAt := status.Since.UnixMilli()
				connection.CheckedAt = &checkedAt
			}
		}
	}
	return Server{
		Id: srv.ID, Name: srv.Name, Kind: ServerKind(srv.Kind), Environment: Environment(srv.Environment), Config: config,
		Permissions: ServerPermissions{ReadOnly: srv.ReadOnly, AllowDelete: srv.AllowDelete, AllowControl: srv.AllowControl},
		Connection:  connection, CreatedAt: srv.CreatedAt, UpdatedAt: srv.UpdatedAt,
	}, nil
}

// ListServers returns all servers.
func (s *server) ListServers(ctx context.Context, _ ListServersRequestObject) (ListServersResponseObject, error) {
	servers, err := s.deps.Store.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	response := ListServers200JSONResponse{Items: make([]Server, 0, len(servers))}
	for _, row := range servers {
		dto, err := s.serverDTO(row)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, dto)
	}
	return response, nil
}

// GetServer returns a server.
func (s *server) GetServer(ctx context.Context, request GetServerRequestObject) (GetServerResponseObject, error) {
	row, found, err := s.deps.Store.GetServer(ctx, request.ServerId)
	if err != nil {
		return nil, err
	}
	if !found {
		return GetServerdefaultApplicationProblemPlusJSONResponse{Body: notFound("server"), StatusCode: http.StatusNotFound}, nil
	}
	dto, err := s.serverDTO(row)
	if err != nil {
		return nil, err
	}
	return GetServer200JSONResponse(dto), nil
}

// UpdateServer changes the name, environment, settings or permissions of a server.
func (s *server) UpdateServer(ctx context.Context, request UpdateServerRequestObject) (UpdateServerResponseObject, error) {
	body := request.Body
	patch := store.ServerPatch{Name: body.Name}
	if body.Name != nil && strings.TrimSpace(*body.Name) == "" {
		return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem("name must not be empty"), StatusCode: http.StatusBadRequest}, nil
	}
	if body.Environment != nil {
		environment := string(*body.Environment)
		patch.Environment = &environment
	}
	if body.Config != nil {
		encoded, err := json.Marshal(body.Config)
		if err != nil {
			return nil, err
		}
		config := string(encoded)
		patch.Config = &config
	}
	if p := body.Permissions; p != nil {
		patch.ReadOnly, patch.AllowDelete, patch.AllowControl = &p.ReadOnly, &p.AllowDelete, &p.AllowControl
	}
	if body.Token != nil && len(*body.Token) < minTokenLength {
		return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem(tokenRule), StatusCode: http.StatusBadRequest}, nil
	}
	updated, err := s.deps.Store.UpdateServer(ctx, request.ServerId, patch)
	if failure, ok := storeProblem(err, "server"); ok {
		return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: failure, StatusCode: failure.Status}, nil
	}
	if err != nil {
		return nil, err
	}
	if body.Token != nil {
		if updated.Kind != "http" || s.deps.HTTP == nil {
			return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem("only http servers have a token"), StatusCode: http.StatusBadRequest}, nil
		}
		if err := s.deps.HTTP.SetToken(updated.ID, *body.Token); err != nil {
			return nil, err
		}
	}
	if updated.Kind != "local" && (body.Config != nil || body.Token != nil) {
		// New connection settings: a fresh connection, and the projects pick up the new target
		if s.deps.SSH != nil && updated.Kind == "ssh" {
			s.deps.SSH.Forget(updated.ID)
		}
		if err := s.restartProjects(ctx, updated.ID); err != nil {
			return nil, err
		}
	}
	dto, err := s.serverDTO(updated)
	if err != nil {
		return nil, err
	}
	return UpdateServer200JSONResponse(dto), nil
}

func (s *server) projectDTO(ctx context.Context, p store.Project) (Project, error) {
	pending, err := s.deps.Store.PendingFiles(ctx, p.ID)
	if err != nil {
		return Project{}, err
	}
	return Project{
		Id: p.ID, ServerId: p.ServerID, Name: p.Name, RootPath: p.RootPath, LogsDir: p.LogsDir, LocalPath: optional(p.LocalPath),
		ConfigPath: p.ConfigPath, Editor: Editor(p.Editor),
		Retention:    Retention{Days: p.RetentionDays, MaxBytes: p.RetentionMaxBytes},
		PendingFiles: pending, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}, nil
}

// ListProjects returns projects, optionally of one server.
func (s *server) ListProjects(ctx context.Context, request ListProjectsRequestObject) (ListProjectsResponseObject, error) {
	var serverID int64
	if request.Params.ServerId != nil {
		serverID = *request.Params.ServerId
	}
	projects, err := s.deps.Store.ListProjects(ctx, serverID)
	if err != nil {
		return nil, err
	}
	response := ListProjects200JSONResponse{Items: make([]Project, 0, len(projects))}
	for _, project := range projects {
		dto, err := s.projectDTO(ctx, project)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, dto)
	}
	return response, nil
}

// GetProject returns a project.
func (s *server) GetProject(ctx context.Context, request GetProjectRequestObject) (GetProjectResponseObject, error) {
	project, found, err := s.deps.Store.GetProject(ctx, request.ProjectId)
	if err != nil {
		return nil, err
	}
	if !found {
		return GetProjectdefaultApplicationProblemPlusJSONResponse{Body: notFound("project"), StatusCode: http.StatusNotFound}, nil
	}
	dto, err := s.projectDTO(ctx, project)
	if err != nil {
		return nil, err
	}
	return GetProject200JSONResponse(dto), nil
}

var editors = map[Editor]bool{EditorVscode: true, EditorPhpstorm: true, EditorCursor: true, EditorSublime: true, EditorNone: true}

// validatePaths checks the fields shared by create and update; empty means valid.
func validateProjectFields(name, rootPath, logsDir *string, editor *Editor) string {
	switch {
	case name != nil && strings.TrimSpace(*name) == "":
		return "name must not be empty"
	case rootPath != nil && !path.IsAbs(*rootPath):
		return "root_path must be an absolute path"
	case logsDir != nil && !path.IsAbs(*logsDir):
		return "logs_dir must be an absolute path"
	case editor != nil && !editors[*editor]:
		return "unknown editor " + string(*editor)
	}
	return ""
}

// CreateProject adds a project to a local server and starts its import.
func (s *server) CreateProject(ctx context.Context, request CreateProjectRequestObject) (CreateProjectResponseObject, error) {
	fail := func(p Problem) (CreateProjectResponseObject, error) {
		return CreateProjectdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: p.Status}, nil
	}
	body := request.Body
	if invalid := validateProjectFields(&body.Name, &body.RootPath, &body.LogsDir, body.Editor); invalid != "" {
		return fail(badRequestProblem(invalid))
	}
	serverRow, found, err := s.deps.Store.GetServer(ctx, body.ServerId)
	if err != nil {
		return nil, err
	}
	if !found {
		return fail(notFound("server"))
	}
	if serverRow.Kind == "http" {
		// The base URL is one application: the library there has one logs directory
		existing, err := s.deps.Store.ListProjects(ctx, serverRow.ID)
		if err != nil {
			return nil, err
		}
		if len(existing) > 0 {
			return fail(problem(http.StatusConflict, "Conflict", "an http server is one application and serves one project: add another server for another base URL"))
		}
	}

	input := store.ProjectInput{ServerID: body.ServerId, Name: strings.TrimSpace(body.Name), RootPath: body.RootPath, LogsDir: body.LogsDir}
	if body.LocalPath != nil {
		input.LocalPath = *body.LocalPath
	}
	if body.ConfigPath != nil {
		input.ConfigPath = *body.ConfigPath
	}
	if body.Editor != nil {
		input.Editor = string(*body.Editor)
	}
	if body.Retention != nil {
		input.RetentionDays, input.RetentionMaxBytes = body.Retention.Days, body.Retention.MaxBytes
	}
	project, err := s.deps.Store.CreateProject(ctx, input)
	if failure, ok := storeProblem(err, "server"); ok {
		return fail(failure)
	}
	if err != nil {
		return nil, err
	}
	s.deps.Projects.Add(project)
	dto, err := s.projectDTO(ctx, project)
	if err != nil {
		return nil, err
	}
	return CreateProject201JSONResponse(dto), nil
}

// UpdateProject changes a project and restarts its import with the new settings.
func (s *server) UpdateProject(ctx context.Context, request UpdateProjectRequestObject) (UpdateProjectResponseObject, error) {
	fail := func(p Problem) (UpdateProjectResponseObject, error) {
		return UpdateProjectdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: p.Status}, nil
	}
	body := request.Body
	if invalid := validateProjectFields(body.Name, body.RootPath, body.LogsDir, body.Editor); invalid != "" {
		return fail(badRequestProblem(invalid))
	}
	patch := store.ProjectPatch{Name: body.Name, RootPath: body.RootPath, LogsDir: body.LogsDir, LocalPath: body.LocalPath, ConfigPath: body.ConfigPath}
	if body.Editor != nil {
		editor := string(*body.Editor)
		patch.Editor = &editor
	}
	if body.Retention != nil {
		patch.RetentionDays, patch.RetentionMaxBytes = body.Retention.Days, body.Retention.MaxBytes
	}
	project, err := s.deps.Store.UpdateProject(ctx, request.ProjectId, patch)
	if failure, ok := storeProblem(err, "project"); ok {
		return fail(failure)
	}
	if err != nil {
		return nil, err
	}
	s.deps.Projects.Add(project)
	dto, err := s.projectDTO(ctx, project)
	if err != nil {
		return nil, err
	}
	return UpdateProject200JSONResponse(dto), nil
}

// DeleteProject stops the import and removes the project data from the viewer (log files stay).
func (s *server) DeleteProject(ctx context.Context, request DeleteProjectRequestObject) (DeleteProjectResponseObject, error) {
	s.deps.Projects.Remove(request.ProjectId)
	err := s.deps.Store.DeleteProject(ctx, request.ProjectId)
	if failure, ok := storeProblem(err, "project"); ok {
		return DeleteProjectdefaultApplicationProblemPlusJSONResponse{Body: failure, StatusCode: failure.Status}, nil
	}
	if err != nil {
		return nil, err
	}
	return DeleteProject204Response{}, nil
}

// projectServer returns the project and the permissions of its server.
func (s *server) projectServer(ctx context.Context, projectID int64) (store.Project, store.Server, *Problem, error) {
	project, found, err := s.deps.Store.GetProject(ctx, projectID)
	if err != nil || !found {
		failure := notFound("project")
		return project, store.Server{}, &failure, err
	}
	serverRow, _, err := s.deps.Store.GetServer(ctx, project.ServerID)
	return project, serverRow, nil, err
}

func notRunning() Problem {
	return problem(http.StatusConflict, "Conflict", "the import of this project is not running")
}

// GetDebuggerState reads the debugger flag from the project config.
func (s *server) GetDebuggerState(ctx context.Context, request GetDebuggerStateRequestObject) (GetDebuggerStateResponseObject, error) {
	fail := func(p Problem) (GetDebuggerStateResponseObject, error) {
		return GetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: p.Status}, nil
	}
	if _, _, failure, err := s.projectServer(ctx, request.ProjectId); err != nil || failure != nil {
		if err != nil {
			return nil, err
		}
		return fail(*failure)
	}
	src, ok := s.deps.Projects.Source(request.ProjectId)
	if !ok {
		return fail(notRunning())
	}
	enabled, err := src.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	return GetDebuggerState200JSONResponse{Enabled: enabled}, nil
}

// SetDebuggerState switches the debugger in the project config.
func (s *server) SetDebuggerState(ctx context.Context, request SetDebuggerStateRequestObject) (SetDebuggerStateResponseObject, error) {
	fail := func(p Problem) (SetDebuggerStateResponseObject, error) {
		return SetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: p.Status}, nil
	}
	_, serverRow, failure, err := s.projectServer(ctx, request.ProjectId)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return fail(*failure)
	}
	if serverRow.ReadOnly || !serverRow.AllowControl {
		return fail(forbidden("the server does not allow switching the debugger"))
	}
	src, ok := s.deps.Projects.Source(request.ProjectId)
	if !ok {
		return fail(notRunning())
	}
	if err := src.SetEnabled(ctx, request.Body.Enabled); err != nil {
		return nil, err
	}
	return SetDebuggerState200JSONResponse{Enabled: request.Body.Enabled}, nil
}

// ClearProjectLogs deletes the project log files and their data in the viewer.
func (s *server) ClearProjectLogs(ctx context.Context, request ClearProjectLogsRequestObject) (ClearProjectLogsResponseObject, error) {
	fail := func(p Problem) (ClearProjectLogsResponseObject, error) {
		return ClearProjectLogsdefaultApplicationProblemPlusJSONResponse{Body: p, StatusCode: p.Status}, nil
	}
	project, serverRow, failure, err := s.projectServer(ctx, request.ProjectId)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return fail(*failure)
	}
	if serverRow.ReadOnly || !serverRow.AllowDelete {
		return fail(forbidden("the server does not allow deleting logs"))
	}
	src, ok := s.deps.Projects.Source(request.ProjectId)
	if !ok {
		return fail(notRunning())
	}
	files, err := src.List(ctx)
	if err != nil {
		return nil, err
	}
	removed := 0
	for _, file := range files {
		if err := src.Remove(ctx, file.Name); err != nil {
			return nil, err
		}
		if err := s.deps.Store.ForgetFile(ctx, project.ID, file.Name); err != nil {
			return nil, err
		}
		removed++
	}
	return ClearProjectLogs200JSONResponse{RemovedFiles: removed}, nil
}
