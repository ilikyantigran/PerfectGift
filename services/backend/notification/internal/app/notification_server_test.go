package app

import (
	"context"
	"testing"
	"time"

	"github.com/ilikyantigran/PerfectGift/services/backend/notification/internal/notify"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/notification/pkg/api/notification/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeDeviceStore is a minimal in-memory notify.DeviceStore for server tests.
type fakeDeviceStore struct {
	byToken map[string]*notify.Device
	seq     int
}

func newFakeDeviceStore() *fakeDeviceStore {
	return &fakeDeviceStore{byToken: make(map[string]*notify.Device)}
}

func (s *fakeDeviceStore) UpsertDevice(_ context.Context, d notify.Device, now time.Time) (notify.Device, error) {
	if ex, ok := s.byToken[d.PushToken]; ok {
		ex.UserID = d.UserID
		ex.Platform = d.Platform
		ex.AppVersion = d.AppVersion
		ex.Active = true
		return *ex, nil
	}
	s.seq++
	d.ID = "dev-" + string(rune('0'+s.seq))
	d.Active = true
	cp := d
	s.byToken[d.PushToken] = &cp
	return cp, nil
}

func (s *fakeDeviceStore) DeactivateDeviceByToken(_ context.Context, token string) error {
	if d, ok := s.byToken[token]; ok {
		d.Active = false
	}
	return nil
}

func (s *fakeDeviceStore) ActiveDevicesForUser(_ context.Context, userID string) ([]notify.Device, error) {
	var out []notify.Device
	for _, d := range s.byToken {
		if d.UserID == userID && d.Active {
			out = append(out, *d)
		}
	}
	return out, nil
}

func TestRegisterDevice_ReturnsDeviceID(t *testing.T) {
	store := newFakeDeviceStore()
	srv := NewServer(store, time.Now)
	resp, err := srv.RegisterDevice(context.Background(), &notificationv1.RegisterDeviceRequest{
		UserId:     "user-1",
		Platform:   notificationv1.Platform_PLATFORM_IOS,
		PushToken:  "tok-1",
		AppVersion: "1.2.3",
	})
	if err != nil {
		t.Fatalf("RegisterDevice: %v", err)
	}
	if resp.GetDeviceId() == "" {
		t.Error("expected a device_id")
	}
}

func TestRegisterDevice_UpsertsSameToken(t *testing.T) {
	store := newFakeDeviceStore()
	srv := NewServer(store, time.Now)
	req := &notificationv1.RegisterDeviceRequest{
		UserId: "user-1", Platform: notificationv1.Platform_PLATFORM_ANDROID, PushToken: "tok",
	}
	r1, _ := srv.RegisterDevice(context.Background(), req)
	r2, _ := srv.RegisterDevice(context.Background(), req)
	if r1.GetDeviceId() != r2.GetDeviceId() {
		t.Errorf("upsert changed id: %s vs %s", r1.GetDeviceId(), r2.GetDeviceId())
	}
	if len(store.byToken) != 1 {
		t.Errorf("device count = %d, want 1 (upsert)", len(store.byToken))
	}
}

func TestRegisterDevice_RejectsUnspecifiedPlatform(t *testing.T) {
	srv := NewServer(newFakeDeviceStore(), time.Now)
	_, err := srv.RegisterDevice(context.Background(), &notificationv1.RegisterDeviceRequest{
		UserId: "user-1", Platform: notificationv1.Platform_PLATFORM_UNSPECIFIED, PushToken: "tok",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestRegisterDevice_RejectsEmptyFields(t *testing.T) {
	srv := NewServer(newFakeDeviceStore(), time.Now)
	_, err := srv.RegisterDevice(context.Background(), &notificationv1.RegisterDeviceRequest{
		Platform: notificationv1.Platform_PLATFORM_IOS, PushToken: "tok",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("missing user_id: code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestUnregisterDevice_DeactivatesAndIsIdempotent(t *testing.T) {
	store := newFakeDeviceStore()
	srv := NewServer(store, time.Now)
	_, _ = srv.RegisterDevice(context.Background(), &notificationv1.RegisterDeviceRequest{
		UserId: "user-1", Platform: notificationv1.Platform_PLATFORM_IOS, PushToken: "tok",
	})

	resp, err := srv.UnregisterDevice(context.Background(), &notificationv1.UnregisterDeviceRequest{PushToken: "tok"})
	if err != nil || !resp.GetOk() {
		t.Fatalf("unregister: ok=%v err=%v", resp.GetOk(), err)
	}
	if store.byToken["tok"].Active {
		t.Error("device should be inactive after unregister")
	}
	// idempotent: unknown token is fine
	if _, err := srv.UnregisterDevice(context.Background(), &notificationv1.UnregisterDeviceRequest{PushToken: "nope"}); err != nil {
		t.Errorf("unregister unknown token should be idempotent, got %v", err)
	}
}
