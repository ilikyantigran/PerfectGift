// Package clients dials the five downstream domain services over gRPC and exposes
// their generated client stubs. The gateway holds these as interface types, so the
// REST layer depends only on the interfaces (and tests substitute fakes). Trace
// context is propagated into every downstream call via the otel client stats handler.
package clients

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/infra/config"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/catalog/v1"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/identity/v1"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/notification/v1"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/surprise/v1"
)

// Clients bundles the downstream gRPC clients plus the connections backing them.
type Clients struct {
	Identity     identityv1.IdentityServiceClient
	Poll         pollv1.PollServiceClient
	Surprise     surprisev1.SurpriseServiceClient
	Catalog      catalogv1.CatalogServiceClient
	Notification notificationv1.NotificationServiceClient

	conns []*grpc.ClientConn
}

// Dial creates a client connection to each configured downstream. Connections are
// lazy (grpc.NewClient does not block), so this returns quickly and does not require
// the downstreams to be up at startup.
func Dial(cfg *config.Config) (*Clients, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	c := &Clients{}
	dial := func(name, addr string) (*grpc.ClientConn, error) {
		if addr == "" {
			return nil, fmt.Errorf("clients: missing downstream address for %s", name)
		}
		// grpc.NewClient parses the target as a URI: a bare "identity:9090" is read
		// as scheme="identity" and the resolver "produces zero addresses". Give it an
		// explicit dns:/// scheme unless one is already present.
		if !strings.Contains(addr, "://") {
			addr = "dns:///" + addr
		}
		conn, err := grpc.NewClient(addr, opts...)
		if err != nil {
			return nil, fmt.Errorf("clients: dial %s (%s): %w", name, addr, err)
		}
		c.conns = append(c.conns, conn)
		return conn, nil
	}

	idConn, err := dial("identity", cfg.Downstreams.Identity)
	if err != nil {
		c.Close()
		return nil, err
	}
	pollConn, err := dial("poll", cfg.Downstreams.Poll)
	if err != nil {
		c.Close()
		return nil, err
	}
	surpriseConn, err := dial("surprise", cfg.Downstreams.Surprise)
	if err != nil {
		c.Close()
		return nil, err
	}
	catalogConn, err := dial("catalog", cfg.Downstreams.Catalog)
	if err != nil {
		c.Close()
		return nil, err
	}
	notifyConn, err := dial("notification", cfg.Downstreams.Notification)
	if err != nil {
		c.Close()
		return nil, err
	}

	c.Identity = identityv1.NewIdentityServiceClient(idConn)
	c.Poll = pollv1.NewPollServiceClient(pollConn)
	c.Surprise = surprisev1.NewSurpriseServiceClient(surpriseConn)
	c.Catalog = catalogv1.NewCatalogServiceClient(catalogConn)
	c.Notification = notificationv1.NewNotificationServiceClient(notifyConn)
	return c, nil
}

// Close tears down every downstream connection.
func (c *Clients) Close() {
	for _, conn := range c.conns {
		_ = conn.Close()
	}
	c.conns = nil
}
