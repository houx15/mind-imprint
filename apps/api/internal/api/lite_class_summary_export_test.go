package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ComposeLiteClassSummaryForTest runs the single-flight summary computation
// for classID as if r were the first requester's request. The class and
// roster are loaded on a background context, so only the computation itself
// sees r's context.
func (a *API) ComposeLiteClassSummaryForTest(r *http.Request, userID, classID uuid.UUID) (string, error) {
	ctx := context.Background()
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return "", err
	}
	roster, err := a.liteWorkspaceRoster(ctx, cls.ID)
	if err != nil {
		return "", err
	}
	entry, err := a.composeLiteClassSummary(r, userID, cls, roster)
	return entry.Summary, err
}
