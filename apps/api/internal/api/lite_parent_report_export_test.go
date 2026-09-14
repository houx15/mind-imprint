package api

import (
	"context"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/liteparent"
)

// LoadLiteParentFactsForTest exposes loadLiteParentFacts to the api_test
// package; Task 3's handlers are its production callers.
func (a *API) LoadLiteParentFactsForTest(ctx context.Context, classID, userID, teacherID uuid.UUID, start, end time.Time) (liteparent.Facts, error) {
	return a.loadLiteParentFacts(ctx, classID, userID, teacherID, start, end)
}

// LoadLiteParentOtherNamesForTest exposes loadLiteParentOtherNames.
func (a *API) LoadLiteParentOtherNamesForTest(ctx context.Context, classID, userID uuid.UUID, selfName string) ([]string, error) {
	return a.loadLiteParentOtherNames(ctx, classID, userID, selfName)
}
