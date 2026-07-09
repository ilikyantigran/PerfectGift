package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func decodeEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("body is not the error envelope: %v (%s)", err, body)
	}
	if env.Error.Code == "" || env.Error.Message == "" {
		t.Fatalf("envelope missing code/message: %s", body)
	}
	return env
}

func TestGRPCStatusMapping(t *testing.T) {
	cases := []struct {
		code     codes.Code
		wantHTTP int
		wantCode string
	}{
		{codes.InvalidArgument, http.StatusBadRequest, "invalid_argument"},
		{codes.Unauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{codes.PermissionDenied, http.StatusForbidden, "permission_denied"},
		{codes.NotFound, http.StatusNotFound, "not_found"},
		{codes.AlreadyExists, http.StatusConflict, "already_exists"},
		{codes.FailedPrecondition, http.StatusUnprocessableEntity, "failed_precondition"},
		{codes.ResourceExhausted, http.StatusTooManyRequests, "rate_limited"},
		{codes.Unavailable, http.StatusBadGateway, "unavailable"},
		{codes.DeadlineExceeded, http.StatusGatewayTimeout, "deadline_exceeded"},
		{codes.Unimplemented, http.StatusNotImplemented, "unimplemented"},
		{codes.Internal, http.StatusInternalServerError, "internal"},
		{codes.Unknown, http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		writeGRPCError(rec, status.Error(c.code, "boom"))
		if rec.Code != c.wantHTTP {
			t.Errorf("code %v: HTTP = %d, want %d", c.code, rec.Code, c.wantHTTP)
		}
		env := decodeEnvelope(t, rec.Body.Bytes())
		if env.Error.Code != c.wantCode {
			t.Errorf("code %v: envelope code = %q, want %q", c.code, env.Error.Code, c.wantCode)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("code %v: content-type = %q", c.code, ct)
		}
	}
}

func TestWriteError_Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "invalid_argument", "bad body", map[string]any{"field": "email"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	env := decodeEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != "invalid_argument" || env.Error.Message != "bad body" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if env.Error.Details["field"] != "email" {
		t.Fatalf("details not preserved: %+v", env.Error.Details)
	}
}
