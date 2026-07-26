package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
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

// SurfaceAnchorsForTest exposes the unexported surfaceAnchors (studioturn.go)
// to the external api_test package. TestSurfaceAnchorsMetersEmptyResult
// (studioturn_test.go, N6 review finding) needs to drive it directly with a
// synthetic cards.Spec whose Params.Tags AND Steps are both empty — the only
// shape for which agent.fallbackAnchors legitimately returns a zero-length
// anchor slice — because no card in the real embedded registry (craap/sift,
// the only two annotate/compare-primitive cards, both fully tagged) can ever
// produce a real (Resolved.Provider populated) + zero-Anchors GenerateResult
// through the public HTTP turn endpoint: their fallback always yields at
// least one tag-anchor. Calling surfaceAnchors directly, with a synthetic
// spec, is the only way to exercise that path at all.
func (a *API) SurfaceAnchorsForTest(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, spec cards.Spec, cardInstanceID, checkedMaterialID string) ([]byte, bool) {
	return a.surfaceAnchors(ctx, store, projectID, spec, cardInstanceID, checkedMaterialID, false)
}
