package api

import (
	"net/http"
	"strings"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
)

// verifyEmail consumes a verification token and marks the account verified.
// Dormant in P2 (signup auto-verifies) but live and tested for the future
// email flow.
func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		httpx.WriteError(w, r, httpx.ErrTokenInvalid())
		return
	}
	evt, err := a.d.Queries.GetActiveEmailVerificationToken(r.Context(), auth.HashToken(token))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrTokenInvalid()) // ErrNoRows → invalid/expired
		return
	}
	if err := a.d.Queries.ConsumeEmailVerificationToken(r.Context(), evt.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := a.d.Queries.MarkEmailVerified(r.Context(), evt.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	me, err := a.buildMeUser(r.Context(), User{ID: u.ID, SchoolID: u.SchoolID, Role: u.Role, DisplayName: u.DisplayName})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}
