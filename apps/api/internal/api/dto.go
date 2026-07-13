package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// numericString converts a pgtype.Numeric cost value to a string representation.
// Returns "0" when the value is NULL/invalid.
func numericString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	v, _ := n.Value()
	return fmt.Sprint(v)
}

const tsLayout = time.RFC3339Nano

// nowPlusDays returns the time d days from now (used for invite TTL).
func nowPlusDays(d int) time.Time {
	return time.Now().Add(time.Duration(d) * 24 * time.Hour)
}

// decodeJSON reads a JSON request body into v, mapping any failure to a 400.
func decodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return httpx.ErrBadRequest("validation_failed", "请求体格式错误", nil)
	}
	return nil
}

type evaluationDTO struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	Scores      json.RawMessage `json:"scores"`
	Narrative   string          `json:"narrative"`
	Model       string          `json:"model"`
	Status      string          `json:"status"`
	CreatedAt   string          `json:"created_at"`
	CompletedAt *string         `json:"completed_at"`
}

func toEvaluationDTO(e sqlc.Evaluation) evaluationDTO {
	d := evaluationDTO{
		ID:        e.ID.String(),
		TaskID:    e.TaskID.String(),
		Scores:    json.RawMessage(e.Scores),
		Narrative: e.Narrative,
		Model:     e.Model,
		Status:    e.Status,
		CreatedAt: e.CreatedAt.Format(tsLayout),
	}
	if e.CompletedAt.Valid {
		s := e.CompletedAt.Time.Format(tsLayout)
		d.CompletedAt = &s
	}
	return d
}

type meSchoolDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type meClassDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RoleInClass string `json:"role_in_class"`
}

type meUserDTO struct {
	ID          string       `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Role        string       `json:"role"`
	AvatarColor string       `json:"avatar_color"`
	School      meSchoolDTO  `json:"school"`
	Classes     []meClassDTO `json:"classes"`
}

// buildMeUser assembles the full /me + signin user payload from the principal.
func (a *API) buildMeUser(ctx context.Context, u User) (meUserDTO, error) {
	full, err := a.d.Queries.GetUserByID(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	school, err := a.d.Queries.GetSchool(ctx, u.SchoolID)
	if err != nil {
		return meUserDTO{}, err
	}
	rows, err := a.d.Queries.ListClassesForUser(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	classes := make([]meClassDTO, 0, len(rows))
	for _, c := range rows {
		classes = append(classes, meClassDTO{ID: c.ID.String(), Name: c.Name, RoleInClass: c.RoleInClass})
	}
	return meUserDTO{
		ID:          full.ID.String(),
		Email:       full.Email,
		DisplayName: full.DisplayName,
		Role:        full.Role,
		AvatarColor: full.AvatarColor,
		School:      meSchoolDTO{ID: school.ID.String(), Name: school.Name},
		Classes:     classes,
	}, nil
}
