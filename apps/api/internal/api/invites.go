package api

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

const defaultInviteTTLDays = 14

func (a *API) createTeacherInvite(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Email       string `json:"email"`
		ExpiresDays int    `json:"expires_days"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	days := body.ExpiresDays
	if days <= 0 {
		days = defaultInviteTTLDays
	}
	code, err := org.NewTeacherInviteCode()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var email *string
	if e := strings.TrimSpace(strings.ToLower(body.Email)); e != "" {
		email = &e
	}
	inv, err := a.d.Queries.CreateTeacherInvite(r.Context(), sqlc.CreateTeacherInviteParams{
		SchoolID:  u.SchoolID,
		Code:      code,
		Email:     email,
		CreatedBy: u.ID,
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"code":       inv.Code,
		"expires_at": inv.ExpiresAt.Format(tsLayout),
	})
}

func (a *API) listTeacherInvites(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListActiveTeacherInvitesBySchool(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, inv := range rows {
		m := map[string]any{
			"id":         inv.ID.String(),
			"code":       inv.Code,
			"expires_at": inv.ExpiresAt.Format(tsLayout),
			"created_at": inv.CreatedAt.Format(tsLayout),
		}
		if inv.Email != nil {
			m["email"] = *inv.Email
		}
		out = append(out, m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invites": out})
}
