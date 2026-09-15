package api

import (
	"context"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/liteweekly"
)

// LoadLiteStudentWeekForTest exposes loadLiteStudentWeek to the api_test
// package; Task 5's handlers are its production callers.
func (a *API) LoadLiteStudentWeekForTest(ctx context.Context, classID, userID uuid.UUID, weekStart time.Time) (liteweekly.StudentWeek, error) {
	return a.loadLiteStudentWeek(ctx, classID, userID, weekStart)
}

// LoadLiteClassWeekForTest exposes loadLiteClassWeek to the api_test package.
func (a *API) LoadLiteClassWeekForTest(ctx context.Context, classID uuid.UUID, weekStart time.Time) ([]liteweekly.StudentWeek, error) {
	return a.loadLiteClassWeek(ctx, classID, weekStart)
}
