package api

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) signin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))

	full, err := a.d.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInvalidCredentials()) // ErrNoRows hidden as invalid creds
		return
	}
	ok, err := auth.VerifyPassword(body.Password, full.PasswordHash)
	if err != nil || !ok {
		httpx.WriteError(w, r, httpx.ErrInvalidCredentials())
		return
	}
	if !full.EmailVerifiedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrEmailUnverified())
		return
	}

	raw, hash, err := auth.NewToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ua := r.UserAgent()
	ip := r.RemoteAddr
	if _, err := a.d.Queries.CreateSession(r.Context(), sqlc.CreateSessionParams{
		UserID:    full.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(sessionTTL),
		UserAgent: &ua,
		Ip:        &ip,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	setSessionCookie(w, raw, a.d.CookieSecure)

	me, err := a.buildMeUser(r.Context(), User{ID: full.ID, SchoolID: full.SchoolID, Role: full.Role, DisplayName: full.DisplayName})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}
