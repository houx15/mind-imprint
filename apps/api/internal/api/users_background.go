package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pageBackgroundPresets is the closed set of page background colorways a student
// may choose (the column CHECK is the backstop). Kept in sync with the frontend
// BACKGROUND_PRESETS in ui/background.tsx and the migration 0074 CHECK.
var pageBackgroundPresets = map[string]bool{
	"paper": true, // 温暖纸感 (default)
	"white": true, // 纯白
	"cream": true, // 米纸
	"blue":  true, // 微蓝
	"green": true, // 微绿
	"slate": true, // 微灰
}

type setPageBackgroundReq struct {
	Background string `json:"background"`
}

// putUserBackground sets the student's chosen page background colorway. Validated
// against the closed preset set; the column CHECK is the backstop. A pure
// preference write — no cost, no model.
func (a *API) putUserBackground(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	var req setPageBackgroundReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !pageBackgroundPresets[req.Background] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_background", "未知的背景配色。", nil))
		return
	}
	if err := a.d.Queries.SetUserPageBackground(r.Context(), sqlc.SetUserPageBackgroundParams{
		UserID:         u.ID,
		PageBackground: req.Background,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"background": req.Background})
}
