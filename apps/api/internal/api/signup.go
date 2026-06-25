package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const defaultAvatarColor = "#7C9CF0"

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		JoinCode    string `json:"join_code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	body.JoinCode = strings.TrimSpace(body.JoinCode)
	if body.Email == "" || body.DisplayName == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "邮箱和姓名不能为空", nil))
		return
	}
	if len(body.Password) < 8 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "密码至少 8 位", nil))
		return
	}

	cls, err := a.d.Queries.GetClassByJoinCode(r.Context(), body.JoinCode)
	if err != nil {
		// ErrNoRows or anything else → invalid code (do not leak existence detail).
		httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email:           body.Email,
		PasswordHash:    hash,
		Role:            "student",
		SchoolID:        cls.SchoolID,
		DisplayName:     body.DisplayName,
		AvatarColor:     defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, // auto-verify in P2
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			httpx.WriteError(w, r, httpx.ErrEmailTaken())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateEnrollment(r.Context(), sqlc.CreateEnrollmentParams{
		UserID: u.ID, ClassID: cls.ID, RoleInClass: "student",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{})
}
