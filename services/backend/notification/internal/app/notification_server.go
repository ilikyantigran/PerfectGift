package app

import (
	"context"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/notification/pkg/api/notification/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the notification.v1 gRPC service. Its only synchronous
// surface is device registration; everything else the service does is driven by
// the bus (see the consumers and the dispatcher). It depends on the device side
// of the Store behind an interface so it is unit-testable without Postgres.
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer
	devices notify.DeviceStore
	now     func() time.Time
}

// NewServer builds the gRPC server implementation. now defaults to time.Now.
func NewServer(devices notify.DeviceStore, now func() time.Time) *Server {
	if now == nil {
		now = time.Now
	}
	return &Server{devices: devices, now: now}
}

// RegisterDevice upserts a device by its push token and returns the device id.
func (s *Server) RegisterDevice(ctx context.Context, req *notificationv1.RegisterDeviceRequest) (*notificationv1.RegisterDeviceResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetPushToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "push_token is required")
	}
	platform, ok := platformFromProto(req.GetPlatform())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "platform must be ios or android")
	}

	d, err := s.devices.UpsertDevice(ctx, notify.Device{
		UserID:     req.GetUserId(),
		Platform:   platform,
		PushToken:  req.GetPushToken(),
		AppVersion: req.GetAppVersion(),
	}, s.now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register device: %v", err)
	}
	return &notificationv1.RegisterDeviceResponse{DeviceId: d.ID}, nil
}

// UnregisterDevice deactivates a device by its push token. Idempotent: a token
// that is unknown or already inactive is not an error.
func (s *Server) UnregisterDevice(ctx context.Context, req *notificationv1.UnregisterDeviceRequest) (*notificationv1.UnregisterDeviceResponse, error) {
	if req.GetPushToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "push_token is required")
	}
	if err := s.devices.DeactivateDeviceByToken(ctx, req.GetPushToken()); err != nil {
		return nil, status.Errorf(codes.Internal, "unregister device: %v", err)
	}
	return &notificationv1.UnregisterDeviceResponse{Ok: true}, nil
}

func platformFromProto(p notificationv1.Platform) (notify.Platform, bool) {
	switch p {
	case notificationv1.Platform_PLATFORM_IOS:
		return notify.PlatformIOS, true
	case notificationv1.Platform_PLATFORM_ANDROID:
		return notify.PlatformAndroid, true
	default:
		return "", false
	}
}
