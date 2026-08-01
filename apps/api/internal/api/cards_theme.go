package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type setCardThemeReq struct {
	Theme string `json:"theme"`
}

// putCardTheme sets the student's chosen 工具卡图鉴 cover colorway. Validated
// against the closed theme set; the column CHECK is the backstop. A pure
// preference write — no cost, no model.
func (a *API) putCardTheme(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	var req setCardThemeReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !cards.ValidTheme(req.Theme) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_theme", "未知的封面配色。", nil))
		return
	}
	if err := a.d.Queries.SetUserCardTheme(r.Context(), sqlc.SetUserCardThemeParams{
		UserID:    u.ID,
		CardTheme: req.Theme,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"theme": req.Theme})
}

// resolveCardTheme returns the theme to render the gallery in: an explicit,
// valid ?theme= wins (a preview without persisting); otherwise the student's
// saved card_theme; otherwise the default colorway (also the fallback if the
// read errors, so the gallery never fails to render over a preference lookup).
func (a *API) resolveCardTheme(ctx context.Context, r *http.Request, userID uuid.UUID) string {
	if q := r.URL.Query().Get("theme"); cards.ValidTheme(q) {
		return q
	}
	if saved, err := a.d.Queries.GetUserCardTheme(ctx, userID); err == nil && cards.ValidTheme(saved) {
		return saved
	}
	return cards.DefaultTheme
}
