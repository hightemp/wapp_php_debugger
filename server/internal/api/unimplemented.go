// Stubs follow StrictServerInterface; a stub is replaced once its operation is implemented.

package api

import "context"

// Operations that are not implemented yet: 501 as Problem Details.

// CheckServer is not implemented yet.
func (s *server) CheckServer(context.Context, CheckServerRequestObject) (CheckServerResponseObject, error) {
	return CheckServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ClearProjectLogs is not implemented yet.
func (s *server) ClearProjectLogs(context.Context, ClearProjectLogsRequestObject) (ClearProjectLogsResponseObject, error) {
	return ClearProjectLogsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// CreateProject is not implemented yet.
func (s *server) CreateProject(context.Context, CreateProjectRequestObject) (CreateProjectResponseObject, error) {
	return CreateProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// CreateServer is not implemented yet.
func (s *server) CreateServer(context.Context, CreateServerRequestObject) (CreateServerResponseObject, error) {
	return CreateServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteProject is not implemented yet.
func (s *server) DeleteProject(context.Context, DeleteProjectRequestObject) (DeleteProjectResponseObject, error) {
	return DeleteProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteRequest is not implemented yet.
func (s *server) DeleteRequest(context.Context, DeleteRequestRequestObject) (DeleteRequestResponseObject, error) {
	return DeleteRequestdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteServer is not implemented yet.
func (s *server) DeleteServer(context.Context, DeleteServerRequestObject) (DeleteServerResponseObject, error) {
	return DeleteServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetDebuggerState is not implemented yet.
func (s *server) GetDebuggerState(context.Context, GetDebuggerStateRequestObject) (GetDebuggerStateResponseObject, error) {
	return GetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetProject is not implemented yet.
func (s *server) GetProject(context.Context, GetProjectRequestObject) (GetProjectResponseObject, error) {
	return GetProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetServer is not implemented yet.
func (s *server) GetServer(context.Context, GetServerRequestObject) (GetServerResponseObject, error) {
	return GetServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetSettings is not implemented yet.
func (s *server) GetSettings(context.Context, GetSettingsRequestObject) (GetSettingsResponseObject, error) {
	return GetSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ListProjects is not implemented yet.
func (s *server) ListProjects(context.Context, ListProjectsRequestObject) (ListProjectsResponseObject, error) {
	return ListProjectsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ListServers is not implemented yet.
func (s *server) ListServers(context.Context, ListServersRequestObject) (ListServersResponseObject, error) {
	return ListServersdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// SetDebuggerState is not implemented yet.
func (s *server) SetDebuggerState(context.Context, SetDebuggerStateRequestObject) (SetDebuggerStateResponseObject, error) {
	return SetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// StreamEvents is not implemented yet.
func (s *server) StreamEvents(context.Context, StreamEventsRequestObject) (StreamEventsResponseObject, error) {
	return StreamEventsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateProject is not implemented yet.
func (s *server) UpdateProject(context.Context, UpdateProjectRequestObject) (UpdateProjectResponseObject, error) {
	return UpdateProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateServer is not implemented yet.
func (s *server) UpdateServer(context.Context, UpdateServerRequestObject) (UpdateServerResponseObject, error) {
	return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateSettings is not implemented yet.
func (s *server) UpdateSettings(context.Context, UpdateSettingsRequestObject) (UpdateSettingsResponseObject, error) {
	return UpdateSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}
