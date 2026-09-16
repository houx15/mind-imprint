package api

import (
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"strings"
)

func combinePblAssignmentBrief(description, instructions string) string {
	description, instructions = strings.TrimSpace(description), strings.TrimSpace(instructions)
	if instructions == "" || description == instructions || strings.HasSuffix(description, "任务说明：\n"+instructions) {
		return description
	}
	if description == "" {
		return "任务说明：\n" + instructions
	}
	return description + "\n\n任务说明：\n" + instructions
}

// New projects snapshot both fields. Existing projects recover instructions
// from their owned assignment, including after the teacher archives it.
func (a *API) pblAssignmentBrief(r *http.Request, atomID uuid.UUID, saved string) (string, error) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		return saved, nil
	}
	instructions, err := a.d.Queries.GetPblAssignmentInstructions(r.Context(), sqlc.GetPblAssignmentInstructionsParams{AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: u.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return saved, nil
	}
	if err != nil {
		return "", err
	}
	return combinePblAssignmentBrief(saved, instructions), nil
}
