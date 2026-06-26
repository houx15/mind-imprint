package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Resolve the code: teacher invite first, then class join code.
	inv, ierr := a.d.Queries.GetActiveTeacherInviteByCode(r.Context(), body.JoinCode)
	if ierr != nil && !errors.Is(ierr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, ierr) // real DB error → 500 via WriteError, never masked
		return
	}
	if ierr == nil {
		if inv.Email != nil && *inv.Email != body.Email {
			httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
			return
		}
		a.signupTeacher(w, r, body.Email, body.DisplayName, hash, inv)
		return
	}
	// ierr == pgx.ErrNoRows → not a teacher invite; try the class join code.
	cls, err := a.d.Queries.GetClassByJoinCode(r.Context(), body.JoinCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	a.signupStudent(w, r, body.Email, body.DisplayName, hash, cls)
}

// signupTeacher creates a teacher (school from the invite, no enrollment) and
// consumes the invite, in one transaction.
func (a *API) signupTeacher(w http.ResponseWriter, r *http.Request, email, displayName, hash string, inv sqlc.TeacherInvite) {
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email: email, PasswordHash: hash, Role: "teacher", SchoolID: inv.SchoolID,
		DisplayName: displayName, AvatarColor: defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, httpx.ErrEmailTaken())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.ConsumeTeacherInvite(r.Context(), sqlc.ConsumeTeacherInviteParams{
		ID: inv.ID, ConsumedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
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

// signupStudent is the unchanged P2 student path, extracted verbatim.
func (a *API) signupStudent(w http.ResponseWriter, r *http.Request, email, displayName, hash string, cls sqlc.Class) {
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email: email, PasswordHash: hash, Role: "student", SchoolID: cls.SchoolID,
		DisplayName: displayName, AvatarColor: defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
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

// isUniqueViolation reports whether err is a Postgres 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
