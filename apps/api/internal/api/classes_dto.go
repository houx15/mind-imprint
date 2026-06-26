package api

import (
	"time"

	"mindimprint/api/internal/store/sqlc"
)

type classDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	JoinCode  string `json:"join_code"`
	SchoolID  string `json:"school_id"`
	CreatedAt string `json:"created_at"`
}

func toClassDTO(c sqlc.Class) classDTO {
	return classDTO{
		ID:        c.ID.String(),
		Name:      c.Name,
		JoinCode:  c.JoinCode,
		SchoolID:  c.SchoolID.String(),
		CreatedAt: c.CreatedAt.Format(tsLayout),
	}
}

type rosterEntryDTO struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email"`
	LastActiveAt    *string `json:"last_active_at"`
	TaskCount       int64   `json:"task_count"`
	EvaluationCount int64   `json:"evaluation_count"`
	CardCount       int64   `json:"card_count"`
}

type teacherDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

// toRosterEntryDTO converts a GetClassRosterRow to a rosterEntryDTO.
// LastActiveAt is interface{} from sqlc (MAX over a LEFT JOIN — nullable).
// pgx v5 materialises a non-null timestamptz as time.Time; NULL becomes nil.
func toRosterEntryDTO(row sqlc.GetClassRosterRow) rosterEntryDTO {
	e := rosterEntryDTO{
		ID:              row.ID.String(),
		DisplayName:     row.DisplayName,
		Email:           row.Email,
		TaskCount:       row.TaskCount,
		EvaluationCount: row.EvaluationCount,
		CardCount:       row.CardCount,
	}
	if ts, ok := row.LastActiveAt.(time.Time); ok {
		s := ts.Format(tsLayout)
		e.LastActiveAt = &s
	}
	return e
}
