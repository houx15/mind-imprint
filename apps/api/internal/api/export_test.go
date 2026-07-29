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

// ReadingBriefForTest exposes the unexported readingBriefFor (reading_brief.go,
// Task 3's one new DB-facing function) to the external api_test package, so a
// DB-backed test can assert its return value directly against real
// reference/proposal rows instead of only threading it through an HTTP
// response or a captured LLM prompt.
func (a *API) ReadingBriefForTest(ctx context.Context, projectID, materialID uuid.UUID) agent.ReadingBrief {
	return a.readingBriefFor(ctx, projectID, materialID)
}

// BuildSpineProjectionForTest exposes the unexported buildSpineProjection
// (projectcoach.go) to the external api_test package. buildSpineProjection
// takes no *http.Request (it rides every coach turn AND the workspace
// summary, neither of which always has one to hang a request-scoped test
// helper off of), so — unlike request-driven endpoints elsewhere — a direct
// method call is the only way to assert its output against seeded rows
// (S2's 在读/已归纳 reading-state lines, projectcoach_projection_test.go).
func (a *API) BuildSpineProjectionForTest(ctx context.Context, projectID uuid.UUID) (string, error) {
	return a.buildSpineProjection(ctx, projectID)
}
