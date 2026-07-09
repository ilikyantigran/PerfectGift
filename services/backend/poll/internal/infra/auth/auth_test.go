package auth

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestIssueParse_RoundTrip(t *testing.T) {
	a := New("s3cr3t")
	tok, err := a.Issue("user-42", time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	sub, err := a.Parse(tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sub != "user-42" {
		t.Fatalf("subject=%q want user-42", sub)
	}
}

func TestParse_WrongSecretRejected(t *testing.T) {
	tok, _ := New("right").Issue("u", time.Minute)
	if _, err := New("wrong").Parse(tok); err == nil {
		t.Fatal("expected verification failure with wrong secret")
	}
}

func TestParse_Expired(t *testing.T) {
	a := New("s")
	tok, _ := a.Issue("u", -time.Minute)
	if _, err := a.Parse(tok); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestInterceptor_ValidTokenSetsSubject(t *testing.T) {
	a := New("s")
	tok, _ := a.Issue("owner-1", time.Minute)
	md := metadata.Pairs("authorization", "Bearer "+tok)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var got string
	var present bool
	_, err := a.UnaryInterceptor()(ctx, nil, &grpc.UnaryServerInfo{},
		func(ctx context.Context, _ interface{}) (interface{}, error) {
			got, present = SubjectFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if !present || got != "owner-1" {
		t.Fatalf("subject=%q present=%v want owner-1", got, present)
	}
}

func TestInterceptor_NoTokenIsAnonymous(t *testing.T) {
	a := New("s")
	ctx := context.Background()
	var present bool
	_, _ = a.UnaryInterceptor()(ctx, nil, &grpc.UnaryServerInfo{},
		func(ctx context.Context, _ interface{}) (interface{}, error) {
			_, present = SubjectFrom(ctx)
			return nil, nil
		})
	if present {
		t.Fatal("expected no subject for anonymous request")
	}
}

func TestInterceptor_InvalidTokenIsAnonymousNotRejected(t *testing.T) {
	a := New("s")
	md := metadata.Pairs("authorization", "Bearer not-a-jwt")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	var present bool
	_, err := a.UnaryInterceptor()(ctx, nil, &grpc.UnaryServerInfo{},
		func(ctx context.Context, _ interface{}) (interface{}, error) {
			_, present = SubjectFrom(ctx)
			return nil, nil
		})
	if err != nil {
		t.Fatalf("interceptor must not reject; got %v", err)
	}
	if present {
		t.Fatal("invalid token must not yield a subject")
	}
}
