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
	// Grade 是闭表里的值（junior2）；GradeLabel 是屏幕上那个词（初二）。
	// 两个都给：前端不必自己维护一份中文对照表，那是第二份会漂的真相。
	Grade      string `json:"grade"`
	GradeLabel string `json:"grade_label"`
}

func toClassDTO(c sqlc.Class) classDTO {
	return classDTO{
		ID:         c.ID.String(),
		Name:       c.Name,
		JoinCode:   c.JoinCode,
		SchoolID:   c.SchoolID.String(),
		CreatedAt:  c.CreatedAt.Format(tsLayout),
		Grade:      c.Grade,
		GradeLabel: classGradeLabel(c.Grade),
	}
}

type rosterEntryDTO struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email"`
	LastActiveAt    *string `json:"last_active_at"`
	ProjectCount    int64   `json:"project_count"`
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
		ProjectCount:    row.ProjectCount,
		EvaluationCount: row.EvaluationCount,
		CardCount:       row.CardCount,
	}
	if ts, ok := row.LastActiveAt.(time.Time); ok {
		s := ts.Format(tsLayout)
		e.LastActiveAt = &s
	}
	return e
}
