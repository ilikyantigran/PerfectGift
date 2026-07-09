package rest

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/auth"
	catalogv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/catalog/v1"
	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/identity/v1"
	notificationv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/notification/v1"
	pollv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/poll/v1"
	surprisev1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/surprise/v1"
)

// These fakes implement the generated <Svc>Client interfaces. Each method delegates
// to a func field so a test can set exactly the behavior it needs; unset methods
// return an error. This is what lets the whole edge be tested with NO real service,
// DB, or network.

var errNotStubbed = errors.New("fake: method not stubbed")

// --- fake verifier ---

type fakeVerifier struct {
	subject string
	err     error
}

func (f fakeVerifier) Verify(context.Context, string) (*auth.Claims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &auth.Claims{Subject: f.subject}, nil
}

// --- Identity ---

type fakeIdentity struct {
	signIn  func(*identityv1.SignInRequest) (*identityv1.SignInResponse, error)
	refresh func(*identityv1.RefreshTokenRequest) (*identityv1.RefreshTokenResponse, error)
	revoke  func(*identityv1.RevokeRequest) (*identityv1.RevokeResponse, error)
	getMe   func(*identityv1.GetMeRequest) (*identityv1.GetMeResponse, error)
}

func (f *fakeIdentity) SignIn(_ context.Context, in *identityv1.SignInRequest, _ ...grpc.CallOption) (*identityv1.SignInResponse, error) {
	if f.signIn == nil {
		return nil, errNotStubbed
	}
	return f.signIn(in)
}
func (f *fakeIdentity) RefreshToken(_ context.Context, in *identityv1.RefreshTokenRequest, _ ...grpc.CallOption) (*identityv1.RefreshTokenResponse, error) {
	if f.refresh == nil {
		return nil, errNotStubbed
	}
	return f.refresh(in)
}
func (f *fakeIdentity) Revoke(_ context.Context, in *identityv1.RevokeRequest, _ ...grpc.CallOption) (*identityv1.RevokeResponse, error) {
	if f.revoke == nil {
		return nil, errNotStubbed
	}
	return f.revoke(in)
}
func (f *fakeIdentity) GetMe(_ context.Context, in *identityv1.GetMeRequest, _ ...grpc.CallOption) (*identityv1.GetMeResponse, error) {
	if f.getMe == nil {
		return nil, errNotStubbed
	}
	return f.getMe(in)
}

// --- Poll ---

type fakePoll struct {
	createPoll     func(*pollv1.CreatePollRequest) (*pollv1.CreatePollResponse, error)
	getPollByToken func(*pollv1.GetPollByTokenRequest) (*pollv1.GetPollByTokenResponse, error)
	submitResponse func(*pollv1.SubmitResponseRequest) (*pollv1.SubmitResponseResponse, error)
	getResponses   func(*pollv1.GetResponsesRequest) (*pollv1.GetResponsesResponse, error)
}

func (f *fakePoll) CreatePoll(_ context.Context, in *pollv1.CreatePollRequest, _ ...grpc.CallOption) (*pollv1.CreatePollResponse, error) {
	if f.createPoll == nil {
		return nil, errNotStubbed
	}
	return f.createPoll(in)
}
func (f *fakePoll) GetPollByToken(_ context.Context, in *pollv1.GetPollByTokenRequest, _ ...grpc.CallOption) (*pollv1.GetPollByTokenResponse, error) {
	if f.getPollByToken == nil {
		return nil, errNotStubbed
	}
	return f.getPollByToken(in)
}
func (f *fakePoll) SubmitResponse(_ context.Context, in *pollv1.SubmitResponseRequest, _ ...grpc.CallOption) (*pollv1.SubmitResponseResponse, error) {
	if f.submitResponse == nil {
		return nil, errNotStubbed
	}
	return f.submitResponse(in)
}
func (f *fakePoll) GetResponses(_ context.Context, in *pollv1.GetResponsesRequest, _ ...grpc.CallOption) (*pollv1.GetResponsesResponse, error) {
	if f.getResponses == nil {
		return nil, errNotStubbed
	}
	return f.getResponses(in)
}

// --- Surprise ---

