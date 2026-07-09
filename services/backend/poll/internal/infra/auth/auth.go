// Package auth resolves the acting user's id from the request's JWT and exposes
// it through the context. The acting id is ALWAYS taken from the verified token,
// never from a request body field — body ids are only ever *target* ids.
//
// Tokens are HS256, signed by Identity with a shared secret. (JWKS/RS256 is the
// eventual target; the interface here lets us swap the verifier without touching
// the handlers.) Anonymous RPCs simply carry no token; owner RPCs check that a
// subject is present and enforce ownership themselves.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ctxKey struct{}

// Authenticator verifies HS256 JWTs with a shared secret.
type Authenticator struct {
	secret []byte
}

func New(secret string) *Authenticator { return &Authenticator{secret: []byte(secret)} }

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

// Parse verifies a raw JWT string and returns its subject.
func (a *Authenticator) Parse(raw string) (string, error) {
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", err
	}
	sub, err := tok.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", errors.New("token has no subject")
	}
	return sub, nil
}

// Issue mints an HS256 token for the subject. Primarily for local dev and tests;
// in production Identity issues these.
func (a *Authenticator) Issue(subject string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// UnaryInterceptor extracts a bearer token from the "authorization" metadata,
// verifies it, and — when valid — attaches the subject to the context. Invalid or
// absent tokens are NOT rejected here (anonymous RPCs are legitimate); handlers
// that require an owner enforce that themselves.
func (a *Authenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if raw := bearerFromMetadata(ctx); raw != "" {
			if sub, err := a.Parse(raw); err == nil {
				ctx = WithSubject(ctx, sub)
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
