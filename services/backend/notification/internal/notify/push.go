package notify

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidToken is returned by a Pusher when a provider reports the token as
// invalid/expired (APNs 410 Unregistered, FCM UNREGISTERED). It is a *terminal*
// per-device outcome, not a transient failure: the dispatcher deactivates the
// device rather than retrying it.
var ErrInvalidToken = errors.New("push token invalid or expired")

// PushMessage is the payload handed to a provider for one device.
type PushMessage struct {
	Type    Type
	Payload []byte
}

// Pusher delivers a message to a single device. Implementations: APNs (iOS) and
// FCM (Android). Tests use a fake. A transient error (network, 5xx, throttle)
// should be returned as a plain error → the dispatcher retries the row with
// backoff. A dead token should be returned as ErrInvalidToken → the dispatcher
// deactivates the device and does not treat it as a row-level failure.
type Pusher interface {
	Push(ctx context.Context, device Device, msg PushMessage) error
}

// Router picks the Pusher for a device's platform. It lets the dispatcher stay
// platform-agnostic and makes "no provider configured for this platform"
// an explicit, testable condition.
type Router struct {
	byPlatform map[Platform]Pusher
}

// NewRouter builds a router from a platform→Pusher map.
func NewRouter(m map[Platform]Pusher) *Router {
	byPlatform := make(map[Platform]Pusher, len(m))
	for k, v := range m {
		byPlatform[k] = v
	}
	return &Router{byPlatform: byPlatform}
}

// Push routes to the provider for the device's platform.
func (r *Router) Push(ctx context.Context, device Device, msg PushMessage) error {
	p, ok := r.byPlatform[device.Platform]
	if !ok || p == nil {
		return fmt.Errorf("no push provider for platform %q", device.Platform)
	}
	return p.Push(ctx, device, msg)
}
