package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// DepsForTest returns a minimal Deps wired to pool for handler integration tests.
// Gateway/catalog fields are left nil because org routes don't call them.
func DepsForTest(pool *pgxpool.Pool) Deps {
	return Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false}
}

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

func (a *API) AssertTeacherOwnsClassForTest(ctx context.Context, classID uuid.UUID) error {
	_, err := a.assertTeacherOwnsClass(ctx, classID)
	return err
}

// GetEnrollmentParamsForTest builds a GetEnrollmentParams from raw string IDs.
func GetEnrollmentParamsForTest(userID uuid.UUID, classID string) sqlc.GetEnrollmentParams {
	return sqlc.GetEnrollmentParams{
		UserID:  userID,
		ClassID: uuid.MustParse(classID),
	}
}
