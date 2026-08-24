package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// putUserOnboarding stamps users.onboarded_at = now(), marking that the student has
// finished or dismissed the new-user guided tour. No request body.
func (a *API) putUserOnboarding(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	if err := a.d.Queries.SetUserOnboardedAt(r.Context(), u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
