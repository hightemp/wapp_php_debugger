package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
	"github.com/hightemp/wapp_php_debugger/server/internal/version"
)

// server implements StrictServerInterface; operations that are not implemented yet live in unimplemented.go.
type server struct {
	deps Deps
}

var _ StrictServerInterface = (*server)(nil)

// GetVersion returns the build version.
func (s *server) GetVersion(context.Context, GetVersionRequestObject) (GetVersionResponseObject, error) {
	app := VersionAppServer
	if s.deps.Desktop != nil {
		app = VersionAppDesktop
	}
	return GetVersion200JSONResponse{App: app, Version: version.Version, Commit: version.Commit, Date: version.Date}, nil
}

// GetUpdates returns the last answer of the update check.
func (s *server) GetUpdates(context.Context, GetUpdatesRequestObject) (GetUpdatesResponseObject, error) {
	if s.deps.Updates == nil || !s.deps.Updates.Enabled() {
		return GetUpdates200JSONResponse{Enabled: false, Current: version.Version}, nil
	}
	last := s.deps.Updates.Last()
	response := GetUpdates200JSONResponse{Enabled: true, Current: last.Current, Newer: last.Newer, Latest: optional(last.Latest), Url: optional(last.URL)}
	if last.CheckedAt != 0 {
		response.CheckedAt = &last.CheckedAt
	}
	return response, nil
}

// GetDesktopSettings returns the settings of the desktop app.
func (s *server) GetDesktopSettings(ctx context.Context, _ GetDesktopSettingsRequestObject) (GetDesktopSettingsResponseObject, error) {
	if s.deps.Desktop == nil {
		return GetDesktopSettingsdefaultApplicationProblemPlusJSONResponse{Body: notFound("desktop app"), StatusCode: http.StatusNotFound}, nil
	}
	settings, err := s.deps.Desktop.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return GetDesktopSettings200JSONResponse(settings), nil
}

// UpdateDesktopSettings saves and applies the settings of the desktop app.
func (s *server) UpdateDesktopSettings(ctx context.Context, request UpdateDesktopSettingsRequestObject) (UpdateDesktopSettingsResponseObject, error) {
	if s.deps.Desktop == nil {
		return UpdateDesktopSettingsdefaultApplicationProblemPlusJSONResponse{Body: notFound("desktop app"), StatusCode: http.StatusNotFound}, nil
	}
	if !request.Body.Notifications.MinLevel.Valid() {
		return UpdateDesktopSettingsdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem("notifications.min_level is error or warning"), StatusCode: http.StatusBadRequest}, nil
	}
	settings, err := s.deps.Desktop.UpdateSettings(ctx, *request.Body)
	if err != nil {
		return nil, err
	}
	return UpdateDesktopSettings200JSONResponse(settings), nil
}

// statusRange reads 404 as [404, 404] and 5xx as [500, 599].
func statusRange(status string) (int, int, bool) {
	if len(status) != 3 || status[0] < '1' || status[0] > '5' {
		return 0, 0, false
	}
	if strings.HasSuffix(status, "xx") {
		base := int(status[0]-'0') * 100
		return base, base + 99, true
	}
	code, err := strconv.Atoi(status)
	return code, code, err == nil
}

const (
	defaultLimit = 100
	maxLimit     = 500
)

// ListRequests returns the request feed.
func (s *server) ListRequests(ctx context.Context, request ListRequestsRequestObject) (ListRequestsResponseObject, error) {
	params := request.Params
	limit := defaultLimit
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit < 1 || limit > maxLimit {
		return ListRequestsdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusBadRequest, "Bad Request", "limit must be between 1 and 500"), StatusCode: http.StatusBadRequest}, nil
	}
	filter := store.RequestFilter{Limit: limit}
	if params.ProjectId != nil {
		filter.ProjectID = *params.ProjectId
	}
	if params.ServerId != nil {
		filter.ServerID = *params.ServerId
	}
	if params.Environment != nil {
		filter.Environment = string(*params.Environment)
	}
	if params.Sapi != nil {
		filter.Sapi = string(*params.Sapi)
	}
	if params.HasErrors != nil {
		filter.HasErrors = *params.HasErrors
	}
	if params.Q != nil {
		filter.Query = *params.Q
	}
	if params.Since != nil {
		filter.Since = *params.Since
	}
	if params.Until != nil {
		filter.Until = *params.Until
	}
	if params.Cursor != nil {
		filter.Cursor = *params.Cursor
	}
	if params.Method != nil {
		filter.Method = *params.Method
	}
	if params.Status != nil {
		low, high, ok := statusRange(*params.Status)
		if !ok {
			return ListRequestsdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem("status is a code (404) or a class (5xx)"), StatusCode: http.StatusBadRequest}, nil
		}
		filter.StatusMin, filter.StatusMax = low, high
	}
	if params.MinDurationMs != nil {
		filter.MinDurationMs = float64(*params.MinDurationMs)
	}
	filter.Ajax = params.IsAjax
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}
	if params.Level != nil {
		filter.Level = string(*params.Level)
	}

	page, err := s.deps.Store.ListRequests(ctx, filter)
	if errors.Is(err, store.ErrInvalidCursor) {
		return ListRequestsdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusBadRequest, "Bad Request", err.Error()), StatusCode: http.StatusBadRequest}, nil
	}
	if err != nil {
		return nil, err
	}

	response := ListRequests200JSONResponse{Items: make([]Request, 0, len(page.Items))}
	for _, row := range page.Items {
		response.Items = append(response.Items, RequestDTO(row))
	}
	if page.NextCursor != "" {
		response.NextCursor = &page.NextCursor
	}
	return response, nil
}

// RequestDTO converts a feed row to the API DTO.
func RequestDTO(r store.RequestRow) Request {
	return Request{
		Id:               r.ID,
		ProjectId:        r.ProjectID,
		ServerId:         r.ServerID,
		RequestKey:       r.Key,
		ParentRequestKey: optional(r.ParentKey),
		FormatVersion:    RequestFormatVersion(r.FormatVersion),
		Sapi:             Sapi(r.Sapi),
		Method:           optional(r.Method),
		Url:              optional(r.URL),
		IsAjax:           r.IsAjax,
		StatusCode:       r.StatusCode,
		ExitCode:         r.ExitCode,
		StartedAt:        r.StartedAt,
		DurationMs:       r.DurationMs,
		Finished:         r.Finished,
		MessageCount:     r.MessageCount,
		ErrorCount:       r.ErrorCount,
		WarningCount:     r.WarningCount,
		ReceivedAt:       r.ReceivedAt,
	}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
