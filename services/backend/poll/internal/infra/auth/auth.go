// Package auth resolves the acting user's id from the request's JWT and exposes it
// through the context. The acting id is ALWAYS taken from the verified token, never
// from a request body field — body ids are only ever *target* ids.
//
// Tokens are Identity-issued EdDSA JWTs, verified LOCALLY against Identity's JWKS
// (see verifier.go), the same way the API gateway verifies them. Anonymous RPCs
// simply carry no token; owner RPCs check that a subject is present and enforce
// ownership themselves.
package auth

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ctxKey struct{}

// WithSubject stores a resolved subject on the context (used by tests and the
// interceptor).
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, ctxKey{}, subject)
}

// SubjectFrom returns the acting user id resolved from the token, if any.
func SubjectFrom(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(ctxKey{}).(string)
	return s, ok && s != ""
}

// Interceptor resolves JWT subjects using a JWKS Verifier.
type Interceptor struct{ v *Verifier }

// NewInterceptor builds an auth interceptor that verifies Identity's access tokens
// against the JWKS published at jwksURL (e.g. http://identity:8080/.well-known/jwks.json).
func NewInterceptor(jwksURL, issuer, audience string) (*Interceptor, error) {
	v, err := NewVerifier(Config{
		Issuer:   issuer,
		Audience: audience,
		Source:   NewHTTPSource(jwksURL, &http.Client{Timeout: 5 * time.Second}),
	})
	if err != nil {
		return nil, err
	}
	return &Interceptor{v: v}, nil
}

// NewInterceptorWithVerifier wraps an already-built Verifier (used by tests).
func NewInterceptorWithVerifier(v *Verifier) *Interceptor { return &Interceptor{v: v} }

// Unary extracts a bearer token from the "authorization" metadata, verifies it, and
// — when valid — attaches the subject to the context. Invalid or absent tokens are
// NOT rejected here (anonymous RPCs are legitimate); handlers that require an owner
// enforce that themselves.
func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if raw := bearerFromMetadata(ctx); raw != "" {
			if claims, err := i.v.Verify(ctx, raw); err == nil {
				ctx = WithSubject(ctx, claims.Subject)
			}
		}
		return handler(ctx, req)
	}
}

func bearerFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	const prefix = "Bearer "
	v := vals[0]
	if len(v) > len(prefix) && v[:len(prefix)] == prefix {
		return v[len(prefix):]
	}
	return v
}
