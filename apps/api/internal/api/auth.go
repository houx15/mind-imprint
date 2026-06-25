// Package api is the /api/v1 HTTP surface: domain handlers, the request-user
// context, and the entitlement seam. It drives internal/store, internal/agent
// (turn + eval), and internal/gateway. P2 uses real session auth via SessionAuth
// + RequireUser (P1 ActAsSeed shim removed).
package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type ctxKey string

const ctxKeyUser ctxKey = "user"

// TxBeginner is the subset of *pgxpool.Pool the signup transaction needs.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// User is the request-scoped principal. Every user has a school (org invariant).
type User struct {
	ID          uuid.UUID
	SchoolID    uuid.UUID
	Role        string
	DisplayName string
}

// WithUser stores u on the context. UserFromContext reads it back.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKeyUser, u)
}

// UserFromContext returns the request user, or ok=false if absent.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(User)
	return u, ok
}

// SeedUserID is the deterministic seeded student from migration 0002_seed.sql.
var SeedUserID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

// SessionAuth resolves the mk_session cookie into the request user context if a
// valid (unexpired) session exists. It never rejects on its own — absence just
// means no user in context; RequireUser does the rejecting. Replaces the P1
// ActAsSeed dev shim.
func SessionAuth(q *sqlc.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			row, err := q.GetSessionWithUserByHash(r.Context(), auth.HashToken(c.Value))
			if err != nil {
				next.ServeHTTP(w, r) // invalid/expired → unauthenticated
				return
			}
			u := User{ID: row.ID, SchoolID: row.SchoolID, Role: row.Role, DisplayName: row.DisplayName}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}

// RequireUser rejects requests with no authenticated user (401).
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
