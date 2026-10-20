package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/hightemp/wapp_php_debugger/server/internal/logformat"
	"github.com/hightemp/wapp_php_debugger/server/internal/pathmap"
	"github.com/hightemp/wapp_php_debugger/server/internal/store"
)

func notFound(what string) Problem {
	return problem(http.StatusNotFound, "Not Found", what+" not found")
}

// GetRequest returns a request with its child requests.
func (s *server) GetRequest(ctx context.Context, request GetRequestRequestObject) (GetRequestResponseObject, error) {
	detail, found, err := s.deps.Store.GetRequestDetail(ctx, request.RequestId)
	if err != nil {
		return nil, err
	}
	if !found {
		return GetRequestdefaultApplicationProblemPlusJSONResponse{Body: notFound("request"), StatusCode: http.StatusNotFound}, nil
	}

	base := RequestDTO(detail.RequestRow)
	children := make([]Request, 0, len(detail.Children))
	for _, child := range detail.Children {
		children = append(children, RequestDTO(child))
	}
	lang := RequestDetailLang(detail.Lang)
	response := GetRequest200JSONResponse{
		Id: base.Id, ProjectId: base.ProjectId, ServerId: base.ServerId, RequestKey: base.RequestKey,
		ParentRequestKey: base.ParentRequestKey, FormatVersion: RequestDetailFormatVersion(base.FormatVersion), Sapi: base.Sapi,
		Method: base.Method, Url: base.Url, IsAjax: base.IsAjax, StatusCode: base.StatusCode, ExitCode: base.ExitCode,
		StartedAt: base.StartedAt, DurationMs: base.DurationMs, Finished: base.Finished, MessageCount: base.MessageCount,
		ErrorCount: base.ErrorCount, WarningCount: base.WarningCount, ReceivedAt: base.ReceivedAt, Pinned: base.Pinned, Note: base.Note,
		Lang: &lang, Ip: optional(detail.IP), ServerIp: optional(detail.ServerIP), ServerName: optional(detail.ServerName),
		Hostname: optional(detail.Hostname), Pid: detail.PID, ProcessUser: optional(detail.ProcessUser), AppUser: optional(detail.AppUser),
		SessionId: optional(detail.SessionID), LibraryVersion: optional(detail.LibraryVersion), PhpVersion: optional(detail.PHPVersion),
		EndedAt: detail.EndedAt, MemoryPeakBytes: detail.MemoryPeakBytes, Children: &children,
	}
	if detail.Argv != nil {
		response.Argv = &detail.Argv
	}
	return response, nil
}

var levels = map[Level]bool{LevelError: true, LevelWarning: true, LevelInfo: true, LevelMessage: true, LevelDebug: true}

// ListMessages returns the messages of a request in seq order.
func (s *server) ListMessages(ctx context.Context, request ListMessagesRequestObject) (ListMessagesResponseObject, error) {
	bad := func(detail string) (ListMessagesResponseObject, error) {
		return ListMessagesdefaultApplicationProblemPlusJSONResponse{Body: problem(http.StatusBadRequest, "Bad Request", detail), StatusCode: http.StatusBadRequest}, nil
	}
	params := request.Params
	filter := store.MessageFilter{RequestID: request.RequestId, Limit: defaultLimit}
	if params.Limit != nil {
		filter.Limit = *params.Limit
	}
	if filter.Limit < 1 || filter.Limit > maxLimit {
		return bad("limit must be between 1 and 500")
	}
	if params.Level != nil {
		for _, level := range *params.Level {
			if !levels[level] {
				return bad("unknown level " + string(level))
			}
			filter.Levels = append(filter.Levels, string(level))
		}
	}
	if params.Type != nil {
		filter.Type = string(*params.Type)
	}
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}
	if params.Q != nil {
		filter.Query = *params.Q
	}
	if params.Cursor != nil {
		filter.Cursor = *params.Cursor
	}

	if _, found, err := s.deps.Store.GetRequest(ctx, request.RequestId); err != nil || !found {
		if err != nil {
			return nil, err
		}
		return ListMessagesdefaultApplicationProblemPlusJSONResponse{Body: notFound("request"), StatusCode: http.StatusNotFound}, nil
	}
	page, err := s.deps.Store.ListMessages(ctx, filter)
	if errors.Is(err, store.ErrInvalidCursor) {
		return bad(err.Error())
	}
	if err != nil {
		return nil, err
	}
	links, err := s.projectLinksForRequest(ctx, request.RequestId)
	if err != nil {
		return nil, err
	}

	response := ListMessages200JSONResponse{Items: make([]Message, 0, len(page.Items))}
	for _, row := range page.Items {
		response.Items = append(response.Items, messageDTO(row, links))
	}
	if page.NextCursor != "" {
		response.NextCursor = &page.NextCursor
	}
	return response, nil
}

