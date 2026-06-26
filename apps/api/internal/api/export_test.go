package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

var (
	SeedSchoolID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	SeedAdminID  = uuid.MustParse("00000000-0000-0000-0000-000000000005")
)

func SessionAuthForTest(pool *pgxpool.Pool, h http.Handler) http.Handler {
	return SessionAuth(sqlc.New(pool))(h)
}

func (a *API) AssertAdminOfSchoolForTest(ctx context.Context, schoolID uuid.UUID) error {
	return a.assertAdminOfSchool(ctx, schoolID)
}
