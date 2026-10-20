package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/hightemp/wapp_php_debugger/server/internal/pathmap"
	"github.com/hightemp/wapp_php_debugger/server/internal/source"
)

// sourceLimit is the largest file the preview reads.
const sourceLimit = 1 << 20

// GetMessageSource returns lines of the file around the place of a message or one of its frames.
// The path comes from the log; the file is read only inside the project root.
func (s *server) GetMessageSource(ctx context.Context, request GetMessageSourceRequestObject) (GetMessageSourceResponseObject, error) {
	fail := func(status int, detail string) (GetMessageSourceResponseObject, error) {
		return GetMessageSourcedefaultApplicationProblemPlusJSONResponse{Body: problem(status, http.StatusText(status), detail), StatusCode: status}, nil
	}
	around := 12
	if request.Params.Around != nil {
		around = *request.Params.Around
	}
	if around < 1 || around > 50 {
		return fail(http.StatusBadRequest, "around is between 1 and 50")
	}
	detail, found, err := s.deps.Store.GetMessage(ctx, request.MessageId)
	if err != nil {
		return nil, err
	}
	if !found {
		return fail(http.StatusNotFound, "no such message")
	}
	file, line := "", 0
	switch {
	case request.Params.Frame != nil:
		if *request.Params.Frame < 0 || *request.Params.Frame >= len(detail.Trace) {
			return fail(http.StatusNotFound, "no such frame")
		}
		frame := detail.Trace[*request.Params.Frame]
		file, line = frame.File, frame.Line
	case detail.Source != nil:
		file, line = detail.Source.File, detail.Source.Line
	}
	if file == "" {
		return fail(http.StatusNotFound, "the message has no place in the code")
	}
	project, found, err := s.deps.Store.GetProject(ctx, detail.ProjectID)
	if err != nil {
		return nil, err
	}
	if !found || s.deps.Projects == nil {
		return fail(http.StatusNotFound, "no such project")
	}
	src, running := s.deps.Projects.Source(project.ID)
	files, readable := src.(source.ProjectFiles)
	if !running || !readable {
		return fail(http.StatusNotFound, "the code preview needs a project on this machine or over SSH")
	}

	data, err := files.ReadProjectFile(ctx, project.RootPath, absolutePath(project.RootPath, file), sourceLimit)
	switch {
	case errors.Is(err, source.ErrOutsideRoot):
		return fail(http.StatusForbidden, "the file is outside the project root")
	case errors.Is(err, source.ErrNotFound):
		return fail(http.StatusNotFound, "the file is not there: "+file)
	case errors.Is(err, source.ErrTooLarge):
		return fail(http.StatusRequestEntityTooLarge, "the file is over 1 MB")
	case err != nil:
		return fail(http.StatusBadGateway, err.Error())
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return fail(http.StatusUnsupportedMediaType, "the file is not text")
	}

	all := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), "\n")
	first := max(1, line-around)
	last := min(len(all), line+around)
	lines := []string{}
	if first <= last {
		lines = all[first-1 : last]
	}
	links, _, err := s.deps.Store.ProjectLinks(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	mapping := linksProject(links)
	return GetMessageSource200JSONResponse{File: pathmap.DisplayPath(mapping, file), Line: line, FirstLine: first, Lines: lines,
		EditorUrl: optional(pathmap.EditorURL(mapping, file, line))}, nil
}

// absolutePath puts a path of the log (relative to the root in v2, absolute in v1) on the server.
func absolutePath(root, file string) string {
	if path.IsAbs(file) || filepath.IsAbs(file) {
		return file
	}
	if path.IsAbs(root) {
		return path.Join(root, file)
	}
	return filepath.Join(root, file)
}