// GetMessage returns a message with context, backtrace and error details.
func (s *server) GetMessage(ctx context.Context, request GetMessageRequestObject) (GetMessageResponseObject, error) {
	detail, found, err := s.deps.Store.GetMessage(ctx, request.MessageId)
	if err != nil {
		return nil, err
	}
	if !found {
		return GetMessagedefaultApplicationProblemPlusJSONResponse{Body: notFound("message"), StatusCode: http.StatusNotFound}, nil
	}
	links, _, err := s.deps.Store.ProjectLinks(ctx, detail.ProjectID)
	if err != nil {
		return nil, err
	}
	project := linksProject(links)

	base := messageDTO(detail.MessageRow, project)
	response := GetMessage200JSONResponse{
		Id: base.Id, RequestId: base.RequestId, Seq: base.Seq, Ts: base.Ts, Type: base.Type, Level: base.Level,
		Message: base.Message, Tags: base.Tags, Source: base.Source, DurationMs: base.DurationMs, HasContext: base.HasContext,
		SizeBytes: base.SizeBytes, ErrorType: optional(detail.ErrorType), ExceptionClass: optional(detail.ExceptionClass),
		Code: optional(detail.Code),
	}
	if base.ErrorKind != nil {
		kind := MessageDetailErrorKind(*base.ErrorKind)
		response.ErrorKind = &kind
	}
	if detail.Source != nil && response.Source != nil && detail.Source.CallType != "" {
		callType := FrameCallType(detail.Source.CallType)
		response.Source.CallType = &callType
	}
	if detail.Context != nil {
		var context map[string]interface{}
		if err := json.Unmarshal(detail.Context, &context); err != nil {
			return nil, err
		}
		response.Context = &context
	}
	if detail.Trace != nil {
		trace := make([]Frame, 0, len(detail.Trace))
		for _, frame := range detail.Trace {
			trace = append(trace, frameDTO(frame, project))
		}
		response.Trace = &trace
	}
	if detail.Previous != nil {
		if err := json.Unmarshal(detail.Previous, &response.Previous); err != nil {
			return nil, err
		}
		for i := range *response.Previous {
			if source := (*response.Previous)[i].Source; source != nil && source.File != nil {
				line := 0
				if source.Line != nil {
					line = *source.Line
				}
				*source = frameDTO(logformat.Frame{File: *source.File, Line: line}, project)
			}
		}
	}
	return response, nil
}

func (s *server) projectLinksForRequest(ctx context.Context, requestID int64) (pathmap.Project, error) {
	row, _, err := s.deps.Store.GetRequest(ctx, requestID)
	if err != nil {
		return pathmap.Project{}, err
	}
	links, _, err := s.deps.Store.ProjectLinks(ctx, row.ProjectID)
	return linksProject(links), err
}

func linksProject(links store.ProjectLinks) pathmap.Project {
	return pathmap.Project{Editor: links.Editor, RootPath: links.RootPath, LocalPath: links.LocalPath, RemoteHost: links.RemoteHost}
}

func messageDTO(row store.MessageRow, project pathmap.Project) Message {
	m := Message{
		Id: row.ID, RequestId: row.RequestID, Seq: row.Seq, Ts: row.TS, Type: MessageType(row.Type), Level: Level(row.Level),
		Message: row.Message, Tags: row.Tags, DurationMs: row.DurationMs, HasContext: row.HasContext, SizeBytes: row.SizeBytes,
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	if row.ErrorKind != "" {
		kind := MessageErrorKind(row.ErrorKind)
		m.ErrorKind = &kind
	}
	if row.Source != nil {
		source := frameDTO(*row.Source, project)
		m.Source = &source
	}
	return m
}

// frameDTO shows paths relative to the project root and adds the IDE link.
func frameDTO(frame logformat.Frame, project pathmap.Project) Frame {
	dto := Frame{
		File:      optional(pathmap.DisplayPath(project, frame.File)),
		Function:  optional(frame.Function),
		Class:     optional(frame.Class),
		EditorUrl: optional(pathmap.EditorURL(project, frame.File, frame.Line)),
	}
	if frame.Line > 0 {
		dto.Line = &frame.Line
	}
	if frame.CallType != "" {
		callType := FrameCallType(frame.CallType)
		dto.CallType = &callType
	}
	return dto
}
