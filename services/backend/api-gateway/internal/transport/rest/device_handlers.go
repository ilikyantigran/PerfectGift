package rest

import (
	"net/http"

	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/notification/v1"
)

type registerDeviceRequest struct {
	Platform   string `json:"platform"`
	PushToken  string `json:"push_token"`
	AppVersion string `json:"app_version"`
}

// POST /v1/devices → Notification.RegisterDevice (user_id from the JWT subject)
func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	var body registerDeviceRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Notification.RegisterDevice(ctx, &notificationv1.RegisterDeviceRequest{
		UserId:     subjectFrom(r.Context()),
		Platform:   body.Platform,
		PushToken:  body.PushToken,
		AppVersion: body.AppVersion,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"device_id": resp.GetDeviceId()})
}
