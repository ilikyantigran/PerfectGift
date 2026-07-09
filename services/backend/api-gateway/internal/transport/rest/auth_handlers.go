package rest

import (
	"net/http"

	identityv1 "github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/pkg/api/identity/v1"
)

// --- JSON DTOs (snake_case per SERVICE.md §3.1 conventions) ---

type userDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status,omitempty"`
}

func userToDTO(u *identityv1.User) *userDTO {
	if u == nil {
		return nil
	}
	return &userDTO{ID: u.GetId(), Email: u.GetEmail(), DisplayName: u.GetDisplayName(), Status: u.GetStatus()}
}

type signInRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenPairResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int64    `json:"expires_in"`
	User         *userDTO `json:"user,omitempty"`
}

// POST /v1/auth/signin → Identity.SignIn
func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var body signInRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Identity.SignIn(ctx, &identityv1.SignInRequest{
		Provider: body.Provider,
		IdToken:  body.IDToken,
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenPairResponse{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
		ExpiresIn:    resp.GetExpiresIn(),
		User:         userToDTO(resp.GetUser()),
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// POST /v1/auth/refresh → Identity.RefreshToken (public but strictly rate-limited)
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Identity.RefreshToken(ctx, &identityv1.RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenPairResponse{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
		ExpiresIn:    resp.GetExpiresIn(),
	})
}

type revokeRequest struct {
	RefreshToken string `json:"refresh_token"`
	SessionID    string `json:"session_id"`
}

// POST /v1/auth/revoke → Identity.Revoke (JWT)
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	var body revokeRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx, cancel := reqCtx(r)
	defer cancel()

	if _, err := s.opts.Identity.Revoke(ctx, &identityv1.RevokeRequest{
		RefreshToken: body.RefreshToken,
		SessionId:    body.SessionID,
	}); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /v1/me → Identity.GetMe (subject taken from the JWT, never the body)
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := reqCtx(r)
	defer cancel()

	resp, err := s.opts.Identity.GetMe(ctx, &identityv1.GetMeRequest{
		UserId: subjectFrom(r.Context()),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToDTO(resp.GetUser())})
}
