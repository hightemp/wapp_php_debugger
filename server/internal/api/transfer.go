package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

// importLimit bounds an imported stream: the ingest takes messages up to 1 MB, thousands of them.
const importLimit = 64 << 20

// ExportRequest gives the request as the v2 stream the library would have written.
func (s *server) ExportRequest(ctx context.Context, request ExportRequestRequestObject) (ExportRequestResponseObject, error) {
	data, key, err := s.deps.Store.ExportRequest(ctx, request.RequestId)
	if errors.Is(err, store.ErrNotFound) {
		return ExportRequestdefaultApplicationProblemPlusJSONResponse{Body: notFound("request"), StatusCode: http.StatusNotFound}, nil
	}
	if err != nil {
		return nil, err
	}
	return ExportRequest200ApplicationxNdjsonResponse{
		Body:          bytes.NewReader(data),
		ContentLength: int64(len(data)),
		Headers:       ExportRequest200ResponseHeaders{ContentDisposition: fmt.Sprintf(`attachment; filename="request-%s.jsonl"`, safeFileName(key))},
	}, nil
}

// ImportRequest adds a request from a v2 stream; one the project has already is pointed to.
func (s *server) ImportRequest(ctx context.Context, request ImportRequestRequestObject) (ImportRequestResponseObject, error) {
	if _, found, err := s.deps.Store.GetProject(ctx, request.ProjectId); err != nil {
		return nil, err
	} else if !found {
		return ImportRequestdefaultApplicationProblemPlusJSONResponse{Body: notFound("project"), StatusCode: http.StatusNotFound}, nil
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, importLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > importLimit {
		return ImportRequestdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem("the stream is larger than 64 MB"), StatusCode: http.StatusBadRequest}, nil
	}
	id, err := s.deps.Store.ImportRequest(ctx, request.ProjectId, data)
	var exists *store.ExistsError
	if errors.As(err, &exists) {
		return ImportRequest200JSONResponse{RequestId: exists.RequestID, Created: false}, nil
	}
	if err != nil {
		return ImportRequestdefaultApplicationProblemPlusJSONResponse{Body: badRequestProblem(err.Error()), StatusCode: http.StatusBadRequest}, nil
	}
	if s.deps.Broker != nil {
		if row, found, err := s.deps.Store.GetRequest(ctx, id); err == nil && found {
			dto := RequestDTO(row)
			s.deps.Broker.Publish(string(RequestCreated), StreamEvent{Event: RequestCreated, Request: &dto})
		}
	}
	return ImportRequest200JSONResponse{RequestId: id, Created: true}, nil
}

// safeFileName keeps the characters of a ULID or a v1 file name that are safe in a header.
func safeFileName(key string) string {
	out := make([]rune, 0, len(key))
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
