package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// RequireRole rejects a request whose context user's role is not in roles (403).
// It is always composed inside RequireUser, so a missing user is already a 401 by
// the time this runs; defensively, an absent user here is also forbidden.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok || !allowed[u.Role] {
				httpx.WriteError(w, r, httpx.ErrForbidden("权限不足"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// assertAdminOfSchool returns nil only if the caller is an admin of schoolID.
// Any mismatch is reported as not-found so cross-tenant probing can't enumerate.
func (a *API) assertAdminOfSchool(ctx context.Context, schoolID uuid.UUID) error {
	u, ok := UserFromContext(ctx)
	if !ok || u.Role != "admin" || u.SchoolID != schoolID {
		return httpx.ErrNotFound("资源不存在")
	}
	return nil
}

// assertTeacherOwnsClass returns the class if the caller teaches it, or is an
// admin of its school. Otherwise (and for a non-existent class) → not-found.
func (a *API) assertTeacherOwnsClass(ctx context.Context, classID uuid.UUID) (sqlc.Class, error) {
	u, ok := UserFromContext(ctx)
	if !ok {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return sqlc.Class{}, err
	}
	if u.Role == "admin" {
		if u.SchoolID != cls.SchoolID {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return cls, nil
	}
	enr, err := a.d.Queries.GetEnrollment(ctx, sqlc.GetEnrollmentParams{UserID: u.ID, ClassID: classID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return sqlc.Class{}, err
	}
	if enr.RoleInClass != "teacher" {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	return cls, nil
}

// requireEdition gates a route group on the edition of the caller's SCHOOL.
// edition is an organisation fact, not an account one: a school buys the lite
// edition or the pro one and every account under it follows — so this reads
// schools.edition rather than any per-user field.
//
// A mismatch is 404, not 403: from a lite student's point of view the pro
// surface does not exist, and vice versa. Same non-leaking convention as
// ownership failures.
func (a *API) requireEdition(want string, h http.Handler) http.Handler {
	return RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFromContext(r.Context())
		school, err := a.d.Queries.GetSchool(r.Context(), u.SchoolID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if school.Edition != want {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		h.ServeHTTP(w, r)
	}))
}