type fakeSurprise struct {
	requestGeneration   func(*surprisev1.RequestGenerationRequest) (*surprisev1.RequestGenerationResponse, error)
	getGenerationStatus func(*surprisev1.GetGenerationStatusRequest) (*surprisev1.GetGenerationStatusResponse, error)
	getIdeas            func(*surprisev1.GetIdeasRequest) (*surprisev1.GetIdeasResponse, error)
	refine              func(*surprisev1.RefineRequest) (*surprisev1.RefineResponse, error)
	saveIdea            func(*surprisev1.SaveIdeaRequest) (*surprisev1.SaveIdeaResponse, error)
}

func (f *fakeSurprise) RequestGeneration(_ context.Context, in *surprisev1.RequestGenerationRequest, _ ...grpc.CallOption) (*surprisev1.RequestGenerationResponse, error) {
	if f.requestGeneration == nil {
		return nil, errNotStubbed
	}
	return f.requestGeneration(in)
}
func (f *fakeSurprise) GetGenerationStatus(_ context.Context, in *surprisev1.GetGenerationStatusRequest, _ ...grpc.CallOption) (*surprisev1.GetGenerationStatusResponse, error) {
	if f.getGenerationStatus == nil {
		return nil, errNotStubbed
	}
	return f.getGenerationStatus(in)
}
func (f *fakeSurprise) GetIdeas(_ context.Context, in *surprisev1.GetIdeasRequest, _ ...grpc.CallOption) (*surprisev1.GetIdeasResponse, error) {
	if f.getIdeas == nil {
		return nil, errNotStubbed
	}
	return f.getIdeas(in)
}
func (f *fakeSurprise) Refine(_ context.Context, in *surprisev1.RefineRequest, _ ...grpc.CallOption) (*surprisev1.RefineResponse, error) {
	if f.refine == nil {
		return nil, errNotStubbed
	}
	return f.refine(in)
}
func (f *fakeSurprise) SaveIdea(_ context.Context, in *surprisev1.SaveIdeaRequest, _ ...grpc.CallOption) (*surprisev1.SaveIdeaResponse, error) {
	if f.saveIdea == nil {
		return nil, errNotStubbed
	}
	return f.saveIdea(in)
}

// --- Catalog ---

type fakeCatalog struct {
	listHolidays  func(*catalogv1.ListHolidaysRequest) (*catalogv1.ListHolidaysResponse, error)
	getCategories func(*catalogv1.GetCategoriesRequest) (*catalogv1.GetCategoriesResponse, error)
}

func (f *fakeCatalog) ListHolidays(_ context.Context, in *catalogv1.ListHolidaysRequest, _ ...grpc.CallOption) (*catalogv1.ListHolidaysResponse, error) {
	if f.listHolidays == nil {
		return nil, errNotStubbed
	}
	return f.listHolidays(in)
}
func (f *fakeCatalog) GetCategories(_ context.Context, in *catalogv1.GetCategoriesRequest, _ ...grpc.CallOption) (*catalogv1.GetCategoriesResponse, error) {
	if f.getCategories == nil {
		return nil, errNotStubbed
	}
	return f.getCategories(in)
}

// --- Notification ---

type fakeNotification struct {
	registerDevice func(*notificationv1.RegisterDeviceRequest) (*notificationv1.RegisterDeviceResponse, error)
}

func (f *fakeNotification) RegisterDevice(_ context.Context, in *notificationv1.RegisterDeviceRequest, _ ...grpc.CallOption) (*notificationv1.RegisterDeviceResponse, error) {
	if f.registerDevice == nil {
		return nil, errNotStubbed
	}
	return f.registerDevice(in)
}

// compile-time checks that the fakes satisfy the generated client interfaces.
var (
	_ identityv1.IdentityClient         = (*fakeIdentity)(nil)
	_ pollv1.PollClient                 = (*fakePoll)(nil)
	_ surprisev1.SurpriseClient         = (*fakeSurprise)(nil)
	_ catalogv1.CatalogClient           = (*fakeCatalog)(nil)
	_ notificationv1.NotificationClient = (*fakeNotification)(nil)
)
