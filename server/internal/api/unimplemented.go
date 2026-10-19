// Stubs follow StrictServerInterface; a stub is replaced once its operation is implemented.

package api

import "context"

// Operations that are not implemented yet: 501 as Problem Details.

// CheckServer is not implemented yet.
// CreateServer is not implemented yet.
// DeleteRequest is not implemented yet.
func (s *server) DeleteRequest(context.Context, DeleteRequestRequestObject) (DeleteRequestResponseObject, error) {
	return DeleteRequestdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// DeleteServer is not implemented yet.
// GetSettings is not implemented yet.
func (s *server) GetSettings(context.Context, GetSettingsRequestObject) (GetSettingsResponseObject, error) {
	return GetSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// StreamEvents is not implemented yet.
func (s *server) StreamEvents(context.Context, StreamEventsRequestObject) (StreamEventsResponseObject, error) {
	return StreamEventsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}

// UpdateSettings is not implemented yet.
func (s *server) UpdateSettings(context.Context, UpdateSettingsRequestObject) (UpdateSettingsResponseObject, error) {
	return UpdateSettingsdefaultApplicationProblemPlusJSONResponse{Body: notImplemented(), StatusCode: 501}, nil
}
