package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// accentPresetIDs are the 8 accent preset ids the web design system ships
// (apps/web/src/ui/accent.tsx). Kept as a server-side allowlist so
// avatar_color can only ever hold a known preset.
var accentPresetIDs = map[string]bool{
	"vermilion": true,
	"clay":      true,
	"tangerine": true,
	"bamboo":    true,
	"teal":      true,
	"indigo":    true,
	"violet":    true,
	"rose":      true,
}

type setAccentReq struct {
	Accent string `json:"accent"`
}

// putUserAccent persists the student's chosen accent preset id into the
// existing users.avatar_color column. Validated against the closed preset
// set — a pure preference write, no cost, no model.
func (a *API) putUserAccent(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	var req setAccentReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !accentPresetIDs[req.Accent] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_accent", "未知的强调色。", nil))
		return
	}
	if err := a.d.Queries.SetUserAvatarColor(r.Context(), sqlc.SetUserAvatarColorParams{
		AvatarColor: req.Accent,
		UserID:      u.ID,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"accent": req.Accent})
}
