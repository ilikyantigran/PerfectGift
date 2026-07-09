package rest

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorEnvelope is the single uniform error shape the gateway returns for every
// failure (SERVICE.md §3.1): { "error": { "code", "message", "details" } }.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// writeError writes the uniform error envelope with the given HTTP status.
func writeError(w http.ResponseWriter, httpStatus int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: errorBody{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

// writeGRPCError maps a downstream gRPC status onto an HTTP status + envelope code.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err)
	httpStatus, code := mapGRPCCode(st.Code())
	writeError(w, httpStatus, code, st.Message(), nil)
}

// mapGRPCCode translates a gRPC code to (HTTP status, envelope code string).
func mapGRPCCode(c codes.Code) (int, string) {
	switch c {
	case codes.OK:
		return http.StatusOK, "ok"
	case codes.InvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "unauthenticated"
	case codes.PermissionDenied:
		return http.StatusForbidden, "permission_denied"
	case codes.NotFound:
		return http.StatusNotFound, "not_found"
	case codes.AlreadyExists:
		return http.StatusConflict, "already_exists"
	case codes.Aborted:
		return http.StatusConflict, "aborted"
	case codes.FailedPrecondition:
		return http.StatusUnprocessableEntity, "failed_precondition"
	case codes.OutOfRange:
		return http.StatusBadRequest, "out_of_range"
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, "rate_limited"
	case codes.Canceled:
		return 499, "canceled" // client closed request
	case codes.Unavailable:
		return http.StatusBadGateway, "unavailable"
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "deadline_exceeded"
	case codes.Unimplemented:
		return http.StatusNotImplemented, "unimplemented"
	default: // Internal, Unknown, DataLoss, ...
		return http.StatusInternalServerError, "internal"
	}
}
