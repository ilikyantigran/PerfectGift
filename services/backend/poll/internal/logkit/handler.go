// Package logkit is a best-effort, resilient logging client for PerfectGift Go
// services. It provides an slog.Handler that behaves exactly like the services'
// current telemetry handler — always writing a trace-correlated JSON line to
// stdout — while ALSO shipping each record to a central log-server.
//
// Shipping is strictly non-mandatory: the app never blocks or fails because of
// logging. Records flow through a bounded in-memory queue into a batcher, and
// on any send failure they are spooled to disk and backfilled once the server
// returns. If LOG_SERVER_URL is empty, shipping is disabled and logkit is a
// pure stdout logger.
//
// A service installs it by replacing its handler setup with one line:
//
//	flush := logkit.Install("identity")
//	defer flush(context.Background())
package logkit

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Handler is the slog.Handler returned by NewHandler. It writes every record to
// stdout (identical to the pre-logkit behaviour) and, when shipping is enabled,
// enqueues a Record for asynchronous delivery to the log-server.
type Handler struct {
	service string
	minLvl  slog.Level

	// stdout is a standard slog JSON handler; delegating to it keeps stdout
	// byte-for-byte compatible with the services' existing output.
	stdout slog.Handler

	// ship enqueues a Record for delivery. nil when shipping is disabled.
	ship func(Record)

	// preset holds attrs added via WithAttrs, tagged with the groups that were
	// open when they were added, so Record.Fields can be reconstructed.
	preset []groupedAttr
	// groups is the stack of currently open groups (from WithGroup).
	groups []string

	// shp is the shared shipper; only the root handler owns Close.
	shp  *shipper
	root bool
}

type groupedAttr struct {
	groups []string
	attr   slog.Attr
}

// NewHandler builds a logkit slog.Handler for serviceName. Left-zero Options
// fields are backfilled with defaults. If opts.ServerURL is empty, the returned
// handler is stdout-only and Close is a no-op.
func NewHandler(serviceName string, opts Options) *Handler {
	opts = opts.withDefaults()

	base := slog.NewJSONHandler(opts.Stdout, &slog.HandlerOptions{Level: opts.Level}).
		WithAttrs([]slog.Attr{slog.String("service", serviceName)})

	h := &Handler{
		service: serviceName,
		minLvl:  opts.Level,
		stdout:  base,
		root:    true,
	}

	if opts.ServerURL != "" {
		h.shp = newShipper(serviceName, opts)
		h.ship = h.shp.enqueue
	}
	return h
}

// Install wires a logkit handler (configured from the environment) as the
// default slog logger for serviceName and returns a flush function to call on
// shutdown. It is the one-line replacement for the services' telemetry logger
// setup.
func Install(serviceName string) (flush func(context.Context)) {
	h := NewHandler(serviceName, OptionsFromEnv())
	slog.SetDefault(slog.New(h))
	return h.Close
}

// Enabled reports whether records at the given level are handled.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLvl
}

// Handle writes the record to stdout and, when shipping is enabled, enqueues a
// Record. It never blocks on shipping and never returns a shipping error.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	traceID, spanID := "", ""
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		traceID = sc.TraceID().String()
		spanID = sc.SpanID().String()
	}

	// stdout: identical to the existing contextHandler — add trace/span IDs as
	// top-level attrs, then delegate to the JSON handler.
	sr := r.Clone()
	if traceID != "" {
		sr.AddAttrs(
			slog.String("trace_id", traceID),
			slog.String("span_id", spanID),
		)
	}
	stdoutErr := h.stdout.Handle(ctx, sr)

	// shipping: best-effort, never blocks, never fails the caller.
	if h.ship != nil {
		h.ship(Record{
			Ts:      formatTS(r.Time),
			Level:   levelString(r.Level),
			Service: h.service,
			Message: r.Message,
			TraceID: traceID,
			SpanID:  spanID,
			Fields:  h.buildFields(r),
		})
	}
	return stdoutErr
}

// WithAttrs returns a handler that adds attrs to every record.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	nh := h.clone()
	nh.stdout = h.stdout.WithAttrs(attrs)
	groups := cloneStrs(h.groups)
	for _, a := range attrs {
		nh.preset = append(nh.preset, groupedAttr{groups: groups, attr: a})
	}
	return nh
}

// WithGroup returns a handler that nests subsequent attrs under name.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.stdout = h.stdout.WithGroup(name)
	nh.groups = append(cloneStrs(h.groups), name)
	return nh
}

// Close drains the queue, flushes in-flight batches, attempts a final spool
// drain, and stops background goroutines. Only the root handler owns shutdown;
// derived (WithAttrs/WithGroup) handlers are no-ops.
func (h *Handler) Close(ctx context.Context) {
	if h.root && h.shp != nil {
		h.shp.close(ctx)
	}
}

// Flush synchronously drains the queue, sends any pending batch, and attempts a
// spool drain, without stopping the handler. Primarily for tests and explicit
// checkpoints.
func (h *Handler) Flush(ctx context.Context) {
	if h.shp != nil {
		h.shp.flush(ctx)
	}
}

func (h *Handler) clone() *Handler {
	c := *h
	c.root = false // derived handlers never own Close
	return &c
}

// buildFields reconstructs the record's structured attributes as a nested map,
// honouring open groups. Never returns nil (encodes as {}).
func (h *Handler) buildFields(r slog.Record) map[string]any {
	fields := map[string]any{}
	for _, ga := range h.preset {
		putAttr(fields, ga.groups, ga.attr)
	}
	r.Attrs(func(a slog.Attr) bool {
		putAttr(fields, h.groups, a)
		return true
	})
	return fields
}

// putAttr inserts attr into fields at the path given by groups, creating nested
// maps as needed. Group-valued attrs recurse.
func putAttr(fields map[string]any, groups []string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) { // empty attr: skip, per slog conventions
		return
	}

	dst := fields
	for _, g := range groups {
		dst = child(dst, g)
	}

	if attr.Value.Kind() == slog.KindGroup {
		sub := attr.Value.Group()
		if len(sub) == 0 { // empty group: omit, per slog conventions
			return
		}
		if attr.Key == "" { // inline group into current level
			for _, sa := range sub {
				putAttr(dst, nil, sa)
			}
			return
		}
		target := child(dst, attr.Key)
		for _, sa := range sub {
			putAttr(target, nil, sa)
		}
		return
	}

	dst[attr.Key] = attr.Value.Any()
}

// child returns the nested map at key within m, creating it if absent (or if a
// non-map value already occupies the key).
func child(m map[string]any, key string) map[string]any {
	if existing, ok := m[key].(map[string]any); ok {
		return existing
	}
	sub := map[string]any{}
	m[key] = sub
	return sub
}

func cloneStrs(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
