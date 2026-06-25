package api

import (
	"net/http"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
)

func (a *API) signout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		// Best-effort revoke; a missing/already-gone session is still a clean signout.
		if err := a.d.Queries.DeleteSessionByHash(r.Context(), auth.HashToken(c.Value)); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	clearSessionCookie(w, a.d.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
