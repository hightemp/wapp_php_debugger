package api

import (
	"context"
	"errors"
	"net/http"

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
	return GetVersion200JSONResponse{Version: version.Version, Commit: version.Commit, Date: version.Date}, nil
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
	if params.Cursor != nil {
		filter.Cursor = *params.Cursor
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
