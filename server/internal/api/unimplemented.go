// Заглушки пишутся по списку StrictServerInterface; заглушка заменяется, когда операция реализована.

package api

import "context"

// Заглушки операций, которые ещё не реализованы: 501 в формате Problem Details.

// CheckServer ещё не реализована.
func (s *server) CheckServer(context.Context, CheckServerRequestObject) (CheckServerResponseObject, error) {
	return CheckServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ClearProjectLogs ещё не реализована.
func (s *server) ClearProjectLogs(context.Context, ClearProjectLogsRequestObject) (ClearProjectLogsResponseObject, error) {
	return ClearProjectLogsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// CreateProject ещё не реализована.
func (s *server) CreateProject(context.Context, CreateProjectRequestObject) (CreateProjectResponseObject, error) {
	return CreateProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// CreateServer ещё не реализована.
func (s *server) CreateServer(context.Context, CreateServerRequestObject) (CreateServerResponseObject, error) {
	return CreateServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteProject ещё не реализована.
func (s *server) DeleteProject(context.Context, DeleteProjectRequestObject) (DeleteProjectResponseObject, error) {
	return DeleteProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteRequest ещё не реализована.
func (s *server) DeleteRequest(context.Context, DeleteRequestRequestObject) (DeleteRequestResponseObject, error) {
	return DeleteRequestdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteServer ещё не реализована.
func (s *server) DeleteServer(context.Context, DeleteServerRequestObject) (DeleteServerResponseObject, error) {
	return DeleteServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetDebuggerState ещё не реализована.
func (s *server) GetDebuggerState(context.Context, GetDebuggerStateRequestObject) (GetDebuggerStateResponseObject, error) {
	return GetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetMessage ещё не реализована.
func (s *server) GetMessage(context.Context, GetMessageRequestObject) (GetMessageResponseObject, error) {
	return GetMessagedefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetProject ещё не реализована.
func (s *server) GetProject(context.Context, GetProjectRequestObject) (GetProjectResponseObject, error) {
	return GetProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetRequest ещё не реализована.
func (s *server) GetRequest(context.Context, GetRequestRequestObject) (GetRequestResponseObject, error) {
	return GetRequestdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetServer ещё не реализована.
func (s *server) GetServer(context.Context, GetServerRequestObject) (GetServerResponseObject, error) {
	return GetServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// GetSettings ещё не реализована.
func (s *server) GetSettings(context.Context, GetSettingsRequestObject) (GetSettingsResponseObject, error) {
	return GetSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ListMessages ещё не реализована.
func (s *server) ListMessages(context.Context, ListMessagesRequestObject) (ListMessagesResponseObject, error) {
	return ListMessagesdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ListProjects ещё не реализована.
func (s *server) ListProjects(context.Context, ListProjectsRequestObject) (ListProjectsResponseObject, error) {
	return ListProjectsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// ListServers ещё не реализована.
func (s *server) ListServers(context.Context, ListServersRequestObject) (ListServersResponseObject, error) {
	return ListServersdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// SetDebuggerState ещё не реализована.
func (s *server) SetDebuggerState(context.Context, SetDebuggerStateRequestObject) (SetDebuggerStateResponseObject, error) {
	return SetDebuggerStatedefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// StreamEvents ещё не реализована.
func (s *server) StreamEvents(context.Context, StreamEventsRequestObject) (StreamEventsResponseObject, error) {
	return StreamEventsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateProject ещё не реализована.
func (s *server) UpdateProject(context.Context, UpdateProjectRequestObject) (UpdateProjectResponseObject, error) {
	return UpdateProjectdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateServer ещё не реализована.
func (s *server) UpdateServer(context.Context, UpdateServerRequestObject) (UpdateServerResponseObject, error) {
	return UpdateServerdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateSettings ещё не реализована.
func (s *server) UpdateSettings(context.Context, UpdateSettingsRequestObject) (UpdateSettingsResponseObject, error) {
	return UpdateSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}
