// Package api is the /api/v1 HTTP surface: domain handlers, the request-user
// context, and the entitlement seam. It drives internal/store, internal/agent
// (turn + eval), and internal/gateway. P1 injects a seeded student via ActAsSeed;
// P2 replaces that middleware with real session auth (same context key).
package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type ctxKey string

const ctxKeyUser ctxKey = "user"

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

// ActAsSeed is the P1 dev middleware: load the seeded student and inject it as
// the request user. P2 swaps this for real session auth with no handler change.
func ActAsSeed(q *sqlc.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			row, err := q.GetUserByID(r.Context(), SeedUserID)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			u := User{ID: row.ID, SchoolID: row.SchoolID, Role: row.Role, DisplayName: row.DisplayName}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
